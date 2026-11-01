# Reproduce a feedback report

A player-submitted bug report that named a table carries a replayable snapshot
(`match.json`, `log.json`, the seat's `view.json`, captured by the server's
feedback capture). Turning it into a failing test is one command:

```sh
go run ./cmd/repro <feedback-dir>              # replay, verify the head, print the board at the
                                               # report point (turn, step, life, battlefield, stack,
                                               # last ~20 log lines)
go run ./cmd/repro -at N <feedback-dir>        # replay to intent N instead (flags may follow the
                                               # directory; -list prints the intent timeline to find N)
go run ./cmd/repro -omniscient <feedback-dir>  # show every hand in the summary
go run ./cmd/repro -emit-test <pkg> <feedback-dir>
                                               # copy the snapshot into <pkg>/testdata/feedback/<id>/
                                               # and write a failing test skeleton (feedback.EngineAt)
                                               # as the target package's EXTERNAL test package
                                               # (package <name>_test: feedback imports the engine
                                               # tier, so an internal-package skeleton would be an
                                               # import cycle); replace the TODO with the assertion
```

Exit 0 is a verified replay: the rebuilt event stream matches the recording
event for event and the final head equals `log.json`'s `head`. A non-zero exit
prints `DIVERGED` naming the first mismatching event. **A corpus change since
the report was filed is a real, expected cause** (the snapshot replays against
the corpus as it was at the report's pin), and so is an engine change; read the
first diverging event before assuming either. In a test, load the same snapshot
with `feedback.EngineAt(t, dir, n)` (`testkit/testutil/feedback`; `n < 0` = every
recorded intent) and assert at the engine it returns.

## Committed fixtures

A committed fixture's `match.json` does not embed token script text: Forge
scripts are GPL-3.0 and must never be committed. `-emit-test` and the fixture
generator strip `match.json`'s `tokens`/`tokens_unread` and sync the exact text
into the gitignored `cmd/repro/testdata/.tokens/<id>/`, keyed by the fixture id.
`feedback.Load` resolves token scripts from that directory first (the exact
historical text) and falls back to the live corpus (`.cards/` at the current
`FORGE_REF`) when it is missing, on a fresh clone, so a fixture still replays
without re-embedding the text.

A capture also records the name-card universe mode (`name_universe`) and the
exact sorted label list that match offered (`name_universe_names`), because a
universe-backed match poses `NameCard` asks the legacy path never poses, and a
replay rebuilt without the mode refuses the recorded name intent. The committed
fixture keeps only the mode bit (the list is about 24k entries) and
`feedback.config` re-derives it from the live corpus, so a `FORGE_REF` move can
renumber a recorded name choice and shows up as the same `DIVERGED`.

`cmd/repro/testdata/feedback/20260914T120000Z-fb01/` is a real capture produced by
`host.SnapshotForFeedback`; regenerate it with
`REPRO_REGEN_FIXTURE=1 go test ./cmd/repro -run TestGenerateCommittedFixture`
after a corpus pin bump (`FORGE_REF`), or the fixture's own gate reports
`DIVERGED` exactly as a stale report would.
