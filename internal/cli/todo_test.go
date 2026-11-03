package cli

import (
	"strings"
	"testing"

	"github.com/actuallyakshat/tars/internal/store"
)

func (e *env) todoLists() []store.TodoList {
	e.t.Helper()
	return decode[[]store.TodoList](e.t, e.ok("todo", "ls", "--json"))
}

func texts(l store.TodoList) string {
	var out []string
	for _, it := range l.Items {
		mark := ""
		if it.Done {
			mark = "✓"
		}
		out = append(out, it.Text+mark)
	}
	return strings.Join(out, ",")
}

func TestTodoListLifecycle(t *testing.T) {
	e := newEnv(t)
	if got := e.ok("todo", "ls", "--json"); strings.TrimSpace(got) != "[]" {
		t.Fatalf("empty ls should print [], got %q", got)
	}
	if out := e.ok("todo"); !strings.Contains(out, "No todo lists yet") {
		t.Fatalf("bare todo: %q", out)
	}

	l := decode[store.TodoList](e.t, e.ok("todo", "list", "new", "Ship", "checklist", "--json"))
	if l.ID != 1 || l.Name != "Ship checklist" || l.Items == nil {
		t.Fatalf("%+v", l)
	}
	if out := e.ok("todo", "list", "new", "Release"); !strings.Contains(out, "Created todo list Release") {
		t.Fatalf("%q", out)
	}
	e.ok("todo", "list", "new", "Cleanup")

	e.ok("todo", "list", "rename", "ship checklist", "Ship")
	e.ok("todo", "list", "move", "cleanup", "1")
	var names []string
	for _, l := range e.todoLists() {
		names = append(names, l.Name)
	}
	if strings.Join(names, ",") != "Cleanup,Ship,Release" {
		t.Fatal(names)
	}

	// Deleting by id works, and an empty list needs no --force.
	del := decode[map[string]any](e.t, e.ok("todo", "list", "rm", "3", "--json"))
	if del["name"] != "Cleanup" {
		t.Fatal(del)
	}
}

func TestTodoListValidation(t *testing.T) {
	e := newEnv(t)
	e.ok("todo", "list", "new", "Release")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"todo", "list", "new", "RELEASE"}, `already a todo list called "RELEASE"`},
		{[]string{"todo", "list", "new", "42"}, "can't be just a number"},
		{[]string{"todo", "list", "new", "  "}, "needs a name"},
		{[]string{"todo", "list", "new", strings.Repeat("x", 81)}, "longer than 80"},
		{[]string{"todo", "list", "rename", "nope", "x"}, "not found"},
		{[]string{"todo", "list", "move", "release", "0"}, "invalid position"},
		{[]string{"todo", "list", "move", "release", "2"}, "out of range"},
		{[]string{"todo", "list", "move", "release", "x"}, "invalid position"},
		{[]string{"todo", "show", "nope"}, "not found"},
		{[]string{"todo", "add", "nope", "changelog"}, "not found"},
		{[]string{"todo", "add", "release", " "}, "needs some text"},
		{[]string{"todo", "add", "release", strings.Repeat("y", 501)}, "longer than 500"},
		{[]string{"todo", "edit", "abc", "x"}, `invalid todo id "abc"`},
		{[]string{"todo", "edit", "0", "x"}, "invalid todo id"},
		{[]string{"todo", "check", "9"}, "todo 9: not found"},
		{[]string{"todo", "rm", "9"}, "todo 9: not found"},
	} {
		err := e.fails(tc.args...)
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("tars %s: %q, want it to mention %q", strings.Join(tc.args, " "), err, tc.want)
		}
		if strings.HasSuffix(err.Error(), ": invalid") {
			t.Errorf("error leaks the sentinel: %q", err)
		}
	}
}

func TestTodoItems(t *testing.T) {
	e := newEnv(t)
	e.ok("todo", "list", "new", "Release")
	e.ok("todo", "list", "new", "Cleanup")
	it := decode[store.TodoItem](e.t, e.ok("todo", "add", "release", "update", "changelog", "--json"))
	if it.ID != 1 || it.Text != "update changelog" || it.ListID != 1 || it.Done {
		t.Fatalf("%+v", it)
	}
	e.ok("todo", "add", "release", "tag release")
	e.ok("todo", "add", "#1", "bump deps")
	// Text after -- may start with a dash.
	e.ok("todo", "add", "cleanup", "--", "-race in CI")

	e.ok("todo", "edit", "#2", "tag", "v0.4.0")
	checked := decode[[]store.TodoItem](e.t, e.ok("todo", "check", "1", "3", "--json"))
	if len(checked) != 2 || !checked[0].Done || !checked[1].Done {
		t.Fatalf("%+v", checked)
	}
	e.ok("todo", "uncheck", "3")
	ls := e.todoLists()
	if texts(ls[0]) != "update changelog✓,tag v0.4.0,bump deps" || texts(ls[1]) != "-race in CI" {
		t.Fatal(texts(ls[0]), "|", texts(ls[1]))
	}

	// A typo among ids changes nothing.
	e.fails("todo", "check", "2", "99")
	if texts(e.todoLists()[0]) != "update changelog✓,tag v0.4.0,bump deps" {
		t.Fatal("partial check")
	}

	e.ok("todo", "move", "3", "1")
	if got := texts(e.todoLists()[0]); got != "bump deps,update changelog✓,tag v0.4.0" {
		t.Fatal(got)
	}
	e.ok("todo", "move", "2", "1", "--list", "cleanup")
	ls = e.todoLists()
	if texts(ls[0]) != "bump deps,update changelog✓" || texts(ls[1]) != "tag v0.4.0,-race in CI" {
		t.Fatal(texts(ls[0]), "|", texts(ls[1]))
	}
	if err := e.fails("todo", "move", "1", "5"); !strings.Contains(err.Error(), "use 1 to 2") {
		t.Fatal(err)
	}

	out := e.ok("todo", "show", "release")
	if !strings.Contains(out, "Release #1 1/2 done") || !strings.Contains(out, "[x]   #1 update changelog") {
		t.Fatalf("%q", out)
	}

	cleared := decode[map[string]any](e.t, e.ok("todo", "clear", "release", "--json"))
	if cleared["cleared"] != float64(1) {
		t.Fatal(cleared)
	}
	e.ok("todo", "rm", "3")
	if l := e.todoLists()[0]; len(l.Items) != 0 {
		t.Fatal(texts(l))
	}
}

func TestTodoListRmNeedsForceWhenItHasItems(t *testing.T) {
	e := newEnv(t)
	e.ok("todo", "list", "new", "Release")
	e.ok("todo", "add", "release", "changelog")
	err := e.fails("todo", "list", "rm", "release")
	if !strings.Contains(err.Error(), "Release still has 1 todo; add --force") {
		t.Fatal(err)
	}
	if len(e.todoLists()) != 1 {
		t.Fatal("list deleted without --force")
	}
	e.ok("todo", "list", "rm", "release", "--force")
	if len(e.todoLists()) != 0 {
		t.Fatal("list kept with --force")
	}
	e.fails("todo", "check", "1") // its items went with it
}
