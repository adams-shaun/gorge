# Rules guards inside gorge

These checks run in the gorge repository. None needs another engine. Each catches
a different kind of change. The failure meanings are in the table on
[Testing](../contributing/testing.md#ratchets-and-goldens-what-a-failure-means).

## Ratchets

A ratchet is a test whose expected value is a committed table, and which fails in
both directions. A new gap is a regression. A listed gap that your change closed is
a stale entry, and you delete it in the same commit. A branch never adds an entry.

| Ratchet | Guards | Table |
|---|---|---|
| `TestEveryRepoDeckIsFullySupported` | every card in the repo decks needs only registered primitives | `engine/rules/testdata/known-unsupported/`, one file per card |
| `TestEveryRepoDeckParamsAreRead` | every script parameter on those cards is read by code | `engine/rules/testdata/paramcensus/` |
| `TestEveryStandardSetParamsAreRead` | the same, over the Standard sets | `engine/rules/testdata/paramcensus-sets/` |
| `TestKnownApproximationsOnlyShrinks` | the register of known approximations | `docs/agents/known-approximations/ALLOWED.txt` |

A primitive can be registered and still ignore a parameter, so the parameter
census scans the engine's source for every read of a card's parameters, and
attributes each to the primitive that reads it. A read it cannot attribute fails
the scan, so a consumer cannot hide by renaming a map. The census is derived from
code, not hand-listed.

The repo-deck acceptance tests in `engine/rules/acceptance_test.go` also play the
decks (`TestRepoDecksPlayAtEverySeatCount`) and replay each finished game from
its log (`TestRepoDeckGamesReplayExactly`). A replay that diverges means something
mutated state outside `events.Apply`, or something nondeterministic reached an
event.

## Golden heads and replays

The engine is a pure function of its config and intents, so a game's event log
hashes to a chain head. Golden heads pin those.

- `TestHeads`: deterministic bot games over the Legacy repo decks at 2, 4, 6 and
  8 seats, goldens in `engine/rules/testdata/heads/`.
- `TestSetHeads`: small two-seat games over decks from the Standard sets, chosen to
  play mechanics the first family leaves unplayed. Games and goldens are under
  `engine/rules/testdata/setheads/`.
- `make sim` plays verified four-seat games and replays each.

A changed head means a change altered what happens. If intended, re-pin with the
environment variable named in the test and say in the commit message which event
first differs and why. `cmd/headdiff` finds that event. See
[Invariants](../architecture/invariants.md) for the determinism rules behind this.

## CR coverage tables

`engine/rules/crcoverage/` holds one file per Comprehensive Rules chapter, one row
per rule: a status (`follows`, `partial`, `deviates`, `not-impl`, `definition`,
`n/a` or `unknown`), the code that decides it, the tests, and notes.
`TestCRCoverageIsCurrent` fails when a row stops matching the code, so a change
that alters how a rule is handled updates its row in the same commit. The README
in that directory has the current counts. dirac keeps its own table in
`dirac/crcoverage/`.

`make conformance` runs `go test ./engine/rules -run TestCR -v`: the focused
audit of casting (CR 601), activating, triggering, resolving, continuous and
replacement effects, state-based actions and combat, with one file per leaf.

## Known approximations

`docs/agents/known-approximations/` lists the places where the engine still stands
in for a choice or rule it cannot yet model, one page per approximation. It is a
closing register: `ALLOWED.txt` holds one line per remaining page with a byte cap,
and lines can only be deleted or have the cap lowered.

## cardfuzz

`cmd/cardfuzz` plays bot-versus-bot games between random 60-card mono-colour decks
drawn from the whole supported corpus, to find panics, livelocks, rejected
submissions and replay divergences that curated decks never reach. Sampling
favours the cards the accumulated coverage state has seen least, so repeated runs
walk the whole pool. Every finished game is replayed and compared (`-verify`).

Each failure is one JSON line with both decklists, the seed and the diagnostic.
`cardfuzz -repro FILE -line N` replays it. Other lanes: `-autopay` for the
payment-plan wrapper, `-manabrew` to send every decision through the ManaBrew
wire, and `-spellbench twin|outcome` to compare a bot's native play with the
SpellBench v2 world. A found bug becomes a unit test in the same directory.

## From a player's report

`go run ./cmd/repro <feedback-dir>` replays a bug report's snapshot, verifies the
hash chain and prints the board at the report point. `-emit-test <pkg>` writes a
failing test skeleton. See
[`docs/agents/repro.md`](https://github.com/adams-shaun/gorge/blob/main/docs/agents/repro.md).

## What runs where

| When | What |
|---|---|
| While you work | Focused tests with `-run`, capped, with the corpus linked |
| Before a PR | `TestHeads`, `make sim`, the guards for the files you touched |
| After merge and in CI | The full suite, with the per-test budget (4 GB, 4 vCPU, 1 minute) |
| Nightly in gorge-compliance | Verdict, ratchet and drift reports |

Never run `go test ./...` as an inner loop. Missing `.cards` makes corpus tests
skip and the package still prints `ok`, so check with `-v`.
