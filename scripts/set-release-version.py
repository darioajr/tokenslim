#!/usr/bin/env python3
"""Stamp release metadata from a validated tag in the build checkout."""
import argparse
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
TAG_PATTERN = r'v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)'


def stamp(tag, root=ROOT):
    if not re.fullmatch(TAG_PATTERN, tag):
        raise ValueError('release tag must be vMAJOR.MINOR.PATCH (for example v1.2.3)')
    version = tag[1:]
    paths = [root / '.claude-plugin/plugin.json',
             root / 'integrations/codex/tokenslim/.codex-plugin/plugin.json']
    # Read and validate both manifests before changing any files.
    manifests = []
    for path in paths:
        manifest = json.loads(path.read_text(encoding='utf-8'))
        if manifest.get('name') != 'tokenslim':
            raise ValueError(f'unexpected plugin name in {path}')
        manifest['version'] = version
        manifests.append((path, manifest))
    for path, manifest in manifests:
        path.write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
    (root / 'VERSION').write_text(version + '\n', encoding='utf-8')
    return version


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--tag', required=True)
    args = parser.parse_args()
    try:
        print(f'Release metadata set to {stamp(args.tag)}')
    except (ValueError, OSError) as error:
        parser.exit(1, f'Cannot set release version: {error}\n')
