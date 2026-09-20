// Package compressor implements deterministic, local reducers. Unrecognized lines survive verbatim.
package compressor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Context struct {
	Command, Mode   string
	GroupTimestamps bool
	DisableRepeats  bool
}
type Result struct {
	Output string
	Lossy  bool
}
type Compressor interface {
	Name() string
	Compress(Context, string) Result
}
type Reducer struct{ Kind string }

func (r Reducer) Name() string { return r.Kind }
func (r Reducer) Compress(c Context, s string) Result {
	clean := Sanitize(s)
	var b bytes.Buffer
	// Compact preserves numeric lexemes, duplicate keys and string escapes.
	if json.Valid([]byte(clean)) && json.Compact(&b, []byte(clean)) == nil {
		return Result{b.String(), b.String() != s}
	}
	if c.Mode == "smart" {
		switch r.Kind {
		case "maven", "node-test":
			clean = reduceSuccess(clean, r.Kind)
		case "kubernetes-log", "docker-log":
			if c.GroupTimestamps {
				clean = groupLogs(clean)
			}
		}
	}
	if !c.DisableRepeats {
		clean = Repeats(clean)
	}
	if clean == "" && s != "" {
		return Result{s, false}
	}
	return Result{clean, clean != s}
}

var ansi = regexp.MustCompile("\x1b(?:\\[[0-?]*[ -/]*[@-~]|\\][^\x07\x1b]*(?:\x07|\x1b\\\\))")
var fileLine = regexp.MustCompile(`\.[a-z][a-z0-9]*:[0-9]+`)
var diagnosticWords = []string{"error", "fatal", "fail", "warn", "panic", "exception", "caused by", "suppressed:", "assert", "expected", "actual", "received", "unreachable", "destroy", "replacement", "security", "vulnerab", "race", "snapshot", "coverage", "tests run", "test run", "test suites", "test files", "tests suites", "tests files", "build success", "build failure", "plan:", "play recap"}

func Critical(s string) bool {
	lower := strings.ToLower(s)
	for _, word := range diagnosticWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return strings.Contains(lower, ":") && fileLine.MatchString(lower)
}
func Sanitize(s string) string {
	s = ansi.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	// A bare CR may precede an error; retain every redraw as a separate line.
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blanks := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			blanks++
		} else {
			blanks = 0
		}
		if blanks <= 2 {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
func Repeats(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] && lines[i] != "" {
			j++
		}
		line := lines[i]
		if j-i > 1 {
			line += fmt.Sprintf(" [repeated x%d]", j-i)
		}
		out = append(out, line)
		i = j
	}
	return strings.Join(out, "\n")
}

var transfer = regexp.MustCompile(`^(?:\[INFO\] )?(?:Downloading|Downloaded) from [^:]+: https?://\S+(?: \([^\r\n]+\))?$`)
var passed = regexp.MustCompile(`^\s*PASS\s+\S+.*$`)

func reduceSuccess(s, kind string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	count := 0
	flush := func() {
		if count > 0 {
			label := "dependency transfers"
			if kind == "node-test" {
				label = "passed suites"
			}
			out = append(out, fmt.Sprintf("[TokenSlim: %d %s omitted]", count, label))
			count = 0
		}
	}
	// After a diagnostic starts, preserve its complete body, even lines that look like noise.
	diagnostic := false
	for _, l := range lines {
		if Critical(l) {
			diagnostic = true
		}
		noise := kind == "maven" && transfer.MatchString(l) || kind == "node-test" && passed.MatchString(l)
		if noise && !diagnostic {
			count++
			continue
		}
		flush()
		out = append(out, l)
	}
	flush()
	return strings.Join(out, "\n")
}

var timestamp = regexp.MustCompile(`^(.*?)(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})) (.*)$`)

func logParts(s string) (key, ts string) {
	m := timestamp.FindStringSubmatch(s)
	if m == nil || Critical(s) {
		return "", ""
	}
	if _, e := time.Parse(time.RFC3339Nano, m[2]); e != nil {
		return "", ""
	}
	return m[1] + m[3], m[2]
}
func groupLogs(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		key, first := logParts(lines[i])
		j := i + 1
		last := first
		if key != "" {
			for j < len(lines) {
				k, t := logParts(lines[j])
				if k != key {
					break
				}
				last = t
				j++
			}
		}
		line := lines[i]
		if j-i > 1 {
			line += fmt.Sprintf(" [x%d; %s..%s]", j-i, first, last)
		}
		out = append(out, line)
		i = j
	}
	return strings.Join(out, "\n")
}

// Intact checks normalized critical lines as an ordered subsequence. An adjacent
// exact-repeat marker may account for repeated occurrences of the same line.
func Intact(original, output string) bool {
	if original != "" && output == "" {
		return false
	}
	// JSON compaction changes lines but preserves all bytes within values.
	var b bytes.Buffer
	if json.Valid([]byte(original)) && json.Compact(&b, []byte(original)) == nil && b.String() == output {
		return true
	}
	cursor := 0
	prev := ""
	for _, line := range strings.Split(Sanitize(original), "\n") {
		if !Critical(line) {
			prev = ""
			continue
		}
		if line == prev {
			continue
		}
		at := strings.Index(output[cursor:], line)
		if at < 0 {
			return false
		}
		cursor += at + len(line)
		prev = line
	}
	return true
}
