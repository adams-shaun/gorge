# Report — stat:CountersRemain

Implemented `S:Mode$ CountersRemain` for the two corpus carriers. The worktree was rebased onto `main` before implementation (it reported up to date), and `.cards` was already present. Measured prevalence: 2 files, `Me, the Immortal` and `Skullbriar, the Walking Grave`.

## What changed

- `rules/statics.go`: registered `stat:CountersRemain`; added `countersRemainApplies`, using the canonical active-static walk and `ValidCard$` filter (missing filters fail closed).
- `rules/engine.go`: tags a final, replacement-adjusted battlefield departure when its own active static matches, except moves to hand/library.
- `events/actions.go`, `events/apply.go`: carries the preservation marker in the existing MoveZone `Counter` payload while retaining any existing payload; replay decodes it and preserves counters during the Move fold. Ordinary moves and moves to hand/library still clear counters. No event kind/field or encoding changed.
- `rules/counters_remain_test.go`: real-corpus test on Me, the Immortal; asserts static/preconditions, adds counters, moves to exile and back to the battlefield, then verifies a hand move clears them.

## Gates and measurements

Corpus check:

```text
$ grep -rlE '^S:Mode\\$ CountersRemain' .cards/cardsfolder | sort
.cards/cardsfolder/m/me_the_immortal.txt
.cards/cardsfolder/s/skullbriar_the_walking_grave.txt
$ grep -rlE '^S:Mode\\$ CountersRemain' .cards/cardsfolder | wc -l
2
```

Targeted test:

```text
$ go test -run 'TestCountersRemainPreservesCountersExceptHandAndLibrary$' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.593s
```

The test was also run before the final guard-only refinement:

```text
$ go test -run 'TestCountersRemainPreservesCountersExceptHandAndLibrary$' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.595s
```

`make report` (first run rebuilt the stale IR cache):

```text
CGO_ENABLED=0 go build -o bin/forgec ./cmd/forgec
bin/forgec report -dir .cards
forgec: IR cache unusable (IR cache version 3, want 5 — run `make compile-cards`); compiling fresh from cardsfolder
corpus: 95f04e8a04c8925fa97cb226fc3341cabcc90a53 @ 95f04e8a04c8925fa97cb226fc3341cabcc90a53 (GPL-3.0, 33669 files)
cards: 33667  playable: 29738 (88.3%)
tokens: 839
```
The primitive is registered and report completed against the full corpus.

Required behaviour goldens:

```text
$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  3.574s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  1.333s
```

Formatting/type check and whitespace:

```text
$ gofmt -l events/actions.go events/apply.go rules/engine.go rules/statics.go rules/counters_remain_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output, exit 0)
$ git diff --check
(no output, exit 0)
```

## Fails without the fix

Copied `rules/engine.go` to `.ds4/scratch/`, removed the event-tagging hunk, and ran the targeted test. It failed on the actual exile move. Restored the file and verified it byte-identically with `cmp`.

```text
$ go test -run 'TestCountersRemainPreservesCountersExceptHandAndLibrary$' ./rules/
--- FAIL: TestCountersRemainPreservesCountersExceptHandAndLibrary (0.58s)
    counters_remain_test.go:28: P1P1 counters after battlefield-to-exile move = 0, want 2
FAIL
FAIL  github.com/adams-shaun/gorge/rules  0.593s
FAIL
RESTORED_BYTE_IDENTICAL
```

## Issues

No additional unfixed defects found in this scope. No AGENTS.md approximation row was present for this primitive to delete. No CR-lane test was added: the requested behaviour is directly pinned by the real-carrier engine test.

Commit: `c1b64881 feat(rules): preserve counters for CountersRemain statics`
