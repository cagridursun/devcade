# Releasing DevCade

This describes how release-candidate artifacts are built and checked. Nothing
here publishes anything: creating a tag, a GitHub release, a Homebrew tap or a
Scoop bucket are separate owner decisions (listed at the end).

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
5. Build the reference artifacts in CI: run the **Release candidate**
   workflow (`.github/workflows/release.yml`, *Actions > Release candidate >
   Run workflow*) with the version. It tests the tooling and installers,
   builds on `ubuntu-latest`, runs `sha256sum --strict -c`, smoke-tests the
   linux/amd64 binary, installs it with `install.sh` from the built files,
   and uploads `dist/release/` as the workflow artifact
   `devcade-<version>` (kept 30 days). It has `contents: read` only, uses no
   secrets, and creates no tag or release.
6. Compare the workflow artifact's `SHA256SUMS` with the local one. They
   should match when both used the same Go patch release; if not, use the
   workflow artifact and note the difference.
7. Do manual checks on real terminals (see `docs/terminal-checklist.md`).

Publishing is **not** part of these steps. When the owner decides to publish
(see below), the files to attach to the GitHub release `v<version>` are the
six archives, `SHA256SUMS`, `install.sh` and `install.ps1` from the same
build. The formula and manifest go to their own repositories, not to the
release.

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

## Outstanding owner decisions

These need the owner; the tooling does not do any of them.

1. **License.** None is chosen, so archives ship `LICENSE-NOTICE.txt` saying
   no license is granted, the formula has no `license` line and the Scoop
   manifest says `Unknown`. "Non-commercial" is an intent, not a license.
   Homebrew core in particular requires an accepted open-source license.
   After choosing one: add `LICENSE`, update `packaging/LICENSE-NOTICE.txt`
   (or ship the license file in the archives), and set `license` in both
   templates.
2. **Repository visibility / download access.** The repository is private,
   so release assets cannot be downloaded anonymously: the installers,
   Homebrew and Scoop will all fail with download errors until the release
   assets are publicly reachable.
3. **Tag and GitHub release.** Whether and when to create tag `v<version>`
   and publish a (pre-)release with the artifacts listed above.
4. **Homebrew tap.** Whether to create `cagridursun/homebrew-devcade` and
   commit `Formula/devcade.rb` from the generated `homebrew/devcade.rb`
   (enables `brew install cagridursun/devcade/devcade`). Submitting to
   Homebrew core is a further, separate decision.
5. **Scoop bucket.** Whether to create a bucket repository (for example
   `cagridursun/scoop-devcade`) with the generated `scoop/devcade.json`, or
   to submit to an existing bucket. winget is not prepared.
6. **Signing and notarization.** Binaries are unsigned. macOS notarization
   needs an Apple Developer ID; Windows Authenticode needs a code-signing
   certificate. Both need secrets in CI and changes to the build (signing must
   happen before archiving and checksumming).
