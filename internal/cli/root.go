// Package cli wires tars commands. Every command supports --json for agents.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/akshat-dubey/tars/internal/store"
	"github.com/akshat-dubey/tars/internal/ui"
)

var Version = "dev"

type app struct {
	st     *store.Store
	json   bool
	dbPath string
	out    io.Writer
}

func (a *app) width() int {
	if f, ok := a.out.(*os.File); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			return w
		}
	}
	return 100
}

// emit prints v as JSON in --json mode, otherwise the human rendering.
func (a *app) emit(v any, human func() string) error {
	if a.json {
		enc := json.NewEncoder(a.out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	_, err := fmt.Fprint(a.out, human())
	return err
}

func NewRoot() *cobra.Command {
	a := &app{}
	root := &cobra.Command{
		Use:   "tars",
		Short: "TARS · Track · Assign · Run · Ship — a task ledger you and your agents share",
		Long: `TARS keeps your engineering tasks, Jira links, repo know-how and personal
preferences in one place, so you can drive it yourself or through an agent.

Run with no arguments to open the interactive task list.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			a.out = cmd.OutOrStdout()
			path := a.dbPath
			if path == "" {
				var err error
				if path, err = store.DefaultPath(); err != nil {
					return err
				}
			}
			st, err := store.Open(path)
			if err != nil {
				return fmt.Errorf("open %s: %w", path, err)
			}
			a.st = st
			return nil
		},
		PersistentPostRunE: func(*cobra.Command, []string) error {
			if a.st != nil {
				return a.st.Close()
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f, ok := a.out.(*os.File); ok && term.IsTerminal(int(f.Fd())) && !a.json {
				return ui.RunInbox(a.st)
			}
			// Not a terminal (piped, or an agent): print the list instead.
			return a.list(store.Filter{})
		},
	}
	root.PersistentFlags().BoolVar(&a.json, "json", false, "machine-readable JSON output")
	root.PersistentFlags().StringVar(&a.dbPath, "db", "", "database path (default $TARS_HOME/tars.db or ~/.local/share/tars/tars.db)")

	root.AddCommand(
		uiCmd(a),
		addCmd(a), lsCmd(a), showCmd(a), editCmd(a), mvCmd(a), doneCmd(a),
		noteCmd(a), blockCmd(a), rmCmd(a),
		briefCmd(a), contextCmd(a),
		repoCmd(a), profileCmd(a),
	)
	return root
}

func Execute() {
	if err := NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}
}

func uiCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "ui",
		Aliases: []string{"board"},
		Short:   "Open the interactive task list (same as bare tars)",
		Args:    cobra.NoArgs,
		RunE:    func(*cobra.Command, []string) error { return ui.RunInbox(a.st) },
	}
}

func parseID(s string) (int64, error) {
	if len(s) > 0 && s[0] == '#' {
		s = s[1:]
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid task id %q", s)
	}
	return id, nil
}
