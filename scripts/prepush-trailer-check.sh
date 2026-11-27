#!/bin/sh
# prepush-trailer-check.sh LOCAL_SHA [REMOTE_SHA] -- the pre-push hook's trailer scan.
#
# Prints the offending commits ("<abbrev> <subject>", at most 3) and exits 1 when a commit the push
# ADDS carries an AI-attribution trailer or an issue-tracker reference; prints nothing and exits 0
# otherwise. LOCAL_SHA is what is being pushed; REMOTE_SHA is accepted for the caller's convenience
# and deliberately unused (see below).
#
# "Adds" means reachable from LOCAL_SHA and from no origin remote ref. The obvious range
# $REMOTE_SHA..$LOCAL_SHA is WRONG for a rebase: it also holds every commit main gained since the old
# head, which the push does not add and cannot change, so one such commit anywhere in main's history
# makes every rebase push of every branch impossible. That is exactly what happened on 11-27: #5602
# and #5609 were rebased onto main, the range pulled in two already-merged commits that carry a
# Co-Authored-By line (629141dfc1, 9983533de5), the hook refused both pushes, and both tickets went
# human_needed. Scanning what origin lacks is the same predicate a new branch already used.
set -u
local_sha=${1:?usage: prepush-trailer-check.sh LOCAL_SHA [REMOTE_SHA]}
# --grep matches any line of the message, so this finds a trailer in the body while printing only the
# commit's "<abbrev> <subject>" -- with a bare `git log --format='%h %s%n%b' | grep` the report showed
# the trailer line alone and never said which commit to amend.
out=$(git log --format='%h %s' "$local_sha" --not --remotes=origin \
	--grep='^Co-Authored-By:' --grep='^Generated with .*Claude' --grep='^Ref:' --grep='[j]ira' |
	head -3)
if [ -n "$out" ]; then
	printf '%s\n' "$out"
	exit 1
fi
exit 0
