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

	"github.com/actuallyakshat/tars/internal/store"
	"github.com/actuallyakshat/tars/internal/ui"
)

var Version = "dev"

// Flavor is "dev" in builds made by `tars dev update` (set with -ldflags).
// A dev build is named tars-dev and keeps its own database, so it runs next
// to the released tars without touching its data.
var Flavor = ""

func isDev() bool { return Flavor == "dev" }

// self is this binary's command name, for messages that tell someone what to run.
func self() string {
	if isDev() {
		return "tars-dev"
	}
	return "tars"
}

type app struct {
	st     *store.Store
	json   bool
	dbPath string
	out    io.Writer
	sh     shell // runs external commands for update; nil means the real shell
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

func NewRoot() *cobra.Command { return newRoot(&app{}) }

func newRoot(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:   self(),
		Short: "A task list you and your coding agent share",
		Long: `TARS keeps your engineering tasks, Jira links, repo know-how and personal
preferences in one place, so you can drive it yourself or through an agent.

Run with no arguments to open the interactive task list.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			a.out = cmd.OutOrStdout()
			if cmd.Annotations["nodb"] != "" {
				return nil
			}
			path := a.dbPath
			if path == "" {
				var err error
				if path, err = store.DefaultPathFor(isDev()); err != nil {
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
		reportCmd(a), answerCmd(a), sessionsCmd(a), seenCmd(a),
		contextCmd(a),
		repoCmd(a), profileCmd(a), todoCmd(a), usageCmd(a), updateCmd(a), devCmd(a),
		captureCmd(a), unfiledCmd(a), capturedCmd(a), fileCmd(a), dismissCmd(a), linkCmd(a), unlinkCmd(a), mergeCmd(a), hookCmd(a),
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
