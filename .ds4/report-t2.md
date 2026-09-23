# count:CardManaCostLKI — implementation report (round 2: rebase + re-verification)

Ticket `agent-20260919T203859Z-cf55fee2` in worktree
`agent-20260919T203859Z-cf55fee2`, branch `wt/agent-20260919T203859Z-cf55fee2`.

## Round-2 context

The round-1 finding was only a rebase blocker: the controller's `git rebase
main` failed because `.ds4/report-t1.md` was unstaged. This round: preserved
the round-1 report artifact by committing it, rebuilt the branch on current
`main` (`f8e330c3`), resolved the one conflict (the shared `.ds4/report-t1.md`
file — main's accumulated multi-ticket report wins; the conflicting content was
this ticket's round-1 report and is superseded by this file), and re-ran every
gate from the rebased tree. The code commit (Root cause B) applied cleanly on
the new base — no code conflict.

## What changed and why (per file)

- **`rules/trigger_referents.go`** — `targetSpecContext` (the resolver behind a
  trigger's target ask) now builds an effects `Ctx` carrying the trigger stack
  object's captured `TriggerContext`, `Remembered`, fire-time `LKI`
  (+ power/toughness/`ptValid`) and the paid X, with `ctx.Host = e` and the
  source face's `SVars` table. Its `Resolve` closure:
  - `X` follows the same **two-shape contract** as the already-merged
    resolution-time `(*Ctx).resolveNumericRHS` / `specCtx` (`rules/mana.go`
    `fixLifeXCost` shape): a `Count$xPaid` body is the announced/captured X,
    any other authored body is evaluated with `effects.EvalCountOK` against the
    host-bound Ctx, and no authored `SVar:X` falls back to the in-flight cast X
    then the stack object's paid X.
  - any other name resolves through the source face's SVar table with the same
    host-bound Ctx; a missing or unresolvable body fails closed `(0, false)`.
  This structurally covers **every** trigger-target filter whose numeric RHS is
  an SVar-backed value, not only Hammerhead Tyrant (the brief's measured class
  is 138 `ValidTgts$` lines with a non-literal numeric RHS).
- **`rules/card_mana_cost_lki_test.go`** (new): the direct resolver contract
  test `TestTriggerTargetSpecContextResolvesSourceXShapes` (Count$xPaid →
  `(3,true)`, plain body `"4"` → `(4,true)`, unresolvable body → fail closed),
  plus the end-to-end `TestHammerheadTyrantTargetsAtMostTheCausingSpellManaValue`
  on the **real corpus** Hammerhead Tyrant with two mana values (4 and 2) and
  a cmc-5 exclusion.
- **`effects/ref_property_lki_test.go`** (new): `TestCardManaCostLKIReadsRememberedSnapshot`
  — proves the `CardManaCostLKI` property reads the **LKI snapshot**'s mana
  value, with a live object whose current mana value is non-zero and **differs**
  from the snapshot (the non-coincidence control).

### Root cause A status

The brief's Root cause A (`evalRefProperty` lacks a `CardManaCostLKI` case) was
already fixed on `main` before this branch: commit `d1da297d`
(`feat(effects): admit TriggerRemembered$<Property> count ref`) added
`CardManaCostLKI` beside `CardManaCost` (`effects/count.go`, now ~:945; the
brief's line anchors :559/:606/:644 were stale — `evalRefProperty` is at :884
on this base). It was **not** changed by this branch; the effects regression
test pins it. Root cause B is the only code change here and is required in
addition to A (verified: without B the trigger's ask still offers nothing —
see "Fails without the fix").

## Workspace

`.cards` **present** (symlink to `/home/sadams/projects/gorge/.cards`), so the
corpus run is real. Confirmed: `TestHammerheadTyrant...` takes 1.09 s with NO
`SKIP` and the real card loads via `choiceCorpusCard`.

Corpus prevalence re-measured at this worktree's base with GNU grep — every
brief claim held:

```text
CardManaCostLKI lines: 58
CardManaCostLKI files: 56
SpellTargeted$CardManaCostLKI files: 4
nonliteral RHS ValidTgts lines: 138
LKI property forms: 58 $CardManaCostLKI   (nothing else)
```

## Fails without the fix

Copied the fixed `rules/trigger_referents.go` to `.ds4/scratch/`, replaced it
with the pre-change version (`HEAD~1:rules/trigger_referents.go`, main's base),
ran the two new tests, confirmed FAIL, then restored the fixed file
byte-identically (`cmp` exit 0).

```text
$ go test -run 'TestTriggerTargetSpecContextResolvesSourceXShapes|TestHammerheadTyrantTargetsAtMostTheCausingSpellManaValue' ./rules/
--- FAIL: TestTriggerTargetSpecContextResolvesSourceXShapes (0.00s)
    card_mana_cost_lki_test.go:26: fixed SVar:X = (3, true), want (4, true)
--- FAIL: TestHammerheadTyrantTargetsAtMostTheCausingSpellManaValue (0.65s)
    --- FAIL: .../Four_Mana_Test_Spell (0.65s)
        card_mana_cost_lki_test.go:84: Hammerhead trigger target ask = ... Kind:priority ...
        ...; trigger must offer targets
    --- FAIL: .../Two_Mana_Test_Spell (0.00s)
        card_mana_cost_lki_test.go:84: ... trigger must offer targets
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.682s
baseline_exit=1   restored_cmp=0
```

The effects property test is **not** claimed to fail against this branch's base
(Root cause A already landed on main); it confirms and pins that pre-existing
implementation with a distinct live-value control.

## Every new test can fail (precondition assertions)

- `TestHammerheadTyrant...` asserts the trigger source is on the battlefield,
  every opponent permanent is on the battlefield, the target ask exists and is
  `decision.KTarget`, each option is a live-face `permanent`, and the offered
  set has exactly the expected cardinality (a vacuous/no-ask setup fails at the
  `d == nil || d.Kind != KTarget` guard). Both subtests fail without the fix.
- `TestTriggerTargetSpecContextResolvesSourceXShapes` asserts both the positive
  values and the fail-closed shape; fails without the fix (shown above).
- `TestCardManaCostLKIReadsRememberedSnapshot` asserts the live mana value is
  non-zero and **differs** from the snapshot value before comparing, so a
  coincidence cannot pass.

## Gates run (all from the rebased tree; exact output pasted)

```text
$ go build ./...
(no output; exit 0)

$ go test -run 'TestHammerheadTyrant|TestVialSmasherChosenPlayerTakesDamage|TestSpellCastActivatorThisTurnCastGatesTheTrigger|TestNightmareUnmaking|TestWhirOfInvention|TestTriggerTargetSpecContextResolvesSourceXShapes' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.677s

$ go test -run 'TestCtxSpecContextResolvesXAndSVarNumericRHS|TestNumericRHS|TestCardManaCostLKIReadsRememberedSnapshot' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.049s

$ gofmt -l effects/count.go effects/ref_property_lki_test.go rules/trigger_referents.go rules/card_mana_cost_lki_test.go
(no output)

$ go vet ./effects/ ./rules/
(no output; exit 0)

$ go run ./cmd/gentypes -check
(no output; exit 0)

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	4.688s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	2.169s
```

Not run (daemon-only gates the brief excludes): `TestHeads` / whole-`rules`
acceptance, `make sim`, `make report`, CR conformance. The round-1 report
recorded `make report` at **33667 / 29775 (88.4%)** unchanged by this change;
`Hammerhead Tyrant` is in no repo deck, so no head movement is expected and
none of the goldens above moved.

## Issues

Defects found and **not** fixed by this round (all already named in the brief;
recorded here for the ledger):

1. **`SpellTargeted$<Property>` ref is unmodelled** — 4 corpus files
   (`reject_imperfection`, `press_the_enemy`, `gales_redirection`,
   `sound_the_trumpets`) carry `SpellTargeted$CardManaCostLKI`; the ref half
   fails closed in `effects/count.go`'s ref resolver, so they read 0 even with
   the property landed. **Filed this round** to
   `.ds4/new-tickets/spelltargeted-cardmanacostlki-ref.md`. A corpus-pinned
   engine test is the right vehicle; CR 608.2c is the rule it would cite.
2. **`TriggerRemembered$<Property>` ref is unmodelled** — already open as
   `agent-20260918T233200Z-4a2fcd44`; the 1
   `TriggerRemembered$CardManaCostLKI` carrier needs both tickets.
3. **A trigger's `SVar:X` is not bound onto its ability stack object** — this
   round's fix recomputes it on demand in `targetSpecContext` (source-face SVar
   table + `triggerPaidX`). Placement-time consumers other than the target ask
   that read `o.X` directly still see 0, and `resolveTop`'s
   `ctx.X = o.X; if 0 { triggerPaidX }` still does not read an authored
   `SVar:X`. This ticket's scope (numeric filter RHS) is fully served by the
   on-demand resolver and the resolution path goes through `SpecContext`
   (`resolveNumericRHS`, already SVar-aware since `7c8e775e`), so no visible
   behaviour gap remains for this card. The clean structural fix — evaluate and
   store the trigger's SVar X once at `pushTrigger` — is deliberately left for
   a follow-up; it would touch `events`/`state` binding and is outside this
   brief. No new ticket filed (the brief already tracks it as its Issues item
   4); escalate if a further consumer surfaces.
4. **Root cause B's general class** (no trigger target ask could resolve any
   non-literal numeric RHS — 138 `ValidTgts$` lines) is **fixed** by this
   round's structural resolver, closing the brief's Issues item 3.

No AGENTS.md "Known approximations" row was present for this shape, so none was
deleted and `knownApproximationRows` is unchanged.

Commit: `a2789eee fix(rules): resolve trigger target X from source SVar`
