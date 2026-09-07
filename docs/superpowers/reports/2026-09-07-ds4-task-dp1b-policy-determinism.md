# dp1b — deterministic decisions and hand retention

Base: main 587f31d, existing wt/dp1b; no merge/rebase/history operations. Corpus resolves to `/home/sadams/projects/gorge/.cards`. No dispatch report filename was specified, so this report follows the existing reports directory convention.

## Changes

- `botpolicy/cast.go`: extract cardWorth and commandTax without changing main's CMC-scaled CR1 arithmetic; preserve Produces and all adapter fields.
- `botpolicy/policy.go`, `trigger.go`: reuse dp1's deterministic trigger order, optional acceptance and sacrifice/exile ranking. Commander-zone uses shared CMC-scaled penalty and the same >=0 boundary as casting. Discard and mulligan bottoming use a SEPARATE hand-retention ranking: protect basics and zero-CMC noncreatures, otherwise discard greatest printed CMC first, without any creature premium. Ties use option index and returned sets are index-sorted.
- `botpolicy/dp1_test.go`: import dp1 coverage, correct commander CMC fixture, restrict cast-worth list tests to sacrifice/exile.
- `botpolicy/discard_test.go`: basic/nonbasic land protection, cheap/expensive spells, expensive creatures, class-neutral ties, multiple picks, only lands, seed independence, and commander mana-axis/zero-boundary cases.
- `botpolicy/policy_test.go`: replace the factless discard placeholder with a real land/spell fixture; update bottoming and deterministic trigger expectations, retain mulligan RNG coverage.
- `seat/bot_test.go`: reuse dp1's RNG-contract test updates; no adapter changes. dp1's rules/testbot_test.go comment-only change was deliberately omitted.

No new Card field: Basic identifies basics; zero-CMC noncreatures are conservatively land-like, covering nonbasics without claiming exact land identification. Castable includes hand lands and indicates zone eligibility, NOT current affordability. No existing fact supplies reliable future available mana or land count, so this is a cheap-play/mana-development heuristic, not an affordability solver. It also retains free artifacts, can overretain lands when flooded, and overprices alternative-cost spells (Force of Will, delve). Those limits are explicit rather than hidden behind a false Castable check. No protocol regeneration or new adapter parity obligation.

## Results and concerns

Constructed seat-0: baseline 68.5% -> 67.5% (-1pp; overlapping CIs). A controlled ablation restoring dp1's cardWorth ranking for discard/bottom on this SAME base measured 67.0%, so the discard fix adds 0.5pp to that resolution, not evidence of a significant strength gain. Commander seat-0: 94.5% -> 95.0%. Do not compare these directly to dp1's different base as an isolated effect.

Discard first-option: 100% -> 18.2% constructed, 25.1% commander. Commander-zone: 100% -> 92.2%. No inference from blockers/attackers rows. Trigger ordering need not always select index zero (commander is 82.3%); it is deterministic across seeds.

Remaining production RNG outside legacy: ONLY `policy.go` KMulligan `r.IntN(3)` keep/mulligan coin. Kept as dp1 requested; benchmark does not enable mulligans. No shuffle remains.

Optional triggers still always accepted; effect/library counts unavailable, so an optional draw can deck out. Commander leave does not guarantee useful access from exile/library/graveyard. These are known heuristics, not rules-engine changes.

Deviations/open gates: race NOT run because the later task-system instruction explicitly forbids it and delegates it to controller. Full deck-pair matrix/coin-flip pair share was NOT measured (the prescribed single-pair invocations do not report it); controller follow-up remains. No claim about matrix strength or pair-share change. Build/vet and affected package tests were run, never go test ./.... Expected TestHeads failures remain; controller owns golden regeneration. Legacy and heads literals have zero diff.

## Measured chain attribution

Temporary instrumentation ran main's original Decide alongside the new policy with independently identically seeded RNGs, stopping comparison per seat at the first differing intent. No instrumentation remains. First divergence on ALL FOUR head games was mulligan bottoming for seat 1:

- 2: Island -> Baleful Strix.
- 4: Baleful Strix -> Gurmag Angler.
- 6: Spell Pierce -> Dismember.
- 8: Underground Sea -> Force of Will.

This directly attributes the first changed chain event to hand retention, not card primitive/replay changes. Later trigger/discard decisions can also differ; this is first-divergence evidence, not a claim that bottoming alone explains every subsequent event. Focused coverage ratchet and five exact replays PASS; full rules fails only TestHeads. Raw expected/actual pairs and instrumentation follow.

## Gate output

### `go test ./internal/archtest/ ./botpolicy/ ./seat/; go build ./...; go vet ./...`

```text
ok  	github.com/adams-shaun/gorge/internal/archtest	0.135s
ok  	github.com/adams-shaun/gorge/botpolicy	0.006s
ok  	github.com/adams-shaun/gorge/seat	(cached)
```

### `go test ./botpolicy/ ./seat/ (after final test edit)`

```text
ok  	github.com/adams-shaun/gorge/botpolicy	0.005s
ok  	github.com/adams-shaun/gorge/seat	(cached)
```

### `GOMEMLIMIT=5GiB go test ./rules -run 'TestHeads|TestRepoDeckGamesReplayExactly|TestEveryRepoDeckIsFullySupported' -v`

```text
=== RUN   TestEveryRepoDeckIsFullySupported
    acceptance_test.go:113: ratchet: 0 of 423 distinct cards across the repo decks are not fully supported
--- PASS: TestEveryRepoDeckIsFullySupported (0.28s)
=== RUN   TestRepoDeckGamesReplayExactly
    acceptance_test.go:316: seed 0: 1257 intents, chain 92bde3f9b5e24daa, replay OK
    acceptance_test.go:316: seed 1: 828 intents, chain 6d8a0ae8c166a0a7, replay OK
    acceptance_test.go:316: seed 2: 1200 intents, chain b379308a6f33cc6e, replay OK
    acceptance_test.go:316: seed 3: 998 intents, chain dcd71c0619f2d408, replay OK
    acceptance_test.go:316: seed 4: 802 intents, chain b5a2306e82f2ffa7, replay OK
--- PASS: TestRepoDeckGamesReplayExactly (0.42s)
=== RUN   TestHeads
    heads_test.go:188: 2 seats: chain head b984373baa683987, golden e6cfa5853c674b1b — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:188: 4 seats: chain head 7d178f2232d5e1e7, golden 082262b548424173 — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:188: 6 seats: chain head 78c5c443d7e290a6, golden 761486a7d9f76753 — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:188: 8 seats: chain head a5ec8770907c1496, golden 968be0bdc1f6a43b — if this move is intended, update acceptanceHeads and name the cause in the commit body
--- FAIL: TestHeads (0.60s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	1.301s
FAIL
```

### `GOMEMLIMIT=5GiB go test ./rules/`

```text
--- FAIL: TestHeads (0.74s)
    heads_test.go:188: 2 seats: chain head b984373baa683987, golden e6cfa5853c674b1b — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:188: 4 seats: chain head 7d178f2232d5e1e7, golden 082262b548424173 — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:188: 6 seats: chain head 78c5c443d7e290a6, golden 761486a7d9f76753 — if this move is intended, update acceptanceHeads and name the cause in the commit body
    heads_test.go:188: 8 seats: chain head a5ec8770907c1496, golden 968be0bdc1f6a43b — if this move is intended, update acceptanceHeads and name the cause in the commit body
FAIL
FAIL	github.com/adams-shaun/gorge/rules	5.316s
FAIL
```

### `GOMEMLIMIT=5GiB go test ./rules -run '^TestHeads$' -v (temporary first-divergence instrumentation)`

```text
=== RUN   TestHeads
ATTR seats=2 kind=mulligan old=[0] new=[5] options=[{Index:0 Kind:bottom Label:Island Obj:65 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:1 Kind:bottom Label:Delver of Secrets Obj:82 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:2 Kind:bottom Label:Polluted Delta Obj:70 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:3 Kind:bottom Label:Polluted Delta Obj:73 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:4 Kind:bottom Label:Thoughtseize Obj:114 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:5 Kind:bottom Label:Baleful Strix Obj:85 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:6 Kind:bottom Label:Delver of Secrets Obj:81 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0}]
    heads_test.go:188: 2 seats: chain head b984373baa683987, golden e6cfa5853c674b1b — if this move is intended, update acceptanceHeads and name the cause in the commit body
ATTR seats=4 kind=mulligan old=[0] new=[2] options=[{Index:0 Kind:bottom Label:Baleful Strix Obj:84 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:1 Kind:bottom Label:Underground Sea Obj:63 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:2 Kind:bottom Label:Gurmag Angler Obj:86 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:3 Kind:bottom Label:Delver of Secrets Obj:82 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:4 Kind:bottom Label:Cabal Therapy Obj:115 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:5 Kind:bottom Label:Force of Will Obj:93 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:6 Kind:bottom Label:Daze Obj:95 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0}]
    heads_test.go:188: 4 seats: chain head 7d178f2232d5e1e7, golden 082262b548424173 — if this move is intended, update acceptanceHeads and name the cause in the commit body
ATTR seats=6 kind=mulligan old=[0] new=[1] options=[{Index:0 Kind:bottom Label:Spell Pierce Obj:119 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:1 Kind:bottom Label:Dismember Obj:118 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:2 Kind:bottom Label:Brainstorm Obj:100 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:3 Kind:bottom Label:Fatal Push Obj:110 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:4 Kind:bottom Label:Delver of Secrets Obj:82 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:5 Kind:bottom Label:Swamp Obj:69 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:6 Kind:bottom Label:Brainstorm Obj:101 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0}]
    heads_test.go:188: 6 seats: chain head 78c5c443d7e290a6, golden 761486a7d9f76753 — if this move is intended, update acceptanceHeads and name the cause in the commit body
ATTR seats=8 kind=mulligan old=[0] new=[1] options=[{Index:0 Kind:bottom Label:Underground Sea Obj:61 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:1 Kind:bottom Label:Force of Will Obj:93 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:2 Kind:bottom Label:Spell Pierce Obj:120 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:3 Kind:bottom Label:Polluted Delta Obj:72 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:4 Kind:bottom Label:Force of Will Obj:94 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:5 Kind:bottom Label:Daze Obj:95 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0} {Index:6 Kind:bottom Label:Thoughtseize Obj:113 Player:1 Attacker:0 AltCostIndex:0 Mode: Amount:0 Ability:0}]
    heads_test.go:188: 8 seats: chain head a5ec8770907c1496, golden 968be0bdc1f6a43b — if this move is intended, update acceptanceHeads and name the cause in the commit body
--- FAIL: TestHeads (0.61s)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.616s
FAIL
```

Build and vet emitted no output and succeeded. `git diff --check` and `git diff --exit-code -- botpolicy/legacy.go rules/heads_test.go` emitted no output and succeeded.

### before — `go run ./cmd/botbench -decision-stats -games 200 -seed 0 -format constructed`

Game-by-game lines omitted; summary and entire table verbatim:

```text
games played: 200
A wins: 101  B wins: 99  draws: 0  (same policy "bot" on both sides: the split is ~50% by construction; read the seat split below)
A win rate: 50.5%  95% CI [43.6%, 57.4%] (normal approximation to the binomial)
seat 0 wins: 137  seat 1 wins: 63  seat 0 win rate: 68.5%  95% CI [62.1%, 74.9%] (normal approximation to the binomial)
mean turns per game: 17.0

decision stats (200 games):
kind              count  mean/game  mean opts  singleton%  first-option%
attackers         1695   8.47       1.97       41.7%       77.6%
blockers          272    1.36       1.82       52.9%       15.1%
choose/discard    209    1.04       8.11       0.0%        100.0%
choose/exile      163    0.81       5.22       2.5%        100.0%
choose/name       106    0.53       8.25       0.0%        100.0%
choose/number     84     0.42       13.00      0.0%        100.0%
choose/sacrifice  83     0.41       1.76       56.6%       100.0%
choose/type       35     0.17       9.00       0.0%        100.0%
priority/ability  1833   9.16       5.91       0.0%        46.9%
priority/cast     3476   17.38      5.75       0.0%        70.6%
priority/land     2163   10.81      6.16       0.0%        55.8%
priority/pass     58964  294.82     2.99       0.0%        73.3%
priority/tap      7987   39.94      6.05       0.0%        36.7%
target            2085   10.43      5.78       11.6%       36.1%
trigger_optional  1175   5.88       2.00       0.0%        48.7%
trigger_order     225    1.12       2.13       0.0%        45.3%
```

### before — `go run ./cmd/botbench -decision-stats -games 200 -seed 0 -format commander`

Game-by-game lines omitted; summary and entire table verbatim:

```text
games played: 200
A wins: 101  B wins: 99  draws: 0  (same policy "bot" on both sides: the split is ~50% by construction; read the seat split below)
A win rate: 50.5%  95% CI [43.6%, 57.4%] (normal approximation to the binomial)
seat 0 wins: 189  seat 1 wins: 11  seat 0 win rate: 94.5%  95% CI [91.3%, 97.7%] (normal approximation to the binomial)
mean turns per game: 25.0

decision stats (200 games):
kind              count  mean/game  mean opts  singleton%  first-option%
attackers         3586   17.93      3.97       25.7%       48.1%
blockers          1514   7.57       4.63       28.3%       13.0%
choose/discard    197    0.98       8.19       0.0%        100.0%
choose/name       52     0.26       19.44      0.0%        100.0%
choose/sacrifice  322    1.61       4.57       0.0%        100.0%
choose/type       44     0.22       18.00      0.0%        100.0%
choose/x          138    0.69       1.55       73.9%       73.9%
commander_zone    264    1.32       2.00       0.0%        100.0%
modes             22     0.11       3.00       0.0%        100.0%
priority/ability  1782   8.91       6.59       0.0%        44.3%
priority/cast     6501   32.51      7.35       0.0%        54.7%
priority/land     3240   16.20      8.47       0.0%        45.4%
priority/pass     94025  470.12     4.40       0.0%        69.2%
priority/tap      21846  109.23     7.92       0.0%        42.5%
target            1723   8.62       9.59       7.8%        31.6%
trigger_optional  477    2.38       2.00       0.0%        47.2%
trigger_order     732    3.66       2.13       0.0%        50.3%
```

### after — `go run ./cmd/botbench -decision-stats -games 200 -seed 0 -format constructed`

Game-by-game lines omitted; summary and entire table verbatim:

```text
games played: 200
A wins: 103  B wins: 97  draws: 0  (same policy "bot" on both sides: the split is ~50% by construction; read the seat split below)
A win rate: 51.5%  95% CI [44.6%, 58.4%] (normal approximation to the binomial)
seat 0 wins: 135  seat 1 wins: 65  seat 0 win rate: 67.5%  95% CI [61.0%, 74.0%] (normal approximation to the binomial)
mean turns per game: 16.9

decision stats (200 games):
kind              count  mean/game  mean opts  singleton%  first-option%
attackers         1693   8.46       1.94       43.5%       77.4%
blockers          275    1.38       1.83       53.1%       18.5%
choose/discard    209    1.04       8.13       0.0%        18.2%
choose/exile      151    0.76       5.32       2.6%        70.2%
choose/name       107    0.54       8.34       0.0%        100.0%
choose/number     81     0.41       13.00      0.0%        100.0%
choose/sacrifice  76     0.38       1.80       52.6%       84.2%
choose/type       35     0.17       9.00       0.0%        100.0%
priority/ability  1819   9.10       5.92       0.0%        47.1%
priority/cast     3482   17.41      5.76       0.0%        70.5%
priority/land     2171   10.86      6.19       0.0%        56.1%
priority/pass     59844  299.22     3.03       0.0%        72.8%
priority/tap      7893   39.47      6.04       0.0%        36.9%
target            2048   10.24      5.76       11.4%       36.6%
trigger_optional  1174   5.87       2.00       0.0%        100.0%
trigger_order     229    1.15       2.16       0.0%        100.0%
```

### after — `go run ./cmd/botbench -decision-stats -games 200 -seed 0 -format commander`

Game-by-game lines omitted; summary and entire table verbatim:

```text
games played: 200
A wins: 104  B wins: 96  draws: 0  (same policy "bot" on both sides: the split is ~50% by construction; read the seat split below)
A win rate: 52.0%  95% CI [45.1%, 58.9%] (normal approximation to the binomial)
seat 0 wins: 190  seat 1 wins: 10  seat 0 win rate: 95.0%  95% CI [92.0%, 98.0%] (normal approximation to the binomial)
mean turns per game: 24.7

decision stats (200 games):
kind              count  mean/game  mean opts  singleton%  first-option%
attackers         3519   17.59      3.71       26.1%       48.8%
blockers          1548   7.74       4.74       28.1%       13.6%
choose/discard    187    0.94       8.14       0.0%        25.1%
choose/name       52     0.26       19.35      0.0%        100.0%
choose/sacrifice  302    1.51       4.21       0.0%        75.2%
choose/type       44     0.22       18.00      0.0%        100.0%
choose/x          135    0.68       1.41       76.3%       76.3%
commander_zone    270    1.35       2.00       0.0%        92.2%
modes             22     0.11       3.00       0.0%        100.0%
priority/ability  1246   6.23       5.54       0.0%        49.8%
priority/cast     6546   32.73      7.29       0.0%        53.1%
priority/land     3193   15.96      8.24       0.0%        45.6%
priority/pass     92778  463.89     4.28       0.0%        68.2%
priority/tap      20353  101.77     7.37       0.0%        42.1%
target            1658   8.29       9.35       7.9%        29.2%
trigger_optional  479    2.40       2.00       0.0%        100.0%
trigger_order     781    3.90       2.12       0.0%        82.3%
```

### Determinism

Repeated the exact after constructed invocation, then `cmp /tmp/dp1b-after-constructed /tmp/dp1b-repeat`: exit 0, no output. Both complete outputs SHA256:

```text
d4c232aba88117da9e9f823efcc99aa90ee04e3eb6192b1ab59396bcdcd1e0ca
```

### dp1 ranking ablation

Temporarily routed discard/bottom to chooseWorst, ran the same constructed command, then restored final source. Summary:

```text
seat 0 wins: 134  seat 1 wins: 66  seat 0 win rate: 67.0%  95% CI [60.5%, 73.5%] (normal approximation to the binomial)
choose/discard    213    1.06       8.12       0.0%        30.0%
```
