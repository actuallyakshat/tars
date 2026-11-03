package usage

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Status says where a live value came from.
const (
	Fresh       = "fresh"       // fetched just now
	Cached      = "cached"      // fetched within its time to live
	Stale       = "stale"       // the last good copy; fetching failed
	Unavailable = "unavailable" // never fetched, and fetching failed
)

var client = &http.Client{Timeout: 15 * time.Second}

// fetched is a document from one of urls, cached at path for ttl. When every
// URL fails it falls back to the cached copy, however old.
type fetched struct {
	Body   []byte
	URL    string // which URL it came from ("" when only the cache had it)
	At     time.Time
	Status string
}

// fetchCached returns the cached document while it's fresh, otherwise fetches
// it, checking each response with valid before caching it.
func fetchCached(path string, ttl time.Duration, valid func([]byte) error, urls ...string) (fetched, error) {
	info, statErr := os.Stat(path)
	if statErr == nil && time.Since(info.ModTime()) < ttl {
		if b, err := os.ReadFile(path); err == nil && valid(b) == nil {
			return fetched{Body: b, At: info.ModTime(), Status: Cached}, nil
		}
	}
	var errs []error
	for _, u := range urls {
		b, err := get(u)
		if err == nil {
			err = valid(b)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", u, err))
			continue
		}
		_ = writeAtomic(path, b)
		return fetched{Body: b, URL: u, At: time.Now(), Status: Fresh}, nil
	}
	if statErr == nil {
		if b, err := os.ReadFile(path); err == nil && valid(b) == nil {
			return fetched{Body: b, At: info.ModTime(), Status: Stale}, errors.Join(errs...)
		}
	}
	return fetched{Status: Unavailable}, errors.Join(errs...)
}

func get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tars")
	if strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Accept", "application/vnd.github.raw+json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
