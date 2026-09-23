package gitpanel

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/hittable/shellapp/internal/commitmsg"
	"github.com/hittable/shellapp/ui/components/texteditor"
	"github.com/hittable/shellapp/ui/theme"
)

// composePath is the virtual path the message buffer is filed under. It has
// no extension on purpose: Chroma then picks no lexer, so a commit message is
// never syntax-highlighted as whatever language it happens to resemble.
const composePath = "COMMIT_EDITMSG"

// typeAbbrev shortens a type for the narrow rungs of the picker's shrink
// ladder. The footer is a single truncated row, so the full list does not fit
// on a small terminal.
var typeAbbrev = map[string]string{
	"feat": "fea", "fix": "fix", "docs": "doc", "style": "sty",
	"refactor": "ref", "perf": "per", "test": "tes", "build": "bui",
	"ci": "ci", "chore": "cho", "revert": "rev",
}

// startCompose collects the staged changes and opens the type picker. It is
// the `c` key's entry point. Collecting is only git plumbing, so it stays on
// the UI goroutine; generation is what goes async.
func (p *Panel) startCompose() {
	if p.Repo == nil {
		return
	}
	d, err := commitmsg.Collect(p.Repo)
	if err != nil {
		p.Message = "commit: " + err.Error()
		return
	}
	if len(d.Files) == 0 {
		p.Message = "nothing staged"
		return
	}
	p.digest = d

	// A repo that does not use conventional commits should not have one
	// imposed on it: skip the picker and use rules with no type requirement,
	// so neither the draft nor the commit-time repair invents one. PlainRules
	// rather than a mutated DefaultRules — a Rules value whose fields are all
	// zero cannot be told apart from an unset one, and gets defaulted back.
	if !d.Convention {
		p.composeRules = commitmsg.PlainRules()
		p.openCompose("")
		return
	}
	p.composeRules = commitmsg.DefaultRules()
	p.composeType = commitmsg.InferType(d)
	p.prompt, p.promptTitle = promptType, "type"
}

// openCompose replaces the detail pane with the message editor, pre-filled
// with the heuristic draft, and starts generation when a model is installed.
// The editor opens before the model answers so there is always something
// usable on screen.
func (p *Panel) openCompose(typ string) {
	p.composeType = typ
	text := p.draft
	if text == "" {
		text = commitmsg.Generate(p.digest, commitmsg.Options{
			Type:  typ,
			Rules: p.composeRules,
		}).String()
	}
	ed := texteditor.New()
	ed.SetContent(composePath, text)
	ed.SetSize(p.Width-4, p.detailRows()-2)
	ed.Focus()
	ed.OnChanged = func(string) { p.userEdited = true }
	p.MsgEditor, p.Composing, p.userEdited = ed, true, false
	p.cursorToEnd()

	// Only a hand-edited draft blocks generation. A draft that is merely the
	// last mechanical one must not suppress the model forever — that is what
	// happens to anyone who presses c once, escapes, then installs a model.
	if !p.draftEdited {
		p.startGeneration()
	}
}

// startGeneration asks the model to rewrite the draft. Chunks arrive on a
// background goroutine and are delivered as coalesced messages; with no Send
// hook wired (as in the panel's own tests) it runs inline instead, mirroring
// how async degrades when Async is nil.
func (p *Panel) startGeneration() {
	if p.Drafter == nil || p.digest == nil {
		return
	}
	p.genSeq++
	seq := p.genSeq
	ctx, cancel := context.WithCancel(context.Background())
	p.genCancel = cancel
	p.Generating = true
	p.genStart = time.Now()
	p.Busy = "drafting message"

	p.genMu.Lock()
	p.genBuf.Reset()
	p.genMu.Unlock()

	d := p.digest
	opts := commitmsg.Options{Type: p.composeType, Rules: p.composeRules}

	if p.Send == nil {
		msg, err := p.Drafter.DraftStream(ctx, d, opts, func(string) {})
		p.GenDone(GenDoneMsg{Seq: seq, Text: msg.String(), Source: msg.Source, Err: err})
		return
	}
	go func() {
		msg, err := p.Drafter.DraftStream(ctx, d, opts, func(s string) {
			p.genMu.Lock()
			p.genBuf.WriteString(s)
			p.genMu.Unlock()
			p.wakeGen(seq)
		})
		p.Send(GenDoneMsg{Seq: seq, Text: msg.String(), Source: msg.Source, Err: err})
	}()
}

// wakeGen coalesces a token burst into at most one redraw per 8ms. A 3B model
// emits 30-80 tokens a second and each redraw repaints the whole frame, so
// sending per token would spend the frame budget on nothing. Same shape as the
// terminal's wake, for the same reason.
func (p *Panel) wakeGen(seq int) {
	if !p.genWake.CompareAndSwap(false, true) {
		return
	}
	go func() {
		time.Sleep(8 * time.Millisecond)
		p.genWake.Store(false)
		if p.Send != nil {
			p.Send(GenChunkMsg{Seq: seq})
		}
	}()
}

// GenChunk applies whatever has streamed so far.
func (p *Panel) GenChunk(msg GenChunkMsg) {
	if msg.Seq != p.genSeq || !p.Composing || p.userEdited || p.MsgEditor == nil {
		return
	}
	p.genMu.Lock()
	text := p.genBuf.String()
	p.genMu.Unlock()
	if strings.TrimSpace(text) == "" {
		return
	}
	p.MsgEditor.SetContent(composePath, text)
	p.cursorToEnd()
}

// GenDone ends a generation. A cancelled or failed run keeps whatever is
// already in the buffer — which is never empty, because the heuristic draft
// was there before the model started.
func (p *Panel) GenDone(msg GenDoneMsg) {
	if msg.Seq != p.genSeq {
		return
	}
	p.Generating, p.Busy, p.genCancel = false, "", nil
	if msg.Err != nil {
		if ctxCancelled(msg.Err) {
			p.Message = "generation cancelled"
		} else {
			p.Message = "heuristic draft · " + msg.Err.Error()
		}
		return
	}
	if p.Composing && !p.userEdited && p.MsgEditor != nil && strings.TrimSpace(msg.Text) != "" {
		p.MsgEditor.SetContent(composePath, msg.Text)
		p.cursorToEnd()
	}
	// Label the draft honestly. A user told when the model was discarded will
	// believe the label the times it is not.
	switch msg.Source {
	case commitmsg.SourceHeuristic:
		p.Message = "model output rejected · heuristic draft"
	case commitmsg.SourceRepaired:
		p.Message = "drafted by the model · reformatted to match commitlint"
	}
}

func ctxCancelled(err error) bool {
	return err == context.Canceled || strings.Contains(err.Error(), context.Canceled.Error())
}

// cursorToEnd puts the caret on the last line. SetContent restores the cursor
// to where it was, which during a stream is the top of a buffer that has since
// grown, so the view would never follow the text being written.
func (p *Panel) cursorToEnd() {
	if p.MsgEditor == nil {
		return
	}
	p.MsgEditor.GotoLine(strings.Count(p.MsgEditor.GetContent(), "\n"))
}

// CancelGeneration stops any in-flight model call, keeping whatever has
// already streamed. The screen calls this when the model goes away underneath
// it — `hittable model disable` in the integrated terminal, say.
func (p *Panel) CancelGeneration() { p.cancelGeneration() }

// cancelGeneration stops an in-flight model call and keeps the buffer.
func (p *Panel) cancelGeneration() {
	if p.genCancel != nil {
		p.genCancel()
		p.genCancel = nil
	}
	p.genSeq++ // any chunk still in flight is now stale
	p.Generating, p.Busy = false, ""
}

// stopCompose leaves the message editor, keeping the text so the next c
// restores it.
func (p *Panel) stopCompose() {
	p.cancelGeneration()
	if p.MsgEditor != nil {
		p.draft, p.draftEdited = p.MsgEditor.GetContent(), p.userEdited
	}
	p.Composing, p.MsgEditor, p.digest = false, nil, nil
}

// commitCompose validates the buffer and commits. Repair is mechanical and
// takes microseconds, so it runs inline; the commit itself goes through run,
// as every other mutation in the panel does.
func (p *Panel) commitCompose() {
	if p.MsgEditor == nil {
		return
	}
	p.cancelGeneration()
	text := strings.TrimSpace(p.MsgEditor.GetContent())
	if text == "" {
		p.Message = "empty commit message"
		return
	}
	if repaired, ok := commitmsg.Repair(text, p.composeRules, p.composeType); ok {
		text = repaired
	}
	p.Composing, p.MsgEditor, p.draft, p.draftEdited, p.digest = false, nil, "", false, nil
	p.run("commit", func() error { return p.Repo.Commit(text) })
}

// detailEditor is whichever editor currently owns the detail pane, or nil.
// Mouse events over that pane go to it rather than to diff selection.
func (p *Panel) detailEditor() *texteditor.TextEditor {
	switch {
	case p.Composing:
		return p.MsgEditor
	case p.Editing:
		return p.Editor
	}
	return nil
}

// genElapsed is how long the current generation has been running, rendered
// for the footer. A 3B spends seconds on prompt processing before it emits a
// single token, so a spinner alone leaves the user unsure anything is
// happening; a climbing number is unambiguous.
func (p *Panel) genElapsed() string {
	if !p.Generating || p.genStart.IsZero() {
		return ""
	}
	return fmt.Sprintf(" %ds", int(time.Since(p.genStart).Seconds()))
}

// composeHint is the message editor's status-row help.
func (p *Panel) composeHint() string {
	switch {
	case p.Generating:
		return "drafting" + p.genElapsed() + " · esc cancel · ctrl+s commit"
	case p.Drafter != nil:
		return "commit message · ctrl+s commit · ctrl+r redraft · esc cancel"
	default:
		return "commit message · ctrl+s commit · esc cancel"
	}
}

// handleComposeKey routes keys while the message editor is up.
func (p *Panel) handleComposeKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		if p.Generating {
			p.cancelGeneration()
			p.Message = "generation cancelled"
			return
		}
		p.stopCompose()
		p.Refresh()
	case "ctrl+s":
		p.commitCompose()
	case "ctrl+r":
		if p.Drafter != nil {
			p.userEdited = false
			p.startGeneration()
		}
	default:
		p.MsgEditor.Update(msg)
	}
}

// ---------- type picker ----------

// handleTypeKey drives the conventional-commit type picker.
func (p *Panel) handleTypeKey(msg tea.KeyMsg) {
	types := commitmsg.DefaultRules().Types
	i := indexOf(types, p.composeType)
	switch msg.String() {
	case "esc":
		p.prompt, p.digest = promptNone, nil
	case "enter":
		p.prompt = promptNone
		p.openCompose(p.composeType)
	case "left", "up", "shift+tab":
		p.composeType = types[(i-1+len(types))%len(types)]
	case "right", "down", "tab":
		p.composeType = types[(i+1)%len(types)]
	default:
		// First letter jumps, as the method picker in the URL bar does.
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
			r := strings.ToLower(string(msg.Runes))
			for n := 1; n <= len(types); n++ {
				c := types[(i+n)%len(types)]
				if strings.HasPrefix(c, r) {
					p.composeType = c
					break
				}
			}
		}
	}
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return 0
}

// typePickerFoot renders the picker into the single footer row, shrinking to
// fit: full names, then three-letter abbreviations, then the selected type
// alone with arrows. The frame-size regression renders down to 50 columns and
// a row wider than the pane would wrap and scroll the whole frame.
func (p *Panel) typePickerFoot(width int) string {
	types := commitmsg.DefaultRules().Types
	sel := indexOf(types, p.composeType)

	build := func(label func(string) string) string {
		var b strings.Builder
		b.WriteString(" type ")
		for i, t := range types {
			if i > 0 {
				b.WriteString(" ")
			}
			if i == sel {
				b.WriteString(theme.PromptStyle.Render(" " + label(t) + " "))
			} else {
				b.WriteString(theme.MutedStyle.Render(label(t)))
			}
		}
		return b.String()
	}

	full := build(func(t string) string { return t })
	if ansi.StringWidth(full) <= width {
		return full
	}
	short := build(func(t string) string { return typeAbbrev[t] })
	if ansi.StringWidth(short) <= width {
		return short
	}
	// Last rung: the selection and the arrows telling you it can move. The
	// truncate is a backstop for a pane too narrow even for that.
	minimal := theme.MutedStyle.Render(" ‹ ") +
		theme.PromptStyle.Render(" "+p.composeType+" ") +
		theme.MutedStyle.Render(" ›")
	return ansi.Truncate(minimal, width, "…")
}
