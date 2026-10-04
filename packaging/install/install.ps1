<#
.SYNOPSIS
DevCade installer for Windows (user-local, no administrator rights).

.DESCRIPTION
Downloads devcade_<version>_windows_<arch>.zip and SHA256SUMS from the base
URL, verifies the archive's SHA-256 against SHA256SUMS, and only then extracts
it and installs devcade.exe, README.md and LICENSE-NOTICE.txt into the install
directory (default: %LOCALAPPDATA%\Programs\devcade). Nothing downloaded is
executed. An existing devcade.exe that is not a DevCade binary is never
replaced unless -Force is given. PATH is not changed; the script prints the
command to add the directory yourself.

Run it from a file (not through Invoke-Expression):
  powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Version 1.0.0

.PARAMETER Version
Release version, for example 1.0.0 or 1.0.0-rc.1 (env: DEVCADE_VERSION).

.PARAMETER BaseUrl
https:// URL prefix or local directory holding the release files
(env: DEVCADE_BASE_URL). Default:
https://github.com/cagridursun/devcade/releases/download/v<version>

.PARAMETER InstallDir
Target directory (env: DEVCADE_INSTALL_DIR).

.PARAMETER Arch
amd64 or arm64. Detected automatically when omitted.

.PARAMETER Force
Replace an existing devcade.exe even if it is not recognized as DevCade.
#>
[CmdletBinding()]
param(
    [string]$Version = $env:DEVCADE_VERSION,
    [string]$BaseUrl = $env:DEVCADE_BASE_URL,
    [string]$InstallDir = $env:DEVCADE_INSTALL_DIR,
    [string]$Arch,
    [switch]$Force
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Marker = 'github.com/cagridursun/devcade'

function Say([string]$Message) { Write-Host "devcade-install: $Message" }
function Fail([string]$Message) {
    [Console]::Error.WriteLine("devcade-install: error: $Message")
    exit 1
}

function Get-DetectedArch {
    $name = $null
    try {
        $name = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    } catch {
        $name = $null
    }
    if (-not $name) {
        $name = $env:PROCESSOR_ARCHITEW6432
        if (-not $name) { $name = $env:PROCESSOR_ARCHITECTURE }
    }
    if (-not $name) { $name = 'unknown' }
    switch ($name.ToUpperInvariant()) {
        'X64' { return 'amd64' }
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { Fail "unsupported architecture: $name (supported: x64/amd64, arm64)" }
    }
}

function Test-IsDevCade([string]$Path) {
    try {
        $bytes = [System.IO.File]::ReadAllBytes($Path)
    } catch {
        return $false
    }
    # Latin-1 maps every byte to one char, so the ASCII marker is found as-is.
    $text = [System.Text.Encoding]::GetEncoding(28591).GetString($bytes)
    return $text.Contains($Marker)
}

function Assert-Destination([string]$Dest) {
    if (Test-Path -LiteralPath $Dest -PathType Container) {
        Fail "$Dest is a directory; refusing to replace it"
    }
    if ((Test-Path -LiteralPath $Dest) -and -not $Force) {
        if (-not (Test-IsDevCade $Dest)) {
            Fail "$Dest exists and is not a DevCade binary; refusing to overwrite it (remove it or use -Force)"
        }
    }
}

# --- version ----------------------------------------------------------------

if (-not $Version) { Fail 'no version given; use -Version X.Y.Z (or DEVCADE_VERSION)' }
if ($Version.StartsWith('v')) { $Version = $Version.Substring(1) }
if ($Version -cnotmatch '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$') {
    Fail "invalid version '$Version' (expected X.Y.Z or X.Y.Z-prerelease)"
}

# --- platform ---------------------------------------------------------------

if ($Arch) {
    $Arch = $Arch.ToLowerInvariant()
    if ($Arch -eq 'x64') { $Arch = 'amd64' }
    if ($Arch -ne 'amd64' -and $Arch -ne 'arm64') {
        Fail "unsupported architecture: $Arch (supported: amd64, arm64)"
    }
} else {
    $Arch = Get-DetectedArch
}

# --- locations --------------------------------------------------------------

if (-not $BaseUrl) { $BaseUrl = "https://github.com/cagridursun/devcade/releases/download/v$Version" }
$BaseUrl = $BaseUrl.TrimEnd('/', '\')
if ($BaseUrl -match '^[A-Za-z][A-Za-z0-9+.-]*://') {
    if ($BaseUrl -notmatch '^https://') {
        Fail "refusing base URL '$BaseUrl': only https:// URLs or a local directory are allowed"
    }
    $Mode = 'url'
} else {
    if (-not (Test-Path -LiteralPath $BaseUrl -PathType Container)) { Fail "base directory not found: $BaseUrl" }
    $Mode = 'dir'
}

if (-not $InstallDir) {
    if (-not $env:LOCALAPPDATA) { Fail 'LOCALAPPDATA is not set; pass -InstallDir' }
    $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\devcade'
}
$Archive = "devcade_${Version}_windows_${Arch}.zip"
$Dest = Join-Path $InstallDir 'devcade.exe'

Assert-Destination $Dest

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("devcade-install-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $Tmp | Out-Null

function Get-ReleaseFile([string]$Name) {
    $out = Join-Path $Tmp $Name
    if ($Mode -eq 'dir') {
        $src = Join-Path $BaseUrl $Name
        if (-not (Test-Path -LiteralPath $src -PathType Leaf)) { Fail "download failed: $src does not exist" }
        Copy-Item -LiteralPath $src -Destination $out
        return $out
    }
    $url = "$BaseUrl/$Name"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $out
    } catch {
        Fail "download failed: $url ($($_.Exception.Message)); interrupted, missing, or not public: private repositories cannot be downloaded anonymously"
    }
    return $out
}

try {
    if ($Mode -eq 'url') {
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    }
    Say "installing DevCade $Version for windows/$Arch"
    $sumsPath = Get-ReleaseFile 'SHA256SUMS'
    $zipPath = Get-ReleaseFile $Archive

    # --- verify -------------------------------------------------------------
    $found = @()
    foreach ($line in [System.IO.File]::ReadAllLines($sumsPath)) {
        if ($line -match '^([0-9A-Fa-f]{64}) [ *](.+)$' -and $Matches[2] -ceq $Archive) {
            $found += $Matches[1].ToLowerInvariant()
        }
    }
    if ($found.Count -ne 1) { Fail "SHA256SUMS has $($found.Count) entries for $Archive (expected exactly 1)" }
    $expected = $found[0]
    $actual = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        Fail "checksum mismatch for $Archive (expected $expected, got $actual); the download is corrupt or was tampered with; nothing was installed"
    }
    Say "verified SHA-256 $actual"

    # --- extract and install (only after verification) ------------------------
    $x = Join-Path $Tmp 'x'
    try {
        Expand-Archive -LiteralPath $zipPath -DestinationPath $x
    } catch {
        Fail "cannot extract ${Archive}: $($_.Exception.Message)"
    }
    $bin = Join-Path $x 'devcade.exe'
    if (-not (Test-Path -LiteralPath $bin -PathType Leaf)) { Fail "$Archive does not contain devcade.exe" }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Assert-Destination $Dest
    $staged = Join-Path $InstallDir ".devcade.install.$PID.exe"
    try {
        Copy-Item -LiteralPath $bin -Destination $staged
        Move-Item -LiteralPath $staged -Destination $Dest -Force
    } catch {
        Remove-Item -LiteralPath $staged -Force -ErrorAction SilentlyContinue
        Fail "cannot install to ${Dest}: $($_.Exception.Message) (is devcade running?)"
    }
    foreach ($doc in 'README.md', 'LICENSE-NOTICE.txt') {
        $src = Join-Path $x $doc
        if (Test-Path -LiteralPath $src -PathType Leaf) {
            Copy-Item -LiteralPath $src -Destination (Join-Path $InstallDir $doc) -Force
        }
    }
    Say "installed $Dest"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $onPath = $false
    foreach ($p in (("$userPath;$env:Path") -split ';')) {
        if ($p -and ($p.TrimEnd('\') -ieq $InstallDir.TrimEnd('\'))) { $onPath = $true }
    }
    if ($onPath) {
        Say 'run: devcade'
    } else {
        Say "$InstallDir is not on your PATH. To add it for your user, run:"
        Say "  [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ';$InstallDir', 'User')"
        Say 'then open a new terminal.'
    }
} catch {
    Fail $_.Exception.Message
} finally {
    Remove-Item -LiteralPath $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}
