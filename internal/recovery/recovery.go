// Package recovery exposes bounded, read-only views of verified cache originals.
package recovery

import (
	"encoding/json"
	"fmt"
	"strings"
	"tokenslim/internal/cache"
	"unicode/utf8"
)

const MaxPageBytes = 64 << 10
const MaxLines = 500

type Service struct{ Store cache.Store }
type Args struct {
	Ref       string `json:"ref"`
	Stream    string `json:"stream,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	MaxBytes  int    `json:"max_bytes,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Query     string `json:"query,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

func (s Service) Call(name string, a Args) (any, error) {
	raw, record, err := s.Store.Get(a.Ref)
	if err != nil {
		return nil, fmt.Errorf("cannot recover reference: %w", err)
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("cached original is not UTF-8 text")
	}
	if name == "tokenslim_describe" {
		return map[string]any{"record": record, "streams": streams(record.Format), "max_page_bytes": MaxPageBytes, "max_lines": MaxLines}, nil
	}
	stream := a.Stream
	if stream == "" {
		if name == "tokenslim_get_original" {
			stream = "raw"
		} else {
			stream = "auto"
		}
	}
	text, stream, err := selectText(raw, record.Format, stream)
	if err != nil {
		return nil, err
	}
	base := map[string]any{"ref": a.Ref, "stream": stream, "total_bytes": len(text)}
	switch name {
	case "tokenslim_get_original":
		size := a.MaxBytes
		if size == 0 {
			size = 16 << 10
		}
		if size < 1 || size > MaxPageBytes || a.Offset < 0 || a.Offset > len(text) {
			return nil, fmt.Errorf("invalid offset or max_bytes (1..%d)", MaxPageBytes)
		}
		if a.Offset < len(text) && !utf8.RuneStart(text[a.Offset]) {
			return nil, fmt.Errorf("offset must be a UTF-8 boundary")
		}
		end := min(len(text), a.Offset+size)
		for end > a.Offset && end < len(text) && !utf8.RuneStart(text[end]) {
			end--
		}
		if end == a.Offset && end < len(text) {
			return nil, fmt.Errorf("max_bytes is too small for the next UTF-8 character")
		}
		base["text"], base["offset"], base["next_offset"], base["has_more"] = text[a.Offset:end], a.Offset, end, end < len(text)
		return base, nil
	case "tokenslim_get_range":
		if a.StartLine < 1 || a.EndLine < a.StartLine || a.EndLine-a.StartLine >= MaxLines {
			return nil, fmt.Errorf("use inclusive 1-based start_line/end_line, at most %d lines", MaxLines)
		}
		var b strings.Builder
		last, count := 0, 0
		for line := range strings.SplitAfterSeq(text, "\n") {
			if line == "" {
				continue
			}
			count++
			if count < a.StartLine {
				continue
			}
			if count > a.EndLine {
				break
			}
			if b.Len()+len(line) > MaxPageBytes {
				return nil, fmt.Errorf("range exceeds %d bytes; use get_original byte pagination", MaxPageBytes)
			}
			b.WriteString(line)
			last = count
		}
		if last == 0 {
			return nil, fmt.Errorf("start_line exceeds available lines")
		}
		base["text"], base["start_line"], base["end_line"] = b.String(), a.StartLine, last
		return base, nil
	case "tokenslim_search_original":
		if a.Query == "" || len(a.Query) > 4096 {
			return nil, fmt.Errorf("query must contain 1..4096 bytes")
		}
		limit := a.Limit
		if limit == 0 {
			limit = 20
		}
		start := a.StartLine
		if start == 0 {
			start = 1
		}
		if limit < 1 || limit > MaxLines || start < 1 {
			return nil, fmt.Errorf("invalid limit or start_line")
		}
		matches := []map[string]any{}
		count, used, next, more := 0, 0, 0, false
		for line := range strings.SplitAfterSeq(text, "\n") {
			if line == "" {
				continue
			}
			count++
			if count < start || !strings.Contains(line, a.Query) {
				continue
			}
			if len(matches) >= limit || used+len(line) > MaxPageBytes {
				if len(matches) == 0 {
					return nil, fmt.Errorf("matching line exceeds page size; use get_original byte pagination")
				}
				next, more = count, true
				break
			}
			matches = append(matches, map[string]any{"line": count, "text": line})
			used += len(line)
		}
		base["matches"], base["has_more"], base["next_line"] = matches, more, next
		return base, nil
	}
	return nil, fmt.Errorf("unknown recovery tool")
}

func streams(format string) []string {
	switch format {
	case "claude-bash-json":
		return []string{"raw", "stdout", "stderr"}
	case "codex-tool-json":
		return []string{"raw", "output"}
	default:
		return []string{"raw"}
	}
}
func selectText(raw []byte, format, stream string) (string, string, error) {
	if stream == "auto" {
		stream = "raw"
		switch format {
		case "claude-bash-json":
			stream = "stdout"
		case "codex-tool-json":
			stream = "output"
		}
	}
	if stream == "raw" {
		return string(raw), stream, nil
	}
	allowed := false
	for _, v := range streams(format) {
		if stream == v {
			allowed = true
		}
	}
	if !allowed {
		return "", "", fmt.Errorf("stream %q is unavailable for format %q", stream, format)
	}
	var text string
	if format == "codex-tool-json" && json.Unmarshal(raw, &text) == nil {
		return text, stream, nil
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || len(object[stream]) == 0 || string(object[stream]) == "null" || json.Unmarshal(object[stream], &text) != nil {
		return "", "", fmt.Errorf("cached response has no text stream %q", stream)
	}
	return text, stream, nil
}
