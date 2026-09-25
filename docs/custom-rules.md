# Custom aggregation rules

Custom rules are opt-in YAML configuration. They run only in smart mode, after
built-in specialized reductions and before generic adjacent-repeat aggregation.
They never execute commands or modify source files. Safe/off mode and a disabled
compressor bypass them; Terraform stays safe. Complete valid JSON uses the
existing lossless compactor and bypasses custom rules.

Example `.tokenslim.yaml` for a worker whose health messages differ only in a
counter and duration:

```yaml
version: 1
mode: smart
rules:
  - name: health
    match:
      command: 'worker logs*'
      regex: '(?P<tick>[0-9]+) INFO api=\w+ GET /health duration=(?P<duration>[0-9]+)ms'
    ignore_groups: [tick, duration]
    action:
      aggregate: true
```

Given a consecutive run like:

```text
1 INFO api=payments GET /health duration=3ms
2 INFO api=payments GET /health duration=5ms
3 INFO api=payments GET /health duration=4ms
```

TokenSlim retains the first normalized line and emits a count:

```text
1 INFO api=payments GET /health duration=3ms
[TokenSlim rule=health: 3 lines aggregated]
```

The change is applied only when that group becomes shorter. Final whole-output
thresholds, reduction budget, integrity and cache checks must still succeed.
Otherwise the original output is returned and no rule usage is counted.

## Matching contract

- `name` is unique: a leading ASCII letter followed by up to 63 letters, digits,
  underscores or hyphens. Names appear in output markers and metrics.
- `enabled` defaults to true. `enabled: false` disables that rule; its definition
  must still be valid so configuration problems do not silently persist.
- `match.command` matches the entire command. Only `*` (any sequence, including
  slashes/spaces) and `?` (one character) are wildcards; all other characters are
  literal. This is not shell parsing or a regular expression.
- `match.regex` uses [Go regular expressions](https://pkg.go.dev/regexp) and must
  match the whole normalized line. There is no partial-line suppression.
- Without `ignore_groups`, only identical matched lines group. With it, only
  the selected named captures may differ; every other byte participates in the
  comparison. Different services, containers, messages or status values remain
  separate unless their fields are explicitly ignored.
- Ignored captures must exist once in the regex. Duplicate names and missing
  captures are configuration errors. Nested/overlapping ignored captures do not
  aggregate when matched. Repeated capture expressions only expose their last
  captured occurrence, following Go regexp semantics.
- Only adjacent matching lines group. Blank lines, nonmatches and TokenSlim
  markers break a group. The first matching rule in configuration order owns the
  line, including when it finds no savings; rules do not cascade.
- Recognized diagnostics and their following body are protected for the remainder
  of each stream. This uses the conservative diagnostic recognition of the
  built-in reducers, including warnings, source locations, assertions and skips.
  It cannot identify every possible custom diagnostic format.

The first line is kept after normal ANSI/whitespace normalization. Values removed
by ignoring fields are recoverable from the cache; the summary does not retain
all timestamps, durations or test identifiers. Configure only fields you intend
to aggregate. The first line and count are not a replacement for the original
when inspecting an ambiguous event.

Limits: 32 rules, 512 bytes per command pattern, 2048 bytes per regex and 8 ignored
capture names per rule. Only `action.aggregate: true` is supported. Arbitrary
replacement, dropping lines, scripting and remote calls are not supported.

## Configuration and preview

Project `rules` replaces the complete global list. Use `rules: []` in the project
to disable inherited rules. Existing CLI/project/global precedence is unchanged.

```sh
tokenslim config validate
tokenslim config show
tokenslim benchmark --mode smart --command 'worker logs' --json worker.log
```

The benchmark runs without cache or metrics writes. Its `rules` field shows
accepted prospective group/line counts. Invalid configuration makes the CLI
return an error and hooks retain the native result. Cache-disabled operations,
rejected budgets and unsuccessful cache writes never record rule effects.

Generic `group_repeated_lines: false` controls the built-in exact-repeat pass;
explicit custom rules are independently enabled by their `aggregate` action.
Per-compressor mode and enabled settings still apply to the entire pipeline.

## Validation

Unit tests cover identity, partial matches, duplicate/missing captures, overlapping
captures, precedence, safe mode, JSON, diagnostic bodies and deterministic output.
`make demo` checks accepted/rejected changes, exact cache recovery, both adapters,
invalid-rule pass-through, metric attribution, session filtering and read-only
reports. Rule statistics count only accepted transformations. Fuzz tests check
repeatability and diagnostic retention.
