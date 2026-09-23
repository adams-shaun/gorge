# Merge resolution report — cli-20260922T225143Z-bc326d39

## Operation and starting state

- `git status` at start: `On branch wt/cli-20260922T225143Z-bc326d39; nothing to commit, working tree clean` — no rebase or merge was in flight (the daemon's failed integration was rolled back; `main` was NOT an ancestor of HEAD).
- Ran `git merge main`. Auto-merged `AGENTS.md` and `rules/cast.go`; one content conflict: `internal/testutil/agentsdoc_test.go`.

## The conflict and its resolution

`internal/testutil/agentsdoc_test.go`, the `knownApproximationRows` constant only:

- HEAD (this branch, tip `e748019d`): `knownApproximationRows = 53`
- main (tip `78d3b764`): `knownApproximationRows = 50`

Both constants were measured at their own tips; the merge auto-combined BOTH sides' AGENTS.md row deletions, so neither described the merged table. I re-measured the merged `AGENTS.md` using the test's own row-counting logic (`| `-prefixed lines between the `## Known approximations` heading and the next `## ` heading, dropping the header row): **45 data rows**. Set `knownApproximationRows = 45`.

No other file had conflict markers. No behavioural choice was involved — the branch's damage-LKI fix (`56f98b13` + `e748019d`) and main's changes to `rules/cast.go` auto-merged.

## Commands and outputs

`git merge main`:

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Auto-merging rules/cast.go
Automatic merge failed; fix conflicts and then commit the result.
```

Row measurement:

```text
awk '/^## Known approximations/{inside=1; next} inside&&/^## /{inside=0} inside&&/^\| /{c++} END{print c}' AGENTS.md
46   (includes the header row => 45 data rows)
```

After staging the resolved file: `GIT_EDITOR=true git merge --continue` →

```text
[wt/cli-20260922T225143Z-bc326d39 f89a62fd] Merge branch 'main' into wt/cli-20260922T225143Z-bc326d39
```

Post-merge checks (`.cards` present):

- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` → `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` → `ok github.com/adams-shaun/gorge/rules 0.741s`
- `go test ./rules -run 'TestHeads'` → `ok github.com/adams-shaun/gorge/rules 1.744s` (main moved the golden heads; the merged tree reproduces them — no head movement from this branch's cost-LKI fix)

Final `git status --short`: empty (clean). HEAD = `f89a62fd`.

## Notes / unsure about

- The stale `.ds4/report-mrg1.md` from the sibling branch (677ee477) was tracked in git and the merge staged main's/theirs version of it; this report overwrites it. Not a conflict file.
- The merged approximation table at 45 rows is lower than EITHER side's constant because the two branches closed different rows and the auto-merge keeps both closures — measured, not assumed.
