# Merge-conflict resolution — agent-20260918T231813Z-2ff69b35 (round 2, merge)

## State found

`git status` at start: **clean, no operation in flight** on
`wt/agent-20260918T231813Z-2ff69b35` (tip `6bd74f0d`, merge-base with main
`19b8fb3a`, 6 commits ahead). The daemon's earlier rebase attempt (conflict at
`303ca9ad` on `.ds4/report-r2.md`) and its merge fallback (conflicts on
`.ds4/report-sol1.md` and `effects/filter.go`) had both been backed out, so I
performed the integration fresh as a merge of main into the branch — the repo's
own convention (`Merge branch 'main' into wt/…` commits throughout history).

## Conflicted files and resolution

### `.ds4/report-sol1.md` (the only content conflict this round)

- **Branch side:** appended a "cost-draw1 — agent-20260918T231813Z-2ff69b35,
  sol1 reconciliation" section (report relocation audit, checks, issues).
- **Main side:** appended a "Teapot Slinger / Convoke expend-4 — verification
  report (agent-20260923T113045Z-aa7f7a4e)" section.
- Both sides appended independent sections after the common base text; the two
  do not contradict. **Resolution: keep both** — branch's section first, then
  main's, separated by a `---`, with zero bytes of either removed.
- `.ds4/report-r2.md`, which conflicted during the daemon's rebase, auto-merged
  clean in the merge (both sides' distinct appends slotted together); verified
  via `git status` (staged `M`, no markers) and a marker grep (0 hits).

### `effects/filter.go` (auto-merged, then one ratchet fix)

Auto-merged cleanly — the sides touched disjoint regions (branch:
`faceIsTheChosenType` + the bare `sharesCreatureTypeWith` arm + the
`wordPredicate` bare form; main: the `StrictlySelf` predicate alias +
`SpecUsesConvokedAmount`). Verified with gofmt and the targeted tests below.

**One post-merge ratchet fix** (per the merge brief: fixing a newly-enforced
ratchet is part of resolving the merge): main's `TestEveryRepoDeckParamsAreRead`
rot guard flagged the branch's own `faceIsTheChosenType` —
`effects/filter.go:836: dynamic Params key "param" that is not a function
parameter`. The function looped `for _, param := range []string{"AddType",
"AddTypes"} { … st.Params[param] … }`, which the guard cannot attribute.
Unrolled it into two literal-key reads (`st.Params["AddType"]`,
`st.Params["AddTypes"]`) with the identical comma-split/trim/compare body —
behaviour byte-identical, no test edited, no golden touched.

## Commands run (real output)

```
$ git merge main
Auto-merging .ds4/report-sol1.md
CONFLICT (content): Merge conflict in .ds4/report-sol1.md
Auto-merging effects/filter.go
Automatic merge failed; fix conflicts and then commit the result.

$ grep -c '<<<<<<<\|=======\|>>>>>>>' .ds4/report-sol1.md   (after edit)
0

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  (before the filter.go fix)
--- FAIL: TestEveryRepoDeckParamsAreRead (0.13s)
    paramcensus_test.go:2810: paramcensus rot guard: 1 findings:
        paramcensus: 1 unclassified Params reads (the census cannot rot):
        ../effects/filter.go:836:36: faceIsTheChosenType: dynamic Params key "param" that is not a function parameter -- resolve it via a parameter or classify it
  (after the fix)
ok  	github.com/adams-shaun/gorge/rules	0.777s

$ go test -run 'TestTitanOfLittjara|TestDrawXCost|TestDrawCost|TestConvoked|TestStrictlySelf|TestAnimate|TestTriggerRemembered|TestDefinedLibrary|TestSharesCreatureType|TestSevinne|TestCopyOptional' ./effects/ ./rules/
ok  	github.com/adams-shaun/gorge/effects	0.609s
ok  	github.com/adams-shaun/gorge/rules	0.681s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.018s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.229s

$ gofmt -l effects/filter.go (and the other merged .go files)
(no output)

$ go run ./cmd/gentypes -check
(no output; exit 0)

$ git commit --no-edit
[wt/agent-20260918T231813Z-2ff69b35 f39cda12] Merge branch 'main' into wt/agent-20260918T231813Z-2ff69b35

$ git status
nothing to commit, working tree clean
```

`.cards` exists as a symlink to the real corpus, so corpus-backed tests ran
rather than skipped.

## Issues

None found during the resolution. No golden, head, or ratchet table was edited;
the only production edit was the guard-driven literal-key unroll in
`faceIsTheChosenType`, which is behaviour-preserving.
