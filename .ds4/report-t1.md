# Verify and pin — unless/attack windows and multi-ability mana sources

**Ticket:** `agent-20260922T201246Z-000e743d`
**Outcome:** the brief's premise **holds** — the defect (a payment window
dropping a permanent with several free mana abilities) is already fixed on
`main`, and the existing regression tests genuinely pin it, on the real corpus
and with their own precondition assertions. They FAIL when the fix is reverted
(both windows). **No production code was changed and no new test was added:**
the tests the brief names already prove the behaviour, and the brief says not
to duplicate a test that already does.

`.cards` exists in this worktree as a symlink to
`/home/sadams/projects/gorge/.cards`, so the corpus-backed tests ran rather
than silently skipping (proven below with `-count=1 -v`: `--- PASS`, not
`--- SKIP`).

## Premise check (the brief's numbers)

| brief says | measured | verdict |
|---|---|---|
| fix commit `bbc863e1` exists | `bbc863e1 fix(rules): let unless-cost payments tap alternatives and gate non-mana parts` | holds |
| `bbc863e1` is on current main | `git merge-base --is-ancestor bbc863e1 HEAD` → yes | holds |
| current `HEAD` is `ab2d4b63` | actual `HEAD` = `f8e330c3`; `ab2d4b63` is an ancestor of it | **stale** — main advanced past `ab2d4b63`, but the fix is still an ancestor, so the substance holds |
| `windowManaUnits` retains each priceable ability as an `alts` entry | confirmed, `rules/mana_available.go:164-206` | holds |
| `askUnlessMana`/`answerUnlessMana` offer/resolve each alternative | confirmed, `rules/unless_payment.go:513-615` | holds |
| `attackManaSources` projects each alternative | confirmed, `rules/attack_cost.go:596-616` | holds |
| tests `TestUnlessCostPayableRealDualLandAlternatives` / `TestCounterDazePaysFromRealDualLand` exist | confirmed, `rules/unless_tap_dual_test.go:27,68` | holds |

At the time I measured, `HEAD` == `main` (`git rev-list --left-right --count
main...HEAD` → `0 0`, both `f8e330c3`), so this verification is against the
main of the dispatch. While I worked, `main` advanced 5 commits to
`c4560130` (the Bare `K:Vanishing` work); my branch is therefore 5 behind / 1
ahead, which the controller integrates. Those 5 commits touch only
`cards/kw_vanishing.go`, `cards/kw_vanishing_bare_test.go`,
`rules/vanishing_oot_test.go` and a report file — **none touch
`rules/mana_available.go`, `rules/unless_payment.go` or `rules/attack_cost.go`**
(`git diff --name-only f8e330c3..main`), so the verification below holds at
current main unchanged. I did not rebase (forbidden in this repo).

## What I verified, per source file

- **`rules/mana_available.go` — `windowManaUnits`.** It no longer drops a
  permanent with several free abilities: each priceable, free-cost,
  non-`RestrictValid$`, deterministically-priced ability is appended as one
  `windowManaAlt`, and a permanent keeps a unit (`freeCount`, `alts`) as long
  as at least one alt survives. The doc comment states this explicitly.
- **`rules/unless_payment.go` — `askUnlessMana` / `answerUnlessMana`.** The
  unless window emits one option per alt (each labelled with its production,
  e.g. "Tap Volcanic Island for {U}"), safe options first; the answer resolves
  `src.alts[ai].ma` directly (no nested colour ask), which is exactly what the
  end-to-end test exercises.
- **`rules/unless_payment.go` — `UnlessCostPayable`.** Affordability now
  searches `unlessManaReachable` over one alternative per source (`units[i].alts`)
  rather than a wrong per-source sum, so a one-pip `{U}`/`{R}` tax is payable
  by a dual but `{U}{R}` is not.
- **`rules/attack_cost.go` — `attackManaSources`.** Iterates
  `windowManaUnits` and emits one `attackManaSource` per alt, so a multi-ability
  dual pays an attack prop (widening landed in `89c77778a`, which is also on
  main and is covered by `rules/attackprop_window_test.go`).

## Gates — real output

The brief's named command plus its two sibling unless tests, run with
`-count=1 -v` once so the output proves the corpus tests executed (a cached or
missing-corpus run cannot print `--- PASS` for each):

```
$ go test -count=1 -v -run 'TestUnlessCostPayableRealDualLandAlternatives|TestCounterDazePaysFromRealDualLand|TestUnlessCostSacComponentUnpayableNotOffered|TestUnlessWindowOptionsAreBotLegal' ./rules/
=== RUN   TestUnlessCostPayableRealDualLandAlternatives
--- PASS: TestUnlessCostPayableRealDualLandAlternatives (0.63s)
=== RUN   TestCounterDazePaysFromRealDualLand
--- PASS: TestCounterDazePaysFromRealDualLand (0.00s)
=== RUN   TestUnlessCostSacComponentUnpayableNotOffered
--- PASS: TestUnlessCostSacComponentUnpayableNotOffered (0.00s)
=== RUN   TestUnlessWindowOptionsAreBotLegal
--- PASS: TestUnlessWindowOptionsAreBotLegal (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.688s
```

The attack-window siblings (green):

```
$ go test -run 'TestAttackPropPaysWithAMultiAbilityManaSource|TestAttackPropPaysWithAMultiAbilityChoiceSource|TestAttackPropSingleDualLandCannotStretchToTwo|TestAttackPropPaysTheSelectedAlternative' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.687s
```

The two mandatory behaviour goldens outside `rules/`:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.812s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.198s
```

No head/ratchet movement: no production file was modified, so there is nothing
for `TestHeads` or `knownUnsupported` to move.

## Fails without the fix

I copied each non-test file to `.ds4/scratch/`, reverted the fix's hunk in the
real file, ran the pinning tests, confirmed they FAIL, then restored the file
byte-identically (`cmp` against the scratch copy → identical; `git status`
clean). Two independent reverts:

**(1) unless window — add back the old `len(free) != 1` drop in
`windowManaUnits` (`rules/mana_available.go`):**

```
--- FAIL: TestUnlessCostPayableRealDualLandAlternatives (0.59s)
    unless_tap_dual_test.go:46: Volcanic Island window membership = [], want one unit with two alts
--- FAIL: TestCounterDazePaysFromRealDualLand (0.00s)
    unless_tap_dual_test.go:88: Daze did not offer a payable pay/decline election: [... Kind:mode Label:Don't pay ...]
--- FAIL: TestUnlessWindowOptionsAreBotLegal (0.00s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.636s
```

This is the exact reported symptom: the dual is dropped from the window, the
pay election is not offered, Daze counters the creature.

**(2) attack window — add back the pre-`89c77778a` `freeCount != 1 || len(alts) != 1`
guard in `attackManaSources` (`rules/attack_cost.go`):**

```
--- FAIL: TestAttackPropPaysTheSelectedAlternative (0.67s)
    attackprop_altselect_test.go:62: precondition: attackManaSources(1) = [], want two alts with units 1 and 2
--- FAIL: TestAttackPropPaysWithAMultiAbilityManaSource (0.00s)
    attackprop_window_test.go:83: precondition: attackManaSources(1) = 0, want 2 (one option per ability)
--- FAIL: TestAttackPropSingleDualLandCannotStretchToTwo (0.00s)
    attackprop_window_test.go:238: precondition: attackManaSources(1) = 0, want 2 alternatives
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.731s
```

Both files restored byte-identically (`cmp` succeeded) and the working tree is
clean of those edits.

## Deviations from the brief

- The brief names the exact command
  `go test -run 'TestUnlessCostPayableRealDualLandAlternatives|TestCounterDazePaysFromRealDualLand' ./rules/`.
  I ran that plus its two sibling unless tests in the same file, with
  `-count=1 -v`, so the output proves the tests EXECUTED rather than skipped
  on a missing corpus. Same package, one invocation, no budget impact.
- No new test was added; the brief's own instruction is not to duplicate a
  test that already proves the behaviour, and both windows are already pinned
  (unless on the real corpus; attack on a synthetic land whose two intrinsics
  are a Volcanic Island's exact shape — see Issues).
- No production change, per the brief.

## Issues

- **`windowManaUnit.freeCount` is production-dead and its doc comment is
  stale.** After `89c77778a` widened the attack window, no production consumer
  reads `freeCount`; the only reader in the repo is a test precondition
  (`rules/attackprop_window_test.go:76-77`). The field's doc comment
  (`rules/mana_available.go:127`) still says a consumer "can require
  `freeCount == 1 && len(alts) == 1`, reproducing the pre-alternatives
  membership exactly" — a guard nothing applies. Not a behaviour defect; it is
  dead state plus a misleading comment. Cleanup options: delete the field and
  rewrite the test precondition to assert `len(n) == 1 && len(n[0].alts) == 2`,
  or keep it and correct the comment to say it is test-visible only. Low
  priority.
- **Attack-window multi-ability coverage is synthetic, not real-corpus.** The
  unless window is pinned against real corpus cards (Volcanic Island, Daze,
  Kardur's Vicious Return), but `rules/attackprop_window_test.go` pins the
  attack window with inline `dualFreeLandScript` ("Cost T / Produced U" +
  "Cost T / Produced R"), which is structurally identical to Volcanic Island's
  two intrinsics. This is a coverage-shape asymmetry, not a defect; the
  synthetic card cannot drift from the corpus the way a real card can, but a
  future `FORGE_REF` change is not exercised by it. If the operator wants
  real-corpus parity for the attack prop, the narrow follow-up is one test in
  a NEW file using `testutil.CorpusRegistry` + Volcanic Island + a real
  CantAttackUnless carrier.
- **No CR-lane gap identified.** The verified behaviour is payment-window
  machinery (CR 601.2g/h and CR 508.1d-adjacent), and the conformance lane does
  not appear to exercise multi-ability mana sources; I did not write a CR-lane
  test because the brief did not ask for one and scoped this to verification.
  If a CR citation is wanted, CR 601.2g (a mana ability is activated one at a
  time, each yielding its own production) is the rule the tests enforce.

## Open concerns

None blocking. Two discrepancies from the brief, both non-substantive: the
stale `HEAD` value (`ab2d4b63` vs the actual `f8e330c3` at dispatch) and the
concurrent `main` advance during the round (5 unrelated Vanishing commits).
The fix `bbc863e1` is an ancestor of every one of those states, the verified
surface is untouched by the advance, and all tests pass at the dispatch's
main. My one commit (`1e6ecb73`) changes only `.ds4/report-t1.md`; no
production file is in it.
