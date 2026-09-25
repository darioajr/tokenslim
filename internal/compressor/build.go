package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	pytestPassed = regexp.MustCompile(`^\S+\.py::.+ PASSED(?:[ \t]+\[[ \t]*[0-9]{1,3}%\])?$`)
	vitestPassed = regexp.MustCompile(`^[ \t]*[✓✔] \S+\.(?:[cm]?[jt]sx?) \([0-9]+ tests?\)[ \t]+[0-9]+(?:\.[0-9]+)?m?s$`)
	gradleCached = regexp.MustCompile(`^> Task :[A-Za-z0-9_.:/-]+ (?:UP-TO-DATE|FROM-CACHE|NO-SOURCE)$`)
	goRun        = regexp.MustCompile(`^=== RUN[ \t]+(\S+)$`)
	goPassed     = regexp.MustCompile(`^--- PASS: (\S+) \([0-9]+(?:\.[0-9]+)?s\)$`)
	pytestBody   = regexp.MustCompile(`^_{3,} .+ _{3,}$`)
)

func buildDiagnostic(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return Critical(line) || pytestBody.MatchString(line) ||
		strings.Contains(lower, "xfail") || strings.Contains(lower, "xpass") ||
		strings.HasPrefix(lower, "stdout |") || strings.HasPrefix(lower, "stderr |") ||
		strings.Contains(lower, "captured stdout") || strings.Contains(lower, "captured stderr") ||
		strings.HasPrefix(lower, "e: ") || strings.HasPrefix(lower, "w: ") ||
		strings.HasPrefix(lower, "=== pause") || strings.HasPrefix(lower, "=== cont") ||
		strings.Contains(lower, "skip")
}

// reduceBuild only removes whole recognized success records. Go's RUN/PASS
// pair must be adjacent and name the same test; logs and parallel interleaving
// are never swallowed. Package summaries and all unknown lines are retained.
func reduceBuild(s, kind string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	label := map[string]string{"pytest": "passed tests", "vitest": "passed test files", "go-test": "passed tests", "gradle": "cached or no-source tasks"}[kind]
	count := 0
	flush := func() {
		if count > 0 {
			out = append(out, fmt.Sprintf("[TokenSlim: %d %s omitted]", count, label))
			count = 0
		}
	}
	diagnostic := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if buildDiagnostic(line) {
			diagnostic = true
		}
		matched := false
		if !diagnostic {
			switch kind {
			case "pytest":
				matched = pytestPassed.MatchString(line)
			case "vitest":
				matched = vitestPassed.MatchString(line)
			case "gradle":
				matched = gradleCached.MatchString(line)
			case "go-test":
				if run := goRun.FindStringSubmatch(line); run != nil && i+1 < len(lines) && !buildDiagnostic(lines[i+1]) {
					passed := goPassed.FindStringSubmatch(lines[i+1])
					if passed != nil && passed[1] == run[1] {
						matched = true
						i++
					}
				}
			}
		}
		if matched {
			count++
			continue
		}
		flush()
		out = append(out, line)
	}
	flush()
	return strings.Join(out, "\n")
}
