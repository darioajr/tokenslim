// Package rules compiles explicit, deterministic user aggregation rules.
package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type Spec struct {
	Name    string `yaml:"name"`
	Enabled *bool  `yaml:"enabled,omitempty"`
	Match   struct {
		Command string `yaml:"command"`
		Regex   string `yaml:"regex"`
	} `yaml:"match"`
	IgnoreGroups []string `yaml:"ignore_groups,omitempty"`
	Action       struct {
		Aggregate bool `yaml:"aggregate"`
	} `yaml:"action"`
}
type Effect struct {
	Groups int `json:"groups"`
	Lines  int `json:"lines"`
}
type Rule struct {
	Name          string
	command, line *regexp.Regexp
	ignored       []int
}

var namePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

func Compile(specs []Spec) ([]Rule, error) {
	if len(specs) > 32 {
		return nil, fmt.Errorf("at most 32 custom rules are allowed")
	}
	result := []Rule{}
	names := map[string]bool{}
	for _, s := range specs {
		if !namePattern.MatchString(s.Name) || names[s.Name] {
			return nil, fmt.Errorf("invalid or duplicate rule name %q", s.Name)
		}
		names[s.Name] = true
		if s.Match.Command == "" || len(s.Match.Command) > 512 || strings.ContainsAny(s.Match.Command, "\r\n") {
			return nil, fmt.Errorf("rule %s: command must contain 1..512 bytes without newlines", s.Name)
		}
		if s.Match.Regex == "" || len(s.Match.Regex) > 2048 || !s.Action.Aggregate || len(s.IgnoreGroups) > 8 {
			return nil, fmt.Errorf("rule %s: provide regex (1..2048 bytes), aggregate: true, and at most 8 ignored groups", s.Name)
		}
		pattern := regexp.QuoteMeta(s.Match.Command)
		pattern = strings.ReplaceAll(strings.ReplaceAll(pattern, `\*`, ".*"), `\?`, ".")
		command, err := regexp.Compile("^(?:" + pattern + ")$")
		if err != nil {
			return nil, err
		}
		line, err := regexp.Compile("^(?:" + s.Match.Regex + ")$")
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", s.Name, err)
		}
		r := Rule{Name: s.Name, command: command, line: line}
		seen := map[string]bool{}
		for _, name := range s.IgnoreGroups {
			index := line.SubexpIndex(name)
			occurrences := 0
			for _, capture := range line.SubexpNames() {
				if name == capture {
					occurrences++
				}
			}
			if name == "" || index < 1 || occurrences != 1 || seen[name] {
				return nil, fmt.Errorf("rule %s: ignored group %q must name one unique capture", s.Name, name)
			}
			seen[name] = true
			r.ignored = append(r.ignored, index)
		}
		if s.Enabled == nil || *s.Enabled {
			result = append(result, r)
		}
	}
	return result, nil
}
func (r Rule) MatchesCommand(command string) bool { return r.command.MatchString(command) }

// Key keeps all bytes except explicitly ignored named captures. Sentinels
// preserve capture boundaries; nested/overlapping ignored captures fail closed.
func (r Rule) Key(line string) (string, bool) {
	if strings.ContainsRune(line, 0) {
		return "", false
	}
	indexes := r.line.FindStringSubmatchIndex(line)
	if indexes == nil || indexes[0] != 0 || indexes[1] != len(line) {
		return "", false
	}
	type span struct{ start, end int }
	spans := []span{}
	for _, index := range r.ignored {
		start, end := indexes[index*2], indexes[index*2+1]
		if start >= 0 {
			spans = append(spans, span{start, end})
		}
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start == spans[j].start {
			return spans[i].end < spans[j].end
		}
		return spans[i].start < spans[j].start
	})
	var b strings.Builder
	cursor := 0
	for _, p := range spans {
		if p.start < cursor {
			return "", false
		}
		b.WriteString(line[cursor:p.start])
		b.WriteByte(0)
		cursor = p.end
	}
	b.WriteString(line[cursor:])
	return b.String(), true
}
