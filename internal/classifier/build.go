package classifier

import (
	"path"
	"regexp"
	"strings"
)

var pythonInterpreter = regexp.MustCompile(`^python(?:[0-9]+(?:\.[0-9]+)*)?(?:\.exe)?$`)

// BuildReduction accepts simple invocations whose output has an identified
// producer. Shell pipelines, scripts and unknown wrappers remain conservative.
func BuildReduction(command string) string {
	if strings.ContainsAny(command, ";&|<>\n\r\"'`$()") {
		return ""
	}
	args := strings.Fields(command)
	if len(args) == 0 {
		return ""
	}
	executable := path.Base(strings.ReplaceAll(args[0], `\`, "/"))
	switch executable {
	case "pytest", "pytest.exe":
		return "pytest"
	case "go", "go.exe":
		if len(args) > 1 && args[1] == "test" {
			return "go-test"
		}
	case "gradle", "gradlew", "gradle.bat", "gradlew.bat":
		return "gradle"
	case "vitest", "vitest.cmd":
		return "vitest"
	case "npx", "pnpm", "yarn", "bun":
		args = args[1:]
		if len(args) > 0 && ((executable == "pnpm" || executable == "yarn") && args[0] == "exec" || executable == "bun" && args[0] == "x") {
			args = args[1:]
		}
		if len(args) > 0 && args[0] == "vitest" {
			return "vitest"
		}
	default:
		if pythonInterpreter.MatchString(executable) && len(args) > 2 && args[1] == "-m" && args[2] == "pytest" {
			return "pytest"
		}
	}
	return ""
}
