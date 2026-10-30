package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/actuallyakshat/tars/internal/store"
)

type env struct {
	t  *testing.T
	db string
}

func newEnv(t *testing.T) *env {
	return &env{t: t, db: filepath.Join(t.TempDir(), "tars.db")}
}

// run executes one tars invocation in-process, like a fresh CLI call.
func (e *env) run(stdin string, args ...string) (string, error) {
	e.t.Helper()
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"--db", e.db}, args...))
	err := root.Execute()
	return out.String(), err
}

func (e *env) ok(args ...string) string {
	e.t.Helper()
	out, err := e.run("", args...)
	if err != nil {
		e.t.Fatalf("tars %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (e *env) fails(args ...string) error {
	e.t.Helper()
	_, err := e.run("", args...)
	if err == nil {
		e.t.Fatalf("tars %s: expected an error", strings.Join(args, " "))
	}
	return err
}

func decode[T any](t *testing.T, s string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("bad json: %v\n%s", err, s)
	}
	return v
}

func (e *env) task(id string) store.Task {
	e.t.Helper()
	return decode[store.Task](e.t, e.ok("show", id, "--json"))
}

func TestAddAndList(t *testing.T) {
	e := newEnv(t)
	out := e.ok("add", "Fix", "CSV", "export", "-j", "bill-881", "-r", "billing", "-t", "backend,perf", "-p", "high", "-d", "tomorrow")
	if !strings.Contains(out, "Added #1") {
		t.Fatalf("unexpected output: %q", out)
	}

	tk := e.task("1")
	tomorrow := time.Now().AddDate(0, 0, 1).Format(time.DateOnly)
	if tk.Title != "Fix CSV export" || tk.Jira != "BILL-881" || tk.Repo != "billing" ||
		tk.Priority != 1 || tk.Due != tomorrow || tk.Status != "inbox" ||
		strings.Join(tk.Tags, ",") != "backend,perf" {
		t.Fatalf("task: %+v", tk)
	}

	e.ok("add", "Plain task")
	all := decode[[]store.Task](t, e.ok("ls", "--json"))
	if len(all) != 2 || all[0].ID != 1 { // high priority sorts first
		t.Fatalf("ls: %+v", all)
	}
	if all[1].Priority != 2 {
		t.Fatalf("default priority should be normal (2), got %d", all[1].Priority)
	}

	human := e.ok("ls")
	for _, want := range []string{"INBOX", "Fix CSV export", "BILL-881", "#backend"} {
		if !strings.Contains(human, want) {
			t.Errorf("human ls missing %q:\n%s", want, human)
		}
	}
}

func TestEmptyListIsJSONArray(t *testing.T) {
	e := newEnv(t)
	if out := strings.TrimSpace(e.ok("ls", "--json")); out != "[]" {
		t.Fatalf("want [], got %q", out)
	}
}

func TestBareCommandPrintsListWhenNotATerminal(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "visible")
	if out := e.ok(); !strings.Contains(out, "visible") {
		t.Fatalf("bare tars should list tasks when piped: %q", out)
	}
}

func TestListFilters(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "with jira", "-j", "A-1", "-s", "ready")
	e.ok("add", "without jira", "-r", "web", "-t", "ui")
	e.ok("add", "finished", "-j", "A-2", "-s", "done")

	count := func(args ...string) int {
		return len(decode[[]store.Task](t, e.ok(append(args, "--json")...)))
	}
	if n := count("ls", "--jira"); n != 1 {
		t.Errorf("--jira: %d", n)
	}
	if n := count("ls", "--jira", "--all"); n != 2 {
		t.Errorf("--jira --all: %d", n)
	}
	if n := count("ls", "--no-jira"); n != 1 {
		t.Errorf("--no-jira: %d", n)
	}
	if n := count("ls", "-s", "ready,wip"); n != 1 {
		t.Errorf("-s ready,wip: %d", n)
	}
	if n := count("ls", "-r", "web"); n != 1 {
		t.Errorf("-r web: %d", n)
	}
	if n := count("ls", "-t", "ui"); n != 1 {
		t.Errorf("-t ui: %d", n)
	}
	if n := count("ls", "-q", "finish", "--all"); n != 1 {
		t.Errorf("-q: %d", n)
	}
	e.fails("ls", "--jira", "--no-jira")
	e.fails("ls", "-s", "someday")
}

func TestEdit(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "orig", "-t", "a,b")
	e.fails("edit", "1")           // nothing to change
	e.fails("edit", "1", "--json") // global flags don't count as changes
	e.fails("edit", "1", "--title", "  ")
	e.fails("edit", "1", "-p", "whenever")
	e.fails("edit", "1", "-d", "someday")

	e.ok("edit", "#1", "--title", "renamed", "--untag", "a", "-t", "c", "--branch", "tars/1", "--worktree", "/tmp/wt", "-s", "wip")
	tk := e.task("1")
	if tk.Title != "renamed" || strings.Join(tk.Tags, ",") != "b,c" || tk.Branch != "tars/1" ||
		tk.Worktree != "/tmp/wt" || tk.Status != "in-progress" {
		t.Fatalf("edit: %+v", tk)
	}
	if len(tk.Log) != 1 || tk.Log[0].Kind != "status" || tk.Log[0].Text != "inbox → in-progress" {
		t.Fatalf("status change not logged: %+v", tk.Log)
	}

	e.ok("edit", "1", "-d", "none")
	if tk := e.task("1"); tk.Due != "" {
		t.Fatalf("due not cleared: %q", tk.Due)
	}
}

func TestBodyAndAcceptanceFromStdin(t *testing.T) {
	e := newEnv(t)
	if _, err := e.run("line one\nline two\n", "add", "from stdin", "-b", "-"); err != nil {
		t.Fatal(err)
	}
	if tk := e.task("1"); tk.Body != "line one\nline two" {
		t.Fatalf("body: %q", tk.Body)
	}
	if _, err := e.run("- exports 1M rows\n", "edit", "1", "-a", "-"); err != nil {
		t.Fatal(err)
	}
	if tk := e.task("1"); tk.Acceptance != "- exports 1M rows" {
		t.Fatalf("acceptance: %q", tk.Acceptance)
	}
}

func TestMoveDoneAndBlock(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "one")
	e.ok("add", "two")

	moved := decode[[]store.Task](t, e.ok("mv", "1", "2", "review", "--by", "agent", "--json"))
	if len(moved) != 2 || moved[0].Status != "in-review" || moved[1].Status != "in-review" {
		t.Fatalf("mv: %+v", moved)
	}
	if tk := e.task("1"); tk.Log[0].Author != "agent" {
		t.Fatalf("author not recorded: %+v", tk.Log)
	}
	e.fails("mv", "1", "someday")
	e.fails("mv", "99", "ready")

	e.ok("block", "1", "--on", "Rahul", "--follow-up", "+2d", "--reason", "needs staging keys")
	tk := e.task("1")
	want := time.Now().AddDate(0, 0, 2).Format(time.DateOnly)
	if tk.Status != "blocked" || tk.WaitingOn != "Rahul" || tk.FollowUp != want {
		t.Fatalf("block: %+v", tk)
	}
	if last := tk.Log[len(tk.Log)-1]; last.Text != "Blocked: needs staging keys" {
		t.Fatalf("reason not logged: %+v", last)
	}

	e.ok("done", "1")
	tk = e.task("1")
	if tk.Status != "done" || tk.DoneAt == "" || tk.WaitingOn != "" || tk.FollowUp != "" {
		t.Fatalf("done should clear blocker fields: %+v", tk)
	}
	if n := len(decode[[]store.Task](t, e.ok("ls", "--json"))); n != 1 {
		t.Fatalf("done task still listed: %d open", n)
	}
}

func TestNote(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "x")
	e.ok("note", "1", "root", "cause", "found")
	if _, err := e.run("## Parked\nnext: write test\n", "note", "1", "-", "--by", "agent", "-k", "park"); err != nil {
		t.Fatal(err)
	}
	tk := e.task("1")
	if len(tk.Log) != 2 || tk.Log[0].Text != "root cause found" || tk.Log[0].Author != "user" {
		t.Fatalf("log: %+v", tk.Log)
	}
	if p := tk.Log[1]; p.Kind != "park" || p.Author != "agent" || !strings.Contains(p.Text, "next: write test") {
		t.Fatalf("park entry: %+v", p)
	}
	e.fails("note", "99", "orphan")
	if _, err := e.run("   \n", "note", "1", "-"); err == nil {
		t.Fatal("empty note accepted")
	}
}

func TestRemoveAndBadIDs(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "x")
	e.fails("show", "abc")
	e.fails("show", "0")
	e.fails("show", "-3")
	e.ok("rm", "#1")
	e.fails("show", "1")
	e.fails("rm", "1")
}

func TestRepoProfileAndContext(t *testing.T) {
	e := newEnv(t)
	repoDir := t.TempDir()
	e.ok("repo", "set", "billing", "--path", repoDir)
	e.ok("repo", "set", "billing", "-S", "TESTS", "-x", "make test")
	dod := filepath.Join(t.TempDir(), "dod.md")
	os.WriteFile(dod, []byte("PR open, CI green"), 0o644)
	e.ok("repo", "set", "billing", "-S", "dod", "-f", dod)
	e.fails("repo", "set", "billing", "-S", "db") // section without content

	r := decode[store.Repo](t, e.ok("repo", "show", "billing", "--json"))
	if r.Path != repoDir || len(r.Sections) != 2 || r.Sections[1].Name != "tests" {
		t.Fatalf("repo: %+v", r)
	}
	if rs := decode[[]store.Repo](t, e.ok("repo", "ls", "--json")); len(rs) != 1 {
		t.Fatalf("repo ls: %+v", rs)
	}

	e.ok("profile", "set", "style", "-x", "small PRs")
	e.fails("profile", "set", "style")
	e.ok("add", "fix it", "-r", "billing", "-j", "BILL-1", "-a", "exports work")
	e.ok("note", "1", "tried X, failed", "--by", "agent")

	md := e.ok("context", "1")
	for _, want := range []string{
		"CLAUDE.md / AGENTS.md > repo profile > personal profile",
		"### style", "small PRs",
		"## Repo profile: billing", "### tests", "make test", "### dod",
		"## Task #1: fix it", "BILL-1", "exports work", "tried X, failed",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("context missing %q", want)
		}
	}

	ctx := decode[agentContext](t, e.ok("context", "1", "--json"))
	if ctx.Repo == nil || ctx.Repo.Name != "billing" || ctx.Task == nil || len(ctx.Profile) != 1 {
		t.Fatalf("context json: %+v", ctx)
	}

	// A task whose repo has no profile still yields context, just without a repo.
	e.ok("add", "orphan", "-r", "nope")
	if ctx := decode[agentContext](t, e.ok("context", "2", "--json")); ctx.Repo != nil {
		t.Fatalf("unexpected repo: %+v", ctx.Repo)
	}

	e.ok("repo", "rm", "billing")
	e.fails("repo", "show", "billing")
}

func TestBriefJSON(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "check me", "-s", "review")
	e.ok("add", "due", "-d", "today")
	b := decode[store.Brief](t, e.ok("brief", "--json"))
	if len(b.InReview) != 1 || len(b.DueToday) != 1 || len(b.Inbox) != 1 {
		t.Fatalf("brief: %+v", b)
	}
	if out := e.ok("brief"); !strings.Contains(out, "Waiting for your review") {
		t.Fatalf("human brief: %s", out)
	}
}

func TestReportAnswerAndSessions(t *testing.T) {
	e := newEnv(t)
	// Pretend to run inside a Claude Code session with a transcript on disk.
	cfg := t.TempDir()
	proj := filepath.Join(cfg, "projects", "-work")
	os.MkdirAll(proj, 0o755)
	os.WriteFile(filepath.Join(proj, "sess-42.jsonl"), []byte(
		`{"type":"user","cwd":"/work"}`+"\n"+`{"type":"custom-title","customTitle":"webhook retry"}`+"\n"), 0o644)
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-42")

	e.ok("add", "Add retry to webhook sender")
	e.fails("report", "1")                                     // --done is required
	e.fails("report", "1", "--done", strings.Repeat("x", 241)) // over the cap
	e.fails("report", "9", "--done", "nope")                   // unknown task

	e.ok("report", "1", "--done", "Backoff, 5 tries", "--verified", "212 passed",
		"--needs-you", "Alert #oncall or #billing-alerts?", "--next", "Open PR")
	tk := e.task("1")
	if tk.Report == nil || tk.Report.SessionID != "sess-42" || !tk.NeedsYou {
		t.Fatalf("report: %+v", tk.Report)
	}
	if len(tk.Sessions) != 1 || tk.Sessions[0].Name != "webhook retry" || tk.Sessions[0].LaunchDir != "/work" {
		t.Fatalf("session should be linked with its name and folder: %+v", tk.Sessions)
	}

	if n := len(decode[[]store.Task](t, e.ok("ls", "--needs-you", "--json"))); n != 1 {
		t.Fatalf("ls --needs-you: %d", n)
	}
	human := e.ok("show", "1")
	for _, want := range []string{"Latest report", "Backoff, 5 tries", "needs you", "claude --resume sess-42", "webhook retry"} {
		if !strings.Contains(human, want) {
			t.Errorf("show missing %q:\n%s", want, human)
		}
	}
	if md := e.ok("context", "1"); !strings.Contains(md, "waiting on the user") || !strings.Contains(md, "claude --resume sess-42") {
		t.Fatalf("context should carry the report and session:\n%s", md)
	}

	e.ok("answer", "#1", "billing-alerts")
	if tk := e.task("1"); tk.NeedsYou || tk.Report.Answer != "billing-alerts" {
		t.Fatalf("answer: %+v", tk.Report)
	}
	e.fails("answer", "1", "again")
	if md := e.ok("context", "1"); !strings.Contains(md, "user answered:** billing-alerts") {
		t.Fatalf("context should show the answer:\n%s", md)
	}

	ss := decode[[]store.TaskSession](t, e.ok("sessions", "1", "--json"))
	if len(ss) != 1 || ss[0].ID != "sess-42" {
		t.Fatalf("sessions: %+v", ss)
	}

	// Any change made from inside a session links it, not only reports.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sess-43")
	e.ok("add", "Second task")
	e.ok("note", "1", "follow-up from another session")
	if tk := e.task("2"); len(tk.Sessions) != 1 || tk.Sessions[0].ID != "sess-43" {
		t.Fatalf("add should link the session: %+v", tk.Sessions)
	}
	if tk := e.task("1"); len(tk.Sessions) != 2 || tk.Sessions[0].ID != "sess-43" {
		t.Fatalf("note should link the session, newest first: %+v", tk.Sessions)
	}

	// Outside Claude Code nothing is linked.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	e.ok("add", "Third task")
	if tk := e.task("3"); len(tk.Sessions) != 0 {
		t.Fatalf("no session expected: %+v", tk.Sessions)
	}
}

func TestSeenClearsUnread(t *testing.T) {
	e := newEnv(t)
	e.ok("add", "one")
	e.ok("add", "two")
	e.ok("note", "1", "--by", "agent", "worked on it")
	e.ok("note", "2", "--by", "agent", "this too")
	if !e.task("1").Unread || !e.task("2").Unread {
		t.Fatal("agent notes should mark tasks unread")
	}
	out := decode[map[string][]int64](t, e.ok("seen", "1", "#2", "--json"))
	if len(out["seen"]) != 2 {
		t.Fatalf("seen output: %+v", out)
	}
	if e.task("1").Unread || e.task("2").Unread {
		t.Fatal("seen should clear unread")
	}
	e.fails("seen", "99")
	e.fails("seen")
}
