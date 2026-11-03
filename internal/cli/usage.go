package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/actuallyakshat/tars/internal/claude"
	"github.com/actuallyakshat/tars/internal/store"
	"github.com/actuallyakshat/tars/internal/usage"
)

// usageCmd backs the desktop app's Usage page. It's hidden: the page is
// where usage is meant to be read.
func usageCmd(a *app) *cobra.Command {
	var days int
	var task string
	cmd := &cobra.Command{
		Use:    "usage",
		Short:  "What Claude Code cost, from its transcripts, priced at live rates",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if days < 0 {
				return fmt.Errorf("--days can't be negative")
			}
			var taskID int64
			if task != "" {
				id, err := parseID(task)
				if err != nil {
					return err
				}
				taskID = id
			}
			s, err := a.usage(days, taskID)
			if err != nil {
				return err
			}
			return a.emit(s, func() string { return renderUsage(s) })
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "days to cover, today included (0 for everything)")
	cmd.Flags().StringVar(&task, "task", "", "only this task's sessions, over all time")
	return cmd
}

// cacheDir holds the fetched prices and exchange rates, next to the database.
func (a *app) cacheDir() (string, error) {
	path := a.dbPath
	if path == "" {
		var err error
		if path, err = store.DefaultPathFor(isDev()); err != nil {
			return "", err
		}
	}
	return filepath.Join(filepath.Dir(path), "cache"), nil
}

func (a *app) usage(days int, taskID int64) (usage.Summary, error) {
	dir, err := a.cacheDir()
	if err != nil {
		return usage.Summary{}, err
	}
	links, err := a.st.SessionLinks()
	if err != nil {
		return usage.Summary{}, err
	}
	captured, err := a.st.CaptureRepos()
	if err != nil {
		return usage.Summary{}, err
	}
	if taskID > 0 {
		if _, err := a.st.GetTask(taskID); err != nil {
			return usage.Summary{}, err
		}
	}

	now := time.Now()
	o := usage.Options{Until: now, Loc: time.Local, Links: map[string][]usage.Link{}}
	if days > 0 && taskID == 0 {
		y, m, d := now.Date()
		o.Since = time.Date(y, m, d-days+1, 0, 0, 0, 0, time.Local)
	}
	var sessions []string
	for id, ls := range links {
		onTask := false
		for _, l := range ls {
			onTask = onTask || l.TaskID == taskID
			o.Links[id] = append(o.Links[id], usage.Link{TaskID: l.TaskID, Title: l.Title, Status: l.Status, From: l.FirstAt})
		}
		if onTask {
			sessions = append(sessions, id)
		}
	}

	// A session's repo is the one tars captured it in, else the repo profile
	// covering its folder, else the folder's name.
	byDir := map[string]string{}
	o.Repo = func(session, cwd string) string {
		if r := captured[session]; r != "" {
			return r
		}
		if cwd == "" {
			return ""
		}
		name, ok := byDir[cwd]
		if !ok {
			name = filepath.Base(cwd)
			if r, err := a.st.FindRepoByPath(cwd); err == nil {
				name = r.Name
			}
			byDir[cwd] = name
		}
		return name
	}

	start := time.Now()
	var files []string
	if taskID > 0 {
		for _, id := range sessions {
			files = append(files, usage.SessionTranscripts(claude.ConfigDir(), id)...)
		}
	} else {
		files = usage.Transcripts(claude.ConfigDir(), o.Since)
	}
	keep := func(r usage.Record) bool { return o.Since.IsZero() || !r.At.Before(o.Since) }
	records := usage.ReadFiles(files, keep)
	scanned := time.Since(start)

	if taskID > 0 {
		// Only what was charged to this task: a session may have moved on.
		var mine []usage.Record
		for _, r := range records {
			if t := usage.TaskFor(o.Links[r.Session], r.At); t != nil && t.TaskID == taskID {
				mine = append(mine, r)
			}
		}
		records = mine
	}

	prices, info := usage.LoadPrices(dir)
	s := usage.Summarize(records, prices, o)
	s.Pricing = info
	s.FX = usage.LoadFX(dir)
	s.Files = len(files)
	s.ScanMillis = scanned.Milliseconds()
	return s, nil
}

func renderUsage(s usage.Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Claude Code usage, %s to %s\n", s.Since, s.Until)
	fmt.Fprintf(&b, "  Cost      $%.2f", s.Total.CostUSD)
	for c, r := range s.FX.Rates {
		fmt.Fprintf(&b, "  (%s %.2f at %.4f)", c, s.Total.CostUSD*r, r)
	}
	fmt.Fprintf(&b, "\n  Tokens    %d\n  Sessions  %d\n", s.Total.TotalTokens, s.Total.Sessions)
	for _, m := range s.Models {
		fmt.Fprintf(&b, "  %-28s $%.2f\n", m.Key, m.CostUSD)
	}
	if len(s.Unpriced) > 0 {
		fmt.Fprintf(&b, "  No price for: %s\n", strings.Join(s.Unpriced, ", "))
	}
	fmt.Fprintf(&b, "Prices: %s (%s)\n", s.Pricing.Source, s.Pricing.Status)
	return b.String()
}
