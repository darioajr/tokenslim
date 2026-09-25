package compressor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tokenslim/internal/classifier"
)

func TestGolden(t *testing.T) {
	cases := []struct{ name, kind, mode string }{{"generic", "generic", "safe"}, {"maven", "maven", "smart"}, {"node", "node-test", "smart"}, {"kubernetes", "kubernetes-log", "smart"}, {"docker", "docker-log", "smart"}, {"json", "generic", "safe"}, {"composer", "composer", "smart"}, {"php-test", "php-test", "smart"}, {"php-test", "laravel", "smart"}, {"phpunit-testdox", "php-test", "smart"}, {"pest-upstream", "php-test", "smart"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, e := os.ReadFile(filepath.Join("../../testdata", c.name+".input.txt"))
			if e != nil {
				t.Fatal(e)
			}
			want, e := os.ReadFile(filepath.Join("../../testdata", c.name+".expected.txt"))
			if e != nil {
				t.Fatal(e)
			}
			got := (Reducer{c.kind}).Compress(Context{Mode: c.mode, GroupTimestamps: true, Command: map[string]string{"composer": "composer install", "php-test": "vendor/bin/phpunit --testdox", "laravel": "php artisan test"}[c.kind]}, string(a)).Output
			if got != string(want) {
				t.Fatalf("got %q\nwant %q", got, want)
			}
			// Exercise Windows input line endings while keeping the golden output exact.
			crlf := strings.ReplaceAll(string(a), "\n", "\r\n")
			if actual := (Reducer{c.kind}).Compress(Context{Mode: c.mode, GroupTimestamps: true, Command: map[string]string{"composer": "composer install", "php-test": "vendor/bin/phpunit --testdox", "laravel": "php artisan test"}[c.kind]}, crlf).Output; actual != string(want) {
				t.Fatalf("CRLF input: got %q, want %q", actual, want)
			}
			if !Intact(string(a), got) {
				t.Fatal("critical loss")
			}
		})
	}
}
func TestDiagnostics(t *testing.T) {
	s := "ERROR first\rERROR second\nCaused by: PaymentException\n app.java:42\nExpected: 4\nActual: 2\nWARN vulnerability\n"
	got := (Reducer{"maven"}).Compress(Context{Mode: "smart"}, s).Output
	for _, line := range strings.Split(Sanitize(s), "\n") {
		if !strings.Contains(got, line) {
			t.Fatal(line)
		}
	}
	if Intact(s, "ERROR first\n") {
		t.Fatal("guard accepted missing errors")
	}
	if Intact("ERROR A\nERROR B", "ERROR B\nERROR A") {
		t.Fatal("guard accepted reordered errors")
	}
}
func TestNoCrossContainerOrSeverityGrouping(t *testing.T) {
	s := "a | 2026-09-19T10:00:00Z INFO ready\nb | 2026-09-19T10:00:01Z INFO ready\na | 2026-09-19T10:00:02Z WARN ready\na | 2026-09-19T10:00:03Z WARN ready\n"
	if groupLogs(s) != s {
		t.Fatal("merged containers or warnings")
	}
}
func TestPreserveDiagnosticBody(t *testing.T) {
	s := "FAIL snapshot\nPASS this is expected output\nDownloaded from central: https://example.test/a\n"
	if reduceSuccess(s, "node-test") != s || reduceSuccess(s, "maven") != s {
		t.Fatal("removed diagnostic body")
	}
}
func TestJSONNumbers(t *testing.T) {
	s := "{\n\"error\": 9007199254740993,\n\"same\":1, \"same\":2\n}"
	got := (Reducer{"generic"}).Compress(Context{Mode: "safe"}, s).Output
	if got != "{\"error\":9007199254740993,\"same\":1,\"same\":2}" {
		t.Fatal(got)
	}
}
func FuzzReducer(f *testing.F) {
	for _, s := range []string{"ERROR failure\rprogress", "\x1b[31mred\x1b[0m", "{\"a\":1}", "retry\nretry\n", "tests/test_a.py::test_a PASSED [100%]\n", "=== RUN   TestA\n--- PASS: TestA (0.00s)\n", " ✓ tests/a.test.ts (1 test) 1ms\n", "> Task :compileJava UP-TO-DATE\n"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1<<20 {
			t.Skip()
		}
		for _, kind := range []string{"generic", "maven", "node-test", "kubernetes-log", "docker-log", "php", "composer", "php-test", "laravel", "pytest", "go-test", "vitest", "gradle"} {
			r := Reducer{kind}
			a := r.Compress(Context{Mode: "smart", GroupTimestamps: true, Command: map[string]string{"composer": "composer install", "php-test": "vendor/bin/pest", "laravel": "php artisan test", "pytest": "pytest -v", "go-test": "go test -v", "vitest": "vitest run", "gradle": "gradle build"}[kind]}, s)
			b := r.Compress(Context{Mode: "smart", GroupTimestamps: true, Command: map[string]string{"composer": "composer install", "php-test": "vendor/bin/pest", "laravel": "php artisan test", "pytest": "pytest -v", "go-test": "go test -v", "vitest": "vitest run", "gradle": "gradle build"}[kind]}, s)
			if a != b {
				t.Fatal("nondeterminism")
			}
			if !Intact(s, a.Output) {
				t.Fatal("diagnostics lost")
			}
		}
	})
}
func BenchmarkReducer(b *testing.B) {
	s := strings.Repeat("INFO heartbeat accepted\n", 30000) + "ERROR boom app.go:5\n"
	b.SetBytes(int64(len(s)))
	b.ReportAllocs()
	for b.Loop() {
		(Reducer{"generic"}).Compress(Context{Mode: "safe"}, s)
	}
}

func TestPHPDiagnosticBodies(t *testing.T) {
	for _, diagnostic := range []string{"PHP Deprecated: old API", "PHP Notice: undefined variable", "WARN risky test", "Tests: 1 skipped", "⨯ handles payment", "× handles payment", "FAIL Tests\\Feature\\OrderTest", "SQLSTATE[HY000]: General error", "Stack trace:", "✘ charges a card", "⚠ uses an old API", "∅ handles payment", "↩ handles payment", "1 test triggered 1 deprecation:"} {
		for _, kind := range []string{"composer", "php-test", "laravel"} {
			s := diagnostic + "\n  ✓ successful-looking assertion text\n  PASS Tests\\Unit\\ExampleTest\n  - Downloading vendor/package (1.0.0)\n#0 /app/vendor/framework.php(42): handle()\n"
			got := (Reducer{kind}).Compress(Context{Mode: "smart", Command: map[string]string{"composer": "composer install", "php-test": "vendor/bin/pest", "laravel": "php artisan test", "pytest": "pytest -v", "go-test": "go test -v", "vitest": "vitest run", "gradle": "gradle build"}[kind]}, s).Output
			if got != s {
				t.Errorf("%s removed body for %q: %q", kind, diagnostic, got)
			}
		}
	}
}

func TestPHPSafeMode(t *testing.T) {
	s := "  ✓ works\n  PASS Tests\\Unit\\ExampleTest\n  - Downloading vendor/package (1.0.0)\n"
	for _, kind := range []string{"php", "composer", "php-test", "laravel"} {
		if got := (Reducer{kind}).Compress(Context{Mode: "safe"}, s).Output; got != s {
			t.Errorf("%s: %q", kind, got)
		}
	}
}

func TestPHPCommandScope(t *testing.T) {
	s := "  ✓ customer record was migrated\n  PASS customer record was verified\n  - Downloading vendor/package (1.0.0)\n"
	for _, command := range []string{"php artisan migrate", "php artisan queue:work", "php artisan custom:report test", "sail artisan migrate", "php artisan --env test migrate", "cat vendor/bin/pest", "echo php artisan test", "php artisan test && php artisan migrate", "composer run-script install", "composer test", "composer show"} {
		kind := classifier.Detect(command, s)
		got := (Reducer{kind}).Compress(Context{Mode: "smart", Command: command}, s).Output
		if got != s {
			t.Errorf("%s changed non-test output: %q", command, got)
		}
	}
}

func TestComposerScriptBoundary(t *testing.T) {
	s := "  - Downloading vendor/one (1.0.0)\n> @php artisan custom:report\n  - Downloading vendor/two (2.0.0)\n"
	want := "[TokenSlim: 1 dependency transfers omitted]\n> @php artisan custom:report\n  - Downloading vendor/two (2.0.0)\n"
	if got := (Reducer{"composer"}).Compress(Context{Mode: "smart", Command: "composer install"}, s).Output; got != want {
		t.Fatalf("got %q", got)
	}
}
