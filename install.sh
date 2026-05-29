#!/usr/bin/env bash
# Install outlook-pst-mcp from GitHub Releases into ~/.local/bin.
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/EvilFreelancer/outlook-pst-mcp/main/install.sh | bash
#   ./install.sh [--version X.Y.Z] [--install-dir DIR] [--workspace DIR] [-y]
set -euo pipefail

OUTLOOK_PST_MCP_REPO="${OUTLOOK_PST_MCP_REPO:-EvilFreelancer/outlook-pst-mcp}"
OUTLOOK_PST_MCP_API="${OUTLOOK_PST_MCP_API:-https://api.github.com}"
OUTLOOK_PST_MCP_INSTALL_DIR="${OUTLOOK_PST_MCP_INSTALL_DIR:-}"
OUTLOOK_PST_MCP_WORKSPACE="${OUTLOOK_PST_MCP_WORKSPACE:-}"
OUTLOOK_PST_MCP_VERSION="${OUTLOOK_PST_MCP_VERSION:-}"
YES=0

usage() {
  cat <<EOF
Usage: install.sh [options]

Installs the outlook-pst-mcp release binary and prints an MCP stdio
configuration for the installed path.

Options:
  --version X.Y.Z   Install a specific release (default: latest)
  --install-dir D   Binary directory (default: ~/.local/bin)
  --workspace D     MCP workspace directory (default: ~/.local/share/outlook-pst-mcp)
  --repo OWNER/NAME Override GitHub repo (default: EvilFreelancer/outlook-pst-mcp)
  -y, --yes         Non-interactive
  -h, --help        Show this help

Environment:
  OUTLOOK_PST_MCP_REPO, OUTLOOK_PST_MCP_VERSION, OUTLOOK_PST_MCP_INSTALL_DIR,
  OUTLOOK_PST_MCP_WORKSPACE, OUTLOOK_PST_MCP_API

After install:
  export PATH="\$HOME/.local/bin:\$PATH"   # if needed
  outlook-pst-mcp --help
  Add the printed mcpServers entry to your MCP client.
  Use import_pst with an absolute PST path after the client starts the server.
EOF
}

log() { printf 'outlook-pst-mcp-install: %s\n' "$*"; }
die() { log "error: $*"; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version) OUTLOOK_PST_MCP_VERSION="${2:-}"; shift 2 ;;
    --install-dir) OUTLOOK_PST_MCP_INSTALL_DIR="${2:-}"; shift 2 ;;
    --workspace) OUTLOOK_PST_MCP_WORKSPACE="${2:-}"; shift 2 ;;
    --repo) OUTLOOK_PST_MCP_REPO="${2:-}"; shift 2 ;;
    -y|--yes) YES=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1 (try --help)" ;;
  esac
done

need_cmd uname
need_cmd curl
need_cmd tar
need_cmd install
need_cmd mktemp

OS="$(uname -s)"
ARCH="$(uname -m)"
case "$OS" in
  Linux) GOOS=linux ;;
  *) die "unsupported OS: $OS (use install.ps1 on Windows)" ;;
esac
case "$ARCH" in
  x86_64|amd64) GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  *) die "unsupported CPU: $ARCH" ;;
esac

if [ -z "$OUTLOOK_PST_MCP_INSTALL_DIR" ]; then
  OUTLOOK_PST_MCP_INSTALL_DIR="${HOME}/.local/bin"
fi
if [ -z "$OUTLOOK_PST_MCP_WORKSPACE" ]; then
  OUTLOOK_PST_MCP_WORKSPACE="${HOME}/.local/share/outlook-pst-mcp"
fi

mkdir -p "$OUTLOOK_PST_MCP_INSTALL_DIR" "$OUTLOOK_PST_MCP_WORKSPACE"

api_latest="${OUTLOOK_PST_MCP_API%/}/repos/${OUTLOOK_PST_MCP_REPO}/releases/latest"
api_tag=""
if [ -n "$OUTLOOK_PST_MCP_VERSION" ]; then
  ver="${OUTLOOK_PST_MCP_VERSION#v}"
  api_tag="${OUTLOOK_PST_MCP_API%/}/repos/${OUTLOOK_PST_MCP_REPO}/releases/tags/${ver}"
fi

fetch_release_json() {
  local url="$1"
  curl -fsSL \
    -H "Accept: application/vnd.github+json" \
    -H "User-Agent: outlook-pst-mcp-install" \
    "$url"
}

if [ -n "$api_tag" ]; then
  REL_JSON="$(fetch_release_json "$api_tag")" || die "release ${OUTLOOK_PST_MCP_VERSION} not found on ${OUTLOOK_PST_MCP_REPO}"
else
  REL_JSON="$(fetch_release_json "$api_latest")" || die "could not fetch latest release from ${OUTLOOK_PST_MCP_REPO}"
fi

TAG="$(printf '%s' "$REL_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')"
TAG="${TAG#v}"
[ -n "$TAG" ] || die "could not parse release tag from GitHub API"

ASSET="outlook-pst-mcp_${TAG}_${GOOS}_${GOARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${OUTLOOK_PST_MCP_REPO}/releases/download/${TAG}/${ASSET}"
CHECKSUM_URL="https://github.com/${OUTLOOK_PST_MCP_REPO}/releases/download/${TAG}/SHA256SUMS"
DEST="${OUTLOOK_PST_MCP_INSTALL_DIR}/outlook-pst-mcp"

if [ -f "$DEST" ] && [ "$YES" -eq 0 ]; then
  printf 'Replace existing %s with %s? [y/N] ' "$DEST" "$TAG"
  read -r ans || ans=""
  case "$ans" in
    y|Y|yes|YES) ;;
    *) log "cancelled"; exit 0 ;;
  esac
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

log "downloading ${ASSET} (${TAG})"
curl -fsSL -o "${TMP}/${ASSET}" "$DOWNLOAD_URL"
if command -v sha256sum >/dev/null 2>&1; then
  curl -fsSL -o "${TMP}/SHA256SUMS" "$CHECKSUM_URL"
  (cd "$TMP" && sha256sum -c SHA256SUMS --ignore-missing)
else
  log "sha256sum not found; skipping checksum verification"
fi

tar -xzf "${TMP}/${ASSET}" -C "$TMP"
[ -f "${TMP}/outlook-pst-mcp" ] || die "archive missing outlook-pst-mcp binary"
install -m 0755 "${TMP}/outlook-pst-mcp" "$DEST"
log "installed ${DEST}"

if ! command -v readpst >/dev/null 2>&1; then
  log "readpst not found; install pst-utils/libpst before importing real PST files"
fi

if ! printf '%s' "${PATH:-}" | tr ':' '\n' | grep -qx "$OUTLOOK_PST_MCP_INSTALL_DIR"; then
  log "add to PATH: export PATH=\"${OUTLOOK_PST_MCP_INSTALL_DIR}:\$PATH\""
fi

log "done: $("$DEST" --help 2>/dev/null | head -1 || echo "outlook-pst-mcp installed")"
cat <<EOF

MCP client configuration:
{
  "mcpServers": {
    "outlook-pst": {
      "command": "${DEST}",
      "args": ["-workspace", "${OUTLOOK_PST_MCP_WORKSPACE}"]
    }
  }
}

Next: restart your MCP client and call import_pst with an absolute PST path.
EOF
