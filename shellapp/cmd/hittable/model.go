package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hittable/shellapp/internal/appconfig"
	"github.com/hittable/shellapp/internal/llmhost"
)

var flagModelYes bool
var flagModelDryRun bool

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage the local model used for AI commit messages",
	Long: `hittable can draft commit messages with a small model running on this machine.
Nothing is downloaded until you run ` + "`hittable model enable`" + `, and everything it
installs lives in one directory that ` + "`hittable model delete`" + ` removes.

  hittable model enable    download the runtime and the model (asks first)
  hittable model status    what is installed, how much disk, is it running
  hittable model disable   stop the server and free the memory, keep the files
  hittable model delete    remove it from this machine and reclaim the disk

Without a model the Git panel still drafts a commit message from the staged
diff; the model rewrites it, and every claim it makes is checked back against
the changes before you see it.`,
}

var modelEnableCmd = &cobra.Command{
	Use:           "enable",
	Short:         "Download and set up the local model",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runModelEnable,
}

var modelDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Stop the model server and turn drafting off, keeping the files",
	Long: `Stops the local model server and turns AI drafting off.

The model stays on disk, so ` + "`hittable model enable`" + ` turns it straight back
on with nothing to download. Use ` + "`hittable model delete`" + ` to reclaim the disk.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runModelDisable,
}

var modelDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Remove the model and runtime from this machine entirely",
	Long: `Stops the server and deletes everything hittable downloaded.

Re-enabling afterwards means downloading the model again. To stop the server
and free memory without that cost, use ` + "`hittable model disable`" + `.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runModelDelete,
}

var modelStatusCmd = &cobra.Command{
	Use:           "status",
	Short:         "Show what is installed and whether it is running",
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runModelStatus,
}

func runModelEnable(*cobra.Command, []string) error {
	plan, err := llmhost.Preview()
	if err != nil {
		return err
	}
	if !plan.Supported {
		return fmt.Errorf("no prebuilt runtime for this platform: %s", plan.Unsupported)
	}

	printPlan(plan)

	// Nothing left to fetch: re-running is just the smoke test, so none of the
	// space-and-memory gates below apply. Checking them anyway would refuse to
	// re-verify an existing install on a machine that has since filled up.
	alreadyThere := plan.RuntimePresent && plan.ModelPresent

	// Refuse rather than fail at 94%: a 2GB download that cannot land is
	// worse than never starting.
	const diskHeadroom = 13 // ×10, so 1.3
	if !alreadyThere && plan.FreeDisk > 0 && plan.FreeDisk*10 < plan.InstallBytes*diskHeadroom {
		return fmt.Errorf("not enough free disk: %s available, %s needed (including headroom)",
			humanBytes(plan.FreeDisk), humanBytes(plan.InstallBytes*diskHeadroom/10))
	}
	if !alreadyThere && plan.TotalRAM > 0 && plan.TotalRAM < 4<<30 {
		return fmt.Errorf("this machine has %s of memory; the model needs about %s to run",
			humanBytes(plan.TotalRAM), humanBytes(plan.RAMBytes))
	}
	if flagModelDryRun {
		return nil
	}
	tight := !alreadyThere && plan.TotalRAM > 0 && plan.TotalRAM < 8<<30
	if tight {
		fmt.Fprintf(os.Stderr, "\n  Warning: %s of memory total. Generating will use about %s,\n"+
			"  which on this machine is likely to swap.\n", humanBytes(plan.TotalRAM), humanBytes(plan.RAMBytes))
	}
	if !alreadyThere && !flagModelYes && !confirm("\n  Continue?") {
		fmt.Fprintln(os.Stderr, "  Nothing was downloaded.")
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	bar := newProgressBar()
	if err := llmhost.Install(ctx, bar.update); err != nil {
		bar.done()
		if ctx.Err() != nil {
			return fmt.Errorf("interrupted — rerun `hittable model enable` to resume")
		}
		return err
	}
	bar.done()

	// Files on disk is not the same as "this works". The smoke test is where
	// an unsigned binary, a missing dylib or a bad GGUF actually surfaces.
	fmt.Fprintln(os.Stderr, "  Checking it runs…")
	smokeCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	out, tps, err := llmhost.SmokeTest(smokeCtx)
	if err != nil {
		return fmt.Errorf("installed, but it did not run: %w", err)
	}
	fmt.Fprintf(os.Stderr, "  ok — %.0f tokens/sec · %q\n", tps, truncate(out, 48))

	cfg, _ := appconfig.Load("")
	cfg.AI.Enabled = true
	if err := appconfig.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Fprintln(os.Stderr, "\n  AI commit drafts are on. Press c in the Git panel.")
	fmt.Fprintln(os.Stderr, "  `hittable model disable` removes it and frees the space.")
	return nil
}

func runModelDisable(*cobra.Command, []string) error {
	running, ram, err := llmhost.Stop()
	if err != nil {
		return err
	}
	cfg, _ := appconfig.Load("")
	cfg.AI.Enabled = false
	if err := appconfig.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	if running {
		fmt.Printf("Stopped the model server — about %s of memory released.\n", humanBytes(ram))
	} else {
		fmt.Println("The model server was not running.")
	}
	st, _ := llmhost.Status()
	if st.ModelInstalled {
		fmt.Printf("The model is still on disk (%s). `hittable model enable` turns it back on\n"+
			"with nothing to download; `hittable model delete` reclaims the space.\n", humanBytes(st.TotalBytes))
	}
	fmt.Println("Commit drafting falls back to the staged diff.")
	return nil
}

func runModelDelete(*cobra.Command, []string) error {
	st, err := llmhost.Status()
	if err != nil {
		return err
	}
	if !st.RuntimeInstalled && !st.ModelInstalled {
		fmt.Println("Nothing installed.")
		return nil
	}
	if !flagModelYes {
		fmt.Printf("\n  This deletes %s from ~/.hittable.\n", humanBytes(st.TotalBytes))
		fmt.Println("  Enabling it again means downloading the model over the network.")
		fmt.Println("  To stop the server and free memory without that, use `hittable model disable`.")
		if !confirm("\n  Delete it?") {
			fmt.Println("  Nothing was deleted.")
			return nil
		}
	}
	freed, err := llmhost.Remove()
	if err != nil {
		return err
	}
	cfg, _ := appconfig.Load("")
	cfg.AI.Enabled = false
	if err := appconfig.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("Deleted the local model and runtime — %s freed.\n", humanBytes(freed))
	fmt.Println("Commit drafting falls back to the staged diff. Settings were kept.")
	return nil
}

func runModelStatus(*cobra.Command, []string) error {
	st, err := llmhost.Status()
	if err != nil {
		return err
	}
	cfg, _ := appconfig.Load("")

	row := func(k, v string) { fmt.Printf("  %-12s %s\n", k, v) }
	if st.RuntimeInstalled {
		row("runtime", fmt.Sprintf("llama.cpp %s · %s", st.RuntimeTag, humanBytes(st.RuntimeBytes)))
	} else {
		row("runtime", "not installed")
	}
	if st.ModelInstalled {
		row("model", fmt.Sprintf("%s · %s", st.ModelFile, humanBytes(st.ModelBytes)))
	} else {
		row("model", "not installed")
	}
	row("disk", humanBytes(st.TotalBytes)+" total")
	if st.ServerRunning {
		row("server", fmt.Sprintf("running · pid %d · port %d", st.ServerPID, st.ServerPort))
	} else {
		row("server", "not running (it starts on first use and stops when idle)")
	}
	switch {
	case st.Endpoint != "":
		row("endpoint", st.Endpoint+" (configured; nothing is downloaded or spawned)")
	case !cfg.AI.Enabled:
		row("ai", "off — `hittable model enable` turns it back on")
	case !llmhost.Installed():
		row("ai", "on, but nothing installed — run `hittable model enable`")
	default:
		row("ai", "on")
	}
	return nil
}

func printPlan(p llmhost.Plan) {
	fmt.Fprintln(os.Stderr, "\n  Download")
	if p.RuntimePresent {
		fmt.Fprintf(os.Stderr, "    %-42s %10s\n", p.RuntimeName, "present")
	} else {
		fmt.Fprintf(os.Stderr, "    %-42s %10s\n", p.RuntimeName, humanBytes(p.RuntimeBytes))
	}
	if p.ModelPresent {
		fmt.Fprintf(os.Stderr, "    %-42s %10s\n", p.ModelName, "present")
	} else {
		fmt.Fprintf(os.Stderr, "    %-42s %10s\n", p.ModelName, humanBytes(p.ModelBytes))
	}
	fmt.Fprintf(os.Stderr, "    %-42s %10s\n", "", strings.Repeat("─", 10))
	fmt.Fprintf(os.Stderr, "    %-42s %10s\n", "total", humanBytes(p.DownloadBytes))

	fmt.Fprintln(os.Stderr, "\n  Disk")
	if p.RuntimePresent && p.ModelPresent {
		// Already on disk, so nothing is about to be consumed. Saying "2.13 GB,
		// free drops to 12.35" here would be a plain lie about what happens next.
		fmt.Fprintf(os.Stderr, "    %-42s %10s\n", "already in ~/.hittable", humanBytes(p.InstallBytes))
		fmt.Fprintf(os.Stderr, "    %-42s %10s\n", "free now, and after", humanBytes(p.FreeDisk))
	} else {
		fmt.Fprintf(os.Stderr, "    %-42s %10s\n", "installs to ~/.hittable", humanBytes(p.InstallBytes))
		if p.FreeDisk > 0 {
			fmt.Fprintf(os.Stderr, "    %-42s %10s\n",
				"free now "+humanBytes(p.FreeDisk)+", after", humanBytes(p.FreeDisk-p.InstallBytes))
		}
	}
	fmt.Fprintln(os.Stderr, "\n  Memory")
	fmt.Fprintf(os.Stderr, "    about %s resident while generating\n", humanBytes(p.RAMBytes))
	if p.TotalRAM > 0 {
		fmt.Fprintf(os.Stderr, "    this machine has %s\n", humanBytes(p.TotalRAM))
	}
	fmt.Fprintln(os.Stderr, "    the server stops on its own after 10 idle minutes")
}

// confirm asks a yes/no question. A non-interactive stdin answers no, so a
// script that pipes into this never starts a 2GB download by accident.
func confirm(q string) bool {
	if !isTTY(os.Stdin) {
		return false
	}
	fmt.Fprintf(os.Stderr, "%s [y/N] ", q)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// progressBar redraws in place on a terminal and prints one line per 10% when
// piped. `model enable` is usually run in the app's own integrated terminal,
// which is a real PTY, so the in-place bar works there too.
type progressBar struct {
	tty      bool
	last     time.Time
	lastPct  int
	label    string
	started  time.Time
	anyPrint bool
}

func newProgressBar() *progressBar {
	return &progressBar{tty: isTTY(os.Stderr), lastPct: -1}
}

func (b *progressBar) update(label string, done, total int64) {
	if label != b.label {
		if b.anyPrint && b.tty {
			fmt.Fprintln(os.Stderr)
		}
		b.label, b.started, b.lastPct = label, time.Now(), -1
	}
	if total <= 0 {
		return
	}
	pct := int(done * 100 / total)
	if b.tty {
		if time.Since(b.last) < 100*time.Millisecond && done < total {
			return
		}
		b.last = time.Now()
		const w = 24
		filled := pct * w / 100
		bar := strings.Repeat("█", filled) + strings.Repeat("░", w-filled)
		fmt.Fprintf(os.Stderr, "\r  %-22s %9s / %-9s %3d%% ▕%s▏ %s",
			truncate(label, 22), humanBytes(done), humanBytes(total), pct, bar, b.rate(done))
		b.anyPrint = true
		return
	}
	if pct/10 != b.lastPct/10 || done == total {
		fmt.Fprintf(os.Stderr, "  %s %d%% (%s / %s)\n", label, pct, humanBytes(done), humanBytes(total))
		b.anyPrint = true
	}
	b.lastPct = pct
}

func (b *progressBar) rate(done int64) string {
	el := time.Since(b.started).Seconds()
	if el < 0.5 || done == 0 {
		return ""
	}
	bps := float64(done) / el
	return fmt.Sprintf("%s/s", humanBytes(int64(bps)))
}

func (b *progressBar) done() {
	if b.anyPrint && b.tty {
		fmt.Fprintln(os.Stderr)
	}
}

func humanBytes(n int64) string {
	switch {
	case n < 0:
		return "—"
	case n < 1000:
		return fmt.Sprintf("%d B", n)
	case n < 1000*1000:
		return fmt.Sprintf("%.0f KB", float64(n)/1000)
	case n < 1000*1000*1000:
		return fmt.Sprintf("%.0f MB", float64(n)/1e6)
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/1e9)
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func init() {
	modelEnableCmd.Flags().BoolVarP(&flagModelYes, "yes", "y", false, "skip the confirmation prompt")
	modelEnableCmd.Flags().BoolVar(&flagModelDryRun, "dry-run", false, "show what would be downloaded, then stop")
	modelCmd.AddCommand(modelEnableCmd, modelDisableCmd, modelDeleteCmd, modelStatusCmd)
	modelDeleteCmd.Flags().BoolVarP(&flagModelYes, "yes", "y", false, "skip the confirmation prompt")
}
