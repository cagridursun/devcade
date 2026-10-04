# DevCade Homebrew tap

Install the terminal arcade on macOS or Linux. Homebrew downloads the published
binary, checks its SHA-256 and provides the devcade command; no Go or source
checkout is required.

```sh
brew install cagridursun/devcade/devcade
devcade
```

Or add the tap once and use the short name:

```sh
brew tap cagridursun/devcade
brew install devcade
```

Update: `brew update && brew upgrade devcade`.
Remove: `brew uninstall devcade`. Player settings and scores are retained.

Current package: 1.0.0-rc.1. Supports macOS/Linux on Intel and ARM64.
Use an interactive terminal of at least 80 x 24.

[Game repository](https://github.com/cagridursun/devcade) ·
[Installation guide](https://github.com/cagridursun/devcade/blob/main/docs/install.md)

Maintainers: copy Formula/devcade.rb from the exact published release workflow
artifact (homebrew/devcade.rb). Never reuse hashes from a local rebuild.
Binary install checks run on macOS and Linux for each push or pull request.
