# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported | 4-seat |
|---|---:|---:|---:|---:|---:|---:|
| BIG | 30 | 27 | 0 | 2 | 25 | 25 |
| BLB | 278 | 261 | 2 | 0 | 259 | 258 |
| DFT | 276 | 255 | 0 | 4 | 251 | 251 |
| DSK | 276 | 251 | 4 | 2 | 245 | 245 |
| ECL | 273 | 244 | 1 | 0 | 243 | 242 |
| EOE | 266 | 250 | 0 | 11 | 239 | 239 |
| FDN | 517 | 517 | 0 | 0 | 517 | 514 |
| FIN | 312 | 276 | 0 | 3 | 273 | 272 |
| FRA | 269 | 255 | 0 | 6 | 249 | 245 |
| HOB | 172 | 159 | 0 | 0 | 159 | 157 |
| LCI | 290 | 260 | 1 | 21 | 238 | 235 |
| MKM | 279 | 252 | 2 | 30 | 220 | 216 |
| MSH | 276 | 246 | 1 | 11 | 234 | 234 |
| OTJ | 276 | 252 | 1 | 2 | 249 | 248 |
| SOS | 270 | 257 | 0 | 0 | 257 | 257 |
| SPM | 192 | 169 | 1 | 2 | 166 | 165 |
| TDM | 276 | 257 | 3 | 1 | 253 | 253 |
| TLA | 285 | 261 | 1 | 5 | 255 | 254 |
| TMT | 194 | 183 | 0 | 0 | 183 | 182 |
| TRC | 337 | 301 | 0 | 32 | 269 | 266 |
| TRK | 311 | 311 | 0 | 0 | 311 | 306 |
| WOE | 281 | 264 | 0 | 18 | 246 | 245 |
| MBC | 2188 | 1772 | 1 | 917 | 854 | 838 |
| ALL | 34442 | 23649 | 21 | 17608 | 6020 | 5971 |

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
| ev-dr-mbc-aaab | 648 |
| ev-dr-mbc-aaba | 608 |
| ev-dr-mbc-aabb | 457 |
| ev-dr-mbc-abaa | 768 |
| ev-dr-mbc-abab | 443 |
| ev-dr-mbc-abba | 454 |
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

21 cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.

| what differed | cards |
|---|---:|
| decision | 1 |
| options | 4 |
| options-options | 1 |
| projection | 2 |
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
