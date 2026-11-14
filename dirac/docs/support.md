# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported | 4-seat |
|---|---:|---:|---:|---:|---:|---:|
| BIG | 30 | 17 | 0 | 1 | 16 | 16 |
| BLB | 278 | 227 | 2 | 0 | 225 | 224 |
| DFT | 276 | 221 | 0 | 3 | 218 | 218 |
| DSK | 276 | 231 | 4 | 2 | 225 | 225 |
| ECL | 273 | 214 | 1 | 0 | 213 | 212 |
| EOE | 266 | 209 | 0 | 11 | 198 | 198 |
| FDN | 517 | 461 | 0 | 0 | 461 | 458 |
| FIN | 312 | 226 | 0 | 2 | 224 | 223 |
| FRA | 269 | 193 | 0 | 4 | 189 | 186 |
| HOB | 172 | 117 | 0 | 0 | 117 | 116 |
| LCI | 290 | 217 | 1 | 17 | 199 | 198 |
| MKM | 279 | 193 | 2 | 24 | 167 | 165 |
| MSH | 276 | 210 | 1 | 10 | 199 | 199 |
| OTJ | 276 | 206 | 1 | 2 | 203 | 202 |
| SOS | 270 | 226 | 0 | 0 | 226 | 226 |
| SPM | 192 | 151 | 1 | 1 | 149 | 148 |
| TDM | 276 | 207 | 1 | 0 | 206 | 206 |
| TLA | 285 | 212 | 1 | 5 | 206 | 205 |
| TMT | 194 | 147 | 0 | 0 | 147 | 146 |
| TRC | 337 | 269 | 0 | 29 | 240 | 238 |
| TRK | 311 | 256 | 0 | 0 | 256 | 251 |
| WOE | 281 | 179 | 0 | 13 | 166 | 166 |
| MBC | 2195 | 1581 | 0 | 837 | 744 | 730 |
| ALL | 34442 | 20877 | 18 | 15842 | 5017 | 4978 |

`ALL` is the whole corpus; `MBC` is the Mystery Booster Commander draft pool, every name its packs deal that gorge supports; the rest are the Standard sets. `4-seat` counts the supported cards whose lowering holds at any seat count: a card whose lowering reads the one opponent of a two-player game (`kop.CardDef.TwoSeat`) is refused in a game of three or more seats (`StallTwoSeat`). It is a ceiling, not evidence: no 4-seat game is in the evidence runs.

## What supported covers

Evidence comes from the `fdn` and `blb` deck pools and one synthetic pool per Standard set: 1,500 pairs each, seed 1, the random policy, both surfaces, mulligans 0 and 1, every run passing `kfuzz -absolute`. A synthetic pool deals its decks from the cards it plays, so no pair is lost to a refused card; it skips only a pair the kernel refuses together, so it can play fewer games, and its cards met the floor in these:

| pool | games per run |
|---|---:|
| blb | 1500 |
| cmd5 | 1500 |
| dr-mbc-aaaa | 1362 |
| dr-mbc-abbb | 1010 |
| ev-dr-mbc-aaaa | 810 |
| ev-dr-mbc-aaab | 200 |
| ev-dr-mbc-aaba | 608 |
| ev-dr-mbc-aabb | 457 |
| ev-dr-mbc-abaa | 768 |
| ev-dr-mbc-abab | 63 |
| ev-dr-mbc-abba | 128 |
| ev-dr-mbc-abbb | 328 |
| ev-dr-mbc-baaa | 520 |
| ev-dr-mbc-baab | 712 |
| ev-dr-mbc-baba | 543 |
| ev-dr-mbc-babb | 200 |
| ev-dr-mbc-bbaa | 712 |
| ev-dr-mbc-bbab | 520 |
| ev-dr-mbc-bbba | 552 |
| ev-dr-mbc-bbbb | 96 |
| ev-std-big | 200 |
| ev-std-dft | 200 |
| ev-std-dsk | 11 |
| ev-std-eoe | 200 |
| ev-std-fin | 200 |
| ev-std-fra | 200 |
| ev-std-hob | 200 |
| ev-std-lci | 168 |
| ev-std-mkm | 264 |
| ev-std-msh | 169 |
| ev-std-otj | 55 |
| ev-std-sos | 200 |
| ev-std-spm | 200 |
| ev-std-tdm | 200 |
| ev-std-tla | 200 |
| fdn | 1500 |
| pauper | 1500 |
| std-big | 1500 |
| std-blb | 1500 |
| std-dft | 1500 |
| std-dsk | 1207 |
| std-ecl | 1471 |
| std-eoe | 1500 |
| std-fdn | 1474 |
| std-fin | 1500 |
| std-fra | 1500 |
| std-hob | 1500 |
| std-lci | 250 |
| std-mkm | 1500 |
| std-msh | 1500 |
| std-otj | 1500 |
| std-sos | 1500 |
| std-spm | 1500 |
| std-tdm | 1500 |
| std-tla | 1500 |
| std-tmt | 1430 |
| std-trc | 1500 |
| std-trk | 1500 |
| std-woe | 1500 |

Nothing else is covered. A card none of these runs reaches is unverified, whatever it does. A supported card has played clean in these games, not in every game: the standing sweep and census are separate runs, last measured at 164 and 244 divergent pairs, and are outside this claim.

## Refused by name

20 cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.

| what differed | cards |
|---|---:|
| decision | 1 |
| options | 4 |
| options-options | 1 |
| projection | 1 |
| projection-Cards.Chars.Colors | 1 |
| projection-Cards.Damage | 1 |
| projection-Cards.Flags | 2 |
| projection-Cards.Power | 1 |
| projection-Cards.Zone | 1 |
| projection-NStack | 2 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-tokatkask | 1 |
| unmodelled-tokenpt | 1 |
