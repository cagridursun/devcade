# Package-manager distribution bootstrap

These directories contain the initial contents of two separate public repositories:

- `homebrew-devcade/` -> `cagridursun/homebrew-devcade`
- `scoop-devcade/` -> `cagridursun/scoop-devcade`

Copy the contents of each directory to its repository root. Do not nest the
repository inside the directory again. The initial manifests target the
already published v1.0.0-rc.1 archives; their hashes were checked against the
release asset digests and SHA256SUMS from run 37201955505.

Publication checklist:

1. Create both repositories as public, with an initial main branch.
2. Publish the corresponding root files through GitHub's contents/Git APIs.
3. Wait for native install checks in both repositories: Homebrew on macOS/Linux
   and Scoop on Windows. These checks download the real released binaries.
4. Confirm the tap/bucket files are public, then merge the main-repository
   package-manager documentation PR. Until then, the PR is a publication draft.

For future releases, replace Formula/devcade.rb and bucket/devcade.json with
the generated manifests from that exact release workflow artifact. These
bootstrap copies are a snapshot of the first distribution, not the source
of truth for later releases. Do not rebuild published archives to obtain hashes.
No cross-repository write token is needed for manual publication via the connector.
