# Known slow tests

Top-level tests that run over the one-minute budget (4 cores, `docs/site/contributing/testing.md`).
`scripts/test_targets.py` fails a run (BUDGET, exit 5) when a test **the diff adds or changes** is over
60 s; a test it leaves alone is only reported with a `note:` line. The tests listed here are the ones already
over the budget. This list only shrinks: a row leaves when its test is fixed, and
`test_known_slow_list_only_shrinks` in `scripts/test_targets_budget_test.py` fails on a row it does not pin.
Never add a row: split the new test instead. (The seed below is the one exception.)

Owner action, one per row (the first word of the last column):

- `split`: cut it into independent chunk tests that share fixtures; never cut coverage.
- `shard`: run it as N `-run`-selectable shards, as `TestKernelWorldsInFuzzGame5Shard0` and `Shard1` are.
- `nightly`: tag it so only the nightly lane runs it.

| Package | Test | Median s | Owner action |
|---|---|---|---|
| dirac/oracle | TestGraveReduceLockstep | 157.3 | nightly |
| dirac/oracle | TestGrantSearchPainlandFundingLockstep | 117.4 | nightly |
| engine/rules | TestKernelWorldsInFuzzGame2 | 117.4 | split |
| dirac/kgen | TestTableBuildDeterministic | 91.3 | split |
| dirac/oracle | TestCountheadsLockstep | 89.2 | nightly |
| dirac/oracle | TestFiredmgLockstep | 81.1 | nightly |
| dirac/oracle | TestCascadeLockstep | 77.4 | nightly |
| dirac/oracle | TestMBCTheWorldTreeReprosLockstep | 72.1 | shard |
| dirac/oracle | TestCopyTokenTappedLockstep | 70.8 | nightly |
| dirac/oracle | TestEclblightLockstep | 69.8 | nightly |
| dirac/oracle | TestDividedXTotalClassLockstep | 68.5 | nightly |
| engine/rules | TestKernelWorldsInFuzzGame5Shard1 | 65.5 | shard |
| dirac/oracle | TestLeftzoneMutation | 63.6 | nightly |

## Seeding

The 13 rows above are the seed: the tests with a median over 60 s in the precheck audit's slowest-50 table
(400 newest prechecks, 11-16..11-20, `AUDIT.md`). Their pins are in `KNOWN_SLOW_PINS` in
`scripts/test_targets_budget_test.py`, and `testkit/testutil/testdata/wall_exceptions.txt` stays `!unseeded`
until the post-merge `testbudget check` run seeds it. From here the table and the pins only shrink together:
delete a row and its pin when the test fits, never add one.
