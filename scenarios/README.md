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
and output without repetition. Each case runs in `safe` and `smart` mode.

Checks cover byte savings including the recovery marker, consistent token
estimates, preservation of selected diagnostics and exit codes, exact cache
recovery, determinism, unchanged Read output, and untouched source files. The demo
cache uses an isolated temporary directory that is removed when the run finishes.
Reports remain available in `results/`.

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
