#!/bin/sh
set -eu
version=${VERSION:-$(cat VERSION)}
python3 scripts/validate-release.py --tag "v$version"
export TOKENSLIM_RELEASE_VERSION="$version"
mkdir -p dist
for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  platform=${target%/*}
  arch=${target#*/}
  suffix=""
  if [ "$platform" = windows ]; then suffix=".exe"; fi
  stage=$(mktemp -d)
  trap 'rm -rf "$stage"' EXIT HUP INT TERM
  mkdir -p "$stage/claude/bin" "$stage/codex/bin"
  CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$stage/claude/bin/tokenslim$suffix" ./cmd/tokenslim
  cp "$stage/claude/bin/tokenslim$suffix" "$stage/codex/bin/tokenslim$suffix"
  cp -R .claude-plugin hooks skills "$stage/claude/"
  cp -R integrations/codex/tokenslim/.codex-plugin integrations/codex/tokenslim/hooks integrations/codex/tokenslim/skills "$stage/codex/"
  cp README.md LICENSE VERSION "$stage/claude/"
  cp -R docs "$stage/claude/"
  mkdir -p "$stage/claude/scripts"
  cp scripts/codex-hook-config.py "$stage/claude/scripts/"
  cp README.md LICENSE VERSION "$stage/codex/"
  cp -R docs "$stage/codex/"
  mkdir -p "$stage/codex/scripts"
  cp scripts/codex-hook-config.py "$stage/codex/scripts/"
  for agent in claude codex; do
    if [ "$platform" = windows ]; then
      python3 - "$stage/$agent" "dist/tokenslim-$version-$agent-$platform-$arch.zip" <<'ZIP'
from pathlib import Path
import sys
import zipfile
root = Path(sys.argv[1])
with zipfile.ZipFile(sys.argv[2], 'w', zipfile.ZIP_DEFLATED) as archive:
    for path in sorted(root.rglob('*')):
        if path.is_file():
            archive.write(path, path.relative_to(root).as_posix())
ZIP
    else
      tar -czf "dist/tokenslim-$version-$agent-$platform-$arch.tar.gz" -C "$stage/$agent" .
    fi
  done
  rm -rf "$stage"
  trap - EXIT HUP INT TERM
done
python3 - <<'PY'
from pathlib import Path
import hashlib
import os
archives = sorted(p for ext in ('tar.gz', 'zip')
                  for p in Path('dist').glob(f"tokenslim-{os.environ['TOKENSLIM_RELEASE_VERSION']}-*.{ext}"))
Path('dist/SHA256SUMS').write_text(''.join(f'{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n' for p in archives))
PY
