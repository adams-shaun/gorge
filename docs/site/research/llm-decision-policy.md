# An LLM decision policy

*QLoRA SFT, DPO and a win-rate gate, 2026-10-08 → 10-10*

Can a fine-tuned language model, trained on search-teacher labels, pick better than the heuristic bot on the decisions where search disagrees with it? And does blending its preference into the search win games?

!!! abstract "Verdict"
    **Not adopted.** Per-decision gains were small, and the blend's win-rate gate is null: +0.48 points against a pre-registered +3 point bar. Nothing landed on main.

<ul class="gg-facts">
  <li><strong>+0.0019</strong> value per decision for DPO over the bot, ±0.0021</li>
  <li><strong>50.48%</strong> for search + LLM blend against plain search, over 7,100 games</li>
  <li><strong>87%</strong> of held-out decisions where SFT matched the bot</li>
</ul>

## Setup

- Base model Qwen/Qwen3.5-9B, LoRA rank 16 on all linear layers, answer-only loss, on two RTX PRO 6000 (96 GB).
- Labels from gorge's search teacher (label-v1 records). An unmerged Go branch added world rescoring, a deck catalog and holdout flags to it.
- Data: 11,192 decisions from 3,000 games over 10 mono pairs. Training: two GPUs, batch 8, no 4-bit quantisation, 31 minutes per epoch.
- Preference pairs for DPO: value = (8×Q8 + 64×Q64)/72; candidates ≥ 0.06 below the best are rejected (at most 2 per decision); 8,634 pairs.
- Fresh-seed test set t1: seed 60,000,000, 2,370 decisions. SpellBench split: a catalog over 261 pairings; held-out decks fdn07-ur, fdn12-ugw, faeries, wildfire, uw-control, the-epic-storm.

## Results

Units are value per decision (the 64-world Q of the picked candidate minus the bot's), with paired 95% intervals. The write-up warns that these intervals are too narrow, because decisions within one game are correlated.

**How much is there to gain.** On c1 (11,129 rescored decisions) the 8-world search's overrides of the bot gain +0.042 ±0.005 where they occur (2,501 decisions). Over all decisions the 8-world search gains +0.0094 ±0.0011, and a 64-world oracle ceiling is +0.0283 ±0.0012.

**SFT** on 539 held-out decisions matched the bot 87% of the time and the search pick 71.2% (always-bot: 77.7%).

=== "Fresh seeds, t1"

    2,370 decisions:

    | Kind | n | search − bot | DPO − bot |
    |---|---|---|---|
    | casting | 1,271 | +0.0104 ±0.0038 | +0.0065 ±0.0027 |
    | attackers | 521 | +0.0034 ±0.0029 | −0.0046 ±0.0044 |
    | blockers | 160 | −0.0004 ±0.0071 | −0.0132 ±0.0113 |
    | targets | 418 | +0.0053 ±0.0053 | +0.0016 ±0.0044 |
    | all | 2,370 | +0.0072 ±0.0024 | +0.0019 ±0.0021 |

    DPO − SFT: +0.0036 ±0.0016. Blending, Q8 + λ(log π_cand − log π_bot): λ 0.02 (fixed in advance) +0.0006 ±0.0017; λ 0.01 (chosen after) +0.0016 ±0.0015; λ 0.05 −0.0016 ±0.0020.

=== "SpellBench retrain"

    34,625 training decisions; sbt = 1,242 rescored held-out-deck decisions:

    | Policy | sbt all | sbt casting | t1 all | t1 casting |
    |---|---|---|---|---|
    | SFT | +0.0001 ±0.0016 | +0.0008 ±0.0020 | −0.0010 ±0.0013 | +0.0016 ±0.0012 |
    | DPO | +0.0018 ±0.0020 | +0.0029 ±0.0027 | +0.0045 ±0.0022 | +0.0094 ±0.0031 |
    | search | +0.0007 ±0.0028 | — | +0.0072 ±0.0024 | — |
    | blend 0.02 − search | +0.0045 ±0.0024 | +0.0064 ±0.0036 | +0.0020 ±0.0017 | +0.0037 ±0.0027 |

**Win-rate gate.** Pre-registered: λ 0.02, 7,100 games on the 71 held-out pairings, base seed 90,000,000, bar +3 points. Search + LLM blend against plain search: **3584-3516 of 7100 = 50.48%, Wilson 95% [49.32, 51.64]**, +0.48 points; seed-clustered 50.48% ±0.34 over 3,550 seeds. The blend changed the search's pick on 1,426 decisions. 926 games had a seat whose observation the bridge did not support.

The gate's result survives only in the run's output in a working log; the write-up and the pre-registration report predate it.

## Data

The working directory and the bridge directory are gone. Surviving: the write-up (kept privately), two unmerged gorge branches (the rescoring flags and the bridge), and the gate's output.
