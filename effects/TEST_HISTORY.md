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
| 2026-09-18T07:41Z | 1dc6b115+ | 0.5 | 361 | 0 | sadams |
