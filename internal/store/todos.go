package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits on what a todo list holds. They keep names usable as CLI arguments
// and stop a runaway agent from filling the database.
const (
	MaxTodoListName = 80
	MaxTodoText     = 500
	MaxTodoLists    = 200
	MaxTodoItems    = 1000 // per list
)

// TodoList is a named checklist. Lists and their items keep the order the
// user gives them.
type TodoList struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Position  int        `json:"position"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
	Items     []TodoItem `json:"items"`
}

type TodoItem struct {
	ID        int64  `json:"id"`
	ListID    int64  `json:"list_id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	Position  int    `json:"position"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	DoneAt    string `json:"done_at,omitempty"`
}

// ErrInvalid marks input the user can fix: an empty name, a duplicate, a
// position out of range.
var ErrInvalid = errors.New("invalid")

// invalidErr reads as its message alone but matches ErrInvalid.
type invalidErr struct{ msg string }

func (e invalidErr) Error() string        { return e.msg }
func (e invalidErr) Is(target error) bool { return target == ErrInvalid }

func invalid(format string, a ...any) error { return invalidErr{fmt.Sprintf(format, a...)} }

// CleanTodoListName trims a list name and checks it. Names are single line,
// unique ignoring case, and never all digits, so a CLI argument is either an
// id or a name and never both.
func CleanTodoListName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", invalid("a list needs a name")
	case utf8.RuneCountInString(name) > MaxTodoListName:
		return "", invalid("list name is longer than %d characters", MaxTodoListName)
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return "", invalid("list name can't contain line breaks or control characters")
	case strings.HasPrefix(name, "-"):
		return "", invalid("list name can't start with \"-\"")
	}
	if _, err := strconv.ParseInt(strings.TrimPrefix(name, "#"), 10, 64); err == nil {
		return "", invalid("list name can't be just a number")
	}
	return name, nil
}

// CleanTodoText trims an item's text and checks it. Items are one line.
func CleanTodoText(text string) (string, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return "", invalid("a todo needs some text")
	case utf8.RuneCountInString(text) > MaxTodoText:
		return "", invalid("todo is longer than %d characters", MaxTodoText)
	case strings.IndexFunc(text, unicode.IsControl) >= 0:
		return "", invalid("todo can't contain line breaks or control characters")
	}
	return text, nil
}

// TodoLists returns every list in order, each with its items in order.
func (s *Store) TodoLists() ([]*TodoList, error) {
	rows, err := s.db.Query(`SELECT id, name, position, created_at, updated_at FROM todo_lists ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	var out []*TodoList
	byID := map[int64]*TodoList{}
	for rows.Next() {
		l := &TodoList{Items: []TodoItem{}}
		if err := rows.Scan(&l.ID, &l.Name, &l.Position, &l.CreatedAt, &l.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, l)
		byID[l.ID] = l
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	items, err := s.todoItems(`SELECT ` + todoItemCols + ` FROM todo_items ORDER BY list_id, position, id`)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if l := byID[it.ListID]; l != nil {
			l.Items = append(l.Items, it)
		}
	}
	if out == nil {
		out = []*TodoList{}
	}
	return out, nil
}

// FindTodoList resolves a list by id ("3" or "#3") or by name, ignoring case.
func (s *Store) FindTodoList(ref string) (*TodoList, error) {
	ref = strings.TrimSpace(ref)
	if id, err := strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64); err == nil {
		return s.GetTodoList(id)
	}
	var id int64
	err := s.db.QueryRow(`SELECT id FROM todo_lists WHERE name = ?`, ref).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("todo list %q: %w", ref, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return s.GetTodoList(id)
}

func (s *Store) GetTodoList(id int64) (*TodoList, error) {
	l := &TodoList{}
	err := s.db.QueryRow(`SELECT id, name, position, created_at, updated_at FROM todo_lists WHERE id = ?`, id).
		Scan(&l.ID, &l.Name, &l.Position, &l.CreatedAt, &l.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("todo list %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	if l.Items, err = s.todoItems(`SELECT `+todoItemCols+` FROM todo_items WHERE list_id = ? ORDER BY position, id`, id); err != nil {
		return nil, err
	}
	if l.Items == nil {
		l.Items = []TodoItem{}
	}
	return l, nil
}

func (s *Store) GetTodoItem(id int64) (*TodoItem, error) {
	items, err := s.todoItems(`SELECT `+todoItemCols+` FROM todo_items WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("todo %d: %w", id, ErrNotFound)
	}
	return &items[0], nil
}

const todoItemCols = `id, list_id, text, done, position, created_at, updated_at, done_at`

func (s *Store) todoItems(query string, args ...any) ([]TodoItem, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TodoItem
	for rows.Next() {
		var it TodoItem
		if err := rows.Scan(&it.ID, &it.ListID, &it.Text, &it.Done, &it.Position, &it.CreatedAt, &it.UpdatedAt, &it.DoneAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// nameTaken reports whether another list (not except) already uses name.
func nameTaken(tx *sql.Tx, name string, except int64) error {
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM todo_lists WHERE name = ? AND id != ?`, name, except).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return invalid("there's already a todo list called %q", name)
	}
	return nil
}

// CreateTodoList adds an empty list at the end.
func (s *Store) CreateTodoList(name string) (*TodoList, error) {
	name, err := CleanTodoListName(name)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := nameTaken(tx, name, 0); err != nil {
		return nil, err
	}
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM todo_lists`).Scan(&n); err != nil {
		return nil, err
	}
	if n >= MaxTodoLists {
		return nil, invalid("you already have %d todo lists, the most there can be", MaxTodoLists)
	}
	ts := now()
	res, err := tx.Exec(`INSERT INTO todo_lists (name, position, created_at, updated_at) VALUES (?, ?, ?, ?)`, name, n, ts, ts)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTodoList(id)
}

func (s *Store) RenameTodoList(id int64, name string) (*TodoList, error) {
	name, err := CleanTodoListName(name)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := listExists(tx, id); err != nil {
		return nil, err
	}
	if err := nameTaken(tx, name, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE todo_lists SET name = ?, updated_at = ? WHERE id = ?`, name, now(), id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTodoList(id)
}

func listExists(tx *sql.Tx, id int64) error {
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM todo_lists WHERE id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("todo list %d: %w", id, ErrNotFound)
	}
	return nil
}

// MoveTodoList puts a list at index pos (0 is first) among all lists.
func (s *Store) MoveTodoList(id int64, pos int) (*TodoList, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	ids, err := idsIn(tx, `SELECT id FROM todo_lists ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	ids, err = moveID(ids, id, pos, "todo list")
	if err != nil {
		return nil, err
	}
	if err := renumber(tx, "todo_lists", ids); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTodoList(id)
}

// DeleteTodoList removes a list. One that still has items is kept unless
// force is set, so a stray command can't throw work away.
func (s *Store) DeleteTodoList(id int64, force bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := listExists(tx, id); err != nil {
		return err
	}
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM todo_items WHERE list_id = ?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 && !force {
		return invalid("todo list %d still has %d %s; delete with --force to remove them too", id, n, plural(n, "todo", "todos"))
	}
	if _, err := tx.Exec(`DELETE FROM todo_lists WHERE id = ?`, id); err != nil {
		return err
	}
	ids, err := idsIn(tx, `SELECT id FROM todo_lists ORDER BY position, id`)
	if err != nil {
		return err
	}
	if err := renumber(tx, "todo_lists", ids); err != nil {
		return err
	}
	return tx.Commit()
}

// AddTodoItem appends an unchecked item to the end of a list.
func (s *Store) AddTodoItem(listID int64, text string) (*TodoItem, error) {
	text, err := CleanTodoText(text)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := listExists(tx, listID); err != nil {
		return nil, err
	}
	var n int
	if err := tx.QueryRow(`SELECT count(*) FROM todo_items WHERE list_id = ?`, listID).Scan(&n); err != nil {
		return nil, err
	}
	if n >= MaxTodoItems {
		return nil, invalid("todo list %d already has %d todos, the most a list can hold", listID, MaxTodoItems)
	}
	ts := now()
	res, err := tx.Exec(`INSERT INTO todo_items (list_id, text, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		listID, text, n, ts, ts)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if err := touchList(tx, listID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTodoItem(id)
}

func (s *Store) EditTodoItem(id int64, text string) (*TodoItem, error) {
	text, err := CleanTodoText(text)
	if err != nil {
		return nil, err
	}
	return s.updateItem(id, `UPDATE todo_items SET text = ?, updated_at = ? WHERE id = ?`, text, now(), id)
}

// SetTodoDone checks or unchecks an item.
func (s *Store) SetTodoDone(id int64, done bool) (*TodoItem, error) {
	ts := now()
	doneAt := ""
	if done {
		doneAt = ts
	}
	// Checking an item that's already checked keeps when it was first done.
	return s.updateItem(id, `UPDATE todo_items SET done = ?, updated_at = ?,
		done_at = CASE WHEN ? AND done THEN done_at ELSE ? END WHERE id = ?`, done, ts, done, doneAt, id)
}

func (s *Store) updateItem(id int64, query string, args ...any) (*TodoItem, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var listID int64
	err = tx.QueryRow(`SELECT list_id FROM todo_items WHERE id = ?`, id).Scan(&listID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("todo %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(query, args...); err != nil {
		return nil, err
	}
	if err := touchList(tx, listID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTodoItem(id)
}

// MoveTodoItem puts an item at index pos (0 is first) in list toList, which
// is its own list when toList is 0. Moving to another list may also use the
// index just past its last item.
func (s *Store) MoveTodoItem(id, toList int64, pos int) (*TodoItem, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var from int64
	err = tx.QueryRow(`SELECT list_id FROM todo_items WHERE id = ?`, id).Scan(&from)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("todo %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	if toList == 0 {
		toList = from
	}
	if toList != from {
		if err := listExists(tx, toList); err != nil {
			return nil, err
		}
		var n int
		if err := tx.QueryRow(`SELECT count(*) FROM todo_items WHERE list_id = ?`, toList).Scan(&n); err != nil {
			return nil, err
		}
		if n >= MaxTodoItems {
			return nil, invalid("todo list %d already has %d todos, the most a list can hold", toList, MaxTodoItems)
		}
		if _, err := tx.Exec(`UPDATE todo_items SET list_id = ?, position = ?, updated_at = ? WHERE id = ?`, toList, n, now(), id); err != nil {
			return nil, err
		}
		left, err := idsIn(tx, `SELECT id FROM todo_items WHERE list_id = ? ORDER BY position, id`, from)
		if err != nil {
			return nil, err
		}
		if err := renumber(tx, "todo_items", left); err != nil {
			return nil, err
		}
		if err := touchList(tx, from); err != nil {
			return nil, err
		}
	}
	ids, err := idsIn(tx, `SELECT id FROM todo_items WHERE list_id = ? ORDER BY position, id`, toList)
	if err != nil {
		return nil, err
	}
	if ids, err = moveID(ids, id, pos, "todo"); err != nil {
		return nil, err
	}
	if err := renumber(tx, "todo_items", ids); err != nil {
		return nil, err
	}
	if err := touchList(tx, toList); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetTodoItem(id)
}

// DeleteTodoItems removes items, all or nothing.
func (s *Store) DeleteTodoItems(ids ...int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	lists := map[int64]bool{}
	for _, id := range ids {
		var listID int64
		err := tx.QueryRow(`SELECT list_id FROM todo_items WHERE id = ?`, id).Scan(&listID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("todo %d: %w", id, ErrNotFound)
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM todo_items WHERE id = ?`, id); err != nil {
			return err
		}
		lists[listID] = true
	}
	for l := range lists {
		if err := renumberItems(tx, l); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ClearDoneTodos removes a list's checked items and returns how many went.
func (s *Store) ClearDoneTodos(listID int64) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := listExists(tx, listID); err != nil {
		return 0, err
	}
	res, err := tx.Exec(`DELETE FROM todo_items WHERE list_id = ? AND done`, listID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if err := renumberItems(tx, listID); err != nil {
		return 0, err
	}
	return int(n), tx.Commit()
}

func renumberItems(tx *sql.Tx, listID int64) error {
	ids, err := idsIn(tx, `SELECT id FROM todo_items WHERE list_id = ? ORDER BY position, id`, listID)
	if err != nil {
		return err
	}
	if err := renumber(tx, "todo_items", ids); err != nil {
		return err
	}
	return touchList(tx, listID)
}

func touchList(tx *sql.Tx, id int64) error {
	_, err := tx.Exec(`UPDATE todo_lists SET updated_at = ? WHERE id = ?`, now(), id)
	return err
}

func idsIn(tx *sql.Tx, query string, args ...any) ([]int64, error) {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// moveID returns ids with id moved to index pos. pos must be a valid index.
func moveID(ids []int64, id int64, pos int, what string) ([]int64, error) {
	from := -1
	for i, x := range ids {
		if x == id {
			from = i
		}
	}
	if from < 0 {
		return nil, fmt.Errorf("%s %d: %w", what, id, ErrNotFound)
	}
	if pos < 0 || pos >= len(ids) {
		return nil, invalid("position %d is out of range: use 1 to %d", pos+1, len(ids))
	}
	out := append(append([]int64{}, ids[:from]...), ids[from+1:]...)
	out = append(out[:pos], append([]int64{id}, out[pos:]...)...)
	return out, nil
}

// renumber sets position = index for each id, in table (a fixed name).
func renumber(tx *sql.Tx, table string, ids []int64) error {
	stmt, err := tx.Prepare(`UPDATE ` + table + ` SET position = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, id := range ids {
		if _, err := stmt.Exec(i, id); err != nil {
			return err
		}
	}
	return nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
