package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/actuallyakshat/tars/internal/parse"
	"github.com/actuallyakshat/tars/internal/store"
	"github.com/actuallyakshat/tars/internal/ui"
)

var statusAliases = map[string]string{
	"todo": store.StatusInbox, "new": store.StatusInbox,
	"wip": store.StatusInProgress, "progress": store.StatusInProgress, "doing": store.StatusInProgress, "started": store.StatusInProgress,
	"review":  store.StatusInReview,
	"waiting": store.StatusBlocked,
	"closed":  store.StatusDone, "complete": store.StatusDone,
}

func normalizeStatus(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if store.ValidStatus(s) {
		return s, nil
	}
	if v, ok := statusAliases[s]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown status %q (use %s)", s, strings.Join(store.Statuses, ", "))
}

// readText returns v, or all of stdin when v is "-".
func readText(cmd *cobra.Command, v string) (string, error) {
	if v != "-" {
		return v, nil
	}
	b, err := io.ReadAll(cmd.InOrStdin())
	return strings.TrimRight(string(b), "\n"), err
}

type taskFlags struct {
	title, body, jira, repo, due, priority, status, accept, branch, worktree string
	tags, untag                                                              []string
}

func (f *taskFlags) register(cmd *cobra.Command, edit bool) {
	fl := cmd.Flags()
	if edit {
		fl.StringVar(&f.title, "title", "", "new title")
		fl.StringSliceVar(&f.untag, "untag", nil, "remove tag(s)")
	}
	fl.StringVarP(&f.body, "body", "b", "", "description (\"-\" reads stdin)")
	fl.StringVarP(&f.jira, "jira", "j", "", "Jira key, e.g. PROJ-123")
	fl.StringVarP(&f.repo, "repo", "r", "", "repo profile name")
	fl.StringSliceVarP(&f.tags, "tag", "t", nil, "tag(s), repeatable or comma separated")
	fl.StringVarP(&f.due, "due", "d", "", "due date: 2026-11-03, today, tomorrow, fri, +3d, none")
	fl.StringVarP(&f.priority, "priority", "p", "", "urgent, high, normal, low (or 0-3)")
	fl.StringVarP(&f.status, "status", "s", "", "status: "+strings.Join(store.Statuses, ", "))
	fl.StringVarP(&f.accept, "accept", "a", "", "acceptance criteria (\"-\" reads stdin)")
	fl.StringVar(&f.branch, "branch", "", "git branch holding the work")
	fl.StringVar(&f.worktree, "worktree", "", "git worktree path holding the work")
}

var taskFlagNames = []string{"title", "untag", "body", "jira", "repo", "tag", "due", "priority", "status", "accept", "branch", "worktree"}

func (f *taskFlags) anyChanged(cmd *cobra.Command) bool {
	return slices.ContainsFunc(taskFlagNames, cmd.Flags().Changed)
}

// apply copies every flag the user actually set onto t.
func (f *taskFlags) apply(cmd *cobra.Command, t *store.Task) error {
	ch := cmd.Flags().Changed
	var err error
	if ch("title") {
		t.Title = strings.TrimSpace(f.title)
		if t.Title == "" {
			return fmt.Errorf("title can't be empty")
		}
	}
	if ch("body") {
		if t.Body, err = readText(cmd, f.body); err != nil {
			return err
		}
	}
	if ch("accept") {
		if t.Acceptance, err = readText(cmd, f.accept); err != nil {
			return err
		}
	}
	if ch("jira") {
		t.Jira = strings.ToUpper(strings.TrimSpace(f.jira))
	}
	if ch("repo") {
		t.Repo = strings.TrimSpace(f.repo)
	}
	if ch("due") {
		if t.Due, err = parse.Date(f.due, time.Now()); err != nil {
			return err
		}
	}
	if ch("priority") {
		if t.Priority, err = parse.Priority(f.priority); err != nil {
			return err
		}
	}
	if ch("status") {
		if t.Status, err = normalizeStatus(f.status); err != nil {
			return err
		}
	}
	if ch("branch") {
		t.Branch = f.branch
	}
	if ch("worktree") {
		t.Worktree = f.worktree
	}
	for _, tag := range f.tags {
		tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
		if tag != "" && !slices.Contains(t.Tags, tag) {
			t.Tags = append(t.Tags, tag)
		}
	}
	for _, tag := range f.untag {
		tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
		t.Tags = slices.DeleteFunc(t.Tags, func(x string) bool { return x == tag })
	}
	return nil
}

func addCmd(a *app) *cobra.Command {
	var f taskFlags
	cmd := &cobra.Command{
		Use:   "add <title…>",
		Short: "Add a task",
		Example: `  tars add "Fix CSV export timeout" -j BILL-881 -r billing-svc -d fri -p high
  tars add "Ask Rahul for staging keys" -t followup`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := &store.Task{Title: strings.TrimSpace(strings.Join(args, " ")), Priority: 2}
			if t.Title == "" {
				return fmt.Errorf("title can't be empty")
			}
			if err := f.apply(cmd, t); err != nil {
				return err
			}
			if err := a.st.CreateTask(t); err != nil {
				return err
			}
			return a.emit(t, func() string {
				return ui.Success(fmt.Sprintf("Added #%d %s", t.ID, ui.Bold.Render(t.Title))) + "\n"
			})
		},
	}
	f.register(cmd, false)
	return cmd
}

func (a *app) list(f store.Filter) error {
	tasks, err := a.st.ListTasks(f)
	if err != nil {
		return err
	}
	if tasks == nil {
		tasks = []*store.Task{} // JSON [] rather than null
	}
	return a.emit(tasks, func() string { return ui.List(tasks, a.width()) })
}

func lsCmd(a *app) *cobra.Command {
	var statuses []string
	var all, jira, noJira bool
	var f store.Filter
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tasks (open ones by default)",
		Example: `  tars ls --jira                 # open tasks linked to Jira
  tars ls -s ready,in-review -r billing-svc
  tars ls --all -q export --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, s := range statuses {
				st, err := normalizeStatus(s)
				if err != nil {
					return err
				}
				f.Statuses = append(f.Statuses, st)
			}
			if jira && noJira {
				return fmt.Errorf("--jira and --no-jira are mutually exclusive")
			}
			if jira || noJira {
				f.HasJira = &jira
			}
			f.IncludeDone = all
			return a.list(f)
		},
	}
	fl := cmd.Flags()
	fl.StringSliceVarP(&statuses, "status", "s", nil, "only these statuses (comma separated)")
	fl.BoolVar(&all, "all", false, "include done tasks")
	fl.BoolVar(&jira, "jira", false, "only tasks with a Jira key")
	fl.BoolVar(&noJira, "no-jira", false, "only tasks without a Jira key")
	fl.StringVarP(&f.Repo, "repo", "r", "", "only tasks for this repo")
	fl.StringVarP(&f.Tag, "tag", "t", "", "only tasks with this tag")
	fl.StringVarP(&f.Search, "query", "q", "", "search title, description and Jira key")
	return cmd
}

func showCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show a task with its full log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			t, err := a.st.GetTask(id)
			if err != nil {
				return err
			}
			return a.emit(t, func() string { return ui.Task(t, a.width()) })
		},
	}
}

// save persists t and logs a status transition if there was one.
func (a *app) save(t *store.Task, prevStatus, by string) error {
	if err := a.st.SaveTask(t); err != nil {
		return err
	}
	if prevStatus != t.Status {
		return a.st.AddLog(t.ID, by, "status", prevStatus+" → "+t.Status)
	}
	return nil
}

func editCmd(a *app) *cobra.Command {
	var f taskFlags
	var by string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Change fields of a task",
		Example: `  tars edit 12 -j BILL-900 --untag later -t urgent
  tars edit 12 --branch tars/12-csv-export --worktree ../billing-12`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !f.anyChanged(cmd) {
				return fmt.Errorf("nothing to change; see tars edit --help")
			}
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			t, err := a.st.GetTask(id)
			if err != nil {
				return err
			}
			prev := t.Status
			if err := f.apply(cmd, t); err != nil {
				return err
			}
			if err := a.save(t, prev, by); err != nil {
				return err
			}
			// Agent edits are logged so the TUI can flag them as unread.
			if by == "agent" {
				var fields []string
				for _, name := range taskFlagNames {
					if cmd.Flags().Changed(name) && name != "status" {
						fields = append(fields, name)
					}
				}
				if len(fields) > 0 {
					if err := a.st.AddLog(t.ID, by, "edit", "updated "+strings.Join(fields, ", ")); err != nil {
						return err
					}
				}
			}
			t.Log = nil
			return a.emit(t, func() string { return ui.Success(fmt.Sprintf("Updated #%d", t.ID)) + "\n" })
		},
	}
	f.register(cmd, true)
	byFlag(cmd, &by)
	return cmd
}

func byFlag(cmd *cobra.Command, by *string) {
	cmd.Flags().StringVar(by, "by", "user", "who is making this change: user or agent")
}

func setStatus(a *app, ids []string, status, by string) error {
	var changed []*store.Task
	for _, raw := range ids {
		id, err := parseID(raw)
		if err != nil {
			return err
		}
		t, err := a.st.GetTask(id)
		if err != nil {
			return err
		}
		if err := a.st.SetStatus(t, status, by); err != nil {
			return err
		}
		t.Log = nil
		changed = append(changed, t)
	}
	return a.emit(changed, func() string {
		var b strings.Builder
		for _, t := range changed {
			b.WriteString(ui.Success(fmt.Sprintf("#%d %s → %s", t.ID, t.Title, ui.StatusBadge(t.Status))) + "\n")
		}
		return b.String()
	})
}

func mvCmd(a *app) *cobra.Command {
	var by string
	cmd := &cobra.Command{
		Use:     "mv <id>… <status>",
		Aliases: []string{"move"},
		Short:   "Move task(s) to a status",
		Example: "  tars mv 12 ready\n  tars mv 12 14 in-review --by agent",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := normalizeStatus(args[len(args)-1])
			if err != nil {
				return err
			}
			return setStatus(a, args[:len(args)-1], status, by)
		},
	}
	byFlag(cmd, &by)
	return cmd
}

func doneCmd(a *app) *cobra.Command {
	var by string
	cmd := &cobra.Command{
		Use:   "done <id>…",
		Short: "Mark task(s) done",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setStatus(a, args, store.StatusDone, by)
		},
	}
	byFlag(cmd, &by)
	return cmd
}

func noteCmd(a *app) *cobra.Command {
	var by, kind string
	cmd := &cobra.Command{
		Use:   "note <id> <text…>",
		Short: "Append to a task's log",
		Long: `Append an entry to a task's log. Agents use this to record decisions,
progress, test results and where they stopped. Text "-" reads stdin.`,
		Example: `  tars note 12 "Root cause: N+1 query in exporter"
  tars note 12 --by agent --kind park - < handoff.md`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			text, err := readText(cmd, strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("note is empty")
			}
			if _, err := a.st.GetTask(id); err != nil {
				return err
			}
			if err := a.st.AddLog(id, by, kind, text); err != nil {
				return err
			}
			return a.emit(map[string]any{"id": id, "logged": true}, func() string {
				return ui.Success(fmt.Sprintf("Noted on #%d", id)) + "\n"
			})
		},
	}
	byFlag(cmd, &by)
	cmd.Flags().StringVarP(&kind, "kind", "k", "note", "entry kind: note, park, sweep, test, triage")
	return cmd
}

func blockCmd(a *app) *cobra.Command {
	var on, follow, reason, by string
	cmd := &cobra.Command{
		Use:     "block <id>",
		Short:   "Mark a task blocked, optionally on someone, with a follow-up date",
		Example: `  tars block 12 --on "Rahul (staging keys)" --follow-up thu`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			t, err := a.st.GetTask(id)
			if err != nil {
				return err
			}
			prev := t.Status
			t.Status = store.StatusBlocked
			t.WaitingOn = strings.TrimSpace(on)
			if t.FollowUp, err = parse.Date(follow, time.Now()); err != nil {
				return err
			}
			if err := a.save(t, prev, by); err != nil {
				return err
			}
			if reason != "" {
				if err := a.st.AddLog(id, by, "note", "Blocked: "+reason); err != nil {
					return err
				}
			}
			t.Log = nil
			return a.emit(t, func() string {
				msg := fmt.Sprintf("#%d blocked", id)
				if t.WaitingOn != "" {
					msg += " on " + t.WaitingOn
				}
				if t.FollowUp != "" {
					msg += ", follow up " + t.FollowUp
				}
				return ui.Success(msg) + "\n"
			})
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&on, "on", "", "who or what you're waiting on")
	fl.StringVar(&follow, "follow-up", "", "when to follow up (2026-11-03, fri, +2d…)")
	fl.StringVar(&reason, "reason", "", "why it's blocked (logged)")
	byFlag(cmd, &by)
	return cmd
}

func rmCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <id>…",
		Short: "Delete task(s) permanently",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ids []int64
			for _, raw := range args {
				id, err := parseID(raw)
				if err != nil {
					return err
				}
				if err := a.st.DeleteTask(id); err != nil {
					return err
				}
				ids = append(ids, id)
			}
			return a.emit(map[string]any{"deleted": ids}, func() string {
				return ui.Success(fmt.Sprintf("Deleted %d task(s)", len(ids))) + "\n"
			})
		},
	}
}
