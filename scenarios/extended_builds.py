"""Synthetic stage-3 workloads, not runtime or production benchmarks."""


def cases():
    result = []
    for kind, command in [('rust-test', 'cargo test'), ('dotnet-test', 'dotnet test -v normal'),
                          ('playwright', 'npx playwright test --reporter=list')]:
        lines = []
        if kind == 'dotnet-test':
            lines += ['Test run for /workspace/Sample.Tests.dll (.NETCoreApp,Version=v8.0)',
                      'A total of 1 test files matched the specified pattern.']
        for group in range(20):
            for i in range(10):
                if kind == 'rust-test':
                    line = f'test tests::module_{group}::handles_record_{i} ... ok'
                elif kind == 'dotnet-test':
                    line = f'  Passed Sample.Module{group}.HandlesRecord{i} [1 ms]'
                else:
                    line = f'  ✓  {group*10+i+1} [chromium] › tests/module{group}.spec.ts:{i+1}:1 › handles record {i} (2ms)'
                lines.append(line)
            lines.append('')
        summaries = {
            'rust-test': ['test result: ok. 200 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s'],
            'dotnet-test': ['Test Run Successful.', 'Total tests: 200', '     Passed: 200', ' Total time: 1.000 Seconds'],
            'playwright': ['  200 passed (1.0s)'],
        }
        diagnostics = {
            'rust-test': ['test tests::charge ... FAILED', 'failures:', '---- tests::charge stdout ----',
                          "thread 'tests::charge' panicked at src/lib.rs:12:9:", 'assertion failed: result == 2',
                          'test tests::payload ... ok',
                          'test result: FAILED. 200 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.02s'],
            'dotnet-test': ['  Failed Sample.Tests.Charge [2 ms]', '  Error Message:', 'Expected: 2', 'Actual: 1',
                            '  Stack Trace:', '   at Sample.Tests.Charge() in /workspace/Tests.cs:line 12',
                            '  Passed Sample.Tests.Payload [1 ms]',
                            'Failed! - Failed: 1, Passed: 200, Skipped: 0, Total: 201, Duration: 1 s'],
            'playwright': ['  ✘  201 [chromium] › tests/payment.spec.ts:12:1 › charges card (2ms)',
                           '  1) [chromium] › tests/payment.spec.ts:12:1 › charges card',
                           '    Error: expect(received).toBe(expected)', '    Expected: 2', '    Received: 1',
                           '    attachment #1: trace (application/zip)', '    test-results/payment/trace.zip',
                           '  ✓  202 tests/payload.spec.ts:1:1 › diagnostic payload (2ms)',
                           '  1 failed', '  200 passed (1.1s)'],
        }
        for outcome in ('success', 'failure', 'compound', 'diagnostic-first'):
            prefix = {'rust-test': 'test tests::later ... ignored',
                      'dotnet-test': '  Skipped Sample.Tests.Later [1 ms]',
                      'playwright': '  -  1 tests/later.spec.ts:1:1 › deferred test'}[kind]
            body = ([prefix] if outcome == 'diagnostic-first' else []) + lines
            tail = diagnostics[kind] if outcome == 'failure' else summaries[kind]
            body += tail
            changed = outcome in ('success', 'failure')
            result.append((f'{kind}-{outcome}', command + (' && cat other.log' if outcome == 'compound' else ''),
                           '\n'.join(body)+'\n', tail if changed else body,
                           dict(exit_code=1 if outcome == 'failure' else 0,
                                changed={'safe': False, 'smart': changed}, stderr='Runtime metadata: retained\n')))
    return result
