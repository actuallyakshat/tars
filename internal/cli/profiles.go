package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/actuallyakshat/tars/internal/store"
	"github.com/actuallyakshat/tars/internal/ui"
)

// sectionText resolves --text / --file into content ("-" reads stdin).
func sectionText(cmd *cobra.Command, text, file string) (string, error) {
	if cmd.Flags().Changed("file") {
		b, err := os.ReadFile(file)
		return string(b), err
	}
	return readText(cmd, text)
}

func repoCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Repo profiles: definition of done, test commands, DB connections, gotchas",
		Long: `Repo profiles hold know-how an agent needs to work in a repo. They supplement,
and never override, the repo's own CLAUDE.md / AGENTS.md.

Suggested sections: ` + strings.Join(store.RepoSections, ", ") + `. Any name works.`,
	}
	cmd.AddCommand(repoSetCmd(a), repoShowCmd(a), repoLsCmd(a), repoRmCmd(a))
	return cmd
}

func repoSetCmd(a *app) *cobra.Command {
	var path, section, text, file string
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Create a repo profile or set one of its sections",
		Example: `  tars repo set billing-svc --path ~/code/billing-svc
  tars repo set billing-svc -S tests -x "make test; integration needs docker compose up db"
  tars repo set billing-svc -S dod -f ./DOD.md
  tars repo set billing-svc -S gotchas -x ""      # empty text removes a section`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("repo name can't be empty")
			}
			if err := a.st.UpsertRepo(name, path); err != nil {
				return err
			}
			if section != "" {
				if !cmd.Flags().Changed("text") && !cmd.Flags().Changed("file") {
					return fmt.Errorf("--section needs --text or --file")
				}
				content, err := sectionText(cmd, text, file)
				if err != nil {
					return err
				}
				if err := a.st.SetRepoSection(name, strings.ToLower(section), content); err != nil {
					return err
				}
			}
			r, err := a.st.GetRepo(name)
			if err != nil {
				return err
			}
			return a.emit(r, func() string { return ui.Success("Saved repo profile "+ui.Accent.Render(name)) + "\n" })
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&path, "path", "", "local checkout path (lets tars find the profile from inside the repo)")
	fl.StringVarP(&section, "section", "S", "", "section to set: "+strings.Join(store.RepoSections, ", ")+", or any name")
	fl.StringVarP(&text, "text", "x", "", "section content (\"-\" reads stdin, \"\" removes)")
	fl.StringVarP(&file, "file", "f", "", "read section content from a file")
	return cmd
}

func repoShowCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show a repo profile (defaults to the repo containing the current directory)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var r *store.Repo
			var err error
			if len(args) == 1 {
				r, err = a.st.GetRepo(args[0])
			} else {
				r, err = a.st.FindRepoByPath(".")
			}
			if err != nil {
				return err
			}
			return a.emit(r, func() string { return ui.Repo(r, a.width()) })
		},
	}
}

func repoLsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List repo profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			rs, err := a.st.ListRepos()
			if err != nil {
				return err
			}
			if rs == nil {
				rs = []*store.Repo{}
			}
			return a.emit(rs, func() string { return ui.Repos(rs) })
		},
	}
}

func repoRmCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Delete a repo profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.st.DeleteRepo(args[0]); err != nil {
				return err
			}
			return a.emit(map[string]any{"deleted": args[0]}, func() string {
				return ui.Success("Deleted repo profile "+args[0]) + "\n"
			})
		},
	}
}

func profileCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Your personal working preferences, applied in every repo",
		Long: `The personal profile tells agents how you like to work: code style, review
habits, commit messages, things to avoid. Repo CLAUDE.md / AGENTS.md and
repo profiles take precedence where they conflict.

Suggested sections: ` + strings.Join(store.ProfileSections, ", ") + `. Any name works.`,
	}
	var text, file string
	set := &cobra.Command{
		Use:     "set <section>",
		Short:   "Set a profile section (empty text removes it)",
		Example: `  tars profile set style -x "Small PRs. No mocks for the DB layer; use the test container."`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("text") && !cmd.Flags().Changed("file") {
				return fmt.Errorf("needs --text or --file")
			}
			content, err := sectionText(cmd, text, file)
			if err != nil {
				return err
			}
			if err := a.st.SetProfileSection(strings.ToLower(args[0]), content); err != nil {
				return err
			}
			return a.emit(map[string]any{"section": args[0], "saved": true}, func() string {
				return ui.Success("Saved profile section "+args[0]) + "\n"
			})
		},
	}
	set.Flags().StringVarP(&text, "text", "x", "", "content (\"-\" reads stdin)")
	set.Flags().StringVarP(&file, "file", "f", "", "read content from a file")
	show := &cobra.Command{
		Use:   "show",
		Short: "Show your personal profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			secs, err := a.st.Profile()
			if err != nil {
				return err
			}
			if secs == nil {
				secs = []store.Section{}
			}
			return a.emit(secs, func() string { return ui.Profile(secs, a.width()) })
		},
	}
	cmd.AddCommand(set, show)
	return cmd
}

func briefCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "brief",
		Short: "Morning summary: reviews waiting, due, follow-ups, stale work",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, err := a.st.Brief()
			if err != nil {
				return err
			}
			return a.emit(b, func() string { return ui.Brief(b, a.width()) })
		},
	}
}

type agentContext struct {
	Profile []store.Section `json:"profile"`
	Repo    *store.Repo     `json:"repo,omitempty"`
	Task    *store.Task     `json:"task,omitempty"`
}

func contextCmd(a *app) *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "context [task-id]",
		Short: "Everything an agent should know before working: profile, repo profile, task",
		Long: `Prints your personal profile, the relevant repo profile and (optionally) a task
with its full log, as Markdown (or --json). The repo is taken from --repo, then
the task's repo, then the repo profile whose path contains the current directory.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var ctx agentContext
			var err error
			if ctx.Profile, err = a.st.Profile(); err != nil {
				return err
			}
			if len(args) == 1 {
				id, err := parseID(args[0])
				if err != nil {
					return err
				}
				if ctx.Task, err = a.st.GetTask(id); err != nil {
					return err
				}
				if repo == "" {
					repo = ctx.Task.Repo
				}
			}
			if repo != "" {
				ctx.Repo, err = a.st.GetRepo(repo)
			} else {
				ctx.Repo, err = a.st.FindRepoByPath(".")
			}
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return err
			}
			if ctx.Profile == nil {
				ctx.Profile = []store.Section{}
			}
			return a.emit(ctx, func() string { return contextMarkdown(ctx) })
		},
	}
	cmd.Flags().StringVarP(&repo, "repo", "r", "", "repo profile to include")
	return cmd
}

func contextMarkdown(c agentContext) string {
	var b strings.Builder
	b.WriteString("# TARS context\n\n")
	b.WriteString("> Precedence: the repo's CLAUDE.md / AGENTS.md > repo profile > personal profile.\n")

	b.WriteString("\n## Personal profile\n")
	if len(c.Profile) == 0 {
		b.WriteString("\n_(empty)_\n")
	}
	for _, s := range c.Profile {
		fmt.Fprintf(&b, "\n### %s\n\n%s\n", s.Name, strings.TrimSpace(s.Content))
	}

	if c.Repo != nil {
		fmt.Fprintf(&b, "\n## Repo profile: %s\n", c.Repo.Name)
		if c.Repo.Path != "" {
			fmt.Fprintf(&b, "\nPath: `%s`\n", c.Repo.Path)
		}
		for _, s := range c.Repo.Sections {
			fmt.Fprintf(&b, "\n### %s\n\n%s\n", s.Name, strings.TrimSpace(s.Content))
		}
	} else {
		b.WriteString("\n## Repo profile\n\n_(none found)_\n")
	}

	if t := c.Task; t != nil {
		fmt.Fprintf(&b, "\n## Task #%d: %s\n\n", t.ID, t.Title)
		row := func(k, v string) {
			if v != "" {
				fmt.Fprintf(&b, "- **%s:** %s\n", k, v)
			}
		}
		row("status", t.Status)
		row("priority", ui.PriorityName(t.Priority))
		row("jira", t.Jira)
		row("repo", t.Repo)
		row("tags", strings.Join(t.Tags, ", "))
		row("due", t.Due)
		row("branch", t.Branch)
		row("worktree", t.Worktree)
		row("waiting on", t.WaitingOn)
		row("follow up", t.FollowUp)
		if t.Body != "" {
			fmt.Fprintf(&b, "\n### Description\n\n%s\n", strings.TrimSpace(t.Body))
		}
		if t.Acceptance != "" {
			fmt.Fprintf(&b, "\n### Acceptance criteria\n\n%s\n", strings.TrimSpace(t.Acceptance))
		}
		if r := t.Report; r != nil {
			b.WriteString("\n### Latest report\n\n")
			fmt.Fprintf(&b, "- **done:** %s\n", r.Done)
			row("verified", r.Verified)
			row("next", r.Next)
			if r.NeedsYou != "" {
				if r.AnsweredAt != "" {
					fmt.Fprintf(&b, "- **asked the user:** %s\n- **user answered:** %s\n", r.NeedsYou, r.Answer)
				} else {
					fmt.Fprintf(&b, "- **waiting on the user:** %s\n", r.NeedsYou)
				}
			}
		}
		if len(t.Sessions) > 0 {
			b.WriteString("\n### Sessions that worked on this\n\n")
			for _, s := range t.Sessions {
				name := s.Name
				if name == "" {
					name = "(unnamed)"
				}
				fmt.Fprintf(&b, "- %s: `claude --resume %s`\n", name, s.ID)
			}
		}
		if len(t.Log) > 0 {
			b.WriteString("\n### Log\n\n")
			for _, e := range t.Log {
				fmt.Fprintf(&b, "- %s · %s · %s: %s\n", e.At, e.Author, e.Kind, strings.ReplaceAll(strings.TrimSpace(e.Text), "\n", "\n  "))
			}
		}
	}
	return b.String()
}
