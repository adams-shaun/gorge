# Test history — github.com/adams-shaun/gorge/effects

budget_s: 40

<!--
The rows below restart the baseline at the post-perf-wave reality. Every row
in the file before 2026-09-17T13:06Z (through commit 0af7ed65+, 343 tests,
~17-34s) predates Merge pull request #1 (perf/hotspot-optimization-2026-09-17,
ef86e427), which cut the engine's hotspot overhead by more than an order of
magnitude across the whole tree -- the same-day rules rows show the identical
step (228.5s/1084 tests at 06:42Z -> 6.4s/1249 at 13:03Z). Those rows sat
~60x above the suite's true solo wall, so the wall-collapse predicate refused
every honest post-perf measurement as vacuous: 0.49s/346 tests measured three
times back to back (0.496/0.497/0.491s, `go test -count=1 -p=1 -json`, one
real-corpus test at 0.44s and zero skips -- not a vacuous run) was refused
against the 0.0879 s/test poisoned median. The old rows were dropped rather
than averaged because the median is what the predicate reads and no honest
run can pass against them; the skip-fraction defence (the check that caught
the real missing-corpus incident for rules) is preserved by carrying the
measured skipped=0 forward, and this comment records exactly what was dropped
so the pre-perf calibration is not lost.
-->

| date (UTC) | commit | wall_s | tests | skipped | runner |
|---|---|---|---|---|---|
| 2026-09-17T13:06Z | a8a02818+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T13:06Z | a8a02818+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T13:06Z | a8a02818+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T13:06Z | a8a02818+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T13:12Z | 7d5f3fa6+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T13:23Z | 4c6b8a35+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T13:48Z | 535f897a+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T14:04Z | 66288cf7+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T14:09Z | a7c42e8a+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T19:38Z | e70c6839+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T21:55Z | 277ce60f+ | 0.5 | 346 | 0 | sadams |
| 2026-09-17T22:16Z | 2fa2f1b4+ | 0.5 | 346 | 0 | sadams |
| 2026-09-18T00:48Z | 2b20f1fa+ | 0.5 | 352 | 0 | sadams |
| 2026-09-18T00:56Z | 78204dfc+ | 0.5 | 352 | 0 | sadams |
| 2026-09-18T01:11Z | 1b19011d+ | 0.5 | 354 | 0 | sadams |
| 2026-09-18T01:15Z | a6bda06c+ | 0.5 | 354 | 0 | sadams |
| 2026-09-18T07:28Z | 4a5392ab+ | 0.5 | 360 | 0 | sadams |
| 2026-09-18T07:36Z | 1dc6b115+ | 0.5 | 361 | 0 | sadams |
| 2026-09-18T07:41Z | 1dc6b115+ | 0.5 | 361 | 0 | sadams |
| 2026-09-18T21:53Z | 80ff8312+ | 0.5 | 363 | 0 | sadams |
| 2026-09-18T23:44Z | 84eb6523+ | 0.5 | 363 | 0 | sadams |
| 2026-09-18T23:56Z | 796b1aec+ | 0.5 | 364 | 0 | sadams |
| 2026-09-19T06:49Z | 1c83cd12+ | 0.7 | 375 | 0 | sadams |
| 2026-09-19T06:49Z | 1cacd037+ | 0.7 | 375 | 0 | sadams |
| 2026-09-19T06:58Z | 6d76a982+ | 0.6 | 375 | 0 | sadams |
| 2026-09-19T07:22Z | 3dbf576e+ | 0.6 | 375 | 0 | sadams |
| 2026-09-19T07:30Z | 0562c294+ | 0.6 | 375 | 0 | sadams |
| 2026-09-19T08:07Z | 5b4fbdf4+ | 0.6 | 383 | 0 | sadams |
| 2026-09-19T08:09Z | 453f516d+ | 0.6 | 383 | 0 | sadams |
| 2026-09-19T08:26Z | 6b737baf+ | 0.7 | 383 | 0 | sadams |
| 2026-09-19T07:50Z | 3dbf576e+ | 0.6 | 375 | 0 | sadams |
| 2026-09-19T08:31Z | 114844e4+ | 0.6 | 383 | 0 | sadams |
| 2026-09-19T08:16Z | c6ed5430+ | 0.6 | 375 | 0 | sadams |
| 2026-09-19T08:37Z | 86141222+ | 0.6 | 383 | 0 | sadams |
| 2026-09-19T08:51Z | 714e6c55+ | 0.6 | 383 | 0 | sadams |
| 2026-09-19T08:55Z | 714e6c55+ | 0.6 | 383 | 0 | sadams |
| 2026-09-19T09:17Z | b878c1c1+ | 0.6 | 386 | 0 | sadams |
| 2026-09-19T09:32Z | fa475c0a+ | 0.6 | 391 | 0 | sadams |
| 2026-09-19T09:33Z | fa475c0a+ | 0.6 | 386 | 0 | sadams |
| 2026-09-19T09:51Z | 58aba2e0+ | 0.6 | 391 | 0 | sadams |
| 2026-09-19T10:02Z | 58aba2e0+ | 0.6 | 391 | 0 | sadams |
| 2026-09-19T10:06Z | a64efe95+ | 0.7 | 391 | 0 | sadams |
| 2026-09-19T10:22Z | 0d4d5413+ | 0.6 | 391 | 0 | sadams |
| 2026-09-19T10:42Z | 7b346c4f+ | 0.6 | 391 | 0 | sadams |
| 2026-09-19T10:32Z | e1aacffd+ | 0.6 | 392 | 0 | sadams |
| 2026-09-19T10:41Z | 7124d742+ | 0.7 | 392 | 0 | sadams |
| 2026-09-19T11:04Z | ad577e8b+ | 0.6 | 396 | 0 | sadams |
| 2026-09-19T11:03Z | ad577e8b+ | 0.6 | 392 | 0 | sadams |
| 2026-09-19T11:40Z | 5476ca3a+ | 0.6 | 400 | 0 | sadams |
| 2026-09-19T11:37Z | 5476ca3a+ | 0.6 | 396 | 0 | sadams |
| 2026-09-19T11:47Z | acca5b28+ | 0.6 | 396 | 0 | sadams |
| 2026-09-19T12:01Z | 96dc60a7+ | 0.6 | 406 | 0 | sadams |
| 2026-09-19T11:50Z | ea3fc488+ | 0.6 | 400 | 0 | sadams |
| 2026-09-19T11:51Z | ea3fc488+ | 0.6 | 400 | 0 | sadams |
| 2026-09-19T11:51Z | ea3fc488+ | 0.8 | 400 | 0 | sadams |
| 2026-09-19T12:07Z | 8b968831+ | 0.6 | 406 | 0 | sadams |
| 2026-09-19T12:14Z | a8a4e9ec+ | 0.6 | 412 | 0 | sadams |
| 2026-09-19T12:28Z | 5637c708+ | 0.6 | 416 | 0 | sadams |
| 2026-09-19T12:29Z | 1c193131+ | 0.9 | 412 | 0 | sadams |
| 2026-09-19T12:45Z | 928ce360+ | 0.6 | 416 | 0 | sadams |
| 2026-09-19T12:51Z | f06bbc96+ | 0.6 | 421 | 0 | sadams |
| 2026-09-19T13:08Z | b7d1af69+ | 0.6 | 421 | 0 | sadams |
| 2026-09-19T13:10Z | 6bbd0ddc+ | 0.6 | 421 | 0 | sadams |
| 2026-09-19T13:22Z | c616b336+ | 0.6 | 421 | 0 | sadams |
| 2026-09-19T13:30Z | c616b336+ | 0.6 | 421 | 0 | sadams |
| 2026-09-19T14:11Z | dcc66982+ | 0.6 | 421 | 0 | sadams |
| 2026-09-19T14:12Z | dcc66982+ | 0.6 | 421 | 0 | sadams |
| 2026-09-19T13:50Z | 309c4d5d+ | 0.6 | 426 | 0 | sadams |
| 2026-09-19T13:50Z | 309c4d5d+ | 0.6 | 426 | 0 | sadams |
| 2026-09-19T14:09Z | f2ecf5f3+ | 0.6 | 428 | 0 | sadams |
| 2026-09-19T14:57Z | 0fc47d35+ | 0.6 | 428 | 0 | sadams |
| 2026-09-19T14:59Z | 80ff5680+ | 0.6 | 429 | 0 | sadams |
| 2026-09-19T15:00Z | 89cf0b5e+ | 0.6 | 429 | 0 | sadams |
| 2026-09-19T15:04Z | b6c9d60f+ | 0.6 | 429 | 0 | sadams |
| 2026-09-19T14:44Z | 0fc47d35+ | 0.6 | 433 | 0 | sadams |
| 2026-09-19T14:47Z | 0fc47d35+ | 0.6 | 433 | 0 | sadams |
| 2026-09-19T15:00Z | 0f6eb418+ | 0.6 | 434 | 0 | sadams |
| 2026-09-19T15:12Z | 52de7d9a+ | 0.6 | 435 | 0 | sadams |
| 2026-09-19T15:19Z | 7976fafc+ | 0.6 | 434 | 0 | sadams |
| 2026-09-19T15:20Z | 7976fafc+ | 0.6 | 434 | 0 | sadams |
| 2026-09-19T15:33Z | 737f2021+ | 0.6 | 437 | 0 | sadams |
| 2026-09-19T15:50Z | 36c5a8ae+ | 0.6 | 437 | 0 | sadams |
| 2026-09-19T15:37Z | 737f2021+ | 0.6 | 435 | 0 | sadams |
| 2026-09-19T15:43Z | 29f82200+ | 0.6 | 435 | 0 | sadams |
| 2026-09-19T16:15Z | 716e13b3+ | 0.6 | 440 | 0 | sadams |
| 2026-09-19T16:26Z | 1f883a04+ | 0.6 | 444 | 0 | sadams |
| 2026-09-19T16:54Z | 4a5ed564+ | 0.7 | 448 | 0 | sadams |
| 2026-09-19T16:55Z | 4a5ed564+ | 0.6 | 448 | 0 | sadams |
| 2026-09-19T17:02Z | 694aabe8+ | 0.6 | 444 | 0 | sadams |
| 2026-09-19T17:03Z | 694aabe8+ | 0.7 | 444 | 0 | sadams |
| 2026-09-19T17:04Z | 694aabe8+ | 0.6 | 444 | 0 | sadams |
| 2026-09-19T18:45Z | e81f5a4a+ | 0.6 | 448 | 0 | sadams |
| 2026-09-19T18:47Z | e81f5a4a+ | 0.6 | 448 | 0 | sadams |
| 2026-09-19T19:46Z | fd8ee910+ | 0.7 | 448 | 0 | sadams |
| 2026-09-19T20:33Z | 84234eb6+ | 0.7 | 448 | 0 | sadams |
| 2026-09-19T20:15Z | e0666a0c+ | 0.7 | 448 | 0 | sadams |
| 2026-09-19T20:19Z | e0666a0c+ | 1.4 | 448 | 0 | sadams |
| 2026-09-19T20:22Z | e0666a0c+ | 1.2 | 448 | 0 | sadams |
| 2026-09-19T20:34Z | e0666a0c+ | 0.7 | 448 | 0 | sadams |
| 2026-09-19T20:38Z | e0666a0c+ | 1.2 | 448 | 0 | sadams |
| 2026-09-19T20:43Z | e0666a0c+ | 0.7 | 448 | 0 | sadams |
| 2026-09-19T20:46Z | e0666a0c+ | 0.6 | 448 | 0 | sadams |
| 2026-09-19T20:47Z | e0666a0c+ | 0.6 | 448 | 0 | sadams |
| 2026-09-19T20:07Z | aaa03f41+ | 0.7 | 449 | 0 | sadams |
| 2026-09-19T21:00Z | 437024f5+ | 0.7 | 449 | 0 | sadams |
| 2026-09-19T18:00Z | 690e1363+ | 0.6 | 459 | 0 | sadams |
| 2026-09-19T18:00Z | 690e1363+ | 0.6 | 459 | 0 | sadams |
| 2026-09-19T18:01Z | 690e1363+ | 0.6 | 459 | 0 | sadams |
| 2026-09-19T18:01Z | e8f7c629+ | 0.6 | 460 | 0 | sadams |
| 2026-09-20T06:17Z | c2065108+ | 0.8 | 460 | 0 | sadams |
| 2026-09-20T06:43Z | ebc1fa52+ | 0.8 | 462 | 0 | sadams |
| 2026-09-20T07:00Z | 46ff9ecd+ | 0.9 | 465 | 0 | sadams |
| 2026-09-20T07:47Z | 7dc26571+ | 0.9 | 466 | 0 | sadams |
| 2026-09-20T08:05Z | e10ffadd+ | 0.6 | 466 | 0 | sadams |
| 2026-09-20T07:53Z | f0231353+ | 0.6 | 466 | 0 | sadams |
| 2026-09-20T08:08Z | bea3e4de+ | 0.7 | 466 | 0 | sadams |
| 2026-09-20T08:08Z | 6edbf7ff+ | 0.7 | 466 | 0 | sadams |
| 2026-09-20T08:23Z | 2b075ecb+ | 0.7 | 468 | 0 | sadams |
| 2026-09-20T08:44Z | 36ccf3fd+ | 0.6 | 468 | 0 | sadams |
| 2026-09-20T09:21Z | 5d4c1c55+ | 0.6 | 469 | 0 | sadams |
| 2026-09-20T09:28Z | 5d4c1c55+ | 0.7 | 469 | 0 | sadams |
| 2026-09-20T09:28Z | 5d4c1c55+ | 0.7 | 469 | 0 | sadams |
| 2026-09-20T09:30Z | 5d4c1c55+ | 0.7 | 469 | 0 | sadams |
| 2026-09-20T08:39Z | a6247a01+ | 0.6 | 468 | 0 | sadams |
| 2026-09-20T08:48Z | bf0663b6+ | 0.6 | 468 | 0 | sadams |
| 2026-09-20T08:56Z | 6db7ff5a+ | 0.6 | 468 | 0 | sadams |
| 2026-09-20T08:57Z | 6db7ff5a+ | 0.9 | 468 | 0 | sadams |
| 2026-09-20T08:57Z | 6db7ff5a+ | 0.7 | 468 | 0 | sadams |
| 2026-09-20T08:57Z | 6db7ff5a+ | 0.7 | 468 | 0 | sadams |
| 2026-09-20T09:06Z | a997ec1c+ | 0.6 | 468 | 0 | sadams |
| 2026-09-20T09:07Z | a997ec1c+ | 0.6 | 468 | 0 | sadams |
| 2026-09-20T09:08Z | a997ec1c+ | 1.2 | 468 | 0 | sadams |
| 2026-09-20T08:51Z | 088ffe7e+ | 0.7 | 474 | 0 | sadams |
| 2026-09-20T08:59Z | 6db7ff5a+ | 0.6 | 468 | 0 | sadams |
| 2026-09-20T09:08Z | 5a1d2af0+ | 0.6 | 474 | 0 | sadams |
| 2026-09-20T09:43Z | 88846a33+ | 0.7 | 474 | 0 | sadams |
| 2026-09-20T08:58Z | 6db7ff5a+ | 0.7 | 468 | 0 | sadams |
| 2026-09-20T09:10Z | a342b627+ | 0.6 | 468 | 0 | sadams |
| 2026-09-20T09:17Z | fee123ea+ | 0.9 | 474 | 0 | sadams |
| 2026-09-20T09:38Z | 1dff2eb2+ | 0.6 | 474 | 0 | sadams |
| 2026-09-20T09:41Z | 1dff2eb2+ | 0.7 | 474 | 0 | sadams |
| 2026-09-20T09:44Z | 170427f1+ | 0.6 | 474 | 0 | sadams |
| 2026-09-20T09:29Z | 4e057f33+ | 0.7 | 474 | 0 | sadams |
| 2026-09-20T10:19Z | 387fbeec+ | 0.7 | 475 | 0 | sadams |
| 2026-09-20T10:19Z | 387fbeec+ | 0.7 | 475 | 0 | sadams |
| 2026-09-20T10:06Z | 31622586+ | 0.7 | 475 | 0 | sadams |
| 2026-09-20T09:57Z | eb145f8d+ | 0.7 | 478 | 0 | sadams |
| 2026-09-20T09:57Z | eb145f8d+ | 0.6 | 478 | 0 | sadams |
| 2026-09-20T10:17Z | e3421f90+ | 0.6 | 479 | 0 | sadams |
| 2026-09-20T10:09Z | 31622586+ | 0.7 | 475 | 0 | sadams |
| 2026-09-20T10:23Z | c9413d11+ | 0.6 | 476 | 0 | sadams |
| 2026-09-20T10:34Z | c81c1dfb+ | 0.6 | 480 | 0 | sadams |
| 2026-09-20T10:41Z | 14a2d60f+ | 0.7 | 481 | 0 | sadams |
| 2026-09-20T10:42Z | 14a2d60f+ | 0.6 | 481 | 0 | sadams |
| 2026-09-20T11:06Z | 4f14efec+ | 0.7 | 481 | 0 | sadams |
| 2026-09-20T11:06Z | 4f14efec+ | 0.7 | 481 | 0 | sadams |
| 2026-09-20T11:10Z | 4f14efec+ | 0.6 | 481 | 0 | sadams |
| 2026-09-20T10:59Z | 146e84f6+ | 0.7 | 481 | 0 | sadams |
| 2026-09-20T10:21Z | 31622586+ | 0.6 | 475 | 0 | sadams |
| 2026-09-20T10:58Z | cac74854+ | 0.6 | 475 | 0 | sadams |
| 2026-09-20T11:36Z | 3dfef229+ | 0.6 | 481 | 0 | sadams |
| 2026-09-20T12:00Z | e221d869+ | 0.7 | 486 | 0 | sadams |
| 2026-09-20T12:22Z | d51ea8eb+ | 0.9 | 487 | 0 | sadams |
| 2026-09-20T12:50Z | ac1de794+ | 0.7 | 487 | 0 | sadams |
| 2026-09-20T11:46Z | e221d869+ | 0.6 | 481 | 0 | sadams |
| 2026-09-20T11:49Z | e221d869+ | 0.6 | 490 | 0 | sadams |
| 2026-09-20T12:02Z | aa8ecfc2+ | 0.6 | 481 | 0 | sadams |
| 2026-09-20T12:07Z | e1743fda+ | 0.7 | 494 | 0 | sadams |
| 2026-09-20T12:02Z | aa8ecfc2+ | 0.6 | 481 | 0 | sadams |
| 2026-09-20T12:20Z | 0225cf09+ | 0.6 | 496 | 0 | sadams |
| 2026-09-20T12:41Z | 2d53362e+ | 0.7 | 496 | 0 | sadams |
| 2026-09-20T12:18Z | 1d850ce7+ | 0.7 | 490 | 0 | sadams |
| 2026-09-20T11:59Z | aa8ecfc2+ | 0.6 | 481 | 0 | sadams |
| 2026-09-20T12:48Z | 3bb592b2+ | 0.7 | 496 | 0 | sadams |
| 2026-09-20T12:46Z | 236c4c22+ | 0.7 | 496 | 0 | sadams |
| 2026-09-20T12:44Z | 0eb29b94+ | 0.7 | 504 | 0 | sadams |
| 2026-09-20T13:05Z | 1982a9a6+ | 0.6 | 510 | 0 | sadams |
| 2026-09-20T13:08Z | c31e261c+ | 0.6 | 504 | 0 | sadams |
| 2026-09-20T13:02Z | c31e261c+ | 0.6 | 505 | 0 | sadams |
| 2026-09-20T13:13Z | bfebb1a3+ | 0.8 | 511 | 0 | sadams |
| 2026-09-20T13:11Z | 323aae64+ | 0.7 | 504 | 0 | sadams |
| 2026-09-20T13:31Z | c8aa3e5a+ | 0.6 | 508 | 0 | sadams |
| 2026-09-20T13:02Z | 323aae64+ | 0.9 | 508 | 0 | sadams |
| 2026-09-20T13:13Z | 48fa3f59+ | 0.7 | 509 | 0 | sadams |
| 2026-09-20T14:16Z | 9544b0d2+ | 0.8 | 522 | 0 | sadams |
| 2026-09-20T13:52Z | 176f30d7+ | 0.6 | 518 | 0 | sadams |
| 2026-09-20T14:28Z | 4461e0fe+ | 0.7 | 524 | 0 | sadams |
| 2026-09-20T13:49Z | 64aa08e4+ | 0.6 | 526 | 0 | sadams |
| 2026-09-20T14:06Z | 356d3cc5+ | 0.7 | 526 | 0 | sadams |
| 2026-09-20T14:39Z | 3c6b9aa1+ | 0.6 | 530 | 0 | sadams |
