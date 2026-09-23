package screens

import (
	"context"
	"os"
	"time"

	"github.com/hittable/shellapp/internal/appconfig"
	"github.com/hittable/shellapp/internal/commitmsg"
	"github.com/hittable/shellapp/internal/hithome"
	"github.com/hittable/shellapp/internal/llm"
	"github.com/hittable/shellapp/internal/llmhost"
)

// ai owns the optional local model. Everything here is inert until something
// actually asks for a draft: constructing it performs no network I/O, starts
// no process and reads no model file, because it runs on every launch
// including the overwhelming majority where nothing is installed.
//
// It is not a cache of "is AI available" so much as a cheap observer of it:
// enabling a model from the integrated terminal has to take effect in the
// running app, so live is re-derived from disk on the existing status tick
// rather than decided once at startup.
type ai struct {
	cfg       appconfig.AI
	sup       *llmhost.Supervisor
	client    *llm.HTTPClient
	live      bool   // a model is installed and enabled right now
	cfgStamp  string // config file mtime+size, to notice external edits
	announced bool   // the "AI drafts enabled" line has been shown once
}

func newAI(root string) *ai {
	cfg, _ := appconfig.Load(root)
	a := &ai{cfg: cfg.AI, sup: llmhost.NewSupervisor(), cfgStamp: configStamp()}
	a.client = llm.New(llm.Config{
		Endpoint:    a.endpoint,
		Timeout:     time.Duration(cfg.AI.TimeoutMs) * time.Millisecond,
		FastTimeout: time.Duration(cfg.AI.CompletionTimeoutMs) * time.Millisecond,
	})
	a.live = a.available()
	return a
}

// endpoint is what the llm client calls per request. A configured endpoint is
// used as-is and never spawns or downloads anything; otherwise the supervisor
// starts the sidecar lazily, on this first call.
func (a *ai) endpoint(ctx context.Context) (string, error) {
	if a.cfg.Endpoint != "" {
		return a.cfg.Endpoint, nil
	}
	return a.sup.Endpoint(ctx)
}

// available is two os.Stat calls plus a bool, cheap enough for the 2s tick.
func (a *ai) available() bool {
	if !a.cfg.Enabled {
		return false
	}
	return a.cfg.Endpoint != "" || llmhost.Installed()
}

// drafter is what the Git panel holds. Returning a nil interface (rather than
// a non-nil interface wrapping a nil pointer) matters: the panel tests
// `Drafter == nil` to decide whether to generate at all.
func (a *ai) drafter() gitDrafter {
	if !a.live {
		return nil
	}
	return commitmsg.ClientDrafter{Client: a.client}
}

// gitDrafter mirrors the interface the git panel declares. Naming it here
// keeps the screen honest about what it is handing over.
type gitDrafter interface {
	DraftStream(ctx context.Context, d *commitmsg.Digest, opts commitmsg.Options, onChunk func(string)) (commitmsg.Message, error)
}

func configStamp() string {
	fi, err := os.Stat(hithome.ConfigPath())
	if err != nil {
		return ""
	}
	return fi.ModTime().UTC().Format(time.RFC3339Nano) + "|" + itoa(fi.Size())
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// refreshAI re-derives whether a model is usable, so `hittable model enable`
// run in the integrated terminal lights the feature up without a restart, and
// `model disable` turns it off again. Called from the existing 2s git tick,
// which already shells out to git — two stat calls alongside that are noise.
//
// Returns a status line when the answer changed, otherwise "".
func (m *MainScreen) refreshAI() string {
	if m.AI == nil {
		return ""
	}
	if stamp := configStamp(); stamp != m.AI.cfgStamp {
		m.AI.cfgStamp = stamp
		cfg, _ := appconfig.Load(m.RootDir)
		m.AI.cfg = cfg.AI
	}
	was := m.AI.live
	m.AI.live = m.AI.available()
	if was == m.AI.live {
		return ""
	}
	m.Git.Drafter = m.AI.drafter()
	if m.AI.live {
		// Only announce the transition, not the state: someone who launched
		// with a model already installed does not need to be told.
		if m.AI.announced {
			return "AI commit drafts enabled"
		}
		m.AI.announced = true
		return "AI commit drafts enabled — press c in the Git panel"
	}
	// Anything mid-flight is now pointing at a model that is being deleted.
	m.Git.CancelGeneration()
	return "AI commit drafts disabled"
}

// CloseAI stops a sidecar this process started. A server adopted from another
// hittable window is left alone.
func (m *MainScreen) CloseAI() {
	if m.AI != nil && m.AI.sup != nil {
		m.AI.sup.Close()
	}
}
