// Package metrics uses one atomically committed JSON record per invocation.
// Concurrent hook processes cannot interleave writes; records contain no tool content.
package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"tokenslim/internal/cache"
	"tokenslim/internal/rules"
)

type Record struct {
	Rules                    map[string]rules.Effect `json:"rules,omitempty"`
	SessionID                string                  `json:"session_id,omitempty"`
	Compressor               string                  `json:"compressor"`
	Mode                     string                  `json:"mode"`
	OriginalBytes            int                     `json:"original_bytes"`
	OptimizedBytes           int                     `json:"optimized_bytes"`
	OriginalLines            int                     `json:"original_lines"`
	OptimizedLines           int                     `json:"optimized_lines"`
	EstimatedOriginalTokens  int                     `json:"estimated_original_tokens"`
	EstimatedOptimizedTokens int                     `json:"estimated_optimized_tokens"`
	DurationNS               int64                   `json:"duration_ns"`
	Changed                  bool                    `json:"changed"`
	Reason                   string                  `json:"reason"`
	CreatedAt                time.Time               `json:"created_at"`
}

func Write(home string, r Record) error {
	if e := cache.PrivateDir(home); e != nil {
		return e
	}
	dir := filepath.Join(home, "metrics")
	if e := cache.PrivateDir(dir); e != nil {
		return e
	}
	r.CreatedAt = time.Now().UTC()
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, ".tmp-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, tmp+".json")
}

type Total struct {
	Processed, Changed, OriginalBytes, OptimizedBytes, EstimatedOriginalTokens, EstimatedOptimizedTokens int
	DurationNS                                                                                           int64
}
type RuleTotal struct{ Records, Groups, Lines int }
type Summary struct {
	ByRule       map[string]RuleTotal
	ByReason     map[string]int
	Total        Total
	ByCompressor map[string]Total
}

func (t *Total) add(r Record) {
	t.Processed++
	if r.Changed {
		t.Changed++
	}
	t.OriginalBytes += r.OriginalBytes
	t.OptimizedBytes += r.OptimizedBytes
	t.EstimatedOriginalTokens += r.EstimatedOriginalTokens
	t.EstimatedOptimizedTokens += r.EstimatedOptimizedTokens
	t.DurationNS += r.DurationNS
}
func Read(home, session string) (Summary, error) {
	s := Summary{ByCompressor: map[string]Total{}, ByRule: map[string]RuleTotal{}, ByReason: map[string]int{}}
	dir := filepath.Join(home, "metrics")
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return s, nil
	}
	if e != nil {
		return s, e
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, e := os.ReadFile(filepath.Join(dir, entry.Name()))
		if e != nil {
			return s, e
		}
		var r Record
		if e = json.Unmarshal(b, &r); e != nil {
			return s, e
		}
		if session != "" && r.SessionID != session {
			continue
		}
		s.Total.add(r)
		reason := r.Reason
		if reason == "" {
			reason = "unspecified"
		}
		s.ByReason[reason]++
		if r.Changed {
			for name, effect := range r.Rules {
				total := s.ByRule[name]
				total.Records++
				total.Groups += effect.Groups
				total.Lines += effect.Lines
				s.ByRule[name] = total
			}
		}
		t := s.ByCompressor[r.Compressor]
		t.add(r)
		s.ByCompressor[r.Compressor] = t
	}
	return s, nil
}

// Debug keeps the most recent invocation metadata, never output or raw commands.
func Debug(home string, r Record) error {
	if e := cache.PrivateDir(home); e != nil {
		return e
	}
	dir := filepath.Join(home, "logs")
	if e := cache.PrivateDir(dir); e != nil {
		return e
	}
	r.CreatedAt = time.Now().UTC()
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	return cache.Atomic(filepath.Join(dir, "tokenslim.log"), append(b, '\n'))
}
