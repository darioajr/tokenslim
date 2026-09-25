#!/usr/bin/env python3
"""Deterministic synthetic corpus; no network, model calls or external services."""
import argparse
import datetime as dt
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

from php import cases as php_cases
from builds import cases as build_cases

ROOT = Path(__file__).resolve().parents[1]
BINARY = ROOT / 'bin' / ('tokenslim.exe' if os.name == 'nt' else 'tokenslim')


def cases():
    result = []
    for family, command in [('maven', 'mvn test'), ('node', 'npm test'),
                            ('kubernetes', 'kubectl logs payment-api'),
                            ('docker', 'docker compose logs'), ('generic', 'build')]:
        lines = []
        for group in range(60):
            for repeat in range(12):
                stamp = (dt.datetime(2026, 9, 19, 12) + dt.timedelta(seconds=group*12+repeat)).isoformat()+'Z'
                if family == 'maven':
                    line = f'Downloaded from central: https://repo.example.test/artifact-{group}.jar (128 kB at 1 MB/s)'
                elif family == 'node':
                    line = f'PASS src/module-{group}/suite-{repeat}.test.ts'
                elif family == 'kubernetes':
                    line = f'{stamp} INFO payment-api heartbeat partition={group}'
                elif family == 'docker':
                    line = f'api-{group % 3} | {stamp} INFO heartbeat partition={group}'
                else:
                    line = f'Worker heartbeat partition={group}: checking queue for new tasks'
                lines.append(line)
            # Distinct, meaningful records prevent unrealistic >95% reduction.
            lines.append(f'Checkpoint {group}: processed batch with 120 records; shard={group}; elapsed=40ms')
        critical = ['ERROR PaymentServiceTest.refundExpiredPayment',
                    'Expected: REJECTED; Actual: APPROVED',
                    'PaymentService.java:221',
                    'WARN authentication token expires soon',
                    'Tests run: 312, Failures: 2, Errors: 0',
                    'BUILD FAILURE; exit status 1']
        result.append((family, command, '\n'.join(lines+critical)+'\n', critical))
    result.append(('small', 'echo hello', 'hello\n', ['hello']))
    result.append(('unicode-ansi', 'build',
                   ''.join(f'\x1b[32mProcessando conexão {g}: ação concluída ✓\x1b[0m  \n'*8 + f'Lote {g} preservado\n' for g in range(50))+'ERROR conexão indisponível app.go:41\n',
                   ['ERROR conexão indisponível app.go:41']))
    result.append(('unique', 'build', ''.join(f'Unique event {i}: no repeated content, retain every observation\n' for i in range(100)), ['Unique event 99']))
    return result + php_cases() + build_cases()


def run(args, data=None, env=None):
    p = subprocess.run([str(BINARY), *args], input=data.encode("utf-8") if data is not None else None, capture_output=True, env=env, check=True)
    assert not p.stderr, p.stderr
    return p.stdout.decode("utf-8")


def check_controls(home, work, env, case, key):
    _, command, original, _, _ = case
    controls = {
        'off': 'mode: off\n',
        'disabled': 'mode: smart\ncompressors:\n  php_test:\n    enabled: false\n',
        'safe-override': 'mode: smart\ncompressors:\n  php_test:\n    mode: safe\n',
        'off-override': 'mode: smart\ncompressors:\n  php_test:\n    mode: off\n',
        'threshold': 'mode: smart\nthresholds:\n  minimum_bytes: 1000000\n',
        'line-threshold': 'mode: smart\nthresholds:\n  minimum_lines: 1000000\n',
        'budget': 'mode: smart\ntarget:\n  max_reduction_percent: 1\n',
        'cache-disabled': 'mode: smart\ncache:\n  enabled: false\n',
    }
    for name, config in controls.items():
        Path(home, 'config.yaml').write_text('version: 1\n' + config.replace('php_test:', key + ':'))
        assert run(['optimize', '--command', command, '-'], original, env) == original, name
        for agent in ('claude', 'codex'):
            response = ({'stdout': original, 'stderr': '', 'interrupted': False, 'isImage': False, 'exitCode': 0}
                        if agent == 'claude' else {'output': original, 'exit_code': 0})
            payload = {'hook_event_name': 'PostToolUse', 'tool_name': 'Bash', 'cwd': str(work),
                       'tool_input': {'command': command}, 'tool_response': response}
            assert run(['hook', 'post-tool-use', '--agent', agent], json.dumps(payload), env) == '', (name, agent)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--check', action='store_true')
    parser.parse_args()
    if not BINARY.exists():
        raise SystemExit('Run make build first')
    generated = ROOT/'scenarios/generated'
    results = ROOT/'scenarios/results'
    generated.mkdir(exist_ok=True)
    results.mkdir(exist_ok=True)
    rows = []
    with tempfile.TemporaryDirectory(prefix='tokenslim-demo-') as temp:
        env = dict(os.environ, TOKENSLIM_HOME=temp)
        work = Path(temp)/'workspace'
        work.mkdir()
        baseline_source = work/'source.txt'
        baseline_source.write_text('This project source must remain untouched.\n')
        baseline = hashlib.sha256(baseline_source.read_bytes()).hexdigest()
        for name, command, original, critical, *settings in cases():
            options = settings[0] if settings else {}
            stderr = options.get("stderr", "")
            exit_code = options.get("exit_code", 1)
            file = generated/(name+'.log')
            file.write_text(original, encoding="utf-8", newline="")
            for mode in ('safe', 'smart'):
                Path(temp, 'config.yaml').write_text(f'version: 1\nmode: {mode}\n')
                benchmark = json.loads(run(['benchmark', '--command', command, '--json', str(file)], env=env))
                if 'changed' in options:
                    assert benchmark['changed'] == options['changed'][mode], (name, mode, benchmark)
                optimized = run(['optimize', '--command', command, str(file)], env=env)
                (results/f'{name}-{mode}.log').write_text(optimized, encoding="utf-8")
                assert benchmark['original_bytes'] == len(original.encode('utf-8'))
                assert benchmark['optimized_bytes'] == len(optimized.encode('utf-8'))
                for fingerprint in critical:
                    assert fingerprint in optimized, (name, mode, fingerprint)
                # Repeatability includes stable content-addressed references.
                assert optimized == run(['optimize', '--command', command, str(file)], env=env)
                reference = re.search(r'ref=(ts_[0-9a-f]{64})', optimized)
                if reference:
                    assert run(['cache', 'inspect', reference[1]], env=env) == original
                else:
                    assert optimized == original
                hook_stdout = original if options.get('stream') != 'stderr' else ''
                hook_stderr = stderr if options.get('stream') != 'stderr' else original + stderr
                for agent in ('claude', 'codex'):
                    response = ({'stdout': hook_stdout, 'stderr': hook_stderr, 'interrupted': False, 'isImage': False, 'exitCode': exit_code, 'durationMs': 20, 'future': {'preserve': True}}
                                if agent == 'claude' else {'output': original + stderr, 'exit_code': exit_code, 'wall_time_seconds': 0.02, 'chunk_id': 'php-demo', 'original_token_count': 123})
                    payload = {'hook_event_name': 'PostToolUse', 'tool_name': 'Bash',
                               'session_id': 'demo', 'tool_use_id': name, 'cwd': str(work),
                               'tool_input': {'command': command}, 'tool_response': response}
                    hook = run(['hook', 'post-tool-use', '--agent', agent], json.dumps(payload), env)
                    (results/f'{name}-{mode}-{agent}.json').write_text(hook or '{}\n')
                    if hook:
                        data = json.loads(hook)
                        if agent == 'claude':
                            replaced = data['hookSpecificOutput']['updatedToolOutput']
                            text = replaced['stdout'] + replaced['stderr']
                            if options.get('stream') == 'stderr':
                                assert replaced['stdout'] == ''
                            else:
                                assert replaced['stderr'] == stderr
                            assert {k: v for k, v in replaced.items() if k not in ('stdout', 'stderr')} == {k: v for k, v in response.items() if k not in ('stdout', 'stderr')}
                        else:
                            assert data['continue'] is False
                            replaced = json.loads(data['stopReason'])
                            text = replaced['output']
                            assert {k: v for k, v in replaced.items() if k != 'output'} == {k: v for k, v in response.items() if k != 'output'}
                            if stderr:
                                assert stderr in text
                        for fingerprint in critical:
                            assert fingerprint in text
                        ref = re.search(r'ref=(ts_[0-9a-f]{64})', text)[1]
                        assert run(['cache', 'inspect', ref], env=env) == json.dumps(response)
                    else:
                        assert not benchmark['changed'], (name, mode, agent, 'unexpected fallback')
                    # A pass-through result must also be expected for the new corpus.
                    if 'changed' in options:
                        assert bool(hook) == options['changed'][mode], (name, mode, agent)
                    # Native source reads are excluded on both adapters.
                    payload['tool_name'] = 'Read'
                    assert run(['hook', 'post-tool-use', '--agent', agent], json.dumps(payload), env) == ''
                rows.append(dict(scenario=name, **benchmark))
        control_cases = php_cases() + build_cases()
        for name, key in [('pest-success', 'php_test'), ('pytest-success', 'pytest'),
                          ('go-test-success', 'go_test'), ('vitest-success', 'node_test'),
                          ('gradle-success', 'gradle')]:
            check_controls(temp, work, env, next(c for c in control_cases if c[0] == name), key)
        assert hashlib.sha256(baseline_source.read_bytes()).hexdigest() == baseline
        stats = json.loads(run(['stats', '--json'], env=env))
        assert stats['Total']['Changed'] > 0
        assert run(['hook', 'post-tool-use'], '{invalid', env) == ''
    # Timing is measured separately and is intentionally not deterministic.
    report = ['# TokenSlim — local demo', '',
              'Synthetic corpus; does not represent actual billing savings. Tokens estimated as characters/4.',
              'Each case passed through the Claude Code and Codex adapters without model calls.', '',
              '| Scenario | Mode | Original B | Optimized B | Savings | Estimated tokens before → after |',
              '|---|---|---:|---:|---:|---:|']
    for r in rows:
        reduction = 100*(1-r['optimized_bytes']/r['original_bytes'])
        report.append(f"| {r['scenario']} | {r['mode']} | {r['original_bytes']} | {r['optimized_bytes']} | {reduction:.1f}% | {r['estimated_original_tokens']} → {r['estimated_optimized_tokens']} |")
    report += ['', 'Checks: diagnostics preserved; originals recovered; deterministic output;',
               'stdout/stderr and execution metadata preserved; PHP/build configuration controls honored;',
               'Read ignored; invalid JSON tolerated; source unchanged.',
               'These results validate the hook protocol. A live session requires installation and hook trust in the host agent.', '']
    (results/'report.md').write_text('\n'.join(report), encoding='utf-8')
    (results/'report.json').write_text(json.dumps(rows, indent=2)+'\n')
    print('\n'.join(report))
    print(f'Report: {results / "report.md"}')


if __name__ == '__main__':
    main()
