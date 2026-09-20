# TokenSlim

A local tool-output compressor for **Claude Code and Codex**. TokenSlim reduces
repetition in builds, tests, and logs, preserves diagnostics, and caches the
original output. It does not modify source code, command arguments, or execution.

## With and without TokenSlim

The following results come from the reproducible synthetic scenarios in
the [efficiency demo](https://github.com/darioajr/tokenslim/blob/main/scenarios/README.md) using TokenSlim 0.1.0 in **smart mode**. Without TokenSlim, the
original output is retained; with TokenSlim, eligible output is compressed and
a recovery marker is included in the measured size. **Safe mode is the default**;
see the [safe vs. smart comparison](https://github.com/darioajr/tokenslim/blob/main/scenarios/README.md#comparison-by-mode) for
results in both modes.

| Scenario | Without TokenSlim (bytes) | With TokenSlim (bytes) | Byte reduction | Estimated tokens before → after |
|---|---:|---:|---:|---:|
| Maven build | 66,314 | 7,348 | 88.9% | 16,579 → 1,837 |
| Node tests | 29,714 | 6,932 | 76.7% | 7,429 → 1,733 |
| Kubernetes logs | 48,314 | 11,308 | 76.6% | 12,079 → 2,827 |
| Docker logs | 45,434 | 11,064 | 75.6% | 11,359 → 2,766 |
| Repeated worker logs | 47,594 | 9,140 | 80.8% | 11,899 → 2,285 |
| Small output | 6 | 6 | 0.0% | 2 → 2 |
| Unicode / ANSI logs | 24,099 | 4,155 | 82.8% | 5,425 → 964 |
| Non-repeated output | 6,290 | 6,290 | 0.0% | 1,573 → 1,573 |

These are synthetic log measurements, not production benchmarks or billing
savings. Token estimates use Unicode characters divided by four, rounded up;
actual model tokenization varies. Both adapters are checked for preservation of
selected diagnostics and execution metadata, plus exact recovery of cached
originals. Small or non-repeated output remains unchanged. These checks do not
measure complete conversations, latency improvements, or provider charges.

For reproduction instructions, see [Building and development](docs/build.md).

## Install from a GitHub release

Download the ready-to-use package from
[GitHub Releases](https://github.com/darioajr/tokenslim/releases). You do not need
Go, Make, or a source checkout to run TokenSlim.

1. Open a published release and expand **Assets**.
2. Download the package for your agent (`claude` or `codex`), system, and
   architecture, together with `SHA256SUMS` from the same release.
3. Compare the archive's SHA-256 with its entry in `SHA256SUMS`, then extract
   the archive into a permanent directory.
4. Follow the Claude Code or Codex instructions below using that directory.

Choose an attached `tokenslim-...` package. GitHub's automatic **Source code**
ZIP and tarball downloads do not include the executable. If no release is
published yet, ready-to-use packages are not available from this page.

Package names follow this pattern (replace `VERSION` with the release version,
without the leading `v`):

```text
tokenslim-VERSION-AGENT-SYSTEM-ARCH.tar.gz  # Linux and macOS
tokenslim-VERSION-AGENT-windows-ARCH.zip   # Windows
```

`AGENT` is `claude` or `codex`; `SYSTEM` is `linux` or `darwin` (macOS);
`ARCH` is `amd64` or `arm64`.

### Verify and extract

Linux example, using the Claude Code AMD64 package:

```sh
sha256sum tokenslim-VERSION-claude-linux-amd64.tar.gz
# Compare with the matching line in SHA256SUMS before extracting.
mkdir -p tokenslim-claude
tar -xzf tokenslim-VERSION-claude-linux-amd64.tar.gz -C tokenslim-claude
```

On macOS, use `shasum -a 256` and the corresponding `darwin` archive.
For Codex, choose the `codex` archive and a separate destination directory.

Windows PowerShell example:

```powershell
Get-FileHash .\tokenslim-VERSION-claude-windows-amd64.zip -Algorithm SHA256
# Compare with the matching line in SHA256SUMS before extracting.
Expand-Archive .\tokenslim-VERSION-claude-windows-amd64.zip .\tokenslim-claude
```

Each package includes its executable, plugin manifest, hooks, recovery skill,
and documentation. Keep these files together after extraction.

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

Packaged hooks select the Windows executable automatically. WSL uses the Linux
package and its Linux paths.

## Claude Code

Launch Claude Code from your project directory, pointing it to the extracted
**Claude Code** package with an absolute path:

```sh
claude --plugin-dir "/absolute/path/to/tokenslim-claude"
```

On Windows, use the corresponding Windows path:

```powershell
claude --plugin-dir "C:\Tools\tokenslim-claude"
```

Keep the package in that location and use the flag when starting a session with
TokenSlim. The plugin compresses eligible Bash output and keeps the original
available for recovery. Source reads and edits, image output, and interrupted
executions remain unchanged.

## Codex

Extract the **Codex** package to a permanent directory. From that directory,
use the included helper to generate a hook configuration (Python 3 is needed
only for this helper):

```sh
python3 scripts/codex-hook-config.py > tokenslim-codex-hooks.json
```

On Windows:

```powershell
python scripts/codex-hook-config.py > tokenslim-codex-hooks.json
```

Add the generated `PostToolUse` block to `~/.codex/hooks.json` or
`<your-project>/.codex/hooks.json`, preserving existing hooks. Open `/hooks` in
Codex to review and trust the hook. The project must also be trusted. The helper
only prints configuration; it does not change global preferences. On Windows,
the generated command targets Git Bash and includes the `.exe` suffix.

This configures output compression through a local hook. The package also
includes the Codex plugin manifest and recovery skill for plugin installation.
Use a Codex version supporting the hooks described in the
[Codex documentation](https://learn.chatgpt.com/docs/hooks). See
[compatibility details](docs/architecture.md) for supported output formats and
host limitations.

## Use the included CLI

From the extracted package directory on Linux or macOS:

```sh
./bin/tokenslim version
./bin/tokenslim benchmark --mode smart --command 'mvn test' build.log
./bin/tokenslim optimize --mode smart --command 'mvn test' build.log
./bin/tokenslim stats
./bin/tokenslim cache inspect ts_HASH
```

On Windows, use `.\bin\tokenslim.exe` in place of `./bin/tokenslim`.
Replace `ts_HASH` with the full reference printed in the output. Benchmark is a
dry run that writes neither cache entries nor metrics. The `optimize` command
and hooks save the original output. Flags must precede the file name; `-` reads
stdin. Token counts are local estimates.

## Update

Download and verify the new release package for the same agent and platform.
Extract it to a new directory and update the Claude Code `--plugin-dir` path or
regenerate the Codex hook configuration from the new directory. Review and
trust changed hooks as required by your agent. Restart the agent before
removing the previous package directory. Configuration and cached originals
remain in your TokenSlim state directory.

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
./bin/tokenslim version
./bin/tokenslim status
./bin/tokenslim config show
./bin/tokenslim config path
./bin/tokenslim stats --session ID
./bin/tokenslim cache inspect ts_HASH --metadata
./bin/tokenslim cache prune
./bin/tokenslim cache clear
```

The cache uses zstd and SHA-256, with 0700 directories and 0600 files on Unix.
On Windows, access is controlled by inherited filesystem ACLs. Originals
may contain secrets already present in logs; they remain local. Clearing or
expiring the cache removes the ability to recover those originals. If the cache
fails or is disabled, the original output is retained. Metrics are stored as
atomic local JSON files. There is no telemetry, external service, or model call.
`TOKENSLIM_DEBUG=1` writes metadata for the latest invocation to
`~/.tokenslim/logs/tokenslim.log`.

## Contributor documentation

- [Building from source, tests, and local packaging](docs/build.md)
- [GitHub Actions and release publishing](docs/ci-cd.md)
- [Dependency maintenance](docs/dependencies.md)

Licensed under Apache-2.0.
