# Releasing DevCade

This describes how release artifacts are built, checked and optionally
published. Pull requests and the default manual workflow run only prepare
artifacts. Publishing requires an explicit `publish=true` manual run on main
in a public repository; all three native verification jobs must pass first.

## What the tooling produces

`tools/release` is a standard-library Go program. From the repository root:

```sh
go run ./tools/release -version 1.0.0-rc.1 -out dist/release
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `-version` | (required) | `MAJOR.MINOR.PATCH[-PRERELEASE]`, no leading `v`, no `+build` metadata |
| `-out` | `dist/release` | Output directory. Must be below `<root>/dist` (git-ignored); it is emptied first |
| `-root` | `.` | Repository root (must be the `github.com/cagridursun/devcade` module) |
| `-leaderboard-url` | (empty; keeps the source default) | Override the community HTTPS score API in all six clients; the publish workflow explicitly supplies its endpoint |
| `-base-url` | `https://github.com/cagridursun/devcade/releases/download/v{version}` | URL prefix written into the manifests; `{version}` is substituted; https only |
| `-manifests-only` | `false` | Skip building; regenerate the manifests from an existing `<out>/SHA256SUMS` |
| `-go` | `go` | Go command to build with |

Output in `dist/release/`:

```
devcade_<v>_darwin_amd64.tar.gz    devcade_<v>_linux_amd64.tar.gz    devcade_<v>_windows_amd64.zip
devcade_<v>_darwin_arm64.tar.gz    devcade_<v>_linux_arm64.tar.gz    devcade_<v>_windows_arm64.zip
install.sh  install.ps1            copies of packaging/install/*
SHA256SUMS                         sha256sum format, all of the above
homebrew/devcade.rb                generated from packaging/homebrew/devcade.rb.tmpl
scoop/devcade.json                 generated from packaging/scoop/devcade.json.tmpl
```

Each archive contains, sorted by name, `LICENSE-NOTICE.txt` (from
`packaging/LICENSE-NOTICE.txt`), `README.md` (the repository README) and the
binary (`devcade`, mode 0755, or `devcade.exe`), with no enclosing folder.

The manifests are rendered from `SHA256SUMS` as read back from disk. A missing
entry fails the run, so generated manifests never contain placeholder hashes.

### Build settings

Each target is built with:

```
CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> GOFLAGS=-mod=readonly GOAMD64=v1 GOARM64=v8.0 GOEXPERIMENT=
go build -trimpath -buildvcs=false -ldflags="-s -w -X main.version=<v>" -o ... ./cmd/devcade
```

These variables are pinned so a developer's environment cannot change a
release build.

### Determinism

For a given toolchain version and source tree, the run produces identical
bytes, and so identical `SHA256SUMS`:

- `-trimpath` removes local paths. `-buildvcs=false` keeps git state (commit,
  commit time and the "modified" flag) out of the binary, so the bytes depend
  only on the source files, the toolchain and the flags, not on whether the
  checkout is clean or is a git checkout at all. The commit is identified by
  the release tag instead. (`main.version` is the only injected value;
  `cmd/devcade` has no commit variable.)
- Archives: fixed entry order, fixed mtime (`SOURCE_DATE_EPOCH` if set,
  otherwise 2026-01-01T00:00:00Z), uid/gid 0 with empty owner names, ustar
  format, gzip header without name or timestamp, fixed compression levels,
  and zip entries with Unix modes and a UTC timestamp.
- Text files are normalized to LF, so a Windows checkout with CRLF line
  endings packs the same bytes.

Checked locally on Windows with Go 1.26.8: two runs, the second with an empty
`GOCACHE`, produced byte-identical `SHA256SUMS` and manifests. Identical
output on other host OSes is expected (Go builds are reproducible across
hosts) but has not been compared; use the release workflow's artifact as the
reference build.

## Release-candidate steps

1. Start from the commit to release, with CI green on it.
2. Pick the version, for example `1.0.0-rc.1`.
3. Run the local checks:

   ```sh
   gofmt -l .                 # prints nothing
   go vet ./...
   go test -count=1 ./...
   DEVCADE_RELEASE_E2E=1 go test -count=1 -run EndToEnd ./tools/release
   ```

   The last command cross-compiles all six targets into a temporary folder
   under `dist/`, checks every checksum, archive layout and mode, and runs the
   native binary's `--version`, `--help` and `list`.
4. Build the artifacts and verify them:

   ```sh
   go run ./tools/release -version 1.0.0-rc.1 -out dist/release
   (cd dist/release && sha256sum -c SHA256SUMS)
   tar -tvzf dist/release/devcade_1.0.0-rc.1_linux_amd64.tar.gz   # devcade is -rwxr-xr-x 0/0
   ```

   Unpack the archive for your platform and run `devcade --version` (must
   print `devcade 1.0.0-rc.1`), `devcade --help` and `devcade list`. Try the
   installer against the local files, for example
   `sh packaging/install/install.sh --version 1.0.0-rc.1 --base-url "$PWD/dist/release" --install-dir /tmp/devcade-bin`
   or `install.ps1 -Version 1.0.0-rc.1 -BaseUrl <full path to dist\release> -InstallDir <dir>`.
5. Open **Actions > Release candidate > Run workflow**. Select `main`,
   version `1.0.0-rc.1` and leave **publish unchecked** for a preparation run.
   Pull requests changing release files also run this preparation automatically.
6. The workflow pins Go 1.26.8, verifies source/modules, builds all six
   archives and uploads the `devcade-release` workflow artifact (30 days).
   Linux, macOS and Windows jobs download that same artifact, run native
   race tests, verify its checksums, install the native archive with the
   bundled script and run version/help/game-list/non-TTY smoke tests.
7. Complete or record manual terminal evidence; see the terminal checklist.
   The first release is a pre-release because full human emulator acceptance
   on macOS/Linux and all ARM64 targets is not complete.

## Publish the first release

The shared [leaderboard service](leaderboard.md) is deployed at
`https://devcade.cinesdigital.com`. The owner verified Windows public access,
real score submission and retention after server restart. The workflow defaults
to this address and refuses publication unless its health and four board
endpoints respond successfully. Offline play remains available, and an empty
`DEVCADE_LEADERBOARD_URL` disables network access at runtime.

1. Merge the release-preparation PR after both CI and Release candidate checks
   are green. No tag or release is created by merging.
2. Make `cagridursun/devcade` public using GitHub repository settings. Check
   that the README screenshots and source can be opened signed out.
3. Run **Release candidate** on `main`, version `1.0.0-rc.1`, with
   **publish checked** and `leaderboard_url` set to the shared service. This rebuilds and verifies all artifacts before the
   publication job can run, and smoke-tests the leaderboard Docker image.
4. The publish job refuses a private repository, a branch other than main,
   missing versioned release notes, an existing release or an existing tag.
   It creates `v1.0.0-rc.1` at the exact workflow commit and uploads the six
   archives, `SHA256SUMS`, `install.sh` and `install.ps1` from that verified
   build. A version containing `-` is published as a GitHub pre-release.
   Only this job has `contents: write`; preparation jobs have read access.
5. Open the release signed out, download the native archive and checksum file,
   and try a clean installation from the public URL. These anonymous network
   checks cannot be completed while the repository/release is private.

The formula and manifest remain in the workflow artifact under `homebrew/`
and `scoop/`. Tap/bucket creation and code signing are follow-up work.
A stable `1.0.0` needs its own `docs/releases/1.0.0.md` and a green publication
run; never replace the assets of an already published version.

If the release URL changes, regenerate only the manifests:

```sh
go run ./tools/release -version 1.0.0-rc.1 -out dist/release -manifests-only -base-url https://example.com/devcade/{version}
```

## Testing the tooling

`go test ./tools/...` covers version validation, archive names, archive
contents and modes, byte-identical rebuilds, the `SHA256SUMS` format and
parser, manifest generation (including failure on a missing checksum), and
the output-directory guard. It also runs the installers against local
fixture releases built in a temporary directory (no network):

- `install.sh` (needs `sh`, `tar`, `gzip`, `awk` and a SHA-256 tool; skipped
  with a message otherwise; on Windows it runs under Git Bash): success on
  Linux x86_64/aarch64 and macOS arm64 detection (a fake `uname` on `PATH`),
  `--os/--arch` overrides, `file://` URLs through curl, wrong checksum,
  truncated (interrupted) download, missing archive, missing `SHA256SUMS`,
  missing checksum entry, unsupported architecture and OS, a Windows shell,
  invalid version, refusing `http://`, refusing to overwrite a foreign file or
  a directory, `--force`, and upgrading an existing DevCade binary. Every case
  also checks that no temporary files are left behind.
- `install.ps1` (Windows only, via `powershell -NoProfile -ExecutionPolicy
  Bypass -File`; skipped elsewhere): detected and explicit architectures,
  wrong checksum, truncated download, missing archive, missing checksum
  entry, unsupported architecture, invalid version, refusing `http://`,
  refusing to overwrite a foreign file, `-Force`, and upgrading.

## Distribution status

- License: MIT; binary archives carry the project license and complete
  third-party license texts. Update `THIRD_PARTY_NOTICES.txt` and
  `packaging/LICENSE-NOTICE.txt` if dependencies change.
- The repository is public. [v1.0.0-rc.1](https://github.com/cagridursun/devcade/releases/tag/v1.0.0-rc.1)
  was published on 2026-10-04 from `251560129eca37657e99faf36201a6ab1eff9c1c`
  by [release run 37201955505](https://github.com/cagridursun/devcade/actions/runs/37201955505).
  Build, Docker/service checks, native verification on all three OSes and
  publication succeeded. Future publication runs need a new version and
  matching release notes; existing versions are never overwritten.
- Package-manager distribution uses `cagridursun/homebrew-devcade` and
  `cagridursun/scoop-devcade`. After each new release, update
  `Formula/devcade.rb` and `bucket/devcade.json` from that release workflow
  artifact. These manifests must use the published archives and their actual
  checksums, never a local rebuild. Each distribution repository runs native
  installation checks; wait for green checks before advertising the update.
  See [distribution bootstrap](../packaging/distribution/README.md).
- Signing/notarization remain optional follow-up work requiring the owner's
  certificates. No certificate or account is required for the unsigned RC.
