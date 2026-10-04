# Settings and player profile

Select **Settings** in the main menu, or press **O**. Up/Down chooses a row;
Left/Right or Enter changes its value. Q/Esc returns. Changes take effect
immediately and are saved for the next run.

| Setting | Choices | Default |
| --- | --- | --- |
| Language | English, Türkçe, Español, Nederlands, Français | English |
| Color palette | Black / white, Midnight, Colorful | Black / white |
| Username | 3–20 lowercase ASCII letters, digits or underscores | Guest until chosen |
| Global score sharing | Off / On | Off |

The language applies to menus, game HUDs, controls, pause/resize warnings and
end screens. Game names, key names, usernames and board glyphs stay stable.
Precomposed Latin letters such as `ş`, `ı`, `ñ`, `é` and `œ` occupy one cell;
wide, combining and control characters are still replaced with `?`.
Color remains decorative. `NO_COLOR` forces the monochrome palette even if
another palette is saved. Midnight uses a dark blue background; Colorful
distinguishes headers, players and warnings. Color capability varies by terminal.

The first interactive launch asks for a username. Enter saves it locally;
Backspace edits it and Esc continues as guest. Letters that normally move or
quit (`q`, `w`, `a`, `s`, `d`) are ordinary text on this screen. Ctrl+C always
exits, including on small screens. A guest can play and save local bests.
A username is an unverified alias, not a GitHub/X login. We do not inspect Git
configuration, invoke `gh` or read credentials to guess a player's identity.

## Navigation

`main menu → game → New game / Leaderboard → gameplay / rankings`

An ID such as `devcade snake` opens the same game submenu. Q/Esc during a game
returns to its submenu; another Q/Esc returns to the main menu. The direct
terminal diagnostic still exits on Q/Esc. Enter on an end screen restarts.
The main menu includes **New games coming soon**, the creator attribution and
a selectable **Open creator profile** item. That item launches
`https://x.com/c__dursun` through the OS browser launcher without invoking a
shell. If unavailable, the address remains visible to open manually.

## Storage

Source builds and official release packages use the community leaderboard at
`https://devcade.cinesdigital.com`. `DEVCADE_LEADERBOARD_URL` overrides that
address; setting it to an empty string disables network access for offline-only
play. Score sharing remains Off until enabled in Settings.

Preferences, the alias, per-game personal bests and the anonymous online
credential are stored in `devcade/profile.json` under the OS configuration
directory: `%APPDATA%` on Windows, `~/Library/Application Support` on macOS,
or `$XDG_CONFIG_HOME` / `~/.config` on Linux. `DEVCADE_CONFIG_DIR` overrides
the containing directory for development or portable setups.

Files are replaced through a synced temporary snapshot; a failed write is
shown in the UI. On POSIX the new file uses mode 0600. Keep the profile private
and back it up to retain the online identity; Windows access follows the
containing user's permissions. Malformed or unsupported profile files are
left untouched and the session falls back to defaults without saving over them.

Only completed wins/losses count. Quitting an unfinished run does not record
its score. A restart never clears or lowers a saved best. Losing the local
credential cannot reclaim an occupied online alias automatically. Changing
the username creates a new online identity; old published rows remain.
Turning sharing off stops future submissions; it does not remove old rows.
See [leaderboard deployment and API](leaderboard.md).
