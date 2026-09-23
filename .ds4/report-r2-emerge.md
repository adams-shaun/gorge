# Report — round 2 (findings fix): Emerge reduction is generic-only

Fixes both MAJOR findings from `findings-r2.md`.

## What changed

### `rules/emerge.go`
- **Finding 1 (MAJOR, coloured pips erased):** `reduceGenericThenColored` is replaced
  by `reduceGeneric`, which subtracts the mana-value reduction from the cost's
  GENERIC component only, floored at zero, and leaves `Colored` (including `{C}`
  pips), `Hybrid`, `Phyrexian` and every non-mana part untouched. Per CR 118.7 /
  CR 702.118a a mana-value reduction can only reduce the generic amount: Elder
  Deep-Fiend's `{5}{U}{U}` with a mana-value-10 sacrifice is now `{U}{U}`, never
  free. Both callers (`emergeOfferCost`'s offer pricing and `applyEmergeReduction`'s
  post-sacrifice fold) go through the one helper, so offer and charge can never
  disagree. Corpus check: all 15 `K:Emerge` lines are plain generic+coloured
  (`5 U U`, `6 U`, `7 G`, `5 W`, …) — no hybrid/Phyrexian emerge cost exists, so a
  generic-only reduction covers the whole family.

### `rules/emerge_test.go`
- **Finding 2 (MAJOR, total-only pool assertions):**
  - `TestEmergeCastPaysReducedCost` now asserts the funded pool's composition
    (2U 5C) and the remaining pool's composition after payment (0U 3C, total 3),
    not just `Total()`.
  - `TestEmergeCastReductionFloorsAtZero` (which codified the incorrect
    coloured-reduction behaviour) is **replaced** by
    `TestEmergeCastReductionExceedsGenericKeepsColored`, which asserts exactly the
    case the finding asked for — the reduction exceeds the generic cost but the
    coloured pips remain payable:
    - precondition assertions: Elder Deep-Fiend in seat 0's hand, Emerge Colossus
      (mana value 10) a controlled battlefield creature;
    - the OFFER price itself is `{U}{U}` with generic floored to 0
      (`e.emergeOfferCost(0, deep, face)`);
    - funded pool `UUCC`; after the emerge cast (real decision flow: emerged mode,
      sacrifice ask, mana window, cast trigger, priority) the remaining pool is
      exactly 0U 2C — the two blue pips were actually paid, the 2 generic left;
    - a second engine with a GENERIC-ONLY pool (8×C): the `emerged` mode is **not
      offered** (the `{U}{U}` requirement gates the offer) while the plain `{8}`
      cast stays offered — so the absence is about the emerge mode specifically;
    - replay check as in the sibling tests.

## Fails without the fix

Copied the fixed `rules/emerge.go` to `.ds4/scratch/emerge.go.fixed`, reverted the
`reduceGeneric` hunk in place back to the round-1 coloured-reducing shape, ran the
emerge tests, restored byte-identically (`cmp` OK):

```
--- FAIL: TestEmergeCastReductionExceedsGenericKeepsColored (0.00s)
    emerge_test.go:182: emerge offer cost {Colored:[0 0 0 0 0 0] Generic:0 ... Sac:[{N:1 Spec:Creature ...}]}
        (ok=true), want {U}{U} with the generic floored to 0
FAIL    github.com/adams-shaun/gorge/rules      0.616s
```

(The paid-amount test with mana value 3 does not reach the coloured branch —
reduction 3 < generic 5 — so it correctly passes under both shapes; the boundary
test is the discriminator and fails at the very first assertion.)

## Gates run (real output)

```
$ go test -run 'TestEmergeCastPaysReducedCost|TestEveryRepoDeckIsFullySupported' ./rules/ 2>&1 | tail -3
ok    github.com/adams-shaun/gorge/rules    0.617s

$ go test -run 'TestEmergeCast|TestEveryRepoDeckIsFullySupported' ./rules/ 2>&1 | tail -3
ok    github.com/adams-shaun/gorge/rules    0.629s

$ go test ./internal/archtest/ 2>&1 | tail -2
ok    github.com/adams-shaun/gorge/internal/archtest    2.937s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -2
ok    github.com/adams-shaun/gorge/cmd/botbench    1.300s

$ gofmt -l rules/emerge.go rules/emerge_test.go
(no output, exit 0)

$ go run ./cmd/gentypes -check
(no output, exit 0)
```

- Botbench split did not move (as the brief predicted): Elder Deep-Fiend is in **0**
  repo deck files (re-measured, `grep -Ril 'Elder Deep-Fiend' internal/testutil/decks | wc -l` → 0),
  and no repo deck contains any of the 15 `K:Emerge` corpus cards in a way the
  default split exercises.
- `.cards` was present (symlink to the shared corpus) before the first test run, so
  no run was vacuous.
- `knownUnsupported` untouched (correct: no repo-deck card is affected).
- No Known-approximations row added, grown or needed.

## Commit

`e295ef8b` — `fix(rules): Emerge cost reduction touches only the generic component`
(`rules/emerge.go`, `rules/emerge_test.go` only).

## Issues

- None new. Pre-existing observation, out of scope per the brief: the general
  cost-reduction machinery (`costMods.apply`) was not touched; this round changes
  only the Emerge reduction helper. If a wider audit of how `costMods.apply`
  composes reductions is ever wanted, it is a separate ticket.
- `.ds4/report-t1.md` shows as modified in `git status` — pre-existing local
  modification in the worktree, not made by this round, deliberately not staged
  or committed.
