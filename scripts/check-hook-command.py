#!/usr/bin/env python3
"""Exercise the packaged Bash command with a plugin root containing spaces."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

root = Path(__file__).resolve().parents[1]
source = root / 'bin' / ('tokenslim.exe' if os.name == 'nt' else 'tokenslim')
with tempfile.TemporaryDirectory(prefix='tokenslim hook ') as temporary:
    stage = Path(temporary)
    (stage / 'bin').mkdir()
    shutil.copy2(source, stage / 'bin' / source.name)
    for agent, plugin in [('claude', root), ('codex', root / 'integrations/codex/tokenslim')]:
        command = json.loads((plugin / 'hooks/hooks.json').read_text())['hooks']['PostToolUse'][0]['hooks'][0]['command']
        original = ''.join(f'Worker heartbeat partition={i}: checking queue for new tasks\n' * 12
                           + f'Checkpoint {i}: processed batch with 120 records; elapsed=40ms\n'
                           for i in range(60)) + 'ERROR build failed\n'
        payload = {'hook_event_name': 'PostToolUse', 'tool_name': 'Bash',
                   'tool_input': {'command': 'build'},
                   'cwd': str(stage),
                   'tool_response': {'stdout': original, 'stderr': '', 'interrupted': False,
                                     'isImage': False} if agent == 'claude' else original}
        result = subprocess.run(['bash', '-c', command], input=json.dumps(payload),
                                text=True, encoding='utf-8', capture_output=True, check=True,
                                env=dict(os.environ, CLAUDE_PLUGIN_ROOT=stage.as_posix(),
                                         TOKENSLIM_HOME=str(stage / 'state')))
        response = json.loads(result.stdout)
        assert 'ERROR build failed' in json.dumps(response), response
        assert 'ts_' in result.stdout, response
        assert not result.stderr, result.stderr
print('Both hook commands passed with a spaced plugin path')
