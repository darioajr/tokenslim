# Building and development

This guide is for contributors working from a source checkout. To use TokenSlim,
download a ready-to-use package from [GitHub Releases](https://github.com/darioajr/tokenslim/releases)
and follow the [installation guide](../README.md#install-from-a-github-release).
The release packages already contain the executable.

## Prerequisites

- Git to clone the repository.
- Go 1.26.8 or newer, matching `go.mod` for CI parity.
- Python 3.12 or newer for build helpers, scenarios, and package validation.
- Make and a POSIX shell for the Makefile commands on Linux/macOS.
- Git for Windows for Bash hook checks on Windows.
- A C compiler for Go's race detector (`go test -race`).

```sh
git clone https://github.com/darioajr/tokenslim.git
cd tokenslim
```

Run the following commands from the repository root. The documentation included
in release archives describes this workflow, but those archives do not include
the Go source or development scripts; use a source checkout for this guide.

## Build locally

Linux and macOS:

```sh
make build
./bin/tokenslim version
```

The build writes the native executable to `bin/tokenslim` and copies it into
`integrations/codex/tokenslim/bin/`. The root directory is the Claude Code plugin;
`integrations/codex/tokenslim/` is the Codex plugin.

Windows PowerShell (no Make required):

```powershell
$env:PYTHONUTF8 = "1"
python scripts/build.py
.\bin\tokenslim.exe version
```

Windows builds produce `bin/tokenslim.exe` and a copy in the Codex plugin's
`bin` directory. `scripts/build.py` also works on Linux/macOS. `VERSION` supplies
the binary version unless overridden through the `VERSION` environment variable.

## Run tests and the efficiency demo

Linux and macOS:

```sh
make test          # Go tests with race detection
make lint          # go vet and formatting checks
make demo          # build and run both adapters against synthetic logs
python3 scripts/check-hook-command.py
make benchmark     # local reducer and engine timing benchmarks
make vulncheck     # scan reachable Go vulnerabilities
make workflow-lint # validate GitHub Actions workflows
```

Windows PowerShell:

```powershell
$env:PYTHONUTF8 = "1"
go mod verify
go vet ./...
go test -race ./...
python scripts/build.py
python scenarios/run.py --check
python scripts/check-hook-command.py
```

The hook command check exercises plugin paths containing spaces. On Linux/macOS,
it uses Bash from PATH. On Windows, it locates Git Bash from the Git installation
and verifies the shell environment; it does not use the WSL `bash.exe` launcher.
For a custom installation, set `TOKENSLIM_TEST_BASH` to the absolute path of
Git Bash's `bash.exe`. CI explicitly passes the path of its current Git Bash
shell. Failures report the selected shell, exit code, stdout, and stderr. The synthetic scenarios generate logs in `scenarios/generated/`
and Markdown/JSON results in `scenarios/results/`, testing both safe and smart
modes and both agent adapters. See the
[scenario guide](https://github.com/darioajr/tokenslim/blob/main/scenarios/README.md)
for methodology and the comparison table. Reproduce those results before updating
the documented measurements.

These checks exercise binaries and local hook protocols. They do not establish
billing savings or replace authenticated agent-session testing.

## Try a development plugin

After building, launch Claude Code from the repository root:

```sh
claude --plugin-dir .
```

To generate a Codex hook configuration pointing to the development binary:

```sh
python3 scripts/codex-hook-config.py > tokenslim-codex-hooks.json
```

On Windows, use `python` instead of `python3`. Follow the
[Codex configuration instructions](../README.md#codex) to merge and trust it.

On Linux/macOS, `make install` installs only the CLI into `~/.local/bin` by
default; it does not register an agent plugin. Override `PREFIX` if needed.

## Build redistributable packages

From Linux/macOS with Go, Python, Make, and a POSIX shell:

```sh
make release-check
```

This cross-compiles and validates twelve packages under `dist/`:

| Target OS | Architectures (64-bit only) | Packages per agent |
|---|---|---|
| Linux | AMD64, ARM64 | Two `.tar.gz` archives |
| macOS (`darwin`) | AMD64, ARM64 | Two `.tar.gz` archives |
| Windows | AMD64, ARM64 | Two `.zip` archives |

Each of Claude Code and Codex receives six packages. Windows archives contain
`tokenslim.exe`. Each archive includes the manifest, hooks, recovery skill,
documentation, license, version, and Codex hook configuration helper.
`SHA256SUMS` covers the current version's archives. Validation checks the full
package matrix, required contents, checksums, versions, Unix executable bits,
and Windows PE architecture. No 32-bit targets are generated.

`make release` builds packages without the final archive validation step.
Neither command uploads or publishes anything. Windows contributors can use the
GitHub Actions workflow for the complete distribution build.

CI executes tests on Linux, macOS, and Windows AMD64 and smoke-tests packaged
Linux AMD64 and Windows AMD64 executables. Cross-compilation does not verify
native execution on every architecture; Windows ARM64 is not executed in CI.

## Publish through GitHub Actions

See [CI and releases](ci-cd.md) for the full workflow. Push the source changes
before sending a new `vMAJOR.MINOR.PATCH` tag. No manual version edits are needed:
the workflow stamps `VERSION` and both manifests from the tag in every build
checkout. The tag triggers validation, builds the packages, and creates a draft GitHub
Release containing all twelve archives and `SHA256SUMS`. Review and publish that
draft so users can download its assets.

The trigger is a tag push, not the publication of a release through the GitHub
interface. Creating the release manually first can conflict with the workflow's
release-creation step. CI artifacts are retained temporarily; published release
assets are the user-facing distribution channel.

Release stamping changes the build checkouts and attached packages; it does not
create a version-bump commit on `main`. Local builds use the checked-in version.
To reproduce a tagged release locally, synchronize metadata first (this edits
`VERSION` and both manifests in your working tree):

```sh
python3 scripts/set-release-version.py --tag v0.2.0
make release-check
```

The version-stamping regression tests run without Go or external Python packages:

```sh
python3 -m unittest discover -s scripts -p 'test_*.py'
```
