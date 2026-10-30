package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/akshat-dubey/tars/internal/store"
)

const minColWidth = 28

var (
	cardIdle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colMuted).Padding(0, 1)
	cardSel  = lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(colAccent).Padding(0, 1)
	helpKey  = lipgloss.NewStyle().Foreground(colAccent)
)

type board struct {
	st       *store.Store
	width    int
	height   int
	showDone bool
	columns  []string
	tasks    map[string][]*store.Task
	col      int
	row      map[string]int
	offset   map[string]int
	detail   bool
	vp       viewport.Model
	flash    string
	err      error
}

// RunBoard opens the interactive full-screen board.
func RunBoard(st *store.Store) error {
	b := &board{st: st, row: map[string]int{}, offset: map[string]int{}}
	if err := b.reload(); err != nil {
		return err
	}
	_, err := tea.NewProgram(b, tea.WithAltScreen()).Run()
	return err
}

func (b *board) reload() error {
	tasks, err := b.st.ListTasks(store.Filter{IncludeDone: b.showDone})
	if err != nil {
		return err
	}
	b.tasks = map[string][]*store.Task{}
	for _, t := range tasks {
		b.tasks[t.Status] = append(b.tasks[t.Status], t)
	}
	b.columns = nil
	for _, s := range store.Statuses {
		if s == store.StatusDone && !b.showDone {
			continue
		}
		b.columns = append(b.columns, s)
	}
	if b.col >= len(b.columns) {
		b.col = len(b.columns) - 1
	}
	for _, s := range b.columns {
		if n := len(b.tasks[s]); b.row[s] >= n {
			b.row[s] = max(n-1, 0)
		}
	}
	return nil
}

func (b *board) selected() *store.Task {
	s := b.columns[b.col]
	if len(b.tasks[s]) == 0 {
		return nil
	}
	return b.tasks[s][b.row[s]]
}

func (b *board) Init() tea.Cmd { return nil }

func (b *board) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.width, b.height = msg.Width, msg.Height
		b.vp.Width, b.vp.Height = msg.Width, msg.Height-2
		return b, nil
	case tea.KeyMsg:
		if b.detail {
			return b.updateDetail(msg)
		}
		return b.updateBoard(msg)
	}
	return b, nil
}

func (b *board) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter", "backspace":
		b.detail = false
		return b, nil
	case "ctrl+c":
		return b, tea.Quit
	}
	var cmd tea.Cmd
	b.vp, cmd = b.vp.Update(msg)
	return b, cmd
}

func (b *board) updateBoard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := b.columns[b.col]
	b.flash = ""
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return b, tea.Quit
	case "left", "h":
		if b.col > 0 {
			b.col--
		}
	case "right", "l":
		if b.col < len(b.columns)-1 {
			b.col++
		}
	case "up", "k":
		if b.row[s] > 0 {
			b.row[s]--
		}
	case "down", "j":
		if b.row[s] < len(b.tasks[s])-1 {
			b.row[s]++
		}
	case "g", "home":
		b.row[s] = 0
	case "G", "end":
		b.row[s] = max(len(b.tasks[s])-1, 0)
	case "enter", " ":
		if t := b.selected(); t != nil {
			full, err := b.st.GetTask(t.ID)
			if err != nil {
				b.err = err
				return b, nil
			}
			b.vp = viewport.New(b.width, b.height-2)
			b.vp.SetContent(Task(full, b.width))
			b.detail = true
		}
	case "d":
		b.move(store.StatusDone)
	case "]", "L", "shift+right":
		b.shift(+1)
	case "[", "H", "shift+left":
		b.shift(-1)
	case "+", "=":
		b.bumpPriority(-1)
	case "-":
		b.bumpPriority(+1)
	case "a":
		b.showDone = !b.showDone
		b.err = b.reload()
	case "r":
		b.err = b.reload()
		b.flash = "reloaded"
	}
	return b, nil
}

func (b *board) shift(dir int) {
	t := b.selected()
	if t == nil {
		return
	}
	for i, s := range store.Statuses {
		if s == t.Status {
			if j := i + dir; j >= 0 && j < len(store.Statuses) {
				b.move(store.Statuses[j])
			}
			return
		}
	}
}

func (b *board) move(status string) {
	t := b.selected()
	if t == nil || t.Status == status {
		return
	}
	from := t.Status
	t.Status = status
	if err := b.st.SaveTask(t); err != nil {
		b.err = err
		return
	}
	b.err = b.st.AddLog(t.ID, "user", "status", from+" → "+status)
	b.flash = fmt.Sprintf("#%d → %s", t.ID, status)
	if err := b.reload(); err != nil {
		b.err = err
	}
}

func (b *board) bumpPriority(d int) {
	t := b.selected()
	if t == nil {
		return
	}
	p := clampPriority(t.Priority + d)
	if p == t.Priority {
		return
	}
	t.Priority = p
	if err := b.st.SaveTask(t); err != nil {
		b.err = err
		return
	}
	b.flash = fmt.Sprintf("#%d priority → %s", t.ID, PriorityName(p))
	b.err = b.reload()
}

func (b *board) View() string {
	if b.width == 0 {
		return ""
	}
	if b.detail {
		return b.vp.View() + "\n" + b.footer("↑/↓ scroll", "esc back", "ctrl+c quit")
	}

	// Show as many columns as fit, scrolled so the cursor column is visible.
	visible := max(1, min(len(b.columns), b.width/minColWidth))
	start := 0
	if b.col >= visible {
		start = b.col - visible + 1
	}
	colW := b.width / visible
	bodyH := b.height - 4

	var cols []string
	for i := start; i < start+visible && i < len(b.columns); i++ {
		cols = append(cols, b.renderColumn(i, colW, bodyH))
	}
	header := " " + Accent.Render("TARS") + Muted.Render(" · Track · Assign · Run · Ship")
	if start > 0 || start+visible < len(b.columns) {
		header += Muted.Render(fmt.Sprintf("   columns %d–%d of %d", start+1, start+visible, len(b.columns)))
	}
	out := header + "\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	out = lipgloss.NewStyle().Height(b.height - 1).MaxHeight(b.height - 1).Render(out)

	status := b.footer("←→↑↓ move", "enter open", "[ ] status", "d done", "+/- priority", "a show done", "q quit")
	if b.err != nil {
		status = " " + Error(b.err.Error())
	} else if b.flash != "" {
		status = " " + Success(b.flash)
	}
	return out + "\n" + status
}

func (b *board) footer(items ...string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		k, rest, _ := strings.Cut(it, " ")
		parts[i] = helpKey.Render(k) + " " + Muted.Render(rest)
	}
	return " " + strings.Join(parts, Muted.Render("  ·  "))
}

func (b *board) renderColumn(i, w, h int) string {
	s := b.columns[i]
	tasks := b.tasks[s]
	focused := i == b.col

	head := StatusBadge(s) + Muted.Render(fmt.Sprintf(" %d", len(tasks)))
	if focused {
		head = lipgloss.NewStyle().Underline(true).Render(StatusBadge(s)) + Muted.Render(fmt.Sprintf(" %d", len(tasks)))
	}
	lines := []string{" " + head, ""}

	if len(tasks) == 0 {
		lines = append(lines, Muted.Render("  —"))
		return lipgloss.NewStyle().Width(w).Render(strings.Join(lines, "\n"))
	}

	sel := b.row[s]
	if sel < b.offset[s] {
		b.offset[s] = sel
	}
	avail := h - len(lines)
	for {
		cards, last := b.fitCards(tasks, b.offset[s], w, avail, focused, sel)
		if sel <= last || b.offset[s] >= sel {
			if b.offset[s] > 0 {
				lines = append(lines, Muted.Render(fmt.Sprintf("  ↑ %d more", b.offset[s])))
			}
			lines = append(lines, cards...)
			if rest := len(tasks) - 1 - last; rest > 0 {
				lines = append(lines, Muted.Render(fmt.Sprintf("  ↓ %d more", rest)))
			}
			break
		}
		b.offset[s]++
	}
	return lipgloss.NewStyle().Width(w).Render(strings.Join(lines, "\n"))
}

// fitCards renders cards from start until avail lines are used; returns them and the last index shown.
func (b *board) fitCards(tasks []*store.Task, start, w, avail int, focused bool, sel int) ([]string, int) {
	var out []string
	used, last := 0, start-1
	avail -= 2 // leave room for "more" indicators
	for i := start; i < len(tasks); i++ {
		c := card(tasks[i], w-1, focused && i == sel)
		hgt := lipgloss.Height(c)
		if used+hgt > avail && i > start {
			break
		}
		out = append(out, c)
		used += hgt
		last = i
	}
	return out, last
}

func card(t *store.Task, w int, selected bool) string {
	st := cardIdle
	if selected {
		st = cardSel
	}
	inner := w - 4
	top := Muted.Render(fmt.Sprintf("#%d", t.ID)) + " " + Priority(t.Priority)
	if t.Jira != "" {
		gap := inner - lipgloss.Width(top) - lipgloss.Width(t.Jira)
		top += strings.Repeat(" ", max(gap, 1)) + Jira(t.Jira)
	}

	titleSt := Bold
	if t.Status == store.StatusDone {
		titleSt = Muted.Strikethrough(true)
	}
	wrapped := strings.Split(lipgloss.NewStyle().Width(inner).Render(t.Title), "\n")
	if len(wrapped) > 2 {
		wrapped = wrapped[:2]
		wrapped[1] = Truncate(strings.TrimRight(wrapped[1], " ")+" …", inner)
	}
	for i := range wrapped {
		wrapped[i] = titleSt.Render(strings.TrimRight(wrapped[i], " "))
	}

	var meta []string
	if t.Repo != "" {
		meta = append(meta, Muted.Render(Truncate(t.Repo, inner/2)))
	}
	if t.Status == store.StatusBlocked && t.WaitingOn != "" {
		meta = append(meta, StatusStyle(store.StatusBlocked).Render("⏳ "+Truncate(t.WaitingOn, inner/2)))
	} else if t.Status != store.StatusDone && t.Due != "" {
		meta = append(meta, Due(t.Due))
	}

	body := top + "\n" + strings.Join(wrapped, "\n")
	if len(meta) > 0 {
		body += "\n" + strings.Join(meta, Muted.Render(" · "))
	}
	return st.Width(w - 2).Render(body)
}
