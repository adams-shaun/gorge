# PN22: mana tap choices in sampled-world search

## Question

The search seat normally compares cast choices after the bot has floated mana.
Can comparing which bare mana source to tap improve its play? This opt-in arm
searches alternative `activate` options at a priority decision when the bot
is tapping a source and at least two bare taps are offered. The bot's tap is
candidate 0. Costed mana abilities are excluded. The same sampled worlds and
rollout evaluator already used for cast search score every tap candidate.

## Small paired run

Both arms used search against `bot`, `-search-redeal`, horizon 2, four sampled
worlds, 32 sampler attempts, two workers, and the same seeds within each pair.
The mana arm alone added `-search-mana`. The repo's `.cards` corpus was present.

| Pair | Games | Baseline search wins | Mana-search wins | Mana taps searched | Source changes | Stalls |
|---|---:|---:|---:|---:|---:|---:|
| mono-white-equipment:mono-blue-tempo | 10 | 6 | 6 | 278 | 0 | 0 |
| ur-delver:dimir-tempo | 10 | 5 | 5 | 91 | 2 | 0 |
| **Total** | **20** | **11** | **11** | **369** | **2** | **0** |

The mono pair used seed 300000000; the multicolor pair used 300000100. No
game outcome changed in this smoke run. The multicolor arm changed the first
source twice, so the branch is live. At this sample size it cannot estimate a
small win-rate change. The added search work was substantial: the mono run
asked 399 search decisions with the mana arm versus 121 without it; the
multicolor run asked 169 versus 78. Every asked decision was covered because
the enabled known-card redeal fallback supplied worlds when full replay
sampling starved. The search cost report's mean time per asked decision was
about 244 ms on the mono pair and 120 ms on the multicolor pair with the arm.

**Verdict:** keep the arm opt-in. Searching the order of ordinary mana taps
offers little evidence of a play gain relative to its cost. It also does not
search the complete tap-plan-plus-cast sequence: after each root tap, the
rollout bot handles subsequent mana choices. A useful follow-up would compare
complete payment plans for a *specific cast*, on positions with flexible mana
or mana-producing creatures, before running a larger win-rate gate.

## Outputs and checks

The raw bench reports are in
`/tmp/gorge-pn22-dashboard/pn22/results/{baseline,mana,multicolor-baseline,multicolor}.stdout`
while the dashboard is running. They are scratch artifacts and contain no Forge scripts.
`go test ./internal/searchseat ./cmd/botbench -count=1` passed with the corpus
linked. The flag-off path remains the original candidate set.
