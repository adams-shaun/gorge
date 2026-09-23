# Report — r2 (agent-20260919T192133Z-f7463cbe) — rebase resolution

Ticket: `K:Retrace — the cast-from-graveyard keyword is unimplemented (17 files)`.
**The implementation was already on main when this round started** (commit
`133495da feat(rules): implement kw:Retrace graveyard cast with land discard`,
merged by `fbbe4cb4`), and round t1 in this worktree added the deck-card
regression test (now `7de1a857 test(rules): cover Formless Genesis retrace`,
was `07232a2c` before the rebase). The full t1 report is preserved at
`.ds4/report-t1-retrace.md` (committed this round; previously left uncommitted
at the shared path, which is what blocked the rebase). The prior content of
`.ds4/report-r2.md` (the TriggerRemembered r2 report) is preserved at
`.ds4/report-r2-triggerremembered.md`.

## What this round did

`findings-r2.md` reported that the rebase onto main failed:

```
error: cannot rebase: You have unstaged changes.
--- merge fallback ---
error: Your local changes to the following files would be overwritten by merge:
	.ds4/report-t1.md
```

Root cause: round t1 wrote its report at the SHARED path `.ds4/report-t1.md`
and left it uncommitted; main tracks that path with a different ticket's
report. Same resolution as commits `5558278d` / `83d640d5`:

1. Moved this ticket's t1 report to the unique path
   `.ds4/report-t1-retrace.md` and restored `.ds4/report-t1.md` to its
   tracked content (`git restore <path>` — no branch switch, no shared state
   change).
2. Committed the moved report (`9785e5ba docs(retrace): record the retrace
   t1 report at a unique path`).
3. `git rebase main` — **clean, no conflicts**. The branch is now main
   (`e54228a0`) + exactly two commits:
   - `7de1a857 test(rules): cover Formless Genesis retrace`
     (rules/retrace_formless_test.go +54)
   - `9785e5ba docs(retrace): record the retrace t1 report at a unique path`
4. Re-verified everything after the rebase (output below).

## Brief coverage (all "Done means" items hold)

- **Formless Genesis graveyard cast with land discard** — covered by
  `rules/retrace_formless_test.go` (rebased content unchanged, re-verified):
  asserts the card is in the graveyard and still has Retrace (precondition),
  a land in hand, a `cast`/`retrace` option offered, the `KChoose` discard ask
  offers exactly the hand land, the discard sends it to the graveyard, and the
  spell reaches the stack.
- **Implementation** — rule-side (`rules/legal.go`/`rules/cast.go` per the t1
  report; commit `133495da` on main), not a `cards/keywords.go` expansion:
  Retrace is a casting option, the same family the keywords.go doc assigns to
  the rules side. The fixed additional discard cost has no card-authored
  parameter for the paramcensus to measure. The keyword registers in the
  coverage ratchet.
- **Measured corpus prevalence**: 17 `K:Retrace` files at the pin — matches
  the brief's claim.

## Gate commands and real output (all post-rebase)

`.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards` —
this was a real (not skipped) corpus run.

```
$ go build ./... && go test -run 'Retrace' ./rules/ > .ds4/scratch/retrace.log 2>&1; tail -5 .ds4/scratch/retrace.log
ok  	github.com/adams-shaun/gorge/rules	0.644s
```

Behaviour goldens:

```
$ go test ./internal/archtest/ 2>&1 | tail -2
ok  	github.com/adams-shaun/gorge/internal/archtest	3.330s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -2
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.299s
```

Format / generated types:

```
$ gofmt -l rules/retrace_formless_test.go
[no output]

$ go run ./cmd/gentypes -check
[exit 0]
```

## Fails without the fix

Recorded in the preserved t1 report (`.ds4/report-t1-retrace.md`): with the
Retrace graveyard-offer loop temporarily removed from `rules/legal.go`, the
test fails with `Formless Genesis Retrace cast not offered` (full decision
list pasted there); the source was restored byte-identically (`cmp` exit 0).

## Issues

None found and left unfixed. No chain-head or acceptance-ratchet movement is
expected or observed (no engine behaviour change in this branch; the test only
covers the already-landed primitive).
