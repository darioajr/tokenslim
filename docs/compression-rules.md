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
| pytest | Verbose PASSED records with node IDs | Failures, captured output, skipped/xfail/xpass records and totals |
| Go tests | Adjacent RUN/PASS pairs for the same test | Package results, test logs, failures, races and parallel interleaving |
| Vitest | Recognized successful file records with test counts and duration | Failure trees, console output, skipped tests, totals and coverage |
| Gradle | UP-TO-DATE, FROM-CACHE and NO-SOURCE task records | Executed tasks, skipped/failed tasks, compiler output and build summaries |
| Node/Jest | Consecutive PASS suite records | Failure/assertion bodies, summaries and snapshots |
| Composer | Recognized package downloads and archive extractions | Dependency summaries, scripts, conflicts and security warnings |
| PHPUnit/Pest/Laravel tests | Indented PASS and ✓/✔ success records | Failure bodies, stack traces, test counts, durations and coverage |
| Kubernetes | Adjacent identical messages differing only in RFC3339 timestamps | First/last timestamps, count, severity, warnings/errors |
| Docker | Same as Kubernetes; identity participates in match | Separate containers remain separate |

A diagnostic line ends specialized success-record suppression for the rest of that
stream, protecting multiline failures. Error and warning timestamp variants are
not grouped. Stack frames are retained; no heuristic framework-frame omission is
implemented. Unknown Vitest reporters and pytest progress formats use generic
normalization. Mocha output without a recognized PASS pattern also uses generic
rules. Rust, Terraform and Ansible are classified but use generic rules;
Terraform always stays safe.

Integrity checks retain recognized critical lines in order, including file:line,
assertions, exceptions, warnings, security/race warnings and failure summaries.
Exact adjacent duplicates retain a count. Full-JSON compaction has a separate
lossless check. This is conservative pattern recognition, not a proof of semantic
retention for arbitrary text; recovery of the original remains available.

The output is considered changed only after threshold, budget, integrity and
cache checks succeed. All modifications (even whitespace) cache the original.
Marker overhead is included in reported byte/token savings. Cache disabled means
all potentially changed output is preserved. No random sampling or remote calls.

## PHP and Laravel

Commands are classified as `php`, `composer`, `php-test` or `laravel` (configuration
keys: `php`, `composer`, `php_test`, `laravel`). Recognized entry points include
versioned PHP interpreters, `composer` / `composer.phar`, `vendor/bin/phpunit`,
`phpunit.phar`, `vendor/bin/pest`, `php artisan` and `vendor/bin/sail`. Specific
entry points take precedence over interpreter wrappers. Composer script names are
not resolved; `composer test` and `composer run-script test` use generic rules.

Safe mode uses the same normalization and exact-repeat rules as other commands.
Smart mode summarizes recognized downloads/extractions only for simple
`composer install` / `composer update` invocations, and successful test records
only for direct PHPUnit/Pest, `artisan test`, `sail test`, `sail artisan test`,
`sail pest` or `sail phpunit` invocations. Interpreter wrappers with supported
PHP switches are accepted. Quoted or compound shell commands and unknown
wrappers use generic normalization; detecting a family alone does not authorize
specialized suppression. Composer suppression stops at script output (`>`). PHPUnit dot progress and unrecognized formats remain unchanged
apart from generic normalization. PHP scripts, lint results, Artisan migrations,
queues, custom Artisan commands and Laravel logs always use generic rules;
Laravel timestamps are not grouped. Deprecations, notices, risky/skipped/incomplete
tests and failure markers stop specialized suppression for the rest of the stream.
Migration records, SQL errors, stack frames and test summaries are retained.

Examples:

```sh
tokenslim benchmark --mode smart --command 'composer install' composer.log
tokenslim optimize --mode smart --command 'vendor/bin/pest' tests.log
tokenslim optimize --mode smart --command 'php artisan test' tests.log
```

See the [Laravel test entry points](https://laravel.com/framework/docs/10.x/testing)
and [Sail test commands](https://github.com/laravel/docs/blob/13.x/sail.md#running-tests).

See [PHP/Laravel coverage and measurements](../scenarios/php-coverage.md) for
fixture provenance, benchmark results and the limits of parallel-output support.

## pytest, Go, Vitest and Gradle

Specialized reducers require a simple recognized command: `pytest`,
`python -m pytest` (including versioned interpreters), `go test`, `gradle` /
`gradlew`, direct `vitest`, `npx vitest`, `pnpm [exec] vitest`, `yarn [exec] vitest`
or `bun [x] vitest`. Shell expressions, quoted commands, unknown wrappers and
npm script bodies are not resolved. Such commands receive generic normalization
for these reducers. The existing Jest PASS rule is unchanged.

Configuration keys are `pytest`, `go_test`, `gradle` and **`node_test` for Vitest**.
Vitest has its own classifier/metrics name (`vitest`), while sharing the existing
Node configuration so disabling `node_test` still disables its compressor.

Go only aggregates RUN/PASS pairs with matching names and no intervening output.
It keeps `ok`/`FAIL` package lines and does not summarize benchmark or JSON event
records. Parallel PAUSE/CONT stops specialized suppression. Gradle only summarizes
explicit cached/up-to-date/no-source statuses, never bare task execution lines.
pytest xfail/xpass and failure-section headings, Vitest console sections and
Kotlin compiler `e:`/`w:` lines also stop suppression for the remainder of a stream.

See [build/test coverage](../scenarios/build-coverage.md) for concrete formats,
regression evidence, measurements and unsupported variants.
