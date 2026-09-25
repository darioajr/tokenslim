package classifier

import (
	"regexp"
	"strings"
)

var phpInterpreter = regexp.MustCompile(`^php([0-9]+(\.[0-9]+)*)?(\.exe)?$`)

// PHPReduction deliberately accepts only simple invocations. Compound shell
// commands, quoted expressions and unknown wrappers keep generic normalization:
// their output cannot safely be attributed to a single PHP producer.
func PHPReduction(command string) string {
	if strings.ContainsAny(command, ";&|<>\n\r\"'`$()") {
		return ""
	}
	args := strings.Fields(command)
	if len(args) == 0 {
		return ""
	}
	base := func(s string) string {
		s = strings.ReplaceAll(s, `\`, "/")
		s = s[strings.LastIndex(s, "/")+1:]
		return s
	}
	if phpInterpreter.MatchString(base(args[0])) {
		args = args[1:]
		for len(args) > 0 && strings.HasPrefix(args[0], "-") {
			switch args[0] {
			case "-d", "-c", "--define", "--php-ini":
				if len(args) < 2 {
					return ""
				}
				args = args[2:]
			case "-n", "--no-php-ini", "-f", "--file":
				args = args[1:]
			default:
				return ""
			}
		}
		if len(args) == 0 {
			return ""
		}
	}
	entry := base(args[0])
	args = args[1:]
	switch entry {
	case "phpunit", "phpunit.phar", "phpunit.bat", "pest", "pest.bat":
		return "php-test"
	case "sail":
		if len(args) == 0 {
			return ""
		}
		entry, args = args[0], args[1:]
		if entry == "test" || entry == "pest" || entry == "phpunit" {
			return "php-test"
		}
		if entry != "artisan" {
			return ""
		}
		fallthrough
	case "artisan":
		// Accept only switches whose values are attached, so e.g. --env=test cannot
		// be mistaken for the test subcommand of a different Artisan operation.
		for len(args) > 0 && strings.HasPrefix(args[0], "-") {
			if !strings.Contains(args[0], "=") && args[0] != "--no-ansi" && args[0] != "--ansi" && args[0] != "--no-interaction" && args[0] != "-n" {
				return ""
			}
			args = args[1:]
		}
		if len(args) > 0 && args[0] == "test" {
			return "php-test"
		}
	case "composer", "composer.phar", "composer.bat":
		for len(args) > 0 && (args[0] == "--no-interaction" || args[0] == "-n" || args[0] == "--no-ansi" || args[0] == "--ansi") {
			args = args[1:]
		}
		if len(args) > 0 && (args[0] == "install" || args[0] == "update") {
			return "composer"
		}
	}
	return ""
}
