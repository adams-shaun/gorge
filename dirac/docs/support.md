# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported |
|---|---:|---:|---:|---:|---:|
| BIG | 30 | 23 | 1 | 8 | 14 |
| BLB | 278 | 256 | 3 | 33 | 220 |
| DFT | 276 | 247 | 0 | 23 | 224 |
| DSK | 276 | 242 | 3 | 51 | 188 |
| ECL | 273 | 233 | 2 | 41 | 190 |
| EOE | 266 | 238 | 0 | 0 | 238 |
| FDN | 517 | 517 | 0 | 24 | 493 |
| FIN | 312 | 266 | 2 | 19 | 245 |
| FRA | 269 | 245 | 0 | 25 | 220 |
| HOB | 172 | 152 | 0 | 9 | 143 |
| LCI | 290 | 251 | 1 | 22 | 228 |
| MKM | 279 | 242 | 1 | 55 | 186 |
| MSH | 276 | 232 | 1 | 24 | 207 |
| OTJ | 276 | 241 | 2 | 27 | 212 |
| SOS | 270 | 248 | 1 | 26 | 221 |
| SPM | 192 | 160 | 1 | 23 | 136 |
| TDM | 276 | 249 | 4 | 56 | 189 |
| TLA | 285 | 252 | 1 | 66 | 185 |
| TMT | 194 | 171 | 1 | 10 | 160 |
| TRC | 337 | 287 | 0 | 21 | 266 |
| TRK | 311 | 311 | 0 | 0 | 311 |
| WOE | 281 | 252 | 0 | 13 | 239 |
| ALL | 34442 | 21659 | 28 | 17020 | 4611 |

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
| std-eoe | 1500 |
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

28 cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.

| what differed | cards |
|---|---:|
| decision | 2 |
| options | 7 |
| projection | 1 |
| projection-Cards.Chars.Colors | 1 |
| projection-Cards.Damage | 1 |
| projection-Cards.Flags | 4 |
| projection-Cards.Power | 2 |
| projection-Cards.Zone | 1 |
| projection-NStack | 2 |
| projection-Pool | 2 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-tokatkask | 1 |
| unmodelled-tokenpt | 1 |
