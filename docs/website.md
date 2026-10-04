# Project website

Public page: **https://cagridursun.github.io/devcade/**. The static site in
`site/` describes DevCade in English and Turkish, shows actual gameplay frames,
provides Homebrew/Scoop installation commands and links to the creator's
GitHub and X/Twitter profiles. English is the default; EN/TR switches the
whole page. `?lang=tr` is a shareable Turkish URL. The selection is stored
locally. Optional usage sharing defaults to Off: a visitor may enable it with
the explicit footer control. Only then are visits and successful installation
command copies sent with a random session ID. No account, browsing URL or
leaderboard identity is included. See [analytics.md](analytics.md).

## Scoreboard

On opening the page or pressing Refresh, the browser reads the four public
leaderboard endpoints directly. The server permits GET ranking reads from
`DEVCADE_SITE_ORIGIN` (default `https://cagridursun.github.io`); it does not
permit cross-origin registration, score writes or admin reads. No credentials
are sent. **Updating the deployed leaderboard container is required** for this
read-only CORS support; updating the website alone is not sufficient.

The `Project website` workflow still publishes `leaderboards.json` as a
fallback. It requests snapshots at minutes 7, 22, 37 and 52; GitHub may delay
scheduled runs. Successful live reads do not wait for that schedule. A failed
live read preserves the newest valid copy per game, including an in-memory
live board newer than the published file, with its original timestamp and an
explicit stale warning. The browser also flags timestamps older than 45 minutes.

An unavailable API preserves the previous valid board and its original
timestamp, explicitly marked stale. Without previous data, the page shows
unavailable instead of claiming there are no players. A successfully fetched
empty board displays the empty-state message. One failed game does not prevent
the other games from updating. If GitHub Pages itself is unreachable, site
deployment does not modify the standalone game or leaderboard service.

Only rank, username and best score are published; player IDs are omitted.
Score responses are bounded and validated. Usernames render through
`textContent`, never HTML. The site never registers a player or submits a score.
Sharing scores remains an explicit choice in the game's Settings.

## Publishing and maintenance

Repository **Settings → Pages → Source → GitHub Actions** is required once.
The workflow validates pull requests without deploying them. Site changes on
`main`, scheduled refreshes and manual workflow runs build the static artifact
and deploy it through the `github-pages` environment. Only `main` can deploy.
There is no npm dependency installation or application server.

Local verification, with Node 24:

```sh
node --test tools/pages/*.test.mjs
node tools/pages/build.mjs --offline
python3 -m http.server 8081 --directory dist/pages
```

Open `http://localhost:8081` to preview. The offline build deliberately shows
unavailable scores; it does not invent sample users or send API requests.
Omit `--offline` to read the real public service. `PAGES_PREVIOUS_URL` can
override the previous published snapshot URL for a future custom domain.
Screenshots are copied from `docs/screenshots/` into the published assets.
Update the release label on the page when package-manager versions change.

## Animated Snake preview

The hero plays a 24-second recording of real Snake `Game.Render` output on
an 80 × 24 canvas. `tools/pages/capture-snake` drives the existing Go game
through normal directional inputs, records its ASCII cells and colour roles,
and saves `site/assets/snake-demo.json`. The website replays those captured
frames at their original movement intervals; it does not implement a second
Snake engine or submit demonstration scores to the leaderboard. The font
and terminal colours are rendered by the browser, so they can differ from
the PNG captures and a user's terminal.

Pause/play is available in both page languages. Playback stops when the
preview leaves the viewport or the browser tab becomes hidden. A reduced
motion preference disables autoplay; visitors can explicitly start playback.
If the recording cannot load, the original screenshot remains visible.
The four game-gallery screenshots remain static.

To intentionally capture a new clip after changing the Snake game:

```sh
go run ./tools/pages/capture-snake > site/assets/snake-demo.json
node --test tools/pages/*.test.mjs
```

Initial food placement uses the real game's random source, so regeneration
produces a different valid run. Scheduled Pages refreshes reuse the committed
recording and do not need Go or any extra frontend dependencies.
