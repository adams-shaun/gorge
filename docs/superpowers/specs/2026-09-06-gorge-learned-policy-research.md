# What a real learned policy for gorge would have to be — research, R1

**Status: research. No production code was written for this document.**
Measured at worktree `wt/r1`, base commit `e0a8f6a`, corpus pin `FORGE_REF`
in the Makefile, on this box (AMD Ryzen 9 9950X, 16C/32T, 58 GB, no GPU).
Every number below is either labelled **measured** with the probe that
produced it, or labelled **estimate**.

---

## 0. Headline

**Do not build a learned policy yet, and do not build a determinized search
agent as the alternative either — both are multi-week projects whose first
naive version is, measured on this engine, far worse than the four-line
heuristic that already exists (a 1-ply search seat scores 16.5-23.5% against
the plain bot, §5.3).** Spend the next five engineer-days instead on: (a)
making `botbench` a *matrix* over deck pairs at N≥4000, because every policy
number this project has ever quoted comes from one deck pair at an N whose
interval straddles the effect; and (b) pointing the fitted weight table (B4)
at **spell selection at priority**, which I measured to be the one sensitive
surface left — degrading only "which spell to cast, or none" costs 16.8pp
[15.4, 17.9] at N=4000, against 4.3pp for targeting and 3.8pp for blocking
(§4.2). The bot workstream has spent B2 and B3 on the two *least* sensitive
surfaces.

If a hand-fitted score over spell selection cannot clear +3pp at N=4000 on
that matrix, the band above the trivial rule is narrow and no learned policy
trained on gorge's available signals will find more — stop there. If it can,
§4.3 is the design, and the single fact that governs it is that **gorge has no
teacher**: its only imitation target beats a coin-flip blocker by +3.6pp
[52.1%, 55.2%] at N=4000, and every documented success in the prior XMage
project was warm-started from behaviour-cloning a strong one.

---

## 1. The decision surface, measured

### 1.1 The nine kinds

`decision/decision.go:15-69` declares exactly nine `decision.Kind` values.
Every one of them is dispatched in `rules/turn.go:287-317`. Here is where
each is constructed and what its option list is:

| Kind | constructed at | Min/Max | option `Kind` vocabulary |
|---|---|---|---|
| `KPriority` (`:16`) | `rules/turn.go:214` `askPriority`, options from `rules/legal.go:38` `legalActions` | 1/1 | `play_land` (`legal.go:48`), `cast` (`:74,:82,:87,:91,:113`), `activate` (`:135`), `ability` (`:188`), `pass` (`:196`), `concede` (`:201`) |
| `KTarget` (`:17`) | `rules/stack.go:173` `askTarget` | `TargetMin$`/`TargetMax$` | `player` (only when the spec targets players *and* `TgtZone$` is battlefield, `stack.go:186-190`), else an object kind |
| `KAttackers` (`:18`) | `rules/combat.go:106` `askAttackers` | **0 / len(opts)** | `attacker`, one per creature passing `canAttack` |
| `KBlockers` (`:19`) | `rules/combat.go:172` `askBlockers` | **0 / len(opts)** | `block`, one per legal *(blocker, attacker)* pair; the attacker is on `Option.Attacker` (`decision.go:86`) |
| `KMulligan` (`:28`) | `rules/mulligan.go:89` (keep/mull) and `:113` (bottom) | 1/1, or taken/taken | `keep`, `mulligan`; or `bottom` |
| `KModes` (`:39`) | `effects/misc.go:208` (charm), `effects/copy.go:66` (UnlessCost may-pay) | `CharmNum$`/1 | `mode` |
| `KTriggerOrder` (`:54`) | `rules/trigger_queue.go:511` | n/n (a permutation) | `trigger` |
| `KTriggerOptional` (`:60`) | `rules/trigger_queue.go:567` | 1/1 | `yes`, `no` |
| `KChoose` (`:68`) | `rules/cast.go:311,:373,:430,:599` | varies | `x`, `exile`, `sacrifice`, `name`, `type`, `number`, `yes`, `no` |

`Decision.Validate` (`decision/decision.go:159-180`) is the whole contract: an
answer is between `Min` and `Max` **distinct, in-range indices** into
`Decision.Options`. There is no other legal answer shape. That single fact is
the most important input to the output-representation question in §1.4.

### 1.2 Measured distribution — 660 games, all 66 deck pairs

Probe: a standalone module in the scratchpad that reproduces `cmd/botbench`'s
wiring exactly (`view.Project` → `Seat.Decide` → `Engine.Submit`, the loop at
`cmd/botbench/main.go:159-179`) and histograms every `Decision` before
answering it. 12 repo decks (`internal/testutil/decks/*.json`), every
unordered pair, 10 games each, `Mulligans: 3`, seed base 0. All 660 games
completed. **Measured:**

```
total decisions = 328,637   mean 497.9 decisions/game   mean 18.1 turns

kind                  count  share% mean_opts    min    max  mean_Min  mean_Max   Max>1%
attackers              5082   1.546      2.29      1     19      0.00      2.29    60.47
blockers               1102   0.335      2.55      1     18      0.00      2.55    62.25
choose                  891   0.271      4.30      1     22      0.92      1.13     6.29
modes                   135   0.041      2.44      2      3      1.00      1.00     0.00
mulligan               1518   0.462      2.22      2      7      1.04      1.04     4.35
priority             308778  93.957      3.81      2     33      1.00      1.00     0.00
target                 9712   2.955      5.15      1     62      0.97      1.00     0.00
trigger_optional        992   0.302      2.00      2      2      1.00      1.00     0.00
trigger_order           427   0.130      2.19      2      4      2.19      2.19   100.00
```

Option-count histograms (measured, same run):

```
priority   2:200208 3:15561 4:14277 5:15325 6:14000 7:13238 8:8943 9:6551
           10:4876 11:3021 12:2634 13:1816 14:1357 15:1190 ...(5781 in 18 larger buckets, max 33)
target     1:374 2:5773 3:479 4:403 5:341 6:301 7:204 8:202 9:161 10:191 ...(882 in 44 larger, max 62)
attackers  1:2009 2:1379 3:880 4:420 5:194 6:85 7:30 8:35 ...(max 19)
blockers   1:416 2:338 3:131 4:91 5:19 6:50 7:4 8:23 ...(max 18)
choose     1:302 2:196 3:53 4:42 5:45 6:37 7:31 8:15 9:54 ...(max 22)
mulligan   2:1452 7:66
modes      2:76 3:59
trigger_optional 2:992   trigger_order 2:350 3:75 4:2
```

A second run on the production 2-seat bench configuration
(`death-n-taxes` vs `dimir-tempo`, the only pair `cmd/botbench` ever plays —
see `cmd/botbench/main.go:396-407`), 300 games, no mulligans, gives the same
shape: 92.8% priority, 2.5% target, 1.9% attackers, 1.5% trigger_optional,
0.58% choose, 0.43% blockers, 0.29% trigger_order.

### 1.3 The number that reframes the whole problem

**64.8% of all priority decisions have exactly two options: `pass` and
`concede`** (measured: 200,208 of 308,778 in the all-pairs run; 82,987 of
121,258 — 68.4% — in the single-pair run). `legalActions` offers `pass` and
`concede` unconditionally (`rules/legal.go:196,:201`), so a priority decision
with no other option is a decision with exactly one *sensible* answer.

Full non-pass-option histogram over the all-pairs run (measured):

```
 0 non-pass: 200208 (64.84%)     6: 8943 (2.90%)    12: 1357 (0.44%)
 1 non-pass:  15561 ( 5.04%)     7: 6551 (2.12%)    13: 1190 (0.39%)
 2 non-pass:  14277 ( 4.62%)     8: 4876 (1.58%)    14: 1016 (0.33%)
 3 non-pass:  15325 ( 4.96%)     9: 3021 (0.98%)    15+: 4232 (1.37%, tail to 31)
 4 non-pass:  14000 ( 4.53%)    10: 2634 (0.85%)
 5 non-pass:  13238 ( 4.29%)    11: 1816 (0.59%)
priority decisions with >=1 non-pass option: 108,570 / 308,778 (35.16%)
```

Consequences:

- The **corpus is two-thirds noise by construction.** Filtering forced
  priority rows removes 60% of the training set and removes exactly the class
  imbalance that the prior project's VDWM loss was invented to fight
  (`project_vdwm_novel_synthesis`: "Universal policies face a ~91% majority
  floor on 'pass' actions. Cross-entropy spends most of its gradient
  reinforcing pass"). gorge can just *drop* those rows — XMage could not,
  because there a "pass" was a genuine choice with a legal alternative. This
  is the single cheapest structural advantage gorge has over the prior work.
- The real decision count is **~164 per game** (measured: 141.5 branchy
  priority + ~8.5 attackers + ~1.9 blockers + ~10.9 target + ~6.3
  trigger_optional + ~2.5 choose + ~1.2 trigger_order in the single-pair run),
  not 430.
- **`concede` is offered on every single priority decision** (measured:
  308,778 `concede` options over 308,778 priority decisions). An unmasked
  argmax-over-options policy can and eventually will concede. The current
  heuristic only avoids this because it returns `pass` *before* the
  positional fallback ever runs (`botpolicy/policy.go:145-168`) and because
  `clamp`'s top-up explicitly prefers `pass` (`policy.go:319-327`). Any
  learned policy needs `concede` hard-masked at the encoder, not learned away.

### 1.4 The output representation

**Recommendation: a per-option scoring head. Fixed output width = 1. The
network scores each offered `decision.Option` and the seat answers with the
argmax (or the top-`Min`..`Max` for the subset kinds). There is no global slot
space and no card vocabulary.**

The brief is right that this is the most important question, and right that
it has to be argued against the joint-slot result specifically, so:

**Why not one head per decision kind over a global slot space (the XMage
shape).** That is what produced the ceiling. `project_ai_bc_ceiling` and
`project_beat_cp7_exhausted` blamed BC; `project_phase2_joint_slots_promotion`
proved them both wrong — "The CHOOSER ~47% ceiling is NOT the BC ceiling, nor
a state-feature gap. It's an OUTPUT-slot-coarseness limit." Adding 8 hand-
enumerated joint slots (2 cards × 2 target descriptors) to a 34-slot space,
which attached to only **478 of 49,601 rows**, bought **+5.9pp / +3.7σ at
n=1000** — the largest single lift of that entire project. The lesson is not
"add joint slots". The lesson is *the model must be able to express "this
action against this target" as a distinct output*, and the joint slot space
was a hand-cranked, non-scaling approximation of that.

**Why a scoring head is strictly the generalisation of the joint slot space.**
`decision.Option` already carries the target: `Obj`, `Player`, `Kind`,
`Attacker`, `AltCostIndex`, `Mode`, `Amount`, `Ability`
(`decision/decision.go:73-117`). "Cast Swords to Plowshares @ their Thalia"
and "@ my Thalia" are two different `Option` values that join to two different
`CardView`s. A scorer over option features separates them with no cross
product to enumerate, for every card and every target, at zero vocabulary
cost. The joint slot space is a rank-restricted special case of it.

**Why a global slot space is not merely coarse here but impossible.** The
corpus is 33,667 cards, 19,765 playable (`AGENTS.md:65-66`); the 12 repo decks
use 136 distinct cards (`AGENTS.md:49-50`), and I measured
**186 distinct (option kind, card name) pairs at priority** across all 12 decks. Fitting the fixture is easy. But
gorge ships as a library inside mtgserve and plays whatever deck a user
brings, and the prior project already paid for this exact mistake:
`project_playvsai_policy_vocab_gate` records that an out-of-vocabulary deck
made the policy "pass-everything (no lands)", and had to be fenced behind a
`PolicyVocab` allowlist. A feature-based option encoder degrades to "score by
type, mana value, keywords and P/T" on an unseen card. A slot space degrades
to nonsense.

**Why the decision protocol makes this the natural shape.** `Validate`
(`decision/decision.go:159`) accepts only indices into the offered list.
Legal masking — which took XMage's policy-to-action match from 8.5% to 99.8%
(`project_noncp7_parity`) — is not a feature you add here; it is the only
thing the protocol permits. Scoring options rather than slots means the
"find a legal action matching the argmax slot" layer, and every bug in it,
does not exist.

**Why it is the only shape that handles `KAttackers`/`KBlockers` at all.**
Both are `Min: 0, Max: len(opts)` (`rules/combat.go:121`, `:202`) — a subset
selection over up to 19 options (measured max). That is 2^19 answers. No
categorical head can enumerate it. A per-option score with an independent
sigmoid, plus a small combinatorial repair pass, is the only representation
that fits without a redesign of the wire format.

**Slot count.** The output layer has one scalar per option, so the "slot
count" question moves to the option encoder's width. Proposed, and derived
from the measurements above:

| block | slots | derivation |
|---|---|---|
| `Option.Kind` one-hot | 23 | measured: the complete `Option.Kind` vocabulary over all 9 decision kinds, 660 games — `ability, activate, attacker, block, bottom, cast, concede, exile, keep, mode, mulligan, name, no, number, pass, permanent, play_land, player, sacrifice, trigger, type, x, yes` |
| `decision.Kind` one-hot | 9 | `decision/decision.go:15-69`; one scorer serves all kinds, so it must know which it is answering |
| card type bits | 12 | Land/Creature/Artifact/Enchantment/Instant/Sorcery/Planeswalker/Battle/Legendary/Basic/Token/other |
| colour bits + mana-value buckets | 6 + 9 | WUBRG+C; MV 0..7, 8+ |
| `Option.Mode` + `AltCostIndex` present | 6 + 1 | `decision.go:94-101` — `"", kicked, surged, flashback, miracle` + alt-cost |
| keyword bits | 32 | the 31 registered `kw:` primitives (`AGENTS.md:67-68`) + "other" |
| zone / ownership relation | 8 | mine-vs-theirs × {battlefield, hand, graveyard, exile} |
| hashed card identity | 1 id | into the shared 16,384-row table; lets the model memorise known cards without a fixed vocabulary |
| hashed (card identity × target descriptor) | 1 id | **this is the joint slot, hashed instead of hand-enumerated** |
| dense scalars | 24 | derived P/T, remaining toughness, damage, tapped, summon-sick, attacking, for a `block` option the attacker's P/T deltas, cost vs available mana, would-tap-out, index/len(options) |

**106 categorical slots + 2 hashed ids + 24 dense = a 132-wide option
descriptor; round to 128 sparse slots + 24 dense.** Roughly 12 are active per
option (that is the `nnzO=12` used in the timing in §3).

The state descriptor is a bag of ~200 hashed ids into the same 16,384-row
table plus ~64 dense scalars (§2). Shared embedding table 16,384 × 128 = 2.10 M
parameters; trunk 128×128 = 16 K; total **≈ 2.13 M float32 = 8.5 MB**, or
2.1 MB quantised to int8.

### 1.5 Where the package sits

The dependency order is `cards → state → decision → events → effects → rules
→ view → seat → replay → protocol → host → host/httpapi → cmd/*`. A learned
policy must read `view.View`, so it sits **between `view` and `seat`**: a new
package importing `cards`, `state`, `decision`, `view`, imported by `seat`
(a `seat.NetBot` beside `seat.Bot`) and registered in `cmd/botbench`'s
`policies` map (`cmd/botbench/main.go:61-75`). It must **not** be reachable
from `rules`; `botpolicy` is (via `rules/testbot_test.go`), and that is why
`botpolicy` imports only `decision` and `state` (`botpolicy/policy.go:17-22`).
Keeping the network out of `rules` also keeps the fuzz driver and the
acceptance suite on the deterministic heuristic, which matters for §6's
golden-churn risk.

---

## 2. The observation

### 2.1 What it must be derived from

`view.Project` (`view/view.go:211`) is the seat-legal projection. The only
place the viewer's own private state appears is `view/view.go:262-265`:

```go
if p.ID == viewer {
    pv.Hand = cardViews(g, ch, g.Zone(state.ZHand, p.ID))
    pv.Pool = poolView(p.Pool)
}
```

Everything else in `PlayerView` (`view/view.go:88-114`) is public:
`Life`, `Lost`, `LibrarySize`, `HandSize`, `GraveyardSize`, `Battlefield`,
`Graveyard`, `Exile`. The pending `Decision` is attached only for the seat it
was asked of (`view/view.go:269-276`).

`state.Game.Clone` (`state/game.go:103-118`) deep-copies `Objs` and every
zone slice — **including the opponent's hand and the exact library order**.
Anything that clones and looks is cheating. The rule for the encoder is
therefore mechanical: **the encoder takes a `view.View` and nothing else.**
Not `*state.Game`, not `*rules.Engine`. `seat/bot.go:67-90` is the shape to
copy.

This is the BC-ceiling failure in a new costume, and it is worth naming
precisely: the prior project's LLM-oracle arc reached ρ=+0.66 correlation
with outcomes — and then noted "`GameSnapshot.build()` is a god-view snapshot;
both seats' hands are rendered … the heuristic would systematically
overestimate certainty by reading hidden info"
(`project_llm_oracle_exhausted`). A feature that is present at training time
and absent at inference does not produce a weak policy; it produces a
confidently wrong one.

### 2.2 The proposed encoding

Everything below is available on the `View` and nothing below is hidden.

**Dense scalars (~64).** Turn number and turn-bucket; `Step` one-hot (12
steps) and `Phase` one-hot (5, `view/view.go:53-54`); active/priority is me;
stack depth; for each seat: `Life`, `HandSize`, `LibrarySize`,
`GraveyardSize`, battlefield count, creature count, total derived power,
total derived toughness, untapped-permanent count, untapped-land count; my
`Pool` by colour (6, mine only, `view/view.go:109`); lands played this turn
(derivable from the presence/absence of a `play_land` option); and, at
`KAttackers`/`KBlockers`, attacker/blocker totals.

**Sparse hashed bag (~200 active ids into 16,384 rows).** One id per card in
each of: my hand (`Name` + zone tag), my battlefield, their battlefield, my
graveyard, their graveyard, exiles, and each `StackView` (`view/view.go:171`,
which carries `Kind`, `Name`, `Text`, `Controller`, `Targets`). Battlefield
cards additionally hash a *state-decorated* id — `name|tapped|attacking|
summonsick|counters` — so the model can distinguish a tapped Thalia from an
untapped one without a dense per-object block. `CardView`
(`view/view.go:126-169`) carries `Types`, `ManaCost`, `Power`, `Toughness`,
`Damage`, `Tapped`, `Attacking`, `AttackingPlayer`, `BlockedBy`, `Counters`,
`Keywords`, `Controller`, `Owner`, `SummonSick`, `AttachedTo` — every one of
those is legitimately public and every one is used.

**Resulting width:** ~64 dense floats + ~200 active ids. With `H=128` that is
the `nnzS=200` row timed in §3 at 9.4–14.9 µs per decision.

### 2.3 What is lost, stated plainly

1. **The opponent's hand and the library orders.** Correct and non-negotiable.
   A learned policy must infer, not read.
2. **Your own library contents.** The `View` gives `LibrarySize` only
   (`view/view.go:255`). A human player knows their own decklist. The seat
   *does* know its own deck (it is passed to `rules.Config.Decks`), so the
   policy constructor can be handed the decklist out of band — this is
   legitimate information and cheap to add, but it does not come from the
   `View` and someone will forget that.
3. **Card rules text for non-stack objects.** `StackView.Text`
   (`view/view.go:175`) exists, `CardView` has no text field. The policy
   therefore cannot see *what a card does* from the `View` alone — only its
   name, types, mana cost, keywords and P/T. This is a real hole: it is what
   makes "Swords to Plowshares" and "Path to Exile" different from a 1-mana
   white instant. Fix: the policy package may import `cards` (it is at the
   root of the dependency order) and look the name up in the `Registry`, which
   is public information anyway. The hashed card-identity slot in §1.4 covers
   the memorisation case; the registry lookup would cover the generalisation
   case. **Recommend not building the registry-feature path in phase one** —
   it is a large surface and the measured headroom does not justify it yet.
4. **History.** The `View` is a snapshot. Nothing says what the opponent did
   last turn, what they have cast, or how many cards they have drawn beyond
   the current sizes. Anything history-shaped has to be accumulated by the
   seat across `Decide` calls, which makes the policy stateful and therefore
   harder to make deterministic and replayable. **Recommend against in phase
   one.**

---

## 3. Feasibility in pure Go

`go.mod` declares the module and nothing else; `go list -m all` prints exactly
one line, `github.com/adams-shaun/gorge`. There is no `go.sum`. Zero
dependencies is real. Note, though, that **nothing enforces it**: the
GPL-boundary guard (`cards/boundary_test.go`) checks for Forge scripts in
tracked `.txt` files, not for third-party Go imports. `AGENTS.md:18-19` states
the rule; no test asserts it. A learned-policy programme is exactly the kind
of work that would be tempted to add a dependency, so if this is funded, add
the assertion first.

### 3.1 Measured linear-algebra throughput on this box

Plain Go, `float32`, no assembly, no cgo. Go does not auto-vectorise, so these
are scalar numbers; a hand-written AVX-512 BLAS would be roughly an order of
magnitude faster per core, and is unavailable to us.

**Matrix-vector (batch-1 inference), 1 core — measured:**

```
matvec 256x512      26.200 us/op   10.01 GFLOP/s
matvec 256x256      13.175 us/op    9.95 GFLOP/s
matvec 128x512      13.086 us/op   10.02 GFLOP/s
matvec  64x256       3.318 us/op    9.88 GFLOP/s
matvec 1024x1024   215.624 us/op    9.73 GFLOP/s
```

**Matrix-matrix (minibatch), i-k-j order, 1 goroutine — measured:**

```
matmul   64x512x256    2.477 ms   6.77 GFLOP/s
matmul  256x512x256   10.001 ms   6.71 GFLOP/s
matmul  256x256x256    4.861 ms   6.90 GFLOP/s
matmul  512x512x512   35.290 ms   7.61 GFLOP/s
matmul 1024x1024x1024 267.126 ms  8.04 GFLOP/s
matmul f64 512^3       35.588 ms   7.54 GFLOP/s   (f64 costs nothing extra here)
```

**Matrix-matrix parallel over rows — measured:**

```
1024^3  x2 goroutines  138.685 ms  15.48 GFLOP/s
1024^3  x4             72.054 ms   29.80 GFLOP/s
1024^3  x8             41.878 ms   51.28 GFLOP/s
1024^3 x16             32.102 ms   66.89 GFLOP/s
1024^3 x32             27.621 ms   77.75 GFLOP/s
```

So: **≈10 GFLOP/s per core for matvec, ≈7-8 for matmul, ≈78 GFLOP/s across all
32 threads.** That is enough for the model in §1.4 and nowhere near enough for
anything transformer-shaped. `float64` costs nothing extra, which is worth
knowing if reproducibility ever argues for it.

### 3.2 Measured forward pass of the actual candidate architecture

The kernel that matters is not a matmul: with a sparse hashed input the first
layer is an embedding-bag (a gather-and-add), and the per-option head is a
dot product. Measured, 1 core, `nnz_state=200`, `nnz_opt=12`:

```
H=256, 65,536-row table (67.1 MB):   2 opts 27.8 us   8 opts 33.3 us   33 opts 56.9 us
H=128, 16,384-row table ( 8.4 MB):   2 opts  9.4 us   8 opts 11.5 us   33 opts 20.9 us
H= 64,  4,096-row table ( 1.0 MB):   2 opts  4.1 us   8 opts  5.3 us   33 opts 10.2 us
```

Most of the cost is cache misses on the embedding gather, which is why the
table size matters more than `H`.

**Compare against the engine itself** — measured over 100 games / 42,909
decisions of `death-n-taxes` vs `dimir-tempo` at `GOMAXPROCS=1`:

```
per game        17.5 ms
per decision    40.9 us   of which  view.Project 16.4-17.7 us
                                     Engine.Submit 21.5-22.9 us
                                     botpolicy.Decide 2.0-2.3 us
```

So the recommended `H=128` policy at ~11 µs/decision adds **~27% to the cost
of a self-play game** if it runs on every decision (430/game → +4.7 ms on
17.5 ms). It should not run on every decision: the same §1.3 filter that
drops forced priority rows from the corpus short-circuits them at inference
too — pass without a forward pass — which cuts it to ~164 decisions/game and
**+10%** (17.5 ms → ~19.3 ms). That is the whole latency answer:

- **In-process request path (mtgserve):** ~11 µs per decision against an
  engine step that already costs 41 µs and an HTTP round trip that costs
  milliseconds. Irrelevant.
- **Self-play corpus generation:** ~19-22 ms/game single-core → ~1,450-1,680 games/s
  across 32 threads → **~5.2-6.0 M games/hour** (estimate, from the measured
  per-game cost; the engine is single-goroutine per match
  (`gorge-context.md`), so this is embarrassingly parallel). The prior project
  worked with 1,000-game corpora. gorge can generate 1,000 games in under a
  second of wall time. **Corpus size is simply not a constraint here, and
  that is the largest single difference from the XMage work.**

### 3.3 Measured training throughput in pure Go

An honest SGD step — embedding-bag forward, `H×H` layer with ReLU, per-option
dot, softmax cross-entropy, hand-derived backward through all of it, sparse
embedding update — batch 1024, `nnzS=200`, `nnzO=12`, 4 options. Measured:

```
H=128, 16,384 rows,  1 worker    34.33 ms/step     29,828 rows/s
H=128, 16,384 rows,  8 workers    9.15 ms/step    111,965 rows/s
H=128, 16,384 rows, 32 workers    6.08 ms/step    168,314 rows/s
H=256, 65,536 rows, 32 workers   13.12 ms/step     78,043 rows/s
H= 64,  4,096 rows, 32 workers    3.16 ms/step    323,712 rows/s
```

A 10 M-row epoch at `H=128` is **60 s parallel, 5.6 min single-threaded.**
Training in Go is comfortably feasible. Two caveats that are specific to this
repo and that the numbers above quietly violate:

- The 32-worker figure uses **Hogwild-style unsynchronised sparse embedding
  updates**. That is a data race in the Go memory model. `.githooks/pre-push`
  keeps `-race` opt-in and the controller runs it by hand at merge whenever a
  diff touches concurrency (`gorge-context.md`), so this *will* be caught.
  Race-clean alternatives: per-worker gradient accumulation with a
  deterministic reduce (costs memory, keeps most of the speed) or
  `sync/atomic` adds on the embedding rows.
- Parallel float accumulation is order-dependent, so a 32-worker trainer is
  **not reproducible**. gorge's entire culture is determinism — no
  `time.Now`, no global rand, no map range that can reach an event. A
  non-reproducible checkpoint is a foreign object in this repo. Use the
  single-threaded trainer (measured 29.8 K rows/s, still 5.6 min for 10 M
  rows) or a fixed-shard deterministic reduce.

### 3.4 Hand-derived gradients or autodiff?

**Hand-derived, for a fixed architecture. Recommend against building an
autodiff.**

The measured backward above is ~120 lines. A reverse-mode autodiff worth
having is a tape, a graph, an op registry and a numerical-gradient test suite
— call it 1,500–2,500 lines of new pure-Go code (estimate) with its own
per-package test budget under this repo's `TEST_HISTORY.md` regime. It buys
architecture flexibility, and the prior project's record is explicit that
architecture flexibility was *not* the bottleneck: five PPO variants with
value baselines, advantage normalisation and entropy bonuses all regressed,
and the summary was "The trainer machinery is correct; the bottleneck is the
SAMPLE-vs-deployment distribution mismatch in corpus collection"
(`project_ppo_v2_iteration_attempts`). Spending 2,000 lines to make the part
that was already correct more flexible is the wrong trade.

Freeze one architecture; write forward and backward by hand; pin both with a
finite-difference test.

### 3.5 Train offline in another language, or in Go?

**Recommend: train in Go, in-repo, single-threaded and deterministic.**

The tempting alternative is to dump a corpus to JSONL, train in PyTorch, and
ship only the weights. It is what the prior project did, and it worked *for
that project* because the engine was Java and the bench was slow, so the
Python side was never the bottleneck. Here the tradeoffs invert:

- **The zero-dependency boundary is a licensing and identity claim, not a
  build preference** (`AGENTS.md:18-19`). Weights are data, so a PyTorch-
  trained `.bin` does not literally add a Go dependency. But the *reproducer*
  for the shipped artifact would then be a Python environment that is not in
  the repo, not pinned, and not covered by any gate. The prior project has a
  memory entirely about losing its checkpoints to a tmpfs wipe
  (`reference_checkpoint_scoreboard`, "Persistence trap"). An in-repo Go
  trainer with a seed makes the checkpoint a *derived* artifact you can
  rebuild from `main`, which is the only form that fits this repo.
- **Go training is fast enough** (§3.3) precisely because the model is small,
  and the model is small because §1.4 chose a representation that does not
  need to be big.
- The counter-argument is real and should be stated: if the programme ever
  needs a model large enough that 30 K rows/s single-threaded is limiting,
  Go stops being viable and the answer is to stop, not to add a dependency.
  I would treat "we need PyTorch" as the signal that the design has drifted
  off the constraint, not as a licensing question to negotiate.

### 3.6 Serialization

`go:embed` a versioned little-endian binary blob in the policy package:
magic + format version + architecture hash + training seed + corpus digest +
`float32` (or `int8` + per-row scale) weights. 8.5 MB at `float32`, 2.1 MB at
int8. The architecture hash and the corpus digest are the important part: the
checkpoint has to say which encoder it was trained for, or the first encoder
change silently produces a policy that plays nonsense. Loading is a
`binary.Read` into pre-sized slices — no init-time cost worth measuring.

---

## 4. The training scheme

### 4.1 The problem: gorge has no teacher

This is the difference that governs everything, and it cuts the opposite way
from the prior project.

XMage had **CP7**, a minimax-plus-rollout AI whose self-play mirror was 53.3%.
Every path that eventually worked was warm-started from behaviour-cloning it:
BC → joint slots (44.7 → 50.6) → single-pass PPO (→ 51.8) → self-distillation
PPO ×6 (→ 54.5). Every path that did *not* start from BC of a strong teacher
failed: naive REINFORCE collapsed to 1% at lr=1e-4 and to 8.4% on the second
generation at lr=1e-5 (`project_pathC_reinforce_results`); naive iterated
self-play walked in [4%, 47%] over ten generations with no trend
(`project_phase5_iter_regression`); a full-control agent built on heuristics
plus learned heads capped at ~45-49% and every one of four learned combat
heads lost to the hand-written heuristic (`project_noncp7_parity`).

gorge's only imitation target is `botpolicy.Decide`. Measured, N=4000, seed 0,
`death-n-taxes` vs `dimir-tempo`:

```
bot vs legacy   A wins 2146/4000   53.6%   95% CI [52.1%, 55.2%]   17.9 turns
```

That is the *entire* measured value of the B2 combat heuristics over
`botpolicy.LegacyDecide`, which differs from `Decide` in exactly two branches
— `KAttackers` attacks with every legal attacker and `KBlockers` takes each
legal pair on a coin, one blocker per attacker (`botpolicy/legacy.go:71-89`);
every other kind is identical. So this comparison isolates the combat
heuristics cleanly. **+3.6pp.** At the
customary N=500 the same comparison reads 54.2% [49.8%, 58.6%] — an interval
that straddles 50%, i.e. by the project's own standard, not a result.

Behaviour-cloning a policy whose edge over coin-flip blocking is 3.6pp gives
you, at best, a slower copy of it. The BC ceiling is not a soft constraint
here; the ceiling is at floor level.

### 4.2 Where the headroom actually is — measured

Probe: take the production heuristic, degrade **exactly one decision kind**,
and bench the degraded policy against the intact one on `botbench`'s wiring
(seat alternation, per-game seeds, Wald interval). Seed 0, the production
deck pair, N as shown per row. A = intact bot. **Measured:**

| B degrades | N | A wins | A WR | 95% CI | mean turns |
|---|---|---|---|---|---|
| priority → always pass | 2000 | 2000 | **100.0%** | [100.0%, 100.0%] | 20.8 |
| never attack | 2000 | 1993 | **99.7%** | [99.4%, 99.9%] | 36.0 |
| priority → uniform random legal | 2000 | 1916 | **95.8%** | [94.9%, 96.7%] | 21.9 |
| **spell choice only → uniform random** (keep tap-for-mana and the land drop; randomise among `cast`/`ability`/`pass`) | **4000** | 2673 | **66.8%** | **[65.4%, 68.3%]** | 19.7 |
| attackers+blockers → coin flip per option | 2000 | 1260 | **63.0%** | [60.9%, 65.1%] | 21.4 |
| target → uniform random | 2000 | 1086 | **54.3%** | [52.1%, 56.5%] | 18.6 |
| never block | 2000 | 1075 | **53.8%** | [51.6%, 55.9%] | 17.9 |

Read that table carefully, because it is the whole case. What it measures is
*sensitivity*: how much the outcome moves when a decision surface stops being
played well. Sensitivity is a necessary condition for a learned policy to earn
anything on that surface — if degrading a surface to random costs nothing, no
policy can win anything there — though it is not sufficient.

- **Priority is where essentially all of the value is.** Random legal actions
  lose 45.8pp; never attacking loses 49.7pp; always passing loses everything.
- **The mechanical part of the priority rule is not the interesting part.**
  The rule is: tap for mana in a main phase, then play a land, then cast, then
  activate an ability, then pass (`botpolicy/policy.go:115-168`). Keeping the
  two mechanical steps (mana, land drop) and randomising only *which spell or
  ability, or none* still costs **16.8pp [15.4pp, 17.9pp] at N=4000**. So the
  spell-selection surface is genuinely sensitive, by a wide margin, and it is
  the one surface on which no strategic work has ever been done — the current
  rule is "cast the first `cast` option in zone order", with no notion of
  holding an instant, sequencing, curve, or mana efficiency.
- **The two surfaces the bot workstream has actually spent its effort on are
  the two least sensitive ones.** The entire blocking heuristic — BR1/BR2, the
  thing B2 was built for — is worth **3.8pp** over never blocking at all. The
  entire targeting rule, which B3 is currently rebuilding, is worth **4.3pp**
  over uniform random. Combat as a whole (attack and block, coin-flipped) is
  worth 13.0pp, essentially all of it in "attack at all" rather than in
  choosing well.

**This reorders the priorities of the whole bot workstream, learned or not.**
The sensitivity ranking, measured, is:

```
spell selection at priority   16.8pp   <- no strategic work has ever been done here
combat (attack + block)       13.0pp   <- B2 did this
targeting                      4.3pp   <- B3 is doing this
blocking alone                 3.8pp   <- B2 did this
```

Note what this does *not* say. 16.8pp is the gap between random spell choice
and the trivial in-zone-order rule; it is not the gap between the trivial rule
and an optimal one. The measurable band above the current rule could still be
small. But it is the only surface on this engine where the band is even
plausibly large, and it is the surface a learned policy is best suited to,
because "which of these seven castable things, or none" is exactly a scoring
problem over an offered option list.

### 4.3 The recommended path, if the gate opens

**Teacher: none. Corpus: self-play. Signal: terminal outcome only.** gorge
exposes no intermediate reward and no oracle. This is the setting in which
the prior project failed repeatedly, so every element below exists to
neutralise a specific documented failure.

**Phase 0 — make the measurement honest (2 days, do this regardless).**
`cmd/botbench` plays `names[:seats]` from the sorted repo-deck list
(`cmd/botbench/main.go:396-407`), so every number anyone has ever quoted for
gorge policy work is **one deck pair**: `death-n-taxes` vs `dimir-tempo`. Add
a deck-pair matrix mode (66 unordered pairs) and a shard flag so a matrix run
parallelises across cores. Standardise on N≥4000 per cell.
*Exit:* the matrix runner reproduces the single-pair baseline exactly and a
66-cell matrix at N=1000/cell runs in under 5 minutes wall (66,000 games
at the measured 17.5 ms/game = 19 CPU-minutes, ~40 s across 32 threads).

**Phase 1 — bound the heuristic family, on the surface that matters (3 days,
do this regardless).** §4.2 already answered the sensitivity question:
spell selection at priority is worth 16.8pp, four times what blocking or
targeting are worth. So point B4's fitted weight table at *that* surface —
a scored ordering over `cast`/`ability`/`pass` (curve, mana efficiency,
instant-vs-sorcery timing, hold-up-mana), not more combat tuning — and run it
through the Phase-0 matrix.
*Exit gate for the whole programme:* the fitted table clears **+3pp at N=4000
on the production pair and is non-negative on at least 50 of 66 pairs**. If a
hand-fitted score over that surface cannot move the needle by 3pp, then the
band above the trivial rule is narrow after all, and a learned policy — which
optimises the same objective with a noisier signal and no teacher — will not
find more. Stop there, ship the heuristic, spend the days on M2e.

**Phase 2 — the network (8-12 days).** Only if Phase 1's gate opened.

- *Architecture:* §1.4 and §2.2. `H=128`, 16,384-row shared hashed embedding.
  A value head on the trunk, trained on the same terminal outcomes.
- *Corpus:* self-play, both seats the current network. **Drop every forced
  priority row** (measured 64.8% of priority decisions, §1.3) — it is free
  and it removes the class imbalance that VDWM was invented to fight.
  1 M games ≈ 164 M real decisions; subsample to 10 M rows per generation.
  Generation cost ≈ 12 min wall on 32 threads (estimate from §3.2).
- *What makes iteration stable* — the part every prior attempt got wrong.
  `project_ppo_v2_iteration_attempts` diagnosed it exactly: "PPO requires
  `π_old` to be the action-collection policy. Our SAMPLE collection is much
  weaker than our CHOOSER deployment. The corpus represents a different policy
  than the one we want to improve." `project_self_distillation_iteration`
  fixed it by emitting the corpus from the *deployment* (argmax) policy and
  synthesising `π_old = argmax_softmax_value`, and got six compounding
  iterations, 51.8 → 54.5 mirror and 37.4 → 40.4 cross-deck — the first
  checkpoint in that whole project to exceed its teacher. **Do that from day
  one.** It is a handful of lines: when the seat answers with the argmax
  option, record the softmax probability it assigned. Do not build a SAMPLE
  collection mode at all; it is the known-broken path.
- *Loss:* PPO clipped surrogate + KL anchor to the previous generation, with a
  per-state value baseline and advantage normalisation, from the first
  generation. Naive REINFORCE collapsed twice, at two different learning
  rates, in the prior project. There is no reason to re-derive that.
- *Gating:* never promote a generation that does not clear **+1.96σ at N=4000
  against the current best**. The ten-generation naive loop that walked in
  [4%, 47%] had no gating (`project_phase5_iter_regression`); the gated loop
  that replaced it produced the project's first sustained improvement.
- *If the gradient still drowns:* switch to VDWM — margin loss weighted by
  `(1 − prob_old) × |advantage|` (`project_vdwm_novel_synthesis`). It is ~30
  lines on top of the PPO infrastructure and it was the only thing in that
  project that moved a universal policy's bench win-rate rather than just its
  test accuracy. But it should be second, not first: gorge's forced-row filter
  already removes most of the pathology VDWM was designed for.

*Exit:* ≥+3pp over the Phase-1 baseline at N=4000 on the production pair,
non-negative on ≥50 of 66 pairs, and no pair worse than −5pp.

**Phase 3 — ship (4 days).** `go:embed` the checkpoint; deterministic
single-threaded inference; one regeneration of `rules/heads_test.go` at the
milestone merge (Ruling FL-83) naming the measured cause; a race-clean trainer;
a finite-difference test on the hand-derived gradients; an assertion test that
`go list -m all` still prints one line.

### 4.4 Explicitly rejected, with the reason

| Path | Why not, here |
|---|---|
| BC of `botpolicy.Decide` | The teacher's measured edge over coin-flip blocking is +3.6pp (§4.1). BC cannot exceed its teacher (`project_ai_bc_ceiling`), so the ceiling is at floor level. |
| A global slot space, joint or otherwise | §1.4. 19,765 playable cards; mtgserve plays arbitrary decks; `project_playvsai_policy_vocab_gate` records what out-of-vocabulary decks do to a slot policy. |
| Inference-time MCTS / PUCT | `project_mcts_rollout_inference_no_help` (30-50× slower, worse), `project_puct_attempts` (all three leaf modes 27-36% vs CHOOSER 47%). And measured here: a full playout to game end from mid-game costs **14.4 ms** (§5.3) — 100 rollouts at each of 164 real decisions is 4 CPU-hours *per game*. |
| Training on a search-generated corpus before the search is proven | `project_puct_attempts` Path 2: training on a PUCT corpus that was itself worse than the teacher gave 9.7%. "Cannot bootstrap from a weaker policy." My §5.3 measurements say gorge's naive search *is* the weaker policy. |
| A SAMPLE-mode collection policy | The documented cause of five consecutive PPO regressions. Use argmax + synthesised `π_old`. |
| An autodiff framework | §3.4. |
| Training offline in Python | §3.5. |

---

## 5. What we measure against

### 5.1 The baseline, re-measured

`cmd/botbench` at N=500, seed 0, on `wt/r1` @ `e0a8f6a`, **measured**:

```
A wins: 262  B wins: 238  draws: 0   (same policy on both sides)
A win rate: 52.4%  95% CI [48.0%, 56.8%]
seat 0 wins: 302  seat 1 wins: 198   seat 0 win rate: 60.4%  95% CI [56.1%, 64.7%]
mean turns per game: 17.7
```

The brief quotes `seat 0 59.6% CI [55.3%, 63.9%]`; I get 60.4%. Both are the
same measurement at the same seed and N, so one of them predates a merge that
moved bot decisions. It is a 0.8pp difference inside a ±4.4pp interval and
does not matter — but it is a nice demonstration of why N=500 is not a
measurement instrument.

At N=4000, seed 0, **measured**:

```
bot vs bot:    A 51.1% [49.5%, 52.6%]   seat 0 57.2% [55.6%, 58.7%]   18.3 turns
bot vs legacy: A 53.6% [52.1%, 55.2%]   seat 0 55.4% [53.9%, 56.9%]   17.9 turns
```

The seat play/draw edge is **+7.2pp** and it is nearly twice the size of the
entire measured policy improvement B2 produced. That is the brief's point and
it survives at N=4000.

### 5.2 Is the Wald interval trustworthy here? — measured, and yes

This matters, because the prior project's benches were badly overdispersed:
"bench run-to-run variance is ~±7pp at n=200-300 (41 vs 51 same config ≈ 3σ,
beyond binomial → games are more correlated than independent)"
(`project_noncp7_parity`). Every apparent win in that project's combat hunt
was masked by it.

I ran six **disjoint** 1000-game blocks of `bot vs legacy` at base seeds
100000…600000. **Measured A win rates:** 54.6, 53.9, 52.6, 51.4, 52.6, 53.5.
Sample sd = **1.14pp**; the binomial σ at n=1000 is **1.58pp**. The observed
spread is *below* the binomial prediction (a 6-sample sd estimate has a wide
CI of roughly [0.7, 2.8]pp, so this is "consistent with binomial", not "better
than"). Seat-0 rates across the same blocks: 54.8, 58.7, 54.4, 55.6, 56.0,
53.7 — sd 1.76pp against a binomial 1.58pp. Also consistent.

**Conclusion: there is no evidence of overdispersion in `botbench`, so the
Wald interval it prints (`cmd/botbench/main.go:208-223`) can be taken at face
value.** That is a genuine, measured methodological advantage over the prior
project, and it is a direct consequence of the engine being deterministic and
of `gameSeed` (`main.go:104`) making every game a distinct, independent seed.

### 5.3 What search costs, measured — because it is the named alternative

`rules.Engine.Clone` (`rules/clone.go:20`) and a determinized playout,
measured over 238 samples taken every 40th decision of 20 games, `GOMAXPROCS=1`:

```
Engine.Clone           52.0 - 62.0 us
playout to game end    14.4 ms      (mean 264 decisions each)
```

A 1-ply lookahead over every priority candidate (clone, submit the candidate,
run forward with the heuristic to a leaf), measured over 30 games / 4,245
branchy decisions / 19,988 candidates:

```
leaf = next same-seat priority   mean 0.3 forward steps   212-218 us/candidate    996-1025 us/decision    163-167 ms/game
leaf = end of current step       mean 8.0 forward steps   530-536 us/candidate   2496-2525 us/decision    372-376 ms/game
```

versus 17.5 ms/game with no search. So 1-ply search is **9-21× the cost of the
heuristic** — affordable. But note the first row: **a 1-ply leaf is on average
0.3 forward steps from the root.** Casting a spell puts it on the stack; you
get priority straight back; the position has barely changed. That is the same
wall the prior project hit from a different direction: "two 1-ply-apart states
evaluate almost identically … PUCT can't differentiate alternative actions"
(`project_llm_oracle_exhausted`).

And then the thing that actually decides it. I built a crude 1-ply search seat
— clone every candidate, roll forward to a leaf with the heuristic, score the
leaf with life + board + card advantage, take the argmax — and benched it
against the plain heuristic bot on `botbench`'s wiring. It **cheats** (it does
not re-randomise the hidden zones `Clone` copies), so these are upper bounds.
**Measured:**

| search applied to | leaf | N | search-seat WR | 95% CI | mean turns |
|---|---|---|---|---|---|
| priority | end of step | 200 | **16.5%** | [11.4%, 21.6%] | 22.3 |
| attackers+blockers | end of step | 500 | **19.4%** | [15.9%, 22.9%] | 33.7 |
| attackers+blockers | end of turn | 200 | **22.5%** | [16.7%, 28.3%] | 29.8 |

and the result is insensitive to the eval weights across a 6× swing of the
life/power/toughness/hand coefficients: 22.0%, 23.5%, 22.5%.

That is one evening's probe with an untuned evaluator, and it does **not**
prove a serious search agent would fail. What it does show is that "the fitted
heuristic plus determinized search" is not a cheap alternative to a learned
policy — its first naive version loses by 30-80pp, for the same horizon and
leaf-evaluation reasons the prior project documented across PUCT, MCTS
rollouts, value-guided lookahead and rollout-eval combat search. Both branches
of the brief's final question are multi-week projects.

### 5.4 The N you need, and the matrix

With the Wald interval validated (§5.2), N to detect a δ against 50% at
1.96σ is `N = (0.98/δ)²`:

| δ | N |
|---|---|
| 5.0pp | 384 |
| 3.0pp | 1,067 |
| 2.0pp | 2,401 |
| 1.0pp | 9,604 |

The effect sizes to expect, from §4.1 and §4.2: the whole combat-heuristic
milestone was **+3.6pp**; targeting's total value over random is **4.3pp**;
blocking's is **3.8pp**. So the working N is **4,000**, giving ±1.55pp. At
17.5 ms/game that is **70 seconds of one core** per cell. There is no excuse
for running N=500.

**The matrix.** `cmd/botbench` currently only ever plays
`death-n-taxes` vs `dimir-tempo` (`main.go:396-407`). That is a fatal gap for
policy work: the prior project's per-deck baselines ranged from 24.3% to 54.5%
across five decks on the same architecture
(`reference_checkpoint_scoreboard`), and a change that helps aggro can cost
9pp on control. The matrix should be:

- **66 unordered pairs** of the 12 repo decks, N=1000 per cell (66,000 games
  at the measured 17.5 ms/game = 19 CPU-minutes, ~40 s across 32 threads).
- **The production pair at N=4000** as the headline number.
- Report per-cell A-win-rate with interval, the count of cells non-negative
  at 1σ, and the worst cell. A policy that wins the headline and loses 10pp
  on three matchups is not an improvement, it is a specialisation, and the
  prior project's universal-vs-per-deck arc is entirely about that trade
  (`project_universal_archetype_policy`: mono-green +27pp, aggro −4 to −5pp).
- Keep the seat split in every cell. It is the only thing that catches an
  attribution bug, and at +7.2pp it is bigger than everything being measured.

---

## 6. Cost, risks, and the recommendation

### 6.1 Cost

| Phase | Days (estimate) | Useful if the programme stops? |
|---|---|---|
| 0 — botbench matrix, N discipline, shard mode | 2 | Yes — every future bot task needs it |
| 1 — B4 fitted table pointed at spell selection, through the matrix | 3 | Yes — it is the gate, and B4 is planned anyway |
| 2 — encoder, forward/backward, self-play driver, PPO+self-distill loop, gating harness | 8-12 | No |
| 3 — embed, determinism, race-clean, goldens, gates | 4 | No |
| **total if fully funded** | **17-21** | |

The 8-12 for Phase 2 is the honest number and it is not reducible by reading
the prior project's code: that code is Java and Python against a different
engine. What transfers is the *conclusions*, which is exactly why this
document spends so much space on them. The encoder, the trainer, the corpus
format, the self-play driver, the gating harness and the PPO loop are all new
Go, all under this repo's per-package test-budget regime, and all needing
mutation tests at every enforcement point (`feedback_mutation_test_enforcement_points`).

### 6.2 The three biggest risks

**1. There is no teacher, and every documented success in the prior project
needed one.** Measured: the current heuristic beats a coin-flip blocker by
+3.6pp [52.1%, 55.2%] at N=4000, so behaviour cloning it is worthless, and
the from-scratch RL paths are precisely the ones that failed — naive REINFORCE
collapsed to 1% and 8.4%, naive iterated self-play walked in [4%, 47%] with no
trend, and every one of four learned combat heads lost to a hand-written
heuristic. The mitigations in §4.3 (best-policy gating, PPO clip + KL from
generation one, self-distillation corpus emission) are each documented to fix
one specific failure, but none of them has ever been shown to work *without*
a strong BC warm start. **This is not a risk that can be engineered away; it
is a bet, and it is the reason for the "not yet".**

**2. The effect sizes are smaller than the resolution people actually
measure at, and the measurement channel is one deck pair.** Measured: the seat
play/draw edge is +7.2pp; the entire B2 combat milestone is +3.6pp; blocking
is worth 3.8pp and targeting 4.3pp. At N=500 the half-width is ±4.4pp, which
straddles all four. `cmd/botbench` has never benched anything but
`death-n-taxes` vs `dimir-tempo` (`main.go:396-407`), and the prior project's
per-deck spread on one architecture was 24.3%–54.5%. A programme that reports
single-pair N=500 numbers will produce confident, reproducible, wrong
conclusions — and will produce them *cheaply*, which is worse. Mitigation is
Phase 0 and it is cheap: 4,000 games is 70 seconds of one core, and no
overdispersion was found (§5.2), so the intervals can be believed once the N
is right.

**3. A trained checkpoint is a foreign object in this repo's determinism,
golden and licensing regimes — three separate ways to get stuck at the merge
gate.** (a) *Goldens*: every change to bot decisions moves
`rules/heads_test.go`; B2 moved three of four seat counts, and the rule is one
regeneration per milestone (Ruling FL-83) with the *measured* cause named. A
policy that changes on every retrain wants a regeneration on every retrain,
which the process does not allow — so the network must not be the policy the
acceptance suite runs, which in turn means it is never covered by the
acceptance suite. (b) *Determinism*: the fastest measured trainer (168 K
rows/s at 32 workers) is Hogwild — a real data race that the controller's
`-race` pass will find, and non-reproducible besides; the deterministic
single-threaded trainer is 5.6× slower (still fine, 29.8 K rows/s, but it has
to be chosen deliberately). (c) *Licensing*: the checkpoint would be trained
against `.cards/`, which is GPL-3.0 Forge data that this Apache-2.0 repo
deliberately does not track (`AGENTS.md:14-17`, `cards/boundary_test.go`).
Weights derived from that corpus and committed to this repo are a question
nobody in this project has answered, and it is a question best asked *before*
17 engineer-days, not after. (Note also that nothing currently enforces the
zero-dependency rule — `go list -m all` prints one line today, but no test
says it must.)

### 6.3 The recommendation

**No — do not build the learned policy now. And the fitted heuristic plus
determinized search is not obviously the better place either; my measurements
say naive search on this engine loses to the four-line heuristic by 30-80pp
and would be its own multi-week project.**

What to fund instead, in order:

1. **Phase 0 (2 days).** The `botbench` deck-pair matrix, a shard flag, and a
   documented N≥4000 standard. This is needed by every bot task that will ever
   be dispatched, learned or not.
2. **Phase 1 (3 days).** Point B4's fitted weight table at **spell selection
   at priority** — the surface I measured at 16.8pp sensitivity, four times
   targeting and blocking — rather than at more combat tuning, and run it
   through the matrix. This is the cheap probe of the same band a learned
   policy would be aiming at, with the same objective and a much better signal
   (a human fitting a handful of coefficients against a 70-second bench).
3. **Then re-read this document.** If the fitted table clears +3pp at N=4000
   and is non-negative on ≥50 of 66 pairs, the band is real, §4.3's design is
   ready to execute, and the bet is defensible — a learned scorer over the
   same surface should beat a hand-fitted one, and §1.4's per-option
   representation is the shape that lets it. If the fitted table cannot move
   it, the honest answer is that the band above the trivial rule is narrow,
   gorge's 12-deck fixture does not contain enough exploitable decision
   headroom to pay for 17-21 engineer-days, and the right move is to ship the
   heuristic, keep the matrix, and spend those days on the M2e human-play
   path instead.

A third thing worth saying plainly, because it is actionable today and does
not depend on the gate: **the bot workstream is currently pointed at the wrong
surfaces.** B2 tuned combat (13.0pp sensitive, and ~10 of that is just
"attack at all", which the legacy policy already did). B3 is tuning targeting
(4.3pp). Nobody has touched spell selection (16.8pp). Whatever is decided
about learning, the next hand-written bot task should be there.

The expensive outcome the brief warns about — spending months to rediscover
"cannot beat the teacher with this architecture" — is avoided here not by
picking a cleverer architecture but by refusing to start until a five-day
measurement says there is something to win.

---

## Appendix A — how each number was produced

All probes are throwaway Go modules in the session scratchpad
(`/tmp/claude-1000/…/scratchpad/`), each a separate module with a
`replace github.com/adams-shaun/gorge => /home/sadams/projects/gorge/.worktrees/r1`
directive, reading the corpus from `/home/sadams/projects/gorge/.cards` and
the deck lists from `internal/testutil/decks/`. None of them is in the repo
and none of them was committed. Each reproduces `cmd/botbench`'s loop
(`main.go:159-179`) exactly: `view.Project` → `Seat.Decide` → `Engine.Submit`.

| Probe | Produces |
|---|---|
| `decprobe/` | §1.2 decision histograms, §1.3 non-pass histogram, §1.4 option-kind vocabulary and the 186 priority (kind, card) pairs |
| `costprobe/` | §3.2 per-decision engine cost, §5.3 `Engine.Clone` and playout costs, §5.3 1-ply lookahead costs |
| `matbench/` | §3.1 matvec/matmul GFLOP/s, §3.2 candidate-policy forward pass, §3.3 SGD step throughput |
| `ablate/` | §4.2 per-decision-kind ablation table |
| `search1/` | §5.3 the 1-ply search-seat bench (cheating; upper bound) |
| `cmd/botbench` as shipped | §5.1 baselines, §5.2 the six-block overdispersion check |

**Labelled estimates** (everything else in this document is measured):
self-play throughput of ~5.2-6.0 M games/hour across 32 threads (extrapolated
from the measured 17.5 ms/game plus the measured ~11 µs/decision policy cost);
autodiff at 1,500-2,500 lines; every entry in the engineer-day table; the
8.5 MB checkpoint size (computed from the proposed parameter count, not
measured); the "order of magnitude" gap to a hand-written AVX-512 BLAS.

**Known gaps in the evidence.** The `randchoose` ablation row did not finish
— a `KChoose`-randomising seat appears able to drive a game past the
400,000-intent cap (`cmd/botbench/main.go:96`), which is itself worth a look
and may be a real engine liveness bug rather than a probe artefact. The
`search1` probe cheats by design and its evaluator is untuned, so §5.3's
search numbers are an upper bound on a *naive* search, not a verdict on
search. All benches use the two-seat production deck pair only; §5.4 explains
why that is a defect and Phase 0 fixes it.
