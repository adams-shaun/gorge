# Report — r2 (agent-20260919T192133Z-f7463cbe) — rebase resolution

Ticket: `K:Retrace — the cast-from-graveyard keyword is unimplemented (17 files)`.
**The implementation was already on main when this round started** (commit
`133495da feat(rules): implement kw:Retrace graveyard cast with land discard`,
merged by `fbbe4cb4`), and round t1 in this worktree added the deck-card
regression test (now `7de1a857 test(rules): cover Formless Genesis retrace`,
was `07232a2c` before the rebase). The full t1 report is preserved at
`.ds4/report-t1-retrace.md` (committed this round; previously left uncommitted
at the shared path, which is what blocked the rebase). The prior content of
`.ds4/report-r2.md` (the TriggerRemembered r2 report) is preserved at
`.ds4/report-r2-triggerremembered.md`.

## What this round did

`findings-r2.md` reported that the rebase onto main failed:

```
error: cannot rebase: You have unstaged changes.
--- merge fallback ---
error: Your local changes to the following files would be overwritten by merge:
	.ds4/report-t1.md
```

Root cause: round t1 wrote its report at the SHARED path `.ds4/report-t1.md`
and left it uncommitted; main tracks that path with a different ticket's
report. Same resolution as commits `5558278d` / `83d640d5`:

1. Moved this ticket's t1 report to the unique path
   `.ds4/report-t1-retrace.md` and restored `.ds4/report-t1.md` to its
   tracked content (`git restore <path>` — no branch switch, no shared state
   change).
2. Committed the moved report (`9785e5ba docs(retrace): record the retrace
   t1 report at a unique path`).
3. `git rebase main` — **clean, no conflicts**. The branch is now main
   (`e54228a0`) + exactly two commits:
   - `7de1a857 test(rules): cover Formless Genesis retrace`
     (rules/retrace_formless_test.go +54)
   - `9785e5ba docs(retrace): record the retrace t1 report at a unique path`
4. Re-verified everything after the rebase (output below).

## Brief coverage (all "Done means" items hold)

- **Formless Genesis graveyard cast with land discard** — covered by
  `rules/retrace_formless_test.go` (rebased content unchanged, re-verified):
  asserts the card is in the graveyard and still has Retrace (precondition),
  a land in hand, a `cast`/`retrace` option offered, the `KChoose` discard ask
  offers exactly the hand land, the discard sends it to the graveyard, and the
  spell reaches the stack.
- **Implementation** — rule-side (`rules/legal.go`/`rules/cast.go` per the t1
  report; commit `133495da` on main), not a `cards/keywords.go` expansion:
  Retrace is a casting option, the same family the keywords.go doc assigns to
  the rules side. The fixed additional discard cost has no card-authored
  parameter for the paramcensus to measure. The keyword registers in the
  coverage ratchet.
- **Measured corpus prevalence**: 17 `K:Retrace` files at the pin — matches
  the brief's claim.

## Gate commands and real output (all post-rebase)

`.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards` —
this was a real (not skipped) corpus run.

```
$ go build ./... && go test -run 'Retrace' ./rules/ > .ds4/scratch/retrace.log 2>&1; tail -5 .ds4/scratch/retrace.log
ok  	github.com/adams-shaun/gorge/rules	0.644s
```

Behaviour goldens:

```
$ go test ./internal/archtest/ 2>&1 | tail -2
ok  	github.com/adams-shaun/gorge/internal/archtest	3.330s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -2
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.299s
```

Format / generated types:

```
$ gofmt -l rules/retrace_formless_test.go
[no output]

$ go run ./cmd/gentypes -check
[exit 0]
```

## Fails without the fix

Recorded in the preserved t1 report (`.ds4/report-t1-retrace.md`): with the
Retrace graveyard-offer loop temporarily removed from `rules/legal.go`, the
test fails with `Formless Genesis Retrace cast not offered` (full decision
list pasted there); the source was restored byte-identically (`cmp` exit 0).

## Issues

None found and left unfixed. No chain-head or acceptance-ratchet movement is
expected or observed (no engine behaviour change in this branch; the test only
covers the already-landed primitive).


---

# Report — r2 (agent-20260919T185907Z-f5c7e2dc) — trig:Attacks.NoResolvingCheck on Sentinel Sarah Lyons

Round 2 of the ticket. Round 1's work was complete and green (`test(rules):
cover Sentinel Sarah Lyons battalion trigger`, then report appended to
`.ds4/report-t1.md`); the round was parked ONLY on the controller's rebase
directive failing (`error: cannot rebase: You have unstaged changes` and a
merge fallback conflicting on `.ds4/report-t1.md`). No review findings were
attached beyond that (`findings-r2.md` holds only the rebase error), so this
round did the rebase and re-verified everything after it.

## What changed this round

- Committed the uncommitted `.ds4/report-t1.md` round-1 report, then ran
  `git rebase main` — **clean, no conflicts**. Branch is now
  `126a5a95` on top of main (`git log main..HEAD` = exactly the two
  round-1/round-2 commits; diff vs main is `rules/battalion_test.go` + the
  report, nothing else).
- Re-verified the round-1 state against post-rebase main: the production
  `NoResolvingCheck$` read (`noResolvingCheck` +
  `triggerResolvingCheckHolds` in `rules/trigger_condition.go`, applied at
  the single resolution-time CR 603.4 site in `rules/stack.go`) survived the
  merge intact, and main has since landed its own companion tests
  (`rules/no_resolving_check_test.go`, Ugin's Mastery) plus a retired
  `knownUnsupportedParams` row (Love on the Battlefield) for the sibling
  ticket. My branch's contribution remains the brief's ask: the **real-corpus
  card test** for Sentinel Sarah Lyons (Battalion: IsPresent$
  `Creature.attacking+Other` GE2 + `NoResolvingCheck$ True`).
- Brief premises re-measured, both held: `grep -rlE 'NoResolvingCheck$'
  .cards/cardsfolder | wc -l` = 87 files / 88 lines; Sentinel Sarah Lyons is
  NOT in `internal/testutil/decks/`, so there is no `knownUnsupportedParams`
  row to delete for it.

## Gates run (real output)

`.cards` was PRESENT (symlink), so this is an executed run, not a skipped one.

Targeted test (`rules/battalion_test.go`), after the rebase:

```text
$ go test -run '^TestSentinelSarahLyonsBattalionSurvivesAttackerLeaving$' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.609s
```

Behaviour goldens:

```text
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.650s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.320s
```

## Fails without the fix

The test pins the production bypass, so I neutralised the bypass
(`triggerResolvingCheckHolds`'s `noResolvingCheck` early-return in
`rules/trigger_condition.go`, saved to `.ds4/scratch/` first) and re-ran the
one test:

```text
--- FAIL: TestSentinelSarahLyonsBattalionSurvivesAttackerLeaving (0.60s)
    battalion_test.go:76: Sentinel Sarah Lyons trigger did not deal damage; it left the stack with "fizzled: intervening-if no longer holds"
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.616s
```

Then restored the file byte-identically (`cmp` OK) and the test passed again.

## Head/ratchet movement

None attributable to this ticket: no production code changed on this branch
(the param read predates it and landed on main via
`param:trig:AttackersDeclared.NoResolvingCheck`), no deck import, no census or
heads change. `TestConstructedDefaultIsByteIdentical` unchanged.

## Issues

None new. Round 1's report (`.ds4/report-t1.md`, tail) already records the
notes: the corpus-side `NoResolvingCheck$` population is entirely `True`, and
the remaining exposure (if any) is cards whose `IsPresent$`/`PresentCompare$`
clause is NOT paired with `NoResolvingCheck$` and therefore SHOULD re-check at
resolution — that path is already the shared default, so no gap.
# Report — r2 (agent-20260918T233200Z-f7c5b4f1) — pred:hasABasicLandType

Ticket: `pred:hasABasicLandType` — the "land card with a basic land type"
filter predicate is unknown (fails closed). **Reconciled fix round.** The
task's code and tests were already committed as `1505bd31` (now rebased to
`fe9c7646`); the r2 findings named only a failed `git rebase main` caused by
an uncommitted, destructive overwrite of the shared `.ds4/report-t1.md`.
This round: the overwrite was dropped (text salvaged to scratch), the rebase
was completed cleanly, and the report is this insertion at the top of
`.ds4/report-r2.md` — insertions only, zero deletions.

## Resolution of the r2 findings, each one

### [rebase-failed] "cannot rebase: You have unstaged changes … would be
overwritten by merge: .ds4/report-t1.md" — RESOLVED

The unstaged change was this ticket's own report text written over the
shared accumulate-file: it replaced 1,948 lines of other tickets' committed
reports with its 208 lines (the same mistake the sibling ticket
`79b69706` made in its t1 and fixed in its r2). Resolution, in order:

1. Salvaged the report text to `.ds4/scratch/report-t1-hasbasiclandtype.md`
   (untracked scratch, out of the review path).
2. `git restore .ds4/report-t1.md` — the destructive overwrite is gone;
   `cmp` against `HEAD`'s blob confirms byte-identical restoration.
3. `git rebase main` — applied **cleanly, no conflicts** (the branch's only
   commit, the pred work, does not textually collide with main's
   `sharesCreatureTypeWith` work in `effects/filter.go`; both hunks are in
   the rebased file, verified by grep). Branch is now `fe9c7646` on top of
   main `f8e330c3`.
4. `git diff --stat main HEAD` reads exactly the task's three files,
   223 insertions / 0 deletions.

The full r1 report text (gates, fail-proof, structural notes) is preserved
verbatim below the separator at the bottom of this r2 section.

## Post-rebase verification (everything re-run on the rebased tree, because
main had moved under `effects/filter.go`, `rules/cumulative.go`,
`rules/paramcensus_test.go`)

`.cards` present (symlink → `/home/sadams/projects/gorge/.cards`), so these
runs are real, not vacuous.

```
$ go build ./...            (no output, exit 0)
$ go test -run 'TestHasABasicLandTypePredicate$' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.626s
$ go test -run 'TestSproutingGoblin' -v ./rules/
=== RUN   TestSproutingGoblinKickedETBSearchesBasicLandTypedLand
--- PASS: TestSproutingGoblinKickedETBSearchesBasicLandTypedLand (0.61s)
=== RUN   TestSproutingGoblinUnkickedETBSearchesNothing
--- PASS: TestSproutingGoblinUnkickedETBSearchesNothing (0.01s)
ok  	github.com/adams-shaun/gorge/rules	0.630s
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.927s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.277s
$ gofmt -l effects/filter.go effects/hasbasiclandtype_test.go rules/hasbasiclandtype_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output, exit 0)
```

The 20-game bot split did not move, so no re-pin was needed.

## Fails without the fix — re-proven on the REBASED tree

Removed only the `case "hasABasicLandType"` classifier hunk
(`.ds4/scratch/r2-filter.go.orig` is the scratch copy), ran both tests,
then restored byte-identically (`cmp` clean):

```
$ go test -run 'TestSproutingGoblinKickedETBSearchesBasicLandTypedLand|TestHasABasicLandTypePredicate' ./rules/ ./effects/
--- FAIL: TestSproutingGoblinKickedETBSearchesBasicLandTypedLand (0.74s)
    hasbasiclandtype_test.go:48: pending = … Kind:choose Prompt:turn 2 — discard …}, want a search KChoose for the kicked ETB
FAIL	github.com/adams-shaun/gorge/rules	0.772s
--- FAIL: TestHasABasicLandTypePredicate (0.74s)
    hasbasiclandtype_test.go:40: Land.hasABasicLandType must match a Forest (a basic land type)
    hasbasiclandtype_test.go:43: Land.hasABasicLandType must match a Plains (a basic land type)
    hasbasiclandtype_test.go:66: UnknownPredicates(Land.hasABasicLandType) = [hasABasicLandType], want empty
    hasbasiclandtype_test.go:72: UnknownPredicates of a mixed spec = [hasABasicLandType totallyNotAPredicate], want exactly the unknown token
FAIL	github.com/adams-shaun/gorge/effects	0.760s
RESTORED byte-identical (cmp clean)
```

## Ratchets / head movement (unchanged from r1)

- `knownUnsupported`: Sprouting Goblin is not in the repo deck set and not in
  the table — no row moved in either direction.
- No new trigger mode, count head or param registration; chain heads not run
  (daemon gate); the botbench byte-identical run shows no repo-deck game
  behaviour moved.
- `knownApproximationRows` unchanged; no AGENTS.md row added or grown.

## Deviations from the brief (from r1, unchanged)

1. CR-correct semantics without Wastes: the brief's parenthetical ("or the
   supertype Basic plus a land-type word", "/Wastes") was wrong — Wastes is a
   basic land with NO basic land type (CR 205.3i) and must not match; the
   predicate is exactly the five basic land-type words, taken from the
   existing `chooseBasicLandTypes` so it cannot drift from the
   `Type$ Basic Land` choose.
2. An extra effects-package leaf test beyond the brief's named carrier test,
   because the brief's structural requirement (matcher and
   `UnknownPredicates` share one recogniser) is only provable at the
   classifier leaf.

## Issues

- `hasANonBasicLandType` still unknown/fail-closed (Wonderscape Sage,
  1 corpus file) — already filed as its own ticket
  (`deck-gap-hasanonbasiclandtype.md`, picked up by the orchestrator).
- Nothing else found that this ticket did not fix.

---

# Prior r2 report (pred:ChosenCardStrict, agent-20260918T233200Z-79b69706) — already merged to main; preserved verbatim below

