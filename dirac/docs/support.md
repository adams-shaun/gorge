# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported |
|---|---:|---:|---:|---:|---:|
| BIG | 30 | 23 | 1 | 8 | 14 |
| BLB | 278 | 256 | 5 | 31 | 220 |
| DFT | 276 | 246 | 0 | 22 | 224 |
| DSK | 276 | 231 | 8 | 35 | 188 |
| ECL | 273 | 230 | 9 | 31 | 190 |
| EOE | 266 | 236 | 0 | 118 | 118 |
| FDN | 517 | 505 | 5 | 7 | 493 |
| FIN | 312 | 266 | 5 | 16 | 245 |
| FRA | 269 | 244 | 6 | 18 | 220 |
| HOB | 172 | 152 | 0 | 9 | 143 |
| LCI | 290 | 249 | 6 | 15 | 228 |
| MKM | 279 | 227 | 4 | 37 | 186 |
| MSH | 276 | 223 | 8 | 8 | 207 |
| OTJ | 276 | 238 | 9 | 17 | 212 |
| SOS | 270 | 247 | 6 | 20 | 221 |
| SPM | 192 | 158 | 4 | 18 | 136 |
| TDM | 276 | 242 | 8 | 45 | 189 |
| TLA | 285 | 244 | 10 | 49 | 185 |
| TMT | 194 | 170 | 2 | 8 | 160 |
| TRC | 337 | 267 | 1 | 104 | 162 |
| TRK | 311 | 248 | 0 | 160 | 88 |
| WOE | 281 | 251 | 3 | 9 | 239 |
| ALL | 34442 | 21190 | 105 | 16918 | 4167 |

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
| std-trc | 340 |
| std-trk | 111 |
| std-woe | 146 |

Nothing else is covered. A card none of these runs reaches is unverified, whatever it does. A supported card has played clean in these games, not in every game: the standing sweep and census are separate runs, last measured at 164 and 244 divergent pairs, and are outside this claim.

## Refused by name

105 cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.

| what differed | cards |
|---|---:|
| decision | 2 |
| options | 20 |
| projection | 1 |
| projection-Cards.Chars.Colors | 2 |
| projection-Cards.Chars.Keywords | 2 |
| projection-Cards.Counters | 3 |
| projection-Cards.Damage | 1 |
| projection-Cards.Flags | 13 |
| projection-Cards.Pos | 10 |
| projection-Cards.Power | 6 |
| projection-Cards.Zone | 5 |
| projection-Life | 4 |
| projection-NInst | 1 |
| projection-NStack | 13 |
| projection-Pool | 4 |
| projection-Stack.Ability | 1 |
| projection-Step | 1 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-copycast | 2 |
| unmodelled-entryorder | 3 |
| unmodelled-gravepower | 1 |
| unmodelled-handsize | 1 |
| unmodelled-landgrant | 1 |
| unmodelled-outsidetype | 1 |
| unmodelled-tokatkask | 3 |
| unmodelled-tokenpt | 1 |
