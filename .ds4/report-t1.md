# RepeatEach honours RepeatOptionalForEachPlayer$ (rpteachopt1)

## What changed and why

`RepeatEach` ignored `RepeatOptionalForEachPlayer$` / `RepeatOptionalMessage$`
(seven corpus files: Tempt with Vengeance, Tempt with Reflections, Tempt with
Glory, Tempt with Bunnies, Tempt with Mayhem, Tempting Contract, Zagorka,
Mother of Sanctum). It selected its subjects and then ran every subject's body
unconditionally. The brief's root cause was confirmed: no production read of
either parameter existed.

The fix poses one yes/no election per subject before that subject's body runs;
a yes runs the body once, a no skips only that subject and the loop continues.

### `effects/registry.go`
- `RepeatCursor` gains `Election bool` (mark a cursor parked on an election,
  not a completed body).
- New `RepeatEachOptionalContinuation{Next int32; Accept bool}` — the scoped
  answer of one subject's offer, distinct from `RepeatOptionalContinuation`
  (the `Repeat` do/while's own election). The two cursors are never conflated.
- `Ctx.RepeatEachOptional *RepeatEachOptionalContinuation`.

### `effects/choose_control.go`
- `effRepeatEach` reads `RepeatOptionalForEachPlayer$`/`RepeatOptionalMessage$`
  and, in the subject loop, offers subjects that have not been answered yet.
- New `poseRepeatEachElection`: builds the `KChoose` yes/no decision (player =
  `PlayerOf` the subject, prompt = the message, `ResumeKind
  "repeat_each_optional"`, `ResumeSA` = the RepeatEach SA), calls the shared
  `Ask` boundary, and on a suspended ask parks the loop through the EXISTING
  `SuspendRepeat` payload (subjects, `Next`, `Outer`, `Chosen`, `VoteCounts`,
  `Election: true`). R-9: `AskNoHost`/`AskEmpty` returns false and the subject
  is declined, the loop continuing.

### `rules/resolution.go`
- `repeatCursor` gains `election bool`; `SuspendRepeat` threads
  `s.Election` onto the parked loop frame.
- New `resumeResolution` arm `"repeat_each_optional"`: consumes the parked
  loop frame (`rp.outer`, identified by `election`) so the loop is re-entered
  exactly once at the OFFERED subject (not a second time through the outer
  recursion), rebuilds `Ctx.Repeat` + `Ctx.RepeatEachOptional`, and transfers
  the loop frame's accumulated Remembered (`loopRemembered`) and vote tally
  onto the head so the re-entered loop continues from the first pass's
  bindings. `rp.outer = lf.outer` makes the enclosing chain run after.

A body that suspends on its own nested ask is untouched: its existing
`Next+1` cursor resumes into the NEXT subject's election (covered by the
inline-fixture regression).

No `events.Event` field or ordinal changed. All game-state mutation continues
to go through emitted events. No `knownUnsupportedParams` edit (none of the
seven carriers is in the ratchet, re-verified `<empty grep>`); no
Known-approximations row added (the parameter had no row).

## Exact commands and output

`.cards` was PRESENT as a symlink (`ln -sfn /home/sadams/projects/gorge/.cards
.cards`); confirmed by the corpus test RUNNING rather than skipping (verbose
run below).

### Brief's targeted test command (`rules/`)
```
$ go test -run 'TestRepeatEachOptionalForEachPlayer|TestRepeatEachOptionalForEachPlayerSuspendedBody|TestHeads' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.970s
```
Verbose confirmation that the real-corpus test RAN (not skipped):
```
$ go test -v -run 'TestRepeatEachOptionalForEachPlayerMixedAnswers' ./rules/
=== RUN   TestRepeatEachOptionalForEachPlayerMixedAnswers
--- PASS: TestRepeatEachOptionalForEachPlayerMixedAnswers (0.69s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.708s
```

### Edited non-rules packages (run once each)
```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.630s
$ go test ./botpolicy/
ok  	github.com/adams-shaun/gorge/botpolicy	0.644s
```

### Behaviour goldens outside `rules/`
```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.299s
```
No allowlist edits. No bot-split or chain-head movement (TestHeads passed in
the brief's command; `cmd/botbench` split unchanged) — expected, as no repo
deck carries these parameters.

### Format
```
$ gofmt -l effects/choose_control.go effects/registry.go rules/resolution.go \
      rules/repeat_each_optional_test.go effects/repeat_each_optional_test.go \
      effects/context_test.go botpolicy/repeat_each_optional_test.go
gofmt-clean
$ go run ./cmd/gentypes -check
(no output)
$ go build ./...
(no output)
```

## Fails without the fix

Reverted the feature by disabling the per-subject-election branch in
`effects/choose_control.go` (`if optionalForEach {` → `if false &&
optionalForEach {`), ran the one command, then restored the file
byte-identically (`cmp` against `.ds4/scratch/choose_control.go.orig` printed
`RESTORED`). Failing output:

```
--- FAIL: TestRepeatEachOptionalForEachPlayerMixedAnswers (0.59s)
    repeat_each_optional_test.go:131: precondition failed: 2 Elemental Tokens before any answer, want 0
--- FAIL: TestRepeatEachOptionalForEachPlayerDeclinesEverySubject (0.00s)
    repeat_each_optional_test.go:169: decision = kind attackers resume "", want a repeat_each_optional KChoose for player 1: ...
--- FAIL: TestRepeatEachOptionalForEachPlayerAcceptsBoth (0.00s)
    repeat_each_optional_test.go:195: decision = kind attackers resume "", want a repeat_each_optional KChoose for player 1: ...
--- FAIL: TestRepeatEachOptionalForEachPlayerSuspendedBody (0.00s)
    repeat_each_optional_test.go:264: decision = kind arrange resume "arrange", want a repeat_each_optional KChoose for player 1: ...
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.609s
```

The first failure is the cleanest evidence: without the election, both
opponents' bodies run up front (2 Elementals before any answer); the other
three show the loop finishing with no election asked at all.

## New tests (new files, per the no-shared-append rule)

- `rules/repeat_each_optional_test.go` — end-to-end on the real corpus carrier
  Tempt with Vengeance (`RepeatPlayers$ Player.Opponent`, X=1):
  `…MixedAnswers` (opponent 1 declines → 0 tokens, opponent 2 accepts → 1
  token, in loop order, prompt = the script's `RepeatOptionalMessage$`),
  `…DeclinesEverySubject` (both decline → nothing, and asserts no
  `RepeatEach selector unimplemented` Note so the handler provably ran),
  `…AcceptsBoth`, and `…SuspendedBody` (inline fixture whose per-subject body
  Scries; answering the body's `KArrange` must resume into the NEXT subject's
  election). Each asserts its own precondition (zero tokens before any answer;
  the body's `KArrange` really appeared).
- `effects/repeat_each_optional_test.go` — R-9 no-ask decline (asks once per
  opponent, runs no body, parks nothing), the election cursor payload
  (`Election`, `Next`, captured subjects, subject), and re-entry
  accept/decline (accepted subject runs its body once and the next subject is
  asked; a declined subject runs nothing and the loop still advances).
- `botpolicy/repeat_each_optional_test.go` — the distinct
  `repeat_each_optional` shape round-trips `Decision.Validate` on both option
  orders (a binding check that the offered order is read, so the bot cannot
  livelock re-submitting a rejected answer). The shared KChoose fallback
  answers option 0, which is legal and terminates.

The one pre-existing test file touched is `effects/context_test.go`, only to
record `SuspendRepeat` calls on the effects double (its `fakeHost`); no
production `events.Event` change.

## Issues (found, not fixed)

- **Post-loop accumulation still computes 0.** With the election now correct,
  the carriers' post-loop `SubAbility$` accumulation (`Tempting Contract`'s
  `X: PlayerCountRememberedOwner$Amount` then `DBToken TokenAmount$ X`;
  `Tempt with Vengeance`'s `Y` via `DB$ StoreSVar | Type$ CountSVar`) still
  yields 0 because `DB$ StoreSVar` is unregistered, so "for each opponent who
  does, create N more" creates nothing. The election and each opponent's own
  offer are correct; the shared bonus is the adjacent defect. The brief marks
  the StoreSVar issue superseded, so I did not file a ticket.
- **`ChangeZoneTable$ True` is unread** (`effects/`, `rules/`, `cards/` have
  no read; 47 corpus files carry it, all seven `RepeatOptionalForEachPlayer$`
  carriers among them). Not this feature's parameter; listed so it is visible.
- **Non-player loops with `RepeatOptionalForEachPlayer$`** are asked to the
  subject object's controller (`PlayerOf`). No corpus carrier does this
  (all seven are player loops), so the generalization is unmeasured.
- This feature is invisible to `make ledger` (no CR-lane test drives
  `RepeatOptionalForEachPlayer$`). If it should stop being invisible, a
  CR-lane test citing CR 608.2c/601.2 would fit.

## Deviations from the brief

None. The suspending-body regression was added because the continuation
transport DID change (a new resume kind consuming the parked loop frame), as
the brief conditions it.
