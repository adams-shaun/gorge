# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported |
|---|---:|---:|---:|---:|---:|
| BIG | 30 | 23 | 2 | 8 | 13 |
| BLB | 278 | 253 | 9 | 5 | 239 |
| DFT | 276 | 242 | 8 | 10 | 224 |
| DSK | 276 | 230 | 10 | 32 | 188 |
| ECL | 273 | 230 | 9 | 31 | 190 |
| EOE | 266 | 236 | 3 | 116 | 117 |
| FDN | 517 | 505 | 9 | 3 | 493 |
| FIN | 312 | 264 | 9 | 7 | 248 |
| FRA | 269 | 240 | 8 | 12 | 220 |
| HOB | 172 | 151 | 6 | 2 | 143 |
| LCI | 290 | 244 | 10 | 13 | 221 |
| MKM | 279 | 227 | 7 | 34 | 186 |
| MSH | 276 | 221 | 8 | 6 | 207 |
| OTJ | 276 | 235 | 12 | 11 | 212 |
| SOS | 270 | 246 | 8 | 17 | 221 |
| SPM | 192 | 158 | 4 | 18 | 136 |
| TDM | 276 | 242 | 10 | 42 | 190 |
| TLA | 285 | 242 | 11 | 46 | 185 |
| TMT | 194 | 167 | 4 | 2 | 161 |
| WOE | 281 | 250 | 3 | 9 | 238 |
| ALL | 34072 | 20731 | 155 | 16697 | 3879 |

`ALL` is the whole corpus; the rest are the Standard sets.

## What supported covers

Evidence comes from the `fdn` and `blb` deck pools and one synthetic pool per Standard set: 1,500 pairs each, seed 1, the random policy, both surfaces, mulligans 0 and 1, every run passing `kfuzz -absolute`. A synthetic pool skips a pair holding a refused card and a pair the kernel refuses together, so it plays fewer games, and its cards met the floor in these:

| pool | games per run |
|---|---:|
| blb | 1500 |
| fdn | 1500 |
| std-big | 115 |
| std-blb | 78 |
| std-dft | 194 |
| std-dsk | 105 |
| std-ecl | 45 |
| std-eoe | 10 |
| std-fdn | 568 |
| std-fin | 220 |
| std-fra | 214 |
| std-hob | 97 |
| std-lci | 150 |
| std-mkm | 252 |
| std-msh | 275 |
| std-otj | 70 |
| std-sos | 118 |
| std-spm | 74 |
| std-tdm | 43 |
| std-tla | 42 |
| std-tmt | 342 |
| std-woe | 146 |

Nothing else is covered. A card none of these runs reaches is unverified, whatever it does. A supported card has played clean in these games, not in every game: the standing sweep and census are separate runs, last measured at 164 and 244 divergent pairs, and are outside this claim.

## Refused by name

155 cards, by what differed in the game that showed them. Each has a file in `dirac/support/refused` naming the card, the difference and the game.

| what differed | cards |
|---|---:|
| capacity | 1 |
| decision | 4 |
| options | 33 |
| projection | 1 |
| projection-Cards.AttachedTo | 1 |
| projection-Cards.Chars.Colors | 1 |
| projection-Cards.Chars.Keywords | 2 |
| projection-Cards.Counters | 2 |
| projection-Cards.Damage | 1 |
| projection-Cards.Flags | 14 |
| projection-Cards.Pos | 13 |
| projection-Cards.Power | 10 |
| projection-Cards.Zone | 17 |
| projection-Life | 6 |
| projection-NInst | 1 |
| projection-NStack | 27 |
| projection-Pool | 7 |
| projection-Stack.Ability | 1 |
| projection-Step | 1 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-entryorder | 2 |
| unmodelled-gravepower | 1 |
| unmodelled-handsize | 1 |
| unmodelled-landgrant | 1 |
| unmodelled-outsidetype | 1 |
| unmodelled-tokatkask | 2 |
| unmodelled-tokenpt | 1 |
