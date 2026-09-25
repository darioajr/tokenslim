"""Synthetic build/test formats; see build-coverage.md for scope and evidence."""


def cases():
    result = []
    commands = {'pytest': 'python3 -m pytest -v', 'go-test': 'go test -v ./...',
                'vitest': 'npx vitest run', 'gradle': './gradlew build --console=plain'}
    for kind, command in commands.items():
        lines = []
        for group in range(20):
            for case in range(10):
                if kind == 'pytest':
                    lines.append(f'tests/test_module_{group}.py::test_record_{case} PASSED [ 50%]')
                elif kind == 'go-test':
                    lines += [f'=== RUN   TestRecord{group}_{case}',
                              f'--- PASS: TestRecord{group}_{case} (0.00s)']
                elif kind == 'vitest':
                    lines.append(f' ✓ tests/module_{group}/record_{case}.test.ts (2 tests) 3ms')
                else:
                    status = ('UP-TO-DATE', 'FROM-CACHE', 'NO-SOURCE')[case % 3]
                    lines.append(f'> Task :module{group}:compilePart{case} {status}')
            if kind == 'go-test':
                lines += ['PASS', f'ok  example.test/module{group} 0.004s']
            else:
                lines.append('')
        summary = {
            'pytest': ['==================== 200 passed in 1.00s ===================='],
            'go-test': [],
            'vitest': [' Test Files  200 passed (200)', '      Tests  400 passed (400)', '   Duration  1.00s'],
            'gradle': ['BUILD SUCCESSFUL in 1s', '200 actionable tasks: 140 up-to-date, 60 from cache'],
        }[kind]
        diagnostic = {
            'pytest': ['tests/test_payment.py::test_charge FAILED [100%]',
                       '________________ test_charge ________________',
                       '> assert actual == 2', 'E AssertionError: assert 1 == 2',
                       'tests/test_payment.py:12: AssertionError', '1 failed, 200 passed in 1.01s'],
            'go-test': ['=== RUN   TestCharge', '    payment_test.go:12: expected 2, got 1',
                        '--- FAIL: TestCharge (0.00s)', 'FAIL', 'FAIL example.test/payment 0.004s'],
            'vitest': [' ❯ tests/payment.test.ts (1 test | 1 failed) 2ms',
                       ' FAIL  tests/payment.test.ts > charges card', 'AssertionError: expected 1 to be 2',
                       ' ❯ tests/payment.test.ts:12:3', ' Test Files  1 failed | 200 passed (201)',
                       '      Tests  1 failed | 400 passed (401)', '   Duration  1.01s'],
            'gradle': ['> Task :payment:test FAILED', 'PaymentTest > chargesCard FAILED',
                       '    AssertionError: expected 2, got 1', '        at app.PaymentTest(PaymentTest.java:12)',
                       'FAILURE: Build failed with an exception.', 'BUILD FAILED in 1s'],
        }[kind]
        for outcome in ('success', 'failure', 'compound', 'diagnostic-first'):
            tail = diagnostic if outcome == 'failure' else summary
            prefix = {'pytest': 'tests/test_case.py::test_case XFAIL [  1%]',
                      'go-test': '=== PAUSE TestRecord0_0',
                      'vitest': 'stdout | tests/module_0/record_0.test.ts > prints data',
                      'gradle': 'w: compiler message'}[kind] if outcome == 'diagnostic-first' else ''
            body = ([prefix] if prefix else []) + lines + tail
            original = '\n'.join(body) + '\n'
            changed = outcome in ('success', 'failure')
            protected = tail if changed else body
            result.append((f'{kind}-{outcome}', command + (' && cat other.log' if outcome == 'compound' else ''),
                           original, protected, dict(exit_code=1 if outcome == 'failure' else 0,
                           changed={'safe': False, 'smart': changed}, stderr='Runtime metadata: retained\n')))
    return result
