// Package usage reads Claude Code's transcripts for token usage and prices it
// with live model rates, so tars can show what sessions and tasks cost.
package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Record is one API call's usage. Claude Code writes the same usage on every
// content block of a message, so records are deduplicated by Key.
type Record struct {
	At           time.Time
	Model        string
	Session      string
	Cwd          string
	Input        int64 // uncached input
	CacheRead    int64
	CacheWrite   int64 // all cache writes, 5m and 1h
	CacheWrite1h int64 // the part of CacheWrite written for an hour
	Output       int64
	Fast         bool
	ReportedUSD  *float64 // what Claude Code itself recorded, when it did
	Key          string
}

type tokens struct {
	Input        int64 `json:"input_tokens"`
	CacheRead    int64 `json:"cache_read_input_tokens"`
	CacheWrite   int64 `json:"cache_creation_input_tokens"`
	Output       int64 `json:"output_tokens"`
	CacheWriteBy *struct {
		OneHour int64 `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
}

type line struct {
	Type      string   `json:"type"`
	Timestamp string   `json:"timestamp"`
	RequestID string   `json:"requestId"`
	SessionID string   `json:"sessionId"`
	Cwd       string   `json:"cwd"`
	CostUSD   *float64 `json:"costUSD"`
	Message   *struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			tokens
			Speed      string `json:"speed"`
			Iterations []struct {
				tokens
				Type  string `json:"type"`
				Model string `json:"model"`
			} `json:"iterations"`
		} `json:"usage"`
	} `json:"message"`
}

// synthetic marks Claude Code's local placeholders that never reached the API.
const synthetic = "<synthetic>"

func (t tokens) oneHour() int64 {
	if t.CacheWriteBy == nil {
		return 0
	}
	return min(t.CacheWriteBy.OneHour, t.CacheWrite)
}

// ParseLine returns the usage records in one transcript line: the call
// itself, plus one for each advisor call made inside it (on its own model).
func ParseLine(b []byte) []Record {
	if !bytes.Contains(b, []byte(`"usage"`)) {
		return nil
	}
	var l line
	if json.Unmarshal(b, &l) != nil || l.Type != "assistant" || l.Message == nil || l.Message.Usage == nil {
		return nil
	}
	at, err := time.Parse(time.RFC3339Nano, l.Timestamp)
	if err != nil {
		return nil
	}
	m, u := l.Message, l.Message.Usage
	key := ""
	if m.ID != "" || l.RequestID != "" {
		key = m.ID + ":" + l.RequestID
	}
	var out []Record
	if m.Model != "" && m.Model != synthetic {
		out = append(out, Record{
			At: at, Model: m.Model, Session: l.SessionID, Cwd: l.Cwd,
			Input: u.Input, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, CacheWrite1h: u.oneHour(), Output: u.Output,
			Fast: u.Speed == "fast", ReportedUSD: l.CostUSD, Key: key,
		})
	}
	for i, it := range u.Iterations {
		if it.Type != "advisor_message" || it.Model == "" || it.Model == synthetic {
			continue
		}
		k := ""
		if key != "" {
			k = key + ":advisor:" + strconv.Itoa(i)
		}
		out = append(out, Record{
			At: at, Model: it.Model, Session: l.SessionID, Cwd: l.Cwd,
			Input: it.Input, CacheRead: it.CacheRead, CacheWrite: it.CacheWrite, CacheWrite1h: it.oneHour(), Output: it.Output,
			Fast: u.Speed == "fast", Key: k,
		})
	}
	return out
}

// Read returns the records in r, skipping lines it can't parse.
func Read(r io.Reader, keep func(Record) bool) []Record {
	br := bufio.NewReaderSize(r, 1<<20)
	var out []Record
	for {
		b, err := br.ReadBytes('\n')
		for _, rec := range ParseLine(b) {
			if keep == nil || keep(rec) {
				out = append(out, rec)
			}
		}
		if err != nil {
			return out
		}
	}
}

// Transcripts lists the .jsonl files under Claude Code's projects folder,
// subagent transcripts included, modified at or after since.
func Transcripts(configDir string, since time.Time) []string {
	var files []string
	root := filepath.Join(configDir, "projects")
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".jsonl") {
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().Before(since) {
			return nil
		}
		files = append(files, p)
		return nil
	})
	return files
}

// SessionTranscripts lists one session's transcript and its subagents'.
func SessionTranscripts(configDir, id string) []string {
	if id == "" || strings.ContainsAny(id, `/\`) {
		return nil
	}
	root := filepath.Join(configDir, "projects")
	files, _ := filepath.Glob(filepath.Join(root, "*", id+".jsonl"))
	subs, _ := filepath.Glob(filepath.Join(root, "*", id, "subagents", "*.jsonl"))
	return append(files, subs...)
}

// ReadFiles reads every file and drops repeated records, keeping the first.
func ReadFiles(files []string, keep func(Record) bool) []Record {
	seen := map[string]bool{}
	var out []Record
	for _, p := range files {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		for _, r := range Read(f, keep) {
			if r.Key != "" {
				if seen[r.Key] {
					continue
				}
				seen[r.Key] = true
			}
			out = append(out, r)
		}
		f.Close()
	}
	return out
}
