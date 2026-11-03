package usage

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// Currencies besides USD that costs can be shown in.
var Currencies = []string{"INR"}

// Frankfurter serves the European Central Bank's daily reference rates;
// ExchangeRate-API's open endpoint is the fallback. Neither needs a key.
const (
	fxURL         = "https://api.frankfurter.dev/v1/latest?base=USD&symbols="
	fxFallbackURL = "https://open.er-api.com/v6/latest/USD"
	fxTTL         = 12 * time.Hour
)

// FX is the value of one US dollar in other currencies.
type FX struct {
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
	Date      string             `json:"date,omitempty"` // the day the rates are for
	Source    string             `json:"source,omitempty"`
	FetchedAt string             `json:"fetched_at,omitempty"`
	Status    string             `json:"status"`
	Error     string             `json:"error,omitempty"`
}

// parseFX reads either service's reply, keeping only the currencies tars shows.
func parseFX(b []byte) (FX, error) {
	var doc struct {
		Base     string             `json:"base"`
		BaseCode string             `json:"base_code"`
		Date     string             `json:"date"`
		Updated  string             `json:"time_last_update_utc"`
		Provider string             `json:"provider"`
		Rates    map[string]float64 `json:"rates"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return FX{}, err
	}
	fx := FX{Base: doc.Base, Date: doc.Date, Source: "European Central Bank (frankfurter.dev)", Rates: map[string]float64{}}
	if doc.BaseCode != "" {
		fx.Base, fx.Source = doc.BaseCode, "ExchangeRate-API (open.er-api.com)"
		if t, err := time.Parse(time.RFC1123Z, doc.Updated); err == nil {
			fx.Date = t.UTC().Format(time.DateOnly)
		}
	}
	if fx.Base != "USD" {
		return FX{}, errors.New("rates aren't based on USD")
	}
	for _, c := range Currencies {
		if r, ok := doc.Rates[c]; ok && r > 0 {
			fx.Rates[c] = r
		}
	}
	if len(fx.Rates) < len(Currencies) {
		return FX{}, errors.New("rates are missing a currency")
	}
	return fx, nil
}

// LoadFX returns live exchange rates, cached in dir for 12 hours. When they
// can't be fetched it uses the last good rates, and with none it says so:
// costs then show in USD only, never at a made-up rate.
func LoadFX(dir string) FX {
	var fx FX
	f, err := fetchCached(filepath.Join(dir, "fx-usd.json"), fxTTL,
		func(b []byte) (err error) { fx, err = parseFX(b); return err },
		fxURL+strings.Join(Currencies, ","), fxFallbackURL)
	if f.Status == Unavailable {
		out := FX{Base: "USD", Rates: map[string]float64{}, Status: Unavailable}
		if err != nil {
			out.Error = err.Error()
		}
		return out
	}
	fx.Status = f.Status
	fx.FetchedAt = f.At.UTC().Format(time.RFC3339)
	return fx
}
