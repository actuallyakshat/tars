package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/actuallyakshat/tars/internal/store"
)

// fakeShell records commands. fails maps a command line prefix to an error;
// have lists the programs on PATH.
type fakeShell struct {
	ran   []string
	fails map[string]error
	have  map[string]bool
}

func (f *fakeShell) result(r run) error {
	line := r.String()
	f.ran = append(f.ran, line)
	for prefix, err := range f.fails {
		if strings.HasPrefix(line, prefix) {
			return err
		}
	}
	return nil
}
func (f *fakeShell) Run(r run) error              { return f.result(r) }
func (f *fakeShell) Output(r run) (string, error) { return "", f.result(r) }
func (f *fakeShell) Has(name string) bool         { return f.have[name] }

func runWith(t *testing.T, sh shell, args ...string) (string, error) {
	t.Helper()
	a := &app{}
	if sh != nil {
		a.sh = sh
	}
	root := newRoot(a)
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestUpdateUpgradesWhatsInstalled(t *testing.T) {
	sh := &fakeShell{have: map[string]bool{"brew": true, "claude": true}, fails: map[string]error{
		"brew list --cask tars-app": errors.New("not installed"),
	}}
	out, err := runWith(t, sh, "update")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	want := []string{"brew update", "brew list --cask tars", "brew upgrade --cask tars"}
	if runtime.GOOS == "darwin" { // the app is macOS only
		want = append(want, "brew list --cask tars-app")
	}
	want = append(want, "claude plugin marketplace update tars", "claude plugin update tars@tars")
	if strings.Join(sh.ran, "\n") != strings.Join(want, "\n") {
		t.Fatalf("ran:\n%s", strings.Join(sh.ran, "\n"))
	}
	if runtime.GOOS == "darwin" && !strings.Contains(out, "brew install --cask actuallyakshat/tap/tars-app") {
		t.Fatalf("should say how to install the app:\n%s", out)
	}
}

func TestUpdateKeepsGoingAndRunsWithoutTheDatabase(t *testing.T) {
	// A database from a newer tars must not stop the update that fixes it.
	dir := t.TempDir()
	t.Setenv("TARS_HOME", dir)
	st, err := store.Open(filepath.Join(dir, "tars.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	sh := &fakeShell{have: map[string]bool{"brew": true, "claude": true}, fails: map[string]error{
		"brew upgrade --cask tars": errors.New("network down"),
	}}
	out, err := runWith(t, sh, "update")
	if err == nil || !strings.Contains(err.Error(), "CLI (tars)") {
		t.Fatalf("want a failure naming the CLI step, got %v\n%s", err, out)
	}
	if !strings.Contains(strings.Join(sh.ran, "\n"), "claude plugin update tars@tars") {
		t.Fatal("stopped after the first failure")
	}
}

func TestDevDatabaseIsSeparateAndSeedCopiesTheRealOne(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("TARS_HOME", "")
	t.Setenv("TARS_DEV_HOME", "")
	real, _ := store.DefaultPathFor(false)
	dev, _ := store.DefaultPathFor(true)
	if real == dev || filepath.Base(filepath.Dir(dev)) != "tars-dev" {
		t.Fatalf("real %s, dev %s", real, dev)
	}
	// TARS_HOME never moves the dev database onto the real one.
	t.Setenv("TARS_HOME", filepath.Dir(real))
	if d, _ := store.DefaultPathFor(true); d != dev {
		t.Fatalf("TARS_HOME moved dev to %s", d)
	}

	st, err := store.Open(real)
	if err != nil {
		t.Fatal(err)
	}
	st.CreateTask(&store.Task{Title: "real work"})
	st.Close()

	if _, err := runWith(t, nil, "dev", "seed"); err != nil {
		t.Fatal(err)
	}
	st, _ = store.Open(dev)
	ts, _ := st.ListTasks(store.Filter{})
	st.CreateTask(&store.Task{Title: "dev only"})
	st.Close()
	if len(ts) != 1 || ts[0].Title != "real work" {
		t.Fatalf("seeded: %+v", ts)
	}
	// The dev database has tasks now: replacing them needs --yes.
	if _, err := runWith(t, nil, "dev", "seed"); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("want a --yes error, got %v", err)
	}
	if _, err := runWith(t, nil, "dev", "seed", "--yes"); err != nil {
		t.Fatal(err)
	}
	// The real database is untouched by any of it.
	st, _ = store.Open(real)
	defer st.Close()
	if ts, _ := st.ListTasks(store.Filter{}); len(ts) != 1 {
		t.Fatalf("real changed: %+v", ts)
	}
}

func TestToDevCommands(t *testing.T) {
	cases := map[string]string{
		"run `tars ls --json`":                           "run `tars-dev ls --json`",
		"allowed-tools: Bash(tars:*), Bash(git:*)":       "allowed-tools: Bash(tars-dev:*), Bash(git:*)",
		"command -v tars >/dev/null && tars hook prompt": "command -v tars-dev >/dev/null && tars-dev hook prompt",
		"If `tars` is not installed":                     "If `tars-dev` is not installed",
		"branch tars/12-x, /tars:tars, Tars, stars":      "branch tars/12-x, /tars:tars, Tars, stars",
		"tars-dev ls": "tars-dev ls",
	}
	for in, want := range cases {
		if got := toDevCommands(in); got != want {
			t.Errorf("%q:\n got %q\nwant %q", in, got, want)
		}
	}
}

func TestDevMarketplaceFromTheRealPlugin(t *testing.T) {
	plugin, _ := filepath.Abs("../../plugin")
	market := filepath.Join(t.TempDir(), "m")
	if err := writeDevMarketplace(plugin, market, "/src/tars"); err != nil {
		t.Fatal(err)
	}
	var m struct {
		Name    string `json:"name"`
		Plugins []struct{ Name, Source string }
	}
	b, _ := os.ReadFile(filepath.Join(market, ".claude-plugin", "marketplace.json"))
	if json.Unmarshal(b, &m) != nil || m.Name != "tars-dev" || m.Plugins[0].Name != "tars-dev" || m.Plugins[0].Source != "./plugin" {
		t.Fatalf("marketplace: %s", b)
	}
	b, _ = os.ReadFile(filepath.Join(market, "plugin", ".claude-plugin", "plugin.json"))
	if !strings.Contains(string(b), `"name": "tars-dev"`) {
		t.Fatalf("plugin.json: %s", b)
	}
	hooks, _ := os.ReadFile(filepath.Join(market, "plugin", "hooks", "hooks.json"))
	if !strings.Contains(string(hooks), "tars-dev hook prompt") || strings.Contains(string(hooks), " tars hook") {
		t.Fatalf("hooks: %s", hooks)
	}
	skill, _ := os.ReadFile(filepath.Join(market, "plugin", "skills", "tars", "SKILL.md"))
	if !strings.Contains(string(skill), "Bash(tars-dev:*)") || strings.Contains(string(skill), "`tars ls") || !strings.Contains(string(skill), "\nname: tars\n") {
		t.Fatalf("skill still calls tars")
	}
	if _, err := os.Stat(filepath.Join(market, "plugin", "skills", "update")); err == nil {
		t.Fatal("the dev plugin shouldn't carry /update")
	}
}

func TestDevUpdateBuildsFromTheRememberedCheckout(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_DATA_HOME", base)
	t.Setenv("TARS_DEV_HOME", "")
	t.Setenv("HOME", t.TempDir())
	src, _ := filepath.Abs("../..")

	sh := &fakeShell{have: map[string]bool{"go": true, "claude": true}}
	out, err := runWith(t, sh, "dev", "update", "--src", src)
	// No npm/cargo here, so on macOS the app step fails; the rest must run.
	ran := strings.Join(sh.ran, "\n")
	for _, want := range []string{
		"go build -ldflags -X github.com/actuallyakshat/tars/internal/cli.Version=dev",
		"internal/cli.Flavor=dev -o " + filepath.Join(os.Getenv("HOME"), ".local", "bin", "tars-dev") + " ./cmd/tars",
		"claude plugin marketplace add ",
		"claude plugin install tars-dev@tars-dev",
	} {
		if !strings.Contains(ran, want) {
			t.Errorf("missing %q in:\n%s\n%s %v", want, ran, out, err)
		}
	}
	// Next time it finds the checkout without --src, from anywhere.
	t.Chdir(t.TempDir())
	if got, err := sourceDir(""); err != nil || got != src {
		t.Fatalf("remembered %q, %v", got, err)
	}
	if _, err := sourceDir(t.TempDir()); err == nil {
		t.Fatal("a folder that isn't a checkout should be refused")
	}
}
