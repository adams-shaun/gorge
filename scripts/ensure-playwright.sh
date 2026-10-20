#!/usr/bin/env bash
# ensure-playwright.sh — install the Playwright Chromium that both the smoke
# gate and web's browser-backed Vitest suite launch.
#
# The browser is not part of the repo or web/node_modules: Playwright
# downloads it to a cache. The harness points XDG_CACHE_HOME at a per-run
# temp dir, so an unqualified launch can look in a fresh empty cache every
# run. Pin a stable shared cache and install the pinned revision there.
# `playwright install chromium` is an idempotent ~0.2 s no-op when present.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
: "${PLAYWRIGHT_BROWSERS_PATH:=$HOME/.cache/ms-playwright}"
export PLAYWRIGHT_BROWSERS_PATH

if [ ! -x "$REPO_ROOT/web/node_modules/.bin/playwright" ]; then
  echo "ensure-playwright: web/node_modules/.bin/playwright is missing; provision web/node_modules (never npm ci through a symlink)" >&2
  exit 1
fi

echo "ensure-playwright: ensuring Chromium in $PLAYWRIGHT_BROWSERS_PATH" >&2
( cd "$REPO_ROOT/web" && ./node_modules/.bin/playwright install chromium )
