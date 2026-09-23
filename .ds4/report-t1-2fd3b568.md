# Report — agent-20260918T195920Z-2fd3b568

## Task

Loamcrafter Faun's `SVar:X:TriggerRemembered$Amount` count head was unread, so
the "when you do, return up to that many" arm never asked. The sibling ticket
`agent-20260918T233200Z-4a2fcd44` (already merged on this branch at dispatch:
`f422070d`, its fix `d1da297d`) admitted `TriggerRemembered$<Property>` but
bound it to **plain `Ctx.Remembered`** — the mapping this brief specifies is
wrong for the dominant chain shape. This ticket corrects the mapping, pins
Loamcrafter Faun end to end, and reports the merged sibling's verdict.

## What changed and why (per file)

### `effects/count.go`
- Split `TriggerRemembered` out of the shared
  `TriggeredCard`/`DelayTriggerRemembered`/`RememberedLKI` case (which returns
  plain `c.Remembered` unchanged) into its own case that returns
  `rememberedExcludingCapture(h, c)`.
- Added `rememberedExcludingCapture(h, c)`: `Ctx.Remembered` minus
  `Ctx.Captured` (Forge's host remembered list never contains the trigger's
  own event object). It is the **one home** for that exclusion, so the count
  head and `effImmediateTrigger`'s parent computation cannot drift.
- `Spawner>` re-anchoring now nils `sc.Captured` after substituting
  `sc.Remembered = c.Captured`: the substitution consumes the capture into
  `Remembered`, so a nested capture-excluding ref must not subtract it back
  out. No corpus line writes `Spawner>TriggerRemembered`
  (`/usr/bin/grep -rl 'Spawner>TriggerRemembered' .cards/cardsfolder` → 0);
  this just keeps the composition structurally correct.

**Why the mapping is capture-excluded:** `rules/trigger_match.go` seeds a
firing trigger's ctx with `Remembered == Captured == triggerRememberedFor(...)`
(the event object, for Loamcrafter the ETB'd source itself), and the chain's
`RememberDiscarded$ True` rider appends the discarded lands to `Remembered`
(`effects/cardflow.go` `discardAndRememberEvent`). So at
`TrigImmediateTrig`'s ctx, `Remembered = [Loamcrafter, land…]` while
`Captured = [Loamcrafter]`; plain `c.Remembered` counts the source too. Inside
an `effImmediateTrigger` instance the ctx is already `parent = Remembered −
Captured` with a disjoint `Captured`, so the exclusion is **idempotent** and
the dominant instance-ctx read is unchanged.

### `effects/immediate.go`
- Replaced the inline captured-exclusion loop in `effImmediateTrigger` with a
  call to the shared `rememberedExcludingCapture(h, c)`. Byte-for-byte the
  same result; now the two sites share one implementation.

### `effects/count_triggerremembered_test.go`
- Rewrote the sibling's fixture, which encoded the overcount bug
  (`Captured: rem` with `rem == Remembered`, asserting `Amount == 2`). The
  fixture now seeds the **real chain ctx**: `Captured` = a distinct ETB'd
  source object, `Remembered` = that source plus the two chain objects.
  Assertions: `TriggerRemembered$Amount == 2` (capture excluded),
  `TriggerRemembered$CardPower/Toughness/ManaCost/CardManaCostLKI/
  CardCounters.*` over the two chain objects, `/Op` suffix, and the adjacent
  `TriggeredCard$*`/`RememberedLKI$*` reads still seeing the whole list of 3
  (proving the exclusion is `TriggerRemembered`'s alone). Added the
  instance-ctx idempotence case. Added explicit `EvalCountOK` verdicts for the
  exotics: `CastTotalManaSpent` (0, **true**) and `CardManaCostLKI` (6,
  **true**) are modelled; `GreatestCardManaCost` (0, **false**) and
  `CardTypes` (0, **false**) stay fail-closed.

### `rules/loamcrafter_faun_test.go` (new)
- `TestLoamcrafterFaunWhenYouDoReturnsThatMany`: real corpus card, ETB fires,
  optional any-number land discard ask answered with 2 lands (asserts 2 real
  `Discard` events), then ONE return ask with `Max == 2` (not 3 — the
  overcount guard) over the graveyard nonland permanents, and the answered
  `ChangeZone` moves both to hand.
- `TestLoamcrafterFaunEmptyDiscardIsASilentNoOp`: declining the optional
  discard (0 cards) poses no return ask and leaves the graveyard untouched;
  asserts no `unimplemented`/`ImmediateTrigger`/`Discard` Note ran, so the
  no-ask result is not the feature being unregistered.

## The merged sibling's mapping verdict (Done means #6)

**The sibling landed plain (`return c.Remembered, true`) and it needed
correcting.** Measured, not assumed: with the sibling's plain mapping and this
ticket's tests in place, the effects unit test fails
(`TriggerRemembered$Amount = 3, want 2`) while the Loamcrafter end-to-end test
**passes** — because `effImmediateTrigger` hands each instance an already
capture-excluded `Remembered` with a disjoint `Captured`. So the end-to-end
pin cannot discriminate the mapping; the unit test is the mapping arbiter and
the correction is in this diff, exactly as the brief anticipated.

## Fails without the fix (mandatory)

Snapshot: `effects/count.go` → `.ds4/scratch/count.go.fixed`,
`effects/immediate.go` → `.ds4/scratch/immediate.go.fixed`,
`rules/loamcrafter_faun_test.go` → `.ds4/scratch/loamcrafter_faun_test.go.fixed`.
Both reverts restored byte-identically (`cmp` → `ALL_RESTORED_BYTE_IDENTICAL`).

**A. Plain mapping (the sibling's exact return):** revert only the
`TriggerRemembered` case to `return c.Remembered, true`; the count head tests:
```
$ go test -run 'TestTriggerRememberedRefProperty$' ./effects/
--- FAIL: TestTriggerRememberedRefProperty (0.00s)
    count_triggerremembered_test.go:63: precondition: TriggerRemembered$Amount = 3, want 2 (capture not excluded)
FAIL
FAIL	github.com/adams-shaun/gorge/effects	0.002s
```

**B. Head absent (pre-sibling state):** rename the case so it falls to
`default: return nil, false`; the end-to-end test fails (no return ask was
ever posed — the first non-priority decision is the next turn's cleanup
discard):
```
$ go test -run 'TestLoamcrafterFaunWhenYouDoReturnsThatMany$' ./rules/
--- FAIL: TestLoamcrafterFaunWhenYouDoReturnsThatMany (0.58s)
    loamcrafter_faun_test.go:202: return ask Max = 1, want 2 (the discarded lands, capture excluded): &{Seq:195 Player:1 Kind:choose Prompt:turn 2 — discard 1 card(s) down to the hand-size limit Min:1 Max:1 ...}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.591s
```

Both tests therefore fail on a real behavioural change, not vacuously.

## Brief premise re-measurement (the numbers held)

```
$ /usr/bin/grep -rE '(SP|DB|AB|ST|RE)\$ ImmediateTrigger' .cards/cardsfolder | wc -l
323
$ /usr/bin/grep -rlE '(SP|DB|AB|ST|RE)\$ ImmediateTrigger' .cards/cardsfolder | wc -l
312
$ /usr/bin/grep -rl 'TriggerRemembered\$' .cards/cardsfolder | wc -l
42
```
No repo deck carries an ImmediateTrigger carrier and Loamcrafter Faun is in
none (python walk over `internal/testutil/decks/*.json`), so `TestHeads` and
the ratchet could not move — confirmed by the botbench golden below.

Brief inaccuracy found: the brief's "4 exotics" list is wrong. Re-measured
against `evalRefProperty` (effects/count.go:925-1180), `CastTotalManaSpent`
(case at 1064) and `CardManaCostLKI` (case at 986) are **modelled**; only
`GreatestCardManaCost` and `CardTypes` are absent and stay fail-closed. So the
true exotic count is **2**, not 4. The test pins both verdicts via
`EvalCountOK`.

## Gate commands and their real output

Environment: `.cards` symlink **present** at the worktree root
(`.cards -> /home/sadams/projects/gorge/.cards`), so corpus-backed tests ran
rather than skipped.

Done-means #3 (targeted ImmediateTrigger/TriggerRemembered pins):
```
$ go test -run 'TestLoamcrafterFaun|TestTriggerRemembered|TestRefProperty|TestImmediateTrigger|TestForumFilibuster|TestSpeedYoungAvenger' ./effects ./rules
ok  	github.com/adams-shaun/gorge/effects	0.592s
ok  	github.com/adams-shaun/gorge/rules	(cached)
```

Done-means #4 (affected packages, once, before committing):
```
$ go test ./effects ./rules
ok  	github.com/adams-shaun/gorge/effects	2.557s
ok  	github.com/adams-shaun/gorge/rules	31.127s
```

Done-means #5 (format/vet):
```
$ gofmt -l effects/count.go effects/immediate.go effects/count_triggerremembered_test.go rules/loamcrafter_faun_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output)
$ go vet ./effects ./rules
(no output)
```

Behaviour goldens outside `rules/` (run once, before DONE):
```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.333s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.194s
```
The botbench win split did **not** move, so no head/golden re-pin was needed.

## Structural fix (class, not instance)

The finding was one more capture-excluding caller. Search output
(`/usr/bin/grep -n Captured .cards/cardsfolder` is N/A; in-engine:
`/usr/bin/grep -n 'Captured' effects/*.go`) shows the exclusion is needed in
exactly two places — `effImmediateTrigger`'s parent computation and the
`TriggerRemembered` head — and both now route through the single helper
`rememberedExcludingCapture`. A future ref that needs the same exclusion calls
the helper; the plain `TriggeredCard`/`RememberedLKI` family deliberately does
not, which the unit test pins so widening one does not silently widen the
other.

## Deviations from the brief

1. **Did not `git rebase main`** despite the injected directive at the top of
   the brief (2026-09-23T03:20:03Z). The seat harness (`system-t1.md`,
   `gorge-context.md`) explicitly forbids `git rebase`, and the earlier
   identical directive (2026-09-22T22:26:40Z) was retracted with "rebase
   conflicts are the merge resolver's job". I verified the rebase is
   conflict-free anyway: main's only changes since the merge-base `f422070d`
   are in `effects/count.go` regions ~388, ~1375 and ~4049 (Convoked/Imprint
   work), all disjoint from this diff's ~700-780 regions. The merge/daemon
   resolver can rebase cleanly.
2. **The brief's 4-exotic list is inaccurate** (2 modelled, 2 fail-closed) —
   re-measured and reported above; the tests pin the real verdicts.
3. The end-to-end pin cannot discriminate the capture-excluded mapping from
   plain (measured: plain passes it, because the instance ctx is already
   capture-excluded). The mapping is pinned by the effects unit test instead;
   the end-to-end pin proves the head + chain end to end.

## Issues (defects found, not fixed)

- **`IsTriggerRemembered` filter predicate is unimplemented** (inherited from
  the sibling's report; still open). Corpus: 61 files contain it. Both
  `ValidCard$ Card.IsTriggerRemembered` and
  `ValidPlayer$ Player.controlsCard.IsTriggerRemembered` fail closed in
  `effects/filter.go` / `rules/trigger_match.go`, so a delayed trigger
  registered with it never matches (Blessed Defiance's "when that creature
  dies this turn"). Deserves its own ticket.
- **`TriggerRemembered$GreatestCardManaCost` and `TriggerRemembered$CardTypes`
  stay fail-closed** (2 carriers). Both are absent from `evalRefProperty`;
  `CardTypes` exists only in `evalCountBody`'s distinct-set form. `CardTypes`
  would be a different ticket (add the property to the shared switch);
  `GreatestCardManaCost` rides the open extreme-aggregate ticket `e27469dd`.
- **No CR-lane test warranted** for this change: it makes a previously-dead
  count head answer correctly and both new tests pin it directly.

## Open concerns

- The end-to-end test's return-ask assertion assumes the return ask is the
  first non-priority decision after the discard answer (true and deterministic
  at the fixed seed). If future engine timing shifts, the failure diagnostic
  would name an unrelated cleanup discard rather than "no return ask"; the
  test would still fail, only less legibly.
- `Spawner>TriggerRemembered` is now structurally correct but corpus-unreached
  (0 carriers); no test covers that composition.
