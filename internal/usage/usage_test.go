package usage

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const table = `{
  "claude-opus-5-5": {"input_cost_per_token": 4e-06, "output_cost_per_token": 2e-05,
    "cache_read_input_token_cost": 2e-07, "cache_creation_input_token_cost": 5e-06,
    "cache_creation_input_token_cost_above_1hr": 8e-06,
    "provider_specific_entry": {"fast": 2.0}},
  "claude-haiku-5-5": {"input_cost_per_token": 1e-07, "output_cost_per_token": 5e-07,
    "input_cost_per_token_above_100k_tokens": 5e-07, "output_cost_per_token_above_100k_tokens": 2.5e-06},
  "anthropic/claude-only-prefixed": {"input_cost_per_token": 1e-06, "output_cost_per_token": 1e-06},
  "no-output-price": {"input_cost_per_token": 1e-06},
  "sample_spec": "not a model"
}`

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %.10f, want %.10f", what, got, want)
	}
}

func mustPrices(t *testing.T) Prices {
	t.Helper()
	p, err := ParsePrices([]byte(table))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLookup(t *testing.T) {
	p := mustPrices(t)
	for model, want := range map[string]bool{
		"claude-opus-5-5":        true,
		"Claude-Opus-5-5[1m]":    true,
		"claude-only-prefixed":   true,
		"no-output-price":        false,
		"opus":                   false,
		"claude-unknown-model-9": false,
	} {
		if _, ok := p.Lookup(model); ok != want {
			t.Errorf("Lookup(%q) found = %v, want %v", model, ok, want)
		}
	}
}

func TestPrice(t *testing.T) {
	p := mustPrices(t)
	r := Record{Model: "claude-opus-5-5", Input: 1000, Output: 100, CacheRead: 10000, CacheWrite: 3000, CacheWrite1h: 1000}
	c := p.Price(r)
	near(t, "input", c.Input, 1000*4e-6)
	near(t, "output", c.Output, 100*2e-5)
	near(t, "cache read", c.CacheRead, 10000*2e-7)
	near(t, "cache write (2000 at 5m, 1000 at 1h)", c.CacheWrite, 2000*5e-6+1000*8e-6)
	near(t, "savings", c.Savings, 10000*(4e-6-2e-7))

	r.Fast = true
	near(t, "fast mode doubles", p.Price(r).Total(), 2*c.Total())

	reported := 1.5
	r.Fast, r.ReportedUSD = false, &reported
	near(t, "reported cost wins", p.Price(r).Total(), 1.5)

	if p.Price(Record{Model: "mystery", Input: 10}).Priced {
		t.Error("an unknown model was priced")
	}
}

func TestLongContextTier(t *testing.T) {
	p := mustPrices(t)
	small := p.Price(Record{Model: "claude-haiku-5-5", Input: 100_000, Output: 10})
	near(t, "at the threshold", small.Input, 100_000*1e-7)
	big := p.Price(Record{Model: "claude-haiku-5-5", Input: 100_001, Output: 10})
	near(t, "above it", big.Input, 100_001*5e-7)
	near(t, "above it, output", big.Output, 10*2.5e-6)
}

const transcript = `{"type":"user","message":{"role":"user","content":"hi"},"timestamp":"2026-11-01T10:00:00Z","sessionId":"s1"}
{"type":"assistant","timestamp":"2026-11-01T10:00:01Z","requestId":"req1","sessionId":"s1","cwd":"/w/app","message":{"id":"m1","model":"claude-opus-5-5","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":100,"cache_creation":{"ephemeral_5m_input_tokens":40,"ephemeral_1h_input_tokens":60}}}}
{"type":"assistant","timestamp":"2026-11-01T10:00:01Z","requestId":"req1","sessionId":"s1","cwd":"/w/app","message":{"id":"m1","model":"claude-opus-5-5","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":100}}}
{"type":"assistant","timestamp":"2026-11-01T10:00:02Z","requestId":"req2","sessionId":"s1","cwd":"/w/app","message":{"id":"m2","model":"claude-opus-5-5","usage":{"input_tokens":1,"output_tokens":1,"speed":"fast","iterations":[{"type":"message","input_tokens":1},{"type":"advisor_message","model":"claude-fable-5-1","input_tokens":500,"output_tokens":50}]}}}
{"type":"assistant","timestamp":"2026-11-01T10:00:03Z","sessionId":"s1","message":{"id":"m3","model":"<synthetic>","usage":{"input_tokens":0,"output_tokens":0}}}
not json at all, but mentions "usage"
`

func TestParseAndDedupe(t *testing.T) {
	dir := t.TempDir()
	proj := filepath.Join(dir, "projects", "-w-app")
	sub := filepath.Join(proj, "s1", "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(proj, "s1.jsonl"), transcript)
	// A subagent's transcript names its parent session; a resumed session
	// replays a message already counted.
	write(filepath.Join(sub, "agent-a.jsonl"), `{"type":"assistant","timestamp":"2026-11-01T11:00:00Z","requestId":"req9","sessionId":"s1","isSidechain":true,"message":{"id":"m9","model":"claude-haiku-5-5","usage":{"input_tokens":7,"output_tokens":3}}}`+"\n")
	write(filepath.Join(proj, "s2.jsonl"), strings.Split(transcript, "\n")[1]+"\n")

	files := Transcripts(dir, time.Time{})
	if len(files) != 3 {
		t.Fatalf("found %d transcripts, want 3: %v", len(files), files)
	}
	recs := ReadFiles(files, nil)
	models := map[string]int{}
	for _, r := range recs {
		models[r.Model]++
		if r.Session != "s1" {
			t.Errorf("record on session %q, want s1", r.Session)
		}
	}
	want := map[string]int{"claude-opus-5-5": 2, "claude-fable-5-1": 1, "claude-haiku-5-5": 1}
	for m, n := range want {
		if models[m] != n {
			t.Errorf("%d records for %s, want %d (all: %v)", models[m], m, n, models)
		}
	}
	for _, r := range recs {
		if r.Key == "m1:req1" && r.CacheWrite1h != 60 {
			t.Errorf("1h cache writes = %d, want 60", r.CacheWrite1h)
		}
		if r.Key == "m2:req2" && !r.Fast {
			t.Error("fast mode was lost")
		}
	}
	if got := SessionTranscripts(dir, "s1"); len(got) != 2 {
		t.Errorf("session s1 has %d transcripts, want 2: %v", len(got), got)
	}
	if got := SessionTranscripts(dir, "../x"); got != nil {
		t.Errorf("a path in a session id matched %v", got)
	}
}

func TestTaskFor(t *testing.T) {
	links := []Link{{TaskID: 1, From: "2026-11-01T10:00:00Z"}, {TaskID: 2, From: "2026-11-01T12:00:00Z"}}
	at := func(s string) time.Time { v, _ := time.Parse(time.RFC3339, s); return v }
	for when, want := range map[string]int64{
		"2026-11-01T09:00:00Z": 1, // before every link: the first one
		"2026-11-01T11:00:00Z": 1,
		"2026-11-01T12:00:00Z": 2,
		"2026-11-02T00:00:00Z": 2,
	} {
		if got := TaskFor(links, at(when)); got == nil || got.TaskID != want {
			t.Errorf("TaskFor(%s) = %v, want task %d", when, got, want)
		}
	}
	if TaskFor(nil, time.Now()) != nil {
		t.Error("a session with no links was charged to a task")
	}
}

func TestSummarize(t *testing.T) {
	p := mustPrices(t)
	day := func(d int, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, time.UTC) }
	recs := []Record{
		{At: day(1, 9), Model: "claude-opus-5-5", Session: "a", Cwd: "/w/app", Input: 1000},
		{At: day(3, 9), Model: "claude-opus-5-5", Session: "a", Cwd: "/w/app", Input: 1000},
		{At: day(3, 10), Model: "mystery", Session: "b", Cwd: "/w/lib", Input: 50},
		{At: day(20, 1), Model: "claude-opus-5-5", Session: "a", Input: 1}, // after the period
	}
	s := Summarize(recs, p, Options{
		Since: day(1, 0), Until: day(3, 23), Loc: time.UTC,
		Links: map[string][]Link{"a": {{TaskID: 7, Title: "Fix export", Status: "ready", From: "2026-11-02T00:00:00Z"}}},
		Repo:  func(_, cwd string) string { return filepath.Base(cwd) },
	})
	near(t, "total", s.Total.CostUSD, 2000*4e-6)
	if s.Total.Calls != 3 || s.Total.Sessions != 2 || s.Total.Unpriced != 1 {
		t.Errorf("calls %d sessions %d unpriced %d, want 3 2 1", s.Total.Calls, s.Total.Sessions, s.Total.Unpriced)
	}
	if len(s.Daily) != 3 || s.Daily[1].Calls != 0 || s.Daily[2].Calls != 2 {
		t.Errorf("daily = %+v, want 3 days with the middle one empty", s.Daily)
	}
	if len(s.Unpriced) != 1 || s.Unpriced[0] != "mystery" {
		t.Errorf("unpriced = %v", s.Unpriced)
	}
	if len(s.Tasks) != 1 || s.Tasks[0].Key != "7" || s.Tasks[0].Label != "Fix export" || s.Tasks[0].Calls != 2 {
		t.Errorf("tasks = %+v", s.Tasks)
	}
	if len(s.Repos) != 2 || s.Repos[0].Key != "app" {
		t.Errorf("repos = %+v, want app first", s.Repos)
	}
}

func TestParseFX(t *testing.T) {
	fx, err := parseFX([]byte(`{"amount":1.0,"base":"USD","date":"2026-11-02","rates":{"INR":94.91}}`))
	if err != nil || fx.Rates["INR"] != 94.91 || fx.Date != "2026-11-02" {
		t.Errorf("frankfurter: %+v, %v", fx, err)
	}
	fx, err = parseFX([]byte(`{"result":"success","base_code":"USD","time_last_update_utc":"Tue, 03 Nov 2026 00:02:31 +0000","rates":{"USD":1,"INR":94.8,"EUR":0.9}}`))
	if err != nil || fx.Rates["INR"] != 94.8 || fx.Date != "2026-11-03" || len(fx.Rates) != 1 {
		t.Errorf("open.er-api: %+v, %v", fx, err)
	}
	if _, err := parseFX([]byte(`{"base":"EUR","rates":{"INR":90}}`)); err == nil {
		t.Error("accepted rates based on EUR")
	}
	if _, err := parseFX([]byte(`{"base":"USD","rates":{}}`)); err == nil {
		t.Error("accepted rates without INR")
	}
}

func TestFetchCachedFallsBackToLastGood(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.json")
	if err := os.WriteFile(path, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	ok := func([]byte) error { return nil }
	f, err := fetchCached(path, time.Hour, ok, "http://127.0.0.1:1/unreachable")
	if f.Status != Stale || string(f.Body) != `{"ok":true}` {
		t.Errorf("got %q (%s), want the stale copy", f.Body, f.Status)
	}
	if err == nil {
		t.Error("the failed fetch wasn't reported")
	}
	f, _ = fetchCached(filepath.Join(t.TempDir(), "none.json"), time.Hour, ok, "http://127.0.0.1:1/unreachable")
	if f.Status != Unavailable {
		t.Errorf("status = %s with nothing cached, want unavailable", f.Status)
	}
}
