#!/usr/bin/env bash
# ci-retry-cache.sh LABEL [--] COMMAND...: run a CI step's command, and retry it once when the only reason it
# failed is a Go build-cache entry that vanished mid-build.
#
# A self-hosted box's runners share one GOCACHE (GOFLAGS -trimpath, so the cache keys carry no workspace path).
# Whenever anything trims that cache while a build is running — a disk-pressure trim on the box, a fleet job,
# Go's own trim — a go command can resolve an entry and then find it gone. That is not a defect in the tree:
# 11-28 run 46154534694's fast job failed with 33 lines like
#   ##[error]vet: draft/gorgedevents/e2e_bo3_test.go:10:2: could not import strconv
#   (open /home/ghrunner/.cache/go-build/2a/2af2c27b...-d: no such file or directory)
# and the same tree passed on the retry. Re-running the same command with the same inputs is the fix; the retry
# never changes what is checked and a genuine failure is never retried (see the signature below).
#
# Only the exact transient class is retried: a line naming a <64-hex> cache entry under go-build/ AND "no such
# file or directory" on the same line. A missing source package, an undefined name, a type error or a failing
# test carries no such path, so those fail on the first attempt exactly as before, and the second attempt (when
# it happens) runs every test again with the same flags and exits with a genuine failure just the same.
#
# CI_CACHE_ATTEMPTS (default 2) bounds the attempts; anything else is reset to 2. Each attempt's output is
# streamed as before (through tee) and kept only for the failure scan.
set -uo pipefail
label=${1:-step}
if [ "$#" -lt 2 ] || [ "${label#-}" != "$label" ]; then
  echo "usage: ci-retry-cache.sh LABEL [--] COMMAND..." >&2
  exit 2
fi
shift
[ "${1:-}" = -- ] && shift
if [ "$#" -eq 0 ]; then
  echo "ci-retry-cache.sh: no command after $label" >&2
  exit 2
fi
attempts=${CI_CACHE_ATTEMPTS:-2}
case $attempts in '' | *[!0-9]* | 0) attempts=2 ;; esac
log=$(mktemp "${TMPDIR:-/tmp}/ci-retry-cache.XXXXXX") || exit 1
trap 'rm -f "$log"' EXIT
rc=1
i=0
while [ "$i" -lt "$attempts" ]; do
  i=$((i + 1))
  : >"$log"
  # PIPESTATUS, not pipefail's rc: tee's status must not mask the command's.
  "$@" 2>&1 | tee "$log"
  rc=${PIPESTATUS[0]}
  [ "$rc" -eq 0 ] && exit 0
  if [ "$i" -lt "$attempts" ] &&
    grep -Eq 'go-build/[0-9a-f]{2}/[0-9a-f]{64}[A-Za-z0-9._/-]*:.*no such file or directory' "$log"; then
    echo "ci-retry-cache: $label: a Go build-cache entry vanished mid-build (a shared-box trim); attempt $i/$attempts failed on it, retrying" >&2
    sleep 2
    continue
  fi
  break
done
exit "$rc"
