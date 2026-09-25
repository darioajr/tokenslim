package compressor

import (
	"os"
	"strings"
	"testing"
)

var buildCommands = map[string]string{"pytest": "python3 -m pytest -v", "go-test": "go test -v", "vitest": "npx vitest run", "gradle": "./gradlew build --console=plain", "rust-test": "cargo test", "dotnet-test": "dotnet test", "playwright": "npx playwright test"}

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
				if got != string(expected) || !IntactFor(kind, source, got) {
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
	body := "tests/test_a.py::test_ok PASSED [100%]\n ✓ tests/a.test.ts (1 test) 2ms\n> Task :compile UP-TO-DATE\n=== RUN   TestA\n--- PASS: TestA (0.00s)\ntest tests::create ... ok\n  Passed Sample.Tests.Create [1 ms]\n  ✓  1 sample.spec.js:2:1 › creates record (2ms)\n"
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

func TestPlaywrightIntegrity(t *testing.T) {
	success := "  ✓  1 sample.spec.js:2:1 › creates record (2ms)"
	if !Critical(success) || Intact(success, "summary") || !IntactFor("playwright", success, "summary") {
		t.Fatal("success-location exemption is not scoped")
	}
	for _, line := range []string{"sample.spec.js:2:1", "  ✘  1 sample.spec.js:2:1 › creates record (2ms)", "  ✓  1 sample.spec.js:2:1 › expected exception (2ms)", "  ✓  1 sample.spec.js:2:1 › creates record (retry #1) (2ms)"} {
		if IntactFor("playwright", line, "summary") {
			t.Errorf("guard lost diagnostic %q", line)
		}
	}
}

func TestExtendedBuildControls(t *testing.T) {
	for kind, s := range map[string]string{
		"rust-test":   "test tests::later ... ignored\ntest tests::create ... ok\n",
		"dotnet-test": "  Skipped Sample.Tests.Later [1 ms]\n  Passed Sample.Tests.Create [1 ms]\n",
		"playwright":  "  -  1 sample.spec.js:2:1 › later\n  ✓  2 sample.spec.js:3:1 › creates record (2ms)\n",
	} {
		if got := (Reducer{kind}).Compress(Context{Command: buildCommands[kind], Mode: "smart"}, s).Output; got != s {
			t.Errorf("%s removed issue body: %q", kind, got)
		}
	}
}
