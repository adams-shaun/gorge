# MageZero replication

[MageZero](https://github.com/WillWroble/MageZero) is an AlphaZero-style Magic
agent built on an XMage fork. It encodes every decision as a typed graph and
trains a graph network ("NetGraph") on MCTS self-play. In October 2026 we
replicated its published results and then ran the same experiments on
dirty-dirac, the search kernel described under
[Architecture](../architecture/dirty-dirac/index.md). The goal was to decide
whether MageZero's encoding or network should replace ours.

!!! abstract "Verdict"
    - **The replication is faithful.** Their code on their data, their release
      jar on our hardware, and their gen1 model rerun here all land within noise
      of their published numbers.
    - **Their encoding is not a gain for us.** On their own data our 166K-parameter
      MLP ties their 8.8M-parameter NetGraph on value and targets and is about
      30x cheaper to run. On our data, NetGraph is no better either.
    - **The two engines play the same decks differently.** On identical lists,
      at the same search budget, with no network, Faeries wins more on
      dirty-dirac than on XMage against four of seven opponents. Matching the
      option list, the leaf evaluator and the search math does not close that gap.
    - **Adopted:** the encoder ships as an opt-in, off-by-default recording
      flag, measured free when off. NetGraph is not adopted.

Their sources are `results/experiments.md`, `results/generations.md`,
`magezero-faeries-findings.md` and `results/roundrobin-trainlog.txt` on
MageZero's `graph-encoder-test` branch (8551e53), with the XMage fork at
`graph-encoder` a031818 ("v0.3"). Where this page gives one of their numbers,
it comes from those files or from their published data shards (sha256-checked).

## Setup in one paragraph

Both systems play two-player constructed Magic with **clairvoyant** MCTS: the
search sees the opponent's hand and library order. Every search uses 300
simulations, auto-paid mana and no mulligans. The decks are their nine Pauper
lists (Faeries as the learning seat), their five "MTGA mono" starter decks,
and their UWTempo list. At gen0 neither side has a network: the search runs
on a uniform prior and a hand-written leaf evaluator. Later generations add a
trained net as the prior and value function.

| Term | Meaning |
|---|---|
| **ver4** | Their published gen0 Pauper training data (v0.3 encoding), 2,576 games |
| **jar rerun** | Their v0.3.0-alpha release jar, run on our hardware with their gen0 config (Java 21, no rebuild) |
| **dirty-dirac** | Our kernel (`kplay`) with research card sets built from their lists |
| **NetGraph** | Their network, trained by their unmodified `train.py` |
| **d64 MLP** | Our small production-family net: 1 residual block, d=64, 166K parameters |

## Which comparisons are like-for-like

This section is the re-evaluation. Each row is a claim on this page and how
far its two sides were actually matched.

| Comparison | Matched | Not matched | Status |
|---|---|---|---|
| Their `train.py` on their data, ours vs theirs | code, data, settings | seed, GPU instead of CPU | **like-for-like** |
| Their jar's gen0 Faeries win rate, rerun vs published | jar, config, decks | sample size (399 vs 2,576 games) | **like-for-like** |
| Their gen1 model on their jar, rerun vs published | model, jar, recipe, opponents | 100 vs about 200 games per opponent | **like-for-like** |
| Their `test.py` "classic test" table | model, data, code | nothing material | **like-for-like** |
| d64 MLP vs NetGraph on their data | test states, candidates, metric | priority objective: theirs is unmasked over every ability node | **like-for-like on value and target only** |
| NetGraph vs d64 MLP on our data | records, candidates, metric | NetGraph sees our states through a Go port of their encoder | **like-for-like, through a verified port** (id-parity tested) |
| Faeries gen0: XMage vs dirty-dirac | lists (no substitutes), budget, clairvoyance, leaf, PUCT scale, option list | tree reuse, duplicate-state merging, combat factorisation, rules implementation | **close; residual = engine** |
| UWTempo vs monos: **dirty-dirac 0.642 vs MageZero gen0 0.347** (earlier version of this page) | deck lists (2 substitutes), 300 sims | **opponent pilot** (our heuristic vs XMage depth-6 minimax); their gen0 ran at **150** sims | **not like-for-like**; replaced by [the training curve](#uwtempo-training-curve) |
| UWTempo heuristic pilot 0.261 vs their minimax pilot 0.16 | deck lists | both pilots *and* both opponents differ | **not comparable** |

The two UWTempo rows were the weak point of the first write-up. They crossed
opponent strength with pilot strength, so "our gen0 search already matches
their best RL generation" said more about how weak our opponent pilot is than
about our search. The [training curve](#uwtempo-training-curve) section
redoes the comparison against an opponent calibrated to their minimax.

## Their results reproduce

**Architecture table.** Their `train.py` on their data gives a validation
total of 0.885 against their published 0.905 (their seed noise is about
0.04). On our engine's data in their encoding, their architecture ranking
holds: default ≈ d128 < one message-passing pass < parallel passes < flat
objects ≪ bag of nodes (degenerate, as in theirs).

??? note "Architecture table, theirs and ours"
    Validation total = their `test.py` sum of losses. It compares within one
    data set only.

    | their run | arch | theirs (ver4) | their code on their data | their code on dirty-dirac gen0 |
    |---|---|---|---|---|
    | s6 d256-drop0 (default) | d256, ff512, 2 sequential passes | 0.905 | 0.885 | 0.484, 0.492 (2 seeds) |
    | s8 d256-seq1-drop0 | 1 pass | 1.006 | not run | 0.501 |
    | s9 d128-drop0 | d128 | 0.920 | not run | 0.516 |
    | s10 parallel | parallel passes | 1.143 | not run | 0.659 |
    | s9 flat | flat objects | 1.472 | not run | 0.782 |
    | s1 bag | no edges | degenerate | not run | 4.824 (degenerate) |
    | ours: perfect info off | opponent hand hidden | – | – | 0.485 |

**Gen0 win rate.** Their jar on our hardware, 399 games, gives Faeries
**0.449**, the same as ver4's 0.449 over 2,576 games. Decisions per
seat-game match too (86.7 vs 81.7).

**Gen1.** Their published gen0 model drove their jar here: Faeries searches
value-only at 300 visits with TD discount 0.92 against the gen0 opponents.
Every rerun beats its gen0 baseline and lands within about two standard
errors of their figure. Pooled over the five monos, the rerun wins **0.410**
against their **0.396**. Their behaviour probe (`behaviour.py`) on our games
reproduces gen1's "patient" style: more Pass, far fewer Ninja casts.

<figure class="mzfig">
--8<-- "bots/magezero/gen1-rerun.svg"
</figure>

**Classic test.** Their `test.py` with their published model on all 11 of
their test sets gives identical state counts and every metric within 0.003 of
theirs.

## The engines: same decks, different games

### What each engine asks

<figure class="mzfig">
--8<-- "bots/magezero/decisions.svg"
</figure>

- **Combat is factorised differently.** XMage asks one target question per
  attacking or blocking creature (pick one, or stop). That is 30% of all
  their recorded states. dirty-dirac asks one subset question per declare
  step. A net trained on one learns per-creature marginals; a net trained on
  the other learns a joint choice.
- **XMage records 36% more decisions per game** (81.7 vs 52.4). The gap is
  combat plus main-phase priority. Most of the main-phase gap is land plays,
  which dirty-dirac's default search does not treat as a decision.
- **Matched:** mana payment (both auto-pay), priority stops (every step on
  both turns; a lone Pass is auto-passed), mulligans (off) and pass semantics.

Two follow-ups showed that combat structure does not explain the win-rate
differences below. Widening dirty-dirac's six-candidate cap to 16 moved the
gen0 win rate by +0.008 [−0.018, +0.033] over 650 paired deals. Asking per
creature, XMage-style, moved it by −0.023 [−0.052, +0.006], and by −0.022 at
equal time.

### Faeries gen0, per opponent

<figure class="mzfig">
--8<-- "bots/magezero/faeries-gen0.svg"
</figure>

The grey band runs from their published number to our rerun of their jar,
which is the range XMage itself produces. "Default search" is dirty-dirac's
gen0 spec on MageZero's exact lists. "Matched" adds every opt-in key that
copies MCTS2:

- the same option list: land plays searched, no candidate cap, alternative
  casts such as bestow and plot, duplicate casts merged;
- their leaf evaluator (`GameStateEvaluator3`);
- their PUCT scale and unvisited-node value;
- their choice-node scoring and backprop discount.

Once matched, Elves sits inside the band and the mirror at its edge. Spy, Affinity,
Wildfire and Burn stay above it, by +0.08 to +0.15 (z 1.9 to 3.1 against the
pooled XMage data). What we measured on the way:

- **The PUCT scale is the one search key that moves results**, by 3–6
  points toward XMage on Spy, Affinity and Elves. The other search keys are
  noise-level.
- **One rules bug, now fixed.** dirty-dirac could not activate Saruli
  Caretaker's `{T}, tap an untapped creature` mana ability. Fixing it moved
  Faeries vs Spy from 0.786 to 0.710, about a quarter of that gap. The fix
  also made the kernel refuse the 96 cards whose mana ability it still cannot
  activate, instead of playing them silently wrong.
- **Not tree reuse.** XMage with tree reuse switched off plays the same:
  Faeries vs Affinity 0.51, attack rate 0.93, pre-combat cast share 0.80.
- **Not deck-specific to Faeries.** Two matchups without Faeries (Affinity vs
  Wildfire, Elves vs Burn) agree between engines within noise.
- **Per-card play matches.** Faeries casts every card as often as on XMage,
  on the same side of the turn. Opponents choose the same plays per legal
  window.
- **Not the ninjas.** Ninja behaviour matches: ninjutsu taken in 84.6% vs
  87.8% of offers, and the same timing. Removing all eight ninjas moves both
  engines, but by different amounts against Wildfire.
- **Open: combat.** On XMage, Faeries gets a combat-damage draw ask 2.09
  times per game against 1.12 on dirty-dirac, yet wins less. The next step is
  unblocked-attacker and block rates per engine.

??? note "Search-matching ladder, all arms (400 games per matchup, paired deals)"
    "Play keys" = land search, no candidate cap, alternative casts and
    duplicate-cast merging. "All four" = play keys + their leaf + PUCT scale +
    unvisited value + their choice/backprop scoring. CIs are about ±0.048.

    | Faeries vs | ver4 / jar | default | + play keys | + their leaf | + PUCT 0.5 | all four | all four, 140 sims |
    |---|---|---|---|---|---|---|---|
    | Spy | .47 / .36 | .677 | .598 | .598 | .557 | .537 | .657 |
    | Affinity | .40 / .48 | .590 | .637 | .603 | .573 | .575 | .585 |
    | Elves | .36 / .51 | .460 | .440 | .440 | .403 | .443 | .415 |
    | Wildfire | .58 / .46 | .645 | .703 | .667 | .690 | .665 | .652 |
    | Burn | .56 / .52 | .615 | .650 | .615 | .625 | .635 | .667 |
    | mirror | .51 / .54 | .495 | .485 | .515 | .525 | .505 | .505 |

    The 140-simulation arm is the tree-reuse equivalent: XMage's reused tree
    leaves about 140 new simulations per decision.

??? note "Behaviour probe on the Pauper pool, three sources"
    Their `behaviour.py`, pooled over the seven non-mirror opponents.

    | | theirs ver4 | jar rerun | dirty-dirac |
    |---|---|---|---|
    | games / Faeries win | 2,177 / 0.438 | 270 / 0.437 | 4,334 / 0.493 |
    | Seer / Miscreant top move, no Miscreant out | 0.38 / 0.44 | 0.36 / 0.37 | 0.33 / 0.39 |
    | Seer / Miscreant top move, a Miscreant out | 0.06 / 0.79 | 0.07 / 0.87 | 0.11 / 0.75 |

    | chosen per game (own-turn share) | theirs ver4 | jar rerun | dirty-dirac |
    |---|---|---|---|
    | Faerie Seer | 1.38 (1.00) | 1.54 (1.00) | 1.45 (1.00) |
    | Spellstutter Sprite | 1.31 (0.92) | 1.48 (0.93) | 1.33 (0.90) |
    | Humbling Elder | 1.45 (0.84) | 1.51 (0.83) | 1.45 (0.89) |
    | ninjutsu | 0.91 | 0.97 | 0.95 |
    | Counterspell | 0.66 (0.08) | 0.68 (0.14) | 0.60 (0.09) |
    | Saiba Cryptomancer | 0.35 (0.94) | 0.13 (0.97) | 0.35 (0.94) |

## The networks

### Their data, both nets

Both nets are scored on the same 63,518 ver4 mirror-test states, the same
candidates and the same metric. NetGraph's per-node scores are restricted to
the legal candidates and renormalised.

| net | params | value MSE | target top-1 | priority non-pass top-1 |
|---|---:|---:|---:|---:|
| NetGraph d256 (theirs, their `train.py`) | 8.8M | **0.0366** | 0.720 | 0.457 |
| d64 MLP (ours, via a bag-of-nodes bridge) | 166K | 0.0373 | **0.738** | **0.837** |

Value and target are the fair encoding comparison, and they are equal. The
priority gap is mostly the training objective, not the encoding. NetGraph is
trained with a softmax over all ~80 ability nodes in the graph, including
uncastable ones, so it has to learn legality; ours sees only legal
candidates. Their own reported non-pass recall (0.42–0.49) matches our
measurement of their checkpoint.

**Cost.** On CPU at batch 1, NetGraph takes 4.9 ms per state and the d64 MLP
0.18 ms (27x). On one GPU at batch 512, NetGraph does 31K states/s and the
MLP 1.05M (34x).

### Our data, both nets

Same records and candidates for both, from dirty-dirac gen0 games on the real
lists. NetGraph reads them through a Go port of their `StateEncoder`.

| net | value MSE | priority KL | target top-1 |
|---|---:|---:|---:|
| NetGraph (their encoding, ported) | 0.0591 | 0.0287 | 0.473 |
| d64 MLP (their targets) | 0.0589 | **0.0272** | **0.546** |

On our data their net is no better: value is equal, priority is within 0.002
KL, and the MLP leads on targets.

## UWTempo training curve

MageZero's README headline is UWTempo learning against five "MTGA mono"
starter decks piloted by XMage's depth-6 minimax. It goes from 37% at gen0 to
64% at gen17, at 1,000 games per generation. Their log adds three details
the README omits:

- gens 0–6 searched at **150** simulations, and 300 from gen7;
- the policy prior was **off** until gen12, so early generations are
  value-guided search only;
- after gen17 they restarted with a new value labelling scheme (a second line
  running "gen17b" to "gen23b", 0.56 to 0.66).

<figure class="mzfig">
--8<-- "bots/magezero/uwtempo-curve.svg"
</figure>

Their average panel reads 1–3 points above the mean of their own
per-opponent points at gens 0–2. The curve here (35% at gen0, 65% at gen17) is computed from the
per-opponent numbers in their log (200 games each).

**The dirty-dirac run.** <!-- UWRL:RESULTS -->Running. dirty-dirac trains
UWTempo from scratch on the same schedule: gen0 and gens 1–6 at 150
simulations and 300 from gen7, a value-only prior through gen11, then the
policy prior, 1,000 games per generation, 200 per mono. We have no port of
XMage's minimax, so the opponent pilot is calibrated instead: we pick the
dirty-dirac pilot that reproduces both of their anchors (a heuristic UWTempo
at about 16% and a gen0-search UWTempo at about 35%). This page is updated
when the run finishes.<!-- /UWRL:RESULTS -->

## The encoder as an opt-in flag

`kplay -mzxgraph FILE` writes MageZero's graph encoding (a Go port of their
`StateEncoder`, id-parity tested against their Python vocabulary) as a
sidecar beside the training corpus. `-mzgraph FILE` writes our own features
as a typed graph. Both are off by default. Off, games are byte-identical in
12 of 12 benchmark runs, and the instruction-count ratio is 1.0008 median,
inside run-to-run noise.

## Open questions

- **Combat.** Why XMage's Faeries gets twice the combat-damage draw asks and
  still wins less. This is the remaining engine lead.
- **Opponent strength.** How our heuristic opponent compares with XMage's
  depth-6 minimax head to head. The calibrated UWTempo run measures it
  indirectly, but no direct match has been played.
