# Merge conflict resolution — mrg1

## Result

Integrated current `main` (`4cdffbc1`) into `wt/cli-20260922T225142Z-0ab0cb60`.
The reviewed `8d83f028` fix (`MaxTotalTargetPower$` offset bound + cross-mode
Charm cap read) and main's new commits (land-type statics `0fbc2d10`, cascade
closures `e46f051d`/`fd…`) are both retained. Two content conflicts:

- `internal/testutil/agentsdoc_test.go` — the approximation-row ratchet constant.
- `.ds4/report-mrg1.md` — a tracked per-merge report artifact (this file).

## Background: an earlier merge commit already existed

`HEAD` was already `a4fe169d`, a merge of main **as of `4a7bb2fe`** into
`8d83f028`, produced by a prior mrg1 run that the daemon set aside
(`.ds4/status-merge-mrg1.cutoff.json`, history line "merge resolver mrg1 name
was held by an already-finished run; set it aside, relaunching fresh"). The
daemon's fresh integration attempt rebased onto current `main`, conflicted on
`8d83f028`, then aborted (`git reflog`: `rebase (abort): returning to …`). So
the branch still did NOT contain main's newer commits; this run merged current
`main` (`4cdffbc1`) to complete the integration.

## Conflict 1 — `internal/testutil/agentsdoc_test.go`

Both sides only differ in the explanatory comment above the constant; but the
constant's value was wrong for this merge on BOTH sides.

- **Branch side (HEAD):** the merged table measured **39** rows and documented
  main's earlier closures plus this branch's `maxpower1` closure.
- **Main side:** also **39** rows, documenting "this branch's cascade1 deletion
  and main's token-replacement deletion".
- **Measured truth after merging current main:** the merged `AGENTS.md` has
  **38** data rows. Main's cascade work (`e46f051d`) deletes `(cascade1)`, and
  this branch's reviewed fix deletes `(maxpower1)`; both deleted rows are
  absent from the merged table. Each side's 39 counted only its own merge
  (each side still carried the other side's row).

Resolution: keep the branch's fuller closure-history comment, extend it to name
main's `cascade1` deletion, and set `knownApproximationRows = 38` — the
measured count. Verified with a row count over the merged `AGENTS.md`
(`data rows: 38`) and
`go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/` → `ok`.

## Conflict 2 — `.ds4/report-mrg1.md`

This file is a tracked merge-report artifact that collides on every branch
(its on-main and on-branch versions document different tickets' merges;
`git log main -- .ds4/report-mrg1.md` shows repeated `docs(merge)` commits).
Resolved by replacing the conflicted content with this report for the current
merge.

## Commands and output

- `git status` — clean, no operation in progress; HEAD `a4fe169d`, main `4cdffbc1`.
- `git reflog -15` — showed the prior merge commit, then the fresh run's
  `rebase (start): checkout main` → `rebase (abort)` → `reset: moving to HEAD`,
  i.e. the fresh integration was aborted and never landed.
- `git merge main` — `CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  and `internal/testutil/agentsdoc_test.go`; other changes auto-merged.
- Row count of merged `AGENTS.md` (python3 over the `## Known approximations`
  section) — `merged AGENTS.md data rows: 38`.
- `gofmt -l internal/testutil/agentsdoc_test.go` — no output (clean).
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/` —
  `ok github.com/adams-shaun/gorge/internal/testutil 0.017s`.

## Issues

No new unfixed issue was found while resolving the merge; the integration
introduced no engine-code change. The `knownApproximationRows` comment drift
between branches is cosmetic noise from the tracked per-merge report artifact;
no action needed.
