# CI and releases with GitHub Actions

The project includes two workflows: [CI](../.github/workflows/ci.yml) and
[Release](../.github/workflows/release.yml). They use the Go version declared in
`go.mod`, Python 3.12, and GitHub-hosted runners. No agent accounts, API keys,
Docker, or Kubernetes are required: scenarios use synthetic logs and actual binaries.

## Validation pipeline

CI runs on branch pushes, pull requests, and manual dispatch (`workflow_dispatch`).
The release workflow can also call it through `workflow_call`.

| Job | Checks | Output |
|---|---|---|
| Test (ubuntu-latest) | Dependencies, versions/manifests, go vet, gofmt, race detection, coverage, vulnerability scanning, and scenarios for both agents | `test-results-ubuntu-latest` |
| Test (macos-latest) | The same checks on macOS, except the Linux-only vulnerability scan | `test-results-macos-latest` |
| Test (windows-latest) | Dependencies, manifests, go vet, race detection, coverage, scenarios, and Bash hook execution with spaced paths | `test-results-windows-latest` |
| Windows package smoke test | SHA-256 verification and execution of both packaged Windows AMD64 binaries | None |
| Fuzz | 100,000 fuzz executions per target: compressor and hook | Corpus attached on failure |
| Workflow lint | Workflow syntax and expressions using actionlint 1.7.12 | Fails if a workflow is invalid |
| Packages | After the other jobs: cross-compilation, contents of all twelve archives, checksums, and execution of both Linux amd64 binaries | `release-packages` |

Test artifacts and reports are retained for 14 days. Each test job summary shows
the byte-savings table. Artifacts include the JSON report, scenario outputs, hook
responses, and `coverage.out`. Coverage is informational, without an arbitrary
minimum threshold. Fuzzing uses an execution-count budget instead of a short
wall-clock deadline; each target still has a three-minute test timeout to catch
hangs on shared runners. Latency benchmarks run locally with `make benchmark`; there
is no performance threshold on shared runners.

Packages cover Claude Code and Codex × Linux/macOS/Windows × amd64/arm64. Cross-compilation
does not execute binaries for every architecture. Package smoke tests run Linux AMD64 and Windows AMD64 binaries; the Go test suite
also runs on macOS. Windows ARM64 is cross-compiled but not executed in CI.
Linux/macOS archives use `.tar.gz`; Windows archives use `.zip` and contain
`bin/tokenslim.exe`. All targets are 64-bit. Windows hook tests use Git Bash.

## Enable the workflows in your repository

1. Commit and push the project, including `.github/`, to your GitHub repository.
2. Under **Settings → Actions → General**, allow GitHub Actions and the official
   `actions/*` actions. If your organization restricts write permissions, allow
   `contents: write` for the release publishing job. Other jobs use read permissions.
3. Under **Actions → CI → Run workflow**, run the initial validation. Manual
   dispatch becomes available once the workflow is on the default branch.
4. Optionally configure a default-branch ruleset requiring `Test (ubuntu-latest)`,
   `Test (macos-latest)`, `Test (windows-latest)`, `Windows package smoke test`, `Fuzz`, `Workflow lint`, and `Packages`. Select the check
   names shown by GitHub after the first run.

No PAT or manually configured secret is needed: publishing uses the automatic
`GITHUB_TOKEN`. Workflows do not install or trust agent hooks. The pipeline
validates the local contract, not an authenticated Claude/Codex conversation.

## Create a release

[VERSION](../VERSION) is the version source. Before releasing, also update the
`version` fields in both manifests:

- `.claude-plugin/plugin.json`
- `integrations/codex/tokenslim/.codex-plugin/plugin.json`

The build injects this version into the binary through the linker. Only stable
`MAJOR.MINOR.PATCH` versions are accepted; prereleases are not supported by this
pipeline yet. The tag must be exactly `v` followed by the contents of `VERSION`.

```sh
python3 scripts/validate-release.py
make test lint demo workflow-lint release-check
# After committing and pushing the changes to the remote:
version=$(cat VERSION)
git tag -a "v$version" -m "TokenSlim $version"
git push origin "v$version"
```

Pushing the tag starts the Release workflow, which:

1. Checks the tag, VERSION, manifests, and adapter selection.
2. Runs the complete CI pipeline on the tagged commit.
3. Downloads the packages produced by that run and verifies SHA-256 again.
4. Creates a **draft GitHub Release** with generated notes, all twelve packages,
   and `SHA256SUMS`. Review it under **Releases** and publish when ready.

Only the final job has `contents: write`. The `gh release create` command requires
the tag to exist and does not overwrite an existing release. If a run fails after
creating the draft, check its attachments. Remove only the incomplete draft
before rerunning, or finish the existing draft. Do not move published tags.

## Run locally

```sh
make test          # tests with the race detector
make lint          # go vet and formatting checks
make vulncheck     # reachable Go vulnerabilities (govulncheck 1.8.0)
make demo          # Claude and Codex scenarios
make workflow-lint # pinned actionlint; first use downloads the Go tool
make release-check # twelve packages plus content and checksum validation
```

`make release` writes to `dist/`; it does not publish or upload anything. The
script checks the version before compiling. `SHA256SUMS` includes only packages
for the current version; older local archives are excluded. Each archive contains
the binary, manifest, hook, recovery skill, license, version, documentation, and
Codex configuration script.

To verify a download:

```sh
# In the directory containing all twelve archives and SHA256SUMS:
sha256sum --check SHA256SUMS # Linux
shasum -a 256 -c SHA256SUMS # macOS
```

On Windows, run this in PowerShell and compare the result with the matching
entry in `SHA256SUMS`. Replace VERSION and AGENT with the downloaded values.
Use `Expand-Archive` to extract the ZIP.

```powershell
Get-FileHash .\tokenslim-VERSION-AGENT-windows-amd64.zip -Algorithm SHA256
```

## Maintenance and troubleshooting

- **Tag mismatch:** update VERSION and both manifests in the correct commit.
- **Formatting:** run `gofmt -w cmd internal` and review the changes.
- **Scenario failure:** inspect the `test-results-*` artifacts. Reproduce original
  outputs by running `make demo` locally.
- **Fuzzing failure:** download the attached corpus, copy the failing case into
  the corresponding package directory, and run `go test` to reproduce it.
- **Release permission failure:** check your organization's Actions policy and
  the `publish` job's `GITHUB_TOKEN` permissions.
- **Actions or Go unavailable:** inspect the setup log and the availability of
  the version declared in `go.mod` before changing the compiler.

Actions are pinned by SHA. [Dependabot](../.github/dependabot.yml) proposes weekly
updates to actions and Go dependencies. GitHub Actions version updates are grouped
into one PR so related upload/download upgrades can be reviewed together. Grouping controls future update PRs; it does not authorize automatic merges. The actionlint version is pinned in the
Makefile and must be updated explicitly. ShellCheck integration is disabled so
the same actionlint command runs locally and in CI without an extra dependency.

Official references: [reusable workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows)
and [GITHUB_TOKEN permissions](https://docs.github.com/en/actions/tutorials/authenticate-with-github_token).


## Action versions and runner compatibility

The workflows use these commit-pinned versions:

| Action | Version |
|---|---|
| `actions/checkout` | 7.0.1 |
| `actions/setup-go` | 7.0.0 |
| `actions/setup-python` | 7.0.0 |
| `actions/upload-artifact` | 7.0.1 |
| `actions/download-artifact` | 8.0.1 |

These actions use Node.js 24 internally. This does not change the Go or Python
versions selected for the project. The pipeline uses GitHub-hosted runners; if
you switch to self-hosted runners, verify each action's runner requirements first.
See the [setup-go compatibility notes](https://github.com/actions/setup-go/tree/v7.0.0)
and [checkout compatibility notes](https://github.com/actions/checkout/tree/v7.0.1).

Artifact uploads retain the default ZIP archive behavior, which supports the
multi-file reports and release bundle. The release download extracts that bundle
and fails on digest mismatches by default; the separate SHA-256 check of the
release files also remains enabled. See the
[upload inputs](https://github.com/actions/upload-artifact/tree/v7.0.1) and
[download inputs](https://github.com/actions/download-artifact/tree/v8.0.1).

Dependabot version-update PRs and security alerts are separate features. The
configuration file schedules version updates; it does not enable security alerts
in the repository settings.
