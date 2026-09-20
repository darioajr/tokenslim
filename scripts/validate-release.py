#!/usr/bin/env python3
"""Validate release metadata and optionally the complete distribution (stdlib only)."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import tarfile
import zipfile
import stat
import struct

ROOT = Path(__file__).resolve().parents[1]


def validate(tag=None, packages=False):
    version = (ROOT / 'VERSION').read_text().strip()
    if not re.fullmatch(r'(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)', version):
        raise ValueError('VERSION must be a stable version such as 0.1.0')
    if tag is not None and tag != 'v' + version:
        raise ValueError(f'tag {tag!r} does not match v{version}')
    roots = {'claude': ROOT, 'codex': ROOT / 'integrations/codex/tokenslim'}
    for agent, root in roots.items():
        manifest = json.loads((root / f'.{agent}-plugin/plugin.json').read_text())
        if manifest.get('name') != 'tokenslim' or manifest.get('version') != version:
            raise ValueError(f'{agent} manifest does not match VERSION ({version})')
        hooks = json.loads((root / 'hooks/hooks.json').read_text())['hooks']
        groups = hooks['PostToolUse']
        if set(hooks) != {'PostToolUse'} or len(groups) != 1 or groups[0]['matcher'] != '^Bash$':
            raise ValueError(f'{agent}: expected Bash-only PostToolUse hook')
        command = groups[0]['hooks'][0]['command']
        if f'hook post-tool-use --agent {agent}' not in command:
            raise ValueError(f'{agent}: wrong adapter command')
        if not (root / 'skills/tokenslim/SKILL.md').is_file():
            raise ValueError(f'{agent}: missing recovery skill')
    if packages:
        expected = {f"tokenslim-{version}-{agent}-{os_name}-{arch}." +
                    ('zip' if os_name == 'windows' else 'tar.gz')
                    for agent in roots for os_name in ('linux', 'darwin', 'windows')
                    for arch in ('amd64', 'arm64')}
        dist = ROOT / 'dist'
        entries = [line.split('  ', 1) for line in (dist / 'SHA256SUMS').read_text().splitlines()]
        if len(entries) != len(expected) or {name for _, name in entries} != expected:
            raise ValueError('checksums must list exactly the twelve current-version packages')
        for digest, name in entries:
            path = dist / name
            if hashlib.sha256(path.read_bytes()).hexdigest() != digest:
                raise ValueError(f'checksum mismatch: {name}')
            agent = 'claude' if '-claude-' in name else 'codex'
            windows = '-windows-' in name
            binary = 'bin/tokenslim.exe' if windows else 'bin/tokenslim'
            required = {binary, f'.{agent}-plugin/plugin.json', 'hooks/hooks.json',
                        'skills/tokenslim/SKILL.md', 'README.md', 'LICENSE', 'VERSION',
                        'docs/ci-cd.md', 'scripts/codex-hook-config.py'}
            members = {}
            modes = {}
            if windows:
                with zipfile.ZipFile(path) as archive:
                    for member in archive.infolist():
                        relative = member.filename
                        mode = member.external_attr >> 16
                        if relative.startswith('/') or '..' in Path(relative).parts or stat.S_ISLNK(mode):
                            raise ValueError(f'unsafe archive member: {relative}')
                        members[relative] = archive.read(member)
                        modes[relative] = mode
            else:
                with tarfile.open(path, 'r:gz') as archive:
                    for member in archive.getmembers():
                        relative = member.name.removeprefix('./')
                        if member.name.startswith('/') or '..' in Path(relative).parts or member.issym() or member.islnk():
                            raise ValueError(f'unsafe archive member: {member.name}')
                        if member.isfile():
                            members[relative] = archive.extractfile(member).read()
                            modes[relative] = member.mode
            if not required <= members.keys():
                raise ValueError(f'incomplete archive: {name}')
            if not windows and modes[binary] & 0o111 == 0:
                raise ValueError(f'non-executable binary: {name}')
            if windows and not members[binary].startswith(b'MZ'):
                raise ValueError(f'invalid Windows executable: {name}')
            if windows:
                executable = members[binary]
                pe = struct.unpack_from('<I', executable, 0x3c)[0]
                machine = 0xAA64 if '-arm64.' in name else 0x8664
                if (executable[pe:pe+4] != b'PE\0\0'
                        or struct.unpack_from('<H', executable, pe+4)[0] != machine
                        or struct.unpack_from('<H', executable, pe+24)[0] != 0x20b):
                    raise ValueError(f'wrong Windows architecture: {name}')
            manifest = json.loads(members[f'.{agent}-plugin/plugin.json'])
            if manifest['version'] != version:
                raise ValueError(f'archive manifest version mismatch: {name}')
    return version


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--tag')
    parser.add_argument('--packages', action='store_true')
    args = parser.parse_args()
    try:
        print(f'Release {validate(args.tag, args.packages)} validated')
    except (ValueError, KeyError, OSError, tarfile.TarError, zipfile.BadZipFile, struct.error) as error:
        parser.exit(1, f'Release validation failed: {error}\n')
