"""Exercise a real MCP subprocess against CLI and both adapter cache formats."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import re


def check(binary):
    with tempfile.TemporaryDirectory(prefix='tokenslim-mcp-') as home:
        env = dict(os.environ, TOKENSLIM_HOME=home)
        Path(home, 'config.yaml').write_text('thresholds:\n  minimum_bytes: 0\n  minimum_lines: 0\n')
        original = 'progress event\r\n' * 100 + 'ERROR falha ação\r\nlast'
        def run(args, data):
            return subprocess.check_output([str(binary), *args], input=data, env=env)
        optimized = run(['optimize', '--command', 'build', '-'], original.encode())
        ref = re.search(rb'ref=(ts_[a-f0-9]{64})', optimized)[1].decode()
        records = [(ref, original, 'raw')]
        for agent in ('claude', 'codex'):
            response = ({'stdout': original, 'stderr': 'WARN stderr\n', 'interrupted': False,
                         'isImage': False, 'exitCode': 1} if agent == 'claude' else
                        {'output': original, 'exit_code': 1})
            payload = {'hook_event_name': 'PostToolUse', 'tool_name': 'Bash', 'cwd': home,
                       'tool_input': {'command': 'build'}, 'tool_response': response}
            result = run(['hook', 'post-tool-use', '--agent', agent], json.dumps(payload).encode())
            ref = re.search(rb'ref=(ts_[a-f0-9]{64})', result)[1].decode()
            records.append((ref, json.dumps(response), 'stdout' if agent == 'claude' else 'output'))
        def snapshot():
            return {str(p.relative_to(home)): hashlib.sha256(p.read_bytes()).hexdigest()
                    for p in Path(home).rglob('*') if p.is_file()}
        before = snapshot()
        messages = [dict(jsonrpc='2.0', id=1, method='initialize', params={
            'protocolVersion': '2025-11-25', 'capabilities': {},
            'clientInfo': {'name': 'tokenslim-scenario', 'version': '1'}}),
            dict(jsonrpc='2.0', method='notifications/initialized'),
            dict(jsonrpc='2.0', id=2, method='tools/list')]
        expected = []
        for ref, raw, stream in records:
            for name, arguments in [
                ('tokenslim_describe', {'ref': ref}),
                ('tokenslim_get_original', {'ref': ref}),
                ('tokenslim_get_range', {'ref': ref, 'start_line': 101, 'end_line': 102}),
                ('tokenslim_search_original', {'ref': ref, 'query': 'falha'}),
            ]:
                messages.append(dict(jsonrpc='2.0', id=len(messages), method='tools/call',
                                     params={'name': name, 'arguments': arguments}))
            expected.append((ref, raw, stream))
        proc = subprocess.run([str(binary), 'mcp', 'serve'], env=env,
                              input=b''.join(json.dumps(m).encode()+b'\n' for m in messages),
                              capture_output=True, check=True, timeout=30)
        assert not proc.stderr, proc.stderr
        results = [json.loads(line) for line in proc.stdout.splitlines()]
        assert len(results) == 14, results
        assert results[0]['result']['protocolVersion'] == '2025-11-25'
        assert len(results[1]['result']['tools']) == 4
        for i, (ref, raw, stream) in enumerate(expected):
            description, full, lines, search = [r['result'] for r in results[2+i*4:6+i*4]]
            assert all(not r['isError'] for r in (description, full, lines, search))
            assert description['structuredContent']['record']['id'] == ref
            assert full['structuredContent']['text'] == raw
            assert lines['structuredContent']['stream'] == stream
            assert lines['structuredContent']['text'] == 'ERROR falha ação\r\nlast'
            assert search['structuredContent']['matches'][0]['line'] == 101
        assert snapshot() == before, 'MCP changed local state'
    print('MCP recovery: CLI, Claude and Codex originals/ranges/search verified; no writes.')


if __name__ == '__main__':
    root = Path(__file__).resolve().parents[1]
    check(root / 'bin' / ('tokenslim.exe' if os.name == 'nt' else 'tokenslim'))
