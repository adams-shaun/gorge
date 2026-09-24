# Report — agent-20260919T181318Z-4dd3e7f5, round 3 (Tower Winder OriginAlternative pin)

**Note on this file:** `.ds4/report-r2.md` is a tracked path shared by
concurrent tickets; it held the Loamcrafter Faun ticket's r2 report (ticket
`agent-20260918T195920Z-2fd3b568`). Per the resolution precedent in commit
`91253535`, that content is preserved (committed) at
`.ds4/report-r2-2fd3b568.md`. This ticket's round-1 report is
`.ds4/report-r1.md` (it carries the full Tower Winder test description and the
fails-without-the-fix output). Per the dispatch, the reviewer reads THIS file,
so it now reports this ticket's current round.

## What this round did

Round 2's work was already complete and committed (`7dc6df18` at the time);
the r2 verdict carried **only the daemon module-gate log** — no MAJORs against
the round's own work. This round: (1) controller-ordered rebase, (2) root-cause
the two gate failures, (3) fix the one real defect they exposed.

### 1. Rebase (controller directive)

Tree was clean (`nothing to commit, working tree checked`). `git rebase main`
replayed `7dc6df18` → **`7100eff6`** onto `main` = `562e324d`, no conflicts.
Branch is now `562e324d` + `7100eff6` (Tower Winder pin, unchanged content) +
`6d56a42f` (this round's fix).

### 2. Gate-failure diagnosis (findings-r2)

Two failing packages, one cause. The only writer of the filename
`rules/repro_feedback_20260914T120000Z_fb01_test.go` is
`TestReproEmitTestIntoRulesCompilesAndFailsOnTODO` in `cmd/repro/repro_test.go`:
it emitted its skeleton and snapshot into the **real `rules/` directory** and
removed them at `t.Cleanup`. The gate log is fully explained by residue from an
earlier gate/probe run that was killed between emit and cleanup:

- `TestReproEmitTestIntoRulesCompilesAndFailsOnTODO (0.44s): emit exit 2,
  output:` (empty) — `cmd/repro.main.run` returns **2 for any error with the
  message on stderr** (`cmd/repro/main.go:60-80`), and the probe passes
  `io.Discard` as stderr. An empty output with exit 2 is therefore the
  `emitTest` refusal path: `"repro: %s already exists; not overwriting"`
  (`cmd/repro/main.go:430-436`) — the stale skeleton from the killed run was
  still on disk.
- `# github.com/adams-shaun/gorge/rules_test … open
  rules/repro_feedback_20260914T120000Z_fb01_test.go: no such file or
  directory` — the gate's `rules` build listed that file (it existed at load
  time), then the probe's cleanup deleted it mid-gate, so the compile opened a
  vanished file. Only a concurrent in-gate producer/deleter of that exact
  filename explains it; nothing else in the repo creates it.
- Residue confirmation after the gate: `rules/testdata/feedback/` existed and
  was **empty** — exactly what the old probe's cleanup leaves when it saw
  `rules/testdata` pre-existing (`testdataPreExisting` guard) after removing
  the stale snapshot dir.

Reproductions run this round:
- Stale state reproduced: with a (dummy) `rules/repro_feedback_..._test.go` +
  `rules/testdata/feedback/<id>/` present, `go test ./cmd/repro -run …IntoRules…`
  fails (in the dummy case at setup, because an empty .go poisons `rules_test`;
  with the real stale skeleton it fails as the findings show, at `emit exit 2`).
  Cleaned up afterward; `git status` clean.
- Post-fix, both failing gates pass in isolation and as a package (outputs below).

### 3. The fix — `cmd/repro/repro_test.go` only (commit `6d56a42f`, test-only)

`TestReproEmitTestIntoRulesCompilesAndFailsOnTODO` no longer touches the real
`rules/` dir. It now copies rules' non-test `.go` sources verbatim into
`zzrepro-emitrules/` at the repo root and emits there. Everything the probe
asserts is preserved: the target is an engine-scale package **named `rules`**
(`packageOf` reads the clone), the skeleton must declare `package rules_test`,
and `go test` on it must compile and fail **only on the TODO**. Root-level
scratch dirs are invisible to `go test ./...` (package list enumerated before
tests run) and nothing else builds them, so neither stale residue nor a
mid-build delete can recur. The cycle premise the real-rules target exercised
(feedback imports `rules`, so an internal-package skeleton would cycle) is now
asserted statically inside the test via `go list` on
`internal/testutil/feedback`'s imports. Doc comment rewritten with the failure
history. No production file touched; `effects/zone.go` byte-identical to main
(`git diff main -- effects/zone.go` empty).

Deviation from the brief's scope boundary ("do not change any non-test file" —
this IS a test file, and it is not one of the forbidden golden files): made
because the r2 verdict's gate failures are this round's work item, and leaving
the probe mutating the real `rules/` dir would re-park the ticket on the next
killed gate.

## The ticket's deliverable (unchanged from round 2, commit `7100eff6`)

`rules/changezone_origin_alternative_test.go`:
`TestChangeZoneOriginAlternativeTowerWinderGraveyard` — pins Tower Winder's
ETB trigger offering the **graveyard** Command Tower (named-card filter,
`Graveyard`-only alternative, ETB route), moving it to the searcher's hand, and
NOT offering the hand copy (hand is not a named origin). Full description and
the **fails-without-the-fix** proof (OriginAlternative read forced off at
`effects/zone.go`, ETB poses no search ask) are in `.ds4/report-r1.md`, with
the raw failing output preserved at `.ds4/scratch/t-nofix.log`; the reverted
file was restored byte-identically against `.ds4/scratch/zone.go.bak`. The
brief's false premises (Tower Winder not in any repo deck → the param census
never measured it → no `knownUnsupportedParams` entry to delete) were verified
in round 1 and are restated there.

## Gate commands and their real output (this round, after the rebase + fix)

`.cards` is the **symlink** (`.cards -> /home/sadams/projects/gorge/.cards`,
present before the first run — not created by me).

```
$ go test -run 'TestChangeZoneOriginAlternative' ./rules/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/rules	0.504s

$ go test ./internal/archtest/ 2>&1 | tail -1
ok  	github.com/adams-shaun/gorge/internal/archtest	3.543s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -1
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.642s

$ go run ./cmd/gentypes -check ; echo exit=$?
exit=0                                    # no output = no drift

$ gofmt -l cmd/repro/repro_test.go        # (round-2 file checked earlier too)
<no output>

$ go test ./cmd/repro/ 2>&1 | tail -1     # full package, once — I edited it
ok  	github.com/adams-shaun/gorge/cmd/repro	20.710s

$ go test ./cmd/repro -run 'TestReproEmitTestIntoRulesCompilesAndFailsOnTODO' -v
=== RUN   TestReproEmitTestIntoRulesCompilesAndFailsOnTODO
--- PASS: TestReproEmitTestIntoRulesCompilesAndFailsOnTODO (1.05s)
ok  	github.com/adams-shaun/gorge/cmd/repro	1.064s
```

Post-run residue check: `zzrepro-emitrules`, `zzrepro-emittmp`,
`rules/testdata` all absent; `git status` clean except the committed files.

## Fails without the fix

For the round's own change (the probe rewrite): with the probe reverted to
emitting into the real `rules/` dir, the failure is environmental (a killed
gate between emit and cleanup), not deterministic in a single test run — the
mechanism is instead proven from the findings log itself (the `open …
no such file` filename can only come from this probe; the empty-output exit 2
can only come from the overwrite refusal; the empty `rules/testdata/feedback`
residue matches its cleanup guard exactly). The Tower Winder test's
fails-without-the-fix output is in `.ds4/scratch/t-nofix.log` / r1 report.

## Issues

- None new beyond the fixed one. The only defect found this round WAS the
  fixed one (probe mutating the real `rules/` dir — residue + concurrent-build
  race; fixed in `6d56a42f`).
- Observation, no action taken (out of scope): the daemon module gate runs 28
  packages with `-p` concurrency, and `cmd/repro` was the only package that
  wrote into another package's source dir; with the probe fixed, no test in
  the module mutates another package's dir. If a future test ever needs an
  engine-shaped emit target, the clone pattern in this round's fix is the
  template.
- The stale `report-r2.md` collision (this ticket's report path vs the
  Loamcrafter ticket's r2 report in the controller's `.ds4` copy) is worth a
  controller-side note: ticket-specific report names (like `report-r1.md`
  here) avoid it.
