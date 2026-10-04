# Installation

The canonical installation guide lives in [docs/install.md](docs/install.md).

## macOS and Linux

Install the ready-to-play binary with Homebrew:

```sh
brew install cagridursun/devcade/devcade
devcade
```

To update:

```sh
brew update
brew upgrade cagridursun/devcade/devcade
devcade --version
```

## Windows

Install with Scoop:

```powershell
scoop bucket add devcade https://github.com/cagridursun/scoop-devcade
scoop install devcade
devcade
```

To update:

```powershell
scoop update
scoop update devcade
devcade --version
```

## Direct downloads

Prebuilt archives for macOS, Linux, and Windows on amd64 and arm64 are available from [GitHub Releases](https://github.com/cagridursun/devcade/releases).

The current release candidate is **v1.0.0-rc.2**. Verify downloaded archives against the published `SHA256SUMS` file.

## Requirements

- An interactive terminal of at least **80 × 24**
- No Go installation is required for release binaries
- Windows users should use Windows Terminal or another VT-capable console

For installers, checksums, source builds, upgrades, uninstalling, and platform-specific notes, see the full [installation guide](docs/install.md).
