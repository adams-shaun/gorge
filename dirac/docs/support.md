# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported |
|---|---:|---:|---:|---:|---:|
| BIG | 30 | 25 | 1 | 3 | 21 |
| BLB | 278 | 260 | 3 | 5 | 252 |
| DFT | 276 | 252 | 0 | 6 | 246 |
| DSK | 276 | 245 | 4 | 24 | 217 |
| ECL | 273 | 239 | 2 | 4 | 233 |
| EOE | 266 | 244 | 0 | 5 | 239 |
| FDN | 517 | 517 | 0 | 0 | 517 |
| FIN | 312 | 271 | 1 | 4 | 266 |
| FRA | 269 | 251 | 0 | 12 | 239 |
| HOB | 172 | 157 | 0 | 4 | 153 |
| LCI | 290 | 257 | 1 | 27 | 229 |
| MKM | 279 | 246 | 1 | 35 | 210 |
| MSH | 276 | 241 | 1 | 8 | 232 |
| OTJ | 276 | 244 | 1 | 2 | 241 |
| SOS | 270 | 255 | 0 | 6 | 249 |
| SPM | 192 | 165 | 1 | 6 | 158 |
| TDM | 276 | 255 | 3 | 7 | 245 |
| TLA | 285 | 260 | 1 | 8 | 251 |
| TMT | 194 | 176 | 1 | 5 | 170 |
| TRC | 337 | 292 | 0 | 26 | 266 |
| TRK | 311 | 311 | 0 | 0 | 311 |
| WOE | 281 | 258 | 0 | 12 | 246 |
| ALL | 34442 | 22001 | 25 | 16887 | 5089 |

`ALL` is the whole corpus; the rest are the Standard sets.

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

25 cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.

| what differed | cards |
|---|---:|
| decision | 2 |
| lockstep | 1 |
| options | 5 |
| options-options | 1 |
| projection | 1 |
| projection-Cards.Chars.Colors | 1 |
| projection-Cards.Damage | 1 |
| projection-Cards.Flags | 3 |
| projection-Cards.Power | 1 |
| projection-Cards.Zone | 1 |
| projection-NStack | 2 |
| projection-Pool | 1 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-tokatkask | 1 |
| unmodelled-tokenpt | 1 |
