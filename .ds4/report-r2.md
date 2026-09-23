# Report — r2 (agent-20260923T135903Z-2444d628) — merge resolution

Ticket: `R:Event$ Scry` replacements (Kenessos count rewrite, Eligeth
draw-instead). The **implementation was completed in round t1** — commit
`ae4fed1e feat(rules): apply R:Event$ Scry replacements before the scry look`
— and its full verification report is preserved in this worktree at
`.ds4/report-t1-scry-repl.md` (committed as `9c8c8c37`; previously left
STAGED at the shared path `.ds4/report-t1.md`, which is what blocked the
merge).

## What this round did

`findings-r2.md` reported that the rebase/merge onto main failed:

```
error: cannot rebase: Your index contains uncommitted changes.
error: Please commit or stash them.

--- merge fallback ---
error: Your local changes to the following files would be overwritten by merge:
  .ds4/report-t1.md
Merge with strategy ort failed.
```

Root cause: round t1 wrote its report at the SHARED tracked path
`.ds4/report-t1.md` and left the overwrite staged. Main tracks that path with
a different ticket's report, so the merge refused to touch the file, and the
staged index blocked the rebase. Resolution (the same pattern the Deep Spawn
Mill ticket's r2 used):

1. Moved the t1 report to the ticket-unique path
   `.ds4/report-t1-scry-repl.md` and restored `.ds4/report-t1.md` to its
   tracked content (`git restore --staged --worktree -- .ds4/report-t1.md` —
   no branch switch, no shared-state change). Committed the preserved report
   (`9c8c8c37`), so the branch's only modification to the shared file is
   none — main's version wins at the merge cleanly.
2. Merged main (`b3523eab`) into the branch → merge commit `de0e787a`,
   **clean, no conflicts**. Before merging I verified the overlap: main's
   changes to the three files our commit also touches are all in far-away
   regions — `effects/registry.go` (main adds `Ctx.CopyOpt` at ~line 1222;
   ours adds `Host.Scry` at ~line 168), `effects/context_test.go` (main
   rewrites the `fakeHost.Ask` doc comment at ~line 437; ours adds
   `fakeHost.Scry` at ~line 256), `effects/cardflow.go` (main changes
   `effReveal` at ~line 2267; ours changes `effLookAndArrange` at ~line 2909).
   `rules/replacement.go`, `rules/arrange.go`, `rules/coverage_test.go` and
   `rules/scry_replacement_test.go` are untouched on main since the merge
   base. The merge confirmed: zero conflicts.
3. Re-ran the brief's gates on the MERGED tree (output below) — all green.

## Gates run on the merged tree (real output)

```
$ ls -ld .cards
lrwxrwxrwx 1 sadams sadams 34 Sep 23 08:53 .cards -> /home/sadams/projects/gorge/.cards
```

`.cards` present, so the corpus-backed rules test ran for real (0.65s), not
the vacuous-skip path.

```
$ go build ./... && echo BUILD_OK
BUILD_OK

$ go test -run 'TestKenessosReplacesScryCountBeforeLooking|TestKenessosDoesNotReplaceTheCompletedScryRecord|TestEligethDrawsInsteadOfScrying|TestScryReplacementPrimitiveIsRegistered' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.652s

$ go test ./internal/archtest/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/internal/archtest	3.370s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.245s

$ gofmt -l <changed files>            # empty
$ go run ./cmd/gentypes -check
GENTYPES_OK
```

## Status of the brief

The brief's "Done means" items were all satisfied and verified in round t1 —
see `.ds4/report-t1-scry-repl.md` for the per-item evidence, including the
`## Fails without the fix` paste (all four tests fail with the production
hunks reverted, restored byte-identically after). This round re-verified the
whole set still holds after merging main: build, the targeted four-test
command, `internal/archtest`, the botbench split golden (unmoved — no
re-pin), gofmt and `gentypes -check`. No code was changed this round, so no
ratchet/head movement is possible; the t1 report already records none.

The remaining CR 616.1 deviation (simultaneous Kenessos + Eligeth apply in
deterministic scan order rather than an affected-player order choice) is
unchanged, is recorded in commit `ae4fed1e`'s message and the t1 report, and
was filed as a follow-up ticket in round t1
(`.ds4/new-tickets/scry-replacement-cr616-order.md.filed` marker present).

## Issues

- **Process, not code (recurring):** shared tracked report filenames
  (`.ds4/report-t1.md`, `.ds4/report-r2.md`) collide across tickets — this is
  the second ticket in two days whose merge onto main was blocked by a staged
  overwrite of `report-t1.md` (Deep Spawn Mill r2 hit the identical failure).
  The Deep Spawn r2 report already asked for a controller-level rule; adding
  this occurrence as measured evidence: dispatch reports to ticket-unique
  paths up front, or stop tracking the report slots in git.
- No engine defect was found this round; the merge introduced no conflict and
  no behaviour movement (botbench split unchanged, archtest green).
