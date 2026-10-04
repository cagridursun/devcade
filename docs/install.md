# Installing DevCade

> **Install and play without Go or a source checkout.**
> [v1.0.0-rc.2](https://github.com/cagridursun/devcade/releases/tag/v1.0.0-rc.2)
> is available as a self-contained binary for Windows, macOS and Linux.

## Recommended: Homebrew (macOS and Linux)

If Homebrew is not installed, follow its [official installation guide](https://brew.sh).
Then run:

```sh
brew install cagridursun/devcade/devcade
devcade
```

Alternatively, add the [DevCade tap](https://github.com/cagridursun/homebrew-devcade)
once to use the short package name:

```sh
brew tap cagridursun/devcade
brew install devcade
devcade
```

Homebrew selects the native macOS/Linux archive for Intel or ARM64, verifies
its SHA-256 and puts the binary on PATH. The first form automatically selects
our tap; a fresh Homebrew installation does not know bare `devcade` until the
tap is added. The game does not require Go or a source build.

Update or remove it with:

```sh
brew update
brew upgrade devcade
# To remove the game:
brew uninstall devcade
```

## Recommended: Scoop (Windows)

Use a normal PowerShell window. If Scoop is not installed, follow its
[official installation guide](https://scoop.sh). Scoop needs Git to manage
custom buckets; if Git is missing, install it with `scoop install git`.
You never clone or build the DevCade source repository yourself.

Add the [DevCade bucket](https://github.com/cagridursun/scoop-devcade) once:

```powershell
scoop bucket add devcade https://github.com/cagridursun/scoop-devcade
scoop install devcade
devcade
```

Use `scoop install devcade/devcade` to choose our bucket explicitly if another
bucket has a package with the same name. Scoop downloads the native x64 or
ARM64 ZIP, verifies its SHA-256 and creates the `devcade` command on PATH.
No Go installation or administrator privileges are required for the game.

Update or remove it with:

```powershell
scoop update
scoop update devcade
# To remove the game:
scoop uninstall devcade
```

Both managers install the published **1.0.0-rc.2** candidate. Open a terminal
of at least **80 × 24**; on Windows, Windows Terminal is recommended.
The managers download prebuilt binaries; the remaining sections are optional
alternatives for users who prefer direct downloads or source builds.

## Alternative installation methods

In the commands below, use `1.0.0-rc.2` for the current release candidate, or replace it with a later release version. Release
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
v=1.0.0-rc.2
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
$v = "1.0.0-rc.2"; $arch = "amd64"   # or arm64
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
v=1.0.0-rc.2
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
$v = "1.0.0-rc.2"
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

## Source checkout, build and development

Requires Go **1.26** or newer (`go.mod` declares `go 1.26.0`; CI uses the
latest 1.26.x) and an interactive terminal of at least **80 × 24**.

Install Git and Go 1.26 or newer, then clone the source:

```sh
git clone https://github.com/cagridursun/devcade.git
cd devcade
# Optional: use the exact source of the published candidate.
git checkout v1.0.0-rc.2
```

Run from the repository root:

```sh
go run ./cmd/devcade                  # open the arcade menu
go run ./cmd/devcade snake            # open the game submenu: snake, blockdrop, mazechase, blastgrid
go run ./cmd/devcade list             # list the games (no terminal needed)
go run ./cmd/devcade --diagnostic     # start the terminal diagnostic directly
go run ./cmd/devcade --help
go run ./cmd/devcade --version
```

`--help`, `--version` and `list` never open the fullscreen view or touch the
console, so they work in pipes and scripts. Unknown IDs, extra arguments and
combinations such as `--diagnostic snake` are usage errors (exit status 2),
reported before the terminal is touched.

Build a binary (cgo is not required):

```sh
CGO_ENABLED=0 go build -o bin/devcade ./cmd/devcade        # macOS / Linux
```

```powershell
$env:CGO_ENABLED = "0"; go build -o bin\devcade.exe ./cmd/devcade   # Windows PowerShell
```

Release archives for all six targets:

```sh
go run ./tools/release -version 1.0.0-rc.2 -out dist/release
```

Checks (the same ones CI runs):

```sh
go mod verify
go mod tidy -diff          # fails if go.mod/go.sum are out of date; edits nothing
gofmt -l .                 # must print nothing
go vet ./...
go test -timeout 60s ./...
go test -race -timeout 120s ./...   # needs cgo and a C compiler (gcc/clang)
```

Dependencies are locked in `go.mod` and `go.sum`, which are both committed.
CI fails rather than repairing them.

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

## Uninstalling direct downloads and installer installations

For Homebrew and Scoop, use the uninstall commands above. For direct downloads
or installer scripts, delete the binary: `rm ~/.local/bin/devcade` on macOS/Linux, or the
`%LOCALAPPDATA%\Programs\devcade` folder on Windows (and its `PATH` entry if
you added one). Removing the binary leaves preferences, personal bests and
the online identity intact; see [profile storage](settings.md#storage).

## First launch and rankings

The first run asks for a username; Esc continues as guest. Pick a game, then
New game or Leaderboard. Settings (O) offers five languages, three palettes
and opt-in global score sharing. English and black/white are the defaults.
Personal bests are saved on the device; global rankings need the shared HTTPS
service embedded by the release build. See [settings](settings.md) and
[leaderboard deployment](leaderboard.md). Uninstalling the binary leaves the
profile under the OS configuration directory; back it up to retain identity.
