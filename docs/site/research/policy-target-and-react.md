# The policy target, and learning instant timing

*Two A/B tests, 2026-10-24 → 10-25*

Two pre-registered A/Bs on the from-scratch MLP self-play loop (sploop, the f84x line). The first asked whether the policy target was the bottleneck. The second asked whether self-play could learn when to cast instants without losing strength. Neither changed the network; both changed what the network is trained on.

Every gate is 4,000 games (2,000 pairs, seats swapped), `az:sims=100` on both sides, seed 78, the eval split of pool17k. The 95% half-width at 4,000 games is ±0.0155.

<div class="grid cards" markdown>

-   <span class="gg-dot gg-dot--none" title="not adopted"></span><span class="gg-sr">not adopted</span> **Policy target**

    cqraw is level with cq at g9 (0.50475), and PCR is level (0.5016) for about 1.45× the self-play compute.

-   <span class="gg-dot gg-dot--full" title="adopted"></span><span class="gg-sr">adopted</span> **React + held casts**

    Strength unchanged (0.4965); opponent-turn share 20.9% → 35.2%, against the human 49.6%.

</div>

## Policy-target quality

### Question

Every sploop generation trained with the `cq` policy target and a value target blended from outcome and root value. On the g8 pack (40k states) the trained prior already agreed with the visit counts: KL(visits‖prior) 0.076, and the top-1 visit matched the prior's top-1 87% of the time. The cq target rescales completed Q min-max per state, which makes it about 0.93 one-hot. 39% of states have a root Q span below 0.02; there the target's KL is 0.62 and its top-1 flips 39% of the time, so the net is trained hard on noise. Two fixes were tried:

- **Q, cqraw:** completed Q without the per-state rescale.
- **P, PCR:** playout-cap randomisation, which existed in the kernel's search (FullProb/CheapSims) and had never run in a loop: `sims=600:pcr=0.25:cheap=100`.

### Setup

- Every arm used the same pinned play and pack binaries; arm Q ran on a later trainer build.
- Parent: sploop g8, warm start, 6 generations with equal self-play games per generation across arms, and 1,000-pair gates.
- Arms:
    - C: 200 sims.
    - P: 600 sims with `:pcr=0.25:cheap=100`, recording every decision. The plan expected ~3% more wall time; measured, P's self-play ran at 4.2 games/s against C's 6.1, about 1.45× the compute.
    - Q: 200 sims, cqraw.
- **Rule, fixed before launch:** adopt X only if X6 vs C6 ≥ 0.5155, X6 vs g8 ≥ C6 vs g8, and X's vs-bot at g5 and g6 is no more than 0.02 below C's. Reject at X6 vs C6 ≤ 0.4845. Otherwise extend X and C to 9 generations and gate X9 vs C9 once, adopting at ≥ 0.5155. Loss and KL are logged and never decide.

### Results

| Gate | Score (4,000 games, ±0.0155) |
|---|---|
| C6 vs g8 | 0.51625 |
| Q6 vs C6 | 0.497375 |
| Q6 vs g8 | 0.504625 |
| P6 vs C6 | 0.50725 |
| P6 vs g8 | 0.514875 |
| Q9 vs C9 | 0.50475 |
| P9 vs C9 | 0.501625 |

Both g6 gates fell in the inconclusive band, so both arms ran to g9. vs bot stayed between 0.92 and 0.94 in every arm (P: g7 0.932, g8 0.928, g9 0.935).

### Verdict

**Neither adopted.** The loop keeps cq and no PCR. The calibration fact stands, since the 39% of near-tie states do get the largest cq updates, but fixing it does not move play. The plan names auxiliary heads and a TD(λ) value target as the next levers.

## The react rule and held-cast recording

### Question

DraftZero's experiment 48 ("learning when to cast instants") found that the rule at play alone does nothing: here, the instwin head-to-head gave 0.5035 / 0.498 ([instant timing](instant-timing.md)); for DraftZero, 49.1%. Its gain came from recording held casts as zero-visit candidates. The test asked whether self-play with `:instwin=react`, with held casts recorded, moves instant timing toward human play without losing strength.

### Setup

- Code: the react rule and held recording in the self-play driver, a `+held` pack format (a held-candidate column), and a trainer change: held candidates get 0 in the visits, cq and cqraw targets, and sims-0 records skip the value blend.
- Warm start from the policy-target test's winner. Neither P nor Q won, so the winner is arm C, starting from its g6.
- Arms: C2 is the winner's recipe exactly; R2 is the same recipe with `:instwin=react` in the self-play seats only. No rule is active in any gate. 6 generations, equal games per generation, 1,000-pair gates.
- The first launch failed because the react binaries lacked the v2 entity width; it was relaunched.
- Litmus: each net against itself with no rule at play, `az:sims=100`, 1,000 pairs, seed 78, eval split, with the instant census on.
    - L1: the opponent-turn share of instant-speed casts.
    - L2: own upkeep and draw-step instant-speed casts per 100 seat-games.
    - Human reference: 49.6% overall.
- **Rule, fixed before launch:** adopt only if (S) R6 vs C6 ≥ 0.4845, (T1) L1(R6) ≥ L1(C6) + 5.0 points, and (T2) L2(R6) ≤ 0.5 × L2(C6).

### Results

| Measure | Winner g6 | C6 | R6 |
|---|---|---|---|
| R6 vs C6 (4,000 games) | | | 0.4965 |
| vs the winner's g6 (4,000 games) | | 0.511625 | 0.516875 |
| L1 opponent-turn share (human 49.6%) | 19.7% | 20.9% | 35.2% |
| L2 own upkeep + draw casts per 100 seat-games | 95.65 | 87.20 | 4.10 |

<figure markdown="span">
--8<-- "research/figs/instant-timing.svg"
</figure>

| Card | C6 | R6 |
|---|---|---|
| Stab | 30.1% | 35.2% |
| Luminous Rebuke | 15.7% | 35.0% |
| Faebloom Trick | 5.1% | 29.4% |
| Goblin Surprise | 7.1% | 22.0% |
| Giant Growth | 25.6% | 32.5% |
| Refute | 96.6% | 89.2% |

### Verdict

**Adopted (react + held).** S passes (0.4965 ≥ 0.4845), T1 passes (35.2% ≥ 20.9% + 5.0), and T2 passes (4.10 ≤ 43.6). Strength is unchanged and the timing moves well toward humans, though opponent-turn share is still 35.2% against the human 49.6%.

!!! warning "Caveat"
    The react test ran with the v2 entity block zeroed in both arms, because the react binaries' kernel feature width was 84. The relative result stands; absolute strength is not comparable with the policy-target test or sploop.

The next step this leads to, separate policy and value windows and a league buffer, is in the [open questions](index.md#open-questions).
