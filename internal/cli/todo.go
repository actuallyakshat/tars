package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/actuallyakshat/tars/internal/store"
	"github.com/actuallyakshat/tars/internal/ui"
)

// Todo lists are named checklists kept apart from tasks. Lists are named by
// id or name; items by their id, which never changes, so a script or agent
// can't hit the wrong item after something else was removed. Positions on the
// command line count from 1.

func todoCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Named todo lists: plain checklists, separate from tasks",
		Long: `Todo lists are named checklists, separate from tasks. Make as many as you like
(release checklist, tech debt, review follow-ups, …), add items, check them off, reorder them and
move them between lists.

A list is named by its id or its name (any case). An item is named by its id,
shown next to it in "tars todo ls". Positions count from 1.`,
		Example: `  tars todo list new Release 0.4
  tars todo add "release 0.4" update the CHANGELOG
  tars todo check 3
  tars todo move 3 1                    # to the top of its list
  tars todo move 3 2 --list tech-debt   # second in another list
  tars todo list rm "release 0.4" --force`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return a.todoLs() },
	}
	cmd.AddCommand(
		todoListCmd(a),
		&cobra.Command{
			Use:   "ls",
			Short: "Show every todo list with its items",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return a.todoLs() },
		},
		&cobra.Command{
			Use:   "show <list>",
			Short: "Show one todo list",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				l, err := a.st.FindTodoList(args[0])
				if err != nil {
					return err
				}
				return a.emit(l, func() string { return renderTodoList(l) })
			},
		},
		&cobra.Command{
			Use:     "add <list> <text>…",
			Short:   "Add an item to the end of a list",
			Example: `  tars todo add tech-debt drop the legacy sync flag`,
			Args:    cobra.MinimumNArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				l, err := a.st.FindTodoList(args[0])
				if err != nil {
					return err
				}
				it, err := a.st.AddTodoItem(l.ID, strings.Join(args[1:], " "))
				if err != nil {
					return err
				}
				return a.emit(it, func() string {
					return ui.Success(fmt.Sprintf("Added %s to %s", ui.Accent.Render(fmt.Sprintf("#%d", it.ID)), l.Name)) + "\n"
				})
			},
		},
		&cobra.Command{
			Use:   "edit <item> <text>…",
			Short: "Change an item's text",
			Args:  cobra.MinimumNArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				id, err := parseTodoID(args[0])
				if err != nil {
					return err
				}
				it, err := a.st.EditTodoItem(id, strings.Join(args[1:], " "))
				if err != nil {
					return err
				}
				return a.emit(it, func() string { return ui.Success(fmt.Sprintf("Updated #%d", it.ID)) + "\n" })
			},
		},
		todoCheckCmd(a, true),
		todoCheckCmd(a, false),
		todoMoveCmd(a),
		&cobra.Command{
			Use:   "rm <item>…",
			Short: "Delete item(s); all or none",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				ids, err := parseTodoIDs(args)
				if err != nil {
					return err
				}
				if err := a.st.DeleteTodoItems(ids...); err != nil {
					return err
				}
				return a.emit(map[string]any{"deleted": ids}, func() string {
					return ui.Success(fmt.Sprintf("Deleted %d %s", len(ids), plural(len(ids), "todo", "todos"))) + "\n"
				})
			},
		},
		&cobra.Command{
			Use:   "clear <list>",
			Short: "Delete a list's checked items",
			Args:  cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				l, err := a.st.FindTodoList(args[0])
				if err != nil {
					return err
				}
				n, err := a.st.ClearDoneTodos(l.ID)
				if err != nil {
					return err
				}
				return a.emit(map[string]any{"list": l.ID, "cleared": n}, func() string {
					return ui.Success(fmt.Sprintf("Cleared %d checked %s from %s", n, plural(n, "todo", "todos"), l.Name)) + "\n"
				})
			},
		},
	)
	return cmd
}

func todoListCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Create, rename, reorder and delete todo lists",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return a.todoLs() },
	}
	var force bool
	rm := &cobra.Command{
		Use:   "rm <list>",
		Short: "Delete a list (one with items needs --force)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := a.st.FindTodoList(args[0])
			if err != nil {
				return err
			}
			if len(l.Items) > 0 && !force {
				n := len(l.Items)
				return fmt.Errorf("%s still has %d %s; add --force to delete them too", l.Name, n, plural(n, "todo", "todos"))
			}
			if err := a.st.DeleteTodoList(l.ID, force); err != nil {
				return err
			}
			return a.emit(map[string]any{"deleted": l.ID, "name": l.Name}, func() string {
				return ui.Success("Deleted todo list "+l.Name) + "\n"
			})
		},
	}
	rm.Flags().BoolVar(&force, "force", false, "also delete the list's items")
	emitList := func(l *store.TodoList, msg string) error {
		return a.emit(l, func() string { return ui.Success(msg) + "\n" })
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:     "new <name>…",
			Short:   "Create an empty todo list",
			Example: `  tars todo list new Release checklist`,
			Args:    cobra.MinimumNArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				l, err := a.st.CreateTodoList(strings.Join(args, " "))
				if err != nil {
					return err
				}
				return emitList(l, fmt.Sprintf("Created todo list %s %s", ui.Accent.Render(l.Name), ui.Muted.Render(fmt.Sprintf("#%d", l.ID))))
			},
		},
		&cobra.Command{
			Use:   "rename <list> <new-name>…",
			Short: "Rename a todo list",
			Args:  cobra.MinimumNArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				l, err := a.st.FindTodoList(args[0])
				if err != nil {
					return err
				}
				old := l.Name
				if l, err = a.st.RenameTodoList(l.ID, strings.Join(args[1:], " ")); err != nil {
					return err
				}
				return emitList(l, fmt.Sprintf("Renamed %s to %s", old, ui.Accent.Render(l.Name)))
			},
		},
		&cobra.Command{
			Use:     "move <list> <position>",
			Short:   "Put a list at a position (1 is first)",
			Example: `  tars todo list move tech-debt 1`,
			Args:    cobra.ExactArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				l, err := a.st.FindTodoList(args[0])
				if err != nil {
					return err
				}
				pos, err := parsePosition(args[1])
				if err != nil {
					return err
				}
				if l, err = a.st.MoveTodoList(l.ID, pos); err != nil {
					return err
				}
				return emitList(l, fmt.Sprintf("Moved %s to position %d", l.Name, l.Position+1))
			},
		},
		rm,
	)
	return cmd
}

func todoCheckCmd(a *app, done bool) *cobra.Command {
	use, short, verb := "check <item>…", "Check item(s) off", "Checked"
	if !done {
		use, short, verb = "uncheck <item>…", "Uncheck item(s)", "Unchecked"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ids, err := parseTodoIDs(args)
			if err != nil {
				return err
			}
			// Check every id exists first, so a typo changes nothing.
			for _, id := range ids {
				if _, err := a.st.GetTodoItem(id); err != nil {
					return err
				}
			}
			items := []*store.TodoItem{}
			for _, id := range ids {
				it, err := a.st.SetTodoDone(id, done)
				if err != nil {
					return err
				}
				items = append(items, it)
			}
			return a.emit(items, func() string {
				return ui.Success(fmt.Sprintf("%s %d %s", verb, len(items), plural(len(items), "todo", "todos"))) + "\n"
			})
		},
	}
}

func todoMoveCmd(a *app) *cobra.Command {
	var list string
	cmd := &cobra.Command{
		Use:   "move <item> <position>",
		Short: "Reorder an item, or move it to another list with --list",
		Example: `  tars todo move 7 1               # top of its list
  tars todo move 7 3 --list tech-debt   # third in tech-debt`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseTodoID(args[0])
			if err != nil {
				return err
			}
			pos, err := parsePosition(args[1])
			if err != nil {
				return err
			}
			var to int64
			if cmd.Flags().Changed("list") {
				l, err := a.st.FindTodoList(list)
				if err != nil {
					return err
				}
				to = l.ID
			}
			it, err := a.st.MoveTodoItem(id, to, pos)
			if err != nil {
				return err
			}
			return a.emit(it, func() string {
				return ui.Success(fmt.Sprintf("Moved #%d to position %d", it.ID, it.Position+1)) + "\n"
			})
		},
	}
	cmd.Flags().StringVarP(&list, "list", "l", "", "move into this list (id or name)")
	return cmd
}

func (a *app) todoLs() error {
	ls, err := a.st.TodoLists()
	if err != nil {
		return err
	}
	return a.emit(ls, func() string {
		if len(ls) == 0 {
			return ui.Muted.Render("No todo lists yet. Make one with: "+self()+" todo list new <name>") + "\n"
		}
		var b strings.Builder
		for i, l := range ls {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(renderTodoList(l))
		}
		return b.String()
	})
}

func renderTodoList(l *store.TodoList) string {
	var b strings.Builder
	done := 0
	for _, it := range l.Items {
		if it.Done {
			done++
		}
	}
	fmt.Fprintf(&b, "%s %s %s\n", ui.Bold.Render(l.Name), ui.Muted.Render(fmt.Sprintf("#%d", l.ID)),
		ui.Muted.Render(fmt.Sprintf("%d/%d done", done, len(l.Items))))
	if len(l.Items) == 0 {
		b.WriteString(ui.Muted.Render("  (empty)") + "\n")
	}
	for _, it := range l.Items {
		box, text := "[ ]", it.Text
		if it.Done {
			box, text = "[x]", ui.Muted.Render(it.Text)
		}
		fmt.Fprintf(&b, "  %s %s %s\n", box, ui.Muted.Render(fmt.Sprintf("%4s", fmt.Sprintf("#%d", it.ID))), text)
	}
	return b.String()
}

func parseTodoID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimPrefix(s, "#"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid todo id %q", s)
	}
	return id, nil
}

func parseTodoIDs(args []string) ([]int64, error) {
	ids := make([]int64, 0, len(args))
	for _, raw := range args {
		id, err := parseTodoID(raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// parsePosition turns a 1-based position into the store's 0-based index.
func parsePosition(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid position %q: use 1 for the first", s)
	}
	return n - 1, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
