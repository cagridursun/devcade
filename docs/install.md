# Installing DevCade

> **Status: nothing is published yet.** The release tooling, archives,
> checksums, Homebrew formula, Scoop manifest and installer scripts described
> here are prepared and tested locally, but no GitHub release, tag, Homebrew
> tap or Scoop bucket exists. The repository is private, so even after files
> are attached to a release, anonymous downloads (browser, `curl`, the
> installer scripts, Homebrew, Scoop) only work once the owner makes the
> release downloadable publicly. Until then, build from source (see the
> [README](../README.md)) or use a release-candidate workflow artifact.

In the commands below, use `1.0.0-rc.1` for the first release candidate, or replace it with a later release version. Release
files live at:

```
https://github.com/cagridursun/devcade/releases/download/v<version>/<file>
```

## Release files

Every release has six archives, one per platform, plus `SHA256SUMS` and the
two installer scripts:

| Platform | Archive |
| --- | --- |
| macOS, Apple silicon | `devcade_<version>_darwin_arm64.tar.gz` |
| macOS, Intel | `devcade_<version>_darwin_amd64.tar.gz` |
| Linux x86-64 | `devcade_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `devcade_<version>_linux_arm64.tar.gz` |
| Windows x64 | `devcade_<version>_windows_amd64.zip` |
| Windows ARM64 | `devcade_<version>_windows_arm64.zip` |

Each archive holds three files at its top level (no enclosing folder): the
`devcade` binary (`devcade.exe` on Windows; mode 0755 in the tar archives),
`README.md`, and `LICENSE-NOTICE.txt`. The binaries are self-contained: no Go,
browser or other runtime is needed.

`SHA256SUMS` uses the standard `sha256sum` format
(`<sha256><two spaces><file name>`) and covers the six archives and the two
installer scripts.

**License:** MIT. `LICENSE-NOTICE.txt` includes the project license and the
full license texts for dependencies bundled in the binaries.

## Direct download

Always verify the checksum before extracting or running anything.

### macOS and Linux

```sh
v=1.0.0-rc.1
os=linux      # or darwin
arch=amd64    # or arm64 (Apple silicon, aarch64)
base=https://github.com/cagridursun/devcade/releases/download/v$v
curl -fLO "$base/devcade_${v}_${os}_${arch}.tar.gz"
curl -fLO "$base/SHA256SUMS"

# Linux:
sha256sum --ignore-missing -c SHA256SUMS
# macOS (or Linux without GNU coreutils):
grep " devcade_${v}_${os}_${arch}.tar.gz\$" SHA256SUMS | shasum -a 256 -c

mkdir devcade && tar -xzf "devcade_${v}_${os}_${arch}.tar.gz" -C devcade
mkdir -p ~/.local/bin && mv devcade/devcade ~/.local/bin/
devcade --version
```

The check must print `OK` for the archive; stop if it reports a mismatch.
Add `~/.local/bin` to `PATH` if it is not there yet.

### Windows (PowerShell)

```powershell
$v = "1.0.0-rc.1"; $arch = "amd64"   # or arm64
$base = "https://github.com/cagridursun/devcade/releases/download/v$v"
$zip = "devcade_${v}_windows_$arch.zip"
Invoke-WebRequest -UseBasicParsing "$base/$zip" -OutFile $zip
Invoke-WebRequest -UseBasicParsing "$base/SHA256SUMS" -OutFile SHA256SUMS

$expected = ((Select-String -Path SHA256SUMS -Pattern " $([regex]::Escape($zip))$").Line -split ' ')[0]
$actual = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
if ($actual -ne $expected) { throw "checksum mismatch for $zip" }

Expand-Archive $zip -DestinationPath devcade
.\devcade\devcade.exe --version
```

Use Windows Terminal (or another console that supports VT sequences, Windows
10 1809 or newer). Git Bash's mintty window is not a Windows console; use
`winpty devcade` there.

## Installer scripts

Both scripts download the platform's archive and `SHA256SUMS`, verify the
SHA-256 **before** extracting anything, never execute what they download,
need no administrator rights or `sudo`, and refuse to replace an existing
file that is not a DevCade binary (pass `--force` / `-Force` to override).
They do not change `PATH`; they print how to do it.

Options and environment variables:

| `install.sh` | `install.ps1` | Environment | Meaning |
| --- | --- | --- | --- |
| `--version X` | `-Version X` | `DEVCADE_VERSION` | Release to install (required) |
| `--base-url U` | `-BaseUrl U` | `DEVCADE_BASE_URL` | Where the release files are. Default `https://github.com/cagridursun/devcade/releases/download/v<version>`. `https://` or a local directory; `install.sh` also accepts `file://`. Plain `http://` is refused |
| `--install-dir D` | `-InstallDir D` | `DEVCADE_INSTALL_DIR` | Default `~/.local/bin` / `%LOCALAPPDATA%\Programs\devcade` |
| `--os`, `--arch` | `-Arch` | | Override detection (`darwin`/`linux`, `amd64`/`arm64`) |
| `--force` | `-Force` | | Replace an existing file even if it is not DevCade |

Download the script, read it, check it against `SHA256SUMS`, then run it.

### macOS and Linux: `install.sh`

```sh
v=1.0.0-rc.1
base=https://github.com/cagridursun/devcade/releases/download/v$v
curl -fLO "$base/install.sh" && curl -fLO "$base/SHA256SUMS"
sha256sum --ignore-missing -c SHA256SUMS     # macOS: grep ' install.sh$' SHA256SUMS | shasum -a 256 -c
sh install.sh --version "$v"
```

It detects the OS with `uname -s` and the CPU with `uname -m`
(`x86_64`/`amd64` and `arm64`/`aarch64`; an x86-64 shell running under
Rosetta 2 gets the native Apple silicon build). Other systems and CPUs stop
with an "unsupported" error. It needs `curl` (or `wget` for https), `tar`,
`gzip`, and `sha256sum`, `shasum` or `openssl`.

### Windows: `install.ps1`

```powershell
$v = "1.0.0-rc.1"
$base = "https://github.com/cagridursun/devcade/releases/download/v$v"
Invoke-WebRequest -UseBasicParsing "$base/install.ps1" -OutFile install.ps1
Invoke-WebRequest -UseBasicParsing "$base/SHA256SUMS" -OutFile SHA256SUMS
$line = @(Get-Content SHA256SUMS | Where-Object { $_ -match '^[0-9a-f]{64}  install\.ps1$' })
if ($line.Count -ne 1) { throw "Missing or duplicate installer checksum" }
if ((Get-FileHash install.ps1 -Algorithm SHA256).Hash.ToLowerInvariant() -ne $line[0].Substring(0,64)) { throw "Installer checksum mismatch" }
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Version $v
```

Run it with `-File` as shown (not through `Invoke-Expression`). It works in
Windows PowerShell 5.1 and PowerShell 7, installs `devcade.exe`, `README.md`
and `LICENSE-NOTICE.txt` into `%LOCALAPPDATA%\Programs\devcade`, and supports
x64 and ARM64 Windows.

## Homebrew (macOS and Linux)

The release tooling generates a formula, `homebrew/devcade.rb`, with the
download URL and SHA-256 of each macOS/Linux archive filled in from
`SHA256SUMS`. **No tap exists yet.**

- `brew install cagridursun/devcade/devcade` will work only after the owner
  creates a public tap repository named `cagridursun/homebrew-devcade`,
  commits the generated `Formula/devcade.rb` there, and publishes a release
  whose archives can be downloaded anonymously.
- Bare `brew install devcade` would additionally require the formula to be
  accepted into Homebrew core, which has its own requirements (including an
  open-source license). That has not been requested.

## Scoop (Windows)

The release tooling generates a Scoop manifest, `scoop/devcade.json`
(`64bit` and `arm64`, with hashes from `SHA256SUMS`). **No bucket exists
yet.** Once the owner publishes the release and a bucket repository (for
example `cagridursun/scoop-devcade` containing `bucket/devcade.json`), the
commands would be:

```powershell
scoop bucket add devcade https://github.com/cagridursun/scoop-devcade
scoop install devcade/devcade
```

The generated manifest declares the MIT project license.

## Unsigned binaries

The binaries are **not code-signed and not notarized**.

- **macOS:** files downloaded with a web browser get the quarantine attribute,
  and Gatekeeper then refuses to run an unnotarized binary ("cannot be
  opened" / "could not verify"). After verifying the checksum, either remove
  the attribute with `xattr -d com.apple.quarantine ./devcade`, or try to run
  it once and then choose **Open Anyway** in *System Settings > Privacy &
  Security*. Files fetched with `curl` (including by `install.sh`) are not
  quarantined. The Apple silicon binary carries only the ad-hoc signature the
  Go linker adds, which is not a Developer ID signature.
- **Windows:** a downloaded `.zip` and the files extracted from it can carry
  the "downloaded from the internet" mark. SmartScreen or Microsoft Defender
  may warn about an unrecognized, unsigned program, and unsigned Go binaries
  are occasionally flagged by antivirus heuristics. After verifying the
  checksum you can choose **More info > Run anyway**, or clear the mark with
  `Unblock-File .\devcade.exe`. Do not disable SmartScreen or Defender.
- **Linux:** no signing is involved; verify the checksum.

## Uninstalling

Delete the binary: `rm ~/.local/bin/devcade` on macOS/Linux, or the
`%LOCALAPPDATA%\Programs\devcade` folder on Windows (and its `PATH` entry if
you added one). DevCade stores no settings or save files.

## First launch and rankings

The first run asks for a username; Esc continues as guest. Pick a game, then
New game or Leaderboard. Settings (O) offers five languages, three palettes
and opt-in global score sharing. English and black/white are the defaults.
Personal bests are saved on the device; global rankings need the shared HTTPS
service embedded by the release build. See [settings](settings.md) and
[leaderboard deployment](leaderboard.md). Uninstalling the binary leaves the
profile under the OS configuration directory; back it up to retain identity.
