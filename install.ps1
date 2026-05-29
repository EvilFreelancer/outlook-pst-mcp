# Install outlook-pst-mcp from GitHub Releases (Windows amd64).
# Usage:
#   irm https://raw.githubusercontent.com/EvilFreelancer/outlook-pst-mcp/main/install.ps1 | iex
#   .\install.ps1 [-Version "0.1.1"] [-InstallDir $path] [-Workspace $path] [-Yes]
param(
    [string]$Version = $env:OUTLOOK_PST_MCP_VERSION,
    [string]$Repo = $(if ($env:OUTLOOK_PST_MCP_REPO) { $env:OUTLOOK_PST_MCP_REPO } else { "EvilFreelancer/outlook-pst-mcp" }),
    [string]$InstallDir = $(if ($env:OUTLOOK_PST_MCP_INSTALL_DIR) { $env:OUTLOOK_PST_MCP_INSTALL_DIR } else { "" }),
    [string]$Workspace = $(if ($env:OUTLOOK_PST_MCP_WORKSPACE) { $env:OUTLOOK_PST_MCP_WORKSPACE } else { "" }),
    [string]$Api = $(if ($env:OUTLOOK_PST_MCP_API) { $env:OUTLOOK_PST_MCP_API } else { "https://api.github.com" }),
    [switch]$Yes
)

$ErrorActionPreference = "Stop"

function Write-Info([string]$Message) { Write-Host "outlook-pst-mcp-install: $Message" }

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\outlook-pst-mcp"
}
if (-not $Workspace) {
    $Workspace = Join-Path $env:LOCALAPPDATA "outlook-pst-mcp"
}

$arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -notmatch "64") {
    throw "outlook-pst-mcp-install: Windows arm64 is not published yet; use amd64 Windows."
}

$headers = @{
    Accept       = "application/vnd.github+json"
    "User-Agent" = "outlook-pst-mcp-install"
}

if ($Version) {
    $ver = $Version.TrimStart("v")
    $relUri = "$Api/repos/$Repo/releases/tags/$ver"
} else {
    $relUri = "$Api/repos/$Repo/releases/latest"
}

$rel = Invoke-RestMethod -Uri $relUri -Headers $headers
$tag = ($rel.tag_name -replace "^v", "").Trim()
if (-not $tag) { throw "outlook-pst-mcp-install: empty release tag" }

$asset = "outlook-pst-mcp_${tag}_windows_${arch}.zip"
$downloadUrl = "https://github.com/$Repo/releases/download/$tag/$asset"

New-Item -ItemType Directory -Force -Path $InstallDir, $Workspace | Out-Null

$dest = Join-Path $InstallDir "outlook-pst-mcp.exe"
if ((Test-Path $dest) -and -not $Yes) {
    $ans = Read-Host "Replace existing $dest with $tag? [y/N]"
    if ($ans -notmatch "^[yY]") {
        Write-Info "cancelled"
        exit 0
    }
}

$tmp = Join-Path $env:TEMP ("outlook-pst-mcp-install-" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    Write-Info "downloading $asset ($tag)"
    $zipPath = Join-Path $tmp "archive.zip"
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
    Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
    $bin = Join-Path $tmp "outlook-pst-mcp.exe"
    if (-not (Test-Path $bin)) { throw "outlook-pst-mcp-install: archive missing outlook-pst-mcp.exe" }
    Copy-Item -Path $bin -Destination $dest -Force
    Write-Info "installed $dest"
} finally {
    Remove-Item -Recurse -Force -Path $tmp -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    $newPath = if ($userPath) { "$InstallDir;$userPath" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$InstallDir;$env:Path"
    Write-Info "added $InstallDir to user PATH (open a new terminal if needed)"
}

Write-Info "done"
& $dest --help

Write-Host ""
Write-Host "MCP client configuration:"
Write-Host "{"
Write-Host '  "mcpServers": {'
Write-Host '    "outlook-pst": {'
Write-Host "      `"command`": `"$dest`","
Write-Host "      `"args`": [`"-workspace`", `"$Workspace`"]"
Write-Host "    }"
Write-Host "  }"
Write-Host "}"
Write-Host ""
Write-Host "Next: restart your MCP client and call import_pst with an absolute PST path."
