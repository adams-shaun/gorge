# Report — Verify and pin: Teapot Slinger / Convoke ManaExpend trigger

Task `agent-20260923T113045Z-aa7f7a4e`. Verification-only brief.

## Round t2 note (findings-t2)

Round t1 reported DONE but left the report uncommitted. I verified the
uncommitted artifact (`.ds4/report-t1.md`) against the brief, confirmed its
claims against the current tree by re-running every brief-named gate, then
committed it (staging only that specific path). Nothing in the brief's scope
was found wrong; the production tree and the existing regression test are
unmodified. A stale unrelated report (`report-t2.md`, the player-count
sacrifice task) had been copied into this worktree; this file replaces it as
the durable report for THIS task.

## Outcome

The report's premise is mistaken and the behaviour is already covered at
current main. **No production change, no new test, no code change of any
kind.** The existing real-corpus regression test
`rules/manaexpend_convoke_test.go::TestTeapotSlingerManaExpendCountsConvoke`
already asserts exactly what the brief asks for: after the Convoke payment
the matched trigger is drained onto the stack, the trigger sits on top, and
resolving it takes the opponent 20 → 18.

The brief's "`e.pendingTriggers` is empty" observation is not a missed
trigger: the completed `Submit` drives `Advance`, which drains pending
triggers before returning, so an empty `pendingTriggers` at that boundary is
by design. The test documents this explicitly in its own comment.

## Workspace facts verification

- `.cards` present as a symlink to `/home/sadams/projects/gorge/.cards`
  (found at worktree creation, not created by me) — so the run below is
  corpus-backed, not skipped. The ~0.6 s runtime confirms real execution
  (a skipped corpus run comes in ~ms).
- Corpus cards exist and are real (spot-checked):
  `.cards/cardsfolder/t/teapot_slinger.txt` (573 B),
  `.cards/cardsfolder/c/crowds_favor.txt` (498 B).
- `e7f775f6` is present and is the last commit touching the test file; it
  changed only `rules/manaexpend_convoke_test.go` (18+/8−). Production
  (`b6d47f8b` count, `5390d8b3` carrier-independent) predates it, as briefed.
- Matcher `manaExpendMatches` (`rules/trigmatch_cast.go` ~806–847) reads the
  pay-time `FlagManaExpendCast` and the per-turn crossing `prev = total -
  ev.Amount` against threshold `n`; dispatch is synchronous via `emit`.
  Matches the brief's map.
- The brief's stated line range (13–100) for the test held.

## Gate commands and real output

### Targeted test (Done means #1)

Fresh, uncached run on the committed tree:

```text
$ go test -count=1 -run '^TestTeapotSlingerManaExpendCountsConvoke$' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.637s
```

Cached confirmation earlier in the round also `ok ... 0.597s`. Runtime in the
corpus-backed range, so the test genuinely executed.

### archtest (Done means #3, no allowlist edits)

```text
$ go test ./internal/archtest/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)
```

No allowlist file was touched.

### botbench pinned split (Done means #4)

```text
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)
```

No behaviour change was made, so no split re-pin was needed.

## Non-vacuity evidence for the existing assertion

The brief says not to claim a production-fix-revert failure. I confirmed the
test's own resolution assertion is load-bearing, so it is not a vacuous green.
In `.ds4/scratch/` I copied the test, weakened ONLY the final resolution
assertion (`18` → `20`, i.e. pretend no damage is expected), ran the test, and
restored the file byte-identically:

```text
$ cmp .ds4/scratch/manaexp_orig.go rules/manaexpend_convoke_test.go
RESTORED-IDENTICAL

$ go test -run '^TestTeapotSlingerManaExpendCountsConvoke$' ./rules/   # weakened assertion
--- FAIL: TestTeapotSlingerManaExpendCountsConvoke (0.68s)
    manaexpend_convoke_test.go:96: Convoke expend-4 trigger left opponent at 18 life, want 20
FAIL	github.com/adams-shaun/gorge/rules	0.695s
FAIL
```

The message "left opponent at 18 life" is the measured real post-resolution
life, proving the trigger genuinely resolved and dealt 2 damage — the
assertion cannot pass with the trigger stuck on the stack or never drained.
After restore, the targeted test is green again.

The other preconditions in the test are asserted as required by the dispatch
contract: Teapot Slinger on the battlefield, pool genuinely straddles the
threshold (`got != 4 || got == 3`), the Convoke creature was really tapped,
the pay-time wake event carries exactly 1 mana, and opponent life is 20
before resolution. Values compared genuinely differ (3 prior pool mana vs.
threshold 4).

## Fails without the fix

No new test and no production fix is proposed for this verification-only
task. Current main already contains the regression assertions from
`e7f775f6`; the former matcher-only check did not fail on the described
behaviour — it simply did not verify queue draining and resolution. Per the
brief, I do not claim a production-fix-revert failure. The non-vacuity
evidence above shows the existing resolution assertion itself fails when it
is weakened, so it is a meaningful regression guard. The targeted test passes
on the unmodified tree.

## Deviations from the brief

- The brief's "Done means" implies a new test with its own reverted-fix
  failure output, but its own `## Fails without the fix` section explicitly
  forbids claiming one for this verification-only task. I followed the
  brief's explicit instruction: no new test, and the non-vacuity evidence
  stands in place of a fix-revert failure.
- Round t1's only uncommitted artifact was `.ds4/report-t1.md`. `.ds4` is
  listed in `.gitignore`, so the initial `git add .ds4/report-t1.md` was
  rejected by git's ignored-path advice; the file is nonetheless already
  tracked at HEAD (tracked files are unaffected by the ignore rule), so
  staging it directly succeeded. Committed as `f7374f16`.

## Issues

None found. The implementation and its end-to-end regression test are sound
and already in place. The reported symptom ("`pendingTriggers` empty after
payment therefore trigger missed") is a misreading of the driven boundary —
the empty field is the correct post-drain state, and the stack assertion in
the test is the right place to check the trigger reached resolution.

Out-of-scope observation (not a defect in this task, not filed): the brief
references `.ds4/report-t2.md` as the reviewer's path, while the dispatch's
own instruction also names it. A stale report from an unrelated task was
present there; I overwrote it. No CR-lane test is warranted by this round.
