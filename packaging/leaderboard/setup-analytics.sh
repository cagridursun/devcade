#!/usr/bin/env bash
# Preserve the existing Compose environment and never print administrator secrets.
set -euo pipefail
cd -- "$(dirname -- "$0")"
umask 077
if [[ ! -f .env ]]; then
  echo 'Create .env with LEADERBOARD_HOST before enabling analytics.' >&2
  exit 1
fi
chmod 600 .env
if ! grep -Eq '^LEADERBOARD_HOST=.+$' .env; then
  echo 'Set LEADERBOARD_HOST in the existing .env first.' >&2
  exit 1
fi
if grep -q '^DEVCADE_ADMIN_PASSWORD=' .env; then
  echo 'Existing DEVCADE_ADMIN_PASSWORD preserved; check it is at least 16 characters.'
else
  command -v openssl >/dev/null
  admin_password="$(openssl rand -hex 32)"
  printf '\nDEVCADE_ADMIN_PASSWORD=%s\n' "$admin_password" >> .env
  unset admin_password
  echo 'Administrator password created in .env (permissions 600).'
fi
echo 'Next: rebuild the leaderboard service using Compose project devcade.'
