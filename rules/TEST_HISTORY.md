# Test history — github.com/adams-shaun/gorge/rules

budget_s: 15

<!--
A row far below the others is very likely VACUOUS, not fast. MEASURED
2026-09-08 in .worktrees/fx14, same commit, same tool, back to back:

    with    .cards:  rules 13.5s 446 tests
    without .cards:  rules  3.0s 446 tests

The corpus-backed tests SKIP when the .cards symlink is absent, which is
the normal state of a freshly created worktree until someone links it.
The wall time collapses to under a quarter and the test count does not
move at all, because cmd/testtime counts a package's top-level `run`
events and a test that skips is a test that ran. So this table's own
tripwire cannot see the difference, and neither can the budget check:
a vacuous run is the one that always passes.

Every 2.1-4.8s row here predates the corpus tests and is honest -- the
count column is the tell: those runs are at 208-300 tests, before the
corpus lane existed.

The 2026-09-08 rows at 445-446 tests are a different animal. Four were
vacuous. The one for 2ea102f+ was re-measured with the corpus present
and corrected in place (3.2 -> 13.5). The other two, written from the
fx13, fx11 and fx15 worktrees, were REMOVED rather
than corrected: re-measuring a superseded commit would have meant
inventing a number, and a run that skipped the tests it was timing is
not a slow measurement or a fast one, it is not a measurement.

Read any sub-5s row at 445+ tests as "the corpus was missing", not as a
speedup. Link the worktree before trusting a row it wrote:

    ln -sfn /home/sadams/projects/gorge/.cards .worktrees/<id>/.cards
-->

| date (UTC) | commit | wall_s | tests | runner |
|---|---|---|---|---|
| 2026-09-05T19:58Z | 0d6d403+ | 20.3 | 208 | sadams |
| 2026-09-05T19:59Z | 0d6d403+ | 20.0 | 208 | sadams |
| 2026-09-05T21:44Z | bef1cda+ | 2.4 | 212 | sadams |
| 2026-09-05T21:38Z | bef1cda+ | 2.4 | 209 | sadams |
| 2026-09-05T21:41Z | bef1cda+ | 2.4 | 210 | sadams |
| 2026-09-05T21:46Z | 96e4d34+ | 4.8 | 211 | sadams |
| 2026-09-05T21:56Z | d2e2bf4+ | 2.6 | 215 | sadams |
| 2026-09-05T22:03Z | 8badd31+ | 2.5 | 217 | sadams |
| 2026-09-05T21:53Z | 26831a7+ | 2.4 | 214 | sadams |
| 2026-09-05T22:16Z | ee0df5a+ | 2.4 | 214 | sadams |
| 2026-09-05T22:17Z | d5c6dea+ | 2.6 | 220 | sadams |
| 2026-09-05T22:18Z | e42b3b6+ | 2.6 | 220 | sadams |
| 2026-09-05T23:26Z | bbe6d47+ | 2.4 | 224 | sadams |
| 2026-09-05T22:30Z | bbe6d47+ | 2.6 | 222 | sadams |
| 2026-09-05T22:46Z | a70ff75+ | 2.7 | 226 | sadams |
| 2026-09-05T22:57Z | 6d2f5dd+ | 2.5 | 226 | sadams |
| 2026-09-05T23:04Z | 4a74fc2+ | 2.6 | 232 | sadams |
| 2026-09-05T23:05Z | 6010378+ | 2.5 | 226 | sadams |
| 2026-09-05T23:17Z | 23fa3c2+ | 5.8 | 234 | sadams |
| 2026-09-05T23:18Z | ee18239+ | 2.2 | 234 | sadams |
| 2026-09-05T23:42Z | 8714de3+ | 2.5 | 238 | sadams |
| 2026-09-05T23:52Z | 8714de3+ | 2.3 | 238 | sadams |
| 2026-09-06T00:06Z | e9e419e+ | 2.2 | 240 | sadams |
| 2026-09-06T00:45Z | c15a15f+ | 2.3 | 242 | sadams |
| 2026-09-06T01:15Z | d8c7dd8+ | 2.7 | 245 | sadams |
| 2026-09-06T01:28Z | 112cfcc+ | 2.3 | 250 | sadams |
| 2026-09-06T02:11Z | 1e06ac5+ | 2.1 | 252 | sadams |
| 2026-09-06T03:13Z | 27cd111+ | 2.3 | 256 | sadams |
| 2026-09-06T03:49Z | 10b2f79+ | 2.1 | 262 | sadams |
| 2026-09-06T04:48Z | f10b1e9+ | 4.4 | 262 | sadams |
| 2026-09-06T05:23Z | 5c24cef+ | 4.8 | 262 | sadams |
| 2026-09-06T07:33Z | e6410d6+ | 2.3 | 260 | sadams |
| 2026-09-06T08:04Z | 38b6320+ | 3.8 | 260 | sadams |
| 2026-09-06T08:40Z | 5511925+ | 6.2 | 260 | sadams |
| 2026-09-06T10:38Z | 336e2cd+ | 4.3 | 267 | sadams |
| 2026-09-06T10:45Z | e7891a3+ | 4.8 | 260 | sadams |
| 2026-09-06T15:00Z | 36be378+ | 4.3 | 269 | sadams |
| 2026-09-06T17:00Z | c33c8b7+ | 4.6 | 271 | sadams |
| 2026-09-06T17:46Z | 0f70148+ | 3.2 | 273 | sadams |
| 2026-09-06T17:51Z | 0f70148+ | 4.0 | 271 | sadams |
| 2026-09-06T18:15Z | d3c8e6e+ | 3.1 | 274 | sadams |
| 2026-09-06T18:28Z | d3c8e6e+ | 2.8 | 275 | sadams |
| 2026-09-06T19:09Z | 63fc1ab+ | 2.2 | 278 | sadams |
| 2026-09-06T19:17Z | 44d9781+ | 2.2 | 280 | sadams |
| 2026-09-06T19:28Z | 81488bd+ | 2.2 | 280 | sadams |
| 2026-09-06T19:28Z | 81488bd+ | 2.4 | 276 | sadams |
| 2026-09-06T19:28Z | d07bddf+ | 2.3 | 278 | sadams |
| 2026-09-06T22:38Z | 81892e6+ | 2.2 | 288 | sadams |
| 2026-09-06T22:50Z | 8d467a7+ | 2.1 | 293 | sadams |
| 2026-09-06T22:44Z | 8d467a7+ | 4.2 | 290 | sadams |
| 2026-09-06T22:45Z | 8d467a7+ | 4.3 | 290 | sadams |
| 2026-09-06T22:45Z | 8d467a7+ | 4.1 | 290 | sadams |
| 2026-09-06T22:47Z | 8d467a7+ | 2.1 | 290 | sadams |
| 2026-09-06T23:13Z | c20da91+ | 2.1 | 295 | sadams |
| 2026-09-06T23:21Z | 871333b+ | 2.3 | 304 | sadams |
| 2026-09-06T23:30Z | f9354cb+ | 2.2 | 311 | sadams |
| 2026-09-06T23:55Z | 16660fc+ | 4.4 | 318 | sadams |
| 2026-09-06T23:56Z | 16660fc+ | 2.1 | 318 | sadams |
| 2026-09-07T00:15Z | 111321e+ | 4.4 | 305 | sadams |
| 2026-09-07T00:19Z | 111321e+ | 9.7 | 305 | sadams |
| 2026-09-07T00:33Z | 11fc153+ | 1.7 | 321 | sadams |
| 2026-09-07T00:34Z | fcea152+ | 1.7 | 321 | sadams |
| 2026-09-07T01:15Z | 613e9ae+ | 4.1 | 322 | sadams |
| 2026-09-07T01:17Z | 613e9ae+ | 4.3 | 322 | sadams |
| 2026-09-07T01:17Z | 613e9ae+ | 1.8 | 322 | sadams |
| 2026-09-07T02:05Z | 4549f9a+ | 4.4 | 322 | sadams |
| 2026-09-07T02:00Z | 34c8d11+ | 1.8 | 322 | sadams |
| 2026-09-07T01:50Z | bf10594+ | 4.2 | 322 | sadams |
| 2026-09-07T02:07Z | bf10594+ | 4.4 | 322 | sadams |
| 2026-09-07T02:11Z | b22c3d2 | 4.2 | 322 | sadams |
| 2026-09-07T02:12Z | b22c3d2+ | 4.2 | 322 | sadams |
| 2026-09-07T03:18Z | 8e8bf69+ | 10.2 | 327 | sadams |
| 2026-09-07T03:19Z | 8e8bf69+ | 5.0 | 327 | sadams |
| 2026-09-07T03:28Z | e89df75+ | 1.7 | 328 | sadams |
| 2026-09-07T03:17Z | 8e8bf69+ | 5.6 | 322 | sadams |
| 2026-09-07T03:31Z | 5fc7bb2+ | 1.8 | 322 | sadams |
| 2026-09-07T05:37Z | cdd21d7+ | 4.3 | 328 | sadams |
| 2026-09-07T05:50Z | f162257+ | 4.4 | 328 | sadams |
| 2026-09-07T05:51Z | 0e066ea+ | 1.8 | 328 | sadams |
| 2026-09-07T06:35Z | a767bf9+ | 1.3 | 328 | sadams |
| 2026-09-07T09:24Z | a6c7851+ | 2.0 | 329 | sadams |
| 2026-09-07T10:38Z | 63eb3ab+ | 3.9 | 329 | sadams |
| 2026-09-07T11:44Z | 587f31d+ | 5.0 | 329 | sadams |
| 2026-09-07T11:43Z | 587f31d+ | 1.6 | 337 | sadams |
| 2026-09-07T11:56Z | 7f300ba+ | 1.5 | 338 | sadams |
| 2026-09-07T12:05Z | 0751802+ | 5.2 | 338 | sadams |
| 2026-09-07T10:46Z | 587f31d+ | 1.5 | 330 | sadams |
| 2026-09-07T11:49Z | e4bfbb9+ | 1.8 | 332 | sadams |
| 2026-09-07T12:09Z | 54f8430+ | 5.1 | 341 | sadams |
| 2026-09-07T11:49Z | 587f31d+ | 1.6 | 336 | sadams |
| 2026-09-07T12:11Z | 60c0b4b+ | 5.0 | 348 | sadams |
| 2026-09-07T11:39Z | 587f31d+ | 1.4 | 331 | sadams |
| 2026-09-07T12:03Z | 3cfab11+ | 1.5 | 333 | sadams |
| 2026-09-07T17:54Z | 69cdced+ | 5.1 | 352 | sadams |
| 2026-09-07T17:57Z | 5f50cde+ | 5.9 | 353 | sadams |
| 2026-09-07T11:47Z | fce341a+ | 2.5 | 329 | sadams |
| 2026-09-07T17:47Z | 69cdced+ | 2.5 | 352 | sadams |
| 2026-09-07T18:31Z | 89f2743+ | 5.9 | 357 | sadams |
| 2026-09-07T18:02Z | 5f50cde+ | 2.2 | 354 | sadams |
| 2026-09-07T18:39Z | 8f153c0+ | 2.5 | 354 | sadams |
| 2026-09-07T20:51Z | 63ee22d+ | 7.2 | 359 | sadams |
| 2026-09-07T21:00Z | 63ee22d+ | 6.3 | 359 | sadams |
| 2026-09-07T21:02Z | 63ee22d+ | 2.5 | 357 | sadams |
| 2026-09-07T21:29Z | 5549486+ | 6.4 | 361 | sadams |
| 2026-09-07T21:28Z | 5549486+ | 2.3 | 361 | sadams |
| 2026-09-07T22:47Z | 97ced63+ | 2.5 | 363 | sadams |
| 2026-09-07T22:47Z | 97ced63+ | 2.8 | 365 | sadams |
| 2026-09-07T23:29Z | bb61a7b+ | 7.7 | 367 | sadams |
| 2026-09-07T23:44Z | a928ee1+ | 7.6 | 367 | sadams |
| 2026-09-08T00:09Z | 2d19918+ | 2.6 | 368 | sadams |
| 2026-09-08T00:21Z | 276397d+ | 7.2 | 368 | sadams |
| 2026-09-08T00:23Z | 276397d+ | 7.1 | 368 | sadams |
| 2026-09-08T00:23Z | 276397d+ | 7.2 | 368 | sadams |
| 2026-09-08T00:23Z | 276397d+ | 7.2 | 368 | sadams |
| 2026-09-08T00:23Z | 276397d+ | 7.2 | 368 | sadams |
| 2026-09-08T00:24Z | 276397d+ | 7.1 | 368 | sadams |
| 2026-09-08T00:21Z | 2d19918+ | 2.3 | 369 | sadams |
| 2026-09-08T00:36Z | 2bb7836+ | 7.3 | 370 | sadams |
| 2026-09-07T23:12Z | 2080703+ | 2.3 | 365 | sadams |
| 2026-09-08T00:02Z | c5bb585+ | 7.7 | 374 | sadams |
| 2026-09-08T00:03Z | c5bb585+ | 2.7 | 374 | sadams |
| 2026-09-08T00:03Z | ad87370+ | 2.5 | 374 | sadams |
| 2026-09-08T00:39Z | 94d125c+ | 7.5 | 377 | sadams |
| 2026-09-07T23:43Z | bb61a7b+ | 8.4 | 367 | sadams |
| 2026-09-07T23:45Z | bb61a7b+ | 2.4 | 367 | sadams |
| 2026-09-08T00:19Z | b5f69bb+ | 2.4 | 368 | sadams |
| 2026-09-08T00:41Z | e8b0e7a+ | 8.1 | 378 | sadams |
| 2026-09-08T00:45Z | e3fcd5a+ | 7.3 | 381 | sadams |
| 2026-09-07T23:52Z | a928ee1+ | 2.4 | 371 | sadams |
| 2026-09-08T00:49Z | 7d9f749+ | 7.8 | 386 | sadams |
| 2026-09-08T01:22Z | 71a03cc+ | 2.4 | 393 | sadams |
| 2026-09-08T01:27Z | 73b1b3e+ | 7.8 | 393 | sadams |
| 2026-09-08T02:00Z | 19d28cc+ | 2.2 | 400 | sadams |
| 2026-09-08T02:03Z | 19d28cc+ | 8.5 | 400 | sadams |
| 2026-09-08T02:27Z | daaffdf+ | 2.3 | 414 | sadams |
| 2026-09-08T03:11Z | 3e931c0+ | 2.3 | 422 | sadams |
| 2026-09-08T03:41Z | 0ab3b42+ | 2.3 | 429 | sadams |
| 2026-09-08T04:04Z | 3056b5b+ | 2.2 | 439 | sadams |
| 2026-09-08T04:28Z | ad05bd1+ | 2.3 | 445 | sadams |
| 2026-09-08T05:22Z | df46edf+ | 2.3 | 445 | sadams |
| 2026-09-08T15:06Z | 70aa2c0+ | 8.5 | 445 | sadams |
| 2026-09-08T05:17Z | df46edf+ | 2.9 | 445 | sadams |
| 2026-09-08T15:11Z | f494c09+ | 2.8 | 445 | sadams |
| 2026-09-08T15:12Z | 3ab34e6+ | 9.2 | 445 | sadams |
| 2026-09-08T15:14Z | 729c351+ | 9.3 | 445 | sadams |
| 2026-09-08T15:36Z | 3d12ec9+ | 3.9 | 445 | sadams |
| 2026-09-08T15:38Z | 3d12ec9+ | 2.9 | 445 | sadams |
| 2026-09-08T15:45Z | 595bc6f+ | 10.1 | 445 | sadams |
| 2026-09-08T15:46Z | 595bc6f+ | 9.7 | 445 | sadams |
| 2026-09-08T16:58Z | 7562ad0+ | 2.8 | 445 | sadams |
| 2026-09-08T17:02Z | 7562ad0+ | 2.9 | 445 | sadams |
| 2026-09-08T17:14Z | 085ce32+ | 2.7 | 445 | sadams |
| 2026-09-08T17:16Z | 085ce32+ | 2.8 | 445 | sadams |
| 2026-09-08T18:08Z | 88ea57a+ | 2.9 | 445 | sadams |
| 2026-09-08T18:13Z | 7fb142c+ | 11.1 | 445 | sadams |
| 2026-09-08T18:13Z | 7fb142c+ | 10.9 | 445 | sadams |
| 2026-09-08T19:02Z | 7fb142c+ | 10.8 | 445 | sadams |
| 2026-09-08T19:09Z | 25374ab+ | 11.9 | 445 | sadams |
| 2026-09-08T20:47Z | 9dc1d2e+ | 12.6 | 445 | sadams |
| 2026-09-08T22:02Z | 11311e9+ | 12.0 | 445 | sadams |
| 2026-09-08T22:22Z | 2ea102f+ | 13.5 | 446 | sadams |
| 2026-09-08T22:49Z | 4669ff9+ | 13.6 | 446 | sadams |
| 2026-09-08T23:00Z | 25d61bf+ | 3.2 | 447 | sadams |
| 2026-09-08T22:58Z | 25d61bf+ | 3.2 | 446 | sadams |
