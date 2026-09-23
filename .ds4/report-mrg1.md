# Merge-conflict resolution report — mrg1 (agent-20260922T234314Z-bbfff2fb)

## Entry state and operation

`git status` showed a **clean** worktree on
`wt/agent-20260922T234314Z-bbfff2fb` at `df3f94cf`, with **no merge or rebase in
flight** — the daemon's conflicting rebase had been aborted/left clean. But
`df3f94cf` was a merge of an *older* main (`c4560130`), and main had since
advanced to `30a271af` (`git merge-base --is-ancestor main HEAD` was false; 28
commits in `HEAD..main`).

The branch's fix is `c0a04453 feat(effects): ask each Defined$ player for
GenericChoice` plus its report commit `13395d74`. Both were already in HEAD
(`git merge-base --is-ancestor c0a04453 HEAD` → true).

The `.ds4/merge-conflict-mrg1.md` transcript described a rebase of `13395d74`
onto main conflicting on `.ds4/report-t1.md`, with a merge fallback conflicting
on the same file plus `effects/context_test.go`, `effects/registry.go`,
`rules/resolution.go`. Main's changes since the old merge base touch exactly
those files. So the operation that actually remained was a fresh
**`git merge main`** — not the aborted rebase. I ran `git merge main`.

`.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`, so
the corpus-backed tests ran for real (the `rules` targeted run did not skip).

## Conflicted file and resolution

One content conflict, in `.ds4/report-t1.md` (a tracked accumulated report
file).

- **HEAD (branch) side** wanted: this ticket's `GenericChoice
  per-Defined$-player chooser` report (Ticket `agent-20260922T234314Z-bbfff2fb`,
  Commit `c0a04453`).
- **main side** wanted: the independent `trig:Attacks.NoResolvingCheck on
  Sentinel Sarah Lyons` report (Commit `30f3fc7a`) — main's later work.
- Both sides shared the same preceding report history, unchanged.

**Resolution: kept BOTH reports.** The conflict region was the file's tail
(lines 2164–2457): shared prose ended at the "…as the brief conditions it."
paragraph, then the two unique reports diverged to EOF. I removed only the three
conflict-marker lines, kept the branch's GenericChoice report, added a `---`
separator, then main's Sentinel Sarah Lyons report exactly as written. No prose
was dropped from either unique side; no conflict markers remain. This is the
same keep-both pattern the prior mrg1 rounds documented.

The other conflicted-by-name files **auto-merged cleanly** and were not manually
edited:

- `effects/registry.go` — carries both the branch's `SuspendGenericChoiceRest` /
  `GenericChoiceRest` / `Ctx.GenericChoosers` symbols and main's newer changes.
- `rules/resolution.go` — carries the branch's `genericChoosers` /
  `genericChooserIndex` / `"generic_players"` arm / `SuspendGenericChoiceRest`
  impl and main's scry-replacement work.
- `effects/context_test.go` — the fake host's `SuspendGenericChoiceRest` plus
  main's changes.

No code conflict required a judgement call. Main did **not** touch the branch's
core fix files (`effects/misc.go`, `decision/decision.go`,
`rules/generic_choice_players_test.go`, `rules/clone.go`), so the reviewed fix's
behaviour is intact.

## Commands run (real output)

```text
$ git status
On branch wt/agent-20260922T234314Z-bbfff2fb
nothing to commit, working tree clean

$ git merge-base --is-ancestor main HEAD && echo YES || echo NO
NO

$ git merge main
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
Auto-merging effects/context_test.go
Auto-merging effects/registry.go
Auto-merging rules/resolution.go
Automatic merge failed; fix conflicts and then commit the result.

# (resolved .ds4/report-t1.md: kept both reports, markers removed)

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.777s

$ go test -run 'TestGenericChoice|TestSeizeTheSpotlight|TestSentinelSarahLyonsBattalionSurvivesAttackerLeaving|TestScryReplacement|TestMill' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.665s

$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.612s

$ go test ./decision/
ok  	github.com/adams-shaun/gorge/decision	0.009s

$ git commit --no-edit
[wt/agent-20260922T234314Z-bbfff2fb 15b64ff2] Merge branch 'main' into wt/agent-20260922T234314Z-bbfff2fb

$ git status
On branch wt/agent-20260922T234314Z-bbfff2fb
nothing to commit, working tree clean

$ git merge-base --is-ancestor main HEAD && echo YES || echo NO
YES
```

The required post-merge ratchets passed. The branch adds no new trigger
matcher, closes no `knownUnsupported` / `knownUnsupportedParams` /
`knownUnmodelledCountHeads` entry, and needs no `addedAfterTheSplit` change, so
the ratchet tables are untouched.

## Issues

None introduced or discovered by integration. The conflict was limited to an
accumulated report file; main's engine/test changes auto-merged and the focused
checks passed.

---

# Merge-conflict resolution report — Companion (mrg1)

## Entry state and operation

The worktree was clean on `wt/agent-20260918T234402Z-c77011ce` at
`9497e794`, with no merge or rebase in flight. The reviewed Companion commits
were already present (`f7e41c45`, `1bb5e863`, `9497e794`); `main` was at
`767f3dd4`. The supplied daemon transcript named `.ds4/report-t1.md` as the
conflict. I ran `git merge main`; it reproduced that conflict and reported
`rules/trigger_match.go` as an automatic merge.

`.cards` is present as a symlink to `/home/sadams/projects/gorge/.cards`.

## Conflicted file and resolution

`.ds4/report-t1.md` is an accumulated report file. The branch side adds the
Companion registration report; main adds the independent GenericChoice and
Sentinel Sarah Lyons reports. I kept both sides, removing only the three merge
marker lines. The Companion text remains intact, followed by main's reports.
No engine-code conflict required manual resolution: `rules/trigger_match.go`
auto-merged and retains the Companion registration alongside main's changes.
All other changed files in the merge were automatic main integration, not
conflicts, and were left as merged.

## Commands and output

```text
$ git status --short --branch
## wt/agent-20260918T234402Z-c77011ce

$ git merge main
Auto-merging .ds4/report-t1.md
CONFLICT (content): Merge conflict in .ds4/report-t1.md
Auto-merging rules/trigger_match.go
Automatic merge failed; fix conflicts and then commit the result.

$ git diff --check
(no output; exit 0)

$ go test -run 'TestCompanionPrimitiveIsRegistered|TestCompanionCarrierIsUnderstood|TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' ./rules/
ok   github.com/adams-shaun/gorge/rules  0.801s

$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest  4.113s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench  1.584s
```

The combined rules run includes the Companion regression and the required
post-merge trigger-mode, deck, parameter, and count-head ratchets. No new
trigger matcher or ratchet entry was introduced/closed by this change. Both
behavior goldens passed; no pin change was needed.
