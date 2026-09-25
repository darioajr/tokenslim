"""End-to-end custom rules, recovery, metrics and report checks (stdlib only)."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile

CONFIG = r'''mode: smart
rules:
  - name: health
    match:
      command: 'worker logs*'
      regex: '(?P<tick>[0-9]+) INFO api=\w+ GET /health duration=(?P<duration>[0-9]+)ms'
    ignore_groups: [tick, duration]
    action:
      aggregate: true
'''


def check(binary):
    with tempfile.TemporaryDirectory(prefix='tokenslim-rules-') as home:
        env = dict(os.environ, TOKENSLIM_HOME=home)
        config = Path(home, 'config.yaml')
        original = ''.join(f'{i} INFO api=service{group} GET /health duration={i}ms\n'
                           for group in range(20) for i in range(10))
        diagnostic = 'ERROR private-diagnostic-marker\n123 INFO api=service0 GET /health duration=9ms\n'
        original += diagnostic
        def run(args, data=None, check=True):
            return subprocess.run([str(binary), *args], input=data, env=env,
                                  capture_output=True, check=check)
        for mode, extra, changed in [('safe', '', False), ('off', '', False),
                                     ('smart', 'cache:\n  enabled: false\n', False),
                                     ('smart', 'compressors:\n  generic:\n    enabled: false\n', False),
                                     ('smart', 'target:\n  max_reduction_percent: 1\n', False),
                                     ('smart', '', True)]:
            config.write_text(CONFIG.replace('mode: smart', 'mode: '+mode) + extra)
            assert run(['config', 'validate']).stdout == b'Configuration valid\n'
            benchmark = json.loads(run(['benchmark', '--json', '--command', 'worker logs', '-'], original.encode()).stdout)
            assert benchmark['changed'] == changed
            assert bool(benchmark.get('rules')) == changed
            optimized = run(['optimize', '--command', 'worker logs', '-'], original.encode()).stdout
            if changed:
                assert benchmark['rules']['health'] == {'groups': 20, 'lines': 200}
                assert diagnostic.encode() in optimized
                ref = re.search(rb'ref=(ts_[a-f0-9]{64})', optimized)[1].decode()
                assert run(['cache', 'inspect', ref]).stdout == original.encode()
            else:
                assert optimized == original.encode()
        # Smart configuration is now active. Both adapters must preserve metadata
        # and the complete diagnostic body, with exact JSON recovery.
        for agent in ('claude', 'codex'):
            response = ({'stdout': original, 'stderr': 'WARN separate stderr\n', 'interrupted': False,
                         'isImage': False, 'exitCode': 1} if agent == 'claude' else
                        {'output': original, 'exit_code': 1, 'wall_time_seconds': 0.25})
            payload = {'hook_event_name': 'PostToolUse', 'tool_name': 'Bash', 'cwd': home,
                       'session_id': 'rules-demo', 'tool_input': {'command': 'worker logs'},
                       'tool_response': response}
            result = run(['hook', 'post-tool-use', '--agent', agent], json.dumps(payload).encode()).stdout
            data = json.loads(result)
            updated = data['hookSpecificOutput']['updatedToolOutput'] if agent == 'claude' else json.loads(data['stopReason'])
            text = updated['stdout'] if agent == 'claude' else updated['output']
            assert diagnostic in text and 'rule=health' in text
            for key in response:
                if key not in ('stdout', 'output'):
                    assert updated[key] == response[key]
            ref = re.search(r'ref=(ts_[a-f0-9]{64})', text)[1]
            assert run(['cache', 'inspect', ref]).stdout.decode() == json.dumps(response)
            config.write_text(CONFIG.replace('(?P<tick>[0-9]+)', '['))
            assert run(['hook', 'post-tool-use', '--agent', agent], json.dumps(payload).encode()).stdout == b''
            assert run(['config', 'validate'], check=False).returncode != 0
            config.write_text(CONFIG)
        def snapshot():
            return {str(p.relative_to(home)): hashlib.sha256(p.read_bytes()).hexdigest()
                    for p in Path(home).rglob('*') if p.is_file()}
        before = snapshot()
        reports = {}
        for format in ('text', 'json', 'html'):
            args = ['report', '--format', format]
            reports[format] = run(args).stdout
            assert reports[format] == run(args).stdout
            assert b'private-diagnostic-marker' not in reports[format]
        report = json.loads(reports['json'])
        assert report['rules'] == [{'name': 'health', 'records': 3, 'groups': 60, 'lines': 600}], report
        assert report['total']['processed'] == 8 and report['total']['changed'] == 3
        scoped = json.loads(run(['report', '--session', 'rules-demo', '--format', 'json']).stdout)
        assert scoped['total']['processed'] == 2 and scoped['rules'][0]['records'] == 2
        assert snapshot() == before, 'report wrote local state'
        output = binary.parents[1] / 'scenarios' / 'results'
        output.mkdir(parents=True, exist_ok=True)
        for format, content in reports.items():
            (output / ('rules-report.' + ('txt' if format == 'text' else format))).write_bytes(content)
    print('Custom rules: grouping, diagnostics, cache, both adapters and deterministic reports verified.')


if __name__ == '__main__':
    root = Path(__file__).resolve().parents[1]
    check(root / 'bin' / ('tokenslim.exe' if os.name == 'nt' else 'tokenslim'))
