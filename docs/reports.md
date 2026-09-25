# Local reports

`tokenslim report` reads existing metrics without invoking tools or writing state.
It does not read cached originals or raw commands. It groups records by detected
tool family (compressor), custom rule and outcome.

```sh
tokenslim report
tokenslim report --session SESSION_ID
tokenslim report --format json > report.json
tokenslim report --format html > report.html
```

Open the HTML file in a browser. It is a self-contained snapshot with searchable
rows and sortable columns, without network requests, external assets or a local
web server. Filters affect table rows, not the overall totals. Data remains
visible with JavaScript disabled. Re-run the command to refresh the snapshot.
Session filtering happens when the report is generated; the report contains only
that selected session. Text and JSON formats are suitable for terminal use and
local automation. `tokenslim stats` remains available with its existing format.

## What is measured

- **Outputs / compressed:** all recorded invocations and accepted changes.
- **Original / optimized bytes:** exact recorded sizes, including the recovery
  marker in optimized output.
- **Saved bytes / reduction:** summed original minus optimized bytes. Reduction
  uses total bytes, not an average of per-output percentages. Unchanged outputs
  remain in the denominator.
- **Estimated tokens saved:** difference between summed local token estimates.
  This is not provider tokenization, conversation-level savings or billing data.
- **Rules:** outputs affected, consecutive groups aggregated and lines in those
  groups. Lines include the retained first line. No per-rule byte savings are
  claimed: built-in normalization and multiple rules can contribute to one output.
- **Outcomes:** counts of recorded processing reasons, including thresholds,
  disabled settings, cache failures and accepted compression. Legacy records with
  no reason are grouped under `unspecified`.

Rule counters are stored only when the final transformation is accepted.
Benchmark output can contain prospective counters but creates no metrics files.
Existing metrics without rule fields remain readable. An invocation with metrics
disabled cannot contribute to reports, and nothing reconstructs historical rule
usage from previously stored output. Each invocation counts separately, even if
the content-addressed cache reuses an existing original.

Reports sort rows by name by default and omit generation timestamps, making
repeated rendering of the same metrics snapshot byte-for-byte reproducible.
Concurrent hook writes can change the set of records between report calls; this
is a read of atomic files, not a transactional database snapshot. Malformed
metrics return an error rather than silently distorting totals.

HTML values are escaped. The report includes session/rule identifiers but never
log text, rule regexes or raw commands. Rule naming is therefore visible metadata.

## Reproducible example

```sh
make demo
```

The custom-rule scenario produces `scenarios/results/rules-report.html`, `.json`
and `.txt` from an isolated temporary metrics directory. It verifies report
read-only behavior, correct totals, session selection and deterministic output.
The example includes accepted compression and several rejected cases so the
reported reduction includes unchanged outputs. These are synthetic measurements.

The HTML filter, numeric sorting and narrow-screen layout were also exercised
with a local headless Chrome test; no authenticated host integration is implied.
