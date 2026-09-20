# TokenSlim

A local tool-output compressor for **Claude Code and Codex**. TokenSlim reduces
repetition in builds, tests, and logs, preserves diagnostics, and caches the
original output. It does not modify source code, command arguments, or execution.

## Build and try the demo

Requirements: Go 1.26.8+ and Python 3 for the scenarios.

```sh
make build
make test
make demo
```

Binary: `bin/tokenslim`. The demo generates `scenarios/results/report.md` with
safe/smart comparisons and tests both agents' hook protocols. See the
[scenario guide](scenarios/README.md).

```sh
bin/tokenslim benchmark --mode smart --command 'mvn test' build.log
bin/tokenslim optimize --mode smart --command 'mvn test' build.log
bin/tokenslim stats
bin/tokenslim cache inspect ts_HASH
```

Replace `ts_HASH` with the full reference printed in the output. Benchmark is a
dry run that writes neither cache entries nor metrics. The `optimize` command
and hooks save the original output. Flags must precede the file name; `-` reads
stdin. Token counts are local estimates.

## Supported platforms

Releases contain separate Claude Code and Codex packages for these 64-bit targets:

| System | Architectures | Archive |
|---|---|---|
| Linux | AMD64 (x86-64), ARM64 | `.tar.gz` |
| macOS | AMD64 (Intel), ARM64 (Apple Silicon) | `.tar.gz` |
| Windows | AMD64 (x86-64), ARM64 | `.zip` |

32-bit x86 and ARM are not supported. Download the package matching both your
agent and operating system/architecture, then extract it before installation.
Windows packages contain `bin/tokenslim.exe`; Linux/macOS packages contain
`bin/tokenslim`. WSL uses the Linux package.

### Windows

Install Git for Windows for the Bash hook commands. The current adapters match
only the `Bash` tool; PowerShell tool output is not compressed automatically.
See [Claude Code's Windows setup](https://code.claude.com/docs/en/setup).
The standalone CLI can also run directly from PowerShell:

```powershell
.\bin\tokenslim.exe version
.\bin\tokenslim.exe stats
```

For a source checkout, build and run the scenarios without Make:

```powershell
$env:PYTHONUTF8 = "1"
python scripts/build.py
go test ./...
python scenarios/run.py --check
python scripts/codex-hook-config.py > tokenslim-codex-hooks.json
```

The generated Codex command targets Git Bash and includes the `.exe` suffix on
Windows. Merge its hook block into your Codex hook configuration and trust it as
described below. Packaged hooks select the correct executable automatically.
CI runs native Windows AMD64 tests and package smoke tests; Windows ARM64 is
cross-compiled and archive-validated, without a native ARM64 execution test.

## Claude Code

After running `make build`, start Claude Code from the project root:

```sh
claude --plugin-dir .
```

The plugin registers a `PostToolUse` hook for Bash. The adapter returns
`updatedToolOutput`, preserving stdout/stderr and metadata. Read/Edit/Write,
image outputs, and interrupted executions pass through unchanged. The
implementation was checked against the documentation and its manifest validated
with the local Claude Code CLI; protocol tests do not replace an authenticated
session test.

## Codex

The dedicated package is in `integrations/codex/tokenslim/`. It includes the
manifest, hook, skill, and binary produced by `make build`. To try it without
setting up a marketplace, generate a hook configuration:

```sh
python3 scripts/codex-hook-config.py > /tmp/tokenslim-codex-hooks.json
```

Add the generated PostToolUse block to `~/.codex/hooks.json` or
`<your-project>/.codex/hooks.json`, preserving existing hooks. Open `/hooks` in
Codex to review and trust the hook. The project must also be trusted. The script
only prints configuration; it does not change global preferences.

Codex uses `continue: false` and `stopReason` to replace the result after execution.
It does not use the Claude response contract or block command execution. Strings
and completed execution objects are supported; active sessions and unknown
formats remain unchanged. Code mode and host output limits impose specific
limitations; see [compatibility](docs/architecture.md). This integration requires
a version supporting the hooks described in the
[Codex documentation](https://learn.chatgpt.com/docs/hooks).

## Configuration

Precedence: CLI flags → `.tokenslim.yaml` in the working directory →
`~/.tokenslim/config.yaml` → defaults. `TOKENSLIM_HOME` overrides the state
directory for testing or isolation. Project configuration is not searched for
in ancestor directories.

```yaml
version: 1
mode: safe # off, safe, smart
thresholds:
  minimum_bytes: 4096
  minimum_lines: 40
  minimum_expected_reduction_percent: 10
cache:
  enabled: true
  retention: 7d
  max_size_mb: 1024
metrics:
  enabled: true
```

Both size thresholds must be met. Output without sufficient savings remains
unchanged. Smart mode also reduces Maven transfer messages, PASS suite records,
and timestamped logs. Terraform remains conservative. See the
[compression rules](docs/compression-rules.md) and
[complete configuration example](docs/config.example.yaml).

```sh
bin/tokenslim version
bin/tokenslim status
bin/tokenslim config show
bin/tokenslim config path
bin/tokenslim stats --session ID
bin/tokenslim cache inspect ts_HASH --metadata
bin/tokenslim cache prune
bin/tokenslim cache clear
```

The cache uses zstd and SHA-256, with 0700 directories and 0600 files on Unix.
On Windows, access is controlled by inherited filesystem ACLs. Originals
may contain secrets already present in logs; they remain local. Clearing or
expiring the cache removes the ability to recover those originals. If the cache
fails or is disabled, the original output is retained. Metrics are stored as
atomic local JSON files. There is no telemetry, external service, or model call.
`TOKENSLIM_DEBUG=1` writes metadata for the latest invocation to
`~/.tokenslim/logs/tokenslim.log`.

## Development

```sh
make lint
make vulncheck     # scan reachable Go vulnerabilities
make benchmark
make plugin-test
make workflow-lint # validate GitHub Actions workflows
make release-check # build and verify release packages
make install       # install only the binary into ~/.local/bin
```

Releases include separate Claude/Codex packages for macOS/Linux/Windows and arm64/amd64 (64-bit only),
with checksums. Unit, golden, invariant, and fuzz tests cover the reducers and
adapters. The demo corpus is synthetic; it does not establish billed-token
savings, production p95 latency, or acceptance in live agent sessions.

## GitHub Actions

[CI](.github/workflows/ci.yml) runs tests, the race detector, coverage, fuzzing,
lint, and efficiency scenarios on Linux/macOS/Windows. It also verifies all twelve
Claude/Codex packages and uploads reports as workflow artifacts.

Tags matching `vMAJOR.MINOR.PATCH` trigger the
[release workflow](.github/workflows/release.yml). After repeating validation on
the tagged commit, the pipeline creates a draft GitHub Release with packages and
checksums. `VERSION` and both plugin manifests must agree.

See [setup, pipeline stages, and publishing](docs/ci-cd.md). Workflows start running
once the project is pushed to a GitHub repository with Actions enabled.

See [dependency maintenance](docs/dependencies.md) for module versions and update checks.

Licensed under Apache-2.0.
