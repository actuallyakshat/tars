package store

import (
	"errors"
	"strings"
	"testing"
)

func mustList(t *testing.T, s *Store, name string) *TodoList {
	t.Helper()
	l, err := s.CreateTodoList(name)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func mustItem(t *testing.T, s *Store, list int64, text string) *TodoItem {
	t.Helper()
	it, err := s.AddTodoItem(list, text)
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func itemTexts(t *testing.T, s *Store, list int64) string {
	t.Helper()
	l, err := s.GetTodoList(list)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for i, it := range l.Items {
		if it.Position != i {
			t.Fatalf("item %d at index %d has position %d", it.ID, i, it.Position)
		}
		out = append(out, it.Text)
	}
	return strings.Join(out, ",")
}

func listNames(t *testing.T, s *Store) string {
	t.Helper()
	ls, err := s.TodoLists()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for i, l := range ls {
		if l.Position != i {
			t.Fatalf("list %q at index %d has position %d", l.Name, i, l.Position)
		}
		out = append(out, l.Name)
	}
	return strings.Join(out, ",")
}

func TestTodoListNameValidation(t *testing.T) {
	s := openTemp(t)
	for _, bad := range []string{"", "   ", "42", "#7", "-x", "two\nlines", "tab\there", strings.Repeat("a", MaxTodoListName+1)} {
		if _, err := s.CreateTodoList(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("CreateTodoList(%q) = %v, want ErrInvalid", bad, err)
		}
	}
	l := mustList(t, s, "  Release  ")
	if l.Name != "Release" || l.Items == nil || len(l.Items) != 0 {
		t.Fatalf("got %+v", l)
	}
	// Names are unique ignoring case.
	if _, err := s.CreateTodoList("release"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "already") {
		t.Fatalf("duplicate name: %v", err)
	}
	// Unicode counts characters, not bytes.
	if _, err := s.CreateTodoList(strings.Repeat("é", MaxTodoListName)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTodoList("Q4 2026 launch"); err != nil {
		t.Fatal(err)
	}
}

func TestTodoListFindRenameAndOrder(t *testing.T) {
	s := openTemp(t)
	a, b, c := mustList(t, s, "Cleanup"), mustList(t, s, "Backlog"), mustList(t, s, "Oncall")
	if listNames(t, s) != "Cleanup,Backlog,Oncall" {
		t.Fatal(listNames(t, s))
	}
	for _, ref := range []string{"backlog", "BACKLOG", "#2", "2"} {
		got, err := s.FindTodoList(ref)
		if err != nil || got.ID != b.ID {
			t.Fatalf("FindTodoList(%q) = %v, %v", ref, got, err)
		}
	}
	if _, err := s.FindTodoList("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.FindTodoList("99"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}

	if _, err := s.RenameTodoList(b.ID, "cleanup"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("rename onto another list's name: %v", err)
	}
	// Changing only the case of its own name is fine.
	if l, err := s.RenameTodoList(b.ID, "BACKLOG"); err != nil || l.Name != "BACKLOG" {
		t.Fatalf("%v %v", l, err)
	}
	if _, err := s.RenameTodoList(99, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.RenameTodoList(a.ID, " "); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}

	if _, err := s.MoveTodoList(c.ID, 0); err != nil {
		t.Fatal(err)
	}
	if listNames(t, s) != "Oncall,Cleanup,BACKLOG" {
		t.Fatal(listNames(t, s))
	}
	if _, err := s.MoveTodoList(c.ID, 2); err != nil {
		t.Fatal(err)
	}
	if listNames(t, s) != "Cleanup,BACKLOG,Oncall" {
		t.Fatal(listNames(t, s))
	}
	for _, pos := range []int{-1, 3} {
		if _, err := s.MoveTodoList(a.ID, pos); !errors.Is(err, ErrInvalid) {
			t.Fatalf("pos %d: %v", pos, err)
		}
	}
	if _, err := s.MoveTodoList(99, 0); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestTodoItemsAddEditCheck(t *testing.T) {
	s := openTemp(t)
	l := mustList(t, s, "Release")
	for _, bad := range []string{"", "  ", "a\nb", strings.Repeat("x", MaxTodoText+1)} {
		if _, err := s.AddTodoItem(l.ID, bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("AddTodoItem(%q) = %v", bad, err)
		}
	}
	if _, err := s.AddTodoItem(99, "changelog"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	changelog := mustItem(t, s, l.ID, " changelog ")
	mustItem(t, s, l.ID, "tag release")
	if changelog.Text != "changelog" || changelog.Done || changelog.ListID != l.ID {
		t.Fatalf("%+v", changelog)
	}
	if itemTexts(t, s, l.ID) != "changelog,tag release" {
		t.Fatal(itemTexts(t, s, l.ID))
	}

	got, err := s.EditTodoItem(changelog.ID, "update changelog")
	if err != nil || got.Text != "update changelog" {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := s.EditTodoItem(changelog.ID, ""); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.EditTodoItem(99, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}

	got, err = s.SetTodoDone(changelog.ID, true)
	if err != nil || !got.Done || got.DoneAt == "" {
		t.Fatalf("%+v %v", got, err)
	}
	first := got.DoneAt
	again, _ := s.SetTodoDone(changelog.ID, true)
	if again.DoneAt != first {
		t.Fatalf("checking twice moved done_at: %q → %q", first, again.DoneAt)
	}
	got, _ = s.SetTodoDone(changelog.ID, false)
	if got.Done || got.DoneAt != "" {
		t.Fatalf("%+v", got)
	}
	if _, err := s.SetTodoDone(99, true); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestTodoItemsReorderAndMoveBetweenLists(t *testing.T) {
	s := openTemp(t)
	a, b := mustList(t, s, "A"), mustList(t, s, "B")
	one, two, three := mustItem(t, s, a.ID, "1"), mustItem(t, s, a.ID, "2"), mustItem(t, s, a.ID, "3")
	x := mustItem(t, s, b.ID, "x")

	if _, err := s.MoveTodoItem(three.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	if itemTexts(t, s, a.ID) != "3,1,2" {
		t.Fatal(itemTexts(t, s, a.ID))
	}
	if _, err := s.MoveTodoItem(three.ID, a.ID, 2); err != nil {
		t.Fatal(err)
	}
	if itemTexts(t, s, a.ID) != "1,2,3" {
		t.Fatal(itemTexts(t, s, a.ID))
	}
	if _, err := s.MoveTodoItem(one.ID, 0, 3); !errors.Is(err, ErrInvalid) {
		t.Fatalf("past the end of its own list: %v", err)
	}

	// Into another list: any index up to just past its end.
	got, err := s.MoveTodoItem(two.ID, b.ID, 1)
	if err != nil || got.ListID != b.ID {
		t.Fatalf("%+v %v", got, err)
	}
	if itemTexts(t, s, a.ID) != "1,3" || itemTexts(t, s, b.ID) != "x,2" {
		t.Fatal(itemTexts(t, s, a.ID), itemTexts(t, s, b.ID))
	}
	if _, err := s.MoveTodoItem(one.ID, b.ID, 0); err != nil {
		t.Fatal(err)
	}
	if itemTexts(t, s, a.ID) != "3" || itemTexts(t, s, b.ID) != "1,x,2" {
		t.Fatal(itemTexts(t, s, a.ID), itemTexts(t, s, b.ID))
	}
	if _, err := s.MoveTodoItem(x.ID, a.ID, 5); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if itemTexts(t, s, b.ID) != "1,x,2" {
		t.Fatal("a failed move must change nothing:", itemTexts(t, s, b.ID))
	}
	if _, err := s.MoveTodoItem(x.ID, 99, 0); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.MoveTodoItem(99, 0, 0); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestTodoDeletes(t *testing.T) {
	s := openTemp(t)
	a, b := mustList(t, s, "A"), mustList(t, s, "B")
	mustList(t, s, "C")
	one, two := mustItem(t, s, a.ID, "1"), mustItem(t, s, a.ID, "2")
	three := mustItem(t, s, a.ID, "3")

	if err := s.DeleteTodoItems(one.ID, 99); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if itemTexts(t, s, a.ID) != "1,2,3" {
		t.Fatal("a failed delete must change nothing:", itemTexts(t, s, a.ID))
	}
	if err := s.DeleteTodoItems(one.ID); err != nil {
		t.Fatal(err)
	}
	if itemTexts(t, s, a.ID) != "2,3" {
		t.Fatal(itemTexts(t, s, a.ID))
	}

	s.SetTodoDone(two.ID, true)
	if n, err := s.ClearDoneTodos(a.ID); err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	if itemTexts(t, s, a.ID) != "3" {
		t.Fatal(itemTexts(t, s, a.ID))
	}
	if _, err := s.ClearDoneTodos(99); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}

	// A list with items needs force; its items go with it.
	if err := s.DeleteTodoList(a.ID, false); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "--force") {
		t.Fatal(err)
	}
	if err := s.DeleteTodoList(a.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTodoItem(three.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("items should be deleted with their list:", err)
	}
	if err := s.DeleteTodoList(b.ID, false); err != nil {
		t.Fatal("an empty list needs no force:", err)
	}
	if listNames(t, s) != "C" {
		t.Fatal(listNames(t, s))
	}
	if err := s.DeleteTodoList(a.ID, true); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestTodoListsEmptyIsNotNil(t *testing.T) {
	s := openTemp(t)
	ls, err := s.TodoLists()
	if err != nil || ls == nil || len(ls) != 0 {
		t.Fatalf("%v %v", ls, err)
	}
}
