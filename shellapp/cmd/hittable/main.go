package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/spf13/cobra"

    "github.com/hittable/shellapp/internal/collection"
    "github.com/hittable/shellapp/internal/rootdir"
    "github.com/hittable/shellapp/internal/scaffold"
    "github.com/hittable/shellapp/ui"
    "github.com/hittable/shellapp/ui/components/explorer"
    "github.com/hittable/shellapp/ui/screens"
)

var (
    flagImport string
    flagExport string
    flagOut    string
    flagIcons  string
)

var rootCmd = &cobra.Command{
    Use:   "hittable [path]",
    Short: "Terminal API client for .hit request files",
    Long: `hittable.sh opens a project directory as an API workspace. Requests live in
plain .hit files (shared with the Hittable web app), variables in hittable/env.json.

  hittable                     open the current directory
  hittable ~/code/api          open another directory
  hittable init                create the hittable/ template (test request, env.json, notes)
  hittable -i collection.json  import a Postman v2.1 or Insomnia export, then open
  hittable -e postman          export hittable/ as a Postman collection (<folder>.postman_collection.json)
  hittable -e insomnia         export hittable/ as an Insomnia collection (<folder>.insomnia.json)
  hittable -e postman -o x.json  choose the output file
  hittable uninstall           remove hittable from this machine`,
    Args:          cobra.MaximumNArgs(1),
    SilenceUsage:  true,
    SilenceErrors: true,
    RunE: func(cmd *cobra.Command, args []string) error {
        if flagIcons == "emoji" {
            explorer.IconMode = "emoji"
        }
        // `hittable -e insomnia`: the format lands in args because -e takes an
        // optional value; treat a bare format word as the format, not a path.
        if flagExport != "" && len(args) == 1 {
            switch strings.ToLower(args[0]) {
            case "postman", "insomnia":
                flagExport, args = strings.ToLower(args[0]), nil
            }
        }
        root, err := rootdir.Resolve(args)
        if err != nil {
            return fmt.Errorf("resolving root: %w", err)
        }
        hittableDir := filepath.Join(root, "hittable")

        if flagExport != "" {
            return runExport(root, hittableDir)
        }
        if flagImport != "" {
            rep, err := collection.Import(flagImport, hittableDir)
            if err != nil {
                return fmt.Errorf("import: %w", err)
            }
            fmt.Println(rep)
            screens.InitialStatus = fmt.Sprintf("Imported %d requests from %s into %s", rep.Requests, rep.Format, filepath.Base(rep.Dir))
        }
        return runTUI(root)
    },
}

var initCmd = &cobra.Command{
    Use:   "init [path]",
    Short: "Create the hittable/ template (testcollection/test.hit, env.json, notes/)",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        root, err := rootdir.Resolve(args)
        if err != nil {
            return err
        }
        if _, err := os.Stat(filepath.Join(root, "hittable")); err == nil {
            fmt.Println("hittable/ already exists in", root)
            return nil
        }
        if err := scaffold.Ensure(root); err != nil {
            return err
        }
        fmt.Printf("created %s/hittable/ (testcollection/test.hit, env.json, notes/sample.md)\n", root)
        return nil
    },
}

var uninstallCmd = &cobra.Command{
    Use:   "uninstall",
    Short: "Remove the hittable binary from this machine",
    Args:  cobra.NoArgs,
    RunE: func(cmd *cobra.Command, args []string) error {
        if pids := runningInstances(); len(pids) > 0 {
            return fmt.Errorf("hittable is currently running (pid %s) — close it first, then run `hittable uninstall` again", strings.Join(pids, ", "))
        }
        var removed []string
        for _, p := range installedBinaries() {
            if err := os.Remove(p); err == nil {
                removed = append(removed, p)
            }
        }
        if len(removed) == 0 {
            fmt.Println("nothing to remove")
            return nil
        }
        fmt.Println("removed:")
        for _, p := range removed {
            fmt.Println("  " + p)
        }
        fmt.Println("hittable has been uninstalled. Your project files (hittable/ folders) were left untouched.")
        return nil
    },
}

func runTUI(root string) error {
    app := ui.NewApp(root)
    p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion(),
        tea.WithInput(ui.MouseSafeInput(os.Stdin)))
    screens.SetTeaProgram(p)
    if _, err := p.Run(); err != nil {
        return fmt.Errorf("running TUI: %w", err)
    }
    app.Cleanup()
    return nil
}

func runExport(root, hittableDir string) error {
    if _, err := os.Stat(hittableDir); err != nil {
        return fmt.Errorf("no hittable/ folder in %s (run `hittable init` or import a collection first)", root)
    }
    name := filepath.Base(root)
    var data []byte
    var n int
    var err error
    out := flagOut
    switch strings.ToLower(flagExport) {
    case "postman", "":
        data, n, err = collection.ExportPostman(hittableDir, name)
        if out == "" {
            out = filepath.Join(root, name+".postman_collection.json")
        }
    case "insomnia":
        data, n, err = collection.ExportInsomnia(hittableDir, name)
        if out == "" {
            out = filepath.Join(root, name+".insomnia.json")
        }
    default:
        return fmt.Errorf("unknown export format %q (use postman or insomnia)", flagExport)
    }
    if err != nil {
        return err
    }
    if err := os.WriteFile(out, data, 0o644); err != nil {
        return err
    }
    fmt.Printf("exported %d requests → %s\n", n, out)
    return nil
}

// runningInstances lists other hittable processes (not this one).
func runningInstances() []string {
    out, err := exec.Command("pgrep", "-x", "hittable").Output()
    if err != nil {
        return nil
    }
    me := strconv.Itoa(os.Getpid())
    var pids []string
    for _, l := range strings.Fields(string(out)) {
        if l != me {
            pids = append(pids, l)
        }
    }
    return pids
}

// installedBinaries finds this executable and every `hittable` on PATH.
func installedBinaries() []string {
    seen := map[string]bool{}
    var out []string
    add := func(p string) {
        if p == "" {
            return
        }
        if r, err := filepath.EvalSymlinks(p); err == nil {
            p = r
        }
        if fi, err := os.Stat(p); err != nil || fi.IsDir() || seen[p] {
            return
        }
        seen[p] = true
        out = append(out, p)
    }
    if exe, err := os.Executable(); err == nil {
        add(exe)
    }
    for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
        add(filepath.Join(dir, "hittable"))
    }
    if home, err := os.UserHomeDir(); err == nil {
        add(filepath.Join(home, ".local", "bin", "hittable"))
        add(filepath.Join(home, "go", "bin", "hittable"))
    }
    return out
}

func init() {
    rootCmd.Flags().StringVarP(&flagImport, "import", "i", "", "import a Postman v2.1 or Insomnia export file into hittable/, then open")
    rootCmd.Flags().StringVarP(&flagExport, "export", "e", "", "export hittable/ as a collection: postman (default) or insomnia")
    rootCmd.Flags().Lookup("export").NoOptDefVal = "postman"
    rootCmd.Flags().StringVarP(&flagOut, "out", "o", "", "output file for --export (default: <folder>.postman_collection.json)")
    rootCmd.Flags().StringVar(&flagIcons, "icons", "nerd", `file icon pack: "nerd" (Nerd Font glyphs) or "emoji"`)
    rootCmd.AddCommand(initCmd, uninstallCmd)
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, "error:", err)
        os.Exit(1)
    }
}
