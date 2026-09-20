#!/usr/bin/env python3
"""Build both local plugin packages on Linux, macOS, or Windows."""
import os
from pathlib import Path
import shutil
import subprocess

root = Path(__file__).resolve().parents[1]
version = os.environ.get('VERSION') or (root / 'VERSION').read_text().strip()
extension = subprocess.check_output(['go', 'env', 'GOEXE'], text=True).strip()
binary = root / 'bin' / ('tokenslim' + extension)
binary.parent.mkdir(exist_ok=True)
subprocess.run(['go', 'build', '-trimpath', '-ldflags', f'-s -w -X main.version={version}',
                '-o', str(binary), './cmd/tokenslim'], cwd=root, check=True)
destination = root / 'integrations/codex/tokenslim/bin' / binary.name
destination.parent.mkdir(exist_ok=True)
shutil.copy2(binary, destination)
