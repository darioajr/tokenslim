#!/usr/bin/env python3
"""Exercise the packaged Bash command with a plugin root containing spaces."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

def select_bash(windows=None):
    """Use Git Bash on Windows, never the System32 WSL launcher."""
    if windows is None:
        windows = os.name == 'nt'
    override = os.environ.get('TOKENSLIM_TEST_BASH')
    if override:
        path = Path(override)
        if not path.is_absolute() or not path.is_file():
            raise RuntimeError('TOKENSLIM_TEST_BASH must name an existing absolute Bash path')
        return path
    if windows:
        candidates = []
        git = shutil.which('git')
        if git:
            git_path = Path(git).resolve()
            for directory in (git_path.parent, git_path.parent.parent):
                candidates.extend([directory / 'bin/bash.exe', directory / 'usr/bin/bash.exe'])
        for variable in ('ProgramFiles', 'ProgramFiles(x86)', 'LOCALAPPDATA'):
            directory = os.environ.get(variable)
            if directory:
                prefix = Path(directory)
                if variable == 'LOCALAPPDATA':
                    prefix /= 'Programs'
                candidates.append(prefix / 'Git/bin/bash.exe')
        for candidate in candidates:
            if candidate.is_file():
                return candidate
        raise RuntimeError('Git Bash not found. Set TOKENSLIM_TEST_BASH to its absolute bash.exe path.')
    bash = shutil.which('bash')
    if not bash:
        raise RuntimeError('Bash not found on PATH')
    return Path(bash)


def main():
    bash = select_bash()
    print(f'Hook test shell: {bash}', flush=True)
    if os.name == 'nt':
        probe = subprocess.run([str(bash), '-c', 'uname -s'], text=True,
                               encoding='utf-8', capture_output=True, timeout=30)
        if probe.returncode or not probe.stdout.startswith(('MINGW', 'MSYS')):
            raise RuntimeError(f'Expected Git Bash, got {bash}: '
                               f'{probe.stdout} {probe.stderr}')
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
            result = subprocess.run([str(bash), '-c', command], input=json.dumps(payload),
                                    text=True, encoding='utf-8', capture_output=True, timeout=30,
                                    env=dict(os.environ, CLAUDE_PLUGIN_ROOT=stage.as_posix(),
                                             TOKENSLIM_HOME=str(stage / 'state')))
            if result.returncode:
                raise RuntimeError(f'{agent} hook failed using {bash} (exit {result.returncode})\n'
                                   f'stdout: {result.stdout}\nstderr: {result.stderr}')
            if not result.stdout.strip():
                raise RuntimeError(f'{agent} hook returned no replacement using {bash}\n'
                                   f'stderr: {result.stderr}')
            response = json.loads(result.stdout)
            assert 'ERROR build failed' in json.dumps(response), response
            assert 'ts_' in result.stdout, response
            assert not result.stderr, result.stderr
    print('Both hook commands passed with a spaced plugin path')


if __name__ == '__main__':
    main()
