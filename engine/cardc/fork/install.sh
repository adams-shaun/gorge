#!/usr/bin/env bash
# install.sh: put regenerate.yml on the gorge branch of github.com/adams-shaun/mtgish (CC-33c step 1), then wait
# for its first run. The run regenerates data/mtgish.lines.json from the grammar and fails if it differs from the
# committed file, so a green first run proves the fork's data is reproducible before any patch commit lands.
#
# Run by the operator (it pushes to the fork). It only ever pushes the fork's own gorge branch, fast-forward.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
wf="$here/regenerate.yml"
[ -f "$wf" ] || { echo "install: no $wf" >&2; exit 2; }
work=$(mktemp -d -p "$HOME/.cache")
trap 'rm -rf "$work"' EXIT
gh repo clone adams-shaun/mtgish "$work/mtgish" -- --branch gorge --depth 20 -q
cd "$work/mtgish"
mkdir -p .github/workflows
if cmp -s "$wf" .github/workflows/regenerate.yml; then
	echo "install: regenerate.yml already current on gorge"
else
	cp "$wf" .github/workflows/regenerate.yml
	git add .github/workflows/regenerate.yml
	git -c user.email=spadams85@gmail.com -c user.name="Shaun Adams" commit -q \
		-m "ci: regenerate data/mtgish.lines.json from the grammar and check it matches the committed file"
	git push -q origin HEAD:refs/heads/gorge
	echo "install: pushed $(git rev-parse --short HEAD) to adams-shaun/mtgish gorge"
fi
head=$(git rev-parse HEAD)
echo "install: waiting for the regenerate run on $head"
id=""
for _ in $(seq 1 40); do
	id=$(gh run list -R adams-shaun/mtgish --workflow regenerate.yml --branch gorge --json databaseId,headSha \
		--jq "map(select(.headSha==\"$head\"))[0].databaseId // empty")
	[ -n "$id" ] && break
	sleep 15
done
if [ -z "$id" ]; then
	echo "install: no run started; start one: gh workflow run regenerate.yml -R adams-shaun/mtgish --ref gorge" >&2
	exit 1
fi
gh run watch "$id" -R adams-shaun/mtgish --exit-status >/dev/null && echo "install: regenerate GREEN (run $id): the fork's data reproduces" ||
	{ echo "install: regenerate RED (run $id): gh run view $id -R adams-shaun/mtgish --log-failed" >&2; exit 1; }
