# Merge-conflict resolution — task cli-20260923T060000Z-choose-number

(Note: the previous report-mrg1.md at this path belonged to the ctms-tag merge
seat and was controller-copied worktree context; it is replaced by this report.)

## State found

`git status` was CLEAN — no rebase or merge in flight (the daemon's failed
rebase had rolled back; reflog confirms `rebase (start): checkout main` at
05:30:17 followed by `rebase (abort)`). HEAD was the approved fix `be867983`
("fix(effects): ChooseNumber poses a real mid-resolution number ask (ct1)")
on top of a merge of an older main (`b39e280d`, merging origin/main at
`c6814693`); `main` (`7955156e`) was ahead with the battle-defeated,
pw-combatdamage and equip-reduce integrations. I reproduced the integration
with `git merge main`, which hit exactly the one conflict the daemon saw
(`internal/testutil/agentsdoc_test.go`); AGENTS.md, rules/cast.go and the rest
auto-merged.

## Conflicted file: internal/testutil/agentsdoc_test.go (one hunk, the
`knownApproximationRows` comment block; the constant line itself was common
text and read 28 on both sides)

What each side wanted:

- **Branch (`be867983`, reviewed fix):** deleted the (ct1) row from AGENTS.md
  ("ChooseType/ChooseColor/ChooseNumber mid-resolution asks") and set
  `knownApproximationRows = 28`, with a comment reciting the history it
  inherited: base 30, staticgoad1→ap1 swap, mtsp1 deletion, plus its own ct1
  deletion.
- **Main (`7955156e`):** deleted the (battle1) row from AGENTS.md
  (cli-20260923T060000Z-battle-defeated: CR 310.7 defender, combat-damage
  defense-counter removal, CR 310.11 defeated exile-and-cast, real protector
  policy) and ALSO set `knownApproximationRows = 28`, with a comment reciting
  the same inherited history plus its own battle1 deletion.

Both sides' constant of 28 was measured against its OWN AGENTS.md — the
constants agree numerically only because each deletion is balanced by the
other branch's state. AGENTS.md auto-merged cleanly (both row deletions
applied, verified below), so the merged table measures **27** data rows.
Resolution: kept both sides' comment content merged into one recitation and
**lowered the constant to 27** (shrinkage, the only allowed direction).
Measured against the merged AGENTS.md with the test's own algorithm
(`python` re-implementation of `approximationRows` + `standInCell`):
27 data rows, 4 oversize rows (≤ `knownOversizeRows` = 8). Verified the
merged AGENTS.md contains neither the (ct1) nor the (battle1) row and keeps
(ap1), (mtsp1) gone, (staticgoad1) gone.

rules/cast.go and all other auto-merged paths were left untouched by me.

## Commands run (real output)

- `git merge main`
  → `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`
  (AGENTS.md, rules/cast.go auto-merged)
- `git add internal/testutil/agentsdoc_test.go && git commit --no-edit`
  → `[wt/cli-20260923T060000Z-choose-number 0d7f0754] Merge branch 'main'
  into wt/cli-20260923T060000Z-choose-number`; `git status -sb` clean after.
- `go test -run 'TestKnownApproximations' ./internal/testutil/`
  → `ok  github.com/adams-shaun/gorge/internal/testutil 0.001s`
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestChosenNumber'`
  → `exit=0`, `ok github.com/adams-shaun/gorge/rules 0.827s`; verified with
  `-v` that all 8 matched tests RAN and PASSED (no FAIL, no SKIP — the corpus
  symlink `.cards → /home/sadams/projects/gorge/.cards` was present,
  TestEveryRepoDeckIsFullySupported 0.60s, TestEveryRepoDeckParamsAreRead
  0.13s are real runs):
  TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched,
  TestEveryDispatchedTriggerModeHasAMatcher, TestEveryRepoDeckIsFullySupported,
  TestEveryRepoDeckCountHeadResolves, TestEveryRepoDeckParamsAreRead,
  TestChosenNumberAskIsPosedMidResolution,
  TestChosenNumberAnswerRecordsTheChosenNumber,
  TestChosenNumberZeroAnswerIsRecorded — all PASS.
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean.

## Issues

None found — integration-only change; no new defect surfaced during the
merge. The branch's (ct1) deletion and main's (battle1) deletion both
survived the merge intact.

STATUS=DONE
COMMITS=0d7f0754
TESTS=go test ./internal/testutil -run TestKnownApproximations (ok); go test ./rules -run 'ratchets+TestChosenNumber' (ok, 8/8 PASS verified with -v, .cards present)
