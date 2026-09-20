# Compression rules

Safe mode strips ANSI CSI/OSC controls, trims line-end spaces, normalizes CRLF,
retains bare-CR redraws as separate lines, limits blank runs to two and aggregates
adjacent exact repetitions with a count. It does not drop arbitrary progress text.
Complete valid JSON is compacted with `json.Compact`, preserving number lexemes,
duplicate keys and string values. Embedded JSON inside logs is not rewritten.

Smart mode adds conservative deterministic reducers:

| Type | Reduction | Preservation |
|---|---|---|
| Maven | Consecutive recognized repository transfer messages | All diagnostics, reactor summaries, test counts and stack traces |
| Node/Jest | Consecutive PASS suite records | Failure/assertion bodies, summaries and snapshots |
| Kubernetes | Adjacent identical messages differing only in RFC3339 timestamps | First/last timestamps, count, severity, warnings/errors |
| Docker | Same as Kubernetes; identity participates in match | Separate containers remain separate |

A diagnostic line ends specialized Maven/Node suppression for the rest of that
stream, protecting multiline failures. Error and warning timestamp variants are
not grouped. Stack frames are retained; no heuristic framework-frame omission is
implemented. Vitest/Mocha formats without a recognized PASS pattern receive safe
normalization only. Gradle, pytest, Go, Rust, Terraform and Ansible are classified
but use generic rules; Terraform always stays safe.

Integrity checks retain recognized critical lines in order, including file:line,
assertions, exceptions, warnings, security/race warnings and failure summaries.
Exact adjacent duplicates retain a count. Full-JSON compaction has a separate
lossless check. This is conservative pattern recognition, not a proof of semantic
retention for arbitrary text; recovery of the original remains available.

The output is considered changed only after threshold, budget, integrity and
cache checks succeed. All modifications (even whitespace) cache the original.
Marker overhead is included in reported byte/token savings. Cache disabled means
all potentially changed output is preserved. No random sampling or remote calls.
