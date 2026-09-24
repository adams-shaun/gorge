# Merge-conflict resolution report — agent-20260923T042552Z-0936e140

## What was in flight

`git status` at start: **clean, no merge/rebase in flight**. The daemon's
failed attempt had aborted cleanly. Branch was 125 commits behind `main` and 6
ahead. I re-ran `git merge main` to reproduce, which opened the conflict.

## Conflicted files

Exactly one file was left unmerged: `effects/cardflow.go`.

`rules/paramcensus_test.go` produced a git "Auto-merging ... CONFLICT (content)"
line during the merge attempt, but it auto-resolved cleanly (the branch deleted
the `digUntilWithheldRange` exemption helper while `main`'s changes there were
elsewhere in the file); it never appeared as an unmerged path.

## The two sides

Both sides touched `effDigUntil`'s revealed-rest move block:

- **Branch (`c749b700a`, "feat(effects): implement DigUntil withheld rider
  semantics")** implemented the *other* withheld riders: non-literal `Amount$`
  via `digUntilAmountSVar`, `Shuffle$`/`ShuffleCondition$`, `NoMoveFound$`,
  `FoundLibraryPosition$`, `ImprintFound$`/`ImprintRevealed$`, and
  `NoneFoundDestination$`/`NoneFoundLibraryPosition$`. To support the NoneFound
  branch it introduced local `restDest, restPos := revDest, revPos` and wrapped
  the rest-move loop in `if !noMoveRevealed {`. Its doc comment claimed
  "RevealRandomOrder$ remains a deterministic existing-order stand-in because
  ambient randomness is forbidden."

- **Main (`3b8f333c2`, "feat(effects): DigUntil RevealRandomOrder shuffles the
  bottom return")** implemented `RevealRandomOrder$ True` with a seeded
  Fisher-Yates (`h.Rand`) over a `toReturn` slice, applied when the revealed
  destination is the library and position is `-1`; a stay-in-place placement
  keeps existing order behind one loud Note. It also deleted the corresponding
  AGENTS.md Known-approximations row and updated the doc comment.

These are complementary, not contradictory: the branch's stand-in sentence was
made stale by main's later, deliberate change to exactly that line.

## How I resolved it

**Doc comment** (region 1): kept the branch's full "riders implemented" text
and replaced only the stale final sentence with main's
"RevealRandomOrder$ True is implemented for the library-bottom return
(h.Rand, seeded and replay-exact); a stay-in-place placement keeps the existing
order behind one loud Note."

**Code** (region 2): took the branch's `restDest`/`restPos` + `!noMoveRevealed`
structure and folded main's `toReturn` construction and `revealRandomOrder`
shuffle *inside* the `if !noMoveRevealed` block. The shuffle now gates on
`restDest == state.ZLibrary` and `restPos` (the branch's rest-placement
variables) rather than main's `revDest`/`revPos`, so a NoneFound-library
placement would also get the random bottom return — the natural integration of
the two features. The remainder of the loop keeps the branch's `restDest`/
`restPos` moves, including the `RevealedLibraryPosition$` Note text (restPos).

One stray `}` remained from main's hunk boundary after the fold; removed it.
No conflict markers remain; `gofmt -l` is clean; `go build ./effects/` passes.

I did **not** touch any file outside `effects/cardflow.go`, and made no
behavioural change beyond joining the two sides.

## Commands and output

```
$ git status
On branch wt/agent-20260923T042552Z-0936e140
nothing to commit, working tree clean
$ git rev-list --left-right --count main...HEAD
125     6

$ git merge main
Auto-merging effects/cardflow.go
CONFLICT (content): Merge conflict in effects/cardflow.go
Auto-merging rules/paramcensus_test.go
Automatic merge failed; fix conflicts and then commit the result.

$ grep -n '<<<<<<<\|=======\|>>>>>>>' effects/cardflow.go   # after resolution
(none)

$ gofmt -l effects/cardflow.go
(clean)
$ go build ./effects/
(ok)

$ go test -run 'DigUntil|RevealRandom' ./effects/
ok  github.com/adams-shaun/gorge/effects  0.426s

$ go test -run 'DigUntil|RevealRandom' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.684s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  github.com/adams-shaun/gorge/rules  1.226s

$ go test ./internal/testutil/ -run 'TestKnownApproximation|TestAgentsDoc'
ok  github.com/adams-shaun/gorge/internal/testutil  0.002s

$ git commit --no-edit
[wt/agent-20260923T042552Z-0936e140 091247850] Merge branch 'main' into wt/agent-20260923T042552Z-0936e140

$ git status
nothing to commit, working tree clean
$ git log --oneline -1 --parents
091247850 f415551e0 76dd677b3 Merge branch 'main' into wt/agent-20260923T042552Z-0936e140
$ git rev-list --left-right --count main...HEAD
0       7
```

`.cards` was present as a valid symlink to the shared corpus
(`.cards -> /home/sadams/projects/gorge/.cards`), so the corpus-backed DigUntil
tests actually ran (effects 0.426s, rules 0.684s — not a sub-5s skip).

## Ratchets

All four post-merge ratchets pass (no new `Mode$` matcher and no ratchet entry
moved by this integration), `TestKnownApproximationsOnlyShrinks` passes (main's
deletion of the RevealRandomOrder row is consistent with the constant), and
`TestEveryRepoDeck`/`TestEveryRepoDeckParams` are green — the merge did not move
coverage.

## Uncertainties

None material. The one judgement call is using `restDest`/`restPos` (rather than
`revDest`/`revPos`) as the shuffle's gate, so a `NoneFoundDestination$ Library`
scan that finds nothing also randomises its bottom return. That is the only
reading that keeps both features coherent; a corpus card combining
`RevealRandomOrder$` with `NoneFound*` would exercise it, but no test asserts it
either way.

## Issues

No new defects found. This ticket was integration-only.

STATUS=DONE
COMMITS=09124785037bb0af2b9494993a0912bc38668f7d
TESTS=go test -run 'DigUntil|RevealRandom' ./effects/ ./rules/ (ok); post-merge ratchets + TestKnownApproximation (ok)
