package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	dotnetHeader = regexp.MustCompile(`^(?:Test run for .+\.(?:dll|exe)(?: \(.+\))?|A total of [0-9]+ test files matched the specified pattern\.)$`)

	playwrightLocation = regexp.MustCompile(`\.[cm]?[jt]s:[0-9]+:[0-9]+`)
	rustPassed         = regexp.MustCompile(`^test [A-Za-z0-9_:]+ \.\.\. ok$`)
	dotnetPassed       = regexp.MustCompile(`^[ \t]+Passed[ \t]+\S[^\r\n]* \[(?:< ?)?[0-9]+(?:\.[0-9]+)? (?:ms|s)\]$`)
	playwrightPassed   = regexp.MustCompile(`^[ \t]+(?:[✓✔][ \t]+[0-9]+|[0-9]+[ \t]+[✓✔])[ \t]+(?:\[[^\]\r\n]+\] › )?\S+\.[cm]?[jt]s:[0-9]+:[0-9]+ › .+ \([0-9]+(?:\.[0-9]+)?m?s\)$`)
	playwrightIssue    = regexp.MustCompile(`^(?:[x×-][ \t]+[0-9]+|[0-9]+[ \t]+[x×-])[ \t]+`)

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
		strings.Contains(lower, "ignored") || strings.Contains(lower, "retry") || strings.Contains(lower, "flaky") ||
		playwrightIssue.MatchString(strings.TrimSpace(line)) ||
		strings.Contains(lower, "xfail") || strings.Contains(lower, "xpass") ||
		strings.HasPrefix(lower, "stdout |") || strings.HasPrefix(lower, "stderr |") ||
		strings.Contains(lower, "standard output messages") || strings.Contains(lower, "standard error messages") ||
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
	label := map[string]string{"pytest": "passed tests", "vitest": "passed test files", "go-test": "passed tests", "gradle": "cached or no-source tasks", "rust-test": "passed tests", "playwright": "passed browser tests", "dotnet-test": "passed tests"}[kind]
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
		if !(kind == "playwright" && playwrightSuccess(line)) && !(kind == "dotnet-test" && dotnetHeader.MatchString(line)) && buildDiagnostic(line) {
			diagnostic = true
		}
		matched := false
		if !diagnostic {
			switch kind {
			case "rust-test":
				matched = rustPassed.MatchString(line)
			case "dotnet-test":
				matched = dotnetPassed.MatchString(line)
			case "playwright":
				matched = playwrightSuccess(line)
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

// A location on a complete passed-test row is metadata. Diagnostic words,
// retry/flaky indicators and failure markers still prevent suppression.
func playwrightSuccess(line string) bool {
	return playwrightPassed.MatchString(line) && !buildDiagnostic(playwrightLocation.ReplaceAllString(line, ".source"))
}
