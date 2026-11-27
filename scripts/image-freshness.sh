#!/usr/bin/env bash
# image-freshness.sh <image> <max_days>: fail when <image> has no :YYYY-MM-DD (UTC) tag from the last
# <max_days> days.
#
# daily-image.yml tags every publish with the UTC build day. The script inspects <image>:<day> for today back
# to <max_days> days ago; the first tag found prints
#   image-freshness: newest dated tag <image>:<day>
# and exits 0. None prints
#   image-freshness: no dated tag of <image> in the last <max_days> days
# and exits 1. Test hooks: INSPECT (default docker buildx imagetools inspect) and NOW (epoch seconds).
set -euo pipefail

image=${1:?usage: image-freshness.sh <image> <max_days>}
max_days=${2:?usage: image-freshness.sh <image> <max_days>}
inspect=${INSPECT:-docker buildx imagetools inspect}
now=${NOW:-$(date -u +%s)}

for ((d = 0; d <= max_days; d++)); do
  tag="$image:$(date -u -d "@$((now - d * 86400))" +%F)"
  # $inspect is a command line (word splitting intended).
  # shellcheck disable=SC2086
  if $inspect "$tag" >/dev/null 2>&1; then
    echo "image-freshness: newest dated tag $tag"
    exit 0
  fi
done

echo "image-freshness: no dated tag of $image in the last $max_days days"
exit 1
