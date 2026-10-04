# Private usage statistics panel

This infrastructure adds a private operator dashboard at `/admin/` to the
existing leaderboard server. It never creates players or submits scores. It
starts empty and uses recorded events and GitHub's release download counters.
It is not enabled on a deployed VM merely by merging this code.

## What the numbers mean

| Metric | Definition |
| --- | --- |
| Package downloads | Sum of GitHub `download_count` for published DevCade macOS/Linux/Windows `.tar.gz` and `.zip` assets, including prereleases; sampled at server startup and hourly |
| DAU / WAU / MAU | Distinct consenting installation IDs with a game start or end today / over 7 / over 30 UTC calendar days, including today |
| Installations | Distinct consenting IDs with any native event over 30 UTC days |
| Returning installations | Consenting IDs observed on more than one UTC date in that window |
| Game sessions | Starts in the last 30 UTC days and their recorded outcomes: finished, left for menu, app closed, or no end received |
| Mean / median duration | Completed event pairs in that start cohort; gameplay update time, excluding pauses, resizing interruptions and discarded/capped frames |
| Site visits / command copies | Page visits and successful clipboard copies from visitors who explicitly enable website usage sharing |

A random installation ID is not a verified person. Reinstalls/profile deletion
can create a new ID, and shared profiles can represent several people. Default
opt-out and offline/network failures mean the panel cannot count every player.
Events are client-reported, not anti-cheat verified. Platform/version rows may
overlap if an installation uses several versions. Website and game identifiers
are separate; these totals are not a linked conversion funnel.

Downloads include repeat requests and automated checks. They cannot distinguish
Homebrew, Scoop and direct downloads or prove successful installation. Scripts
and checksum files are excluded. Deleted release assets can lower the current
cumulative total. The first sample can include historical downloads, but usage
events and download history before deployment cannot be reconstructed.

## Consent and data

Game **Settings → Usage statistics** defaults to Off, independently of global
score sharing. Enabling it must successfully persist consent and a separate
cryptographically random 128-bit installation ID. Events contain event/run
IDs, game, final score, outcome, gameplay duration, OS and client version; the
server assigns reception time and source. No username, leaderboard token,
hardware fingerprint, keystrokes or IP address is written to the event journal.
IP addresses are used only in transient server rate-limit buckets. The network
provider/reverse proxy may still see connection metadata.

Website sharing also defaults to Off. Its footer button explicitly enables
visits and successful installation-command copies. The choice is stored
locally, with a random session ID in sessionStorage. No page URL, referrer or
account identity is sent. Disabling cancels pending requests, clears the
session ID and prevents further events. Already accepted events remain stored.

The native client sends asynchronously through a bounded 64-event queue with a
two-second HTTP timeout. On quit it attempts a flush for up to one second.
It does not retry, replay offline events or delay gameplay to await a request.
Unsent events can be lost. Disabling sharing immediately cancels requests.
An explicitly empty `DEVCADE_METRICS_URL` disables the usage endpoint; this is
independent of `DEVCADE_LEADERBOARD_URL`. The default uses the same HTTPS host.

**The published v1.0.0-rc.1 binaries do not include this feature.** Publish a
new client release and update the distribution packages before expecting
installed players to see the setting or produce game events. The website
control also requires deploying the updated Pages artifact.

## Enable on the existing GCP VM

After this branch is merged, run from your VM SSH session:

```sh
cd /opt/devcade
git pull --ff-only origin main
cd packaging/leaderboard
bash setup-analytics.sh
sudo docker compose -p devcade -f compose.yml up -d --build leaderboard
sudo docker compose -p devcade -f compose.yml ps
curl --fail --show-error https://devcade.cinesdigital.com/healthz
curl --fail --show-error https://devcade.cinesdigital.com/v1/leaderboards/snake
```

The setup script preserves the existing `.env` and hostname, sets permissions
to 600, and generates an administrator password only when one is absent. It
does not print secrets. The existing score volume and Caddy service are retained;
do not use `down -v`. If `/opt/devcade` has local Git modifications, inspect
them and resolve the fast-forward refusal before continuing.

View your generated password privately on the VM (do not post this output):

```sh
sed -n 's/^DEVCADE_ADMIN_PASSWORD=//p' .env
```

Open **https://devcade.cinesdigital.com/admin/** in your browser. The login is
`admin` and that password. Basic authentication is safe only through HTTPS
(HTTP loopback is for local tests). HTML, CSS, JavaScript and the JSON endpoint
all require authentication; no credentials are embedded in the public website.
The panel is read-only. Browser Basic authentication can remain cached until
the browser session closes; it has no application logout button.

Without credentials `/admin/` should return 401 once enabled:

```sh
curl -I https://devcade.cinesdigital.com/admin/
```

The native and website endpoints exist only when a password is configured.
An empty password disables metrics and leaves the existing leaderboard
service operational; a nonempty password shorter than 16 characters prevents
startup. `DEVCADE_SITE_ORIGIN` defaults to `https://cagridursun.github.io`;
change it when moving the site to another origin. CORS permits only that
origin; native metrics requests must omit `Origin`. CORS is not authentication
and does not prove that an event was sent by an actual game installation.

## Storage and operations

The existing exclusive data-file lock covers the entire single server instance.
The existing `/data` volume contains:

- `leaderboard.json`: existing score snapshot, unchanged.
- `metrics.jsonl`: validated events, serialized appends and fsync before success.
- `downloads.json`: atomic download snapshots, at most 2,160 hourly samples.

Back up all three files with the service stopped for a consistent recovery.
The event journal is bounded at **50,000 events or 32 MiB**, whichever comes
first. Once full, new events return 503; recorded totals remain available.
The panel's 30-day window does not automatically delete journal entries.
Download history holds up to 90 days of hourly samples; repeated restarts
also add samples. For a growing community, migrate to a database with retention
and historical aggregates before hitting this prototype's cap.

For an intentional event-history reset, stop the service, archive
`metrics.jsonl` outside the active data directory and restart. Usage totals
will reset; downloads and leaderboard scores need not change. Do not edit
journals while the service runs. Malformed event/download files prevent
startup rather than silently inventing or discarding records; restore a valid
backup before restarting. Appending a record is durable but a power failure
during a filesystem write can leave a partial final line needing operator
recovery. Single-writer deployment only; do not share this volume among replicas.

Requests have a 4 KiB body limit, strict fields, server-controlled timestamps,
valid game/score bounds, idempotent event IDs and start/end pairing. A bounded
per-IP quota allows 120 metrics requests per minute. Configure the existing
trusted Caddy address together with its Docker subnet if changing networking.
Failed GitHub polls retain the previous sample and display a stale-refresh
message. No bots, fabricated counts or demonstration events are populated.

## Local checks

```sh
go test -race -timeout 120s ./...
go vet ./...
node --test tools/pages/*.test.mjs
node tools/pages/build.mjs --offline
```

Server/auth/CORS/rate-limit tests use isolated temporary stores and loopback
HTTP. Game consent tests never send to production. Tests cover restart recovery,
duplicate events, UTC windows, session pairing and disabled sharing. Local
preview does not count game demos as usage or submit leaderboard scores.
