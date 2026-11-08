# Instant timing and the entity relational gap

*2026-10-20 → 10-22*

Does the net play instants well? Humans on 17Lands FDN cast about half their instants on the opponent's turn; the bots cast about a fifth there. Does it take kills when they are available? And can the value evaluation see stack targets, or who blocks whom?

The starting measurement: "Bots cast ~21% of instants on the opponent's turn; 17lands FDN humans cast ~50% (z=-10)". For removal, bots 0–14% against humans 37–61%; for combat tricks, bots 0% against humans 61–84%.

!!! abstract "Verdict"
    - The relational block was **adopted** (it is in the 100-wide encoding), but it unlocked no training gain.
    - Rules at play only move casts to main 1; nothing in the training signal rewards holding. The run notes' conclusion: "The loop is flat on every knob tried; it is not an encoding problem and not simple overtraining."
    - This led to the training-side change (held casts recorded as zero-visit candidates), which moved opponent-turn share from 20.9% to 35.2% at equal strength ([react](policy-target-and-react.md#the-react-rule-and-held-cast-recording)).

## What was done

<div class="grid cards" markdown>

-   **1. The relational entity block**

    Columns 68–83 of the entity record: targeted by me / by the opponent, target counts by zone, stack X, kicked, combat foes and their power and toughness, dies, kills ([architecture](architecture.md#entity-records-100-floats-per-visible-card)). Landed as gorge 333264b3b, with matching changes in the kernel and the trainer; old nets play identically (zero-padded). The run notes call it entity format 3; the specification that defined it calls it format 2.

-   **2. A fresh control series, f84**

    26 generations of about 15 minutes each, sims 200, window 4.

-   **3. Knob rungs on that loop**

    Value thinning, 400 sims, playout-cap randomisation, window 12, depth, relational gain, a `:settle` leaf.

-   **4. Rules at play**

    `instwin=noupkeep` and `instwin=noupdraw`.

-   **5. Behaviour probes**

    Kill censuses, forced kills, a stack-target probe.

</div>

## Results

| Try | Result |
|---|---|
| relational block, stack-target probe | the net reads the target (98% consistent sign) but prices it at about 2% of the creature |
| f84 control, 26 generations | flat: 0.775–0.794 vs sb-tactical, 0.61 vs g7, 0.50 vs prev |
| knob rungs | all flat or worse, all stopped |
| behaviour on g14/g26 | instants held through ~40% of opponent turns (p4c 38%); lethal burn taken 89–92% when offered, lethal attack 100% |
| forced kills (`:killfirst`, 2,000 games) | 0.391 ±0.021; own creatures 0.392 ±0.021; opponent's 0.449 ±0.022: taking more creature kills is not better |
| `instwin=noupkeep`, 5 generations from g26 | strength flat (sb-tactical 0.78–0.80, g7 0.61–0.62); own-upkeep casts 36 → under 1 per 100 games, moved to own draw step and main 1; opponent-turn share 25.5% → 23.4% |
| `instwin=noupdraw`, from g4 | g7 0.613 → 0.586; sb-tactical 0.78–0.795; opponent-turn share 26.6% → 21.5% |
| rule at play, head to head (g7, 2,000 games, sims 100) | full-combat+resp 0.5035 ±0.0219; inst8-combat+resp 0.4980 ±0.0219 |

Source: the run notes' table of what was tried, the forced-kill figures from the run's output and the head-to-head from its results. All kept privately.

## Data

The line's generations, census tables and archives, the per-class census runs and the forced-kill counterfactuals are kept privately.
