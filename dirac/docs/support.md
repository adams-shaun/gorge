# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is true: a game plays supported cards only, and a deck holding an unverified or refused card is turned away.

| list | cards | lowered | refused | unverified | supported | 4-seat |
|---|---:|---:|---:|---:|---:|---:|
| BIG | 30 | 27 | 0 | 2 | 25 | 25 |
| BLB | 278 | 261 | 2 | 0 | 259 | 258 |
| DFT | 276 | 255 | 0 | 2 | 253 | 253 |
| DSK | 276 | 251 | 4 | 2 | 245 | 245 |
| ECL | 273 | 244 | 1 | 0 | 243 | 242 |
| EOE | 266 | 251 | 0 | 2 | 249 | 249 |
| FDN | 517 | 517 | 0 | 0 | 517 | 514 |
| FIN | 312 | 277 | 0 | 2 | 275 | 274 |
| FRA | 269 | 255 | 0 | 6 | 249 | 245 |
| HOB | 172 | 159 | 0 | 0 | 159 | 157 |
| LCI | 290 | 260 | 1 | 0 | 259 | 256 |
| MKM | 279 | 252 | 2 | 22 | 228 | 224 |
| MSH | 276 | 248 | 1 | 5 | 242 | 242 |
| OTJ | 276 | 253 | 1 | 1 | 251 | 249 |
| SOS | 270 | 257 | 0 | 0 | 257 | 257 |
| SPM | 192 | 169 | 1 | 2 | 166 | 165 |
| TDM | 276 | 257 | 3 | 0 | 254 | 254 |
| TLA | 285 | 261 | 1 | 3 | 257 | 256 |
| TMT | 194 | 184 | 0 | 0 | 184 | 183 |
| TRC | 337 | 302 | 0 | 32 | 270 | 267 |
| TRK | 311 | 311 | 0 | 0 | 311 | 306 |
| WOE | 281 | 265 | 0 | 10 | 255 | 253 |
| MBC | 2195 | 1931 | 1 | 352 | 1578 | 1547 |
| ALL | 34442 | 24266 | 21 | 16112 | 8133 | 8059 |

`ALL` is the whole corpus; `MBC` is the Mystery Booster Commander draft pool, every name its packs deal that gorge supports; the rest are the Standard sets. `4-seat` counts the supported cards whose lowering holds at any seat count: a card whose lowering reads the one opponent of a two-player game (`kop.CardDef.TwoSeat`) is refused in a game of three or more seats (`StallTwoSeat`). It is a ceiling, not evidence: no 4-seat game is in the evidence runs.

## What supported covers

Evidence comes from the `fdn` and `blb` deck pools and one synthetic pool per Standard set: 1,500 pairs each, seed 1, the random policy, both surfaces, mulligans 0 and 1, every run passing `kfuzz -absolute`. A synthetic pool deals its decks from the cards it plays, so no pair is lost to a refused card; it skips only a pair the kernel refuses together, so it can play fewer games, and its cards met the floor in these:

| pool | games per run |
|---|---:|
| blb | 1500 |
| cmd5 | 1500 |
| dr-mbc-aaaa | 1362 |
| dr-mbc-abbb | 1010 |
| ev-dr-mbc-aaaa | 200 |
| ev-dr-mbc-aaab | 188 |
| ev-dr-mbc-aaba | 97 |
| ev-dr-mbc-aabb | 95 |
| ev-dr-mbc-abaa | 200 |
| ev-dr-mbc-abab | 76 |
| ev-dr-mbc-abba | 131 |
| ev-dr-mbc-abbb | 29 |
| ev-dr-mbc-baaa | 200 |
| ev-dr-mbc-baab | 200 |
| ev-dr-mbc-baba | 155 |
| ev-dr-mbc-babb | 200 |
| ev-dr-mbc-bbaa | 200 |
| ev-dr-mbc-bbab | 96 |
| ev-dr-mbc-bbba | 200 |
| ev-dr-mbc-bbbb | 106 |
| ev-std-big | 200 |
| ev-std-blb | 200 |
| ev-std-dft | 200 |
| ev-std-dsk | 6 |
| ev-std-ecl | 200 |
| ev-std-eoe | 200 |
| ev-std-fin | 198 |
| ev-std-fra | 200 |
| ev-std-hob | 200 |
| ev-std-lci | 173 |
| ev-std-mkm | 200 |
| ev-std-msh | 200 |
| ev-std-otj | 161 |
| ev-std-sos | 200 |
| ev-std-spm | 200 |
| ev-std-tdm | 200 |
| ev-std-tla | 200 |
| ev-std-tmt | 186 |
| ev-std-trk | 200 |
| ev-std-woe | 200 |
| ev-tail-00 | 966 |
| ev-tail-06 | 216 |
| ev-tail-06-c000 | 200 |
| ev-tail-08 | 1120 |
| ev-tail-10 | 200 |
| ev-tail-10-c000 | 200 |
| ev-tail-17 | 157 |
| ev-tail-17-c000 | 31 |
| ev-tail-17-c001 | 41 |
| ev-tail-17-c002 | 29 |
| ev-tail-17-c003 | 35 |
| ev-tail-17-c004 | 44 |
| ev-tail-17-c005 | 43 |
| ev-tail-17-c006 | 14 |
| ev-tail-17-c007 | 30 |
| ev-tail-17-c008 | 37 |
| ev-tail-17-c009 | 43 |
| ev-tail-17-c010 | 43 |
| ev-tail-17-c011 | 28 |
| ev-tail-17-c012 | 30 |
| ev-tail-17-c013 | 44 |
| ev-tail-17-c014 | 22 |
| ev-tail-24 | 719 |
| ev-tail-33 | 200 |
| ev-tail-33-c000 | 200 |
| ev-tail-39 | 200 |
| ev-tail-39-c000 | 48 |
| ev-tail-39-c001 | 48 |
| ev-tail-39-c002 | 48 |
| ev-tail-39-c003 | 48 |
| ev-tail-39-c004 | 48 |
| ev-tail-39-c005 | 48 |
| ev-tail-41 | 200 |
| ev-tail-41-c000 | 48 |
| ev-tail-41-c001 | 48 |
| ev-tail-41-c002 | 48 |
| ev-tail-41-c003 | 48 |
| ev-tail-41-c004 | 48 |
| ev-tail-41-c005 | 48 |
| ev-tail-41-c006 | 48 |
| ev-tail-41-c007 | 48 |
| ev-tail-41-c008 | 48 |
| ev-tail-41-c009 | 48 |
| ev-tail-41-c010 | 48 |
| ev-tail-41-c011 | 48 |
| ev-tail-41-c012 | 48 |
| ev-tail-41-c013 | 48 |
| ev-tail-41-c014 | 48 |
| ev-tail-41-c015 | 48 |
| ev-tail-41-c016 | 48 |
| ev-tail-41-c017 | 48 |
| ev-tail-41-c018 | 48 |
| ev-tail-41-c019 | 48 |
| ev-tail-41-c020 | 48 |
| ev-tail-41-c021 | 48 |
| ev-tail-41-c022 | 48 |
| ev-tail-41-c023 | 48 |
| ev-tail-41-c024 | 48 |
| ev-tail-44 | 200 |
| ev-tail-44-c000 | 27 |
| ev-tail-44-c001 | 14 |
| ev-tail-44-c002 | 28 |
| ev-tail-44-c003 | 29 |
| ev-tail-44-c004 | 13 |
| ev-tail-44-c005 | 7 |
| ev-tail-46 | 200 |
| ev-tail-46-c000 | 50 |
| ev-tail-46-c001 | 50 |
| ev-tail-46-c002 | 50 |
| ev-tail-46-c003 | 50 |
| ev-tail-49 | 131 |
| ev-tail-49-c000 | 29 |
| ev-tail-49-c001 | 34 |
| ev-tail-49-c002 | 33 |
| ev-tail-49-c003 | 39 |
| ev-tail-49-c004 | 25 |
| ev-tail-49-c005 | 20 |
| ev-tail-49-c006 | 21 |
| ev-tail-49-c007 | 29 |
| ev-tail-49-c008 | 28 |
| ev-tail-49-c009 | 34 |
| ev-tail-49-c010 | 32 |
| ev-tail-49-c011 | 33 |
| ev-tail-49-c012 | 36 |
| ev-tail-49-c013 | 31 |
| ev-tail-49-c014 | 37 |
| ev-tail-49-c015 | 41 |
| ev-tail-49-c016 | 36 |
| ev-tail-53 | 200 |
| ev-tail-53-c000 | 48 |
| ev-tail-53-c001 | 48 |
| ev-tail-53-c002 | 48 |
| ev-tail-53-c003 | 48 |
| ev-tail-53-c004 | 48 |
| ev-tail-53-c005 | 48 |
| ev-tail-53-c006 | 48 |
| ev-tail-53-c007 | 48 |
| ev-tail-55 | 200 |
| ev-tail-56 | 134 |
| ev-tail-56-c001 | 39 |
| ev-tail-57 | 200 |
| ev-tail-57-c000 | 200 |
| ev-tail-62 | 200 |
| ev-tail-62-c000 | 48 |
| ev-tail-62-c001 | 48 |
| ev-tail-62-c002 | 48 |
| ev-tail-62-c003 | 48 |
| ev-tail-62-c004 | 48 |
| ev-tail-62-c005 | 48 |
| ev-tail-62-c006 | 48 |
| ev-tail-62-c007 | 48 |
| ev-tail-62-c008 | 48 |
| ev-tail-62-c009 | 48 |
| ev-tail-62-c010 | 48 |
| ev-tail-62-c011 | 48 |
| ev-tail-62-c012 | 48 |
| ev-tail-62-c013 | 48 |
| ev-tail-62-c014 | 48 |
| ev-tail-62-c015 | 48 |
| ev-tail-62-c016 | 48 |
| ev-tail-62-c017 | 48 |
| ev-tail-62-c018 | 48 |
| ev-tail-62-c019 | 48 |
| ev-tail-62-c020 | 48 |
| ev-tail-62-c021 | 48 |
| ev-tail-62-c022 | 48 |
| ev-tail-62-c023 | 48 |
| ev-tail-62-c024 | 48 |
| ev-tail-62-c025 | 48 |
| ev-tail-62-c026 | 48 |
| ev-tail-66 | 200 |
| ev-tail-66-c000 | 43 |
| ev-tail-66-c001 | 23 |
| ev-tail-66-c002 | 39 |
| ev-tail-66-c003 | 29 |
| ev-tail-72 | 200 |
| ev-tail-73 | 200 |
| ev-tail-73-c000 | 100 |
| ev-tail-73-c001 | 100 |
| ev-tail-75 | 200 |
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
| projection | 1 |
| projection-Cards.Chars.Colors | 1 |
| projection-Cards.Damage | 1 |
| projection-Cards.Flags | 2 |
| projection-Cards.Power | 1 |
| projection-Cards.Zone | 1 |
| projection-NStack | 2 |
| unmodelled | 2 |
| unmodelled-clonelayer | 1 |
| unmodelled-landgrant | 1 |
| unmodelled-tokatkask | 1 |
| unmodelled-tokenpt | 1 |
