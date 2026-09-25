"""Synthetic PHP workloads; format provenance is documented in php-coverage.md.

These are generated examples, not locally captured PHP executions or production
benchmarks. Upstream expected-output excerpts are separate golden fixtures.
"""


def cases():
    result = []
    # Keep the package operation ledger: versions and removals matter even when
    # archive transfer chatter is summarized.
    for operation in ('install', 'update'):
        lines = ['Installing dependencies from lock file' if operation == 'install'
                 else 'Updating dependencies',
                 'Package operations: 80 installs, 0 updates, 0 removals']
        for i in range(80):
            if operation == 'update':
                lines.append(f'  - Locking example/package-{i} (1.0.{i})')
            lines.extend([f'  - Downloading example/package-{i} (1.0.{i})',
                          f'  - Installing example/package-{i} (1.0.{i}): Extracting archive'])
        tail = ['Generating optimized autoload files', '> @php artisan package:discover --ansi',
                '  INFO  Discovering packages.',
                '  example/application ........................................ DONE',
                '80 packages you are using are looking for funding.',
                'Use the `composer fund` command to find out more!']
        # Enough retained real-format metadata to remain within the default budget.
        tail += [f'  example/provider-{i} ........................................ DONE' for i in range(10)]
        result.append((f'composer-{operation}', f'composer {operation}',
                       '\n'.join(lines + tail) + '\n',
                       [line for line in lines if not line.startswith(('  - Downloading', '  - Installing'))] + tail,
                       dict(exit_code=0, changed={'safe': False, 'smart': True},
                            stream='stderr' if operation == 'update' else 'stdout',
                            stderr='Composer runtime metadata: PHP 8.4\n')))
    # Chunk boundaries retain suite names (TestDox) or blank lines (Pest).
    for name, command, style in [
        ('phpunit-testdox', 'vendor/bin/phpunit --testdox', 'testdox'),
        ('pest-success', 'vendor/bin/pest', 'pest'),
        ('artisan-test', 'php artisan test', 'pest'),
        ('sail-test', './vendor/bin/sail test', 'pest'),
        ('pest-parallel', 'vendor/bin/pest --parallel', 'parallel'),
        ('artisan-parallel', 'php artisan test --parallel', 'parallel'),
    ]:
        lines = ['PHPUnit 12.5.0 by Sebastian Bergmann and contributors.',
                 'Runtime: PHP 8.4.0', 'Configuration: /workspace/phpunit.xml'] if style == 'testdox' else []
        for group in range(20):
            if style == 'testdox':
                lines.append(f'Example {group} (Tests\\Unit\\Example{group})')
            elif style == 'parallel':
                # Interleaved worker-prefixed records are intentionally unknown.
                lines.append(f'[worker {group % 4}] PASS Tests\\Unit\\Example{group}')
            else:
                lines.append(f'   PASS  Tests\\Unit\\Example{group}')
            for i in range(10):
                prefix = f'[worker {group % 4}]' if style == 'parallel' else ' '
                lines.append(f'{prefix} {"✔" if style == "testdox" else "✓"} processes record {group}-{i} with valid input 0.01s')
            lines.append('')
        tail = ['Time: 00:02.000, Memory: 24.00 MB', 'OK (200 tests, 200 assertions)'] if style == 'testdox' else [
            '  Tests: 200 passed (200 assertions)', '  Duration: 2.00s']
        if style == 'parallel':
            tail.append('  Parallel: 4 processes')
        result.append((name, command, '\n'.join(lines + tail) + '\n', tail,
                       dict(exit_code=0, changed={'safe': False, 'smart': style != 'parallel'})))
    # ANSI + CRLF with a full diagnostic body that includes apparent successes.
    lines = []
    for group in range(20):
        lines.append(f'\x1b[32m   PASS  Tests\\Feature\\Example{group}\x1b[0m')
        lines.extend(f'  ✓ handles request {group}-{i} with valid input 0.01s' for i in range(10))
        lines.append('')
    diagnostic = ['   FAIL  Tests\\Feature\\PaymentTest', '  ⨯ charges a card',
                  '  Expected response status code [200] but received 500.',
                  '  at tests/Feature/PaymentTest.php:42',
                  '  #0 /workspace/vendor/framework.php(31): handle()',
                  '  ✓ this is assertion payload, keep it verbatim',
                  '  Tests: 1 failed, 200 passed (201 assertions)', '  Duration: 2.01s',
                  '  Coverage: 82.50%']
    result.append(('pest-failure-ansi', 'vendor/bin/pest --colors=always',
                   '\r\n'.join(lines + diagnostic) + '\r\n', diagnostic,
                   dict(exit_code=1, changed={'safe': False, 'smart': True},
                        stderr='PHP Deprecated: legacy API in /workspace/app/Service.php on line 21\n')))
    # Default progress, issue markers and non-test commands must be retained.
    for name, command, header in [
        ('phpunit-default', 'vendor/bin/phpunit', 'PHPUnit 12.5.0 by Sebastian Bergmann and contributors.'),
        ('php-issues', 'vendor/bin/phpunit --testdox', ' ⚠ invokes legacy API'),
        ('artisan-migrate', 'php artisan migrate', '  INFO  Running migrations.'),
        ('artisan-queue', 'php artisan queue:work --once', '  INFO  Processing jobs.'),
        ('artisan-custom', 'php artisan custom:report', '  INFO  Building report.'),
        ('php-compound', 'php artisan test && php artisan custom:report', '  INFO  Combined output.'),
        ('composer-script', 'composer run-script test', '> @php artisan custom:report'),
    ]:
        lines = [header]
        if name == 'phpunit-default':
            lines += [f'{"." * 60} {60 * (i+1):4} / 4800 ({(i+1)*100//80:3}%)' for i in range(80)]
            tail = ['Time: 00:04.000, Memory: 32.00 MB', 'OK (4800 tests, 4800 assertions)']
        else:
            lines += [f'  ✓ preserved domain record {i}: customer account state is active' for i in range(90)]
            tail = ['  Summary: 90 records processed']
            if name == 'php-issues':
                tail += [' ∅ unfinished case', ' ↩ deferred case', ' ✘ rejected payment',
                         '   │ custom diagnostic without an error keyword',
                         '  ✓ diagnostic payload must survive',
                         'Tests: 90, Assertions: 89, Skipped: 1, Incomplete: 1, Risky: 1.',
                         '1 test triggered 1 deprecation:']
        original = '\n'.join(lines + tail) + '\n'
        result.append((name, command, original, lines + tail,
                       dict(exit_code=1 if name == 'php-issues' else 0,
                            changed={'safe': False, 'smart': False})))
    return result
