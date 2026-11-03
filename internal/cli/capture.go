package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/actuallyakshat/tars/internal/claude"
	"github.com/actuallyakshat/tars/internal/store"
	"github.com/actuallyakshat/tars/internal/ui"
)

// staleAfter is how long an open capture can go without transcript activity
// before tars assumes its session ended without telling it (a crash or a
// killed terminal) and settles it the next time someone lists unfiled work.
const staleAfter = 2 * time.Hour

// gitState is where a session is running.
type gitState struct {
	Root     string // the main checkout, shared by every worktree
	Worktree string // the checkout the session is in
	Branch   string // "" when detached
}

func git(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// readGit reports false outside a git repo.
func readGit(dir string) (gitState, bool) {
	var g gitState
	if dir == "" {
		return g, false
	}
	if g.Worktree = git(dir, "rev-parse", "--show-toplevel"); g.Worktree == "" {
		return g, false
	}
	g.Root = g.Worktree
	if common := git(dir, "rev-parse", "--path-format=absolute", "--git-common-dir"); filepath.Base(common) == ".git" {
		g.Root = filepath.Dir(common)
	}
	g.Branch = git(dir, "symbolic-ref", "--short", "-q", "HEAD")
	return g, true
}

// treeState is the commit checked out and a hash of uncommitted changes, so
// settle can see work the transcript doesn't name (edits through Bash, or by
// subagents).
func treeState(dir string) (head, tree string) {
	head = git(dir, "rev-parse", "-q", "--verify", "HEAD")
	sum := sha256.Sum256([]byte(git(dir, "status", "--porcelain")))
	return head, hex.EncodeToString(sum[:8])
}

// trunk branches are shared by unrelated work, so they never identify a task.
func trunk(branch string) bool {
	switch branch {
	case "", "HEAD", "main", "master", "develop", "dev", "trunk":
		return true
	}
	return false
}

// repoFor names the repo a checkout belongs to: the tars repo profile whose
// path covers it, otherwise the main checkout's folder name.
func (a *app) repoFor(g gitState) (name string, off bool) {
	for _, dir := range []string{g.Worktree, g.Root} {
		if r, err := a.st.FindRepoByPath(dir); err == nil {
			return r.Name, r.CaptureOff
		}
	}
	return filepath.Base(g.Root), false
}

// match is the task a session belongs to, or the tasks it might.
type match struct {
	Task       *store.Task
	Why        string
	Candidates []*store.Task
}

var (
	tarsBranch = regexp.MustCompile(`^tars/(\d+)(?:-|$)`)
	jiraKey    = regexp.MustCompile(`\b[A-Z][A-Z0-9]+-\d+\b`)
	wordRe     = regexp.MustCompile(`[a-z0-9]+`)
)

// findTask works out which task a session is for. Branch, worktree, PR and
// Jira matches are certain; matching the prompt's words against task titles
// only produces candidates for the user to confirm.
func (a *app) findTask(g gitState, repo, prompt string) (match, error) {
	if m := tarsBranch.FindStringSubmatch(g.Branch); m != nil {
		id, _ := strconv.ParseInt(m[1], 10, 64)
		if t, err := a.st.GetTask(id); err == nil {
			return match{Task: t, Why: "branch " + g.Branch}, nil
		}
	}
	open, err := a.st.ListTasks(store.Filter{})
	if err != nil {
		return match{}, err
	}
	sameRepo := func(t *store.Task) bool { return t.Repo == "" || t.Repo == repo }
	certain := func(why string, keep func(*store.Task) bool) (match, bool) {
		var hits []*store.Task
		for _, t := range open {
			if keep(t) {
				hits = append(hits, t)
			}
		}
		switch len(hits) {
		case 0:
			return match{}, false
		case 1:
			return match{Task: hits[0], Why: why}, true
		}
		if len(hits) > 3 {
			hits = hits[:3]
		}
		return match{Candidates: hits}, true
	}
	if !trunk(g.Branch) {
		if m, ok := certain("branch "+g.Branch, func(t *store.Task) bool { return t.Branch == g.Branch && sameRepo(t) }); ok {
			return m, nil
		}
	}
	if g.Worktree != g.Root {
		if m, ok := certain("worktree "+g.Worktree, func(t *store.Task) bool { return t.Worktree == g.Worktree }); ok {
			return m, nil
		}
	}
	if m, ok := certain("its pull request", func(t *store.Task) bool { return t.PR != "" && strings.Contains(prompt, t.PR) }); ok {
		return m, nil
	}
	for _, key := range append(jiraKey.FindAllString(strings.ToUpper(g.Branch), -1), jiraKey.FindAllString(prompt, -1)...) {
		if m, ok := certain("Jira "+key, func(t *store.Task) bool { return strings.EqualFold(t.Jira, key) }); ok {
			return m, nil
		}
	}
	return match{Candidates: similar(open, repo, prompt)}, nil
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "this": true, "that": true, "from": true,
	"into": true, "fix": true, "add": true, "make": true, "can": true, "you": true, "please": true,
	"should": true, "when": true, "what": true, "are": true, "not": true, "its": true, "our": true,
	"let": true, "lets": true, "also": true, "now": true, "use": true, "need": true, "want": true,
}

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range wordRe.FindAllString(strings.ToLower(s), -1) {
		if len(w) >= 3 && !stopWords[w] {
			out[w] = true
		}
	}
	return out
}

// similar returns up to three open tasks in the repo whose titles share
// enough words with the prompt.
func similar(open []*store.Task, repo, prompt string) []*store.Task {
	p := words(prompt)
	type scored struct {
		t *store.Task
		n int
	}
	var hits []scored
	for _, t := range open {
		if t.Repo != repo && t.Repo != "" {
			continue
		}
		title := words(t.Title + " " + t.Jira)
		n := 0
		for w := range title {
			if p[w] {
				n++
			}
		}
		if n >= 2 || (n >= 1 && n == len(title)) {
			hits = append(hits, scored{t, n})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].n > hits[j].n })
	var out []*store.Task
	for i := 0; i < len(hits) && i < 3; i++ {
		out = append(out, hits[i].t)
	}
	return out
}

// hookInput is what Claude Code sends a hook on stdin.
type hookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Prompt         string `json:"prompt"`
}

func hookCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "hook",
		Short:  "Entry points for the Claude Code plugin's hooks",
		Hidden: true,
	}
	read := func(cmd *cobra.Command) (hookInput, bool) {
		var in hookInput
		b, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
		if err != nil || json.Unmarshal(b, &in) != nil || in.SessionID == "" {
			return in, false
		}
		return in, true
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "prompt",
		Short: "UserPromptSubmit: on a session's first prompt, find the task it's for",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in, ok := read(cmd)
			if !ok {
				return nil
			}
			msg, err := a.capturePrompt(in)
			if err != nil || msg == "" {
				return err
			}
			_, err = fmt.Fprintln(a.out, msg)
			return err
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "end",
		Short: "SessionEnd: record what the session changed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			in, ok := read(cmd)
			if !ok {
				return nil
			}
			c, err := a.st.GetCapture(in.SessionID)
			if errors.Is(err, store.ErrNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			if in.TranscriptPath != "" {
				c.Transcript = in.TranscriptPath
			}
			return a.settle(c)
		},
	})
	return cmd
}

// capturePrompt runs once per session. It returns context for Claude: which
// task the session is linked to, or which tasks to ask the user about.
func (a *app) capturePrompt(in hookInput) (string, error) {
	if _, err := a.st.GetCapture(in.SessionID); err == nil {
		return "", nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", err
	}
	if on, err := a.st.CaptureEnabled(); err != nil || !on {
		return "", err
	}
	c := &store.Capture{SessionID: in.SessionID, Transcript: in.TranscriptPath, Prompt: firstLines(in.Prompt, 500)}
	g, ok := readGit(in.Cwd)
	if !ok {
		c.State = store.CaptureSkipped
		_, err := a.st.StartCapture(c)
		return "", err
	}
	repo, off := a.repoFor(g)
	c.Repo, c.Root, c.Worktree, c.Branch = repo, g.Root, g.Worktree, g.Branch
	c.Head, c.Tree = treeState(g.Worktree)
	if off {
		c.State = store.CaptureSkipped
		_, err := a.st.StartCapture(c)
		return "", err
	}
	if fresh, err := a.st.StartCapture(c); err != nil || !fresh {
		return "", err
	}

	m, err := a.findTask(g, repo, in.Prompt)
	if err != nil {
		return "", err
	}
	if m.Task != nil {
		if err := a.fileSession(c, m.Task); err != nil {
			return "", err
		}
		return fmt.Sprintf(`tars: this session is linked to task #%d "%s" (matched by %s).
Run `+"`tars context %d`"+` for its preferences and history before changing code, if you haven't.
When you finish a piece of work, record it in a line or two: `+"`tars report %d --done \"…\"`"+` (add --needs-you "…" if you need an answer).`,
			m.Task.ID, m.Task.Title, m.Why, m.Task.ID, m.Task.ID), nil
	}
	// Sessions started by a tars command link themselves; don't ask in them.
	if len(m.Candidates) == 0 || strings.HasPrefix(strings.TrimSpace(in.Prompt), "/tars:") {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("tars: this session might be work on one of these tasks:\n")
	for _, t := range m.Candidates {
		fmt.Fprintf(&b, "  #%d %s [%s]\n", t.ID, t.Title, t.Status)
	}
	b.WriteString(`Before starting, ask the user once, in one short line, which task this is (or none).
If they pick one, run ` + "`tars link <id>`" + `. If they say none, carry on and don't ask again.
If you can't ask (a non-interactive run), skip this.`)
	return b.String(), nil
}

// fileSession ties a captured session to a task and records the branch and
// worktree on the task when it has none, so the session's other threads
// match it without asking.
func (a *app) fileSession(c *store.Capture, t *store.Task) error {
	s := claude.Inspect(claude.ConfigDir(), c.SessionID)
	if s.LaunchDir == "" {
		s.LaunchDir = c.Worktree
	}
	if err := a.st.LinkSession(t.ID, store.Session{ID: c.SessionID, Name: s.Name, LaunchDir: s.LaunchDir}); err != nil {
		return err
	}
	changed := false
	if t.Branch == "" && !trunk(c.Branch) {
		t.Branch, changed = c.Branch, true
	}
	if t.Worktree == "" && c.Worktree != "" && c.Worktree != c.Root {
		t.Worktree, changed = c.Worktree, true
	}
	if t.Repo == "" && c.Repo != "" {
		t.Repo, changed = c.Repo, true
	}
	if changed {
		return a.st.SaveTask(t)
	}
	return nil
}

// settle records what a session changed: an unlinked session that did work
// becomes unfiled, one that didn't is dropped.
func (a *app) settle(c *store.Capture) error {
	w := claude.ReadWork(c.Transcript)
	// The transcript can't see edits made through Bash or subagents; git can.
	// Only trust git when the session used such a tool, so a question-only
	// session doesn't pick up the user's own edits from their editor.
	if w.Shell && c.Worktree != "" {
		if _, err := os.Stat(c.Worktree); err == nil {
			head, tree := treeState(c.Worktree)
			if c.Head != "" && head != c.Head {
				if n, err := strconv.Atoi(git(c.Worktree, "rev-list", "--count", c.Head+".."+head)); err == nil && n > w.Commits {
					w.Commits = n
				}
			}
			if c.Tree != "" && tree != c.Tree && w.Edits == 0 {
				w.Edits = 1
			}
		}
	}
	files := make([]string, 0, len(w.Files))
	for _, f := range w.Files {
		if rel, err := filepath.Rel(c.Worktree, f); err == nil && c.Worktree != "" && !strings.HasPrefix(rel, "..") {
			f = rel
		}
		files = append(files, strings.ReplaceAll(f, ",", " "))
		if len(files) == 20 {
			break
		}
	}
	if s := claude.Inspect(claude.ConfigDir(), c.SessionID); s.Name != "" {
		if err := a.st.RecordSession(store.Session{ID: s.ID, Name: s.Name, LaunchDir: s.LaunchDir}); err != nil {
			return err
		}
	}
	return a.st.EndCapture(c.SessionID, w.Edits, w.Commits, files)
}

// unfiled settles sessions that went quiet without ending, then lists the
// sessions waiting for the user to file.
func (a *app) unfiled() ([]*store.Capture, error) {
	open, err := a.st.Captures(store.CaptureOpen)
	if err != nil {
		return nil, err
	}
	for _, c := range open {
		last, err := time.Parse(time.RFC3339, c.StartedAt)
		if fi, statErr := os.Stat(c.Transcript); statErr == nil {
			last, err = fi.ModTime(), nil
		}
		if err == nil && time.Since(last) > staleAfter {
			if err := a.settle(c); err != nil {
				return nil, err
			}
		}
	}
	return a.st.Captures(store.CaptureUnfiled)
}

func firstLines(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		s = string(r[:max]) + "…"
	}
	return s
}

// titleFor suggests a task title for an unfiled session.
func titleFor(c *store.Capture) string {
	if c.Name != "" {
		return c.Name
	}
	line, _, _ := strings.Cut(c.Prompt, "\n")
	if line = firstLines(line, 80); line != "" {
		return line
	}
	return "Work in " + c.Repo
}

func linkCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "link <id>",
		Short: "Link the Claude session running this command to a task",
		Long: `Link the Claude Code session running this command to a task. tars asks
Claude to run this when a session might belong to one of several tasks and
the user picks one.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			sid := claude.CurrentID()
			if sid == "" {
				return fmt.Errorf("not in a Claude Code session; file a session from outside with tars file <session> %d", id)
			}
			t, err := a.st.GetTask(id)
			if err != nil {
				return err
			}
			c, err := a.st.GetCapture(sid)
			if errors.Is(err, store.ErrNotFound) {
				c = &store.Capture{SessionID: sid}
				if g, ok := readGit(mustGetwd()); ok {
					c.Root, c.Worktree, c.Branch = g.Root, g.Worktree, g.Branch
					c.Repo, _ = a.repoFor(g)
				}
			} else if err != nil {
				return err
			}
			if err := a.fileSession(c, t); err != nil {
				return err
			}
			return a.emit(map[string]any{"task_id": id, "session_id": sid}, func() string {
				return ui.Success(fmt.Sprintf("Linked this session to #%d %s", id, t.Title)) + "\n"
			})
		},
	}
}

func mustGetwd() string {
	d, _ := os.Getwd()
	return d
}

func unfiledCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "unfiled",
		Short: "Claude sessions that changed code but aren't linked to a task yet",
		Long: `Claude sessions that edited files or committed but weren't matched to a
task. File each into a task (tars file <session> <id>), make it a new task
(tars file <session> --new), or dismiss it (tars dismiss <session>).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cs, err := a.unfiled()
			if err != nil {
				return err
			}
			if cs == nil {
				cs = []*store.Capture{}
			}
			return a.emit(cs, func() string {
				if len(cs) == 0 {
					return ui.Muted.Render("Nothing unfiled.") + "\n"
				}
				var b strings.Builder
				for _, c := range cs {
					fmt.Fprintf(&b, "%s  %s\n", ui.Accent.Render(shortID(c.SessionID)), ui.Bold.Render(titleFor(c)))
					meta := []string{c.Repo}
					if c.Branch != "" {
						meta = append(meta, c.Branch)
					}
					meta = append(meta, fmt.Sprintf("%d edits", c.Edits))
					if c.Commits > 0 {
						meta = append(meta, fmt.Sprintf("%d commits", c.Commits))
					}
					fmt.Fprintf(&b, "          %s\n", ui.Muted.Render(strings.Join(meta, " · ")))
				}
				b.WriteString(ui.Muted.Render("\nFile one: tars file <session> <task-id>  ·  tars file <session> --new  ·  tars dismiss <session>") + "\n")
				return b.String()
			})
		},
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func fileCmd(a *app) *cobra.Command {
	var newTask bool
	var title, status string
	cmd := &cobra.Command{
		Use:   "file <session> [task-id]",
		Short: "File a captured session into a task, or make it a new task",
		Example: `  tars file 3f2a9c1e 21
  tars file 3f2a9c1e --new
  tars file 3f2a9c1e --new -t "Retry webhooks with backoff" -s in-review`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := a.st.FindCapture(args[0])
			if err != nil {
				return err
			}
			var t *store.Task
			switch {
			case newTask && len(args) == 2:
				return fmt.Errorf("give a task id or --new, not both")
			case newTask:
				t = &store.Task{Title: title, Repo: c.Repo, Status: status, Priority: 2}
				if t.Title == "" {
					t.Title = titleFor(c)
				}
				if c.Prompt != "" {
					t.Body = c.Prompt
				}
				if err := a.st.CreateTask(t); err != nil {
					return err
				}
			case len(args) == 2:
				id, err := parseID(args[1])
				if err != nil {
					return err
				}
				if t, err = a.st.GetTask(id); err != nil {
					return err
				}
			default:
				return fmt.Errorf("give the task to file it into, or --new")
			}
			if err := a.fileSession(c, t); err != nil {
				return err
			}
			if err := a.st.AddLog(t.ID, "user", "note", "Filed session "+shortID(c.SessionID)+": "+titleFor(c)); err != nil {
				return err
			}
			return a.emit(map[string]any{"task_id": t.ID, "session_id": c.SessionID}, func() string {
				return ui.Success(fmt.Sprintf("Filed into #%d %s", t.ID, t.Title)) + "\n"
			})
		},
	}
	cmd.Flags().BoolVar(&newTask, "new", false, "make the session a new task")
	cmd.Flags().StringVarP(&title, "title", "t", "", "title for --new (default: the session's title)")
	cmd.Flags().StringVarP(&status, "status", "s", store.StatusInbox, "status for --new")
	return cmd
}

func dismissCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "dismiss <session>…",
		Short: "Mark captured sessions as not task work",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []string
			for _, arg := range args {
				c, err := a.st.FindCapture(arg)
				if err != nil {
					return err
				}
				if err := a.st.SetCaptureState(c.SessionID, store.CaptureDismissed); err != nil {
					return err
				}
				ids = append(ids, c.SessionID)
			}
			return a.emit(map[string]any{"dismissed": ids}, func() string {
				return ui.Success(fmt.Sprintf("Dismissed %d session(s)", len(ids))) + "\n"
			})
		},
	}
}

func mergeCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "merge <from> <into>",
		Short: "Fold one task into another: sessions, reports and log move over",
		Long: `Fold task <from> into task <into> and delete <from>. Its sessions, reports,
log and captures move to <into>; fields <into> has empty are filled in, and
tags are combined. Use it when two tasks turn out to be the same work.`,
		Example: "  tars merge 31 21",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			from, err := parseID(args[0])
			if err != nil {
				return err
			}
			into, err := parseID(args[1])
			if err != nil {
				return err
			}
			if err := a.st.MergeTask(from, into); err != nil {
				return err
			}
			return a.emit(map[string]any{"from": from, "into": into}, func() string {
				return ui.Success(fmt.Sprintf("Merged #%d into #%d", from, into)) + "\n"
			})
		},
	}
}

func captureCmd(a *app) *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "capture [on|off]",
		Short: "Turn automatic session capture on or off, everywhere or for one repo",
		Long: `With the tars plugin installed, every Claude Code session in a git repo is
captured: linked to its task when tars can tell which, or held as unfiled
when it changed code and tars can't. This switches that on or off.`,
		Example: `  tars capture              # show what's on
  tars capture off           # everywhere
  tars capture off --repo billing-svc`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"on", "off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if args[0] != "on" && args[0] != "off" {
					return fmt.Errorf("say on or off, not %q", args[0])
				}
				on := args[0] == "on"
				if repo != "" {
					if _, err := a.st.GetRepo(repo); errors.Is(err, store.ErrNotFound) {
						// Capture runs in every git repo, so let the current one be
						// switched off by name without setting up a profile first.
						g, ok := readGit(mustGetwd())
						if !ok || filepath.Base(g.Root) != repo {
							return fmt.Errorf("no repo called %q; add it with tars repo set %s --path <dir>", repo, repo)
						}
						if err := a.st.UpsertRepo(repo, g.Root); err != nil {
							return err
						}
					}
					if err := a.st.SetRepoCapture(repo, on); err != nil {
						return err
					}
				} else if err := a.st.SetSetting("capture", args[0]); err != nil {
					return err
				}
			}
			on, err := a.st.CaptureEnabled()
			if err != nil {
				return err
			}
			repos, err := a.st.ListRepos()
			if err != nil {
				return err
			}
			off := []string{}
			for _, r := range repos {
				if r.CaptureOff {
					off = append(off, r.Name)
				}
			}
			return a.emit(map[string]any{"on": on, "off_repos": off}, func() string {
				s := "Capture is on"
				if !on {
					s = "Capture is off"
				}
				if len(off) > 0 {
					s += ", except in " + strings.Join(off, ", ")
					if !on {
						s = "Capture is off everywhere"
					}
				}
				return s + "\n"
			})
		},
	}
	cmd.Flags().StringVarP(&repo, "repo", "r", "", "switch one repo instead of everywhere")
	return cmd
}
