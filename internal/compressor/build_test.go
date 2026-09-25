package compressor

import (
	"os"
	"strings"
	"testing"
)

var buildCommands = map[string]string{"pytest": "python3 -m pytest -v", "go-test": "go test -v", "vitest": "npx vitest run", "gradle": "./gradlew build --console=plain"}

func TestBuildGolden(t *testing.T) {
	for kind, command := range buildCommands {
		t.Run(kind, func(t *testing.T) {
			input, err := os.ReadFile("../../testdata/" + kind + ".input.txt")
			if err != nil {
				t.Fatal(err)
			}
			expected, err := os.ReadFile("../../testdata/" + kind + ".expected.txt")
			if err != nil {
				t.Fatal(err)
			}
			for _, source := range []string{string(input), strings.ReplaceAll(string(input), "\n", "\r\n"), "\x1b[32m" + string(input) + "\x1b[0m"} {
				got := (Reducer{kind}).Compress(Context{Command: command, Mode: "smart"}, source).Output
				if got != string(expected) || !Intact(source, got) {
					t.Fatalf("got %q, want %q", got, expected)
				}
			}
			if got := (Reducer{kind}).Compress(Context{Command: command, Mode: "safe"}, string(input)).Output; got != string(input) {
				t.Fatal("safe suppression")
			}
			if got := (Reducer{kind}).Compress(Context{Command: command + " && cat other.log", Mode: "smart"}, string(input)).Output; got != string(input) {
				t.Fatal("compound suppression")
			}
		})
	}
}

func TestBuildDiagnosticBodies(t *testing.T) {
	body := "tests/test_a.py::test_ok PASSED [100%]\n ✓ tests/a.test.ts (1 test) 2ms\n> Task :compile UP-TO-DATE\n=== RUN   TestA\n--- PASS: TestA (0.00s)\n"
	for _, diagnostic := range []string{"FAIL test", "tests/test_a.py::test_known XFAIL", "tests/test_a.py::test_fixed XPASS", "________________ test_case ________________", "stdout | test case", "stderr | test case", "w: compiler message", "e: compiler message", "=== PAUSE TestA", "SKIPPED test", "AssertionError: mismatch", "WARNING: DATA RACE", "panic: boom"} {
		for kind, command := range buildCommands {
			s := diagnostic + "\n" + body
			if got := (Reducer{kind}).Compress(Context{Command: command, Mode: "smart"}, s).Output; got != s {
				t.Errorf("%s removed %q body: %q", kind, diagnostic, got)
			}
		}
	}
}

func TestUnrecognizedBuildOutput(t *testing.T) {
	for kind, s := range map[string]string{
		"go-test": "=== RUN   TestA\n    a_test.go:4: useful message\n--- PASS: TestA (0.00s)\n=== RUN   TestB\n--- PASS: TestC (0.00s)\nPASS\nok  example.test/app 0.002s\n",
		"pytest":  "tests/test_a.py ... [100%]\n3 passed in 0.03s\n",
		"vitest":  " ✓ tests/a.test.ts (2 tests | 1 skipped) 2ms\n ✓ tests/b.test.ts (2 tests) 3ms\n Test Files 2 passed (2)\n",
		"gradle":  "> Task :compileJava\n> Task :test SKIPPED\n> Task :jar UP-TO-DATE\nBUILD SUCCESSFUL in 1s\n",
	} {
		if got := (Reducer{kind}).Compress(Context{Command: buildCommands[kind], Mode: "smart"}, s).Output; got != s {
			t.Errorf("%s: %q", kind, got)
		}
	}
}
