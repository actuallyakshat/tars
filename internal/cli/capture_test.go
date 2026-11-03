package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/actuallyakshat/tars/internal/store"
)

// gitRepo makes a repo with one commit on branch.
func gitRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "billing")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", dir},
		{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if branch != "main" {
		if out, err := exec.Command("git", "-C", dir, "checkout", "-q", "-b", branch).CombinedOutput(); err != nil {
			t.Fatalf("checkout: %v\n%s", err, out)
		}
	}
	return dir
}

// transcript writes a transcript with the given tool calls.
func transcript(t *testing.T, calls ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(`{"type":"user","message":{"content":"hi"}}` + "\n")
	for _, c := range calls {
		b.WriteString(`{"type":"assistant","message":{"content":[` + c + `]}}` + "\n")
	}
	p := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func hookJSON(session, cwd, prompt, transcriptPath string) string {
	b, _ := json.Marshal(hookInput{SessionID: session, Cwd: cwd, Prompt: prompt, TranscriptPath: transcriptPath})
	return string(b)
}

func (e *env) hook(event, stdin string) string {
	e.t.Helper()
	out, err := e.run(stdin, "hook", event)
	if err != nil {
		e.t.Fatalf("hook %s: %v\n%s", event, err, out)
	}
	return out
}

func newCaptureEnv(t *testing.T) *env {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	return newEnv(t)
}

const editCall = `{"type":"tool_use","name":"Edit","input":{"file_path":"/x/a.go"}}`

func TestCaptureLinksByBranchAndRecordsIt(t *testing.T) {
	e := newCaptureEnv(t)
	e.ok("add", "Retry webhooks", "-r", "billing")
	dir := gitRepo(t, "tars/1-retry")

	out := e.hook("prompt", hookJSON("s1", dir, "keep going", ""))
	if !strings.Contains(out, "linked to task #1") || !strings.Contains(out, "tars report 1") {
		t.Fatalf("context: %q", out)
	}
	// Only the first prompt does anything.
	if out := e.hook("prompt", hookJSON("s1", dir, "more", "")); out != "" {
		t.Fatalf("second prompt said %q", out)
	}
	tk := e.task("1")
	if tk.Branch != "tars/1-retry" || len(tk.Sessions) != 1 || tk.Sessions[0].ID != "s1" {
		t.Fatalf("task after link: %+v", tk)
	}

	// A second thread on a plain branch recorded on the task matches it too.
	other := gitRepo(t, "feat/webhooks")
	e.ok("edit", "1", "--branch", "feat/webhooks")
	if out := e.hook("prompt", hookJSON("s2", other, "next part", "")); !strings.Contains(out, "#1") {
		t.Fatalf("second thread: %q", out)
	}
	if tk := e.task("1"); len(tk.Sessions) != 2 {
		t.Fatalf("sessions: %+v", tk.Sessions)
	}
}

func TestCaptureByJiraAndNeverOnTrunk(t *testing.T) {
	e := newCaptureEnv(t)
	e.ok("add", "CSV export", "-j", "BILL-881", "-r", "billing")
	e.ok("edit", "1", "--branch", "main")
	dir := gitRepo(t, "main")

	// "main" is on the task but trunk branches never match.
	if out := e.hook("prompt", hookJSON("s1", dir, "hello", "")); out != "" {
		t.Fatalf("matched on trunk: %q", out)
	}
	if out := e.hook("prompt", hookJSON("s2", dir, "look at BILL-881 please", "")); !strings.Contains(out, "Jira BILL-881") {
		t.Fatalf("jira: %q", out)
	}
}

func TestCaptureAsksWhenOnlyWordsMatch(t *testing.T) {
	e := newCaptureEnv(t)
	e.ok("add", "Retry failed webhooks with backoff", "-r", "billing")
	e.ok("add", "Dark mode for settings", "-r", "billing")
	e.ok("add", "Retry webhooks elsewhere", "-r", "other")
	dir := gitRepo(t, "main")

	out := e.hook("prompt", hookJSON("s1", dir, "the webhooks retry loop is broken", ""))
	if !strings.Contains(out, "#1 Retry failed webhooks") || strings.Contains(out, "#2") || strings.Contains(out, "#3") {
		t.Fatalf("candidates: %q", out)
	}
	if !strings.Contains(out, "tars link <id>") {
		t.Fatalf("no instruction: %q", out)
	}
	// Commands from the plugin link themselves: no question.
	if out := e.hook("prompt", hookJSON("s2", dir, "/tars:tars the webhooks retry", "")); out != "" {
		t.Fatalf("asked in a tars command: %q", out)
	}

	// The user picks #1; Claude runs tars link from inside the session.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s1")
	e.ok("link", "1")
	if c := capture(t, e, "s1"); c.State != store.CaptureLinked || c.TaskID != 1 {
		t.Fatalf("capture: %+v", c)
	}
}

func capture(t *testing.T, e *env, id string) *store.Capture {
	t.Helper()
	st, err := store.Open(e.db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c, err := st.GetCapture(id)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestUnfiledFileAndDismiss(t *testing.T) {
	e := newCaptureEnv(t)
	dir := gitRepo(t, "main")
	worked := transcript(t, editCall, `{"type":"tool_use","name":"Bash","input":{"command":"git commit -m x"}}`)
	idle := transcript(t, `{"type":"tool_use","name":"Read","input":{"file_path":"/x/a.go"}}`)

	e.hook("prompt", hookJSON("aaaa1111", dir, "Make exports faster\nmore detail", worked))
	e.hook("prompt", hookJSON("bbbb2222", dir, "what does this do?", idle))
	e.hook("prompt", hookJSON("cccc3333", dir, "tweak a thing", worked))
	for _, s := range []string{"aaaa1111", "bbbb2222", "cccc3333"} {
		e.hook("end", hookJSON(s, dir, "", ""))
	}
	if c := capture(t, e, "bbbb2222"); c.State != store.CaptureDropped {
		t.Fatalf("idle session: %+v", c)
	}
	list := decode[[]store.Capture](t, e.ok("unfiled", "--json"))
	if len(list) != 2 || list[0].Edits != 1 || list[0].Commits != 1 || list[0].Repo != "billing" {
		t.Fatalf("unfiled: %+v", list)
	}

	e.ok("file", "aaaa", "--new")
	tk := e.task("1")
	if tk.Title != "Make exports faster" || tk.Repo != "billing" || len(tk.Sessions) != 1 {
		t.Fatalf("new task: %+v", tk)
	}
	e.ok("dismiss", "cccc")
	if list := decode[[]store.Capture](t, e.ok("unfiled", "--json")); len(list) != 0 {
		t.Fatalf("still unfiled: %+v", list)
	}
	e.fails("file", "zzzz", "1")
}

func TestCaptureSwitches(t *testing.T) {
	e := newCaptureEnv(t)
	dir := gitRepo(t, "main")
	e.ok("repo", "set", "billing", "--path", dir)
	e.ok("capture", "off", "--repo", "billing")
	e.hook("prompt", hookJSON("s1", dir, "x", ""))
	if c := capture(t, e, "s1"); c.State != store.CaptureSkipped {
		t.Fatalf("repo switch: %+v", c)
	}
	e.ok("capture", "on", "--repo", "billing")
	e.ok("capture", "off")
	e.hook("prompt", hookJSON("s2", dir, "x", ""))
	st, _ := store.Open(e.db)
	defer st.Close()
	if _, err := st.GetCapture("s2"); err == nil {
		t.Fatal("captured with capture off")
	}
	// Outside git, and bad input, are silent.
	e.ok("capture", "on")
	e.hook("prompt", hookJSON("s3", t.TempDir(), "x", ""))
	e.hook("prompt", "not json")
	e.hook("end", "{}")
}

func TestMerge(t *testing.T) {
	e := newCaptureEnv(t)
	e.ok("add", "Webhooks", "-t", "a")
	e.ok("add", "Webhook retries", "-j", "BILL-1", "-t", "b")
	dir := gitRepo(t, "tars/2-x")
	e.hook("prompt", hookJSON("s1", dir, "x", ""))
	e.ok("note", "2", "from two")
	e.ok("merge", "2", "1")
	tk := e.task("1")
	if tk.Jira != "BILL-1" || strings.Join(tk.Tags, ",") != "a,b" || len(tk.Sessions) != 1 || tk.Branch != "tars/2-x" {
		t.Fatalf("merged: %+v", tk)
	}
	e.fails("show", "2")
	e.fails("merge", "1", "1")
}

// Edits made through Bash (or by subagents) aren't named in the transcript;
// git shows them.
func TestSettleSeesShellEditsThroughGit(t *testing.T) {
	e := newCaptureEnv(t)
	dir := gitRepo(t, "main")
	shell := transcript(t, `{"type":"tool_use","name":"Bash","input":{"command":"sed -i '' s/a/b/ x.go"}}`)
	e.hook("prompt", hookJSON("s1", dir, "rename things", shell))
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.hook("end", hookJSON("s1", dir, "", shell))
	if c := capture(t, e, "s1"); c.State != store.CaptureUnfiled || c.Edits != 1 {
		t.Fatalf("shell edits: %+v", c)
	}

	// A session that only read files doesn't pick up the user's own edits.
	idle := transcript(t, `{"type":"tool_use","name":"Read","input":{"file_path":"/x"}}`)
	e.hook("prompt", hookJSON("s2", dir, "what is this", idle))
	os.WriteFile(filepath.Join(dir, "y.go"), []byte("package x\n"), 0o644)
	e.hook("end", hookJSON("s2", dir, "", idle))
	if c := capture(t, e, "s2"); c.State != store.CaptureDropped {
		t.Fatalf("read-only session: %+v", c)
	}
}
