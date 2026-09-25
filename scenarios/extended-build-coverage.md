# Playwright, Cargo and .NET coverage

Stage 3 adds smart success-record reducers for Playwright list output,
Cargo/libtest and .NET/VSTest. Safe mode retains generic normalization.

## Coverage and limits

| Family | Recognized records | Preserved / not specialized |
|---|---|---|
| Playwright | Successful list rows with ordinal, optional project, file:line:column, title and duration | Failure/skip/retry/flaky records, traces and attachments, assertions, final counts, line/dot/custom reporters |
| Cargo/libtest | `test tests::name ... ok` with ASCII test identifiers | Compiler output, ignored tests, panics, should-panic annotations, doctest source locations, Unicode names, JSON, nextest and totals |
| .NET/VSTest | English `Passed TestName [1 ms]` / `[< 1 ms]` / `[1 s]` rows | Failure/skip records, stack traces, run headers, summaries, captured output, MTP/localized/custom output |
| .NET build | Command classification and generic normalization | All nonrepeated build/restore records |

A diagnostic stops suppression for the remainder of its stream. This is a
conservative format recognizer, not a full test-protocol parser. Diagnostic words
inside successful names may intentionally reduce savings. Parallel output is
only reduced when it still forms a complete recognized record; arbitrary worker
prefixes and interleaved fragments are retained. Test names and per-test durations
in summarized records remain recoverable from the original cache entry.

Playwright source locations normally match the integrity guard's file:line rule.
`IntactFor` makes a narrow exception for complete recognized Playwright success
rows. The generic `Intact` behavior is unchanged, and failures, bare locations,
retry titles and diagnostic words remain protected. Regression tests check this
scope and successful-looking records inside failure bodies.

Configuration keys: `playwright`, `rust_test`, `dotnet_test`, `dotnet_build`.
No new dependencies are needed to run TokenSlim or its test suite.

## Evidence

`testdata/playwright.input.txt` comes from a local Playwright **1.55.1** run on
Node **22.23.1**, using the list reporter and one worker. ANSI sequences in the
assertion diff were removed; names, timings and diagnostic text are retained.
The capture has two passing tests and one deliberate assertion failure. It uses
no browser fixture, so no browser installation or browser execution is claimed.
`testdata/playwright-source.txt` contains the source. Reproduce in a disposable
folder with `@playwright/test@1.55.1`, name the source `sample.spec.js`, and run:

```sh
npx playwright test --reporter=list --workers=1
```

Exit code 1 is expected. Absolute paths and timing values vary between machines.
Golden regressions also replay this capture with ANSI wrapping and CRLF.

Cargo and .NET fixtures are synthetic examples based on the official
[Rust test output](https://doc.rust-lang.org/book/ch11-02-running-tests.html)
and [.NET CLI testing](https://learn.microsoft.com/en-us/dotnet/core/tutorials/testing-with-cli)
documentation. Neither runtime was installed in the implementation environment;
these fixtures are not a claim of full runtime-version certification.
See also [Playwright reporters](https://playwright.dev/docs/test-reporters) and
[.NET testing platforms](https://learn.microsoft.com/en-us/dotnet/core/testing/unit-testing-with-dotnet-test).

## Validation and measurement

`extended_builds.py` generates 12 synthetic workloads: success, failure,
compound command and diagnostic-first output for each test family. Every case
runs in safe and smart mode through Claude Code and Codex adapters, including
stdout/stderr, metadata, deterministic content and exact cache recovery checks.
Mode/compressor overrides, thresholds, reduction budgets and disabled cache are
checked for all three compressor keys. Golden tests and fuzz seeds include the
new formats. .NET build has classification coverage and retains generic rules.

Run `make test lint demo`. Runtime dependencies are not required to replay
fixtures. The snapshot below was generated with the working tree based on
TokenSlim 0.2.0 plus stage 3. Byte measurements include the recovery marker;
safe mode and compound/diagnostic-first controls remain unchanged. These are
synthetic savings, not production measurements or billing predictions.

| Scenario | Original bytes | Smart bytes | Reduction |
|---|---:|---:|---:|
| rust-test-success | 9,416 | 992 | 89.5% |
| rust-test-failure | 9,600 | 1,177 | 87.7% |
| dotnet-test-success | 9,525 | 1,104 | 88.4% |
| dotnet-test-failure | 9,701 | 1,280 | 86.8% |
| playwright-success | 15,852 | 1,079 | 93.2% |
| playwright-failure | 16,222 | 1,449 | 91.1% |
