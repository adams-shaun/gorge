# uc1b correction report

## Changes and scope

On `wt/uc1`, parent `8f153c0`; original feature base `5f50cde`.
- `AGENTS.md`: replaces the false cost/payer row and adds switched and multi-target limitations (exact rows below).
- `effects/misc.go`: comment corrections only, including removal of the false claim that all real scripts have targets. No executor changes.
- `rules/counter_unlesspay_test.go`: baseline now precedes resolution; decline AND affordable-pay subtests assert pre → pre → pre+1 and the expected target zone.
- `.ds4/task-uc1-report.md`: prominent retraction of the invalid historical claims; this report supersedes them.
- `.ds4/scratch/uc1b_probe_test.go`: reproducible measurement artifact, copied temporarily into rules for probes, removed before ordinary gates. No corpus scripts included.

No changes to heads_test.go, resolution.go, copy.go, event/decision kinds or event fields. This revision moves NO heads relative to 8f153c0.

## Corrected production comment

```go
// Unlike effCopySpellAbility, the default payer is the first target's
// controller. UnlessPayer$ is NOT read here. The corpus's targeted Counter
// shapes name TargetedController or ThisTargetedController, matching that
// default; untargeted shapes fall back to c.Controller, which does NOT
// implement general payer selectors such as Player or RememberedController.
// See AGENTS.md for the unsupported cost, switched and multi-target shapes.
```

## Re-executable corpus findings (raw lines, not compiled-SA counts)

Commands below run from this worktree. Counts are deliberately scoped to raw matching lines. They differ from the brief; I have not equated these with counts from the compiled cache or guessed why another measurement differed.

```sh
grep -rE '(SP|AB|DB)\$ Counter( \||$)' .cards/cardsfolder | grep -F 'UnlessPayer$' | wc -l
# 34
grep -rE '(SP|AB|DB)\$ Counter( \||$)' .cards/cardsfolder | grep -F 'UnlessPayer$' | grep -E 'TargetType\$|ValidTgts\$|Defined\$ Targeted' | grep -oE 'UnlessPayer\$ [^|]+' | sort | uniq -c
#       9 UnlessPayer$ TargetedController
#       3 UnlessPayer$ ThisTargetedController
grep -rE '(SP|AB|DB)\$ Counter( \||$)' .cards/cardsfolder | grep -E 'UnlessCost\$ (Y|Z)( \||$)' | wc -l
# 17
grep -rE '(SP|AB|DB)\$ Counter( \||$)' .cards/cardsfolder | grep -E 'UnlessCost\$ X( \||$)' | wc -l
# 21
grep -rE '(SP|AB|DB)\$ Counter( \||$)' .cards/cardsfolder | grep -F 'UnlessSwitched$ True' | cut -d: -f1
# .cards/cardsfolder/b/brain_gorgers.txt
# .cards/cardsfolder/i/ice_cave.txt
# .cards/cardsfolder/d/dash_hopes.txt
# .cards/cardsfolder/p/phantasmagorian.txt
# .cards/cardsfolder/t/temporal_extortion.txt
grep -rE '(SP|AB|DB)\$ Counter( \||$)' .cards/cardsfolder | grep -F 'UnlessCost$' | grep -F 'TargetMax$'
```

The last command yields one line, Repulsive Mutation's DBCounter, with **TargetMin 0 / TargetMax 1**. It does NOT demonstrate a live multi-target counter. Returning before the target loop is still a latent multi-target defect.

### Original grep, run verbatim: surprising result

```sh
grep -rl "SP$ Counter" .cards/cardsfolder | xargs grep -l "UnlessPayer"
```

Actual output:
```
.cards/cardsfolder/l/logic_knot.txt
.cards/cardsfolder/d/dash_hopes.txt
.cards/cardsfolder/r/rethink.txt
.cards/cardsfolder/r/reasonable_doubt.txt
.cards/cardsfolder/r/rune_snag.txt
.cards/cardsfolder/p/pact_of_negation.txt
.cards/cardsfolder/s/spell_rupture.txt
```

`grep --version | head -1` prints `grep (GNU grep) 3.11`. Contrary to the brief, that command does not match nothing here: GNU basic regex treats this non-terminal `$` literally. But it selects FILES containing SP Counter and searches those entire files for UnlessPayer; it cannot establish that a Counter SA carries the parameter, nor examine all AB/DB Counter shapes. The old report's claimed result and its “honoured” justification are withdrawn regardless. The escaped same-line grep above is the relevant replacement.

## UnlessSwitched deferral and safety boundary

Deferred, not fixed. The ask is backwards for switched Counter shapes. Five raw files are listed above, including Ice Cave (absent from the brief's four-card list). A boolean flip alone would still offer wrong payers and substitute generic mana for sacrifice/life/discard/dynamic costs. Correcting the whole interaction needs cost and payer support outside this comment/guard correction, and `rules/resolution.go` is explicitly out of scope.

```sh
grep -riEn 'Brain Gorgers|Dash Hopes|Ice Cave|Phantasmagorian|Temporal Extortion' internal/testutil/decks
# no output, exit 1
```

Deferral is bounded to the existing repo-deck acceptance workload and does not add a new regression in uc1b (production edits are comments only). It is NOT safe for arbitrary corpus play. The regression introduced by 8f153c0 remains an explicit open concern for the controller, not an assertion of support. General payer selection also remains unsupported.

## Measurement procedure

These commands were run (using `go -C` for isolated archives, without switching this worktree):

```sh
mkdir -p /tmp/uc1b/base /tmp/uc1b/parent
git archive 5f50cde | tar -x -C /tmp/uc1b/base
git archive 8f153c0 | tar -x -C /tmp/uc1b/parent
for tree in base parent; do
  git -C /tmp/uc1b/$tree init -q
  ln -s /home/sadams/projects/gorge/.cards /tmp/uc1b/$tree/.cards
done
gofmt -w .ds4/scratch/uc1b_probe_test.go
cp .ds4/scratch/uc1b_probe_test.go rules/uc1b_probe_test.go
for tree in base parent; do
  cp .ds4/scratch/uc1b_probe_test.go /tmp/uc1b/$tree/rules/uc1b_probe_test.go
  GOMEMLIMIT=5GiB go -C /tmp/uc1b/$tree test -p=2 ./rules -run 'TestHeads|TestUC1BProbe' -v > /tmp/uc1b/$tree.log 2>&1
done
GOMEMLIMIT=5GiB go test -p=2 ./rules -run 'TestUC1BProbe|TestCounterWithSubAbilityRunsChainExactlyOnce' -v > /tmp/uc1b/current.log 2>&1
rm rules/uc1b_probe_test.go
```

For a rerun use fresh archive directories (or retain the initialized ones and skip setup). Original base TestHeads passes; parent TestHeads fails only at 2/4. Probe passes on all three trees. Full probe output below records ParseCost/Cost.Pay results, draw counts, Resolve card names, ModeChosen and immediately following events. TestHeads only prints mismatches; the probe prints all four actual/golden pairs.

### Corrected head attribution (for the controller's golden/merge comment)

Daze#95, dimir-tempo seat 1, resolves once at 2 seats and once at 4 seats, and never at 6/8. Spell Pierce resolves ZERO times at every seat count on both trees. Daze now asks `Pay 1 — don't counter`. At 2 seats the payment fails and the counter still occurs, but the new decision/ModeChosen changes the head. At 4 seats the payment SUCCEEDS: ModeChosen sequence 1907 is immediately followed by ManaAdd(-1), Counter B, player 1, sequence 1908. The target is spared; total countered moves drop from 3 to 2. Thus this is not just log churn from an unsuccessful payment. Counter totals original base → parent/current are 1→1, 3→2, 2→2, 5→5. No additional movement belongs to uc1b.

### Draw guard

Both branches measure **14 → 14 → 15** (pre-resolution, suspended, resumed). The affordable-pay branch also asserts Grizzly Bears reaches the battlefield, while decline asserts graveyard. The old 14-at-pending baseline could hide an early draw; that vacuous comparison is removed.

### Cost findings

Probe calls the real ParseCost and Cost.Pay. ExileFromGrave<1/All>, Discard<1/Hand>, DamageYou<4>, Y and Z all become Generic:1 and are payable with one colourless mana, not an empty pool. X becomes Generic:0/X:1 and pays from an empty pool. The cast-time X reachability caveat is retained from the gate's end-to-end finding; this probe measures the payment API, not that cast path.

## Corrected ledger rows (verbatim)

| `Counter` reads `UnlessCost$` and asks KModes of the first target's controller (otherwise `c.Controller`), recorded by `ModeChosen` and paid through `payMana`. Only FLOATING mana is spent: no mid-resolution tap-to-pay. **Unsupported costs silently substitute mana, rather than always declining:** `ExileFromGrave<1/All>` (Grip of Amnesia), `Discard<1/Hand>` and `DamageYou<4>` each parse as {1}, payable with one floating mana; `Y`/`Z` likewise become flat {1}, without reading their SVars. `X` parses as zero generic plus one X and pays from an empty pool; the cast-time X-choice currently prevents the affected cards reaching this end-to-end path. `UnlessPayer$` is NOT read: targeted corpus selectors coincide with the default, but untargeted selectors such as `Player`/`RememberedController` are not implemented. A no-ask host declines deterministically with a Note. | `effects/misc.go` (`effCounter`), `rules/mana.go` | M4 (cost grammar, payer selection and tap-to-pay) |
| `Counter` ignores `UnlessSwitched$ True`: paying should CAUSE the counter, but instead prevents it. This is a backwards-ask regression from the previous unconditional counter on Brain Gorgers, Dash Hopes, Ice Cave, Phantasmagorian and Temporal Extortion. Deferred, NOT safe for general corpus play: these shapes also need non-mana/dynamic costs and general payer selection, not just a flipped boolean. Repo-deck acceptance does not certify them. | `effects/misc.go` (`effCounter`) | M4 (switched costs and payer selection) |
| A paid `Counter` returns before the target loop, skipping EVERY target rather than only `Targets[0]`; one raw corpus Counter+UnlessCost SA carries `TargetMax`, but its value is **1** (Repulsive Mutation), so that occurrence is NOT evidence of a live multi-target counter. Latent multi-target limitation, deferred pending per-target asks. | `effects/misc.go` (`effCounter`) | M4 |

## Original base probe and TestHeads — actual output

```
=== RUN   TestHeads
--- PASS: TestHeads (0.55s)
=== RUN   TestUC1BProbe
    uc1b_probe_test.go:15: 2 seats: chain head 6ade7e2262bc8787, golden 6ade7e2262bc8787
    uc1b_probe_test.go:23: seats=2 resolve Daze#95 controller=1
    uc1b_probe_test.go:41: seats=2 countered=1 Daze-resolves=1 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 4 seats: chain head c8cbd9c6b4851767, golden c8cbd9c6b4851767
    uc1b_probe_test.go:23: seats=4 resolve Daze#95 controller=1
    uc1b_probe_test.go:35: seats=4 event=2101 mode={Seq:2101 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:2102 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:2103 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:2104 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=4 countered=3 Daze-resolves=1 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 6 seats: chain head baafe0b87f436bec, golden baafe0b87f436bec
    uc1b_probe_test.go:35: seats=6 event=3629 mode={Seq:3629 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:3630 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:3631 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:3632 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=6 countered=2 Daze-resolves=0 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 8 seats: chain head 700f85d871d35367, golden 700f85d871d35367
    uc1b_probe_test.go:35: seats=8 event=5995 mode={Seq:5995 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:5996 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:5997 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:5998 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=8 countered=5 Daze-resolves=0 Spell-Pierce-resolves=0
    uc1b_probe_test.go:49: cost=ExileFromGrave<1/All> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Discard<1/Hand> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=DamageYou<4> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Y parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Z parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=X parsed={Colored:[0 0 0 0 0 0] Generic:0 X:1 Tap:false Sac:[] SubCounter:[]} empty=true one=true
--- PASS: TestUC1BProbe (0.55s)
PASS
ok  	github.com/adams-shaun/gorge/rules	1.105s
```

## Immediate parent probe and TestHeads — actual output

```
=== RUN   TestHeads
    heads_test.go:268: 2 seats: chain head a95fd3b1da972438, golden 6ade7e2262bc8787 — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:268: 4 seats: chain head ba76d1bb389a3c2b, golden c8cbd9c6b4851767 — if this move is intended, update acceptanceHeads and name the cause in the commit body
--- FAIL: TestHeads (0.54s)
=== RUN   TestUC1BProbe
    uc1b_probe_test.go:15: 2 seats: chain head a95fd3b1da972438, golden 6ade7e2262bc8787
    uc1b_probe_test.go:23: seats=2 resolve Daze#95 controller=1
    uc1b_probe_test.go:35: seats=2 event=1172 mode={Seq:1172 Kind:mode_chosen Player:1 Obj:95 From:library To:library Amount:0 Step:untap Counter: Text:Pay 1 — don't counter IDs:[] Pairs:[] Secret:false} source=Daze#95
    uc1b_probe_test.go:37: following={Seq:1173 Kind:move_zone Player:0 Obj:79 From:stack To:graveyard Amount:0 Step:untap Counter: Text:countered IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:1174 Kind:move_zone Player:0 Obj:95 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:1175 Kind:priority Player:1 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=2 countered=1 Daze-resolves=1 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 4 seats: chain head ba76d1bb389a3c2b, golden c8cbd9c6b4851767
    uc1b_probe_test.go:23: seats=4 resolve Daze#95 controller=1
    uc1b_probe_test.go:35: seats=4 event=1907 mode={Seq:1907 Kind:mode_chosen Player:1 Obj:95 From:library To:library Amount:0 Step:untap Counter: Text:Pay 1 — don't counter IDs:[] Pairs:[] Secret:false} source=Daze#95
    uc1b_probe_test.go:37: following={Seq:1908 Kind:mana_add Player:1 Obj:0 From:library To:library Amount:-1 Step:untap Counter:B Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:1909 Kind:move_zone Player:0 Obj:95 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:1910 Kind:priority Player:1 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:35: seats=4 event=2124 mode={Seq:2124 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:2125 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:2126 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:2127 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=4 countered=2 Daze-resolves=1 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 6 seats: chain head baafe0b87f436bec, golden baafe0b87f436bec
    uc1b_probe_test.go:35: seats=6 event=3629 mode={Seq:3629 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:3630 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:3631 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:3632 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=6 countered=2 Daze-resolves=0 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 8 seats: chain head 700f85d871d35367, golden 700f85d871d35367
    uc1b_probe_test.go:35: seats=8 event=5995 mode={Seq:5995 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:5996 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:5997 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:5998 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=8 countered=5 Daze-resolves=0 Spell-Pierce-resolves=0
    uc1b_probe_test.go:49: cost=ExileFromGrave<1/All> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Discard<1/Hand> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=DamageYou<4> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Y parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Z parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=X parsed={Colored:[0 0 0 0 0 0] Generic:0 X:1 Tap:false Sac:[] SubCounter:[]} empty=true one=true
--- PASS: TestUC1BProbe (0.53s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	1.081s
FAIL
```

## Current probe and fixed guard — actual output

```
=== RUN   TestCounterWithSubAbilityRunsChainExactlyOnce
=== RUN   TestCounterWithSubAbilityRunsChainExactlyOnce/decline
    counter_unlesspay_test.go:179: draws pre=14 suspended=14 resumed=15
=== RUN   TestCounterWithSubAbilityRunsChainExactlyOnce/pay
    counter_unlesspay_test.go:179: draws pre=14 suspended=14 resumed=15
--- PASS: TestCounterWithSubAbilityRunsChainExactlyOnce (0.24s)
    --- PASS: TestCounterWithSubAbilityRunsChainExactlyOnce/decline (0.00s)
    --- PASS: TestCounterWithSubAbilityRunsChainExactlyOnce/pay (0.00s)
=== RUN   TestUC1BProbe
    uc1b_probe_test.go:15: 2 seats: chain head a95fd3b1da972438, golden 6ade7e2262bc8787
    uc1b_probe_test.go:23: seats=2 resolve Daze#95 controller=1
    uc1b_probe_test.go:35: seats=2 event=1172 mode={Seq:1172 Kind:mode_chosen Player:1 Obj:95 From:library To:library Amount:0 Step:untap Counter: Text:Pay 1 — don't counter IDs:[] Pairs:[] Secret:false} source=Daze#95
    uc1b_probe_test.go:37: following={Seq:1173 Kind:move_zone Player:0 Obj:79 From:stack To:graveyard Amount:0 Step:untap Counter: Text:countered IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:1174 Kind:move_zone Player:0 Obj:95 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:1175 Kind:priority Player:1 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=2 countered=1 Daze-resolves=1 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 4 seats: chain head ba76d1bb389a3c2b, golden c8cbd9c6b4851767
    uc1b_probe_test.go:23: seats=4 resolve Daze#95 controller=1
    uc1b_probe_test.go:35: seats=4 event=1907 mode={Seq:1907 Kind:mode_chosen Player:1 Obj:95 From:library To:library Amount:0 Step:untap Counter: Text:Pay 1 — don't counter IDs:[] Pairs:[] Secret:false} source=Daze#95
    uc1b_probe_test.go:37: following={Seq:1908 Kind:mana_add Player:1 Obj:0 From:library To:library Amount:-1 Step:untap Counter:B Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:1909 Kind:move_zone Player:0 Obj:95 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:1910 Kind:priority Player:1 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:35: seats=4 event=2124 mode={Seq:2124 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:2125 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:2126 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:2127 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=4 countered=2 Daze-resolves=1 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 6 seats: chain head baafe0b87f436bec, golden baafe0b87f436bec
    uc1b_probe_test.go:35: seats=6 event=3629 mode={Seq:3629 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:3630 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:3631 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:3632 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=6 countered=2 Daze-resolves=0 Spell-Pierce-resolves=0
    uc1b_probe_test.go:15: 8 seats: chain head 700f85d871d35367, golden 700f85d871d35367
    uc1b_probe_test.go:35: seats=8 event=5995 mode={Seq:5995 Kind:mode_chosen Player:2 Obj:179 From:library To:library Amount:0 Step:untap Counter: Text:Exile target creature with power or toughness 1 or less. IDs:[] Pairs:[] Secret:false} source=Warping Wail#179
    uc1b_probe_test.go:37: following={Seq:5996 Kind:move_zone Player:0 Obj:179 From:stack To:graveyard Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:5997 Kind:priority Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:37: following={Seq:5998 Kind:decision_ask Player:2 Obj:0 From:library To:library Amount:0 Step:untap Counter: Text:priority IDs:[] Pairs:[] Secret:false}
    uc1b_probe_test.go:41: seats=8 countered=5 Daze-resolves=0 Spell-Pierce-resolves=0
    uc1b_probe_test.go:49: cost=ExileFromGrave<1/All> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Discard<1/Hand> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=DamageYou<4> parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Y parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=Z parsed={Colored:[0 0 0 0 0 0] Generic:1 X:0 Tap:false Sac:[] SubCounter:[]} empty=false one=true
    uc1b_probe_test.go:49: cost=X parsed={Colored:[0 0 0 0 0 0] Generic:0 X:1 Tap:false Sac:[] SubCounter:[]} empty=true one=true
--- PASS: TestUC1BProbe (0.53s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.776s
```

## Ordinary gates — actual output

```sh
GOMEMLIMIT=5GiB go test -p=2 ./effects ./rules
```
```
ok  	github.com/adams-shaun/gorge/effects	1.722s
--- FAIL: TestHeads (0.56s)
    heads_test.go:268: 2 seats: chain head a95fd3b1da972438, golden 6ade7e2262bc8787 — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:268: 4 seats: chain head ba76d1bb389a3c2b, golden c8cbd9c6b4851767 — if this move is intended, update acceptanceHeads and name the cause in the commit body
FAIL
FAIL	github.com/adams-shaun/gorge/rules	5.795s
FAIL
```
```sh
GOMEMLIMIT=5GiB go test -p=2 ./rules -run 'TestHeads|TestRepoDeckGamesReplayExactly|TestEveryRepoDeckIsFullySupported' -v
```
```
=== RUN   TestEveryRepoDeckIsFullySupported
    acceptance_test.go:113: ratchet: 0 of 423 distinct cards across the repo decks are not fully supported
--- PASS: TestEveryRepoDeckIsFullySupported (0.25s)
=== RUN   TestRepoDeckGamesReplayExactly
    acceptance_test.go:316: seed 0: 1169 intents, chain c75c959c70be64e7, replay OK
    acceptance_test.go:316: seed 1: 834 intents, chain e1ef981fe369f1db, replay OK
    acceptance_test.go:316: seed 2: 1195 intents, chain 3f93aa007a53835d, replay OK
    acceptance_test.go:316: seed 3: 983 intents, chain 0f8aa8436669b793, replay OK
    acceptance_test.go:316: seed 4: 797 intents, chain cc262c5ece3b0eb2, replay OK
--- PASS: TestRepoDeckGamesReplayExactly (0.41s)
=== RUN   TestHeads
    heads_test.go:268: 2 seats: chain head a95fd3b1da972438, golden 6ade7e2262bc8787 — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:268: 4 seats: chain head ba76d1bb389a3c2b, golden c8cbd9c6b4851767 — if this move is intended, update acceptanceHeads and name the cause in the commit body
--- FAIL: TestHeads (0.54s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	1.210s
FAIL
```
```sh
go run ./cmd/gentypes -check
```
```
(no output; exit 0)
```
```sh
GOMEMLIMIT=5GiB go vet -p=2 ./effects ./rules
```
```
(no output; exit 0)
```
```sh
gofmt -l effects rules
```
```
(no output; exit 0)
```

Effects passes; rules fails solely at TestHeads (two expected mismatches inherited from parent). Focused acceptance: 2 tests pass, 1 fails (TestHeads); all five replay seeds pass and ratchet is 0/423. Gentypes, vet and formatting are clean. No golden regeneration was attempted.
