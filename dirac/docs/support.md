# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported |
|---|---:|---:|---:|---:|---:|
| BIG | 30 | 23 | 1 | 1 | 21 |
| BLB | 278 | 256 | 3 | 33 | 220 |
| DFT | 276 | 248 | 0 | 2 | 246 |
| DSK | 276 | 242 | 4 | 50 | 188 |
| ECL | 273 | 234 | 1 | 43 | 190 |
| EOE | 266 | 239 | 0 | 0 | 239 |
| FDN | 517 | 517 | 0 | 24 | 493 |
| FIN | 312 | 268 | 1 | 22 | 245 |
| FRA | 269 | 245 | 0 | 6 | 239 |
| HOB | 172 | 153 | 0 | 1 | 152 |
| LCI | 290 | 251 | 1 | 22 | 228 |
| MKM | 279 | 242 | 1 | 31 | 210 |
| MSH | 276 | 235 | 1 | 27 | 207 |
| OTJ | 276 | 241 | 1 | 28 | 212 |
| SOS | 270 | 249 | 0 | 28 | 221 |
| SPM | 192 | 161 | 1 | 24 | 136 |
| TDM | 276 | 250 | 3 | 5 | 242 |
| TLA | 285 | 253 | 1 | 2 | 250 |
| TMT | 194 | 171 | 1 | 0 | 170 |
| TRC | 337 | 287 | 0 | 21 | 266 |
| TRK | 311 | 311 | 0 | 0 | 311 |
| WOE | 281 | 252 | 0 | 6 | 246 |
| ALL | 34442 | 21705 | 24 | 16850 | 4831 |

`ALL` is the whole corpus; the rest are the Standard sets.

## What supported covers

Evidence comes from the `fdn` and `blb` deck pools and one synthetic pool per Standard set: 1,500 pairs each, seed 1, the random policy, both surfaces, mulligans 0 and 1, every run passing `kfuzz -absolute`. A synthetic pool deals its decks from the cards it plays, so no pair is lost to a refused card; it skips only a pair the kernel refuses together, so it can play fewer games, and its cards met the floor in these:

| pool | games per run |
|---|---:|
| blb | 1500 |
| fdn | 1500 |
| pauper | 1500 |
| std-big | 1500 |
| std-blb | 75 |
| std-dft | 1500 |
| std-dsk | 105 |
| std-ecl | 45 |
| std-eoe | 1500 |
| std-fdn | 568 |
| std-fin | 199 |
| std-fra | 1500 |
| std-hob | 1500 |
| std-lci | 250 |
| std-mkm | 1500 |
| std-msh | 275 |
| std-otj | 70 |
| std-sos | 118 |
| std-spm | 74 |
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
| projection-Cards.Flags | 3 |
| projection-Cards.Power | 1 |
| projection-Cards.Zone | 1 |
| projection-NStack | 2 |
| projection-Pool | 1 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-tokatkask | 1 |
| unmodelled-tokenpt | 1 |
