# Contributing to DevCade

Bug reports, fixes, documentation improvements, terminal compatibility reports, new tests, and focused feature proposals are welcome.

Small, well-scoped pull requests are much easier to review than broad rewrites.

## Before you start

- Check the existing issues and pull requests before opening a duplicate.
- For a bug, include your OS, terminal emulator, architecture, DevCade version, and a minimal reproduction when possible.
- For a larger feature or a new game, open an issue first so the scope and v1 direction can be discussed.
- Do not commit credentials, leaderboard tokens, admin passwords, analytics data, or other secrets.

## Development setup

DevCade is written in Go. The CI workflow currently uses **Go 1.26.x**.

```sh
git clone https://github.com/cagridursun/devcade.git
cd devcade
go mod download
go test ./...
```

For the same core checks used by CI:

```sh
go mod verify
go mod tidy -diff
gofmt -l .
go vet ./...
go test ./...
```

On Linux, macOS, and Windows, CI also runs race tests and builds the terminal client.

## Running from source

```sh
go run ./cmd/devcade
```

DevCade requires an interactive terminal of at least **80 × 24**.

The global leaderboard is optional. Local development can run without it. See [docs/leaderboard.md](docs/leaderboard.md) for the local end-to-end setup.

## Adding or changing a game

Games live under `internal/games/<id>` and implement the engine contracts described in the main [README](README.md#architecture).

Please keep game packages independent from terminal, filesystem, and network concerns. If gameplay behavior changes, update or add deterministic tests and keep [docs/games.md](docs/games.md) accurate.

## Pull requests

Before opening a pull request:

1. Run the relevant tests.
2. Run `gofmt` on changed Go files.
3. Update documentation when behavior changes.
4. Keep the diff focused.
5. Explain what changed, why, and how you validated it.

## Security issues

Do not report an undisclosed vulnerability in a public issue. Follow [SECURITY.md](SECURITY.md) instead.

## Code of Conduct

Participation in this project is governed by [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
