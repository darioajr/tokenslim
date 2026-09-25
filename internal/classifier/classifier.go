package classifier

import (
	"regexp"
	"strings"
)

var patterns = []struct {
	name string
	re   *regexp.Regexp
}{
	// Match specific PHP entry points before the interpreter or Composer wrappers.
	{"laravel", regexp.MustCompile(`(^|[\s;&|/\\])(artisan|sail)(\s|$)`)},
	{"php-test", regexp.MustCompile(`(^|[\s;&|/\\])(phpunit|pest)(\.phar|\.bat)?(\s|$)`)},
	{"composer", regexp.MustCompile(`(^|[\s;&|/\\])composer(\.phar|\.bat)?(\s|$)`)},
	{"php", regexp.MustCompile(`(^|[\s;&|/\\])php([0-9]+(\.[0-9]+)*)?(\.exe)?(\s|$)`)},
	{"maven", regexp.MustCompile(`(^|[\s;&|/])(mvn|mvnw)(\s|$)`)},
	{"gradle", regexp.MustCompile(`(^|[\s;&|/])(gradle|gradlew)(\s|$)`)},
	{"node-test", regexp.MustCompile(`(^|[\s;&|/])((npm|pnpm|yarn)\s+(run\s+)?test|jest|vitest|mocha)(\s|$)`)},
	{"pytest", regexp.MustCompile(`(^|[\s;&|/])pytest(\s|$)`)},
	{"go-test", regexp.MustCompile(`(^|[\s;&|/])go\s+test(\s|$)`)},
	{"rust-test", regexp.MustCompile(`(^|[\s;&|/])cargo\s+test(\s|$)`)},
	{"kubernetes-log", regexp.MustCompile(`(^|[\s;&|/])kubectl\s+logs(\s|$)`)},
	{"docker-log", regexp.MustCompile(`(^|[\s;&|/])docker\s+(compose\s+)?logs(\s|$)`)},
	{"terraform", regexp.MustCompile(`(^|[\s;&|/])terraform\s+(plan|apply)(\s|$)`)},
	{"ansible", regexp.MustCompile(`(^|[\s;&|/])ansible-playbook(\s|$)`)},
}

func Detect(command, output string) string {
	for _, p := range patterns {
		if p.re.MatchString(command) {
			return p.name
		}
	}
	if strings.Contains(output, "[INFO] BUILD ") || strings.Contains(output, "[ERROR] Failed to execute goal") {
		return "maven"
	}
	if strings.Contains(output, "Test Suites:") || strings.Contains(output, "Test Files ") {
		return "node-test"
	}
	return "generic"
}
