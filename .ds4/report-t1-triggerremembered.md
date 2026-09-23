# Report — agent-20260918T233200Z-4a2fcd44

## Task

Admit `TriggerRemembered$<Property>` as an `evalRefProperty` ref over the
trigger-context remembered bindings with the existing property switch, plus a
regression test.

## What changed

**`effects/count.go`** — one ref name added to `refTargets`' existing
`TriggeredCard`/`DelayTriggerRemembered`/`RememberedLKI` case, which returns
`c.Remembered`:

```go
"DelayTriggerRememberedLKI", "TriggerRemembered", "RememberedLKI":
    // TriggerRemembered ... is Forge's name for the trigger's own
    // RememberObjects$ capture, the same set this engine threads through
    // Ctx.Remembered at resolution ...
    return c.Remembered, true
```

Why `Ctx.Remembered` and not a new slot: the corpus carriers are all
`ImmediateTrigger` chains. `effImmediateTrigger` sets each instance's
`cc.Remembered` to the introspected set (`RememberObjects$ Remembered`), and
`Ctx.Captured` is deliberately left as the OUTER trigger's event capture, which
is why the `Spawner>` arm exists. Loamcrafter Faun's `TrigReturn` sees the
discarded lands in `Remembered`; `TriggeredCard` already reads the same slot, so
`TriggerRemembered` binds identically. The property switch itself needed no
change — the corpus members (`Amount`, `CardPower`, `CardToughness`,
`CardManaCost`, `CardManaCostLKI`, `CardCounters.<KIND>`) all already resolve,
and `CastTotalManaSpent` resolves through the same switch.

**`effects/count_triggerremembered_test.go`** (new file) — pins every corpus
property spelling on a two-object remembered fixture, including a
`CardCounters.P1P1` counter and a `/HalfDown` operator, asserts the
precondition (remembered set populated, counter placed) so the test cannot
pass vacuously, asserts `TriggeredCard$` returns the same values on the same
slot, and pins `UnknownRef$CardPower` at 0.

## Brief premise re-measurement

The brief's prevalence claims held exactly (re-measured):

```
$ /usr/bin/grep -rho 'TriggerRemembered\$[A-Za-z]*' .cards/cardsfolder | sort | uniq -c
      8 TriggerRemembered$Amount
      2 TriggerRemembered$CardCounters
     14 TriggerRemembered$CardManaCost
      1 TriggerRemembered$CardManaCostLKI
     13 TriggerRemembered$CardPower
      2 TriggerRemembered$CardToughness
      1 TriggerRemembered$CardTypes
      1 TriggerRemembered$CastTotalManaSpent
      1 TriggerRemembered$GreatestCardManaCost
$ /usr/bin/grep -rl 'TriggerRemembered\$' .cards/cardsfolder | wc -l
42
```

Also verified: `TriggerRemembered` is never a `Defined$` spelling, only a
count-ref head and (separately) the `IsTriggerRemembered` filter predicate —
so `refTargets` is the single fix point; `effects/context.go`'s
`knownDefinedTargets` does not need a parallel arm.

`api:ImmediateTrigger` has landed (`effects/immediate.go`, registered), so the
Loamcrafter Faun chain is exercisable — but Loamcrafter Faun is not in the
repo-deck acceptance set (no rollup entry in `rules/acceptance_test.go`), so
there is no deck-side ratchet row to flip.

## Gate commands and output

Environment: `.cards` symlink was **present** at worktree root
(`.cards -> /home/sadams/projects/gorge/.cards`), so runs were real, not
skipped. Branch was already at `main` (`7b4ab59e`), so no rebase was needed;
`git rev-parse HEAD` == `git rev-parse main` before any edit.

Targeted regression (green):
```
$ go test -run 'TestTriggerRememberedRefProperty$' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.002s
```

Edited package, run once (green):
```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.588s
```

Format / generated-types checks:
```
$ gofmt -l effects/count.go effects/count_triggerremembered_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output)
```

Golden checks outside `rules/`:
```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.471s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.299s
```

## Fails without the fix

Reverted only the `effects/count.go` hunk (copy preserved as
`.ds4/scratch/count.go.fixed`; restored byte-identically afterwards, `cmp`
confirmed `RESTORED_BYTE_IDENTICAL`), then ran the one test:

```
$ go test -run 'TestTriggerRememberedRefProperty$' ./effects/
--- FAIL: TestTriggerRememberedRefProperty (0.00s)
    count_triggerremembered_test.go:86: TriggerRemembered$Amount = 0, want 2
    count_triggerremembered_test.go:86: TriggerRemembered$CardPower = 0, want 5
    count_triggerremembered_test.go:86: TriggerRemembered$CardToughness = 0, want 6
    count_triggerremembered_test.go:86: TriggerRemembered$CardManaCost = 0, want 6
    count_triggerremembered_test.go:86: TriggerRemembered$CardManaCostLKI = 0, want 6
    count_triggerremembered_test.go:86: TriggerRemembered$CardCounters.P1P1 = 0, want 2
    count_triggerremembered_test.go:86: TriggerRemembered$CardCounters.P1P1/HalfDown = 0, want 1
    count_triggerremembered_test.go:86: TriggerRemembered$Amount/Twice = 0, want 4
    count_triggerremembered_test.go:95: EvalCountOK(TriggerRemembered$Amount) = (0, false), want (2, true): head not admitted
FAIL
FAIL	github.com/adams-shaun/gorge/effects	0.002s
```

## Deviations from the brief

None. The brief said the fix is "one ref-name + the same prop switch"; that is
exactly what landed. No `events.Kind`, no `state` field, no decision, no
botpolicy change.

## Issues

- **`IsTriggerRemembered` filter predicate is unimplemented** (out of this
  brief's scope — the brief names only the `TriggerRemembered$<Property>`
  count-ref head). Corpus: **61 files** contain `IsTriggerRemembered`
  (`/usr/bin/grep -rl 'IsTriggerRemembered' .cards/cardsfolder | wc -l`),
  62 occurrences. Both the card predicate (`ValidCard$ Card.IsTriggerRemembered`
  in `DelayedTrigger` registrations) and the player predicate
  (`ValidPlayer$ Player.controlsCard.IsTriggerRemembered`) fail closed. The
  count-ref head admitted here does not cover it; a `TriggerRemembered$`
  *filter* task is needed. Symptom: a delayed trigger registered with
  `ValidCard$ Card.IsTriggerRemembered` never matches, so its `Execute$` body
  never runs (e.g. Blessed Defiance's "when that creature dies this turn"
  never creates its token). Locate in `effects/filter.go`/`rules/trigger_match.go`
  predicate dispatch.
- **`TriggerRemembered$CardTypes` remains fail-closed** (one carrier, Mount
  Velus Manticore). The `CardTypes` property is not in `evalRefProperty`'s
  switch at all — it is only modelled in `evalCountBody`'s
  `Count$Valid…CardTypes` distinct-set form, not the `<Ref>$<Property>` form.
  So this row is a *property* gap, not the ref-name gap this ticket closed;
  a fix would add `CardTypes` (distinct real card types among the referenced
  objects) to the shared property switch and would cover any future
  `TriggeredCard$CardTypes`/`Remembered$CardTypes` carrier too. That carrier
  also uses `RememberObjects$ RememberedCard`, which is an unrecognised
  `ImmediateTrigger` `RememberObjects$` value (loud Note, whole-set fallback),
  so closing only the property would not make the card work; the
  `RememberObjects$ RememberedCard` spelling needs its own ticket.
- **`TriggerRemembered$GreatestCardManaCost`** (Ill-Timed Explosion) stays
  fail-closed as the brief allowed — it is an extreme aggregate and rides the
  open extreme-aggregate ticket `e27469dd`.

No CR-lane test is warranted for this change: it closes a covered head and the
new test pins it directly; the two issues above are candidates for the backlog,
not the CR lane.
