# Report — task agent-20260919T062939Z-4b5f8950 (round t2)

**Ticket:** `RepeatOptional$` on `DB$ Repeat` — the may-repeat election is never posed
**Round t2 job (findings-t2.md):** verify t1's uncommitted changes against the
brief and commit them. **Outcome:** the t1 report's substance is CONFIRMED; I
committed it and closed the one real gap against the brief's "Done means"
that t1 left open (the plain `DB$ Repeat` no-host fallback iteration count).

## What changed and why

### 1. `.ds4/report-t1.md` (verified, then committed — `84d276a8`)
t1's report concluded the brief's premise is FALSE: both halves of this ticket
were already implemented and merged to `main` before the seat was dispatched.
I re-verified every load-bearing claim independently (not trusting the report):

- **HEAD is an ancestor of `main`.** `git merge-base --is-ancestor HEAD main`
  → YES. `main..HEAD` is empty (no unique commits).
- **`effRepeat` reads `RepeatOptional$`.** `effects/misc.go:2882`
  `optional := strings.EqualFold(..., sa.Params["RepeatOptional"]), "True")`;
  `poseRepeatOptionalElection` (misc.go:3008) poses the `KChoose`
  "Repeat this process?" election. The brief's symptom ("reads only
  `MaxRepeat`/`RepeatNum`") is false at this HEAD.
- **The sibling spelling is read too.** `effects/choose_control.go:1565`
  reads `RepeatOptionalForEachPlayer$` for `DB$ RepeatEach`.
- **The landing commits exist on `main`:** `46928423`/`cli-20260922T150843Z-7fb23a6f`
  (the `DB$ Repeat` half) and `5fcdf7d0`/`agent-20260922T194522Z-d7f24b09`
  (the `RepeatEach`/`RepeatOptionalForEachPlayer$` half). This ticket
  `agent-20260919T062939Z-4b5f8950` is their ORIGINAL filing.

Committing this report destroys the unrelated `kw:Backup` report that previously
occupied `.ds4/report-t1.md` (it is a shared/reused report slot — `git log`
shows many tickets overwriting it: `c8b97fb0`, `9ed10179`, …). That is the
established convention here; the prior content is preserved in git history and
at `.ds4/scratch/report-t1-HEAD-backup.md`.

### 2. `effects/repeat_optional_no_host_test.go` (new — `58288c4e`)
t1 left one thing undone against the brief. The brief's "Done means" has TWO
conjuncts: a real corpus carrier poses the election with the answer bounding
the loop count, **and** the deterministic no-host fallback repeats the
documented count. t1 argued the second had no dedicated test. Re-measured, t1
was partly wrong and partly right:

- A no-host test for the plain spelling DOES exist:
  `effects/misc_test.go::TestAdNauseamRepeatOptionalUsesRealCorpusAbility` sets
  `h.askResult = false` on the real `Ad Nauseam` Repeat SA and asserts
  `h.askCount == 1` (the election was posed).
- But it does NOT assert the iteration count the brief names. Nothing pinned
  that the no-host path runs the body exactly ONCE and then stops — the
  documented R-9 shape (`poseRepeatOptionalElection` returns false →
  `effRepeat` returns after one pass; `effects/misc.go:2944,2971`).

I added that missing leaf, tied to the real corpus carrier and asserting both
halves so it cannot pass vacuously: the body ran (`run == 1`, not 0 — the
do/while body runs before the first election, CR 608.2c), it did not iterate
(`run == 1`, not 2+), the election was actually posed (`askCount == 1` with
`ResumeKind == "repeat_optional"` — proves the feature's handler ran rather
than the Repeat falling through unregistered), and the SA precondition
(`Ad Nauseam` still carries `RepeatOptional$ True` + a `RepeatSubAbility$`)
is asserted first.

**Structural approach chosen (Fix the class):** the new test binds to the
real corpus card via `testutil.CorpusRegistry`, not an invented SA, and asserts
the *count* (the loop bound), not merely "an ask happened". The `RepeatEach`
sibling's equivalent (`effects/repeat_each_optional_test.go:38`
`TestRepeatEachOptionalForEachPlayerNoAskDeclines`) asserts `ran == 0` for its
decline-every-subject fallback; the plain-spelling fallback's documented answer
is one pass then stop (`ran == 1`), which is now pinned symmetrically.

## Gate commands and real output

```
$ go test -run 'TestAdNauseamRepeatOptionalNoHostRunsOneIterationThenStops' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.610s            # corpus loaded (not a ~0.00s vacuous run)

$ go test -run 'TestAdNauseamOptionalRepeatElectionStopsOnNo|TestAdNauseamOptionalRepeatYesIterates|TestForbiddenRitualBodyAskResumesToRepeatElection|TestAdNauseamRepeatOptionalUsesRealCorpusAbility|TestAdNauseamRepeatOptionalNoHostRunsOneIterationThenStops|TestRepeatEachOptionalForEachPlayerNoAskDeclines' ./rules/ ./effects/
ok  	github.com/adams-shaun/gorge/rules	0.975s
ok  	github.com/adams-shaun/gorge/effects	0.859s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)

$ go run ./cmd/gentypes -check
(no output = pass)
$ gofmt -l effects/repeat_optional_no_host_test.go
(no output = formatted)
```

No head/ratchet movement: no engine behaviour changed (the feature was already
merged); the new test only pins existing behaviour. `TestHeads`,
`knownUnsupported`, `knownUnsupportedParams`, `knownUnmodifiableCountHeads`
are untouched.

`.cards` was **found present** as a symlink to
`/home/sadams/projects/gorge/.cards` (not created by me); the 0.610s/0.975s
durations confirm non-vacuous corpus runs.

## Fails without the fix

I simulated the pre-fix symptom in the real file (backed up to
`.ds4/scratch/misc.go.bak`, restored byte-identically, `cmp` → identical),
by making `effRepeat` ignore `RepeatOptional$`:

```
$ sed -i 's/optional := strings.EqualFold(...sa.Params["RepeatOptional"]...)/optional := false/' effects/misc.go
$ go test -run 'TestAdNauseamRepeatOptionalNoHostRunsOneIterationThenStops' ./effects/
--- FAIL: TestAdNauseamRepeatOptionalNoHostRunsOneIterationThenStops (0.59s)
    repeat_optional_no_host_test.go:74: no-host RepeatOptional posed 0 elections, want 1 (the first election is always offered)
FAIL
FAIL	github.com/adams-shaun/gorge/effects	0.601s
$ cmp effects/misc.go .ds4/scratch/misc.go.bak && echo RESTORED_IDENTICAL
RESTORED_IDENTICAL
```

The test fails when `RepeatOptional$` is unread (0 elections posed) and passes
with it read. The frame is restored byte-identically (`git diff --stat
effects/misc.go` is empty).

## Controller directive: rebase

The brief's injected directive (2026-09-23T03:20:03Z) says to run `git rebase
main`. I did **not** run it: `system-t2.md` lists `git rebase` among the
commands never to run in a seat, and the issue history records the controller
retracting this same rebase-first directive twice ("the daemon rebases at
landing; a dirty or conflicted rebase mid-round only produces BLOCKED"). The
branch is a clean ancestor of `main` with no unique commits, so landing is a
fast-forward. If the daemon nonetheless wants a rebase, it can do it at the
merge gate without conflict.

## Deviations from the brief

- I did not implement `RepeatOptional$` handling: it was already merged before
  dispatch. Writing it again would re-land merged code and conflict with
  `7fb23a6f`/`d7f24b09`. Reported, not silently skipped.
- I did not rebase (see above).

## Issues

- **Duplicate ledger entry (bookkeeping, not code).** Ticket
  `agent-20260919T062939Z-4b5f8950` is the original filing of the two
  already-MERGED entries `cli-20260922T150843Z-7fb23a6f` and
  `agent-20260922T194522Z-d7f24b09`, which between them implement reading
  `RepeatOptional$`/`RepeatOptionalForEachPlayer$` and posing the election.
  It should be closed as superseded/duplicate by the controller (I cannot edit
  `.ds4/ledger.json` — it is derived).
- **Corpus prevalence held.** `/usr/bin/grep -rl 'RepeatOptional'
  .cards/cardsfolder | wc -l` → **14**, matching the brief.
- **`Dance with Calamity` (the brief's named non-deck carrier).** Its
  `RepeatOptional$` election is now posed; its remaining gap is the separately
  ticketed `api:GenericChoice` driver (`agent-20260918T202223Z-eb7aab2a`),
  exactly as the brief itself notes. Not re-filed.
- **No new CR-lane test needed.** This is implemented behaviour, not an
  approximation; no ledger-invisible defect here.
