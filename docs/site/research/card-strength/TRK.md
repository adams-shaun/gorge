# TRK (Star Trek): simulated card strength

A cold-start card list for TRK, made from simulated games alone, before any human play data existed for the set. It is a prior, not a measurement of how the cards play for people. Every card is in [TRK.csv](TRK.csv) (list id `trk-nonet-11k`).

<ul class="gg-facts">
  <li><strong>11,000</strong> games, 10,970 decided, 0 errors</li>
  <li><strong>291</strong> cards ranked of 292 eligible</li>
  <li><strong>0.758</strong> split-half reliability (Spearman, ridge value)</li>
  <li><strong>0.403</strong> the recipe's agreement with 17Lands on FDN, at 3,000 games</li>
</ul>

## How the list was made

- **Tool:** a valuation tool that is not in this repository, from a local build with uncommitted changes; the gorge commit was not recorded.
- **Setup:** az search at 25 simulations, 11,000 games, seed 21.
- **Decks:** random two-colour decks of 23 distinct nonland cards (15 creatures) drawn uniformly from each colour pair's eligible cards, plus basics. 292 TRK cards were eligible.
- **Pilot:** the az search with no network, 25 simulations per searched decision, on both seats. It knows no card, so the list carries no knowledge from another set.
- **Games:** 11,000 played, 0 errors. 10,970 were decided; 22 ended without a winner and 8 stalled, and those 30 are left out of the estimate.
- **Estimator:** in-hand membership. Each seat's in-hand set is its opening hand after mulligans plus every card it later drew (the set 17Lands counts for GIH WR). A ridge linear-probability model (lambda 50) of the seat's win on in-hand membership, fitted over the 21,940 decided seat-games; the value is the coefficient plus 0.5. The shrunk in-hand win rate, (wins + 10) / (n + 20), is also stored, as `gih` in the CSV.
- **Published prior:** the ridge ranks are mapped onto the distribution of FDN's 17Lands `gih_wr` (cards with at least 2,000 games) by quantile matching, giving the "Prior (gih_wr scale)" column below. The ranks are the ridge ranks: Spearman between the prior and the ridge value is 0.999998 (rounded to 6 decimals; the stored prior is rounded to 4 decimals, which makes ties). This prior is TRK's draft-bot rating file, `draft/limited/pickbot/ratings/TRK.json` (generated from this CSV by `draft/limited/pickbot/internal/gentrk`).
- **Date:** 2026-11-03.

A second 11,000-game batch (seed 22) was planned to follow. No output or log of it exists, so this list is the first batch alone.

## Validity

There is no 17Lands data for TRK, so the list cannot be scored directly. What can be said:

- **Split-half reliability.** Games 1 to 5,500 and 5,501 to 10,990 were fitted separately (the last 10 are in neither). Spearman between the halves over 291 cards is 0.758 for the ridge value and 0.745 for the shrunk in-hand rate. Adjacent ranks are not reliable; the intervals in the table show how far apart two cards must be.
- **The same binary and recipe on FDN**, where 17Lands data exists (3,000 games, seed 11): Spearman against 17Lands GIH WR over 262 cards is 0.403 for the ridge value and 0.378 for the in-hand rate; its split-half is 0.360. That run has about a quarter of this one's games and is noisy, so it probably understates what the recipe reaches at this size; it does not measure it. See [FDN](FDN.md), list `fdn-nonet-3k`.
- **An earlier 20,000-game no-network FDN run** scored 0.50 by in-hand win rate, the same ceiling as the scripted bot. Its data no longer exists, so that figure cannot be re-checked here.

Figures are rounded to 3 decimals.

## Known biases

The same recipe on FDN, compared card by card with 17Lands (mean of simulated percentile minus 17Lands percentile, 0 to 100 scale, 262 cards; [FDN](FDN.md#known-biases) has the method):

| Group | Cards | Mean percentile difference |
|---|---:|---:|
| Text mentions sacrifice | 25 | -15.7 |
| Mana value 1-2 | 90 | -5.8 |
| Mana value 6+ | 30 | +0.0 |
| Enchantments (Enchantment in the type line) | 18 | +10.3 |
| Instants | 36 | -14.7 |
| Artifacts (not creature or enchantment) | 12 | -17.6 |

On this list, 16 of the 291 measured cards are colourless; none is in the top 40, 6 are in the bottom 20, and their median rank is 246.5. Colourless cards are eligible for every deck, so they have 1,394 to 2,102 games in hand (median 1,456.5) where coloured cards have a median of 778. Read together with the artifact figure above, the bottom of the list probably overstates how weak the colourless artifacts are.

Group means are from the `fdn-nonet-3k` list in `FDN.csv`, rounded to 1 decimal; the colourless counts from the `colour`, `rank` and `games_in_hand` columns of `TRK.csv`.

## The list

Value is the ridge value at 4 decimals; the interval is value ± 1.96 standard errors from the ridge fit. Games in hand is the number of decided seat-games in which the card was in hand. Prior is the stored published prior. Rarity is from gorge's [`draft/limited/setdata/sets/TRK.json`](https://github.com/adams-shaun/gorge/blob/main/draft/limited/setdata/sets/TRK.json) at gorge b14b8a543; colour is from the card's colour bits (W, U, B, R, G; C for colourless). Every eligible card has a rarity there.

One eligible card, All Good Things..., was never in a hand in any decided game; it has no value and no rank, and the published prior gives it 0.5499 only because the ridge leaves it at 0.5.

Top 40 of 291:

| Rank | Card | Rarity | Colour | Value | 95% interval | Games in hand | Prior (gih_wr scale) |
|---:|---|---|---|---:|---|---:|---:|
| 1 | Sheoldred, the Apocalypse | M | B | 0.6995 | 0.6681 to 0.7310 | 849 | 0.6412 |
| 2 | Georgiou, Terran Emperor | M | B | 0.6586 | 0.6260 to 0.6912 | 804 | 0.6289 |
| 3 | Captain Jean-Luc Picard | M | W | 0.6502 | 0.6185 to 0.6819 | 833 | 0.6265 |
| 4 | Excalbian Gamemaster | M | W | 0.6442 | 0.6127 to 0.6757 | 848 | 0.6226 |
| 5 | Mugato | U | G | 0.6150 | 0.5818 to 0.6481 | 751 | 0.621 |
| 6 | Jadzia Dax, Living Legacy | M | G | 0.6137 | 0.5820 to 0.6455 | 834 | 0.6206 |
| 7 | Khan, Despot by Design | U | R | 0.6053 | 0.5735 to 0.6370 | 835 | 0.6202 |
| 8 | Interstellar Effigy | C | U | 0.5915 | 0.5600 to 0.6230 | 857 | 0.6193 |
| 9 | V'Ger, the Intruder | U | U | 0.5901 | 0.5591 to 0.6212 | 889 | 0.6183 |
| 10 | Spock, Always Your Friend | U | UR | 0.5890 | 0.5378 to 0.6403 | 237 | 0.6181 |
| 11 | The Caretaker | R | U | 0.5888 | 0.5578 to 0.6197 | 900 | 0.6171 |
| 12 | Vulcan Mind-Meld | U | U | 0.5876 | 0.5515 to 0.6237 | 608 | 0.6161 |
| 13 | Captain James T. Kirk | M | R | 0.5807 | 0.5498 to 0.6116 | 891 | 0.6154 |
| 14 | Holo-Engineer | R | G | 0.5778 | 0.5452 to 0.6104 | 778 | 0.6141 |
| 15 | Bristly Bill, Spine Sower | M | G | 0.5763 | 0.5446 to 0.6080 | 834 | 0.6122 |
| 16 | Gwyndala, the Progeny | R | U | 0.5753 | 0.5451 to 0.6055 | 941 | 0.6099 |
| 17 | The Wrath of Khan | R | R | 0.5742 | 0.5364 to 0.6121 | 540 | 0.6064 |
| 18 | Captain Kathryn Janeway | M | G | 0.5724 | 0.5398 to 0.6051 | 786 | 0.6045 |
| 19 | Shuttle Ace | C | W | 0.5723 | 0.5408 to 0.6038 | 854 | 0.6043 |
| 20 | Munitions Enthusiast | U | R | 0.5717 | 0.5408 to 0.6027 | 884 | 0.6041 |
| 21 | Deanna Troi, Psychic Counsel | R | W | 0.5715 | 0.5398 to 0.6033 | 836 | 0.6035 |
| 22 | Captain Jonathan Archer | R | G | 0.5702 | 0.5378 to 0.6026 | 792 | 0.6012 |
| 23 | Captain Michael Burnham | U | R | 0.5673 | 0.5356 to 0.5990 | 836 | 0.5954 |
| 24 | Gul Dukat, Colonial Prefect | M | B | 0.5661 | 0.5337 to 0.5985 | 790 | 0.5935 |
| 25 | Silicate Surveyor | C | R | 0.5655 | 0.5338 to 0.5972 | 837 | 0.5911 |
| 26 | Syndicate Liquidators | U | G | 0.5653 | 0.5328 to 0.5977 | 783 | 0.5911 |
| 27 | Primary Unimatrix Adjunct | R | W | 0.5647 | 0.5335 to 0.5960 | 866 | 0.591 |
| 28 | Targ Wrangler | U | R | 0.5576 | 0.5265 to 0.5887 | 874 | 0.5909 |
| 29 | Will Riker, First in the Field | R | R | 0.5563 | 0.5242 to 0.5884 | 813 | 0.5904 |
| 30 | Saurian Explorer | C | G | 0.5563 | 0.5246 to 0.5879 | 836 | 0.59 |
| 31 | Cha'DIch Investigator | C | B | 0.5561 | 0.5238 to 0.5884 | 802 | 0.5892 |
| 32 | The Measure of a Man | U | W | 0.5552 | 0.5184 to 0.5921 | 580 | 0.5879 |
| 33 | Photosynthetic Physician | C | G | 0.5523 | 0.5204 to 0.5842 | 824 | 0.5868 |
| 34 | Ghost of Illyria | U | W | 0.5514 | 0.5193 to 0.5835 | 813 | 0.5862 |
| 35 | Una Chin-Riley, Number One | U | W | 0.5511 | 0.5189 to 0.5833 | 805 | 0.5861 |
| 36 | Fugitive Listener | C | B | 0.5511 | 0.5187 to 0.5834 | 800 | 0.5857 |
| 37 | Dominion Assault Force | C | G | 0.5509 | 0.5176 to 0.5843 | 741 | 0.5854 |
| 38 | Khan, Vengeful Heart | M | B | 0.5509 | 0.5190 to 0.5828 | 828 | 0.5848 |
| 39 | Antagonist Incorporator | R | W | 0.5507 | 0.5198 to 0.5817 | 896 | 0.5847 |
| 40 | Dahar Master | C | R | 0.5505 | 0.5187 to 0.5824 | 829 | 0.584 |

Bottom 20:

| Rank | Card | Rarity | Colour | Value | 95% interval | Games in hand | Prior (gih_wr scale) |
|---:|---|---|---|---:|---|---:|---:|
| 272 | Saavik, Stoic Student | U | W | 0.4306 | 0.3996 to 0.4617 | 886 | 0.4872 |
| 273 | Stable Wormhole | R | G | 0.4288 | 0.3921 to 0.4656 | 580 | 0.487 |
| 274 | A Good Day to Die | C | B | 0.4288 | 0.3925 to 0.4651 | 599 | 0.4867 |
| 275 | Romulan Bird-of-Prey | R | U | 0.4264 | 0.3905 to 0.4622 | 620 | 0.4864 |
| 276 | Kirk, Commanding Officer | U | WR | 0.4258 | 0.3721 to 0.4796 | 199 | 0.486 |
| 277 | Flare of Fortitude | M | W | 0.4257 | 0.3877 to 0.4637 | 533 | 0.4851 |
| 278 | Savvy Profiteer | C | R | 0.4256 | 0.3946 to 0.4565 | 889 | 0.4841 |
| 279 | Peace in Our Time | R | W | 0.4254 | 0.3880 to 0.4629 | 561 | 0.483 |
| 280 | U.S.S. Enterprise-D, Galaxy-Class | R | C | 0.4253 | 0.4010 to 0.4497 | 1478 | 0.4819 |
| 281 | Perils of the Past | C | R | 0.4238 | 0.3851 to 0.4624 | 507 | 0.481 |
| 282 | Ensign Pavel Chekov | U | U | 0.4222 | 0.3903 to 0.4541 | 828 | 0.4784 |
| 283 | Final Frontier | C | G | 0.4188 | 0.3832 to 0.4544 | 626 | 0.4759 |
| 284 | The Holodeck | R | C | 0.4185 | 0.3937 to 0.4433 | 1418 | 0.4709 |
| 285 | D'Vana Tendi, Optimistic Ensign | U | UG | 0.4160 | 0.3629 to 0.4691 | 209 | 0.4681 |
| 286 | Engineered Explosives | R | C | 0.4140 | 0.3896 to 0.4385 | 1460 | 0.4677 |
| 287 | Combadge | U | C | 0.4071 | 0.3824 to 0.4318 | 1426 | 0.4648 |
| 288 | Anthropological Mission | U | U | 0.4049 | 0.3679 to 0.4418 | 576 | 0.457 |
| 289 | Isolinear Processor | U | C | 0.4009 | 0.3764 to 0.4254 | 1492 | 0.4479 |
| 290 | Kobayashi Maru Scenario | M | U | 0.3752 | 0.3388 to 0.4116 | 598 | 0.4463 |
| 291 | Replicator | R | C | 0.3619 | 0.3373 to 0.3864 | 1453 | 0.4384 |
