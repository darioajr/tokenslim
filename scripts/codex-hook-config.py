#!/usr/bin/env python3
"""Print a Codex hook configuration without changing user or project settings."""
import json
from pathlib import Path
import shlex
import os

binary = Path(__file__).resolve().parents[1] / 'bin' / ('tokenslim.exe' if os.name == 'nt' else 'tokenslim')
print(json.dumps({'hooks': {'PostToolUse': [{'matcher': '^Bash$', 'hooks': [
    {'type': 'command', 'command': shlex.quote(binary.as_posix()) + ' hook post-tool-use --agent codex', 'timeout': 10}
]}]}}, indent=2))
