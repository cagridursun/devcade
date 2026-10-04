# Terminal FC rollout notes

Terminal FC introduces the stable game ID `terminalfc` across the client,
profile validation, leaderboard API, usage analytics and the public website.

## Deployment order

1. Deploy leaderboard/metrics backend support that accepts `terminalfc` and
   enforces its 0–1750 score range.
2. Verify the backend endpoints with local/staging data.
3. Publish the static website/snapshot tooling that knows the new game ID.
4. Release the DevCade client containing Terminal FC.
5. Only after the client release, update package-manager distribution metadata
   through the normal release process.

A client with score sharing enabled must not be released before the production
leaderboard backend recognizes `terminalfc`; otherwise valid completed-match
submissions would be rejected.

No backend, website, release, Homebrew formula, Scoop manifest or production
data is deployed by this development branch.
