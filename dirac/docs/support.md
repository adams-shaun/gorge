# Card support

Generated: `DIRAC_UPDATE_SUPPORT=1 go test -run TestSupportDocIsCurrent ./dirac/kgen` rewrites this page.

A card is **supported** when kgen lowers it, it is not refused by name, and it has evidence: clean lockstep games, listed in `dirac/support/evidence`. A card with no evidence is **unverified**, not presumed clean. See the [glossary](glossary.md).

`cardset.EnforceEvidence` is false, so unverified cards still play.

| list | cards | lowered | refused | unverified | supported |
|---|---:|---:|---:|---:|---:|
| BIG | 30 | 23 | 0 | 23 | 0 |
| BLB | 278 | 253 | 0 | 165 | 88 |
| DFT | 276 | 242 | 0 | 226 | 16 |
| DSK | 276 | 230 | 0 | 225 | 5 |
| ECL | 273 | 230 | 0 | 223 | 7 |
| EOE | 266 | 236 | 0 | 230 | 6 |
| FDN | 517 | 505 | 0 | 243 | 262 |
| FIN | 312 | 264 | 1 | 258 | 5 |
| FRA | 269 | 240 | 0 | 235 | 5 |
| HOB | 172 | 151 | 0 | 146 | 5 |
| LCI | 290 | 244 | 0 | 236 | 8 |
| MKM | 279 | 227 | 0 | 221 | 6 |
| MSH | 276 | 221 | 0 | 215 | 6 |
| OTJ | 276 | 235 | 0 | 228 | 7 |
| SOS | 270 | 246 | 0 | 240 | 6 |
| SPM | 192 | 158 | 0 | 153 | 5 |
| TDM | 276 | 242 | 0 | 225 | 17 |
| TLA | 285 | 242 | 0 | 237 | 5 |
| TMT | 194 | 167 | 0 | 161 | 6 |
| WOE | 281 | 250 | 0 | 241 | 9 |
| ALL | 34072 | 20731 | 2 | 20386 | 343 |

`ALL` is the whole corpus; the rest are the Standard sets.

## Refused by name

| class | card | what differs |
|---|---|---|
| capacity | Jumbo Cactuar | its +9999 end-of-turn pump needs more than the int8 pump field holds |
| projection | Prowling Geistcatcher | gorge puts the +1/+1 counter when the sacrificed creature was a token; the kernel does not |
