# Card strength lists

Per-set card values estimated from simulated games alone. Each page says how its list was made, how far it can be trusted, and its known biases; the CSV beside it holds every card.

<div class="grid cards" markdown>

-   :material-star-four-points: [**TRK, Star Trek**](TRK.md)

    A cold-start list from 11,000 games with a search pilot that knows no card. 291 cards ranked. [TRK.csv](TRK.csv)

-   :material-cards: [**FDN, Foundations**](FDN.md)

    Three lists, each scored against 17Lands human play. 265 cards each. [FDN.csv](FDN.csv)

</div>

| Set | List id | Pilot | Estimator | Games played | Spearman vs 17Lands GIH WR | Date | Built from |
|---|---|---|---|---:|---:|---|---|
| TRK | `trk-nonet-11k` | az search, no network, 25 sims | ridge on in-hand membership | 11,000 | none (no 17Lands data) | 2026-11-03 | a local build with uncommitted changes |
| FDN | `fdn-bot-80k` | scripted bot | shrunk in-hand win rate | 80,000 | 0.518 | 2026-10-16 | gorge 10f10c858 (inferred) |
| FDN | `fdn-azc6g3-20k` | az search, az-c6 g3 network, 25 sims | shrunk in-hand win rate | 20,000 | 0.609 | 2026-10-16 | gorge 10f10c858 (inferred) |
| FDN | `fdn-nonet-3k` | az search, no network, 25 sims | ridge on in-hand membership | 3,000 | 0.403 | 2026-11-03 | the same build as TRK |

Agreement with human play, Spearman ρ against 17Lands GIH WR on FDN:

<div class="gg-bars" style="--gg-bar-label: 8rem">
<div class="gg-bar" role="img" title="fdn-azc6g3-20k: 0.609" aria-label="fdn-azc6g3-20k: 0.609"><span class="gg-bar__label" aria-hidden="true">fdn-azc6g3-20k</span><span class="gg-seg gg-seg--ok" style="--w:60.9%"></span></div>
<div class="gg-bar" role="img" title="fdn-bot-80k: 0.518" aria-label="fdn-bot-80k: 0.518"><span class="gg-bar__label" aria-hidden="true">fdn-bot-80k</span><span class="gg-seg gg-seg--ok" style="--w:51.8%"></span></div>
<div class="gg-bar" role="img" title="fdn-nonet-3k: 0.403" aria-label="fdn-nonet-3k: 0.403"><span class="gg-bar__label" aria-hidden="true">fdn-nonet-3k</span><span class="gg-seg gg-seg--ok" style="--w:40.3%"></span></div>
<div class="gg-bar" role="img" title="rarity alone: 0.217" aria-label="rarity alone: 0.217"><span class="gg-bar__label" aria-hidden="true">rarity alone</span><span class="gg-seg gg-seg--unv" style="--w:21.7%"></span></div>
</div>
<ul class="gg-legend gg-bars__key"><li><i class="gg-seg gg-seg--ok"></i>simulated list</li><li><i class="gg-seg gg-seg--unv"></i>rarity alone (C &lt; U &lt; R &lt; M)</li><li>bar length is ρ on a 0–1 scale</li></ul>

Spearman figures are over the 262 FDN cards with at least 500 17Lands games-in-hand, rounded to 3 decimals; rarity alone scores 0.217. The `fdn-azc6g3-20k` network was trained on FDN, so that list is not a cold-start result. `fdn-nonet-3k` is the FDN calibration of the TRK recipe.

## How the lists were made

A valuation tool built on the draftlab composition API (not in this repository) builds random two-colour decks, plays them with one pilot on both seats, and writes one line per game with each seat's in-hand cards. The values for every card are then fitted at once, and the CSVs and every derived figure on these pages (the Spearman figures, the split-half reliabilities, the 95% intervals and the bias tables) come from the game files. Each page states its method in words.

Runs described in the experiment's write-up whose data no longer exists (the 2026-10-14 FDN runs) have no list here; the FDN page quotes their figures and [Cold-start card valuation](../card-valuation.md) has the full results.

!!! info "17Lands data"
    `FDN.csv` and the FDN page carry 17Lands GIH WR beside the simulated values, from gorge's [`draft/limited/pickbot/ratings/FDN.json`](https://github.com/adams-shaun/gorge/blob/main/draft/limited/pickbot/ratings/FDN.json): data from 17Lands (<https://www.17lands.com>), public datasets, CC BY 4.0.
