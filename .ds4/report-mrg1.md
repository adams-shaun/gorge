# Merge-conflict resolution: mrg1

## Starting state

The worktree was clean on `wt/cli-20260922T225138Z-e21c29e8`, with no rebase or merge in progress. HEAD was `d8fb30c6`, a merge commit integrating `main`; the failed daemon operation described in `.ds4/merge-conflict-mrg1.md` had already been resolved and committed in this worktree. The committed merge includes the conflict resolution for `internal/testutil/agentsdoc_test.go`, and the prior merge report records the resolution of the fallback conflict in `.ds4/report-mrg1.md`. No conflict markers remain in `internal/testutil/agentsdoc_test.go`.

## Conflicted files and resolution

- `internal/testutil/agentsdoc_test.go`: reported as a content conflict while replaying `f68b9396`. The committed resolution sets `knownApproximationRows = 73`, agreeing with the merged `AGENTS.md` table. The existing focused ratchet test passed.
- `.ds4/report-mrg1.md`: the merge fallback reported conflicting report histories. It is now replaced with this round's report, preserving the required durable account of the completed integration.
- `AGENTS.md` and `rules/legal.go`: the daemon reported automatic merges, not conflicts. They are present in the already-committed merge.

No new engine changes were made in this resolver session. The merge operation was already complete at entry, so there was no rebase/merge command to continue.

## Commands and output

```text
git status --short --branch
## wt/cli-20260922T225138Z-e21c29e8
(clean)

git log --oneline --decorate -10
... d8fb30c6 Merge branch 'main' into wt/cli-20260922T225138Z-e21c29e8

git grep -n -E '<<<<<<<|>>>>>>>' -- internal/testutil/agentsdoc_test.go
(no matches)

[ -e .cards ] && echo '.cards present'
.cards present

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|CastTarget|CastOffer'
ok   github.com/adams-shaun/gorge/rules 0.740s
```

The worktree has the real `.cards` corpus, so this was not a corpus-omitted run.

## Ratchets and concerns

The requested rules ratchets and target-offer tests passed. The merged approximation count is 73. No uncertainty remains about the conflict resolution itself; the merge commit predates this resolver session, and this session only updated the durable report.

## Issues

None found.
