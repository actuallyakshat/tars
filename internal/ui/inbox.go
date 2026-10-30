package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/actuallyakshat/tars/internal/claude"
	"github.com/actuallyakshat/tars/internal/parse"
	"github.com/actuallyakshat/tars/internal/store"
)

// splitMinWidth is the terminal width at which the detail opens beside the list.
const splitMinWidth = 120

// maxRowWidth keeps Jira, repo and due within easy reach of the title on very
// wide terminals instead of pinning them to the far edge.
const maxRowWidth = 120

const refreshEvery = 5 * time.Second

var (
	selBg    = lipgloss.AdaptiveColor{Light: "#ECEAFB", Dark: "#2B2840"}
	ruleSt   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#DDDDDD", Dark: "#3A3A3A"})
	groupSt  = lipgloss.NewStyle().Foreground(colMuted).Bold(true)
	tabSt    = lipgloss.NewStyle().Foreground(colMuted)
	tabOnSt  = lipgloss.NewStyle().Foreground(colAccent).Bold(true).Underline(true)
	unreadSt = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	textSt   = lipgloss.NewStyle().Foreground(colText)
)

// --- views and grouping ---

type view struct {
	name     string
	statuses []string // nil = every open (not done) task
	needsYou bool     // only tasks with an unanswered agent question
}

const (
	viewNeedsYou = 0
	viewOpen     = 1
)

var views = []view{
	{name: "Needs you", needsYou: true},
	{"Open", nil, false},
	{"Inbox", []string{store.StatusInbox}, false},
	{"Ready", []string{store.StatusReady}, false},
	{"Review", []string{store.StatusInReview}, false},
	{"Blocked", []string{store.StatusBlocked}, false},
	{"Done", []string{store.StatusDone}, false},
}

func (v view) has(t *store.Task) bool {
	if v.needsYou {
		return t.NeedsYou && t.Status != store.StatusDone
	}
	if v.statuses == nil {
		return t.Status != store.StatusDone
	}
	for _, s := range v.statuses {
		if t.Status == s {
			return true
		}
	}
	return false
}

type grouping int

const (
	byPriority grouping = iota
	byDue
	byStatus
	byRepo
	byNone
	groupingCount
)

func (g grouping) String() string {
	return [...]string{"priority", "due", "status", "repo", "none"}[g]
}

var statusOrder = map[string]int{
	store.StatusInReview: 0, store.StatusInProgress: 1, store.StatusReady: 2,
	store.StatusInbox: 3, store.StatusBlocked: 4, store.StatusDone: 5,
}

var statusLabel = map[string]string{
	store.StatusInReview: "In review", store.StatusInProgress: "In progress", store.StatusReady: "Ready",
	store.StatusInbox: "Inbox", store.StatusBlocked: "Blocked", store.StatusDone: "Done",
}

// groupOf returns the group label and its sort rank for t.
func groupOf(t *store.Task, g grouping, now time.Time) (string, int) {
	switch g {
	case byPriority:
		p := clampPriority(t.Priority)
		return [...]string{"Urgent", "High", "Normal", "Low"}[p], p
	case byDue:
		days, ok := daysUntil(t.Due, now)
		switch {
		case !ok:
			return "No date", 5
		case days < 0:
			return "Overdue", 0
		case days == 0:
			return "Today", 1
		case days == 1:
			return "Tomorrow", 2
		case days < 7:
			return "This week", 3
		default:
			return "Later", 4
		}
	case byStatus:
		return statusLabel[t.Status], statusOrder[t.Status]
	case byRepo:
		if t.Repo == "" {
			return "No repo", 1
		}
		return t.Repo, 0
	}
	return "", 0
}

func matches(t *store.Task, q string) bool {
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	hay := strings.ToLower(strings.Join([]string{t.Title, t.Jira, t.Repo, strings.Join(t.Tags, " "), fmt.Sprintf("#%d", t.ID)}, " "))
	return strings.Contains(hay, q)
}

// --- model ---

type mode int

const (
	modeList mode = iota
	modeSearch
	modeAdd
	modeNote
	modeStatus
	modePriority
	modeAnswer
)

type row struct {
	spacer bool
	header string
	count  int
	task   *store.Task
}

type undoEntry struct {
	before store.Task
	what   string
}

type tickMsg struct{}

type inbox struct {
	st  *store.Store
	now func() time.Time

	width, height int

	all    []*store.Task // every task, done included
	tasks  []*store.Task // current view, filtered and sorted in display order
	rows   []row
	counts []int
	unread []int

	view  int
	group grouping
	query string
	cur   int // index into tasks
	top   int // first visible row

	detail     bool
	showLog    bool
	vp         viewport.Model
	detailTask *store.Task
	dirty      bool

	mode  mode
	input textinput.Model
	help  bool

	flash    string
	flashErr bool
	undo     *undoEntry

	copy func(string) error // clipboard writer; swapped out in tests
}

// RunInbox opens the interactive task list.
func RunInbox(st *store.Store) error {
	m, err := newInbox(st)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newInbox(st *store.Store) (*inbox, error) {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 500
	m := &inbox{st: st, now: time.Now, input: ti, vp: viewport.New(0, 0), view: viewOpen, copy: clipboard.WriteAll}
	if err := m.reload(); err != nil {
		return nil, err
	}
	// Open on the user's open decisions when there are any.
	if m.counts[viewNeedsYou] > 0 {
		m.setView(viewNeedsYou)
	}
	return m, nil
}

func (m *inbox) selected() *store.Task {
	if m.cur < 0 || m.cur >= len(m.tasks) {
		return nil
	}
	return m.tasks[m.cur]
}

// reload refetches everything and rebuilds the list, keeping the selection on
// the same task when it is still visible.
func (m *inbox) reload() error {
	var keep int64
	if t := m.selected(); t != nil {
		keep = t.ID
	}
	all, err := m.st.ListTasks(store.Filter{IncludeDone: true})
	if err != nil {
		return err
	}
	m.all = all
	m.counts = make([]int, len(views))
	m.unread = make([]int, len(views))
	for _, t := range all {
		for i, v := range views {
			if v.has(t) {
				m.counts[i]++
				if t.Unread {
					m.unread[i]++
				}
			}
		}
	}
	m.rebuild(keep)
	m.dirty = true
	return nil
}

// rebuild filters, groups and sorts the current view without hitting the store.
func (m *inbox) rebuild(keep int64) {
	now := m.now()
	type item struct {
		t     *store.Task
		label string
		rank  int
	}
	var items []item
	for _, t := range m.all {
		if views[m.view].has(t) && matches(t, m.query) {
			label, rank := groupOf(t, m.group, now)
			items = append(items, item{t, label, rank})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.label != b.label {
			return a.label < b.label
		}
		if a.t.Priority != b.t.Priority {
			return a.t.Priority < b.t.Priority
		}
		if (a.t.Due == "") != (b.t.Due == "") {
			return a.t.Due != ""
		}
		if a.t.Due != b.t.Due {
			return a.t.Due < b.t.Due
		}
		if m.view == len(views)-1 { // Done: most recently finished first
			return a.t.DoneAt > b.t.DoneAt
		}
		return a.t.ID < b.t.ID
	})

	m.tasks = m.tasks[:0]
	m.rows = m.rows[:0]
	for i, it := range items {
		if m.group != byNone && (i == 0 || it.label != items[i-1].label) {
			if i > 0 {
				m.rows = append(m.rows, row{spacer: true})
			}
			n := 0
			for _, x := range items[i:] {
				if x.label != it.label {
					break
				}
				n++
			}
			m.rows = append(m.rows, row{header: it.label, count: n})
		}
		m.rows = append(m.rows, row{task: it.t})
		m.tasks = append(m.tasks, it.t)
	}

	m.cur = min(m.cur, len(m.tasks)-1)
	for i, t := range m.tasks {
		if t.ID == keep {
			m.cur = i
			break
		}
	}
	m.cur = max(m.cur, 0)
	if len(m.tasks) == 0 {
		m.detail = false
	}
}

// --- layout ---

func (m *inbox) split() bool { return m.detail && m.width >= splitMinWidth }

func (m *inbox) bodyHeight() int { return max(m.height-3, 1) } // top bar, rule, footer

func (m *inbox) listWidth() int {
	if m.split() {
		return max(52, m.width*45/100)
	}
	return m.width
}

func (m *inbox) detailWidth() int {
	if m.split() {
		return m.width - m.listWidth() - 1 // divider
	}
	return m.width
}

// syncDetail loads the selected task into the detail pane and marks it seen.
func (m *inbox) syncDetail() {
	m.vp.Width, m.vp.Height = max(m.detailWidth()-2, 10), m.bodyHeight()
	if !m.detail {
		m.detailTask = nil
		return
	}
	t := m.selected()
	if t == nil {
		m.detail = false
		return
	}
	changed := m.detailTask == nil || m.detailTask.ID != t.ID
	if !changed && !m.dirty {
		return
	}
	full, err := m.st.GetTask(t.ID)
	if err != nil {
		m.setErr(err)
		return
	}
	if full.Unread {
		if err := m.st.MarkSeen(full.ID); err == nil {
			full.Unread, t.Unread = false, false
			for i, v := range views {
				if v.has(t) {
					m.unread[i]--
				}
			}
		}
	}
	m.detailTask = full
	m.vp.SetContent(detailContent(full, m.vp.Width, m.showLog, m.now()))
	if changed {
		m.vp.GotoTop()
	}
	m.dirty = false
}

// --- update ---

func (m *inbox) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *inbox) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(m.width-30, 10)
		m.dirty = true
	case tickMsg:
		// Pick up changes agents made while the TUI is open.
		if m.mode == modeList {
			if err := m.reload(); err != nil {
				m.setErr(err)
			}
		}
		cmd = tick()
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeSearch, modeAdd, modeNote, modeAnswer:
			cmd = m.updateInput(msg)
		case modeStatus, modePriority:
			m.updatePicker(msg)
		default:
			if m.help {
				m.help = false
				break
			}
			if q := m.updateList(msg); q != nil {
				return m, q
			}
		}
	}
	m.syncDetail()
	return m, cmd
}

func (m *inbox) move(d int) {
	if len(m.tasks) == 0 {
		return
	}
	m.cur = max(0, min(len(m.tasks)-1, m.cur+d))
}

func (m *inbox) setView(i int) {
	m.view = (i + len(views)) % len(views)
	m.cur = 0
	m.top = 0
	m.rebuild(0)
}

func (m *inbox) setErr(err error) { m.flash, m.flashErr = err.Error(), true }

func (m *inbox) say(format string, args ...any) {
	m.flash, m.flashErr = fmt.Sprintf(format, args...), false
}

func (m *inbox) updateList(msg tea.KeyMsg) tea.Cmd {
	m.flash = ""
	key := msg.String()
	fullDetail := m.detail && !m.split()

	// Scrolling the detail pane.
	if m.detail {
		switch key {
		case "pgdown", "ctrl+d", " ":
			m.vp.HalfPageDown()
			return nil
		case "pgup", "ctrl+u":
			m.vp.HalfPageUp()
			return nil
		}
		if fullDetail {
			switch key {
			case "down":
				m.vp.ScrollDown(1)
				return nil
			case "up":
				m.vp.ScrollUp(1)
				return nil
			}
		}
	}

	switch key {
	case "q":
		return tea.Quit
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(+1)
	case "home":
		m.cur = 0
	case "end":
		m.cur = max(len(m.tasks)-1, 0)
	case "pgdown":
		m.move(m.bodyHeight() - 1)
	case "pgup":
		m.move(-(m.bodyHeight() - 1))
	case "enter":
		if m.detail && m.split() {
			m.detail = false
		} else if m.selected() != nil {
			m.detail = true
		}
	case "esc", "backspace", "left", "h":
		switch {
		case m.detail:
			m.detail = false
		case key == "esc" && m.query != "":
			m.query = ""
			m.rebuild(m.selectedID())
		}
	case "tab":
		m.setView(m.view + 1)
	case "shift+tab":
		m.setView(m.view - 1)
	case "1", "2", "3", "4", "5", "6", "7":
		m.setView(int(key[0] - '1'))
	case "g":
		m.group = (m.group + 1) % groupingCount
		m.rebuild(m.selectedID())
		m.say("Grouped by %s", m.group)
	case "l":
		m.showLog = !m.showLog
		m.dirty = true
		if !m.detail && m.selected() != nil {
			m.detail = true
		}
	case "/":
		return m.startInput(modeSearch, m.query, "type to filter")
	case "c":
		return m.startInput(modeAdd, "", "Fix export BILL-881 !high @billing #perf due:fri")
	case "a":
		t := m.selected()
		switch {
		case t == nil:
		default:
			// An answered question can be answered again: the box starts with
			// the current answer, to change it.
			full, err := m.st.GetTask(t.ID)
			if err != nil || full.Report == nil || full.Report.NeedsYou == "" {
				m.say("#%d has no open question", t.ID)
				break
			}
			return m.startInput(modeAnswer, full.Report.Answer, Truncate(full.Report.NeedsYou, 80))
		}
	case "o":
		m.copyResume()
	case "n":
		if m.selected() != nil {
			return m.startInput(modeNote, "", "what happened, what's next…")
		}
	case "x":
		m.toggleDone()
	case "s":
		if m.selected() != nil {
			m.mode = modeStatus
		}
	case "p":
		if m.selected() != nil {
			m.mode = modePriority
		}
	case "u":
		m.doUndo()
	case "r":
		if err := m.reload(); err != nil {
			m.setErr(err)
		} else {
			m.say("Refreshed")
		}
	case "?":
		m.help = true
	}
	return nil
}

func (m *inbox) selectedID() int64 {
	if t := m.selected(); t != nil {
		return t.ID
	}
	return 0
}

func (m *inbox) startInput(md mode, value, placeholder string) tea.Cmd {
	m.mode = md
	m.input.Placeholder = placeholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m *inbox) endInput() {
	m.mode = modeList
	m.input.Blur()
	m.input.Reset()
}

func (m *inbox) updateInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		if m.mode == modeSearch {
			m.query = ""
			m.rebuild(m.selectedID())
		}
		m.flash = ""
		m.endInput()
		return nil
	case "enter":
		m.submitInput()
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.mode == modeSearch {
		m.query = strings.TrimSpace(m.input.Value())
		m.cur, m.top = 0, 0
		m.rebuild(0)
	}
	return cmd
}

func (m *inbox) submitInput() {
	val := strings.TrimSpace(m.input.Value())
	switch m.mode {
	case modeSearch:
		m.endInput()
		if val != "" {
			m.say("%d match %q · esc clears", len(m.tasks), val)
		}
	case modeAdd:
		if val == "" {
			m.endInput()
			return
		}
		q, err := parse.QuickAdd(val, m.now())
		if err != nil {
			m.setErr(err) // keep the prompt open so the text can be fixed
			return
		}
		t := &store.Task{Title: q.Title, Jira: q.Jira, Repo: q.Repo, Tags: q.Tags, Due: q.Due, Priority: q.Priority}
		if err := m.st.CreateTask(t); err != nil {
			m.setErr(err)
			return
		}
		m.endInput()
		if !views[m.view].has(t) {
			m.setView(viewOpen)
		}
		m.query = ""
		if err := m.reload(); err != nil {
			m.setErr(err)
			return
		}
		m.rebuild(t.ID)
		m.say("Added #%d to Inbox", t.ID)
	case modeAnswer:
		t := m.selected()
		m.endInput()
		if t == nil || val == "" {
			return
		}
		if err := m.st.Answer(t.ID, val, "user"); err != nil {
			m.setErr(err)
			return
		}
		m.afterChange(t.ID)
		m.say("Answered #%d. The next agent on it will see your answer.", t.ID)
	case modeNote:
		t := m.selected()
		m.endInput()
		if t == nil || val == "" {
			return
		}
		if err := m.st.AddLog(t.ID, "user", "note", val); err != nil {
			m.setErr(err)
			return
		}
		m.dirty = true
		m.say("Noted on #%d", t.ID)
	}
}

func (m *inbox) updatePicker(msg tea.KeyMsg) {
	key := msg.String()
	pickStatus := m.mode == modeStatus
	m.mode = modeList
	if key == "esc" || len(key) != 1 || key[0] < '1' {
		return
	}
	n := int(key[0] - '1')
	t := m.selected()
	if t == nil {
		return
	}
	if pickStatus && n < len(store.Statuses) {
		m.changeStatus(t, store.Statuses[n])
	} else if !pickStatus && n < 4 {
		m.changePriority(t, n)
	}
}

// snapshot copies t so it can be restored by undo.
func snapshot(t *store.Task) store.Task {
	c := *t
	c.Tags = append([]string(nil), t.Tags...)
	c.Log = nil
	return c
}

func (m *inbox) changeStatus(t *store.Task, status string) {
	if t.Status == status {
		return
	}
	before := snapshot(t)
	if err := m.st.SetStatus(t, status, "user"); err != nil {
		m.setErr(err)
		return
	}
	m.undo = &undoEntry{before: before, what: fmt.Sprintf("#%d → %s", t.ID, status)}
	m.afterChange(t.ID)
	m.say("#%d → %s · u to undo", t.ID, statusLabel[status])
}

func (m *inbox) changePriority(t *store.Task, p int) {
	if t.Priority == p {
		return
	}
	before := snapshot(t)
	t.Priority = p
	if err := m.st.SaveTask(t); err != nil {
		m.setErr(err)
		return
	}
	m.undo = &undoEntry{before: before, what: fmt.Sprintf("#%d priority", t.ID)}
	m.afterChange(t.ID)
	m.say("#%d priority → %s · u to undo", t.ID, PriorityName(p))
}

func (m *inbox) toggleDone() {
	t := m.selected()
	if t == nil {
		return
	}
	if t.Status == store.StatusDone {
		m.changeStatus(t, store.StatusInbox)
		return
	}
	m.changeStatus(t, store.StatusDone)
}

// afterChange reloads, keeping the cursor on the changed task if it is still in
// view, otherwise on the task that took its place.
func (m *inbox) afterChange(id int64) {
	pos := m.cur
	if err := m.reload(); err != nil {
		m.setErr(err)
		return
	}
	for i, t := range m.tasks {
		if t.ID == id {
			m.cur = i
			return
		}
	}
	m.cur = max(0, min(pos, len(m.tasks)-1))
}

// copyResume copies the command that reopens the latest Claude session that
// worked on the selected task.
func (m *inbox) copyResume() {
	t := m.selected()
	if t == nil {
		return
	}
	ss, err := m.st.TaskSessions(t.ID)
	if err != nil {
		m.setErr(err)
		return
	}
	if len(ss) == 0 {
		m.say("No Claude session is linked to #%d yet", t.ID)
		return
	}
	cmd := claude.ResumeCommand(ss[0].ID)
	if err := m.copy(cmd); err != nil {
		m.setErr(fmt.Errorf("couldn't copy to the clipboard: %w. Run: %s", err, cmd))
		return
	}
	m.say("Copied: %s", cmd)
}

func (m *inbox) doUndo() {
	if m.undo == nil {
		m.say("Nothing to undo")
		return
	}
	u := m.undo
	m.undo = nil
	restored := u.before
	if err := m.st.SaveTask(&restored); err != nil {
		m.setErr(err)
		return
	}
	if err := m.st.AddLog(restored.ID, "user", "undo", "undid "+u.what); err != nil {
		m.setErr(err)
		return
	}
	m.afterChange(restored.ID)
	m.say("Undid %s", u.what)
}

// --- view ---

func (m *inbox) View() string {
	if m.width == 0 {
		return ""
	}
	bodyH := m.bodyHeight()
	var body string
	switch {
	case m.help:
		body = helpView(m.width, bodyH)
	case m.split():
		// Fix the list's width so the divider never shifts with short content.
		list := lipgloss.NewStyle().Width(m.listWidth()).Render(m.listView(m.listWidth(), bodyH))
		div := ruleSt.Render(strings.Repeat("│\n", bodyH-1) + "│")
		det := lipgloss.NewStyle().PaddingLeft(1).Render(m.vp.View())
		body = lipgloss.JoinHorizontal(lipgloss.Top, list, div, det)
	case m.detail:
		body = lipgloss.NewStyle().PaddingLeft(1).Render(m.vp.View())
	default:
		body = m.listView(m.width, bodyH)
	}
	body = lipgloss.NewStyle().Width(m.width).Height(bodyH).MaxHeight(bodyH).Render(body)
	footer := ansi.Truncate(m.footer(), m.width, "…") // key hints can outgrow narrow terminals
	return m.topBar() + "\n" + ruleSt.Render(strings.Repeat("─", m.width)) + "\n" + body + "\n" + footer
}

func (m *inbox) topBar() string {
	left := " " + Accent.Render("TARS") + "  "
	for i, v := range views {
		label := fmt.Sprintf("%s %d", v.name, m.counts[i])
		st := tabSt
		if i == m.view {
			st = tabOnSt
		}
		left += st.Render(label)
		// A space before the dot, and the same slot reserved when there is no
		// dot, so tabs neither crowd it nor shift when it appears.
		if m.unread[i] > 0 && i != len(views)-1 {
			left += " " + unreadSt.Render("●")
		} else {
			left += "  "
		}
		left += "   "
	}
	right := "group: " + m.group.String()
	if m.query != "" {
		right = "/" + Truncate(m.query, 20) + "  " + right
	}
	right = Muted.Render(right) + " "
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return ansi.Truncate(left, m.width, "…") // left is styled; runewidth would cut escapes
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *inbox) footer() string {
	t := m.selected()
	switch m.mode {
	case modeSearch:
		return " " + Accent.Render("/") + " " + m.input.View()
	case modeAdd, modeNote, modeAnswer:
		label := "New task"
		switch {
		case m.mode == modeNote && t != nil:
			label = fmt.Sprintf("Note on #%d", t.ID)
		case m.mode == modeAnswer && t != nil:
			label = fmt.Sprintf("Answer #%d", t.ID)
		}
		out := " " + Accent.Render(label+":") + " " + m.input.View()
		if m.flashErr && m.flash != "" {
			out += "  " + Error(m.flash)
		}
		return out
	case modeStatus:
		parts := make([]string, len(store.Statuses))
		for i, s := range store.Statuses {
			parts[i] = helpKey.Render(fmt.Sprint(i+1)) + " " + StatusStyle(s).Render(statusIcon[s]+" "+s)
		}
		return fmt.Sprintf(" Move #%d to  ", t.ID) + strings.Join(parts, "  ") + Muted.Render("   esc cancel")
	case modePriority:
		var parts []string
		for p := 0; p < 4; p++ {
			parts = append(parts, helpKey.Render(fmt.Sprint(p+1))+" "+PriorityName(p))
		}
		return fmt.Sprintf(" Priority of #%d  ", t.ID) + strings.Join(parts, "  ") + Muted.Render("   esc cancel")
	}
	if m.flash != "" {
		if m.flashErr {
			return " " + Error(m.flash)
		}
		return " " + Success(m.flash)
	}
	if m.help {
		return keyHints("any key", "close help")
	}
	if m.detail {
		return keyHints("j/k", "next/prev", "↑↓", "scroll", "a", "answer", "o", "copy resume", "l", "activity", "n", "note", "x", "done", "esc", "close", "?", "help")
	}
	return keyHints("↑↓", "move", "⏎", "open", "x", "done", "s", "status", "c", "new", "/", "search", "tab", "view", "g", "group", "?", "help")
}

var helpKey = lipgloss.NewStyle().Foreground(colAccent)

func keyHints(kv ...string) string {
	var parts []string
	for i := 0; i+1 < len(kv); i += 2 {
		parts = append(parts, helpKey.Render(kv[i])+" "+Muted.Render(kv[i+1]))
	}
	return " " + strings.Join(parts, "   ")
}

func (m *inbox) listView(width, height int) string {
	if len(m.rows) == 0 {
		msg := "Nothing here."
		switch {
		case m.query != "":
			msg = fmt.Sprintf("No tasks match %q. esc clears the search.", m.query)
		case m.view == viewNeedsYou:
			msg = "Nothing needs you. When an agent gets stuck, its question shows up here."
		case m.counts[viewOpen] == 0 && m.view == viewOpen:
			msg = "No open tasks. Press c to add one."
		}
		return "\n  " + Muted.Render(msg)
	}

	// Keep the cursor visible, and its group header too when scrolling up.
	curRow := 0
	for i, r := range m.rows {
		if r.task != nil && r.task == m.selected() {
			curRow = i
			break
		}
	}
	if curRow < m.top {
		m.top = curRow
		for m.top > 0 && m.rows[m.top-1].task == nil && !m.rows[m.top-1].spacer {
			m.top-- // show the group header above the cursor
		}
	}
	if curRow >= m.top+height {
		m.top = curRow - height + 1
	}
	m.top = max(0, min(m.top, len(m.rows)-1))

	cols := m.columns(width)
	lines := make([]string, 0, height)
	for _, r := range m.rows[m.top:min(len(m.rows), m.top+height)] {
		switch {
		case r.spacer:
			lines = append(lines, "")
			continue
		case r.task == nil:
			lines = append(lines, " "+groupSt.Render(strings.ToUpper(r.header))+Muted.Render(fmt.Sprintf(" · %d", r.count)))
			continue
		}
		lines = append(lines, cols.render(r.task, r.task == m.selected(), m.now()))
	}
	return strings.Join(lines, "\n")
}

// columns holds the widths of the right-aligned metadata for the visible list.
type columns struct {
	width, idW, jiraW, repoW, whenW int
}

func (m *inbox) columns(width int) columns {
	c := columns{width: min(width, maxRowWidth), idW: 2, whenW: 8}
	for _, t := range m.tasks {
		c.idW = max(c.idW, len(fmt.Sprintf("#%d", t.ID)))
		c.jiraW = max(c.jiraW, min(runewidth.StringWidth(t.Jira), 12))
		c.repoW = max(c.repoW, min(runewidth.StringWidth(t.Repo), 14))
	}
	// Narrow lists drop metadata, least important first.
	if width < 90 {
		c.repoW = 0
	}
	if width < 64 {
		c.jiraW = 0
	}
	return c
}

func fit(s string, w int) string {
	return runewidth.FillRight(Truncate(s, w), w)
}

// render draws one task row. Every segment is styled from plain text exactly
// once so the selection background never splits an escape sequence.
func (c columns) render(t *store.Task, sel bool, now time.Time) string {
	paint := func(st lipgloss.Style, s string) string {
		if sel {
			st = st.Background(selBg)
		}
		return st.Render(s)
	}
	plain := lipgloss.NewStyle()

	var b strings.Builder
	if sel {
		b.WriteString(paint(Accent, "▌"))
	} else {
		b.WriteString(" ")
	}
	if t.NeedsYou {
		b.WriteString(paint(needsSt, "?"))
	} else if t.Unread {
		b.WriteString(paint(unreadSt, "●"))
	} else {
		b.WriteString(paint(plain, " "))
	}
	b.WriteString(paint(plain, " "))
	ptext, pst := priorityMark(t.Priority)
	b.WriteString(paint(pst, ptext))
	b.WriteString(paint(plain, " "))
	b.WriteString(paint(StatusStyle(t.Status), statusIcon[t.Status]))
	b.WriteString(paint(plain, " "))
	b.WriteString(paint(Muted, fmt.Sprintf("%-*s", c.idW, fmt.Sprintf("#%d", t.ID))))
	b.WriteString(paint(plain, "  "))

	var right strings.Builder
	rightW := 0
	addCol := func(st lipgloss.Style, s string, w int) {
		if w == 0 {
			return
		}
		right.WriteString(paint(plain, "  "))
		right.WriteString(paint(st, fit(s, w)))
		rightW += w + 2
	}
	addCol(jiraSt, t.Jira, c.jiraW)
	addCol(Muted, t.Repo, c.repoW)
	when, wst := dueText(t.Due, now)
	if t.Status == store.StatusDone {
		when, wst = shortAgo(t.DoneAt, now), Muted
	}
	right.WriteString(paint(plain, "  "))
	right.WriteString(paint(wst, runewidth.FillLeft(Truncate(when, c.whenW), c.whenW)))
	right.WriteString(paint(plain, " "))
	rightW += c.whenW + 3

	leftW := 3 + 2 + 1 + 1 + 1 + c.idW + 2
	titleW := max(c.width-leftW-rightW, 8)
	tst := textSt
	switch {
	case t.Status == store.StatusDone:
		tst = Muted
	case t.Unread:
		tst = Bold
	}
	title := Truncate(t.Title, titleW)
	used := runewidth.StringWidth(title)
	b.WriteString(paint(tst, title))
	if t.Status == store.StatusBlocked && t.WaitingOn != "" {
		if room := titleW - used - 2; room >= 8 {
			suffix := Truncate("waiting on "+t.WaitingOn, room)
			b.WriteString(paint(plain, "  "))
			b.WriteString(paint(StatusStyle(store.StatusBlocked), suffix))
			used += 2 + runewidth.StringWidth(suffix)
		}
	}
	b.WriteString(paint(plain, strings.Repeat(" ", max(titleW-used, 0))))
	b.WriteString(right.String())
	return b.String()
}

func shortAgo(ts string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", max(int(d.Minutes()), 1))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Local().Format("Jan 2")
}

// detailContent renders the reading pane for one task.
func detailContent(t *store.Task, width int, showLog bool, now time.Time) string {
	width = max(width, 20)
	wrap := lipgloss.NewStyle().Width(width)
	var b strings.Builder

	ptext, pst := priorityMark(t.Priority)
	head := Muted.Render(fmt.Sprintf("#%d", t.ID)) + "  " + StatusBadge(t.Status)
	if t.Priority != 2 {
		head += "  " + pst.Render(strings.TrimSpace(ptext)+" "+PriorityName(t.Priority))
	}
	b.WriteString(head + "\n\n")
	b.WriteString(wrap.Inherit(Bold).Render(t.Title) + "\n\n")
	if r := t.Report; r != nil {
		b.WriteString(groupSt.Render("REPORT") + Muted.Render("  "+Ago(r.At)) + "\n")
		b.WriteString(reportLines(r, width))
		if r.NeedsYou != "" && r.AnsweredAt == "" {
			b.WriteString(Muted.Render("          press ") + helpKey.Render("a") + Muted.Render(" to answer") + "\n")
		}
		b.WriteString("\n")
	}

	field := func(label, value string) {
		if value != "" {
			b.WriteString(Muted.Render(fmt.Sprintf("%-10s", label)) + value + "\n")
		}
	}
	field("jira", Jira(t.Jira))
	field("repo", t.Repo)
	if t.Due != "" {
		text, st := dueText(t.Due, now)
		field("due", t.Due+"  "+st.Render(text))
	}
	if len(t.Tags) > 0 {
		field("tags", tagSt.Render("#"+strings.Join(t.Tags, "  #")))
	}
	field("waiting", t.WaitingOn)
	if t.FollowUp != "" {
		text, st := dueText(t.FollowUp, now)
		field("follow up", t.FollowUp+"  "+st.Render(text))
	}
	field("branch", t.Branch)
	field("worktree", t.Worktree)
	b.WriteString(Muted.Render(fmt.Sprintf("%-10s", "created")+Ago(t.CreatedAt)+" · updated "+Ago(t.UpdatedAt)) + "\n")

	section := func(title, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		b.WriteString("\n" + groupSt.Render(strings.ToUpper(title)) + "\n")
		b.WriteString(wrap.Render(strings.TrimSpace(text)) + "\n")
	}
	section("Description", t.Body)
	section("Acceptance criteria", t.Acceptance)
	if len(t.Sessions) > 0 {
		b.WriteString("\n" + groupSt.Render("SESSIONS") + "  " + helpKey.Render("o") + Muted.Render(" copies the latest resume command") + "\n")
		for _, ss := range t.Sessions {
			name := ss.Name
			if name == "" {
				name = "Unnamed session"
			}
			b.WriteString(textSt.Render(Truncate(name, width-14)) + Muted.Render("  "+Ago(ss.LastAt)) + "\n")
			b.WriteString(Muted.Render("  claude --resume "+ss.ID) + "\n")
		}
	}

	b.WriteString("\n")
	if len(t.Log) == 0 {
		b.WriteString(Muted.Render("No activity yet") + "\n")
		return b.String()
	}
	last := t.Log[len(t.Log)-1]
	if !showLog {
		summary := fmt.Sprintf(" · %d %s · last: %s · %s · %s", len(t.Log), plural(len(t.Log), "entry", "entries"), last.Author, last.Kind, Ago(last.At))
		b.WriteString(groupSt.Render("▸ ACTIVITY") + Muted.Render(summary) + "  " + helpKey.Render("l") + Muted.Render(" expand") + "\n")
		return b.String()
	}
	b.WriteString(groupSt.Render("▾ ACTIVITY") + "  " + helpKey.Render("l") + Muted.Render(" collapse") + "\n")
	for i := len(t.Log) - 1; i >= 0; i-- { // newest first
		e := t.Log[i]
		who := Muted.Render(e.Author)
		if e.Author == "agent" {
			who = Accent.Render("agent")
		}
		b.WriteString("\n" + who + Muted.Render(" · "+e.Kind+" · "+Ago(e.At)) + "\n")
		b.WriteString(wrap.PaddingLeft(2).Render(strings.TrimSpace(e.Text)) + "\n")
	}
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func helpView(width, height int) string {
	type section struct {
		title string
		keys  []string
	}
	render := func(secs ...section) string {
		var b strings.Builder
		for _, s := range secs {
			b.WriteString("  " + groupSt.Render(strings.ToUpper(s.title)) + "\n")
			for i := 0; i+1 < len(s.keys); i += 2 {
				b.WriteString("    " + helpKey.Render(fmt.Sprintf("%-14s", s.keys[i])) + textSt.Render(s.keys[i+1]) + "\n")
			}
			b.WriteString("\n")
		}
		return b.String()
	}
	nav := section{"Navigate", []string{"↑↓ j k", "move", "home end", "first / last", "tab ⇧tab", "next / previous view", "1–7", "jump to view", "enter", "open / close details", "esc ← h", "back, clear search"}}
	details := section{"Details", []string{"↑↓ pgup pgdn", "scroll", "j k", "next / previous task", "l", "expand / collapse activity"}}
	act := section{"Selected task", []string{"a", "answer the agent's question", "o", "copy claude --resume command", "x", "done (again: reopen)", "s", "change status", "p", "change priority", "n", "add a note", "u", "undo last change"}}
	list := section{"List", []string{"c", "new task (quick add)", "/", "search", "g", "group: priority · due · status · repo", "r", "refresh", "q", "quit"}}

	var keys string
	if width >= 100 {
		trim := func(s string) string { return strings.TrimRight(s, "\n") }
		left := lipgloss.NewStyle().Width(48).Render(trim(render(nav, details)))
		keys = lipgloss.JoinHorizontal(lipgloss.Top, left, trim(render(act, list)))
	} else {
		keys = render(nav, act, details, list)
	}
	quick := "  " + groupSt.Render("QUICK ADD") + "\n" +
		"    " + textSt.Render("Fix export BILL-881 !high @billing #perf due:fri") + "\n" +
		"    " + Muted.Render("!urgent !high !low · @repo · #tag · due:<date> or ^<date> · ABC-123 = Jira") + "\n\n" +
		"  " + unreadSt.Render("●") + Muted.Render(" = the agent updated this task since you last opened it") + "\n"
	return "\n" + strings.TrimRight(keys, "\n") + "\n\n" + quick
}
