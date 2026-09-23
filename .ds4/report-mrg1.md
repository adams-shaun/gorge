# Merge conflict resolution — mrg1

## Result

Merged `main` into `wt/cli-20260922T225142Z-e9128096`. The merge initially conflicted in `internal/testutil/agentsdoc_test.go`; resolved it, applied the table count matching the auto-merged `AGENTS.md`, and completed the merge. Added the missing `spell.Ability` census classification required by the requested post-merge ratchet.

## Entry state and conflict

Initial `git status` reported a clean tree on `wt/cli-20260922T225142Z-e9128096` at reviewed commit `6c86af9b`; no rebase or merge was in progress. Ran `git merge main`, which conflicted only in `internal/testutil/agentsdoc_test.go`. `AGENTS.md` and `effects/filter.go` auto-merged.

The conflict was over `knownApproximationRows` and its explanatory comment. The branch had removed the `CanBeTargetedByTriggeredSpellAbility` approximation row; main had independently updated/deleted approximation rows. I kept both sides' table changes and measured the auto-merged table using the same `| ` row parsing and header removal as the test: **35 data rows**. Set the constant to 35 and replaced both stale comments with a concise note reflecting that merged measurement. `knownOversizeRows` remains unchanged.

No code conflict occurred in `effects/filter.go`; its main and branch changes were auto-merged. The merge also brought main's changes to `AGENTS.md`, `effects/count.go`, `rules/attack_cost.go`, `rules/bestow.go`, `rules/cast.go`, `rules/count_head_ratchet_test.go`, `rules/layers.go`, `rules/legal.go`, `state/object.go`, and associated tests. These were merge results, not hand-edited conflict resolutions.

## Ratchet follow-up

The first requested post-merge rules ratchet run failed because the branch's new `rules/triggered_target_legality.go` reads `spell.Ability.Params`, and main's parameter-census rot guard had not classified that selector base. Added `spell.Ability` to `rules/paramcensus_test.go`'s `baseBuckets` as `bSA`, with a comment identifying it as the stack object's resolved `*cards.SA`. This is the type-justified census entry required by the new rules-side code.

## Commands and output

```text
git status --short --branch && git status
## wt/cli-20260922T225142Z-e9128096
On branch wt/cli-20260922T225142Z-e9128096
nothing to commit, working tree clean

git merge main
Auto-merging AGENTS.md
Auto-merging effects/filter.go
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.

# Measured merged Known approximations data rows
Known approximations data rows: 35

# .cards check
lrwxrwxrwx ... .cards -> /home/sadams/projects/gorge/.cards

go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'
ok   github.com/adams-shaun/gorge/internal/testutil  0.002s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
--- FAIL: TestEveryRepoDeckParamsAreRead (0.12s)
    paramcensus_test.go:2743: paramcensus rot guard: 1 findings:
        paramcensus: 1 unclassified Params reads (the census cannot rot):
        triggered_target_legality.go:20:24: unclassified Params base "spell.Ability" -- add it to baseBuckets with a type justification
FAIL
FAIL    github.com/adams-shaun/gorge/rules  0.759s
FAIL

# After adding the verified spell.Ability -> bSA census classification:
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.755s
```

The failing first ratchet run is recorded above; the same ratchet command passed after the census classification. `.cards` was present as the shared-corpus symlink, so the rules check was not a vacuous corpus-skipped run.

## Issues

No unresolved merge issue. The initial ratchet failure was resolved by adding the missing, type-justified census bucket; the separate `UnlessCost$` grammar remainder remains tracked by the related ticket cited in the issue brief.
