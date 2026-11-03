package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/actuallyakshat/tars/internal/store"
	"github.com/actuallyakshat/tars/internal/ui"
)

// noDB marks commands that must run without opening the database: updating
// is how you fix a tars too old for its database.
var noDB = map[string]string{"nodb": "1"}

// run describes one external command.
type run struct {
	Dir  string
	Env  []string // added to the environment
	Name string
	Args []string
}

func (r run) String() string { return strings.TrimSpace(r.Name + " " + strings.Join(r.Args, " ")) }

// shell runs external commands; tests swap in a fake.
type shell interface {
	// Run streams the command's output to the terminal.
	Run(r run) error
	// Output captures stdout.
	Output(r run) (string, error)
	// Has reports whether name is on PATH.
	Has(name string) bool
}

type osShell struct{ out, err io.Writer }

func (s osShell) cmd(r run) *exec.Cmd {
	c := exec.Command(r.Name, r.Args...)
	c.Dir = r.Dir
	if len(r.Env) > 0 {
		c.Env = append(os.Environ(), r.Env...)
	}
	return c
}

func (s osShell) Run(r run) error {
	c := s.cmd(r)
	// Homebrew and Claude Code may ask to confirm (trusting a tap or a
	// marketplace), so the user's terminal stays connected.
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, s.out, s.err
	return c.Run()
}

func (s osShell) Output(r run) (string, error) {
	var stderr bytes.Buffer
	c := s.cmd(r)
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil && stderr.Len() > 0 {
		err = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return string(out), err
}

func (s osShell) Has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (a *app) shell(cmd *cobra.Command) shell {
	if a.sh != nil {
		return a.sh
	}
	return osShell{out: cmd.OutOrStdout(), err: cmd.ErrOrStderr()}
}

// steps runs a list of named steps, keeps going past failures, and reports
// each at the end.
type steps struct {
	out    io.Writer
	failed []string
}

func (s *steps) do(name string, f func() error) {
	fmt.Fprintln(s.out, ui.Accent.Render("→ ")+ui.Bold.Render(name))
	if err := f(); err != nil {
		fmt.Fprintln(s.out, ui.Error(name+": "+err.Error()))
		s.failed = append(s.failed, name)
		return
	}
}

func (s *steps) note(msg string) { fmt.Fprintln(s.out, "  "+ui.Muted.Render(msg)) }

func (s *steps) err() error {
	if len(s.failed) == 0 {
		return nil
	}
	return fmt.Errorf("%d step(s) failed: %s", len(s.failed), strings.Join(s.failed, ", "))
}

func updateCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update the released tars: CLI, desktop app and Claude Code plugin",
		Long: `Update the released tars to the latest version: the CLI and the Tars app
through Homebrew (whichever are installed), and the Claude Code plugin.
Parts that aren't installed are skipped, with how to install them.

Dev builds (tars-dev, Tars Dev, the tars-dev plugin) are left alone; update
those with tars dev update.`,
		Args:        cobra.NoArgs,
		Annotations: noDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sh := a.shell(cmd)
			s := &steps{out: a.out}
			brew := sh.Has("brew")
			cask := func(name, install string) func() error {
				return func() error {
					if !brew {
						s.note("Homebrew isn't installed; download from https://github.com/actuallyakshat/tars/releases/latest")
						return nil
					}
					if _, err := sh.Output(run{Name: "brew", Args: []string{"list", "--cask", name}}); err != nil {
						s.note("Not installed with Homebrew. Install: brew install --cask " + install)
						return nil
					}
					return sh.Run(run{Name: "brew", Args: []string{"upgrade", "--cask", name}})
				}
			}
			if brew {
				s.do("Refresh Homebrew", func() error { return sh.Run(run{Name: "brew", Args: []string{"update"}}) })
			}
			s.do("CLI (tars)", cask("tars", "actuallyakshat/tap/tars"))
			if runtime.GOOS == "darwin" {
				s.do("App (Tars)", cask("tars-app", "actuallyakshat/tap/tars-app"))
			}
			s.do("Claude Code plugin (tars)", func() error {
				if !sh.Has("claude") {
					s.note("Claude Code isn't on your PATH; skipped.")
					return nil
				}
				if err := sh.Run(run{Name: "claude", Args: []string{"plugin", "marketplace", "update", "tars"}}); err != nil {
					s.note("Not installed? In Claude Code: /plugin marketplace add actuallyakshat/tars, then /plugin install tars@tars")
					return err
				}
				return sh.Run(run{Name: "claude", Args: []string{"plugin", "update", "tars@tars"}})
			})
			if err := s.err(); err != nil {
				return err
			}
			fmt.Fprintln(a.out, ui.Success("Up to date. Restart Claude Code to load a new plugin version."))
			return nil
		},
	}
}

// --- dev builds ---

const (
	devBin         = "tars-dev"
	devApp         = "Tars Dev"
	devIdentifier  = "com.actuallyakshat.tars.dev"
	devPluginName  = "tars-dev"
	modulePath     = "github.com/actuallyakshat/tars"
	sourceFileName = "source" // in the dev data folder: the checkout dev update builds from
)

// devDir is where the dev build keeps its database, source path and plugin.
func devDir() (string, error) {
	p, err := store.DefaultPathFor(true)
	return filepath.Dir(p), err
}

// isCheckout reports whether dir is a tars source checkout.
func isCheckout(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	first, _, _ := strings.Cut(string(b), "\n")
	_, err = os.Stat(filepath.Join(dir, "cmd", "tars"))
	return strings.TrimSpace(first) == "module "+modulePath && err == nil
}

// sourceDir finds the checkout to build: --src, else the repo the command
// runs in, else the one used last time. It's remembered for next time.
func sourceDir(flag string) (string, error) {
	dir, err := devDir()
	if err != nil {
		return "", err
	}
	saved := filepath.Join(dir, sourceFileName)
	var src string
	switch {
	case flag != "":
		if src, err = filepath.Abs(expandHome(flag)); err != nil {
			return "", err
		}
		if !isCheckout(src) {
			return "", fmt.Errorf("%s isn't a tars checkout (no go.mod for %s)", src, modulePath)
		}
	default:
		if root := git(mustGetwd(), "rev-parse", "--show-toplevel"); root != "" && isCheckout(root) {
			src = root
		} else if b, err := os.ReadFile(saved); err == nil && isCheckout(strings.TrimSpace(string(b))) {
			src = strings.TrimSpace(string(b))
		} else {
			return "", fmt.Errorf("run this inside your tars checkout once, or pass --src <dir>")
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return src, os.WriteFile(saved, []byte(src+"\n"), 0o644)
}

func expandHome(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return p
}

func binDir() (string, error) {
	home, err := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin"), err
}

func onPath(dir string) bool {
	for _, d := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(d) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

func devCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Build and install tars-dev from your checkout, next to the released tars",
		Long: `A dev build runs next to the released tars without touching it:

  tars-dev       the CLI, in ~/.local/bin, with its own database
  Tars Dev       the desktop app, in /Applications, using tars-dev
  tars-dev       a second Claude Code plugin whose hooks and skill call tars-dev

Both plugins capture each session, the released one into your real tasks and
the dev one into the dev database. Copy your real data in with tars dev seed.`,
		Annotations: noDB,
	}
	cmd.AddCommand(devUpdateCmd(a), devSeedCmd(a))
	return cmd
}

func devUpdateCmd(a *app) *cobra.Command {
	var src string
	var seed bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Rebuild tars-dev, Tars Dev and the tars-dev plugin from your checkout",
		Long: `Rebuild the dev CLI, app and plugin from your tars checkout and install
them. Run it inside the checkout the first time; after that it remembers
where the checkout is. The first time, start it with go run ./cmd/tars dev update
if your installed tars doesn't have it yet.`,
		Example: `  tars dev update
  tars dev update --seed          # also copy your real tasks into the dev database
  tars dev update --src ~/code/tars`,
		Args:        cobra.NoArgs,
		Annotations: noDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := sourceDir(src)
			if err != nil {
				return err
			}
			fmt.Fprintln(a.out, ui.Muted.Render("Building from "+dir))
			sh := a.shell(cmd)
			s := &steps{out: a.out}

			s.do("CLI (tars-dev)", func() error { return a.buildDevCLI(sh, s, dir) })
			if seed {
				s.do("Copy your real tasks into the dev database", func() error { return seedDev(true) })
			}
			s.do("Claude Code plugin (tars-dev)", func() error { return a.installDevPlugin(sh, s, dir) })
			if runtime.GOOS == "darwin" {
				s.do("App (Tars Dev)", func() error { return a.buildDevApp(sh, s, dir) })
			}
			if err := s.err(); err != nil {
				return err
			}
			fmt.Fprintln(a.out, ui.Success("Dev build installed. Restart Claude Code to load the tars-dev plugin."))
			return nil
		},
	}
	cmd.Flags().StringVar(&src, "src", "", "the tars checkout to build (default: this repo, or the last one used)")
	cmd.Flags().BoolVar(&seed, "seed", false, "replace the dev database with a copy of your real one")
	return cmd
}

func devVersion(sh shell, dir string) string {
	v := "dev"
	if sha, err := sh.Output(run{Dir: dir, Name: "git", Args: []string{"rev-parse", "--short", "HEAD"}}); err == nil && strings.TrimSpace(sha) != "" {
		v += "-" + strings.TrimSpace(sha)
		if st, err := sh.Output(run{Dir: dir, Name: "git", Args: []string{"status", "--porcelain"}}); err == nil && strings.TrimSpace(st) != "" {
			v += "-dirty"
		}
	}
	return v
}

func (a *app) buildDevCLI(sh shell, s *steps, dir string) error {
	if !sh.Has("go") {
		return errors.New("Go isn't installed; it's needed to build from source")
	}
	bin, err := binDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	out := filepath.Join(bin, devBin)
	ldflags := fmt.Sprintf("-X %[1]s/internal/cli.Version=%[2]s -X %[1]s/internal/cli.Flavor=dev", modulePath, devVersion(sh, dir))
	if err := sh.Run(run{Dir: dir, Name: "go", Args: []string{"build", "-ldflags", ldflags, "-o", out, "./cmd/tars"}}); err != nil {
		return err
	}
	s.note("Installed " + out)
	if !onPath(bin) {
		s.note(bin + ` isn't on your PATH. Add it: echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc`)
	}
	return nil
}

// installDevPlugin installs the checkout's plugin as a second plugin,
// tars-dev, whose hooks and skill call tars-dev.
func (a *app) installDevPlugin(sh shell, s *steps, dir string) error {
	if !sh.Has("claude") {
		s.note("Claude Code isn't on your PATH; skipped.")
		return nil
	}
	data, err := devDir()
	if err != nil {
		return err
	}
	market := filepath.Join(data, "marketplace")
	if err := writeDevMarketplace(filepath.Join(dir, "plugin"), market, dir); err != nil {
		return err
	}
	// Re-adding and reinstalling picks up changes even when the version in
	// plugin.json didn't move. The first run has nothing to remove.
	_, _ = sh.Output(run{Name: "claude", Args: []string{"plugin", "uninstall", devPluginName + "@" + devPluginName}})
	_, _ = sh.Output(run{Name: "claude", Args: []string{"plugin", "marketplace", "remove", devPluginName}})
	if err := sh.Run(run{Name: "claude", Args: []string{"plugin", "marketplace", "add", market}}); err != nil {
		return err
	}
	if err := sh.Run(run{Name: "claude", Args: []string{"plugin", "install", devPluginName + "@" + devPluginName}}); err != nil {
		return err
	}
	s.note("In Claude Code its command is /tars-dev:tars.")
	return nil
}

// writeDevMarketplace copies the plugin into market/plugin as tars-dev, with
// every tars command pointed at tars-dev, and writes a marketplace for it.
// /tars:update is left out: the dev build is updated with tars dev update.
func writeDevMarketplace(plugin, market, src string) error {
	if err := os.RemoveAll(market); err != nil {
		return err
	}
	dst := filepath.Join(market, "plugin")
	err := filepath.WalkDir(plugin, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(plugin, p)
		if d.IsDir() {
			if rel == filepath.Join("skills", "update") {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		switch {
		case rel == filepath.Join(".claude-plugin", "plugin.json"):
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			m["name"] = devPluginName
			m["description"] = "Dev build of tars from " + src + ". Calls tars-dev."
			b, _ = json.MarshalIndent(m, "", "  ")
		case strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".json"):
			// The skill keeps its name: in the tars-dev plugin it's /tars-dev:tars.
			b = []byte(strings.Replace(toDevCommands(string(b)), "\nname: tars-dev\n", "\nname: tars\n", 1))
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		return err
	}
	m := map[string]any{
		"name":  devPluginName,
		"owner": map[string]string{"name": "tars dev build"},
		"plugins": []map[string]string{{
			"name": devPluginName, "source": "./plugin", "description": "tars built from " + src,
		}},
	}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.MkdirAll(filepath.Join(market, ".claude-plugin"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(market, ".claude-plugin", "marketplace.json"), b, 0o644)
}

// toDevCommands rewrites the tars command to tars-dev wherever it's run:
// "tars ls", "`tars`", "Bash(tars:*)", "command -v tars". Branch names
// (tars/12-x), slash commands (/tars:tars), "Tars" and words like "stars"
// are left alone.
func toDevCommands(s string) string {
	var b strings.Builder
	isWord := func(c byte) bool {
		return c == '_' || c == '-' || c == '/' || c == '.' || c == ':' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	for i := 0; i < len(s); {
		if !strings.HasPrefix(s[i:], "tars") {
			b.WriteByte(s[i])
			i++
			continue
		}
		var prev, next byte = ' ', ' '
		if i > 0 {
			prev = s[i-1]
		}
		if i+4 < len(s) {
			next = s[i+4]
		}
		cmdEnd := strings.ContainsRune(" `*)\n\t\",'", rune(next)) || i+4 == len(s)
		if !isWord(prev) && (cmdEnd || (next == ':' && prev == '(')) {
			b.WriteString(devBin)
		} else {
			b.WriteString("tars")
		}
		i += 4
	}
	return b.String()
}

func (a *app) buildDevApp(sh shell, s *steps, dir string) error {
	desktop := filepath.Join(dir, "desktop")
	if !sh.Has("npm") || !sh.Has("cargo") {
		return errors.New("the app needs Node (npm) and Rust (cargo) to build")
	}
	if _, err := os.Stat(filepath.Join(desktop, "node_modules")); err != nil {
		if err := sh.Run(run{Dir: desktop, Name: "npm", Args: []string{"ci"}}); err != nil {
			return err
		}
	}
	conf, _ := json.Marshal(map[string]any{"productName": devApp, "mainBinaryName": devApp, "identifier": devIdentifier})
	s.note("This takes a few minutes.")
	// TARS_FLAVOR=dev makes the app look for tars-dev instead of tars.
	if err := sh.Run(run{Dir: desktop, Env: []string{"TARS_FLAVOR=dev"}, Name: "npx",
		Args: []string{"tauri", "build", "--bundles", "app", "--config", string(conf)}}); err != nil {
		return err
	}
	built := filepath.Join(desktop, "src-tauri", "target", "release", "bundle", "macos", devApp+".app")
	if _, err := os.Stat(built); err != nil {
		return fmt.Errorf("the build finished but %s isn't there", built)
	}
	dest := filepath.Join("/Applications", devApp+".app")
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := sh.Run(run{Name: "ditto", Args: []string{built, dest}}); err != nil {
		return err
	}
	s.note("Installed " + dest)
	return nil
}

// seedDev replaces the dev database with a copy of the real one.
func seedDev(force bool) error {
	real, err := store.DefaultPathFor(false)
	if err != nil {
		return err
	}
	dev, err := store.DefaultPathFor(true)
	if err != nil {
		return err
	}
	if real == dev {
		return errors.New("the dev and real databases are the same file; unset TARS_DEV_HOME")
	}
	if _, err := os.Stat(real); err != nil {
		return fmt.Errorf("no real database at %s to copy", real)
	}
	if !force {
		if st, err := store.Open(dev); err == nil {
			ts, _ := st.ListTasks(store.Filter{IncludeDone: true})
			st.Close()
			if len(ts) > 0 {
				return fmt.Errorf("the dev database has %d tasks; pass --yes to replace them", len(ts))
			}
		}
	}
	if err := store.CopyDatabase(real, dev); err != nil {
		return err
	}
	// Bring the copy up to this build's schema now, not on first use.
	st, err := store.Open(dev)
	if err != nil {
		return err
	}
	return st.Close()
}

func devSeedCmd(a *app) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Copy your real tasks into the dev database",
		Long: `Replace the dev database (used by tars-dev and Tars Dev) with a copy of your
real one. Your real database is only read, never changed or upgraded.`,
		Args:        cobra.NoArgs,
		Annotations: noDB,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := seedDev(yes); err != nil {
				return err
			}
			dev, _ := store.DefaultPathFor(true)
			fmt.Fprintln(a.out, ui.Success("Copied your real tasks into "+dev))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "replace a dev database that already has tasks")
	return cmd
}
