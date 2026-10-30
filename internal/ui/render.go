package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/actuallyakshat/tars/internal/store"
)

var (
	cardSt = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colAccent).
		Padding(0, 1)
	headingSt = lipgloss.NewStyle().Foreground(colAccent).Bold(true).MarginTop(1)
	tagSt     = lipgloss.NewStyle().Foreground(colAccent)
)

// Row renders one task on a single line, fitting width.
func Row(t *store.Task, width int, idWidth int) string {
	id := Muted.Render(fmt.Sprintf("%*s", idWidth, fmt.Sprintf("#%d", t.ID)))
	prefix := id + "  " + Priority(t.Priority) + " "

	var meta []string
	if t.Jira != "" {
		meta = append(meta, Jira(t.Jira))
	}
	if t.Repo != "" {
		meta = append(meta, Muted.Render(t.Repo))
	}
	for _, tag := range t.Tags {
		meta = append(meta, tagSt.Render("#"+tag))
	}
	if t.Status == store.StatusBlocked && t.WaitingOn != "" {
		meta = append(meta, StatusStyle(store.StatusBlocked).Render("⏳ "+t.WaitingOn))
	}
	if t.Status != store.StatusDone {
		if d := Due(t.Due); d != "" {
			meta = append(meta, d)
		}
	}
	metaStr := strings.Join(meta, "  ")

	titleW := width - lipgloss.Width(prefix) - lipgloss.Width(metaStr) - 4
	if titleW < 20 {
		// Too narrow: drop metadata rather than crush the title.
		metaStr, titleW = "", width-lipgloss.Width(prefix)-4
	}
	title := Truncate(t.Title, titleW)
	pad := titleW - lipgloss.Width(title)
	if pad < 0 {
		pad = 0
	}
	style := Bold
	if t.Status == store.StatusDone {
		style = Muted.Strikethrough(true)
	}
	return "  " + prefix + style.Render(title) + strings.Repeat(" ", pad) + "  " + metaStr
}

func idWidth(tasks []*store.Task) int {
	w := 2
	for _, t := range tasks {
		if n := len(fmt.Sprintf("#%d", t.ID)); n > w {
			w = n
		}
	}
	return w
}

// List renders tasks grouped by status in workflow order.
func List(tasks []*store.Task, width int) string {
	if len(tasks) == 0 {
		return Muted.Render("  Nothing here. Add something with ") + Accent.Render("tars add \"…\"") + "\n"
	}
	groups := map[string][]*store.Task{}
	for _, t := range tasks {
		groups[t.Status] = append(groups[t.Status], t)
	}
	iw := idWidth(tasks)
	var b strings.Builder
	first := true
	for _, st := range store.Statuses {
		g := groups[st]
		if len(g) == 0 {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		first = false
		b.WriteString(" " + StatusBadge(st) + Muted.Render(fmt.Sprintf("  %d", len(g))) + "\n")
		for _, t := range g {
			b.WriteString(Row(t, width, iw) + "\n")
		}
	}
	return b.String()
}

func field(label, value string) string {
	return Muted.Render(fmt.Sprintf("%-11s", label)) + value
}

// Task renders the full detail card for a task.
func Task(t *store.Task, width int) string {
	inner := min(width, 100) - 4
	var b strings.Builder

	b.WriteString(Muted.Render(fmt.Sprintf("#%d", t.ID)) + "  " + StatusBadge(t.Status) + "\n")
	b.WriteString(Bold.Width(inner).Render(t.Title) + "\n\n")

	b.WriteString(field("priority", Priority(t.Priority)+PriorityName(t.Priority)) + "\n")
	if t.Jira != "" {
		b.WriteString(field("jira", Jira(t.Jira)) + "\n")
	}
	if t.Repo != "" {
		b.WriteString(field("repo", t.Repo) + "\n")
	}
	if len(t.Tags) > 0 {
		b.WriteString(field("tags", tagSt.Render("#"+strings.Join(t.Tags, " #"))) + "\n")
	}
	if t.Due != "" {
		b.WriteString(field("due", t.Due+"  "+Due(t.Due)) + "\n")
	}
	if t.Branch != "" {
		b.WriteString(field("branch", t.Branch) + "\n")
	}
	if t.Worktree != "" {
		b.WriteString(field("worktree", t.Worktree) + "\n")
	}
	if t.PR != "" {
		b.WriteString(field("pr", t.PR) + "\n")
	}
	if t.WaitingOn != "" {
		b.WriteString(field("waiting on", t.WaitingOn) + "\n")
	}
	if t.FollowUp != "" {
		b.WriteString(field("follow up", t.FollowUp+"  "+Due(t.FollowUp)) + "\n")
	}
	b.WriteString(field("created", Ago(t.CreatedAt)) + Muted.Render("  ·  updated "+Ago(t.UpdatedAt)) + "\n")

	block := func(title, body string) {
		if strings.TrimSpace(body) == "" {
			return
		}
		b.WriteString(headingSt.Render(title) + "\n")
		b.WriteString(lipgloss.NewStyle().Width(inner).Render(strings.TrimSpace(body)) + "\n")
	}
	if r := t.Report; r != nil {
		b.WriteString(headingSt.Render("Latest report") + Muted.Render("  "+Ago(r.At)) + "\n")
		b.WriteString(reportLines(r, inner))
	}
	block("Description", t.Body)
	block("Acceptance criteria", t.Acceptance)
	if len(t.Sessions) > 0 {
		b.WriteString(headingSt.Render("Sessions") + "\n")
		b.WriteString(Sessions(t.Sessions))
	}

	if len(t.Log) > 0 {
		b.WriteString(headingSt.Render("Log") + "\n")
		for _, e := range t.Log {
			who := Muted.Render(e.Author)
			if e.Author == "agent" {
				who = Accent.Render("agent")
			}
			head := Muted.Render(Ago(e.At)) + "  " + who + Muted.Render(" · "+e.Kind)
			b.WriteString(head + "\n")
			b.WriteString(lipgloss.NewStyle().Width(inner).PaddingLeft(2).Render(e.Text) + "\n")
		}
	}
	return cardSt.Render(strings.TrimRight(b.String(), "\n")) + "\n"
}

// Brief renders the morning summary.
func Brief(br *store.Brief, width int) string {
	var b strings.Builder
	b.WriteString(Accent.Render("TARS") + Muted.Render(" · "+br.Date) + "\n")

	all := [][]*store.Task{br.NeedsYou, br.InReview, br.InProgress, br.Overdue, br.DueToday, br.FollowUps, br.Ready, br.Inbox, br.DoneRecently}
	var flat []*store.Task
	for _, g := range all {
		flat = append(flat, g...)
	}
	iw := idWidth(flat)

	section := func(title string, tasks []*store.Task, hint string) {
		if len(tasks) == 0 {
			return
		}
		b.WriteString("\n " + Bold.Render(title) + Muted.Render(fmt.Sprintf("  %d", len(tasks))))
		if hint != "" {
			b.WriteString(Muted.Render("  — " + hint))
		}
		b.WriteString("\n")
		for _, t := range tasks {
			b.WriteString(Row(t, width, iw) + "\n")
		}
	}
	section("Needs your answer", br.NeedsYou, "questions an agent left you; answer with tars answer")
	section("Waiting for your review", br.InReview, "agent work to check")
	section("Overdue", br.Overdue, "")
	section("Due today", br.DueToday, "")
	section("Follow-ups due", br.FollowUps, "nudge whoever you're waiting on")
	section("Stale", br.Stale, "in progress, untouched for 3+ days")
	section("In progress", br.InProgress, "")
	section("Ready for a sweep", br.Ready, "")
	section("Inbox", br.Inbox, "needs triage")
	section("Done in the last 24h", br.DoneRecently, "")
	if len(flat) == 0 {
		b.WriteString("\n" + Muted.Render("  All clear. Nothing on your plate.") + "\n")
	}
	return b.String()
}

func sections(secs []store.Section, width int) string {
	var b strings.Builder
	for _, s := range secs {
		b.WriteString(headingSt.Render(s.Name) + "\n")
		b.WriteString(lipgloss.NewStyle().Width(min(width, 100)-4).Render(s.Content) + "\n")
	}
	return b.String()
}

func Repo(r *store.Repo, width int) string {
	var b strings.Builder
	b.WriteString(Accent.Render(r.Name))
	if r.Path != "" {
		b.WriteString(Muted.Render("  " + r.Path))
	}
	b.WriteString("\n")
	if len(r.Sections) == 0 {
		b.WriteString(Muted.Render("No sections yet. Suggested: "+strings.Join(store.RepoSections, ", ")) + "\n")
	}
	b.WriteString(sections(r.Sections, width))
	return cardSt.Render(strings.TrimRight(b.String(), "\n")) + "\n"
}

func Repos(rs []*store.Repo) string {
	if len(rs) == 0 {
		return Muted.Render("  No repo profiles yet. Create one with ") + Accent.Render("tars repo set <name> --path .") + "\n"
	}
	var b strings.Builder
	for _, r := range rs {
		b.WriteString("  " + Accent.Render(r.Name) + "  " + Muted.Render(r.Path) + "\n")
	}
	return b.String()
}

func Profile(secs []store.Section, width int) string {
	var b strings.Builder
	b.WriteString(Accent.Render("Personal profile") + "\n")
	if len(secs) == 0 {
		b.WriteString(Muted.Render("Empty. Suggested sections: "+strings.Join(store.ProfileSections, ", ")) + "\n")
	}
	b.WriteString(sections(secs, width))
	return cardSt.Render(strings.TrimRight(b.String(), "\n")) + "\n"
}

var needsSt = lipgloss.NewStyle().Foreground(statusColor[store.StatusInProgress]).Bold(true)

// reportLines renders a report's fields as aligned label/value lines.
func reportLines(r *store.Report, width int) string {
	var b strings.Builder
	wrap := lipgloss.NewStyle().Width(max(width-10, 20))
	line := func(label string, st lipgloss.Style, v string) {
		if v == "" {
			return
		}
		parts := strings.Split(wrap.Render(v), "\n")
		b.WriteString(Muted.Render(fmt.Sprintf("%-10s", label)) + st.Render(parts[0]) + "\n")
		for _, p := range parts[1:] {
			b.WriteString(strings.Repeat(" ", 10) + st.Render(p) + "\n")
		}
	}
	line("done", lipgloss.NewStyle(), r.Done)
	line("verified", lipgloss.NewStyle(), r.Verified)
	switch {
	case r.NeedsYou != "" && r.AnsweredAt == "":
		line("needs you", needsSt, r.NeedsYou)
	case r.NeedsYou != "":
		line("asked", Muted, r.NeedsYou)
		line("answered", lipgloss.NewStyle(), r.Answer)
	}
	line("next", lipgloss.NewStyle(), r.Next)
	return b.String()
}

// Sessions lists Claude sessions with the command to reopen each one.
func Sessions(ss []store.TaskSession) string {
	if len(ss) == 0 {
		return Muted.Render("  No Claude sessions linked yet.") + "\n"
	}
	var b strings.Builder
	for _, s := range ss {
		name := s.Name
		if name == "" {
			name = "Unnamed session"
		}
		b.WriteString("  " + Bold.Render(name) + Muted.Render("  "+Ago(s.LastAt)) + "\n")
		b.WriteString("  " + Accent.Render("claude --resume "+s.ID) + "\n")
	}
	return b.String()
}
