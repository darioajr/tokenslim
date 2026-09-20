# Dependency maintenance

Versions reviewed on September 20, 2026:

| Component | Version | Decision |
|---|---|---|
| Go toolchain | 1.26.8 | Update the 1.26 series to its current patch release |
| `github.com/klauspost/compress` | 1.20.0 | Already the latest published version |
| `go.yaml.in/yaml/v3` | 3.0.5 | Replace the unmaintained `gopkg.in/yaml.v3` module |
| `actionlint` | 1.7.12 | Update the workflow linter from 1.7.7 |
| `govulncheck` | 1.8.0 | Pin the vulnerability scanner used locally and in CI |

The YAML project moved maintenance to the official YAML organization. TokenSlim
uses its stable v3 API for configuration parsing and serialization. The v4 module
was still at `v4.0.0-rc.6` during this review, so it was not adopted. The module-path
migration removed the old `gopkg.in/check.v1` test dependency as well. Existing
configuration keys and precedence remain unchanged.

The YAML maintainers describe v3 as a security-fix-only legacy branch and recommend
v4 for ongoing development. Revisit v4 once a stable release is available; changing
to a prerelease parser is not required for this maintenance update.

Go 1.27.1 is also available, but this update stays on the supported 1.26 series.
The initial scan with Go 1.26.2 reported standard-library advisories without a
reachable vulnerable call in TokenSlim. Updating the patch release addresses the
outdated standard library without changing the Go release series.

## Repeat the checks

```sh
go list -m -u all
go mod verify
make test lint workflow-lint vulncheck demo release-check
```

`go list -m -u all` detects updates within existing module paths. It cannot identify
that an unmaintained module has moved to a new path; review upstream maintenance
notices as well. `go mod tidy` removes requirements and checksums no longer needed.

`make vulncheck` runs govulncheck 1.8.0 against the current Go toolchain and the
project's reachable code. It requires access to the Go vulnerability database.
The Linux CI test job also runs it. A clean result is a point-in-time check against
known advisories, not a guarantee that dependencies have no unknown defects. Use
`-show verbose` when running the scanner directly to inspect package/module-level
findings that do not appear reachable from the application.

Dependabot tracks the runtime modules through `go.mod` and GitHub Actions through
its workflow configuration. Development tools remain explicitly version-pinned
in the Makefile; review their releases separately when updating dependencies.
Go toolchain updates should be reviewed explicitly too. CI reads the selected
version from `go.mod`; Go installations with automatic toolchain selection can
download the required compiler without replacing the system installation.

References: [YAML maintenance status](https://github.com/yaml/go-yaml),
[compression releases](https://github.com/klauspost/compress/releases),
[actionlint 1.7.12](https://github.com/rhysd/actionlint/releases/tag/v1.7.12), and
[official Go downloads](https://go.dev/dl/).
