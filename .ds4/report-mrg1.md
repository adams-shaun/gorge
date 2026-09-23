# Merge-conflict resolution — task cli-20260922T225143Z-4b0bde0d

## Situation found

`git status` was CLEAN on entry — no rebase or merge in flight. HEAD was
`f590cf11` ("Merge branch 'main' into wt/cli-20260922T225143Z-4b0bde0d"), a
merge a previous resolver run had already committed, but it had merged the
then-current main `2040e5d9`. Main had since advanced to `1be022eb`
(`git merge-base --is-ancestor main HEAD` → false), so the owed integration
was a merge of the NEW main into the branch (rebase is forbidden in a seat).

`.cards` was **present** — a real symlink to
`/home/sadams/projects/gorge/.cards` whose target exists — so every
corpus-backed ratchet below ran for real rather than skipping.

## Conflicted files and resolutions

### `internal/testutil/agentsdoc_test.go`

Both sides had independently reached the same constant, `43`, but wrote
different comments, and neither comment described the combined table.

- Branch side (`f590cf11`): `knownApproximationRows = 43`, comment "Main's
  closures bring the table to 44 rows; this branch's transactional exchange
  closure removes one more." It had deleted the
  `api:ExchangeLifeVariant fails closed` row.
- Main side (`1be022eb`): `knownApproximationRows = 43`, comment naming main's
  closures plus this branch's mulligan-redraw deferral. It had deleted the
  "mulligan declaration's REDRAW resolves immediately" row.

Measured the auto-merged `AGENTS.md` table directly: **42 data rows**. Each
side deleted a *different* row, so the merged table is 43 − 1 = 42, not 43.
Resolved to `knownApproximationRows = 42` with one comment naming BOTH
deletions (main's closures + mulligan-redraw deferral, and this branch's
transactional exchange closure). Verified: the merged table contains neither
the ExchangeLifeVariant row nor the mulligan-REDRAW row.

### `.ds4/report-mrg1.md`

A tracked file (force-added by earlier merge-doc commits despite `.ds4/` being
gitignored). Both tickets wrote their round report to this SAME path, so the
two sides are simply two different tickets' reports for the same filename.
Took **HEAD's** side (`git checkout --ours`): this is ticket
cli-20260922T225143Z-4b0bde0d's own report file, and main's copy is already
preserved in main's history (commit `a5dbe7b1`). This file is being rewritten
with the current report before the merge commit.

### Auto-merged (no textual conflict)

`AGENTS.md` auto-merged correctly (each side deleted a different row; the
merged file has both deletions). `rules/trigmatch_registry_test.go`,
`rules/heads_test.go` and the rest of main's feature work merged cleanly.

## Commands and output

- `git status` → clean, on `wt/cli-20260922T225143Z-4b0bde0d`, no rebase/merge.
- `git merge-base --is-ancestor main HEAD` → exit 1 (main not integrated).
- `git merge main --no-edit` → auto-merged 15+ files; content conflicts in
  `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`.

Measured table row counts after merge:

    $ python3 (heading scan of AGENTS.md)  → data rows: 42
    exchange row present: False
    mulligan row present: False
    $ grep -n 'knownApproximationRows =' internal/testutil/agentsdoc_test.go
    28:	knownApproximationRows = 42

- `gofmt -l internal/testutil/agentsdoc_test.go` → (no output)
- `go run ./cmd/gentypes -check` → (no output)
- `go test -run 'TestKnownApproximations' ./internal/testutil/` →
  `ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s`
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` →
  `ok  	github.com/adams-shaun/gorge/rules	0.784s`
- `go test -run 'TestExchangeLife|Mulligan' ./rules/` →
  `ok  	github.com/adams-shaun/gorge/rules	0.764s`
- `go test ./internal/archtest/` →
  `ok  	github.com/adams-shaun/gorge/internal/archtest	3.910s`

## Uncertainties / notes

- The merge commit only includes main's integration and the two conflict
  resolutions; no unrelated file was edited.
- `rules/heads_test.go` arrived from main already carrying the orchestrator's
  re-pin for main's own tickets — not touched here.
- The `.ds4/report-mrg1.md` filename collision (every second-round merge
  resolver overwriting the same tracked path) is a latent pipeline papercut
  worth a follow-up, but it is out of scope for this integration.

## Issues

None new found during this integration pass. The pre-existing branch fix
(transactional life exchange) and main's feature work are both preserved; the
only change forced by the conflict was the ratchet constant 43 → 42, mandated
by the two independent row deletions now landing together.
