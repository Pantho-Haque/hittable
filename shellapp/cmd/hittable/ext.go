package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/hittable/shellapp/internal/extensions"
	"github.com/hittable/shellapp/internal/noderuntime"
	"github.com/hittable/shellapp/internal/rootdir"
)

// `hittable ext …` manages VS Code extensions from the shell. The same
// actions are in the app's command palette (alt+p).
var extCmd = &cobra.Command{
	Use:   "ext",
	Short: "Manage VS Code extensions (from Open VSX)",
	Long: `Extensions come from Open VSX (open-vsx.org), the open registry VSCodium and
Gitpod use; Microsoft's marketplace only allows Microsoft's own products.
Every download is checked against the registry's SHA-256 and signature.

Themes, snippets and language definitions work as soon as they are installed.
Extensions with code run in the Node.js on this machine; ` + "`hittable ext node install`" + `
installs one globally when there is none.
Code only runs in folders you trust (` + "`hittable ext trust`" + `).`,
}

var extListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed extensions",
	Args:  cobra.NoArgs,
	RunE: func(*cobra.Command, []string) error {
		all, err := extensions.List()
		if err != nil {
			return err
		}
		if len(all) == 0 {
			fmt.Println("no extensions installed (hittable ext search <words>)")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tVERSION\tSTATE\tKIND\tNAME")
		for _, e := range all {
			state := "enabled"
			if !e.Enabled {
				state = "disabled"
			}
			kind := "declarative"
			if e.Manifest.HasCode() {
				kind = "code"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", e.ID, e.Version, state, kind, e.Manifest.Label())
		}
		return w.Flush()
	},
}

var extSearchCmd = &cobra.Command{
	Use:   "search <words>",
	Short: "Search Open VSX",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		res, err := extensions.Search(context.Background(), strings.Join(args, " "), 25)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tINSTALLS\tNAME")
		for _, r := range res {
			mark := ""
			if r.Verified {
				mark = " ✓"
			}
			fmt.Fprintf(w, "%s%s\t%d\t%s\n", r.ID, mark, r.Downloads, r.DisplayName)
		}
		return w.Flush()
	},
}

var extInstallCmd = &cobra.Command{
	Use:   "install <publisher.name[@version] | file.vsix>...",
	Short: "Install extensions (with their dependencies)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		needNode := false
		for _, spec := range args {
			bar := newProgressBar()
			res, err := extensions.Install(ctx, spec, bar.update)
			bar.done()
			if err != nil {
				return fmt.Errorf("%s: %w", spec, err)
			}
			e := res.Installed
			sig := "signature verified"
			if !e.Signed {
				sig = "not signed"
				if e.Source == "vsix" {
					sig = "from a local file"
				}
			}
			fmt.Printf("installed %s %s (%s)\n", e.ID, e.Version, sig)
			for _, d := range res.Deps {
				fmt.Println("  with", d)
			}
			for _, w := range res.Warnings {
				fmt.Println("  note:", w)
			}
			if e.Manifest.HasCode() {
				needNode = true
			}
		}
		if needNode {
			if f, err := noderuntime.Find(); err == nil {
				fmt.Printf("\nIt runs code with Node.js %s (%s).\n", f.Version, f.Path)
			} else if plan, perr := noderuntime.PlanInstall(); perr == nil {
				fmt.Printf("\nThis extension runs code, which needs Node.js %d+ (%v).\nRun `hittable ext node install` to %s.\n",
					noderuntime.MinMajor, err, strings.ToLower(plan.Summary[:1])+plan.Summary[1:])
			} else {
				fmt.Printf("\nThis extension runs code, which needs Node.js %d+: %v\n", noderuntime.MinMajor, perr)
			}
		}
		if needNode {
			fmt.Println("Extension code runs only in folders you trust: `hittable ext trust <folder>`, or answer the prompt in the app.")
		}
		return nil
	},
}

var extUninstallCmd = &cobra.Command{
	Use:   "uninstall <publisher.name>...",
	Short: "Remove extensions",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		for _, id := range args {
			if err := extensions.Uninstall(id); err != nil {
				return err
			}
			fmt.Println("uninstalled", id)
		}
		return nil
	},
}

func enableCmd(on bool) *cobra.Command {
	verb := "disable"
	if on {
		verb = "enable"
	}
	return &cobra.Command{
		Use:   verb + " <publisher.name>...",
		Short: strings.ToUpper(verb[:1]) + verb[1:] + " installed extensions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			for _, id := range args {
				if err := extensions.SetEnabled(id, on); err != nil {
					return err
				}
				fmt.Println(verb+"d", id)
			}
			return nil
		},
	}
}

var extTrustCmd = &cobra.Command{
	Use:   "trust [folder]",
	Short: "Allow extension code to run in a folder (default: the current one)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		root, err := rootdir.Resolve(args)
		if err != nil {
			return err
		}
		if err := extensions.Trust(root); err != nil {
			return err
		}
		fmt.Println("trusted", root)
		return nil
	},
}

var extNodeCmd = &cobra.Command{
	Use:   "node [status | install | remove]",
	Short: "The Node.js extensions with code run in",
	Long: `Extensions with code run in the Node.js already on this machine: HITTABLE_NODE,
then node on PATH, then Homebrew, nvm, fnm, Volta, asdf, mise and the standard
install locations, newest first (Node 20 or newer).

"install" installs Node for your account when there is none: with Homebrew
when it is installed, with winget on Windows, otherwise the official LTS
build (SHA-256 checked) in ~/.local/lib/nodejs with node, npm and npx linked
into ~/.local/bin. It never uses sudo.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		action := "status"
		if len(args) == 1 {
			action = args[0]
		}
		switch action {
		case "status":
			f, err := noderuntime.Find()
			if err != nil {
				fmt.Println(err)
				if plan, perr := noderuntime.PlanInstall(); perr == nil {
					fmt.Println("`hittable ext node install` would:", plan.Summary)
				}
				return nil
			}
			fmt.Printf("Node.js %s (%s): %s\n", f.Version, f.Source, f.Path)
		case "install":
			if f, err := noderuntime.Find(); err == nil {
				fmt.Printf("Node.js %s is already installed (%s): %s\n", f.Version, f.Source, f.Path)
				return nil
			}
			plan, err := noderuntime.PlanInstall()
			if err != nil {
				return err
			}
			fmt.Println(plan.Summary)
			if plan.Command != "" {
				fmt.Println("  $", plan.Command)
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			bar := newProgressBar()
			f, err := noderuntime.InstallGlobal(ctx, func(line string, done, total int64) {
				if line != "" {
					bar.done()
					fmt.Println("  " + line)
					return
				}
				bar.update("Node.js "+noderuntime.Version, done, total)
			})
			bar.done()
			if err != nil {
				if ctx.Err() != nil {
					return fmt.Errorf("interrupted — rerun to resume the download")
				}
				return err
			}
			fmt.Printf("Node.js %s ready (%s): %s\n", f.Version, f.Source, f.Path)
		case "remove":
			removed, err := noderuntime.Remove()
			for _, r := range removed {
				fmt.Println("removed", r)
			}
			if err != nil {
				return err
			}
			if len(removed) == 0 {
				fmt.Println("nothing to remove: hittable did not install the Node.js on this machine (Homebrew: brew uninstall node)")
			}
		default:
			return fmt.Errorf("unknown action %q (status, install or remove)", action)
		}
		return nil
	},
}

func init() {
	extCmd.AddCommand(extListCmd, extSearchCmd, extInstallCmd, extUninstallCmd, enableCmd(true), enableCmd(false), extTrustCmd, extNodeCmd)
	rootCmd.AddCommand(extCmd)
}
