package usage

import (
	"sort"
	"strconv"
	"time"
)

// Link puts a session on a task from From on (a store.SessionLink).
type Link struct {
	TaskID int64
	Title  string
	Status string
	From   string // RFC 3339
}

// TaskFor returns the task a record at t is charged to: the latest link that
// started at or before t, or the first link for anything before them all.
func TaskFor(links []Link, t time.Time) *Link {
	if len(links) == 0 {
		return nil
	}
	best := &links[0]
	for i := range links {
		from, err := time.Parse(time.RFC3339Nano, links[i].From)
		if err == nil && !from.After(t) {
			best = &links[i]
		}
	}
	return best
}

// Tokens counts tokens by type.
type Tokens struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}

func (t Tokens) Total() int64 { return t.Input + t.Output + t.CacheRead + t.CacheWrite }

// CostByType is USD by token type.
type CostByType struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

// Line is one row of a breakdown.
type Line struct {
	Key         string     `json:"key"`
	Label       string     `json:"label,omitempty"`
	Status      string     `json:"status,omitempty"`
	CostUSD     float64    `json:"cost_usd"`
	Cost        CostByType `json:"cost"`
	Tokens      Tokens     `json:"tokens"`
	TotalTokens int64      `json:"total_tokens"`
	SavingsUSD  float64    `json:"cache_savings_usd"`
	Calls       int        `json:"calls"`
	Unpriced    int        `json:"unpriced_calls,omitempty"`
	Sessions    int        `json:"sessions"`

	sessions map[string]bool
}

func (l *Line) add(r Record, c Cost) {
	l.Calls++
	if !c.Priced {
		l.Unpriced++
	}
	l.CostUSD += c.Total()
	l.Cost.Input += c.Input
	l.Cost.Output += c.Output
	l.Cost.CacheRead += c.CacheRead
	l.Cost.CacheWrite += c.CacheWrite
	l.SavingsUSD += c.Savings
	l.Tokens.Input += r.Input
	l.Tokens.Output += r.Output
	l.Tokens.CacheRead += r.CacheRead
	l.Tokens.CacheWrite += r.CacheWrite
	l.TotalTokens = l.Tokens.Total()
	if r.Session != "" {
		if l.sessions == nil {
			l.sessions = map[string]bool{}
		}
		if !l.sessions[r.Session] {
			l.sessions[r.Session] = true
			l.Sessions++
		}
	}
}

// Summary is what Claude Code cost over a period, in USD. FX converts it.
type Summary struct {
	Since       string    `json:"since"`
	Until       string    `json:"until"`
	TimeZone    string    `json:"time_zone"`
	GeneratedAt string    `json:"generated_at"`
	Total       Line      `json:"total"`
	Daily       []Line    `json:"daily"` // every day in the period, oldest first, keyed YYYY-MM-DD
	Models      []Line    `json:"models"`
	Repos       []Line    `json:"repos"`
	Tasks       []Line    `json:"tasks"` // keyed by task id
	Unpriced    []string  `json:"unpriced_models"`
	Pricing     PriceInfo `json:"pricing"`
	FX          FX        `json:"fx"`
	Files       int       `json:"files"`
	ScanMillis  int64     `json:"scan_ms"`
}

// Options shape a summary.
type Options struct {
	Since, Until time.Time // Since zero means from the first record
	Loc          *time.Location
	Links        map[string][]Link
	Repo         func(session, cwd string) string // "" leaves it out of Repos
}

type group struct {
	order []string
	lines map[string]*Line
}

func (g *group) get(key string) *Line {
	if g.lines == nil {
		g.lines = map[string]*Line{}
	}
	l, ok := g.lines[key]
	if !ok {
		l = &Line{Key: key}
		g.lines[key] = l
		g.order = append(g.order, key)
	}
	return l
}

func (g *group) byCost() []Line {
	out := make([]Line, 0, len(g.order))
	for _, k := range g.order {
		out = append(out, *g.lines[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CostUSD != out[j].CostUSD {
			return out[i].CostUSD > out[j].CostUSD
		}
		return out[i].TotalTokens > out[j].TotalTokens
	})
	return out
}

// Summarize prices records inside the period and groups them.
func Summarize(records []Record, prices Prices, o Options) Summary {
	loc := o.Loc
	if loc == nil {
		loc = time.Local
	}
	var total Line
	var days, models, repos, tasks group
	unpriced := map[string]bool{}
	first := o.Until
	for _, r := range records {
		if (!o.Since.IsZero() && r.At.Before(o.Since)) || r.At.After(o.Until) {
			continue
		}
		if r.At.Before(first) {
			first = r.At
		}
		c := prices.Price(r)
		if !c.Priced {
			unpriced[r.Model] = true
		}
		total.add(r, c)
		days.get(r.At.In(loc).Format(time.DateOnly)).add(r, c)
		models.get(r.Model).add(r, c)
		if o.Repo != nil {
			if name := o.Repo(r.Session, r.Cwd); name != "" {
				repos.get(name).add(r, c)
			}
		}
		if t := TaskFor(o.Links[r.Session], r.At); t != nil {
			l := tasks.get(strconv.FormatInt(t.TaskID, 10))
			l.Label, l.Status = t.Title, t.Status
			l.add(r, c)
		}
	}
	since := o.Since
	if since.IsZero() {
		since = first
	}
	s := Summary{
		Since:       since.In(loc).Format(time.DateOnly),
		Until:       o.Until.In(loc).Format(time.DateOnly),
		TimeZone:    loc.String(),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Total:       total,
		Models:      models.byCost(),
		Repos:       repos.byCost(),
		Tasks:       tasks.byCost(),
		Daily:       []Line{},
		Unpriced:    []string{},
	}
	s.Total.Key = "total"
	// Every day in the period, so a chart shows the quiet days too.
	y, m, d := since.In(loc).Date()
	for day := time.Date(y, m, d, 0, 0, 0, 0, loc); !day.After(o.Until); day = day.AddDate(0, 0, 1) {
		k := day.Format(time.DateOnly)
		if l, ok := days.lines[k]; ok {
			s.Daily = append(s.Daily, *l)
		} else {
			s.Daily = append(s.Daily, Line{Key: k})
		}
	}
	for m := range unpriced {
		s.Unpriced = append(s.Unpriced, m)
	}
	sort.Strings(s.Unpriced)
	return s
}
