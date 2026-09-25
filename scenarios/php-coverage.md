# PHP / Laravel consolidation

The suite combines upstream expected-output excerpts with deterministic synthetic
workloads. No PHP, Composer, Laravel, Docker or external service is needed to run
`make test lint demo`. Benchmarks execute TokenSlim, not the PHP tools.

## Format evidence and provenance

The checked-in golden fixtures preserve upstream text; they are **not captures
of PHP executions on this machine**:

- `testdata/phpunit-testdox.input.txt`: the `--EXPECTF--` section of
  [PHPUnit's outcome-and-issues-with-summary.phpt](https://github.com/sebastianbergmann/phpunit/blob/5ebece6ac28a509dc75ac998a6a49553cc3aca2c/tests/end-to-end/testdox/outcome-and-issues-with-summary.phpt).
  `%s` placeholders remain unchanged. Source: PHPUnit 12.5 branch,
  commit `5ebece6ac28a509dc75ac998a6a49553cc3aca2c` (BSD-3-Clause).
- `testdata/pest-upstream.input.txt`: the prefix of
  [Pest's success snapshot](https://github.com/pestphp/pest/blob/5b2293f67adcf1b2320b33f521b94a692d18f360/tests/.snapshots/success.txt),
  ending before `PASS Tests\Features\AfterAll`. Source: Pest 4.x,
  commit `5b2293f67adcf1b2320b33f521b94a692d18f360` (MIT).
  The `Expectation` test name triggers conservative diagnostic protection;
  only the preceding PASS record is summarized.
- The existing Composer and PHP fixtures, and all logs from `php.py`, are
  synthetic examples. Composer package names, counts, versions and timings are
  invented. Parallel worker prefixes are adversarial synthetic input, not a
  claim that every Pest/ParaTest version emits that format.

Golden tests compare exact expected output, also with CRLF input. Regression
tests cover issue symbols (`✘`, `⚠`, `∅`, `↩`, `⨯`, `×`), notices, deprecations,
risky/skipped/incomplete tests, full failure bodies and Composer script boundaries.

## Coverage matrix

| Input / command | Smart behavior | Checked preservation |
|---|---|---|
| Composer install/update | Summarize recognized downloads and archive extractions before a diagnostic or script | Lock ledger, operation counts, autoload and script output |
| PHPUnit TestDox | Summarize indented successful checkmarks | Suite headings, issue/failure bodies, timing, memory and totals |
| PHPUnit default dot progress | Generic normalization | Progress counters and complete summary |
| Pest, Artisan test, Sail test | Summarize indented PASS and successful checkmarks | Failure details, source locations, stack frames, totals, coverage, duration |
| Parallel worker-prefixed output | Generic normalization | Worker identity and unknown records |
| Migrations, queues, custom Artisan commands | Generic normalization even for PASS/checkmarks | All domain records |
| Composer scripts, compound or quoted shell commands, unknown wrappers | Generic normalization | Output that cannot be safely attributed to a supported producer |

Safe mode never applies the specialized PHP rules. Smart mode stops suppressing
records after the first recognized diagnostic in each stream. This deliberately
limits savings when a test name contains diagnostic words. Standard unprefixed
success records may still be recognized when `--parallel` is used; interleaved,
unknown formats are retained. No stack-frame removal is implemented.

## Adapter and configuration checks

Every synthetic case runs through both adapters in safe and smart modes. The
runner checks expected change/pass-through, original and optimized byte counts,
recovery marker overhead, determinism, diagnostic retention, success/failure exit
codes, and exact recovery of the raw cached response JSON. Claude's separate
stdout/stderr and Codex's combined output are exercised. Composer update is
placed on stderr for the Claude case; other cases retain a separate stderr
message. Duration and other accepted metadata survive replacement.

Additional CLI and adapter checks verify global off, disabled PHP test compressor,
per-compressor safe/off overrides, byte/line thresholds, reduction budget and
cache-disabled pass-through. The Go tests also cover per-family overrides and
cache recovery. Existing generic tests cover cache failure, binary input and
interrupted/live executions.

## Measured snapshot

Generated with the working tree based on TokenSlim 0.2.0 plus this consolidation.
The following are synthetic byte measurements including the recovery marker;
all listed safe-mode outputs were unchanged. These are not production or billing
savings. Full token estimates and all control scenarios are in
`results/report.json` and `results/report.md` after running `make demo`.

| Scenario | Original bytes | Smart bytes | Reduction |
|---|---:|---:|---:|
| composer-install | 9,558 | 1,220 | 87.2% |
| composer-update | 12,721 | 7,858 | 38.2% |
| phpunit-testdox | 10,957 | 1,874 | 82.9% |
| pest-success | 10,765 | 1,092 | 89.9% |
| artisan-test | 10,765 | 1,091 | 89.9% |
| sail-test | 10,765 | 1,091 | 89.9% |
| pest-parallel | 12,729 | 12,729 | 0.0% |
| artisan-parallel | 12,729 | 12,729 | 0.0% |
| pest-failure-ansi | 11,334 | 1,372 | 87.9% |
| phpunit-default | 6,522 | 6,522 | 0.0% |

Reproduce with:

```sh
make test lint demo
```

## Remaining runtime validation

PHP, Composer and Docker were unavailable in the implementation environment, so
no local minimal Laravel project or real parallel test process was executed.
Upstream fixtures establish concrete format examples, not compatibility with all
versions or plugins. Before claiming runtime-version compatibility, capture
stdout/stderr and exit status from a disposable minimal project using pinned
Composer dependencies and record `php --version`, `composer --version` and the
lockfile. Include a passing test, an assertion failure, an explicit skip and a
deprecation; run PHPUnit default/TestDox, Pest, Artisan and Sail, including
`--parallel`. Add sanitized captures with provenance and rerun this suite.
