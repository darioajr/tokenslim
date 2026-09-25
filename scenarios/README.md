# Efficiency demo

Run from the repository root:

```sh
make demo
```

Requirements: Go and Python 3. No account, API, Kubernetes, Docker, Node, or Maven
is required. The script generates **synthetic** logs, runs the actual binary,
and tests the Claude Code and Codex adapters. It does not execute the tools whose
logs it simulates.

- `generated/`: reproducible input logs containing no private data.
- `results/`: optimized outputs, hook responses, and Markdown/JSON reports.
- `run.py`: generation, measurement, and automated checks.

Scenarios cover Maven failures, Node tests with assertions, timestamped Kubernetes
logs, Docker service identity, generic repetition, Unicode/ANSI, small output,
and output without repetition. The [PHP/Laravel corpus](php-coverage.md) adds
Composer install/update, PHPUnit, Pest, Artisan/Sail, failure/issue diagnostics,
ANSI/CRLF, parallel-shaped output and non-test command controls. The
[build/test corpus](build-coverage.md) adds pytest, Go, Vitest and Gradle success,
failure, compound-command and diagnostic-first scenarios. The
[stage-3 corpus](extended-build-coverage.md) adds Playwright, Cargo and .NET/VSTest
with the same adapter and configuration checks. Each case runs in `safe` and `smart` mode.

Checks cover byte savings including the recovery marker, consistent token
estimates, preservation of selected diagnostics and exit codes, exact cache
recovery (including original ANSI/CRLF bytes and hook JSON), determinism, stdout
and stderr handling, execution metadata, unchanged Read output, and untouched
source files. PHP and build/test checks also cover mode/compressor overrides, thresholds,
reduction budgets and disabled cache. The demo
cache uses an isolated temporary directory that is removed when the run finishes.
Reports remain available in `results/`.

## Comparison by mode

TokenSlim 0.1.0 snapshot from `results/report.json`. All sizes are bytes and
include the recovery marker when output changes. Safe mode is the default.

| Scenario | Without TokenSlim | With TokenSlim: safe | Safe reduction | With TokenSlim: smart | Smart reduction |
|---|---:|---:|---:|---:|---:|
| Maven build | 66,314 | 10,699 | 83.9% | 7,348 | 88.9% |
| Node tests | 29,714 | 29,714 | 0.0% | 6,932 | 76.7% |
| Kubernetes logs | 48,314 | 48,314 | 0.0% | 11,308 | 76.6% |
| Docker logs | 45,434 | 45,434 | 0.0% | 11,064 | 75.6% |
| Repeated worker logs | 47,594 | 9,140 | 80.8% | 9,140 | 80.8% |
| Unicode / ANSI logs | 24,099 | 4,155 | 82.8% | 4,155 | 82.8% |
| Small output | 6 | 6 | 0.0% | 6 | 0.0% |
| Non-repeated output | 6,290 | 6,290 | 0.0% | 6,290 | 0.0% |

Safe mode groups exact repetition. Smart mode also recognizes supported log
patterns, which explains the additional reductions in Node, Kubernetes, and
Docker scenarios. Zero reduction is expected when no eligible compression is
found. These synthetic results do not predict savings for every workload.

Regenerate the report with the commands above before updating this snapshot.

To compare your own log:

```sh
bin/tokenslim benchmark --mode safe --command 'mvn test' your-build.log
bin/tokenslim benchmark --mode smart --command 'mvn test' your-build.log
```

Estimates use Unicode characters/4. They are not provider token counts or promises
of lower bills. The synthetic corpus demonstrates the mechanism; evaluate real
logs before estimating savings for your project. These tests exercise the local
protocol, not an authenticated conversation with either agent.

The Unicode scenario intentionally includes non-English log data to verify that
compression preserves multilingual output. Documentation and report labels are
in English.
