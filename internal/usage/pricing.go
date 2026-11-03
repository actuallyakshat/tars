package usage

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LiteLLM keeps a public, frequently updated table of model prices.
const (
	PricesURL    = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	pricesAPIURL = "https://api.github.com/repos/BerriAI/litellm/contents/model_prices_and_context_window.json?ref=main"
	pricesTTL    = 24 * time.Hour
)

// tier holds per-token USD prices for prompts larger than Above tokens.
type tier struct {
	Above        int64
	Input        float64
	Output       float64
	CacheRead    float64
	CacheWrite   float64 // a 5 minute cache write
	CacheWrite1h float64
}

// Rate is a model's prices: tiers by prompt size, smallest first, and a
// multiplier for fast mode (0 when the table has none).
type Rate struct {
	Tiers []tier
	Fast  float64
}

// Prices maps lowercased model names to rates.
type Prices map[string]Rate

func num(e map[string]any, key string) (float64, bool) {
	v, ok := e[key].(float64)
	return v, ok
}

func or(e map[string]any, key string, def float64) float64 {
	if v, ok := num(e, key); ok {
		return v
	}
	return def
}

var tierKey = regexp.MustCompile(`^input_cost_per_token_above_(\d+)k_tokens$`)

// ParsePrices reads LiteLLM's table. Models without both input and output
// prices are left out, so they show as unpriced instead of too cheap.
func ParsePrices(b []byte) (Prices, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	p := Prices{}
	for name, raw := range doc {
		var e map[string]any
		if json.Unmarshal(raw, &e) != nil {
			continue
		}
		in, ok1 := num(e, "input_cost_per_token")
		out, ok2 := num(e, "output_cost_per_token")
		if !ok1 || !ok2 {
			continue
		}
		base := tier{Input: in, Output: out}
		base.CacheRead = or(e, "cache_read_input_token_cost", in)
		base.CacheWrite = or(e, "cache_creation_input_token_cost", in)
		base.CacheWrite1h = or(e, "cache_creation_input_token_cost_above_1hr", base.CacheWrite)
		r := Rate{Tiers: []tier{base}}
		for k := range e {
			m := tierKey.FindStringSubmatch(k)
			if m == nil {
				continue
			}
			tin, ok := num(e, k)
			if !ok {
				continue
			}
			n, _ := strconv.ParseInt(m[1], 10, 64)
			sfx := "_above_" + m[1] + "k_tokens"
			t := tier{Above: n * 1000, Input: tin}
			scale := t.Input / in
			t.Output = or(e, "output_cost_per_token"+sfx, out)
			t.CacheRead = or(e, "cache_read_input_token_cost"+sfx, base.CacheRead*scale)
			t.CacheWrite = or(e, "cache_creation_input_token_cost"+sfx, base.CacheWrite*scale)
			t.CacheWrite1h = or(e, "cache_creation_input_token_cost_above_1hr"+sfx, base.CacheWrite1h*scale)
			r.Tiers = append(r.Tiers, t)
		}
		sort.Slice(r.Tiers, func(i, j int) bool { return r.Tiers[i].Above < r.Tiers[j].Above })
		if ps, ok := e["provider_specific_entry"].(map[string]any); ok {
			if f, ok := ps["fast"].(float64); ok && f > 0 {
				r.Fast = f
			}
		}
		p[strings.ToLower(strings.TrimSpace(name))] = r
	}
	if len(p) == 0 {
		return nil, errors.New("no model prices in the table")
	}
	return p, nil
}

// Bare family names could be any generation, so they stay unpriced.
var ambiguous = map[string]bool{"opus": true, "sonnet": true, "haiku": true, "fable": true}

// Lookup finds a model's rate. Context variants like "[1m]" use the base name.
func (p Prices) Lookup(model string) (Rate, bool) {
	k := strings.ToLower(strings.TrimSpace(model))
	if i := strings.IndexByte(k, '['); i >= 0 {
		k = k[:i]
	}
	if k == "" || ambiguous[k] {
		return Rate{}, false
	}
	if r, ok := p[k]; ok {
		return r, true
	}
	r, ok := p["anthropic/"+k]
	return r, ok
}

// Cost is a priced record, in USD.
type Cost struct {
	Input, Output, CacheRead, CacheWrite float64
	Savings                              float64 // what cache reads saved over uncached input
	Priced                               bool
}

func (c Cost) Total() float64 { return c.Input + c.Output + c.CacheRead + c.CacheWrite }

// Price prices a record. A cost Claude Code recorded itself wins over the
// table; the table still splits it by token type.
func (p Prices) Price(r Record) Cost {
	rate, ok := p.Lookup(r.Model)
	if !ok {
		if r.ReportedUSD != nil {
			return Cost{Input: *r.ReportedUSD, Priced: true}
		}
		return Cost{}
	}
	prompt := r.Input + r.CacheRead + r.CacheWrite
	t := rate.Tiers[0]
	for _, x := range rate.Tiers[1:] {
		if prompt > x.Above {
			t = x
		}
	}
	mult := 1.0
	if r.Fast && rate.Fast > 0 {
		mult = rate.Fast
	}
	c := Cost{
		Input:      float64(r.Input) * t.Input * mult,
		Output:     float64(r.Output) * t.Output * mult,
		CacheRead:  float64(r.CacheRead) * t.CacheRead * mult,
		CacheWrite: (float64(r.CacheWrite-r.CacheWrite1h)*t.CacheWrite + float64(r.CacheWrite1h)*t.CacheWrite1h) * mult,
		Savings:    float64(r.CacheRead) * (t.Input - t.CacheRead) * mult,
		Priced:     true,
	}
	if r.ReportedUSD != nil {
		if total := c.Total(); total > 0 {
			s := *r.ReportedUSD / total
			c.Input, c.Output, c.CacheRead, c.CacheWrite, c.Savings = c.Input*s, c.Output*s, c.CacheRead*s, c.CacheWrite*s, c.Savings*s
		} else {
			c = Cost{Input: *r.ReportedUSD, Priced: true}
		}
	}
	return c
}

// PriceInfo says where the prices came from.
type PriceInfo struct {
	Source    string `json:"source"`
	FetchedAt string `json:"fetched_at,omitempty"`
	Status    string `json:"status"`
	Models    int    `json:"models"`
	Error     string `json:"error,omitempty"`
}

// LoadPrices returns LiteLLM's prices, cached in dir for a day.
func LoadPrices(dir string) (Prices, PriceInfo) {
	var p Prices
	f, err := fetchCached(filepath.Join(dir, "model-prices.json"), pricesTTL,
		func(b []byte) (err error) { p, err = ParsePrices(b); return err }, PricesURL, pricesAPIURL)
	info := PriceInfo{Source: PricesURL, Status: f.Status}
	if f.Status == Unavailable {
		if err != nil {
			info.Error = err.Error()
		}
		return Prices{}, info
	}
	info.FetchedAt = f.At.UTC().Format(time.RFC3339)
	info.Models = len(p)
	return p, info
}
