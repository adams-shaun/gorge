# Report — fb-20260923T015847Z-fad49275 (Gitaxian Probe "never showed me opponent's hand")

Verification-only ticket. **No production code changed, no test added, no commit
made** (the working tree is clean; there is nothing to commit).

## Conclusion

**The original symptom cannot be reproduced on current main. It was already
fixed by `868d7c6b` ("fix(effects): RevealHand reveals the whole hand when the
script carries no NumCards$"), merged as `c7f54854`.** Both commits are in this
worktree's history:

```
$ git show --stat 868d7c6b | head -3
commit 868d7c6b6bae8d16ca30ce3f97045d3d7663ac44
    fix(effects): RevealHand reveals the whole hand when the script carries no NumCards$

$ git show --stat c7f54854 | head -2
commit c7f54854215962188fcc9ef9f2ffb1f70e3e8a4e
Merge: 5f3c3a67 ffcb67c0
    merge(fb-20260914T063255Z-ce14a643): gitaxian probe -- prompt should display user's hand to me, but it did no
```

The brief's premise held exactly: the choke point at `effects/cardflow.go`
still reads `wholeHand := sa.API == "RevealHand" && !hasNum` (line 2072), and a
`RevealHand` SA with no `NumCards$` reveals the whole eligible hand
(lines 2239/2263). No re-derivation was needed.

## What I inspected (consumer path)

The brief asks whether a *distinct consumer* of the look event can still fail
to display the transcript. I traced the player-facing path:

- `host/fanout.go:97 eventBodiesFor(viewer, vis, g, evs)` — the one function
  every seat/spectator event body is built from.
- It calls `view.RedactEventFor` then `view.Describe` on the **redacted** event
  and wraps them in `protocol.EventBody{Event, Line}`. There is **no
  look-specific code in `host`** — delivery is a thin delegation to the two
  `view` functions the existing probe tests cover.
- Callers (`fanout.go` broadcast/tail, `undo.go:415`, `viewat.go:123` Events,
  `viewat.go:151` EventsSeat) all go through that single function, so seat and
  spectator bodies "can only drift through that (viewer, vis) pair"
  (the function's own doc).
- The web client renders `e.line` verbatim (`web/src/lib/logrender.ts`); grep
  for `looks at`/`hidden cards` in `web/src` returns nothing, i.e. the client
  adds no separate look handling that could drop or mis-scope the line.

Because the consumer is a pure delegation to `view.RedactEventFor` +
`view.Describe`, the existing real-corpus view tests already cover the
player-facing behavior end to end. There is no distinct, reproducible gap to
add a focused test for, so per the brief I did not duplicate tests.

## Gate commands run (real output)

Focused effect tests named in Done means:

```
$ go test -run 'TestGitaxianProbeLookIsAPrivateLookScopedToTheActivator|TestThoughtKnotSeerRevealHandRevealsTheWholeHand' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.620s
```

Focused view tests named in Done means:

```
$ go test -run 'TestGitaxianProbeLookStaysPrivateFromEveryOtherViewer|TestGitaxianProbeLookDescribeLines|TestGitaxianProbeGameReplaysAndDescribesIdentically' ./view/
ok  	github.com/adams-shaun/gorge/view	0.672s
```

Behaviour goldens (gorge-context.md, run once before reporting):

```
$ go test ./internal/archtest/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/internal/archtest	3.622s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.385s
```

No new test was added, so the "fails without the fix" and "precondition"
requirements are N/A this round. No files changed, so `gofmt -l` and
`go run ./cmd/gentypes -check` are N/A (nothing to format/regenerate).

## Corpus availability

`.cards` was present as a symlink to the real corpus
(`.cards -> /home/sadams/projects/gorge/.cards`), so the corpus-backed tests
above ran for real (not skipped): `./effects/` 0.620s and `./view/` 0.672s are
non-vacuous runtimes, consistent with `CorpusRegistry` resolving Gitaxian
Probe/Force of Will/Brainstorm.

## Verified tests (what each asserts)

- `effects/revealhand_test.go:TestGitaxianProbeLookIsAPrivateLookScopedToTheActivator`
  — loads the real compiled corpus SA, asserts it has `Look$ True` and **no**
  `NumCards$` (fails loudly if the corpus pin moves), drives the look_ack
  pacing gate, then asserts the resumed emit produces exactly one **Secret**
  Note scoped to the activator carrying `slices.Equal(note.IDs, hand)` — the
  WHOLE target hand.
- `view/look_redaction_test.go:TestGitaxianProbeLookStaysPrivateFromEveryOtherViewer`
  — the activator sees the names; target/third seat/public spectator get no
  ids; omniscient keeps them; no public Note anywhere names a hand card.
- `view/look_redaction_test.go:TestGitaxianProbeLookDescribeLines` — the
  looker's line is `"Activator looks at Target's hand: "` and names the cards;
  every other viewer's line is `"Activator looks at hidden cards"`.
- `view/look_redaction_test.go:TestGitaxianProbeGameReplaysAndDescribesIdentically`
  — the game replays byte-identically and per-event transcript lines match.

These are exactly the privacy assertions the brief says not to weaken; I did
not touch them.

## Fails without the fix

N/A — no new test added. (The already-landed `868d7c6b` fix and its tests
predate this ticket.)

## Issues

- No new defects found. The player-facing consumer (`host/fanout.go`
  `eventBodiesFor`) is correct by delegation and needs no look-specific code.
- The one thing this ticket could **not** confirm is the *original* game state:
  no feedback snapshot is present for `feedback/20260923T015847Z-fad49275`,
  and the supplied URL is a live demo (no capture). So the exact point of
  failure in the reporter's match is unverifiable; the finding rests on the
  already-landed fix plus the current behavior tests. This limitation is
  already stated in the brief.

STATUS=DONE
COMMITS=
TESTS=effects{ProbeLook,ThoughtKnotSeer} ok 0.620s; view{ProbeLookPrivate,DescribeLines,ReplayDesc} ok 0.672s; archtest ok 3.622s; botbench byte-identical ok 1.385s
