package compressor

import (
	"fmt"
	"strings"
	"tokenslim/internal/rules"
)

func aggregateRules(s, command string, configured []rules.Rule) (string, map[string]rules.Effect) {
	active := []rules.Rule{}
	for _, r := range configured {
		if r.MatchesCommand(command) {
			active = append(active, r)
		}
	}
	if len(active) == 0 {
		return s, nil
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	effects := map[string]rules.Effect{}
	diagnostic := false
	protected := func(line string) bool { return buildDiagnostic(line) || strings.Contains(line, "[TokenSlim") }
	for i := 0; i < len(lines); {
		if buildDiagnostic(lines[i]) {
			diagnostic = true
		}
		if diagnostic || lines[i] == "" || strings.Contains(lines[i], "[TokenSlim") {
			out = append(out, lines[i])
			i++
			continue
		}
		matched := false
		for _, r := range active {
			key, ok := r.Key(lines[i])
			if !ok {
				continue
			}
			j := i + 1
			originalSize := len(lines[i])
			for j < len(lines) && lines[j] != "" && !protected(lines[j]) {
				next, ok := r.Key(lines[j])
				if !ok || next != key {
					break
				}
				originalSize += 1 + len(lines[j])
				j++
			}
			marker := fmt.Sprintf("[TokenSlim rule=%s: %d lines aggregated]", r.Name, j-i)
			if j-i > 1 && len(lines[i])+1+len(marker) < originalSize {
				out = append(out, lines[i], marker)
				effect := effects[r.Name]
				effect.Groups++
				effect.Lines += j - i
				effects[r.Name] = effect
				i = j
			} else {
				out = append(out, lines[i])
				i++
			}
			matched = true
			break // First matching rule owns the line, even without savings.
		}
		if !matched {
			out = append(out, lines[i])
			i++
		}
	}
	return strings.Join(out, "\n"), effects
}
