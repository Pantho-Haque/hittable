package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hittable/shellapp/internal/selfupdate"
)

var (
	flagUpdateYes   bool
	flagUpdateCheck bool
	flagUpdateTag   string
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hittable to the latest release",
	Long: `Downloads the latest release build for this machine and replaces this binary.

The download is verified against the release's published SHA-256 before it is
installed; a mismatch aborts and changes nothing. Your projects and anything
under ~/.hittable are untouched — a model you have enabled stays enabled.`,
	Args:          cobra.NoArgs,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runUpdate,
}

func runUpdate(*cobra.Command, []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := selfupdate.Config{Current: version}

	target := flagUpdateTag
	if target == "" {
		fmt.Fprintln(os.Stderr, "  checking for a newer version…")
		latest, err := selfupdate.Latest(ctx, cfg)
		if err != nil {
			return err
		}
		target = latest
	}

	switch {
	case version == "dev":
		// A source build has no release it came from; replacing it silently
		// would throw away whatever the developer built.
		fmt.Fprintf(os.Stderr, "\n  This is a source build (version %q), not a release.\n", version)
		fmt.Fprintf(os.Stderr, "  Updating replaces it with %s.\n", target)
		if !flagUpdateYes && !confirm("\n  Continue?") {
			fmt.Fprintln(os.Stderr, "  Nothing was changed.")
			return nil
		}
	case target == version:
		fmt.Printf("hittable %s is already the latest release.\n", version)
		return nil
	}

	if flagUpdateCheck {
		fmt.Printf("hittable %s is available (you have %s).\nRun `hittable update` to install it.\n", target, version)
		return nil
	}

	if pids := runningInstances(); len(pids) > 0 {
		fmt.Fprintf(os.Stderr, "  note: hittable is running elsewhere (pid %v); it keeps the old binary until it exits\n", pids)
	}

	fmt.Fprintf(os.Stderr, "\n  %s → %s  ·  %s/%s\n\n", version, target, runtime.GOOS, runtime.GOARCH)

	bar := newProgressBar()
	path, err := selfupdate.Apply(ctx, cfg, target, func(done, total int64) {
		bar.update(selfupdate.AssetName(target, runtime.GOOS, runtime.GOARCH), done, total)
	})
	bar.done()
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return fmt.Errorf("interrupted — nothing was changed")
		}
		return err
	}

	fmt.Fprintf(os.Stderr, "  updated %s\n", path)
	if out, err := runSelf(path, "version"); err == nil {
		fmt.Fprintf(os.Stderr, "  %s\n", out)
	} else {
		return fmt.Errorf("installed, but the new binary did not run: %w", err)
	}
	return nil
}

// runSelf executes the freshly installed binary to prove it works on this
// machine. "The bytes are on disk" is not the same as "it runs" — an unsigned
// Mach-O on Apple silicon is the case that fails here.
func runSelf(path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := execCommandContext(ctx, path, args...)
	return out, err
}

func init() {
	updateCmd.Flags().BoolVarP(&flagUpdateYes, "yes", "y", false, "skip the confirmation prompt")
	updateCmd.Flags().BoolVar(&flagUpdateCheck, "check", false, "report whether an update exists, install nothing")
	updateCmd.Flags().StringVar(&flagUpdateTag, "tag", "", "install this exact release tag instead of the latest")
}

// execCommandContext runs a command and returns its trimmed combined output.
func execCommandContext(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
