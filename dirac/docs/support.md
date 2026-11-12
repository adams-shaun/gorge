# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported | 4-seat |
|---|---:|---:|---:|---:|---:|---:|
| BIG | 30 | 27 | 0 | 6 | 21 | 21 |
| BLB | 278 | 261 | 2 | 7 | 252 | 251 |
| DFT | 276 | 255 | 0 | 4 | 251 | 251 |
| DSK | 276 | 251 | 4 | 30 | 217 | 217 |
| ECL | 273 | 244 | 1 | 10 | 233 | 232 |
| EOE | 266 | 250 | 0 | 11 | 239 | 239 |
| FDN | 517 | 517 | 0 | 0 | 517 | 515 |
| FIN | 312 | 276 | 1 | 9 | 266 | 265 |
| FRA | 269 | 255 | 0 | 16 | 239 | 235 |
| HOB | 172 | 159 | 0 | 6 | 153 | 152 |
| LCI | 290 | 260 | 1 | 30 | 229 | 226 |
| MKM | 279 | 251 | 2 | 36 | 213 | 209 |
| MSH | 276 | 246 | 1 | 13 | 232 | 232 |
| OTJ | 276 | 251 | 1 | 9 | 241 | 241 |
| SOS | 270 | 257 | 0 | 8 | 249 | 249 |
| SPM | 192 | 169 | 1 | 10 | 158 | 157 |
| TDM | 276 | 256 | 3 | 8 | 245 | 245 |
| TLA | 285 | 261 | 1 | 9 | 251 | 250 |
| TMT | 194 | 183 | 1 | 12 | 170 | 169 |
| TRC | 337 | 301 | 0 | 35 | 266 | 263 |
| TRK | 311 | 311 | 0 | 0 | 311 | 306 |
| WOE | 281 | 263 | 0 | 17 | 246 | 245 |
| ALL | 34442 | 23068 | 24 | 17950 | 5094 | 5065 |

`ALL` is the whole corpus; the rest are the Standard sets. `4-seat` counts the supported cards whose lowering holds at any seat count: a card whose lowering reads the one opponent of a two-player game (`kop.CardDef.TwoSeat`) is refused in a game of three or more seats (`StallTwoSeat`). It is a ceiling, not evidence: no 4-seat game is in the evidence runs.

## What supported covers

Evidence comes from the `fdn` and `blb` deck pools and one synthetic pool per Standard set: 1,500 pairs each, seed 1, the random policy, both surfaces, mulligans 0 and 1, every run passing `kfuzz -absolute`. A synthetic pool deals its decks from the cards it plays, so no pair is lost to a refused card; it skips only a pair the kernel refuses together, so it can play fewer games, and its cards met the floor in these:

| pool | games per run |
|---|---:|
| blb | 1500 |
| fdn | 1500 |
| pauper | 1500 |
| std-big | 1500 |
| std-blb | 1500 |
| std-dft | 1500 |
| std-dsk | 1464 |
| std-ecl | 1455 |
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
| std-tmt | 1500 |
| std-trc | 1500 |
| std-trk | 1500 |
| std-woe | 1500 |

Nothing else is covered. A card none of these runs reaches is unverified, whatever it does. A supported card has played clean in these games, not in every game: the standing sweep and census are separate runs, last measured at 164 and 244 divergent pairs, and are outside this claim.

## Refused by name

24 cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.

| what differed | cards |
|---|---:|
| decision | 2 |
| options | 5 |
| options-options | 1 |
| projection | 1 |
| projection-Cards.Chars.Colors | 1 |
| projection-Cards.Damage | 1 |
| projection-Cards.Flags | 2 |
| projection-Cards.Power | 1 |
| projection-Cards.Zone | 1 |
| projection-NStack | 3 |
| projection-Pool | 1 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-tokatkask | 1 |
| unmodelled-tokenpt | 1 |
