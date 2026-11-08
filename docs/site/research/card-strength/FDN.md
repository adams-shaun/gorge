# FDN (Foundations): simulated card strength

Three card lists for FDN, each made from simulated games alone. FDN is the calibration set: 17Lands publishes human games-in-hand win rates (GIH WR) for it, so every list here can be scored against human play. The 17Lands figures are used only to score the lists, never to make them. Every card of every list is in [FDN.csv](FDN.csv) (column `list`).

| List id | Pilot | Games played (decided) | Estimator ranked by | Spearman vs 17Lands GIH WR | Date | Built from |
|---|---|---:|---|---:|---|---|
| `fdn-bot-80k` | scripted bot | 80,000 (79,994) | shrunk in-hand win rate | 0.518 | 2026-10-16 | gorge 10f10c858 (inferred) |
| `fdn-azc6g3-20k` | az search with the az-c6 generation 3 network | 20,000 (19,993) | shrunk in-hand win rate | 0.609 | 2026-10-16 | gorge 10f10c858 (inferred) |
| `fdn-nonet-3k` | az search, no network | 3,000 (3,000) | ridge on in-hand membership | 0.403 | 2026-11-03 | a local build with uncommitted changes |

Rarity alone (C < U < R < M) scores 0.217 on the same 262 cards. Spearman is rounded to 3 decimals.

## How the lists were made

All three come from one valuation tool, which is not in this repository. It builds random two-colour decks of 23 distinct nonland cards (15 creatures) drawn uniformly from each colour pair's eligible cards, plus basics, plays them with one pilot on both seats, and writes one line per game with each seat's in-hand cards (opening hand after mulligans plus every card drawn later, the set 17Lands counts for GIH WR). 265 FDN cards were eligible. An offline script then fits every card at once:

- `gih`: the shrunk in-hand win rate, (wins + 10) / (n + 20), where n is the number of decided seat-games with the card in hand.
- `ridge`: a ridge linear-probability model (lambda 50) of the seat's win on in-hand membership, coefficient plus 0.5.

Games without a winner and stalled games are left out. Both estimates for every list are in [FDN.csv](FDN.csv); the "Value" column there is the one each list is ranked by.

=== "`fdn-bot-80k`"

    **Scripted bot pilot.**

    - Setup: random mode, 80,000 games, seed 7. The pilot is gorge's scripted bot policy (recorded as `bot@1`).
    - Games: 80,000 played, 0 errors; 159,988 decided seat-games, so 79,994 decided games.
    - Ranked by `gih`. This `gih` column was the teacher for the stage-0 drafter ([draft RL](../draft-rl.md)).
    - Date: 2026-10-16.
    - Commit: no build stamp survives; the run is attributed to gorge 10f10c858, and the tool was unchanged from gorge main 5799819d6. Uncommitted changes cannot be ruled out.

=== "`fdn-azc6g3-20k`"

    **az search with a trained network.**

    - Setup: two chunks of 10,000 games with seeds 11 and 12, az search at 25 simulations with the az-c6 generation 3 network; the two game files were concatenated.
    - Pilot: az search, 25 simulations per searched decision, on both seats, guided by the az-c6 generation 3 network. That network was trained by self-play on 17Lands FDN decks ([AZ loop](../az-loop.md#4-az-c6-from-scratch-g1g7)), so it has seen these cards. This list is therefore not a cold-start measurement.
    - Games: 2 × 10,000 played, 0 errors; 39,986 decided seat-games, so 19,993 decided games. In the first chunk, 125 of 17,296,675 simulations found no world and 5 decisions were refused; the second had none.
    - Ranked by `gih`. It was the teacher for a second stage-0 drafter.
    - Date: 2026-10-16.
    - Commit: as for `fdn-bot-80k`: gorge 10f10c858.

=== "`fdn-nonet-3k`"

    **az search with no network.**

    - Setup: az search at 25 simulations, 3,000 games, seed 11, the same setup as the TRK run.
    - Pilot: az search with no network, both seats. It knows no card, which makes this the calibration of the recipe used for [TRK](TRK.md).
    - Games: 3,000 played, 0 errors, all decided (6,000 seat-games).
    - Ranked by `ridge`, as the TRK list is. A refit of the ridge from the game file matches the run's own values to 1.2e-16.
    - Date: 2026-11-03.
    - Commit: a local build with uncommitted changes; the gorge commit was not recorded.

## Validity

Spearman rank correlation with 17Lands GIH WR (from the public Premier Draft dataset, 791,159 games) over the 262 cards that have at least 500 17Lands games-in-hand and were in a simulated hand:

| List | `gih` | `ridge` | Split-half |
|---|---:|---:|---:|
| `fdn-bot-80k` | 0.518 | 0.483 | not available (game file deleted) |
| `fdn-azc6g3-20k` | 0.609 | 0.593 | not available (game file deleted) |
| `fdn-nonet-3k` | 0.378 | 0.403 | 0.360 (ridge, first vs last 1,500 games) |
| rarity alone | 0.217 | | |

Earlier runs, recorded in the experiment's write-up and no longer on disk:

- Scripted bot pilot, 2026-10-14: agreement with 17Lands stops rising at about 300,000 games, at 0.50; at 188,244 games the in-hand win rate scores 0.49 and the deck regression 0.50; split-half reliability is 0.96 for the regression and 0.97 for the in-hand win rate. A second scripted pilot (lethal-pressure, 500,000 games) scores 0.51. Rarity alone scores 0.22, which these lists reproduce as 0.217.
- az-c4 network pilot, 25 simulations, 20,000 games: 0.57 by in-hand win rate.
- No-network az pilot, 20,000 games: 0.50 by in-hand win rate.

A maintainer's note gives the bot split-half as 0.957; the write-up rounds it to 0.96. Neither primary file survives.

The 10-16 bot list (0.518 at 80,000 games) scores a little above the 10-14 bot figures (0.48 to 0.50 between 100,000 and 500,000 games). The two runs used different builds: the search, payment-plan and manual-tap fixes the write-up lists reached gorge main (e10db2ef3, an ancestor of 5799819d6) between them. This page does not test which change moved it.

ρ is computed as the runs' own analysis does: average ranks for ties, Pearson on ranks. The 17Lands figures are gorge's [`draft/limited/pickbot/ratings/FDN.json`](https://github.com/adams-shaun/gorge/blob/main/draft/limited/pickbot/ratings/FDN.json) (17Lands data, CC BY 4.0, "Data from 17Lands (<https://www.17lands.com>), public datasets").

## Known biases

For each card, the percentile of its simulated value (the list's ranking estimator) minus the percentile of its 17Lands GIH WR, both on a 0 to 100 scale over the 262 scored cards. A negative group mean means the simulation rates the group lower than human play does. Groups come from each card's mana value, type line, and whether its text mentions sacrifice.

<figure markdown="span">
--8<-- "research/figs/card-bias.svg"
</figure>

| Group | Cards | `fdn-bot-80k` | `fdn-azc6g3-20k` | `fdn-nonet-3k` |
|---|---:|---:|---:|---:|
| Text mentions sacrifice | 25 | -20.3 | -12.2 | -15.7 |
| Mana value 1-2 | 90 | -8.9 | -4.8 | -5.8 |
| Mana value 3 | 70 | +2.3 | +4.6 | +3.1 |
| Mana value 4 | 44 | +4.6 | +4.3 | +6.3 |
| Mana value 5 | 28 | +5.0 | +1.3 | +1.1 |
| Mana value 6+ | 30 | +10.0 | -3.9 | +0.0 |
| Enchantments (Enchantment in the type line) | 18 | +9.1 | +4.7 | +10.3 |
| Creatures | 162 | +6.2 | +4.0 | +4.5 |
| Instants | 36 | -19.4 | -13.3 | -14.7 |
| Sorceries | 29 | -10.3 | -5.6 | -1.7 |
| Artifacts (not creature or enchantment) | 12 | -2.7 | -2.7 | -17.6 |

The write-up's residual chart for the 10-14 bot run (500,000 games, simulated minus 17Lands in-hand percentile) gives: sacrifice text (7 cards) -34.0; mana value 2 (60) -12.6; mana value 1 (30) -7.5; instants (36) -5.0; creatures (162) -3.2; mana value 3 (70) +3.8; mana value 4 (44) +5.1; sorceries (29) +8.8; mana value 6+ (30) +11.7; enchantments (18) +22.6. Its sacrifice group is 7 cards, a narrower definition than the 25 cards "mentioning sacrifice" here, and its data is gone, so the two sacrifice figures are not directly comparable.

The biases hold in direction on the 10-16 bot list: sacrifice cards and mana value 1-2 rate low; mana value 6+ and enchantments rate high. Two groups differ: instants rate much lower here (-19.4 against -5.0) and sorceries change sign (-10.3 against +8.8). With a trained network (`fdn-azc6g3-20k`) the sacrifice, cheap-card, enchantment and instant biases shrink and the 6+ bias changes sign. The write-up names the likely causes: a pilot that misuses sacrifice outlets and holds up instant-speed mana poorly, games that run longer than human games, and random 15-creature decks that misjudge board wipes and synergy cards.

Group means are rounded to 1 decimal.

## The lists

Value is the list's ranking estimator at 4 decimals. The interval is value ± 1.96 standard errors: for `gih`, sqrt(gih (1 - gih) / (n + 20)); for `ridge`, from the ridge fit. Games in hand is n. Rarity is from gorge's [`draft/limited/setdata/sets/FDN.json`](https://github.com/adams-shaun/gorge/blob/main/draft/limited/setdata/sets/FDN.json) at gorge b14b8a543; colour is from the card's colour bits (W, U, B, R, G; C for colourless). 17Lands GIH WR is the stored value in gorge's `pickbot/ratings/FDN.json`; where it rests on fewer than 500 games, the count is shown and the card is left out of every Spearman figure above.

### `fdn-bot-80k`

Top 30 of 265:

| Rank | Card | Rarity | Colour | Value | 95% interval | Games in hand | 17Lands GIH WR |
|---:|---|---|---|---:|---|---:|---:|
| 1 | Omniscience | M | U | 0.7242 | 0.7112 to 0.7371 | 4537 | 0.3704 (27 games) |
| 2 | Elenda, Saint of Dusk | R | WB | 0.6483 | 0.6245 to 0.6721 | 1524 | 0.6154 |
| 3 | Sire of Seven Deaths | M | C | 0.6365 | 0.6290 to 0.6441 | 15516 | 0.6203 |
| 4 | Bloodthirsty Conqueror | M | B | 0.6224 | 0.6106 to 0.6342 | 6453 | 0.6272 |
| 5 | Spinner of Souls | R | G | 0.6197 | 0.6074 to 0.6319 | 5985 | 0.6211 |
| 6 | Quilled Greatwurm | M | G | 0.6150 | 0.6026 to 0.6275 | 5874 | 0.55 |
| 7 | Zimone, Paradox Sculptor | M | UG | 0.6099 | 0.5860 to 0.6337 | 1582 | 0.6263 |
| 8 | Twinflame Tyrant | M | R | 0.6018 | 0.5898 to 0.6139 | 6289 | 0.6048 |
| 9 | Empyrean Eagle | U | WU | 0.5911 | 0.5676 to 0.6146 | 1665 | 0.591 |
| 10 | Vivien Reid | M | G | 0.5897 | 0.5755 to 0.6038 | 4608 | 0.6139 |
| 11 | Koma, World-Eater | R | UG | 0.5876 | 0.5636 to 0.6116 | 1595 | 0.5693 |
| 12 | Serra Angel | U | W | 0.5829 | 0.5706 to 0.5952 | 6175 | 0.5477 |
| 13 | Perforating Artist | U | BR | 0.5818 | 0.5572 to 0.6064 | 1527 | 0.5748 |
| 14 | Felling Blow | U | G | 0.5812 | 0.5671 to 0.5953 | 4660 | 0.5871 |
| 15 | Felidar Savior | C | W | 0.5809 | 0.5688 to 0.5930 | 6355 | 0.5736 |
| 16 | Vampire Nighthawk | U | B | 0.5795 | 0.5676 to 0.5915 | 6556 | 0.5823 |
| 17 | Ashroot Animist | R | RG | 0.5795 | 0.5539 to 0.6050 | 1414 | 0.5847 |
| 18 | Herald of Eternal Dawn | M | W | 0.5778 | 0.5656 to 0.5901 | 6262 | 0.5773 |
| 19 | Treetop Snarespinner | C | G | 0.5778 | 0.5653 to 0.5904 | 5942 | 0.5462 |
| 20 | Ghalta, Primal Hunger | R | G | 0.5744 | 0.5618 to 0.5870 | 5915 | 0.5523 |
| 21 | Authority of the Consuls | R | W | 0.5743 | 0.5603 to 0.5883 | 4791 | 0.5203 |
| 22 | Angel of Finality | U | W | 0.5742 | 0.5619 to 0.5865 | 6208 | 0.5668 |
| 23 | Exemplar of Light | R | W | 0.5739 | 0.5617 to 0.5862 | 6216 | 0.5899 |
| 24 | Beast-Kin Ranger | C | G | 0.5739 | 0.5615 to 0.5863 | 6103 | 0.5523 |
| 25 | Arahbo, the First Fang | R | W | 0.5728 | 0.5607 to 0.5849 | 6410 | 0.6092 |
| 26 | Dazzling Angel | C | W | 0.5726 | 0.5605 to 0.5847 | 6433 | 0.5778 |
| 27 | Cackling Prowler | C | G | 0.5721 | 0.5595 to 0.5847 | 5906 | 0.5411 |
| 28 | Scrawling Crawler | R | C | 0.5686 | 0.5609 to 0.5763 | 15931 | 0.5755 |
| 29 | Guarded Heir | U | W | 0.5684 | 0.5561 to 0.5806 | 6247 | 0.5676 |
| 30 | Needletooth Pack | U | G | 0.5674 | 0.5549 to 0.5799 | 6032 | 0.5342 |

Bottom 15:

| Rank | Card | Rarity | Colour | Value | 95% interval | Games in hand | 17Lands GIH WR |
|---:|---|---|---|---:|---|---:|---:|
| 251 | Goblin Negotiation | U | R | 0.4306 | 0.4157 to 0.4454 | 4235 | 0.5195 |
| 252 | Hidetsugu's Second Rite | U | R | 0.4276 | 0.4130 to 0.4422 | 4400 | 0.5624 |
| 253 | Seismic Rupture | U | R | 0.4267 | 0.4119 to 0.4414 | 4304 | 0.5013 |
| 254 | Fanatical Firebrand | C | R | 0.4214 | 0.4092 to 0.4336 | 6262 | 0.53 |
| 255 | Time Stop | R | U | 0.4200 | 0.4055 to 0.4345 | 4442 | 0.4751 |
| 256 | Day of Judgment | R | W | 0.4143 | 0.4002 to 0.4284 | 4694 | 0.5911 |
| 257 | Fleeting Flight | C | W | 0.4140 | 0.4001 to 0.4279 | 4806 | 0.5644 |
| 258 | Axgard Cavalry | C | R | 0.4077 | 0.3956 to 0.4198 | 6328 | 0.5214 |
| 259 | Twinblade Blessing | U | W | 0.4054 | 0.3915 to 0.4194 | 4733 | 0.4683 |
| 260 | Ravenous Amulet | U | C | 0.4012 | 0.3922 to 0.4101 | 11481 | 0.5171 |
| 261 | Thousand-Year Storm | R | UR | 0.3996 | 0.3698 to 0.4294 | 1016 | 0.3479 |
| 262 | Sure Strike | C | R | 0.3992 | 0.3848 to 0.4137 | 4411 | 0.5241 |
| 263 | Bulk Up | U | R | 0.3935 | 0.3790 to 0.4081 | 4300 | 0.483 |
| 264 | Blanchwood Armor | U | G | 0.3749 | 0.3608 to 0.3889 | 4544 | 0.5202 |
| 265 | Hungry Ghoul | C | B | 0.3538 | 0.3424 to 0.3653 | 6675 | 0.5562 |

### `fdn-azc6g3-20k`

Top 30 of 265:

| Rank | Card | Rarity | Colour | Value | 95% interval | Games in hand | 17Lands GIH WR |
|---:|---|---|---|---:|---|---:|---:|
| 1 | Omniscience | M | U | 0.7952 | 0.7708 to 0.8196 | 1030 | 0.3704 (27 games) |
| 2 | Elenda, Saint of Dusk | R | WB | 0.6676 | 0.6194 to 0.7158 | 347 | 0.6154 |
| 3 | Bloodthirsty Conqueror | M | B | 0.6362 | 0.6117 to 0.6606 | 1467 | 0.6272 |
| 4 | Twinflame Tyrant | M | R | 0.6347 | 0.6099 to 0.6596 | 1420 | 0.6048 |
| 5 | Liliana, Dreadhorde General | M | B | 0.6268 | 0.5985 to 0.6550 | 1108 | 0.6412 |
| 6 | Ashroot Animist | R | RG | 0.6238 | 0.5707 to 0.6770 | 299 | 0.5847 |
| 7 | Vivien Reid | M | G | 0.6224 | 0.5931 to 0.6517 | 1034 | 0.6139 |
| 8 | Alesha, Who Laughs at Fate | R | BR | 0.6075 | 0.5579 to 0.6571 | 352 | 0.6183 |
| 9 | Dazzling Angel | C | W | 0.6036 | 0.5785 to 0.6286 | 1443 | 0.5778 |
| 10 | Spinner of Souls | R | G | 0.6030 | 0.5775 to 0.6284 | 1403 | 0.6211 |
| 11 | Felidar Savior | C | W | 0.6013 | 0.5764 to 0.6262 | 1465 | 0.5736 |
| 12 | Authority of the Consuls | R | W | 0.6009 | 0.5724 to 0.6293 | 1120 | 0.5203 |
| 13 | Kaito, Cunning Infiltrator | M | U | 0.5955 | 0.5662 to 0.6248 | 1058 | 0.5827 |
| 14 | Ruby, Daring Tracker | U | RG | 0.5936 | 0.5415 to 0.6456 | 322 | 0.5699 |
| 15 | Day of Judgment | R | W | 0.5928 | 0.5634 to 0.6221 | 1058 | 0.5911 |
| 16 | Sire of Seven Deaths | M | C | 0.5918 | 0.5757 to 0.6079 | 3554 | 0.6203 |
| 17 | Arahbo, the First Fang | R | W | 0.5830 | 0.5581 to 0.6079 | 1486 | 0.6092 |
| 18 | Sun-Blessed Healer | U | W | 0.5813 | 0.5558 to 0.6067 | 1420 | 0.5848 |
| 19 | Serra Angel | U | W | 0.5800 | 0.5538 to 0.6061 | 1349 | 0.5477 |
| 20 | Kykar, Zephyr Awakener | R | WU | 0.5794 | 0.5296 to 0.6291 | 358 | 0.5744 |
| 21 | Quilled Greatwurm | M | G | 0.5753 | 0.5497 to 0.6010 | 1407 | 0.55 |
| 22 | Cat Collector | U | W | 0.5730 | 0.5475 to 0.5986 | 1418 | 0.5857 |
| 23 | Scrawling Crawler | R | C | 0.5728 | 0.5568 to 0.5888 | 3653 | 0.5755 |
| 24 | Vampire Nighthawk | U | B | 0.5722 | 0.5470 to 0.5974 | 1462 | 0.5823 |
| 25 | Beast-Kin Ranger | C | G | 0.5721 | 0.5469 to 0.5973 | 1464 | 0.5523 |
| 26 | Angel of Finality | U | W | 0.5697 | 0.5442 to 0.5951 | 1430 | 0.5668 |
| 27 | Eager Trufflesnout | U | G | 0.5687 | 0.5432 to 0.5942 | 1429 | 0.5647 |
| 28 | Dreadwing Scavenger | U | UB | 0.5654 | 0.5208 to 0.6100 | 454 | 0.6043 |
| 29 | Make Your Move | C | W | 0.5646 | 0.5351 to 0.5941 | 1064 | 0.5526 |
| 30 | Armasaur Guide | C | W | 0.5643 | 0.5384 to 0.5902 | 1387 | 0.5052 |

Bottom 15:

| Rank | Card | Rarity | Colour | Value | 95% interval | Games in hand | 17Lands GIH WR |
|---:|---|---|---|---:|---|---:|---:|
| 251 | Banner of Kinship | R | C | 0.4417 | 0.4228 to 0.4606 | 2629 | 0.5062 |
| 252 | Sphinx's Tutelage | M | U | 0.4415 | 0.4125 to 0.4705 | 1108 | 0.5039 |
| 253 | Midnight Snack | U | B | 0.4415 | 0.4124 to 0.4706 | 1099 | 0.4864 |
| 254 | Goblin Bushwhacker | M | R | 0.4393 | 0.4140 to 0.4645 | 1462 | 0.53 |
| 255 | Brass's Bounty | R | R | 0.4382 | 0.4071 to 0.4693 | 959 | 0.3714 (35 games) |
| 256 | Ravenous Amulet | U | C | 0.4357 | 0.4172 to 0.4542 | 2739 | 0.5171 |
| 257 | Exsanguinate | U | B | 0.4342 | 0.4054 to 0.4630 | 1120 | 0.4818 |
| 258 | Rite of the Dragoncaller | M | R | 0.4252 | 0.3943 to 0.4561 | 963 | 0.5163 |
| 259 | Bulk Up | U | R | 0.4222 | 0.3913 to 0.4531 | 963 | 0.483 |
| 260 | Swiftfoot Boots | U | C | 0.4175 | 0.3988 to 0.4362 | 2646 | 0.4384 |
| 261 | Wildwood Scourge | U | G | 0.4159 | 0.3908 to 0.4409 | 1466 | 0.5121 |
| 262 | Thousand-Year Storm | R | UR | 0.4148 | 0.3560 to 0.4736 | 250 | 0.3479 |
| 263 | Inspiring Call | U | G | 0.4089 | 0.3799 to 0.4379 | 1083 | 0.4606 |
| 264 | Rune-Sealed Wall | U | U | 0.4054 | 0.3811 to 0.4296 | 1549 | 0.5177 |
| 265 | Goblin Negotiation | U | R | 0.3910 | 0.3614 to 0.4206 | 1026 | 0.5195 |

### `fdn-nonet-3k`

Top 30 of 265:

| Rank | Card | Rarity | Colour | Value | 95% interval | Games in hand | 17Lands GIH WR |
|---:|---|---|---|---:|---|---:|---:|
| 1 | Bloodthirsty Conqueror | M | B | 0.6636 | 0.6093 to 0.7180 | 207 | 0.6272 |
| 2 | Quilled Greatwurm | M | G | 0.6460 | 0.5913 to 0.7008 | 201 | 0.55 |
| 3 | Twinflame Tyrant | M | R | 0.6203 | 0.5660 to 0.6746 | 208 | 0.6048 |
| 4 | Sire of Seven Deaths | M | C | 0.5866 | 0.5486 to 0.6245 | 576 | 0.6203 |
| 5 | Mischievous Mystic | U | U | 0.5820 | 0.5304 to 0.6336 | 252 | 0.5957 |
| 6 | Felidar Savior | C | W | 0.5800 | 0.5263 to 0.6336 | 218 | 0.5736 |
| 7 | Angel of Finality | U | W | 0.5800 | 0.5262 to 0.6337 | 214 | 0.5668 |
| 8 | Perforating Artist | U | BR | 0.5796 | 0.5134 to 0.6458 | 69 | 0.5748 |
| 9 | Liliana, Dreadhorde General | M | B | 0.5774 | 0.5195 to 0.6354 | 163 | 0.6412 |
| 10 | Etali, Primal Storm | R | R | 0.5766 | 0.5222 to 0.6310 | 206 | 0.5326 |
| 11 | Authority of the Consuls | R | W | 0.5764 | 0.5205 to 0.6323 | 186 | 0.5203 |
| 12 | Sylvan Scavenging | R | G | 0.5759 | 0.5193 to 0.6326 | 177 | 0.6183 |
| 13 | Elenda, Saint of Dusk | R | WB | 0.5718 | 0.5054 to 0.6383 | 44 | 0.6154 |
| 14 | Giada, Font of Hope | R | W | 0.5708 | 0.5168 to 0.6247 | 212 | 0.6043 |
| 15 | Dragon Trainer | U | R | 0.5689 | 0.5142 to 0.6235 | 201 | 0.5647 |
| 16 | Spinner of Souls | R | G | 0.5665 | 0.5142 to 0.6189 | 237 | 0.6211 |
| 17 | Scrawling Crawler | R | C | 0.5658 | 0.5271 to 0.6045 | 553 | 0.5755 |
| 18 | Bigfin Bouncer | C | U | 0.5647 | 0.5128 to 0.6166 | 245 | 0.5709 |
| 19 | Kaito, Cunning Infiltrator | M | U | 0.5631 | 0.5046 to 0.6216 | 156 | 0.5827 |
| 20 | Vivien Reid | M | G | 0.5600 | 0.5008 to 0.6192 | 148 | 0.6139 |
| 21 | Dwynen, Gilt-Leaf Daen | U | G | 0.5596 | 0.5065 to 0.6127 | 225 | 0.5293 |
| 22 | Exemplar of Light | R | W | 0.5590 | 0.5063 to 0.6116 | 232 | 0.5899 |
| 23 | Felling Blow | U | G | 0.5571 | 0.5001 to 0.6141 | 173 | 0.5871 |
| 24 | Lathril, Blade of the Elves | R | BG | 0.5566 | 0.4899 to 0.6233 | 56 | 0.4917 |
| 25 | Dwynen's Elite | C | G | 0.5562 | 0.5037 to 0.6087 | 235 | 0.5307 |
| 26 | Niv-Mizzet, Visionary | M | UR | 0.5554 | 0.4887 to 0.6221 | 50 | 0.468 |
| 27 | Alesha, Who Laughs at Fate | R | BR | 0.5541 | 0.4875 to 0.6207 | 59 | 0.6183 |
| 28 | Krenko, Mob Boss | R | R | 0.5531 | 0.4987 to 0.6076 | 205 | 0.5533 |
| 29 | Dazzling Angel | C | W | 0.5531 | 0.5002 to 0.6060 | 227 | 0.5778 |
| 30 | Banishing Light | C | W | 0.5529 | 0.4964 to 0.6093 | 179 | 0.5745 |

Bottom 15:

| Rank | Card | Rarity | Colour | Value | 95% interval | Games in hand | 17Lands GIH WR |
|---:|---|---|---|---:|---|---:|---:|
| 251 | Grow from the Ashes | C | G | 0.4279 | 0.3713 to 0.4846 | 177 | 0.4809 |
| 252 | Doubling Season | M | G | 0.4276 | 0.3677 to 0.4876 | 138 | 0.3945 |
| 253 | Swiftfoot Boots | U | C | 0.4211 | 0.3780 to 0.4643 | 409 | 0.4384 |
| 254 | Wardens of the Cycle | U | BG | 0.4205 | 0.3542 to 0.4868 | 42 | 0.5115 |
| 255 | Grim Tutor | M | B | 0.4203 | 0.3625 to 0.4782 | 163 | 0.484 (188 games) |
| 256 | Aegis Turtle | C | U | 0.4149 | 0.3624 to 0.4673 | 237 | 0.4988 |
| 257 | Aetherize | U | U | 0.4119 | 0.3542 to 0.4696 | 165 | 0.5012 |
| 258 | Inspiring Call | U | G | 0.4075 | 0.3504 to 0.4647 | 170 | 0.4606 |
| 259 | Brass's Bounty | R | R | 0.4056 | 0.3460 to 0.4651 | 142 | 0.3714 (35 games) |
| 260 | Rune-Sealed Wall | U | U | 0.4015 | 0.3501 to 0.4530 | 252 | 0.5177 |
| 261 | Midnight Snack | U | B | 0.4011 | 0.3433 to 0.4589 | 163 | 0.4864 |
| 262 | Ravenous Amulet | U | C | 0.3986 | 0.3559 to 0.4413 | 420 | 0.5171 |
| 263 | Banner of Kinship | R | C | 0.3876 | 0.3454 to 0.4297 | 433 | 0.5062 |
| 264 | Divine Resilience | U | W | 0.3839 | 0.3292 to 0.4386 | 202 | 0.5079 |
| 265 | Goblin Surprise | C | R | 0.3805 | 0.3216 to 0.4393 | 150 | 0.5257 |
