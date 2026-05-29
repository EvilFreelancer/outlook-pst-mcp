#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUNDLE="${ROOT}/tools/readpst-bundle"
export LD_LIBRARY_PATH="${BUNDLE}/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"
export PATH="${BUNDLE}/usr/bin:${PATH}"
exec "${ROOT}/bin/outlook-pst-mcp" -workspace "${ROOT}/workspace" "$@"
