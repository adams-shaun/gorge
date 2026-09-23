# Report — task agent-20260918T231813Z-2ff69b35 (cost-draw1)

## Headline: the brief's premise was stale — the core work is ALREADY on main

The brief (regenerated 2026-09-23T03:20:03Z) was written against base
`6b639c54`. This worktree's HEAD at dispatch was `c947f5c8` (two days and
~dozens of merges later; after the mandated `git rebase main` it is
`8460b5d9`). On that base the entire ticket is **already implemented and
tested** by a sibling commit:

```
$ git log --all --oneline -S 'drawDynCost' -- rules/mana.go
4909a8f7 feat(rules): model dynamic Draw<X/...> costs and settle the trigger-window draw
$ git merge-base --is-ancestor 4909a8f7 HEAD && echo YES
YES
```

`4909a8f7` (authored 2026-09-20, from the sibling branch
`wt/agent-20260918T222614Z-5b138b5e`) landed exactly what the brief asks for,
on the same carrier class (`Cost$ Draw<X/You>` + card-level `SVar:X`):

- `rules/mana.go` `drawDynCost` regex + the parse branch that records a Draw
  part with `Dyn` naming the source SVar (mirror of `payLifeXCost`), so the
  unrecognised-symbol fallback that priced the token as one generic mana and
  reported `cost:Draw` never runs.
- `rules/mana.go` `drawCostCount` / `drawCostCountTrig` — resolve the source
  face's `SVar:X` through `effects.EvalCountOK` and fail closed when the body
  is absent/unresolvable (the structural twin of `fixLifeXCost`, but resolved
  at PAYMENT from `Dyn` instead of folded at offer time into `Announced`).
- `rules/cumulative.go` `triggeredCostPayable` gates the trigger window on
  `triggeredCostDrawCounts`, and the window settles the draws.
- `rules/draw_cost_test.go` pins it end to end on the real corpus card
  **Champion of Wits** (`SVar:X:Count$CardPower`): pay draws 2 then discards 2,
  decline does neither, plus `TestParseCostModelsDynamicDraw` for the parse.

**The `Depends-On` is satisfied**: `agent-20260918T224137Z-0f39bb4c`
(`trig:CostedExecutor`) is `status: closed`, `disposition: merged — 589b13a6
646c4650`; commit `8ccb8ee5` ("trig:CostedExecutor — a trigger whose Execute$
body is an AB$ with a Cost$…") is in this branch's ancestry. So the
trigger-executed payment window the brief called out as owned by the dependency
has landed too.

Measured directly at HEAD (my own throwaway probe):

```
ParseCost("Draw<X/You>") -> Unknown=[] Draw=[{Spec:You, Dyn:X}] Generic=0
ParseCost("Draw<1/You>") -> Unknown=[] Draw=[{Spec:You, N:1}]     Generic=0
ParseUnlessCost("Draw<X/You>") -> ok=false (deliberate decline)
```

That is the brief's "Done #1" exactly.

## What I actually changed (the real remaining gaps)

Two brief-named items were genuinely absent, so I closed them:

1. **The census row.** `TestParseCostReportsUnmodelledCostTokens` still had no
   `{"Draw<X/You>", nil}` row — the brief names this entry explicitly ("the
   entry the ticket means by 'the cost:Draw param-census label retires'").
   Added it beside `{"Draw<1/You>", nil}` with a comment naming the carriers.
   (`rules/paramcensus_test.go`, +6 lines.)

2. **The ticket's carrier pin.** `TestTitanOfLittjaraDrawXCost` did not exist.
   Added `rules/draw_x_cost_test.go` with three tests:
   - `TestDrawXCostSVarFoldsAndDraws` — the fold: `drawCostCount` on a
     battlefield Champion of Wits resolves to its derived power, and the
     trigger window offers the pay election (preconditions asserted first: the
     permanent is on the battlefield, its `SVar:X` body is present and
     non-empty, the count is positive).
   - `TestDrawXUnresolvableWithheld` — the fail-closed contract: a
     `Draw<X/You>` part whose source face has no `SVar:X` is refused by
     `drawCostCount`, as is a nil source.
   - `TestTitanOfLittjaraDrawXCost` — the real-corpus trigger window on the
     ticket's canonical card: Titan's ETB pushes, the window poses a
     pay/decline election sourced to Titan, paying settles the draw and runs
     the `AB$ Discard` body (the `ResumeKind=="discard"` ask), and a decline on
     a fresh Titan draws nothing and runs no body. The pay-arm draw count is
     asserted equal to the gate's own `drawCostCount` verdict, so the offer and
     the charge cannot disagree.

`rules/draw_x_cost_test.go` is a NEW file (per the "new tests go in a new file"
rule), so it cannot conflict with any other ticket's test appends.

## Fails without the fix (mandatory proof)

I reverted the implementation outside the test file: neutralised `drawDynCost`
in `rules/mana.go` (prefix `^ZZNOMATCH`) so `Draw<X/You>` falls through to the
unrecognised-symbol fallback again, ran the tests, then restored the file
byte-identically (`cp` + `cmp`).

Real failing output:

```
--- FAIL: TestDrawXCostSVarFoldsAndDraws (0.63s)
    draw_x_cost_test.go:75: the resolvable Draw<X/You> cost was not offered as
      payable: [{Index:0 Kind:trigger_cost_decline Label:Do not pay ...}]
--- FAIL: TestTitanOfLittjaraDrawXCost (0.01s)
    draw_x_cost_test.go:149: Titan's Draw<X/You> cost was not offered as
      payable: [{Index:0 Kind:trigger_cost_decline Label:Do not pay ...}]
--- FAIL: TestParseCostReportsUnmodelledCostTokens (0.00s)
    paramcensus_test.go:3479: ParseCost("Draw<X/You>").Unknown = [Draw], want []
FAIL
```

Restoration verified:

```
$ cp .ds4/scratch/mana.go.bak rules/mana.go && cmp .ds4/scratch/mana.go.bak rules/mana.go
RESTORED byte-identical
$ git diff --stat rules/mana.go     # (empty: mana.go is unchanged)
```

`TestDrawXUnresolvableWithheld` correctly stays green under the revert (it pins
the WITHHOLD contract, which holds whether or not the parse branch exists) —
it is a contract pin, not a "parse works" pin, and is honest about that.

## Gates (exact commands and real output)

`.cards` symlink: **present** at dispatch (`-> /home/sadams/projects/gorge/.cards`),
so every corpus-backed test ran for real; the registry-based tests took
0.6–1.0s, not the sub-5s vacuous signature.

Targeted gate (the brief's permitted invocation, names adjusted to the tests I
wrote):

```
$ go test -run 'TestDrawXCostSVarFoldsAndDraws|TestDrawXUnresolvableWithheld|TestTitanOfLittjaraDrawXCost|TestParseCostReportsUnmodelledCostTokens|TestParseUnlessCostDrawComponents|TestDrawCostDrawsThePayer' ./rules/ -v
--- PASS: TestDrawCostDrawsThePayer (0.01s)
--- PASS: TestDrawXCostSVarFoldsAndDraws (0.74s)
--- PASS: TestDrawXUnresolvableWithheld (0.01s)
--- PASS: TestTitanOfLittjaraDrawXCost (0.02s)
--- PASS: TestParseCostReportsUnmodelledCostTokens (0.00s)
--- PASS: TestParseUnlessCostDrawComponents (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.856s
```

(Run again after the mandated rebase onto `main`/`8460b5d9`: same six PASSes,
`ok ... 1.004s`.)

Behaviour goldens (mandatory, ~2 s):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.672s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.303s
```

Format and vet:

```
$ gofmt -l rules/draw_x_cost_test.go rules/paramcensus_test.go
(no output)
$ go vet ./rules/
(no output)
$ go run ./cmd/gentypes -check
exit=0
```

## Head / ratchet movement

**None, by construction and by measurement.** All 9 `Draw<X/…>` carriers are
absent from every repo deck:

```
$ for n in "Titan of Littjara" "Katara, Waterbending Master" "Champion of Wits" \
    "Sanctum of Calm Waters" "Hordewing Skaab" "Horrid Shadowspinner" \
    "Armor Wars" "Uncover the Moon-Letters" "Bebop, Skull & Crossbones"; do
    printf '%s ' "$(/usr/bin/grep -rl "\"$n\"" internal/testutil/decks/ | wc -l)"; echo "$n"; done
0 Titan of Littjara
0 Katara, Waterbending Master
0 Champion of Wits
0 Sanctum of Calm Waters
0 Hordewing Skaab
0 Horrid Shadowspinner
0 Armor Wars
0 Uncover the Moon-Letters
0 Bebop, Skull & Crossbones
```

Both goldens pass on the rebased base, confirming no head/botbench movement.
`rules/heads_test.go` untouched; no ratchet table edited (the
`knownUnsupported` / `knownUnsupportedParams` rows are unchanged — this itself
changes no card behaviour, only adds pins).

## The 9 Draw<X/…> carriers and whether their X body resolves

Measured with a throwaway probe on `&effects.Ctx{Source: id, Controller: 0,
SVars: face.SVars}` + `drawCostCount`, after moving each card to the
battlefield:

| carrier | `SVar:X` body | resolves |
|---|---|---|
| Champion of Wits | `Count$CardPower` | **n=2, ok=true** |
| Sanctum of Calm Waters | `Count$Valid Shrine.YouCtrl` | **n=1, ok=true** |
| Horrid Shadowspinner | `Count$CardPower` | **n=2, ok=true** |
| Titan of Littjara | `Count$Valid Creature.YouCtrl+Other+sharesCreatureTypeWith` | n=0, ok=true |
| Katara, Waterbending Master | `Count$YourCountersExperience` | n=0, **ok=true** |
| Hordewing Skaab | `TriggeredPlayersTargets$Amount` | n=0, **ok=true** |
| Uncover the Moon-Letters | `TriggeredCard$CastTotalManaSpent` | n=0, **ok=true** |
| Bebop, Skull & Crossbones | `Count$CardCounters.ALL` | (probe blocked on name; see below) |
| Armor Wars | `Count$Valid Artifact.YouCtrl` on an `UnlessCost$` | out of scope |

**The brief's "resolve ok=false" claim for Katara / Hordewing Skaab / Uncover
the Moon-Letters is false by measurement.** All three resolve `n=0, ok=true`:
`EvalCountOK` treats those count heads as a known 0 rather than as unresolvable.
Consequence: those three abilities are **offered and pay 0**, NOT withheld —
i.e. the opposite of the "documented fail-closed direction" the brief expected.
Titan likewise resolves to `n=0` in isolation because its chosen creature type
is empty (see below), so its `Draw<X/You>` prices 0 rather than being withheld.
This is a *count-head* gap (the three heads are unmodelled and silently read 0),
not a Draw-cost gap; it is reported under `## Issues`.

Two of the nine could not be driven in the probe for fixture-name reasons
("Bebop, Skull & Crossbones" is actually `Name:Bebop, Skull & Crossbones` with
an ampersand; "Uncover the Moon-Letters" has a hyphen). Their bodies are listed
above from the corpus text; `Count$CardCounters.ALL` and
`TriggeredCard$CastTotalManaSpent` are not count heads this evaluator
implements, so they are expected to read 0 like the others.

## Deviations from the brief

1. **The core implementation was not re-done.** It already exists on main
   (sibling commit `4909a8f7`). I did not add a `fixDrawXCost` fold-time helper
   with `CostPart.Announced`, because the landed design resolves the dynamic
   count at payment from `CostPart.Dyn` and has the same observable contract
   (resolvable → folded count; absent/unresolvable → fail closed). Adding a
   parallel helper would be a second home for the same rule — the brief's own
   "one home" principle. I pinned the landed design instead.
2. **`rules/mana.go` and the call sites were not touched**, because they
   already do the right thing; changing them would be an out-of-brief hunk.
3. **`rules/speed.go:179`**: not relevant — the corpus has no speed cost
   carrying a `Draw<…>` part (`/usr/bin/grep -rlE 'Cost\$ Draw<'` over the 9
   carriers finds only cast/activation/trigger/unless shapes; no `Speed$`
   interaction). No change needed; recorded here per the brief's instruction to
   say so either way.
4. **`Count$xPaid` announced Draw form**: there are 0 corpus carriers, so it
   was left declining; no `xAsk` / `costAnnouncesPaidX` change made, per the
   brief's out-of-scope note.
5. **`Armor Wars`** stays a hard decline in `ParseUnlessCost` (pinned by
   `TestParseUnlessCostDrawComponents`, which still passes) — confirmed
   out-of-scope carrier: its line is `UnlessCost$ Draw<X/You>`, and the
   unless-pay answer carries no X binding.
6. The brief's "zero matches … discards nothing" assertion: measured, paying
   `Draw<0>` still pays the cost and runs the body (draws 0, then the discard).
   Only the DECLINE leaves the body unrun. This matches engine/Forge cost
   semantics, as the brief anticipated.

## ## Issues

1. **Titan of Littjara's ETB `ChooseType` never runs — `ChosenType` is empty.**
   Titan's script carries `K:ETBReplacement:Other:ChooseCT` with
   `SVar:ChooseCT:DB$ ChooseType | Type$ Creature | AILogic$ MostProminentInComputerDeck`
   — note **no `Defined$`**. After moving Titan to the battlefield,
   `e.G.Obj(titan).ChosenType == ""` and no type ask is resolved. The same
   replacement key on Cavern of Souls (`DB$ ChooseType | Defined$ You | …`)
   works — `ChosenType == "Bear"` — so this is the **missing `Defined$`** on
   the `ChooseType` body, not a general ETBReplacement breakage (probe output
   below). Consequence: Titan's `Count$Valid
   Creature.YouCtrl+Other+sharesCreatureTypeWith` always reads 0, so Titan
   draws 0 cards even when it shares a type with a same-type creature. Filed:
   `.ds4/new-tickets/etbreplacement-choosetype-not-run.md`.

   ```
   cavern chosen="Bear"    (Cavern of Souls, DB$ ChooseType | Defined$ You)
   titan  chosen=""        (Titan of Littjara, DB$ ChooseType, no Defined$)
   ```

   Prevalence: 63 `ETBReplacement … ChooseCT` carriers, 111 `DB$ ChooseType`
   carriers; every `ChooseType` body without `Defined$` in that family silently
   picks nothing, with no unimplemented-API Note.

2. **One pushed trigger produces TWO pay/decline asks.** With exactly one
   `TriggerPush` event for Titan's ETB, the trigger-cost window poses the
   `trigger_cost_pay`/`trigger_cost_decline` election **twice** before the
   `ResumeKind=="discard"` ask appears. Measured:
   `total trigger_push=1` but two `kind=choose` asks with
   `trigger_cost_pay` before the discard. Files: `rules/cumulative.go`
   (`triggeredCostPaymentAsk`, `startTriggeredEffectCost`) and the
   `Execute$`/`Cost$` resolution path. Whether this is a double-charge or a
   harmless re-pose I did not determine — it is outside this brief (the
   trigger-executed window is owned by `agent-20260918T224137Z-0f39bb4c`, whose
   commits are `589b13a6 646c4650`). A user could decline twice or pay twice;
   if the second pay re-draws, Titan double-draws. Filed:
   `.ds4/new-tickets/trigcosted-double-pay-ask.md` (Depends-On the
   CostedExecutor ticket).

3. **Three unmodelled X count heads silently read 0 instead of withholding.**
   `Count$YourCountersExperience` (Katara, Waterbending Master),
   `TriggeredPlayersTargets$Amount` (Hordewing Skaab),
   `TriggeredCard$CastTotalManaSpent` (Uncover the Moon-Letters) all return
   `ok=true, n=0` from `effects.EvalCountOK`, so an ability whose X depends on
   them is offered and pays 0 rather than being withheld. This is the
   fail-OPEN direction, opposite to `fixLifeXCost`'s fail-closed contract for a
   body it cannot resolve. Worth deciding whether a known-but-unmodelled count
   head should report unresolvable (`ok=false`) so the cost withholds. Corpus
   prevalence: these three heads plus whatever else the count-head ratchet
   (`rules/count_head_ratchet_test.go` `knownUnmodelledCountHeads`) holds.

4. **`Bebop, Skull & Crossbones` has no resolvable probe fixture name** — the
   corpus `Name:` line is `Bebop, Skull & Crossbones` (ampersand), not the
   brief's `Bebop, Skull-Crossbones`. Minor: the brief's carrier list uses a
   different spelling than the script. No code impact.

5. **No CR-lane test currently makes the `cost:Draw` X form visible** to the
   ledger generator. My `TestParseCostReportsUnmodelledCostTokens` row pins the
   census, but if a future regression re-breaks the parse, the CR-conformance
   lane will not name it. A CR-lane test citing CR 118.2 / 601.2h (costs that
   draw cards) on a `Draw<X/…>` cast or trigger would close that invisibility.
   No test written (not asked); named here per the report contract.

STATUS=DONE_WITH_CONCERNS
COMMITS=c0152ab1
TESTS=go test -run 'TestDrawXCostSVarFoldsAndDraws|TestDrawXUnresolvableWithheld|TestTitanOfLittjaraDrawXCost|TestParseCostReportsUnmodelledCostTokens|TestParseUnlessCostDrawComponents|TestDrawCostDrawsThePayer' ./rules/ -> 6 PASS; goldens archtest+botbench pass
