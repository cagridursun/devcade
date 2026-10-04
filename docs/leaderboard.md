# Global leaderboard service

The game keeps local personal bests offline. A global ranking requires **one
shared public HTTPS service**, operated separately from the release downloads.
The service implementation and deployment files ship in this repository;
they have not been deployed to a public host by this PR.

## Player behavior

Sharing defaults to Off. A player can view each game's global board without
publishing a name. Turning sharing On sends their chosen username to register
an anonymous identity; the server returns a random ID and private bearer
credential. The profile must be saved successfully before score submissions.
The server stores only a SHA-256 digest of the credential.

Each identity has one best per game. Higher completed scores replace the
previous best; equal/lower retries are idempotent. The board shows the top 20
(Up/Down scrolls) and the viewer's own rank even outside that range. Scores
sort descending, with username as the deterministic tie-breaker. The game
submenus also show the device's own saved best, including while offline.

Network operations run outside the terminal loop, with a three-second request
timeout and an eight-second operation budget. A failed connection keeps local
bests and any previously displayed board. Opening/refreshing a board, enabling
sharing or achieving another best retries saved bests. R refreshes a board.
A taken name prompts choosing another in Settings. No email/password or
GitHub OAuth is required. Aliases and client-reported scores are unverified:
this is a community leaderboard, not an anti-cheat or prize system.

## Local end-to-end run

```sh
go run ./cmd/devcade-leaderboard -listen 127.0.0.1:8080 -data data/leaderboard.json
```

In another terminal:

```sh
DEVCADE_LEADERBOARD_URL=http://127.0.0.1:8080 go run ./cmd/devcade
```

PowerShell equivalent: `$env:DEVCADE_LEADERBOARD_URL='http://127.0.0.1:8080'`.
HTTP is permitted only on loopback. External endpoints require HTTPS;
credential-bearing requests never follow redirects. An explicitly empty
environment variable disables a compiled-in endpoint.

## Public deployment on a Docker host

1. Choose a hostname and point its DNS A/AAAA records to the host. Open inbound
   TCP 80/443 (and UDP 443 if using HTTP/3).
2. From the repository root:

   ```sh
   export LEADERBOARD_HOST=scores.example.com   # replace with your hostname
   docker compose -f packaging/leaderboard/compose.yml up -d --build
   curl --fail https://$LEADERBOARD_HOST/healthz
   ```

   The Compose stack runs the Go server as a non-root user, mounts the `scores`
   persistent volume, and puts Caddy in front for automatic HTTPS. The backend
   port is not exposed on the host. The fixed Docker subnet is
   `172.31.100.0/24`; change it and the trusted proxy CIDR together if it clashes.
   Only Caddy's exact address may supply forwarded client IPs, so clients cannot
   bypass per-IP limits by forging a header. See Caddy's official
   [HTTPS requirements](https://caddyserver.com/docs/quick-starts/https) and
   [forwarded-header policy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy#defaults).
3. Back up `leaderboard.json` on the persistent volume and retain the Caddy
   volumes. Stop the server for a consistent restore. Do not use
   `docker compose down -v` when retaining scores. OS file locks prevent a
   second server writer and are released automatically after process death.
4. In **Release candidate**, supply `leaderboard_url=https://scores.example.com`
   and `publish=true`. The workflow embeds that address in all six binaries,
   checks service health, builds/smoke-tests the server container, runs the
   platform package checks and then publishes. A publication without the
   common endpoint is refused. Preparation/PR runs can build offline binaries.
5. Verify a public installed client from a separate machine, register two
   distinct usernames, and confirm the same shared ranking appears for both.

For a manual release build:

```sh
go run ./tools/release -version 1.0.0-rc.1 -out dist/release \
  -leaderboard-url https://scores.example.com
```

## API and operating limits

| Request | Behavior |
| --- | --- |
| `GET /healthz` | Health response |
| `POST /v1/players` with `{"username":"name"}` | Registers a unique alias; returns ID/token once; 409 if occupied |
| `PUT /v1/best` with `{"game":"snake","score":100}` and bearer token | Authenticated max-only update |
| `GET /v1/leaderboards/snake?player=<id>` | Top 20 and optional own row; the player ID is public, the token is private |

Accepted games are snake, blockdrop, mazechase and blastgrid. Bodies are
limited to 4 KiB and unknown/trailing fields are rejected. Score bounds reject
negative/overflow values, and Snake also enforces its 10-point increments and
full-board maximum. Requests are limited to 120/minute per client IP, including
10 registrations/minute; responses are 429 with Retry-After. In-memory IP
buckets are bounded and are not persisted. The service logs neither tokens
nor request bodies. Public rows expose only alias, public player ID and score.

This first version uses a serialized JSON snapshot, an OS writer lock and a
persistent volume for **one server instance**, capped at 5,000 identities with
four bests each. It is intentionally a small community deployment. Horizontal
replicas require migrating the storage layer to a database; do not share a
volume between replicas. Name moderation/removal and credential recovery are
operator tasks for this release. No analytics or client telemetry is added.

Native API tests cover multiple clients, per-game isolation, restart recovery,
max-only/concurrent updates, own rank outside top 20, validation, authentication,
redirect refusal, rate limits, trusted proxy headers, write rollback and writer
locks. Human public-network and deployed TLS acceptance remain launch checks.
