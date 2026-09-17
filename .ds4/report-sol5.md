# Report — findings-sol5 fix round (dredge parked-draw resume defects)

## What changed and why

The review round (findings-sol5) proved two MAJORs in `resumeResolution`'s
parked-frame arm — both live only on the **stack path** (a resolving spell
parked on a GainLife→Draw replacement body's dredge ask), which the committed
direct-arm probes never exercise. Both fixes are in `rules/resolution.go`; one
commit, `184e5df`.

1. **Final parked answer dropped** (review finding 1, MAJOR). The
   `parkedDraws` gate was `rp.kind == "dredge" && rp.sa == nil &&
   rp.lifeDraws > 0`, so the body's FINAL draw — the one that asked,
   `lifeDraws == 0` — never entered the branch: neither `applyDredge` nor
   `resumeOrdinaryDraw` ran, and the flow fell into the misleading
   `"mid-resolution answer resumed with no sub-ability recorded"` Note (also
   review finding 3, MINOR — that Note is gone because the frame now routes
   through the parkedDraws branch, which emits nothing).
   **Fix:** the gate is now the exact parked-frame signature
   `rp.kind == "dredge" && rp.sa == nil` (verified exact: `DrawFor` is the
   only sa==nil dredge asker — `effDraw`'s frames carry `ResumeSA`, and a
   turn-based draw's direct frame never reaches `resumeResolution` because
   `handleModes` routes `resume.direct` answers to its own arm). The answer
   is applied unconditionally inside the branch, the way the direct arm in
   `handleModes` already does, then `lifeReplacementDraw(rp.player,
   rp.lifeDraws)` runs — a no-op at 0, exactly like the direct arm.

2. **Re-park dropped the continuation chain** (review finding 2, MAJOR). The
   re-park early return (`if e.Suspended() || e.pending != nil { return }`)
   left the re-drive's fresh pending point with `outer == nil` — `rp.outer`
   (built by resolveTop's fx32 linking when the walk first suspended) was
   never attached, so after the cascade completed, `finishResumption` finished
   the object without ever running the sub-ability after the GainLife (Kiss's
   `SubAbility$ DBDraw`).
   **Fix:** on the re-park return, `e.resume.outer = rp.outer` (guarded
   `e.resume != nil`; at this return the re-drive only ever poses a dredge
   ask, so `e.resume` is always the fresh frame and nothing in the re-drive
   reports continuations) — the same fx32 linking discipline the nested-ask
   branch below practises. Every later park re-links the same chain, so
   `rp.outer` survives until the cascade truly ends. The direct arm needs no
   change: its frames carry no outer (verified — a direct frame is built bare
   by `Ask` with an empty stack and its re-drive never links one).

3. **TEST_HISTORY consolidation** (review finding 4, MINOR). `rules/
   TEST_HISTORY.md` carried FOUR stacked `budget_s:` lines (300/124/53/51);
   `loadBudget` takes the first (300s) and testtime's append invariant
   expects exactly one. Consolidated to the one **effective** value
   (`budget_s: 300`), so the gate behaves byte-identically to how it did
   before the consolidation.

New suite defence (`rules/life_draw_dredge_stack_test.go`, 3 tests), the
stack-path siblings the review asked for — the review's exact probe on real
card scripts (Nefarious Lich on the battlefield, Kiss of the Amesha resolving
on the stack targeting its caster, Golgari Thug (Dredge 4) in the graveyard;
Kiss is `SP$ GainLife 7 | SubAbility$ DBDraw`, so the card's answer is
7+2=9 draws and Kiss in the graveyard):

- `TestLifeReplacementDredgeStackDeclinesAll` — decline every ask: 9 asks,
  9 draws, DBDraw ran, Kiss + Thug in the graveyard, no orphan Note, replay
  identical. Exercises BOTH fixes (the final frame is lifeDraws == 0 and
  every intermediate answer re-parks).
- `TestLifeReplacementDredgeStackAcceptsFinalAsk` — decline six, ACCEPT the
  seventh (the lifeDraws == 0 frame): mill 4, Thug back to hand, no Draw
  event for the replaced draw, then 2 ask-free DBDraw draws. Pins fix 1's
  accept arm.
- `TestLifeReplacementDredgeStackAcceptsFirstAsk` — accept the first ask (no
  re-park): mill 4, 1 ask, 8 draws, Thug in hand. The shape the review round
  already verified held; pinned so the ordinary suite defends it.

## Break attempt (tests vs the unfixed engine)

Restored `HEAD:rules/resolution.go` over the working file, ran the three new
tests, restored the fix:

```
=== RUN   TestLifeReplacementDredgeStackDeclinesAll
    life_draw_dredge_stack_test.go:212: dredge asks for 7 life draws + 2 DBDraw draws = 7, want 9
--- FAIL: TestLifeReplacementDredgeStackDeclinesAll (0.47s)
=== RUN   TestLifeReplacementDredgeStackAcceptsFinalAsk
    life_draw_dredge_stack_test.go:257: accepted final dredge milled 0 cards, want 4
--- FAIL: TestLifeReplacementDredgeStackAcceptsFinalAsk (0.46s)
=== RUN   TestLifeReplacementDredgeStackAcceptsFirstAsk
--- PASS: TestLifeReplacementDredgeStackAcceptsFirstAsk (0.48s)
```

Exactly the review's measured symptoms: 7 asks where the card says 9 (DBDraw
never ran — finding 2), mill 0 on the accepted final ask (finding 1), and the
no-re-park shape passing (isolating both defects to the re-park/zero-remainder
path, as the review found).

## Gate commands run (real output)

`.cards` is the real symlink (`ln -s /home/sadams/projects/gorge/.cards`,
present at session start) — no run below is vacuous.

- `go test ./rules/` (full suite, post-fix):
  `ok  github.com/adams-shaun/gorge/rules  142.045s`
- Focused acceptance + the dredge probes:
  ```
  --- PASS: TestEveryRepoDeckIsFullySupported (0.47s)
  --- PASS: TestRepoDecksPlayAtEverySeatCount (1.17s)
  --- PASS: TestRepoDeckGamesReplayExactly (1.09s)
  --- PASS: TestHeads (1.17s)
  --- PASS: TestLifeReplacementDredgeStackDeclinesAll (0.45s)
  --- PASS: TestLifeReplacementDredgeStackAcceptsFinalAsk (0.46s)
  --- PASS: TestLifeReplacementDredgeStackAcceptsFirstAsk (0.45s)
  --- PASS: TestLifeReplacementDredgeAsksOnceAndDrawsAll (0.46s)
  --- PASS: TestLifeReplacementDredgeAcceptAppliesThenContinues (0.46s)
  --- PASS: TestLifeReplacementDredgeReParksForEachDraw (0.46s)
  --- PASS: TestEveryRepoDeckParamsAreRead (0.95s)
  ok  github.com/adams-shaun/gorge/rules  7.604s
  ```
- Neighbouring packages:
  `ok effects 23.578s | ok view 2.017s | ok events | ok state | ok cards |
   ok decision | ok botpolicy | ok replay 10.969s`
- `make sim 2>&1 | grep -c 'replay OK'` → `20`
- `gofmt -l .` (no output) · `go vet ./rules/` (clean) ·
  `go run ./cmd/gentypes -check` (clean)
- Commit-time budget gate: `testtime: rules 137.9s 869 tests 1 skipped
  budget 300s` — under the effective 300s, **no Test-Budget-Approved trailer
  needed** (3 tests added, measured 137.9s vs the first-line budget that was
  already in force; no raise claimed).

## Head / ratchet movement

None. `TestHeads` and `TestEveryRepoDeckIsFullySupported` both pass unchanged
(the fix touches only the suspended-resume path; no golden game reaches a
parked dredge cascade). Nothing in `heads_test.go` or the ratchet was edited.

## Deviations from the findings' prescriptions

- Finding 2's fix is written as `if e.resume != nil { e.resume.outer =
  rp.outer }` rather than an unconditional assignment, because the early
  return can also fire on `e.pending != nil` alone. Today that alternate shape
  is unreachable in this branch (the re-drive only calls `DrawFor`, whose only
  ask is the dredge ask — a Draw emit poses no replacement competition), but
  the guard keeps the fix total if one ever lands. The finding's prescribed
  behaviour is identical whenever `e.resume != nil`, which is every reachable
  case.
- `budget_s` consolidated to 300 (the effective first-line value), not to any
  of the other stacked values — this preserves the gate's behaviour exactly;
  any other choice would have silently changed the enforced budget.

## Open concerns

None new. The fix is confined to `resumeResolution`'s parked-frame arm; the
direct arm, `effDraw`'s own cursor path and the turn-draw path are unchanged
and separately pinned by the pre-existing tests, all of which pass.

## Issues

- (pre-existing, recorded by the previous round, still live) Lich's controller
  survival depends on the unimplemented `R:Event$ GameLoss CantHappen`
  family: plain `Lich` would park its controller at 0 life and die to the SBA
  where Forge keeps them alive (`rules/life_draw_dredge_test.go`'s doc
  comment). The tests here use Nefarious Lich for exactly that reason. A
  CR-lane leaf citing CR 112/704 (replacement "instead of losing the game")
  would make this visible to `make ledger`.
- (pre-existing, unchanged scope) The parked cascade's `e.pending != nil`
  early return without a linked continuation remains a latent hole if a future
  Draw-event replacement competition ever poses an order decision inside
  `lifeReplacementDraw`'s re-drive — measured corpus-unreachable today (a
  `Draw` emit routes through no replacement-order machinery in
  `rules/replacement.go`'s `applyReplacements`), noted so the next person
  touching the Draw path re-checks it.

STATUS=DONE
