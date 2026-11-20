# Search and nets

Search needs an engine that is cheap to copy and step. A net needs a runtime that evaluates it in pure Go. This page says what ships and where it runs.

```mermaid
flowchart LR
  R[Root decision] --> W[Worlds]
  W --> T[Tree]
  N[Net] -.->|prior, value| T
  T --> A[Answer]
```

## Which search runs where

| Search | gorge | dirac |
|---|:-:|:-:|
| PUCT tree over redealt worlds (`bots/azmcts`, `dirac/search`) | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> (`dirac/search`) |
| Sampled-world rollouts (`bots/searchseat`) | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--none" title="not in this repository"></span><span class="gg-sr">none</span> (only the `dirac.Rollout` primitive) |
| Heuristic bot, no search | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> (`dirac/search/hbot`) |
| Mulligan policies | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> (`dirac/search/mull`) |
| Search primitives: copy, step, hide, redeal | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> | <span class="gg-dot gg-dot--full" title="full support"></span><span class="gg-sr">full</span> |

`dirac/search` is `bots/azmcts` ported onto dirac state: the same algorithm, with `DefaultOptions` set to the production config (`FullKinds`, top-6 priors, opponent nodes, closed deck list).
`gorged -engine dirac` still refuses a search policy.
[What dirac offers a search](../architecture/dirac/search.md) lists the operations.

## gorge's tree search

=== "Settings"

    Defaults from `azmcts.DefaultOptions`: 100 simulations per decision, exploration constant 1.5, first-play reduction 0.1, at most 6 candidates, 1,000 steps per simulation.
    `AllKinds` (the default) searches priority, attackers, blockers and target. `FullKinds` adds modes, may-triggers, small choices and multi-target picks.
    Every other decision goes to the default bot. There are no rollouts: each leaf is scored once, by a heuristic or by a net.

=== "Worlds"

    The world source decides what the search may know.

    <span class="gg-badge gg-badge--fair">fair</span> `redeal` re-deals everything the seat can't see, from its own observations.
    `redeal-closed` also hides the opponent's deck list, drawing a believed list from a corpus you supply.

    <span class="gg-badge gg-badge--seer">clairvoyant</span> clones the real engine. The source for it is not in this repository, and the seat refuses it unless a command injects one.

=== "Hosted"

    `az-redeal` is the hosted entry: the `redeal` world, 100 simulations, no net, constructed or Commander, up to four seats, experimental tier.
    `gorged.RegisterNetBot` links a policy that loads a net at startup and adds `-<policy>-net`, `-<policy>-decks`, `-<policy>-belief` and related flags.

## gorge's sampled-rollout seat

`bots/searchseat` is a different search, hosted as the `search` policy (experimental tier). It samples K worlds per decision
(`searchseat.Defaults`: 8), rolls each candidate out to the end of the game (or a turn horizon), and overrides the bot only when a candidate beats its answer by a margin.
It covers attackers and the cast choice by default.

Per-simulation cost is on [Bot speed](../performance/bots.md).

## The net runtime

Nets run in pure Go, with no cgo and no external ML runtime. Two packages read the same `.dzgw` file.

<div class="grid cards" markdown>

-   **`bots/gonet`**

    Loads `mlp` and `gnn` nets. Caches evaluations and card vectors. The graph net uses AVX-512 kernels when the CPU has them.

-   **`bots/netsimd`**

    Whole-layer AVX-512 kernels for the `mlp` net, with a scalar twin that gives the same bits. `dirac/neteval` loads it first, and falls back to gonet for a shape it refuses.

</div>

| What | Limit |
|---|---|
| Architectures | `mlp` and `gnn` in gonet; `mlp` in netsimd |
| netsimd widths | `d`, `ff`, `card_d`, `head_hidden` multiples of 32; `d_emb` any; 1 to 64 trunk blocks |
| Fast path | `d` = `head_hidden` = 64 |
| CPU paths | AVX-512 (F, BW, DQ, VL) on amd64; scalar Go everywhere else, and for `ForceScalar` |
| Raw card width | gonet 68 or 100; netsimd 68, 84 or 100 |

The file format and how a net is loaded are on [Training data and nets](training-data.md).
