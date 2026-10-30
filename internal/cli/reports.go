package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/actuallyakshat/tars/internal/claude"
	"github.com/actuallyakshat/tars/internal/store"
	"github.com/actuallyakshat/tars/internal/ui"
)

// linkSession ties the Claude Code session running this command (if any) to
// the task, so the user can resume it later. Outside Claude Code it's a no-op.
func (a *app) linkSession(taskID int64, id string) error {
	if id == "" {
		id = claude.CurrentID()
	}
	if id == "" {
		return nil
	}
	s := claude.Inspect(claude.ConfigDir(), id)
	return a.st.LinkSession(taskID, store.Session{ID: s.ID, Name: s.Name, LaunchDir: s.LaunchDir})
}

func reportCmd(a *app) *cobra.Command {
	var r store.Report
	var session string
	cmd := &cobra.Command{
		Use:   "report <id>",
		Short: "Record what an agent did on a task, in a few short lines",
		Long: fmt.Sprintf(`Agents end every piece of work with a report instead of a long narrative.
The user reads the report in tars and opens the linked session only if they
want the details. Each field is capped at %d characters.

The Claude Code session running this command is linked automatically.`, store.ReportFieldMax),
		Example: `  tars report 15 \
    --done "Exponential backoff, 5 tries, alerts on final failure" \
    --verified "make test: 212 passed (6 new)" \
    --needs-you "Alert #oncall or #billing-alerts?" \
    --next "Open the PR once answered"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if _, err := a.st.GetTask(id); err != nil {
				return err
			}
			if session == "" {
				session = claude.CurrentID()
			}
			r.TaskID, r.SessionID = id, session
			if err := a.st.AddReport(&r); err != nil {
				return err
			}
			if err := a.linkSession(id, session); err != nil {
				return err
			}
			return a.emit(r, func() string {
				msg := fmt.Sprintf("Reported on #%d", id)
				if r.NeedsYou != "" {
					msg += ", with a question for you"
				}
				return ui.Success(msg) + "\n"
			})
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&r.Done, "done", "", "what changed (required)")
	fl.StringVar(&r.Verified, "verified", "", "how it was checked, e.g. tests and their result")
	fl.StringVar(&r.NeedsYou, "needs-you", "", "one question or decision for the user, if any")
	fl.StringVar(&r.Next, "next", "", "what happens next")
	fl.StringVar(&session, "session", "", "Claude session ID (default: the current session)")
	return cmd
}

func answerCmd(a *app) *cobra.Command {
	var by string
	cmd := &cobra.Command{
		Use:     "answer <id> <answer…>",
		Short:   "Answer the question an agent left on a task",
		Example: `  tars answer 15 "billing-alerts"`,
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if err := a.st.Answer(id, strings.Join(args[1:], " "), by); err != nil {
				if errors.Is(err, store.ErrNothingToAnswer) {
					return fmt.Errorf("#%d has no open question", id)
				}
				return err
			}
			return a.emit(map[string]any{"id": id, "answered": true}, func() string {
				return ui.Success(fmt.Sprintf("Answered #%d. The next agent on it will see your answer.", id)) + "\n"
			})
		},
	}
	byFlag(cmd, &by)
	return cmd
}

func sessionsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "sessions <id>",
		Short: "List the Claude sessions that worked on a task, with resume commands",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if _, err := a.st.GetTask(id); err != nil {
				return err
			}
			ss, err := a.st.TaskSessions(id)
			if err != nil {
				return err
			}
			if ss == nil {
				ss = []store.TaskSession{}
			}
			return a.emit(ss, func() string { return ui.Sessions(ss) })
		},
	}
}
