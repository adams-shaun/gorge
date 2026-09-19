# Search-teacher spike: ensemble determinization over the history-conditioned sampler

Date: 2026-09-19. Base: `9be52252` (AR7 lethal pressure is the default bot).
Code: `internal/searchprobe/teacher.go`, `cmd/searchteacher/`.
Question: is an offline **search teacher** (ensemble determinization / PIMC,
Cowling–Ward–Powley 2012) viable in gorge for labelling hard bot decisions
better than the heuristic bot, as a precursor to a learned per-option scorer?

## Answer

**Go, as an offline labelling teacher — with a confidence margin, and not as a
live seat.** Measured on 1,000 development games per decision kind, a seat
that uses the teacher only at its own decisions of one kind and the default
bot everywhere else beats its own same-seed bot-vs-bot twin by:

| teacher answers | paired Δ vs bot twin (95%) | games whose result changed | sign |
|---|---|---|---|
| attackers | **+2.80pp ± 1.10** | 32 | 30 better / 2 worse |
| priority with ≥2 castable cards | **+2.70pp ± 1.86** | 91 | 59 better / 32 worse |

Both intervals exclude zero. The labels are real but thin: the sampler
produces worlds for only 34% (attackers) / 53% (cast) of decisions, and on
covered decisions the teacher's override is usually a tie in truth. A margin
of ≥0.3 in mean outcome turns the attacker labels into near-clean labels
(18 better, 0 worse against the clairvoyant audit).

Everything below is **measured** unless labelled *estimate*.

## What was built (≈ 450 lines, no production change)

- `searchprobe.TeacherChoice(worlds, candidates, opts)`: for every sampled world
  and every candidate answer, clone the world, map the semantic answer into
  that world with the world's own `Collector`, submit it, then roll out with
  `botpolicy.Decide` for **both** seats to game end (or `HorizonTurns`),
  scoring 1/0.5/0 (or a squashed `LeafScore` at a non-terminal horizon). The
  candidate with the best mean wins; ties (and anything within `Margin`) keep
  candidate 0, the bot's own answer. Common random numbers: all candidates on
  one world share the world's chance stream and rollout seed.
- `searchprobe.AttackCandidates`: bot's declaration first, then no attack,
  all-in, and each one-attacker toggle of the bot's declaration; validated,
  deduplicated, never omitting a `Required` attacker, capped at 6.
- Cast candidates reuse the existing `searchprobe.Candidates` (bot's pick,
  pass, other cast/ability options), asked only when the priority decision
  offers ≥2 distinct castable cards and the bot's answer is cast/ability/pass.
- `SampleOptions.MinESS`: optional relaxation of the resampling gate
  (default 0 keeps the calibration contract ESS ≥ K). With `-min-ess 2`, K
  worlds are resampled with replacement from a thinner accepted pool.
- `cmd/searchteacher`: plays each development seed twice — search seat vs bot
  and bot vs bot — on the ten approved mono pairs, search seat alternating
  seats (`g%2`). Because the default bot consumes no randomness outside
  mulligans, the twin is the exact counterfactual, so the paired difference
  isolates the teacher's overrides. Two diagnostic modes:
  - `-oracle`: **cheats** — searches one clone of the actual engine (true
    hidden zones and future chance). A ceiling, never a label.
  - `-audit`: measurement only — after the sampled-world label is fixed, also
    scores the same candidates on a clone of the actual engine and records it.
    Never reaches the choice.

Hidden information: the choice at every non-oracle decision reads only the
worlds `Sample` returned from the seat's own observation history plus the
public deck lists; the actual engine is touched only to capture the seat's
own observation and to submit the chosen intent.

## Method

- Decks: `mono-white-equipment, mono-blue-tempo, mono-black-aggro,
  mono-red-prowess, mono-green-stompy`, all ten unordered pairs.
- Seeds: development only. K sweep at base 30,000,000; benches at base
  40,000,000. Seed `base + pair*games + g`. The tool refuses any range
  touching [1,000,000, 2,000,000).
- Rollout policy: default `botpolicy.Decide` (AR7). Horizon: game end
  (`-horizon 0`); rollout cap 5,000 submits (never hit).
- Sampler: seed base 54321, 64 or 128 proposal attempts per decision.
- Box: 32 cores shared with other work (load ~11 before these runs); 16–22
  workers. **Timing numbers are single-decision wall time under that load.**
- Go 1.26.3; corpus pin `95f04e8a04c8925fa97cb226fc3341cabcc90a53`.

### Exact commands

```sh
go build -o st ./cmd/searchteacher
C=.cards
# K sweep, attackers, 20 games/pair (200 games), seed 30,000,000
./st -cards $C -games 20 -worlds 8 -attempts 64 -workers 16 -out k8a64.jsonl
./st -cards $C -games 20 -oracle -workers 20 -out oracle20.jsonl
for K in 8 16 32; do
  ./st -cards $C -games 20 -worlds $K -attempts 128 -min-ess 2 -workers 20 -out k${K}a128e2.jsonl
done
./st -cards $C -games 20 -oracle -kinds cast -workers 8 -out oracle-cast20.jsonl
# Benches, 100 games/pair (1000 games), seed 40,000,000, with audit
for KIND in attackers cast; do
  ./st -cards $C -seed 40000000 -games 100 -kinds $KIND -worlds 8 -attempts 128 \
       -min-ess 2 -audit -workers 22 -out bench-$KIND.jsonl
done
```

Per-decision analyses (coverage by accepted-count bin, rejection causes,
override-vs-audit, margin sweep) were small Python reads of the JSONL
`GameRecord.Decisions` fields; every field used is written by the tool.

## Results

### 1. Sampler acceptance and coverage — the binding constraint

Attackers bench (5,313 decisions, 128 attempts, K=8, MinESS 2):

| | value |
|---|---|
| proposal acceptance | 55,360 / 680,064 = **8.1%** |
| decisions with **zero** accepted proposals | 3,229 / 5,313 = **61%** |
| accepted 1–7 / 8–31 / 32+ | 866 / 661 / 557 |
| covered (≥ K worlds at ESS ≥ 2) | **1,814 / 5,313 = 34.1%** |
| coverage by engine turn 1–6 / 7–12 / 13+ | 72.1% / 30.5% / 14.8% |

Cast bench: acceptance 19.1%, coverage 53.0% (82% / 32% / 18% by turn band —
cast decisions are earlier in the game).

Acceptance is bimodal: a history is either reproducible by bot-driven
reconstruction or it is not, and more attempts mostly help the thin middle.
The dominant rejection among uncovered decisions is `identities/hand_to_stack`
(2,872 of 3,499), and within it **PolicyCompetition** (a sampled opponent
hand in which the bot would have cast a *different* spell than the one
observed) outnumbers every other cause combined (219k vs 123k attempt-level
rejections). The proposal guarantees observed cards are in hand in time; it
does not stop the sampled hand from holding something the bot prefers.

Coverage is also deck-dependent: search seat mono-white 22%, mono-blue 32%,
mono-green 41%, mono-black 46%.

### 2. K sweep (attackers, 200 games, seed 30,000,000)

| config | coverage | overrides | paired Δ (95%) | changed | ms / labelled decision (sample + search) |
|---|---|---|---|---|---|
| K=8, 64 att, strict ESS≥K | 19.5% | 23 | +0.50 ± 1.70 | 2↑ 1↓ | 660 + 559 = **1,219** |
| K=8, 128 att, ESS≥2 | 37.7% | 42 | +1.00 ± 2.40 | 4↑ 2↓ | 1,293 + 678 = **1,972** |
| K=16, 128 att, ESS≥2 | 38.7% | 58 | +2.00 ± 2.76 | 6↑ 2↓ | 1,420 + 1,393 = **2,813** |
| K=32, 128 att, ESS≥2 | 39.2% | 60 | +2.50 ± 2.93 | 7↑ 2↓ | 1,628 + 3,036 = **4,664** |
| clairvoyant oracle (cheats) | 100% | 15 | **+7.50 ± 3.66** | 15↑ 0↓ | 0 + 99 |

At N=200 no sampled row is individually significant; the trend is monotone in
K and the direction is consistent. Relaxing the ESS gate roughly doubles
coverage for about 1.6× the sampling cost. K beyond 8 raises the override
count but search cost grows linearly; K=16 is the likely knee (*estimate*:
not resolved at this N).

### 3. The two benches (1,000 games each, seed 40,000,000, K=8, 128 att, ESS≥2)

| | attackers | cast (≥2 castable) |
|---|---|---|
| search seat WR | 53.1% ± 3.1 | 53.0% ± 3.1 |
| bot twin WR (same seeds, same seat) | 50.3% ± 3.1 | 50.3% ± 3.1 |
| **paired Δ** | **+2.80pp ± 1.10** | **+2.70pp ± 1.86** |
| changed outcomes | 30 better / 2 worse (sign test p ≈ 3e-7) | 59 better / 32 worse (p ≈ 0.005) |
| decisions asked | 5,313 | 5,616 |
| covered | 1,814 (34.1%) | 2,979 (53.0%) |
| teacher disagrees with bot (of covered) | 219 (12.1%) | 791 (26.6%) |
| covered decisions where every candidate ties | 1,197 (66%) | 1,207 (41%) |
| ms/labelled decision, sample + search | 1,584 + 882 = **2,467** | 776 + 1,347 = **2,122** |
| errors / stalls | 0 / 0 | 0 / 0 (10 rollout panics → bot fallback, see §6) |

Per-pair (search / twin, n=100 each): no pair where the search seat is below
its twin by more than 4 games; largest gains mono-black:mono-green +9
(attackers) and mono-white:mono-green +14 (cast). Per-pair n is too small to
read individually.

The paired interval is the Wald interval over per-game differences in
{−1, 0, +1}; with only 32 and 91 non-zero differences the sign test is the
more conservative reading and agrees.

### 4. Label quality against the clairvoyant audit

The audit scores each covered decision's candidates on a clone of the actual
engine with bot rollouts — i.e. what each answer would actually have led to
under bot continuation in *this* game.

| | attackers | cast |
|---|---|---|
| covered decisions where the truth differs across candidates | 184 (10%) | 598 (20%) |
| … bot's answer is a true-best option | 69.0% | 62.4% |
| … teacher's answer is a true-best option | **84.2%** | **66.9%** |
| teacher overrides: truly better / equal / worse | 31 / 185 / 3 | 78 / 662 / 51 |

Most overrides are harmless no-ops in truth. Precision rises sharply with the
teacher's own confidence margin (mean value of its pick minus the bot's
pick; K=8 gives steps of 0.125):

| margin > | attackers overrides: better / worse / equal | cast overrides: better / worse / equal |
|---|---|---|
| 0 | 31 / 3 / 185 | 78 / 51 / 662 |
| 0.2 | 26 / 2 / 93 | 53 / 24 / 283 |
| 0.3 | 18 / 0 / 42 | 25 / 9 / 126 |
| 0.5 | 13 / 0 / 9 | 12 / 2 / 23 |

So cast labels are noisy at margin 0 (better:worse 1.5:1) and usable at ≥0.3
(2.8:1); attacker labels are clean from 0.2 up.

### 5. Ceilings (clairvoyant oracle, 200 games, **cheats**)

- attackers: +7.50pp ± 3.66, 15 overrides in 1,019 decisions (1.5%), 87% of
  decisions tie in truth.
- cast: +15.0pp ± 4.96, 30 overrides in 1,094 decisions (2.8%).

These know the future library order, so they overstate what any honest
search can reach; they bound the value on these kinds under bot continuation
and show that pivotal decisions are rare (1.5–3%) but each one flips a game.
The honest teacher recovers ~35% (attackers) and ~18% (cast) of the ceiling
at 34–53% coverage.

### 6. Engine bug found (not fixed — outside this spike's write scope)

`rules/livelock.go` `detect()` (line ~274) loops
`for j := n - 1; j >= n-2*p+1; j--` comparing `sigAt(j)` with `sigAt(j-p)`.
With `p` up to `n/2`, `j-p` reaches `n-3p+1 < 0`, and `sigAt` indexes `-1`
while the window is not yet full (`sigHead == 0`). `Engine.Clone` resets the
watcher (`rules/clone.go:95`), so a short post-clone window with a repeating
signature panics. Measured: 14 panics in 1,094 clairvoyant cast rollouts and
10 in the cast bench. Comparing the two trailing halves needs
`j >= n-p`. `TeacherChoice` converts the panic into an error and the seat
falls back to the bot. Live games never clone, so production is exposed only
in the (theoretical) `sigHead == 0` wrap case.

## Cost per labelled decision

- **Measured:** 2.1–2.5 s wall per labelled decision at K=8 / 128 attempts
  (1.2 s at 64 attempts strict; 2.8 s K=16; 4.7 s K=32), on a loaded box.
  Every *asked* decision pays the sampling cost (0.7–1.3 s mean) even when it
  falls back, so the cost per **covered** label including failed asks is
  ≈ 1.3 s × (1/0.34) + 0.9 s ≈ **4.7 s** (attackers) and ≈ 0.75 × (1/0.53) +
  1.35 ≈ **2.8 s** (cast) — *estimate from the measured means*.
- *Estimate:* ~32 cores → ~25–40k covered labels/hour; of these only 10–20%
  are decisive in truth and ~3–12% carry a margin ≥ 0.3 override. A 100k
  covered-label corpus is ~3–4 hours of the box; a 10k *confident override*
  corpus is closer to a day.
- The bench itself (1,000 games, both kinds' sampling at every eligible
  decision) took 408–489 s wall on 22 workers.

## Recommendation

**Go** for using this as an offline labelling teacher for a per-option scorer,
on these terms:

1. **Settings:** K=16 worlds, 128 attempts, MinESS 2, game-end rollouts with
   the default bot, candidates ≤ 6. Record per-candidate mean values, not
   only the argmax — the scorer should regress on values, which also keeps
   the ~60% tie information ("these are equivalent") instead of discarding it.
2. **Confidence gate:** treat an override as a positive label only at margin
   ≥ 0.25 (attackers) / ≥ 0.3 (cast); below that, label "bot's answer is
   fine". Measured precision there is 18:0 and 25:9.
3. **Kinds:** attackers first (clean labels, +2.8pp). Cast is as valuable in
   aggregate but half its raw overrides are wrong in truth; use it only
   through the margin gate.
4. **Not as a live seat:** 2–5 s per decision is fine offline, not in play.

What would have to be true for the scorer to be worth it — and what to
measure next:

- **The scorer must generalise into uncovered decisions.** Only 34–53% of
  decisions (15–18% after turn 12) get labels, and the uncovered ones are
  systematically later and more complex. Measure the scorer's gain on
  decisions the teacher could not label; if it is ~0, the programme caps at
  roughly the teacher's own +2.8pp per kind.
- **Coverage is the lever, not K.** The next engineering unit is the
  PolicyCompetition rejection: bias the opponent-hand proposal away from
  cards the bot would have preferred over the observed cast (or reweight
  instead of reject). Doubling coverage should roughly double the labelled
  population; K beyond 16 buys little.
- **Confirm on held-out once**, per the L-series gate: the combined
  attackers+cast search seat vs `bot` at seed 1,000,000, 400/pair. The
  development result predicts roughly +3–5pp combined (*estimate*, the two
  kinds are measured separately here and may not add).
- **Fix the livelock `detect()` bound** before any large corpus run so clone
  rollouts stop dropping ~1% of cast decisions.

What this spike does **not** show: that a learned scorer trained on these
labels beats the bot; that the gains hold on non-mono decks or on the
held-out range; or anything about blockers/targets.

## Relation to R1 §5.3

R1's naive 1-ply search seat scored 16.5–23.5% against the bot. The three
reasons it gave all changed here: no hidden-information cheating (sampled
worlds; the cheating version is kept only as a labelled ceiling), the leaf is
the game's end rather than 0.3 steps away, and there is no hand-tuned
evaluator. The result flips from −30pp to a small, significant positive,
applied narrowly and defaulting to the bot whenever the teacher is unsure or
has no worlds.
