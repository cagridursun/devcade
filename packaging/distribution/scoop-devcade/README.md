# DevCade Scoop bucket

Install the terminal arcade on Windows. Scoop downloads the published binary,
checks its SHA-256 and provides the devcade command; no Go or game source
checkout is required. Install Scoop first from https://scoop.sh; Git is needed
by Scoop to manage custom buckets (`scoop install git` if missing).

```powershell
scoop bucket add devcade https://github.com/cagridursun/scoop-devcade
scoop install devcade
devcade
```

Use `scoop install devcade/devcade` to select this bucket explicitly.
Update: `scoop update` then `scoop update devcade`.
Remove: `scoop uninstall devcade`. Player settings and scores are retained.

Current package: 1.0.0-rc.1. Supports x64 and ARM64 Windows.
Use Windows Terminal or another VT-compatible interactive console, at least 80 x 24.

[Game repository](https://github.com/cagridursun/devcade) ·
[Installation guide](https://github.com/cagridursun/devcade/blob/main/docs/install.md)

Maintainers: copy bucket/devcade.json from the exact published release workflow
artifact (scoop/devcade.json). Never reuse hashes from a local rebuild.
A real Scoop binary installation is checked on Windows for each push or pull request.
