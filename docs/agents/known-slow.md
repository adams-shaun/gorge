# Known slow tests

Top-level tests that run over the one-minute budget (4 cores, `docs/site/contributing/testing.md`).
`scripts/test_targets.py` fails a run (BUDGET, exit 5) when a test **the diff adds or changes** is over
60 s; a test it leaves alone is only reported with a `note:` line. The tests listed here are the ones already
over the budget. This list only shrinks: a row leaves when its test is fixed, and
`test_known_slow_list_only_shrinks` in `scripts/test_targets_test.py` fails on a row it does not pin.
Never add a row: split the new test instead.

Owner action, one per row (the first word of the last column):

- `split`: cut it into independent chunk tests that share fixtures; never cut coverage.
- `shard`: run it as N `-run`-selectable shards, as `TestKernelWorldsInFuzzGame5Shard0` and `Shard1` are.
- `nightly`: tag it so only the nightly lane runs it.

| Package | Test | Median s | Owner action |
|---|---|---|---|

## Seeding

The 13 tests with a median over 60 s come from the precheck audit's slowest-50 table (AUDIT.md), which is not
in this repository, and `testkit/testutil/testdata/wall_exceptions.txt` is still `!unseeded`. Until a seat with
that table (or a post-merge `testbudget check` run) fills the rows, the table is empty; fill it and the pin
`KNOWN_SLOW_PINS` in `scripts/test_targets_test.py` together, once.
