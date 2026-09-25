# pytest, Go, Vitest and Gradle coverage

Stage 2 adds deterministic success-record reducers to four existing ecosystems.
All operate only in smart mode, with recognized simple command invocations and
normal threshold, budget, cache and integrity checks. Unknown lines survive.

## Supported formats

| Family | Summarized format | Always retained / current limitations |
|---|---|---|
| pytest | `tests/test_x.py::test_name PASSED [ 50%]`, including class/parameter node IDs and records without percentages | Dot progress, xdist worker prefixes, xfail/xpass/skips, failure bodies, fixtures, captured logs and totals |
| Go | Adjacent `=== RUN TestName` and `--- PASS: TestName (0.00s)` naming the same test | Package results, nonadjacent/nested/parallel records, logs, JSON events, benchmarks and races |
| Vitest | `✓ tests/x.test.ts (2 tests) 3ms`, also singular test and JS/TS module extensions | Verbose/tree/dot/custom reporters, legacy numeric-only counts, project-prefixed paths, skipped tests, console sections and totals |
| Gradle | `> Task :module:task UP-TO-DATE`, `FROM-CACHE` or `NO-SOURCE` | Executed tasks without status, SKIPPED/FAILED, rich-console variants, warnings and final build/operation counts |

Diagnostic detection stops specialized suppression for the rest of the stream;
this includes diagnostic words appearing in successful test names. Go PAUSE/CONT
also stops reduction to avoid attributing interleaved output to the wrong test.
Per-test durations in summarized success records are recoverable from the cache;
final timing summaries remain visible. Gradle groups count tasks collectively,
not separately by cache status.

The classifier reports `vitest`; configuration uses the existing `node_test` key.
The other keys are `pytest`, `go_test` and `gradle`. Simple direct commands and
supported launcher forms are listed in [compression rules](../docs/compression-rules.md).
`npm test` script contents are not inspected to discover Vitest. Shell pipelines,
compound commands and unknown wrappers do not enable these specialized reducers.

## Evidence and reproduction

- `testdata/go-test.input.txt` is captured from an actual Go 1.26.8 darwin/arm64
  execution of `go test -v .` in a temporary module `example.test/sample`.
  The source is `testdata/go-test-source.txt`; copy it to `sample_test.go` in a
  disposable module to reproduce. Exit code 1 is intentional. Only expected
  reduced output replaces the first two successful pairs; the input is unedited.
- The pytest, Vitest and Gradle golden fixtures are synthetic examples based on
  [pytest output documentation](https://docs.pytest.org/en/stable/how-to/output.html),
  [Vitest reporters](https://vitest.dev/guide/reporters) and
  [Gradle task outcomes](https://docs.gradle.org/current/userguide/more_about_tasks.html).
  They are not claimed as captured runs or full runtime-version certification.
- Golden comparisons exercise original, CRLF and ANSI-wrapped input, and safe
  mode plus compound-command pass-through. Additional regressions cover logs,
  mismatched Go names, interleaving, skipped tests, compiler diagnostics and
  successful-looking records inside failure bodies.
- `builds.py` generates 16 synthetic scenarios: success, failure, compound command
  and diagnostic-first output for each family. Both adapters run each in safe
  and smart modes, checking diagnostics, stderr, metadata, cache recovery and
  deterministic output. All four configuration keys are tested for disabled,
  safe/off overrides, thresholds, reduction limits and disabled cache.

Run `make test lint demo`. No pytest, Node, Java or Gradle installation is needed
for the synthetic corpus. The checked-in Go capture is replayed without requiring
an additional nested Go process in CI. Fuzzing includes these reducers and sample
success records.

## Synthetic measurements

Snapshot from the working tree based on TokenSlim 0.2.0 plus stage 2. Byte counts
include the recovery marker. Safe mode and the compound/diagnostic-first controls
remain unchanged in these scenarios. These measurements do not predict real
workload or provider billing savings. Full reports are regenerated in `results/`.

| Scenario | Original bytes | Smart bytes | Reduction |
|---|---:|---:|---:|
| pytest-success | 10,582 | 956 | 91.0% |
| pytest-failure | 10,739 | 1,114 | 89.6% |
| go-test-success | 12,150 | 1,626 | 86.6% |
| go-test-failure | 12,280 | 1,756 | 85.7% |
| vitest-success | 10,399 | 1,074 | 89.7% |
| vitest-failure | 10,583 | 1,258 | 88.1% |
| gradle-success | 8,135 | 1,229 | 84.9% |
| gradle-failure | 8,267 | 1,361 | 83.5% |
