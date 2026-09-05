# Deferred minors, parked findings and final-wave items (digest from the ledger)

Source of truth: progress.md. Each line below is verbatim from it, with its
line number, so the final reviewer can read the surrounding context.

## Lines tagged for the FINAL WAVE
  1910:Ruling T21-i (parked for the final whole-branch fix wave): the code's ruling
  1918:  reference is a real defect. Fix in the final wave; do not let a fixer guess the
  2627:PARKED FOR THE FINAL WHOLE-BRANCH FIX WAVE: N9 code half. Revisit the loop AS A
  3139:  for the FINAL WHOLE-BRANCH FIX WAVE, deliberately not patched a fifth time.
  3219:  round. Sweep in the final wave.
  3690:Task 23: minor (deferred, FINAL WAVE): view/view_test.go:444-446 orphaned doc
  3693:Task 23: minor (deferred, FINAL WAVE): view/redact.go:8-11 doc claims a copy;
  3697:Task 23: minor (deferred, FINAL WAVE, plan-mandated by T23-s): rule 2 filters
  3699:  emitter carries them today. Ruling T23-z: the final wave applies rule 3's
  3710:## 2 fix rounds, review clean, 3 minors deferred to the final wave)
  3770:adjudicates; otherwise it joins T21-i's comment sweep in the FINAL WAVE.
  3846:Task 28: minor (deferred, FINAL WAVE): rules/turn.go:253-254 -- the I-4
  4114:Task 24: minor (deferred, FINAL WAVE candidates): M-1 replay-runs-SHORT

## Every "minor (deferred)" line
  130:Task 2: minor (deferred): parseSA leaves a second head-shaped key in Params on a
  132:Task 2: minor (deferred): parseParams silently drops a `|`-split segment with no
  136:Task 2: minor (deferred): no test covers `R:`/Repl parsing, the on-disk `Parse()`
  138:Task 2: minor (deferred): leading whitespace after a top-level `Key:` is
  151:Task 3: minor (deferred): Link() is not pointer-identity-idempotent for
  155:Task 3: minor (deferred): the cyclic half of the survives-cycles test asserts
  169:Task 4: minor (deferred): a 0-byte .txt is absorbed with no Diag and adds a
  195:Task 4: minor (deferred): on the decode-error branch, zr.Close()'s own error is
  210:Task 5: minor (deferred): a failure between the corpus rename and WriteLock would
  213:Task 5: minor (deferred): DigestDir sorts native-separator paths before
  254:Task 6: minor (deferred): CopyFaceFrom split-card support is unimplemented. Now
  621:Task 12: minor (deferred): the hybrid pinning test asserts parse results for all
  626:Task 12: minor (deferred): `mana.go` `c.Generic += int32(n)` can overflow int32
  669:Task 13: minor (deferred): the two new tests duplicate ~25 lines of fixture
  671:Task 13: minor (deferred): `mtgcore/rules/fix1_test.go` is a meaningless name
  744:Task 15: minor (deferred): `copyTargets` uses `append([]state.Target(nil), s...)`,
  817:Task 16: minor (deferred): the new test sets `g.Obj(id).Zone` directly without
  876:Task 17: minor (deferred): `applyCountOp` silently wraps on int32 overflow — a
  880:Task 17: minor (deferred): chained `/Op` suffixes past the first are silently
  897:Task 17: minor (deferred): `count.go` bounds check reads
  904:Task 17: minor (deferred): `EvalCount` short-circuits to 0 on a nil `Ctx` rather
  990:Task 14: minor (deferred) — CR 608.2b target recheck at resolution is implemented
  998:Task 14: minor (deferred): `castSpell` does not re-validate that the object is
  1002:Task 14: minor (deferred): `events.Damage` routes to player 0's life if `Obj`
  1005:Task 14: minor (deferred): `addMana` with a negative Amount produces a negative
  1023:Task 14: minor (deferred): the `askTarget` fizzle branch (stack.go:98) is fixed
  1086:Task 18: minor (deferred): `effCharm`'s `len(choices) == 0` guard is dead —
  1088:Task 18: minor (deferred): `effRepeat` has no upper bound on its repeat count, so
  1092:Task 18: minor (deferred): `primitives_test.go` is 780 lines, 3.5x the next file.
  1111:Task 18: minor (deferred): `DrawFor` is not a literal no-op for an invalid seat.
  1280:Task 19b: minor (deferred): `alternativeCosts` has an unused `p state.PlayerID`
  1301:Task 19b: minor (deferred): `castSpell`'s out-of-range `AltCostIndex` fallback
  1306:Task 19b: minor (deferred): neither cast option sets `Option.Player` in
  1365:Task 19c: minor (deferred): `Num()` truncates an out-of-range literal via a bare
  1523:Task 20: minor (deferred): split `applyReplacements`/`replacementMatches` out of
  1525:Task 20: minor (deferred): `damageSource()`'s stack-top heuristic interacts with
  3690:Task 23: minor (deferred, FINAL WAVE): view/view_test.go:444-446 orphaned doc
  3693:Task 23: minor (deferred, FINAL WAVE): view/redact.go:8-11 doc claims a copy;
  3697:Task 23: minor (deferred, FINAL WAVE, plan-mandated by T23-s): rule 2 filters
  3846:Task 28: minor (deferred, FINAL WAVE): rules/turn.go:253-254 -- the I-4
  3850:Task 28: minor (deferred): M-1 raw PlayerLost forge in fix1_test.go (organic
  4114:Task 24: minor (deferred, FINAL WAVE candidates): M-1 replay-runs-SHORT

## Every "parked" line
  460:Ruling T11-f (minor, PARKED): Engine.New's per-player genesis loop does not
  464:  refuses. Not caused by this diff and unreachable with real decks. Parked
  1340:Ruling T19c-b (PARKED, carried to Tasks 20/21): three residual gaps, all
  1440:    seats; parked as benign, but Task 21 rewrites elimination and should subsume
  1717:Ruling T20-g (parked, assigned to Task 24): `state.Step` has no `Valid()` method,
  1737:  Parked here explicitly so it is dropped deliberately rather than silently; it is
  1757:elimination path, so Engine.New was never naturally touched. Stays parked.
  1820:  scope, NOT part of the user's trigger-ordering requirement, so it is parked
  1882:Ruling T21-h (parked, NOT this milestone): the non-Trample multi-block "last
  1910:Ruling T21-i (parked for the final whole-branch fix wave): the code's ruling
  1999:   parked since Task 11 and re-deferred by Task 21 on the grounds that nothing
  2175:#### Parked, with pointers
  2627:PARKED FOR THE FINAL WHOLE-BRANCH FIX WAVE: N9 code half. Revisit the loop AS A
  2655:Nothing was parked to make the count look better. What is parked (N9's code half)
  2656:is parked on stated grounds with a pointer.
  2818:means N9's parked code half is now load-bearing for someone else's hang. 360-game
  3039:#### PARKED, FORWARD-LOOKING HAZARD -- carry into any task that gives triggered
  3047:abilities will silently do nothing. This compounds the parked T19c-b item
  3086:adversarial verification of item 1's blast radius (see the parked hazard above):
  3136:### Parked findings, with owners
  3147:- Task 27's `ValidTgts$` fizzle consequence -- see the parked hazard above.
  3209:#### Parked from this review
  3236:R1/R2 done + R3 unstarted in Task 23, all nine parked findings with owners, the
  3506:Ruling T23-r: concern (3) parked, pre-existing (Task 20), documented in
  3634:Ruling T23-x (concern 1): NOT Task 23's, NOT parked. It is a totality

## Named open items with owners (from the resume prompt)
  N9 (Task 22, attempted-set predicate-flip sub-case) — final wave, revisit the loop as a whole; four pins are the gate
  N2 (TargetMin$ 0 + the spec!="" fizzle gate) — whoever implements TargetMin$
  T19c-b (Equip/Attach, TriggeredCard$, non-mana activated-ability enumeration) — M1 acceptance-deck coverage / M2
  T21-h (multi-block last-blocker-absorbs approximation) — combat-UI milestone
  T21-i (code cites wrong ruling numbers; canonical map in the ledger) — final wave
  N4 (optional trigger whose decider departs is silently declined) — CR 800.4a limitations
  CR 800.4a general case — M2
  N3 (cards/parse_test.go:123 130-char comment) — final wave
  T23-z (rule 2 IDs/Pairs filter on zone-move kinds) — final wave
  Task 23 minors: orphaned test comment (view_test.go ~444), redact.go doc vs pass-through aliasing — final wave
  Task 28 minors: turn.go I-4 comment misquotes drainerSrc; fix1_test raw PlayerLost forge; SetZone truncation + budget 10; 4-seat variant t.Logf — final wave
  Task 24 minors: replay-runs-short plain error (no Seq); param name — final wave candidates
  Task 25 notes: redundant pass fixes (accurate for future shapes); playerDamage metric includes Bolt — no action

## Added after the digest was written
  Task 29 (re-review): rules/trigger.go:968 comment headed "Review finding I-3" carries M-6's content; real I-3 block at trigger.go:989 — label fix — final wave
