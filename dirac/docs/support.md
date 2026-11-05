# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported |
|---|---:|---:|---:|---:|---:|
| BIG | 30 | 15 | 1 | 4 | 10 |
| BLB | 278 | 223 | 3 | 29 | 191 |
| DFT | 276 | 215 | 0 | 21 | 194 |
| DSK | 276 | 218 | 3 | 39 | 176 |
| ECL | 273 | 203 | 4 | 29 | 170 |
| EOE | 266 | 197 | 0 | 96 | 101 |
| FDN | 517 | 451 | 1 | 12 | 438 |
| FIN | 312 | 216 | 1 | 18 | 197 |
| FRA | 269 | 187 | 1 | 14 | 172 |
| HOB | 172 | 112 | 0 | 9 | 103 |
| LCI | 290 | 208 | 1 | 15 | 192 |
| MKM | 279 | 174 | 1 | 35 | 138 |
| MSH | 276 | 191 | 1 | 15 | 175 |
| OTJ | 276 | 197 | 2 | 21 | 174 |
| SOS | 270 | 217 | 1 | 22 | 194 |
| SPM | 192 | 145 | 1 | 21 | 123 |
| TDM | 276 | 197 | 3 | 39 | 155 |
| TLA | 285 | 199 | 2 | 46 | 151 |
| TMT | 194 | 137 | 1 | 8 | 128 |
| TRC | 337 | 256 | 0 | 19 | 237 |
| TRK | 311 | 256 | 0 | 0 | 256 |
| WOE | 281 | 171 | 0 | 12 | 159 |
| ALL | 34442 | 18885 | 31 | 15128 | 3726 |

`ALL` is the whole corpus; the rest are the Standard sets.

## What supported covers

Evidence comes from the `fdn` and `blb` deck pools and one synthetic pool per Standard set: 1,500 pairs each, seed 1, the random policy, both surfaces, mulligans 0 and 1, every run passing `kfuzz -absolute`. A synthetic pool deals its decks from the cards it plays, so no pair is lost to a refused card; it skips only a pair the kernel refuses together, so it can play fewer games, and its cards met the floor in these:

| pool | games per run |
|---|---:|
| blb | 1500 |
| fdn | 1500 |
| pauper | 1500 |
| std-big | 115 |
| std-blb | 75 |
| std-dft | 194 |
| std-dsk | 105 |
| std-ecl | 45 |
| std-eoe | 10 |
| std-fdn | 568 |
| std-fin | 199 |
| std-fra | 214 |
| std-hob | 97 |
| std-lci | 250 |
| std-mkm | 252 |
| std-msh | 275 |
| std-otj | 70 |
| std-sos | 118 |
| std-spm | 74 |
| std-tdm | 43 |
| std-tla | 42 |
| std-tmt | 295 |
| std-trc | 1500 |
| std-trk | 1500 |
| std-woe | 146 |

Nothing else is covered. A card none of these runs reaches is unverified, whatever it does. A supported card has played clean in these games, not in every game: the standing sweep and census are separate runs, last measured at 164 and 244 divergent pairs, and are outside this claim.

## Refused by name

38 cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.

| what differed | cards |
|---|---:|
| decision | 2 |
| options | 11 |
| projection | 1 |
| projection-Cards.Chars.Colors | 1 |
| projection-Cards.Counters | 1 |
| projection-Cards.Damage | 1 |
| projection-Cards.Flags | 4 |
| projection-Cards.Pos | 2 |
| projection-Cards.Power | 2 |
| projection-Cards.Zone | 1 |
| projection-NStack | 3 |
| projection-Pool | 2 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-outsidetype | 1 |
| unmodelled-tokatkask | 2 |
| unmodelled-tokenpt | 1 |
