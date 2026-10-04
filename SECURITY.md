# Security Policy

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub's security advisory flow:

<https://github.com/cagridursun/devcade/security/advisories/new>

Do not publish vulnerability details in a public issue, pull request, discussion, or comment before a fix is available.

A useful report includes:

- the affected DevCade version
- the affected component
- the impact
- reproduction steps or a minimal proof of concept
- relevant OS or terminal details, when applicable

Do not include credentials, bearer tokens, admin passwords, or data belonging to another person.

## Scope

Security-sensitive areas include:

- leaderboard authentication and score submission
- persisted player identities and bearer credentials
- optional usage metrics and the admin statistics panel
- installer and release integrity
- terminal restoration and signal handling
- website-to-service CORS boundaries

## Supported versions

DevCade is currently in release-candidate status. Security fixes target the latest published release candidate and the current `main` branch.

## Disclosure

Please allow reasonable time for investigation and remediation before public disclosure.
