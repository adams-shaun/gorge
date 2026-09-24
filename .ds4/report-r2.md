# Report — r2 (agent-20260919T055500Z-a4cd7643) — kw:Sunburst

Ticket: `kw:Sunburst` — a permanent with Sunburst enters with one counter per
colour of mana spent to cast it (+1/+1 for a creature, charge otherwise; the
`DB$ Animate | Keywords$ Sunburst` grant shape on Solar Array / Lux
Artillery). Round 1 implemented the semantics rules-side and is at
`.ds4/report-t1.md` (line ~2505, commit list updated to the post-rebase shas).
This round: the controller-ordered `git rebase main` (one append-append
conflict in the shared `.ds4/report-t1.md`, resolved by keeping BOTH blocks —
main's new damage-replacement report and this ticket's, insertions only), and
the resolution of the single r2 finding.

## Resolution of the r2 findings, each one

### [MAJOR] "the diff only adds a rules-side RegisterNonAPI('kw:Sunburst')
marker and never changes the keyword expander/registry" — FIXED

The finding was right about the brief's letter: Done means names
"the keyword registered in `cards/keywords.go`", and r1 shipped only the
rules-side synthetic (`rules/replacement.go`'s `sunburstEntryMatch`) plus the
`RegisterNonAPI` marker. Fixed in `26590394`:

1. **`cards/kw_sunburst.go` (new)** — registers the `Sunburst` head in the
   `kwExpanders` table (`registerKeyword(kwSunburst, "Sunburst")` in
   `init()`, the standard per-keyword file the split created). The expander
   turns the bare printed `K:Sunburst` line (15 corpus files, no parameter)
   into the entry Repl on the face itself: `Event$ Moved |
   Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$
   Updated`, body `DB$ PutCounter | Defined$ Self | CounterType$ <kind> |
   CounterNum$ Count$Converge | ETB$ True`, the kind decided on the printed
   face's types (P1P1 for a creature, CHARGE otherwise — the same decision
   Forge's CardFactoryUtil makes). The body is byte-for-byte the shape the
   r1 synthetic built, so the converge plumbing (pay-time FlagConverged
   CastInfo) is unchanged.
2. **`cards/kw_registry_test.go`** — `"Sunburst"` joined `expandedHeads`
   with a comment naming the ticket, so both registry ratchets
   (`TestEveryExpandedKeywordHasAnExpander` and
   `TestNoKeywordIsRegisteredThatTheSwitchNeverExpanded`) pin it.
3. **`rules/replacement.go`** — `sunburstEntryMatch` is now the GRANT shape
   only: it keeps the derived-keyword read but returns nil when the
   entering object's PRINTED face carries the `Sunburst` keyword line, because
   the printed face now carries the cards-side expansion and the face-Repl
   scan already collects it — a synthetic on top would put the entry counters
   twice. The grant shape (Solar Array / Lux Artillery deliver the keyword to
   the DERIVED list only; no printed-face expansion can see it) routes through
   the synthetic unchanged. Doc comments on the expander, the synthetic and
   the dispatch site each state the split and the double-count guard.
4. **`rules/sunburst_test.go`** — header comment updated to the two-sided
   architecture; the tests themselves are unchanged (they were already
   exact-count).

The r1 architecture rationale (why the GRANT half must stay rules-side) is
recorded in the expander's and the synthetic's doc comments, and in the
commit message: a printed-face K: expansion can never see a keyword a
resolving Animate grants to the derived list.

### Why not register without expanding (rejected alternative)

`kwExpanders` entries without an expander function are impossible by
construction (`registerKeyword` takes a func), and a no-op expander would be
a dishonest registration — the printed face would carry a live `K:Sunburst`
line that expands to nothing. The implemented split gives the printed route
a real cards-side expansion (the brief's contract) and keeps the grant route
rules-side (the only place it can live).

## Gates run (real output, on the rebased tree, `.cards` present as a symlink)

```
$ go build ./...            (no output, exit 0)
$ go vet ./cards/ ./rules/  (no output, exit 0)
$ gofmt -l cards/kw_sunburst.go cards/kw_registry_test.go rules/replacement.go rules/sunburst_test.go
                            (no output)
$ go run ./cmd/gentypes -check
                            (no output, exit 0)
$ go test -run 'TestSunburst|TestEveryExpandedKeywordHasAnExpander|TestNoKeywordIsRegisteredThatTheSwitchNeverExpanded' ./rules/ ./cards/
ok  	github.com/adams-shaun/gorge/rules	0.467s
ok  	github.com/adams-shaun/gorge/cards	0.002s
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	4.659s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.723s
```

The botbench 20-game split did not move (byte-identical, no re-pin): no
repo-deck game behaviour changed — the printed route delivers the identical
counters the synthetic delivered in r1, and no repo deck carries a Sunburst
card (r1 measurement, unchanged).

## Fails without the fix (both halves proven against the exact-count tests)

**Break 1 — registration reverted** (`cards/kw_sunburst.go` removed, restored
byte-identically after, `cmp` clean): the printed carriers lose their entry
counters entirely (the synthetic's printed-gate now correctly skips them),
which is the exact defect the brief described:

```
--- FAIL: TestSunburstEtchedOracleEntersWithP1P1PerColour (0.44s)
    --- FAIL: .../two_colours_of_mana
        sunburst_test.go:77: precondition: Oracle zone=graveyard, want battlefield
    --- FAIL: .../one_colour_of_mana
        sunburst_test.go:97: precondition: Oracle zone=graveyard, want battlefield
--- FAIL: TestSunburstPentadPrismEntersWithChargePerColour (0.00s)
    sunburst_test.go:142: Prism CHARGE=0, want 2 (two colours spent)
FAIL	github.com/adams-shaun/gorge/rules	0.461s
```

(The Oracles died to the 0-toughness SBA — a silent zero entering, the
r1 defect class, now caught loudly.)

**Break 2 — the printed-gate in `rules/replacement.go` removed** (restored
byte-identically after, `cmp` clean): printed carriers double-count, proving
the gate is load-bearing and the two halves are complementary, not redundant:

```
--- FAIL: TestSunburstEtchedOracleEntersWithP1P1PerColour (0.41s)
    --- FAIL: .../two_colours_of_mana
        sunburst_test.go:80: Oracle P1P1=4, want 2 (two colours spent)
    --- FAIL: .../one_colour_of_mana
        sunburst_test.go:100: Oracle P1P1=2, want 1 (one colour spent)
FAIL	github.com/adams-shaun/gorge/rules	0.419s
```

## Ratchets / head movement

- `knownUnsupported`: unchanged — no Sunburst card is in any repo deck, so no
  row exists to drop (the brief marks the ratchet row conditional on a deck
  import that has not happened).
- No new trigger mode, count head or param registration. Chain heads not run
  (daemon gate). Botbench byte-identical shows no repo-deck movement.
- `knownApproximationRows` unchanged; no AGENTS.md row added or grown. The
  `kw:Sunburst` mention in `faceWantsConverge`'s r1 comment (which said a
  cards-side expansion was "the planned second consumer of this seam") is now
  realised, not approximated.

## Deviations from the brief

None beyond r1's recorded ones (converge-head reuse; the counter kind decided
on the printed face). The brief's registration contract is now met as
written; the semantic home of the GRANT half in `rules/` is a structural
necessity (the derived-keyword list is not a face), stated in the commit
message and both doc comments.

## Issues

- None new. The 19-file `Sunburst` corpus prevalence breaks down as 15
  `K:Sunburst` carriers (all covered by the cards-side expansion), the 2
  `Animate | ... Sunburst` grant riders (covered by the synthetic), and 2
  mentions-only files — no remaining unimplemented shape was found in the
  family this round.

---

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


---

# Report — r2 (agent-20260918T233200Z-e0817443) — dynamic `TargetMin$`/`TargetMax$` bounds

**Round 2 = the controller-ordered rebase round.** The substantive work was
completed, committed and verified in round t1 (commit `033acdd5`, reported in
`.ds4/report-t1.md`). Round r2's findings (`findings-r2.md`) named exactly two
mechanical blockers, both now resolved:

1. *"cannot rebase: You have unstaged changes"* — the uncommitted
   `.ds4/report-t1.md` held this ticket's round-t1 report. Committed as
   `8b7844f9`, resolved so that **nothing from any other ticket is destroyed**:
   this ticket's report is prepended and main's accumulated report file
   (2,162 lines of other tickets' reports) is preserved verbatim below it.
2. *"Your local changes to .ds4/report-t1.md would be overwritten by merge"* —
   the same file, the same cause. Resolved inside the rebase.

## Rebase

```
$ git add -f .ds4/report-t1.md && git commit -m "docs: report dynamic TargetMin/TargetMax round-t1 and preserve accumulated reports"
[wt/agent-20260918T233200Z-e0817443 8b7844f9]
$ git rebase main
Rebasing (1/2) … Rebasing (2/2)
CONFLICT (content): Merge conflict in .ds4/report-t1.md
```

Resolution: the file was rebuilt as the union — my t1 report (221 lines) +
separator + `git show main:.ds4/report-t1.md` (2,162 lines) — `git add`,
`git rebase --continue`. Result:

```
$ git log --oneline -4
3a3d4f24 docs: report dynamic TargetMin/TargetMax round-t1 and preserve accumulated reports
e1ec6436 fix(rules): honour a resolved-zero dynamic TargetMin$/TargetMax$ pair
f8e330c3 merge(agent-20260919T062939Z-4b5f8950): RepeatOptional$ … (main tip)
```

The branch is now linear on main tip `f8e330c3`; the old merge commit
`c6c3b2fa` was dropped by the rebase (its only purpose — carrying main — is
satisfied by the rebase itself). The replayed code commit is `e1ec6436`,
byte-identical in content to `033acdd5`. The branch's tracked diff vs main is
the code fix + its new test file + the two report insertions (227 + 0 deletions
to any other ticket's text — `git diff --stat main...HEAD`:
`.ds4/report-t1.md | 227 +++`, plus the five code/test files, all
insertions-only except the two lines `modeIsKicked` refactoring touched in
`rules/statics.go`).

## Post-rebase verification (fresh, at main tip `f8e330c3` base)

All commands run once each, in this worktree, `.cards` present (symlink to the
shared corpus — confirmed `lrwxrwxrwx .cards -> /home/sadams/projects/gorge/.cards`).

```
$ go build ./...
(clean)

$ go test -run 'TargetMax|TargetMin|TearAsunder|PestInfestation|ResolvedTargetBounds|TriggerPlacementAsk' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.044s

$ go test -v -run 'TestResolvedTargetBoundsResolvedZeroIsHonoured|TestTearAsunderKickedTakesOnlyTheSubTarget|TestTearAsunderUnkickedStillTargetsArtifact|TestPestInfestationZeroXAsksNothing|TestTriggerPlacementAskResolvedZeroPosesNothing|TestAnnouncementAskBareXReadsThePaidX' ./rules/
--- PASS: TestTriggerPlacementAskResolvedZeroPosesNothing (0.00s)
--- PASS: TestResolvedTargetBoundsResolvedZeroIsHonoured (0.00s)
--- PASS: TestTearAsunderKickedTakesOnlyTheSubTarget (0.00s)
--- PASS: TestTearAsunderUnkickedStillTargetsArtifact (0.00s)
--- PASS: TestPestInfestationZeroXAsksNothing (0.00s)
--- PASS: TestAnnouncementAskBareXReadsThePaidX (0.00s)
ok  	github.com/adams-shaun/gorge/rules	0.071s

$ go test -run 'Kicked|Count' ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.567s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.321s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.230s

$ gofmt -l <changed files>          (no output)
$ go run ./cmd/gentypes -check      (no output)
```

`TestConstructedDefaultIsByteIdentical` is **unmoved** — no repo deck
exercises the resolved-zero shape. `internal/archtest` green. No forbidden
trailers in `e1ec6436` (`grep -i 'ref:\|co-authored'` on its message → no
match).

The per-part failure proofs ("Fails without the fix") were executed in round
t1 with the revert-restore-`cmp` protocol and are pasted in
`.ds4/report-t1.md`; the code is byte-identical since, so they stand.

## Substantive summary (unchanged from t1 — details in `.ds4/report-t1.md`)

- The brief's headline premise is stale at main: the X/Y resolver already
  landed as `b3786f11` (merged), including the Pest Infestation `X=3 → Max 3`
  test.
- What this ticket adds (`e1ec6436`): `resolvedTargetBounds` honours a
  **resolved-zero** bound as written (Tear Asunder's kicked `TargetMin$ X |
  TargetMax$ X` over `Count$Kicked.0.1`), keeps the 1-clamp only for an
  unresolved token, seeds `Ctx.PendingKicked` from the pending cast's chosen
  mode at the announcement ask, and makes the three target-ask sites
  (`targetAsk`, `subTargetAsk`, `askTarget`) decline to pose a Min 0/Max 0 ask
  (a hard engine panic shape). 5 new tests in `rules/targetmax_resolved_zero_test.go`.
- Deviations from the brief's letter (all measured, stated in the t1 report):
  no deck-side ratchet row exists for either carrier (neither card is in
  `internal/testutil/decks/`); the World Shaper precon census deck is not
  committed to the repo.

## Issues

- **Coverage gap (not fixed):** `subTargetAsk`'s resolved-zero arm is covered
  only by the shared resolver unit test and the identical `targetAsk`/`askTarget`
  panic proofs — no in-budget fixture reaches `castCostReadsAllTargeted` with a
  sub whose dynamic pair resolves to 0 (corpus carriers: Wayta, Urgent
  Necropsy). Would deserve a targeted test if a carrier lands in a repo deck.
- **No CR-lane test** for the resolved-zero pose/clamp shape: it is a
  decision-pose/clamp defect, not a CR rule the conformance lane cites. A lane
  test citing CR 601.2c/608.2b target-count feasibility (I-2 territory) would
  make the shape visible to the ledger.
- Adjacent, untouched: `modeFlags` still spells the five kicked modes in its
  own switch (it must — they map to different flag bits); `modeIsKicked` is now
  the `Kicked`-predicate home and both must be updated together if a new
  kicked-cast mode lands.

## Commits

- `e1ec6436` fix(rules): honour a resolved-zero dynamic TargetMin$/TargetMax$ pair
- `3a3d4f24` docs: report dynamic TargetMin/TargetMax round-t1 and preserve accumulated reports

# Report — r2 (agent-20260918T195920Z-2fd3b568) — Loamcrafter Faun `TriggerRemembered$Amount`

**Historical reconciliation round (before the sol1 review).** The ticket's
code work and tests were committed along with the t1 report, preserved at
`.ds4/report-t1-2fd3b568.md`; the shared `.ds4/report-t1.md` was restored
to main's content. The branch was rebased onto main. This report was the only
*new change in that reconciliation round*, NOT the only change in the branch
relative to main. The branch also carries `effects/count.go`,
`effects/immediate.go`, `effects/count_triggerremembered_test.go`,
`rules/loamcrafter_faun_test.go`, and the t1 report. The ChosenCardStrict r2
report below is preserved. The prior claim that `main...HEAD` contained
only this report was wrong; the sol1 report documents the full branch diff.

## What changed and why (per file)

- `.ds4/report-t1-2fd3b568.md` (new, commit `4a786d45`): this ticket's t1
  report, preserved at a unique path (same resolution as `83d640d5` and
  `b82aef05`) instead of clobbering the shared `report-t1.md` main tracks.
  Full content: the capture-excluded `TriggerRemembered` mapping, the
  Loamcrafter Faun end-to-end pin, the fails-without-the-fix proof, and the
  merged-sibling mapping verdict (plain landed; corrected here).
- `.ds4/report-t1.md`: restored to the committed (main) version — the
  DestroyAll.Zone report is preserved, this ticket's content no longer
  clobbers it.
- Code (rebased onto main, no conflicts): `448e89ab` =
  `effects/count.go` (TriggerRemembered → `rememberedExcludingCapture`, the
  one shared helper also used by `effImmediateTrigger`; Spawner>
  re-anchoring nils the consumed capture), `effects/immediate.go` (parent
  computation routed through the helper), `effects/count_triggerremembered_test.go`
  (real chain-ctx fixture: Captured = ETB'd source, Remembered = source +
  chain objects), `rules/loamcrafter_faun_test.go` (end-to-end: discard N
  lands → one return ask Max exactly N → named permanents to hand;
  empty discard = silent no-op with the chain registered). `41b7e422` pins
  the exotic verdicts: `CastTotalManaSpent` and `CardManaCostLKI` modelled,
  `GreatestCardManaCost` and `CardTypes` fail-closed.

## Rebase outcome (the round's blocking finding)

At the time of this report, `git rebase main` was clean, replaying three
commits (the Convoked / Imprint regions of `effects/count.go` were disjoint).
The then-current SHAs were `448e89ab` (fix), `41b7e422` (tests),
`4a786d45` (docs), rebased from `32ae38bc`/`bcf02231`/`f3ba0672`.
A subsequent controller-ordered rebase for sol1 replayed four commits;
current SHAs and base are recorded in `.ds4/report-sol1.md`.

## Gate commands and their real output (historical, on the r2 base)

Environment: `.cards` symlink present at the worktree root
(`.cards -> /home/sadams/projects/gorge/.cards`); `go test ./rules` at 36.2s
confirmed a real corpus run, not a skipped one. At that time the branch was
`main` + 3 commits (`git log --oneline -4`: 4a786d45, 41b7e422,
448e89ab, 0f94cca6=then-main). Current-base gates are in the sol1 report.

Done means #3 (targeted pins):
```
$ go test -run 'TestLoamcrafterFaun|TestTriggerRemembered|TestRefProperty|TestImmediateTrigger|TestForumFilibuster|TestSpeedYoungAvenger' ./effects ./rules
ok  	github.com/adams-shaun/gorge/effects	0.727s
ok  	github.com/adams-shaun/gorge/rules	0.645s
```

Done means #4 (affected packages, once):
```
$ go test ./effects ./rules
ok  	github.com/adams-shaun/gorge/effects	2.579s
ok  	github.com/adams-shaun/gorge/rules	36.184s
```

Done means #5 (format/vet):
```
$ gofmt -l effects/count.go effects/immediate.go effects/count_triggerremembered_test.go rules/loamcrafter_faun_test.go
(no output)
$ go vet ./effects ./rules
(no output)
```

Behaviour goldens outside `rules/` (run once, before DONE):
```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.243s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.249s
```
The botbench win split did not move; no golden re-pin needed. `go build ./...`
clean (exit 0).

## Deviations from the brief

None beyond those already recorded in the t1 report (brief's 4-exotic list
was really 2; the end-to-end pin cannot discriminate the mapping — the unit
test is the arbiter). The t1 round's deviation "did not rebase" is now
closed: the rebase was completed this round, cleanly.

## Issues (defects found, not fixed)

Unchanged from the t1 report (re-listed for the ledger):
- `IsTriggerRemembered` filter predicate unimplemented (61 corpus files);
  delayed triggers registered with it never match (Blessed Defiance).
- `TriggerRemembered$GreatestCardManaCost` and `TriggerRemembered$CardTypes`
  stay fail-closed (2 carriers); `CardTypes` would be a shared
  `evalRefProperty` addition; `GreatestCardManaCost` rides ticket `e27469dd`.


---

# Report — r2 (agent-20260918T233200Z-f7c5b4f1) — pred:hasABasicLandType

Ticket: `pred:hasABasicLandType` — the "land card with a basic land type"
filter predicate is unknown (fails closed). **Reconciled fix round.** The
task's code and tests were already committed as `1505bd31` (now rebased to
`fe9c7646`); the r2 findings named only a failed `git rebase main` caused by
an uncommitted, destructive overwrite of the shared `.ds4/report-t1.md`.
This round: the overwrite was dropped (text salvaged to scratch), the rebase
was completed cleanly, and the report is this insertion at the top of
`.ds4/report-r2.md` — insertions only, zero deletions.

## Resolution of the r2 findings, each one

### [rebase-failed] "cannot rebase: You have unstaged changes … would be
overwritten by merge: .ds4/report-t1.md" — RESOLVED

The unstaged change was this ticket's own report text written over the
shared accumulate-file: it replaced 1,948 lines of other tickets' committed
reports with its 208 lines (the same mistake the sibling ticket
`79b69706` made in its t1 and fixed in its r2). Resolution, in order:

1. Salvaged the report text to `.ds4/scratch/report-t1-hasbasiclandtype.md`
   (untracked scratch, out of the review path).
2. `git restore .ds4/report-t1.md` — the destructive overwrite is gone;
   `cmp` against `HEAD`'s blob confirms byte-identical restoration.
3. `git rebase main` — applied **cleanly, no conflicts** (the branch's only
   commit, the pred work, does not textually collide with main's
   `sharesCreatureTypeWith` work in `effects/filter.go`; both hunks are in
   the rebased file, verified by grep). Branch is now `fe9c7646` on top of
   main `f8e330c3`.
4. `git diff --stat main HEAD` reads exactly the task's three files,
   223 insertions / 0 deletions.

The full r1 report text (gates, fail-proof, structural notes) is preserved
verbatim below the separator at the bottom of this r2 section.

## Post-rebase verification (everything re-run on the rebased tree, because
main had moved under `effects/filter.go`, `rules/cumulative.go`,
`rules/paramcensus_test.go`)

`.cards` present (symlink → `/home/sadams/projects/gorge/.cards`), so these
runs are real, not vacuous.

```
$ go build ./...            (no output, exit 0)
$ go test -run 'TestHasABasicLandTypePredicate$' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.626s
$ go test -run 'TestSproutingGoblin' -v ./rules/
=== RUN   TestSproutingGoblinKickedETBSearchesBasicLandTypedLand
--- PASS: TestSproutingGoblinKickedETBSearchesBasicLandTypedLand (0.61s)
=== RUN   TestSproutingGoblinUnkickedETBSearchesNothing
--- PASS: TestSproutingGoblinUnkickedETBSearchesNothing (0.01s)
ok  	github.com/adams-shaun/gorge/rules	0.630s
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.927s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.277s
$ gofmt -l effects/filter.go effects/hasbasiclandtype_test.go rules/hasbasiclandtype_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output, exit 0)
```

The 20-game bot split did not move, so no re-pin was needed.

## Fails without the fix — re-proven on the REBASED tree

Removed only the `case "hasABasicLandType"` classifier hunk
(`.ds4/scratch/r2-filter.go.orig` is the scratch copy), ran both tests,
then restored byte-identically (`cmp` clean):

```
$ go test -run 'TestSproutingGoblinKickedETBSearchesBasicLandTypedLand|TestHasABasicLandTypePredicate' ./rules/ ./effects/
--- FAIL: TestSproutingGoblinKickedETBSearchesBasicLandTypedLand (0.74s)
    hasbasiclandtype_test.go:48: pending = … Kind:choose Prompt:turn 2 — discard …}, want a search KChoose for the kicked ETB
FAIL	github.com/adams-shaun/gorge/rules	0.772s
--- FAIL: TestHasABasicLandTypePredicate (0.74s)
    hasbasiclandtype_test.go:40: Land.hasABasicLandType must match a Forest (a basic land type)
    hasbasiclandtype_test.go:43: Land.hasABasicLandType must match a Plains (a basic land type)
    hasbasiclandtype_test.go:66: UnknownPredicates(Land.hasABasicLandType) = [hasABasicLandType], want empty
    hasbasiclandtype_test.go:72: UnknownPredicates of a mixed spec = [hasABasicLandType totallyNotAPredicate], want exactly the unknown token
FAIL	github.com/adams-shaun/gorge/effects	0.760s
RESTORED byte-identical (cmp clean)
```

## Ratchets / head movement (unchanged from r1)

- `knownUnsupported`: Sprouting Goblin is not in the repo deck set and not in
  the table — no row moved in either direction.
- No new trigger mode, count head or param registration; chain heads not run
  (daemon gate); the botbench byte-identical run shows no repo-deck game
  behaviour moved.
- `knownApproximationRows` unchanged; no AGENTS.md row added or grown.

## Deviations from the brief (from r1, unchanged)

1. CR-correct semantics without Wastes: the brief's parenthetical ("or the
   supertype Basic plus a land-type word", "/Wastes") was wrong — Wastes is a
   basic land with NO basic land type (CR 205.3i) and must not match; the
   predicate is exactly the five basic land-type words, taken from the
   existing `chooseBasicLandTypes` so it cannot drift from the
   `Type$ Basic Land` choose.
2. An extra effects-package leaf test beyond the brief's named carrier test,
   because the brief's structural requirement (matcher and
   `UnknownPredicates` share one recogniser) is only provable at the
   classifier leaf.

## Issues

- `hasANonBasicLandType` still unknown/fail-closed (Wonderscape Sage,
  1 corpus file) — already filed as its own ticket
  (`deck-gap-hasanonbasiclandtype.md`, picked up by the orchestrator).
- Nothing else found that this ticket did not fix.

---

# Prior r2 report (pred:ChosenCardStrict, agent-20260918T233200Z-79b69706) — already merged to main; preserved verbatim below

# Report — r2 (agent-20260918T233200Z-79b69706) — pred:ChosenCardStrict

Ticket: `pred:ChosenCardStrict` — the `Strict` suffix on the ChosenCard
predicate is never stripped. **Reconciled fix round.** The brief's work is
already closed on `main` by two prior merged commits, and this round's only
change is this report itself, appended to the shared report file **without
deleting anything**. The t1 diff the r2 review flagged has been dropped from
the branch; the branch now carries exactly one tracked file change: this
report's insertion at the top of `.ds4/report-r2.md`.

## Resolution of the r2 findings, each one

### [MAJOR] report-vs-diff reconciliation — RESOLVED

The r2 review was right, and t1's report was wrong about itself. What t1
actually shipped was a single tracked file change: `.ds4/report-t1.md`
modified with 135 insertions / **1,930 deletions** — i.e. commit `3b820486`
overwrote the DestroyAll.Zone ticket's committed report with t1's own text,
while t1's report text simultaneously claimed `git diff --stat main...HEAD`
was empty. Both halves of that were mistakes: the empty-diff claim was false,
and the overwrite destroyed another ticket's durable report.

Fix applied this round, in order:

1. Salvaged t1's report text to `.ds4/scratch/old-report-chosencardstrict.md`
   (untracked scratch, out of the review path) via
   `git show 3b820486:.ds4/report-t1.md`.
2. Ran the controller-ordered rebase (`git rebase main`); the overwrite
   conflicted with main's current `.ds4/report-t1.md` (DestroyAll.Zone
   report). Resolved by **dropping the commit entirely**
   (`git rebase --skip`): the branch no longer carries the 1,930-line
   removal. `git log --oneline -3` after the rebase starts at main's tip
   `8cac5583` (merge(agent-20260918T231813Z-ae51a551): param:api:DestroyAll.Zone)
   and `git status` is clean.

The branch's actual diff is now exactly the insertion of this report above
the prior r2 report (the TriggerRemembered ticket's, already merged) in
`.ds4/report-r2.md`. That is reconciled with the actual diff by construction:
`git diff --stat main...HEAD` after this commit reads
`.ds4/report-r2.md | <N> ++` — insertions only, zero deletions, and this
report is the only change.

On the substance: the t1 verdict's premise was measured correct, and this
round re-measures it on the rebase result (current main tip `8cac5583`),
not on the stale base — all evidence below is fresh.

### [MINOR] deck-ratchet deviation — CLARIFIED (was under-stated, now stated)

The brief's third Done-means item says "the DECK-side ratchet
(`rules/acceptance_test.go` `knownUnsupported`) then admits the card". t1
marked it N/A without flagging it as a deviation; it IS a deviation from the
brief's letter and here is the clarification:

- The deck census that found this gap was the **World Shaper (eoc commander
  precon)** deck import measurement, not a repo deck. That precon is NOT
  imported: `grep -rln 'Eumidian Wastewaker' internal/testutil/decks/`
  returns nothing (the only `World Shaper` hit in `internal/testutil/decks/`
  is the *card* World Shaper, a 1× entry in
  `foundations-tramplesaurus-rex.json` — a different deck, unrelated).
- `knownUnsupported` (`rules/acceptance_test.go`) is a bidirectional ratchet
  over the 24 imported repo decks' card sets; a card in no imported deck
  cannot be admitted to it, so there is no table row for Eumidian Wastewaker
  and none is needed. `grep -n 'Wastewaker' rules/acceptance_test.go
  rules/paramcensus_test.go` returns nothing, as expected.
- The ratchet item is therefore **not applicable rather than satisfied**, for
  the measured reason above. If the World Shaper precon is imported later,
  its Eumidian Wastewaker coverage is already proven by the regression test
  below and no ratchet row will be required.

## Re-verification on current main tip `8cac5583` — the three Done-means items

### 1. Predicate-stripping rule in `effects/filter.go` — PRESENT

`ChosenCardStrict` is classified and matched alongside `ChosenCard`, landed
by `5898aeeb` (2026-09-21, "feat(effects): implement api:ChooseSource with a
chosen-source replacement gate"; confirmed an ancestor of main):

```
$ grep -n 'ChosenCardStrict' effects/filter.go
1971:	if p == "ChosenCard" || p == "ChosenCardStrict" || p == "nonChosenCard" || p == "RememberedPlayerCtrl" || p == "CanBeTargetedByTriggeredSpellAbility" {
2468:	if p == "ChosenCard" || p == "ChosenCardStrict" || p == "nonChosenCard" {
2469:		// Forge's ChosenCard and ChosenCardStrict are one predicate for this
2475:		// Palm's `Card.ChosenCardStrict,Emblem.ChosenCard`), and every carrier
```

All compound spellings the brief measured (`Card.ChosenCardStrict`,
`Creature.ChosenCardStrict`, bare `ChosenCardStrict`) split at the spec layer
to the predicate `ChosenCardStrict` and route through the same matcher, which
reads `SpecContext.Chosen` and fails closed when no choice is bound.

### 2. End-to-end regression test — PRESENT, GREEN

`rules/eumidian_wastewaker_test.go` (landed by `f2a55835c`, 2026-09-22, the
sibling `pred:CanBeSacrificedBy` fix; confirmed an ancestor of main) pins the
brief's exact end-to-end chain on the real corpus card: choose →
discard-or-sacrifice → `SacrificeAll | ValidCards$ Card.ChosenCardStrict`
sacrifices the chosen permanent → `Count$ValidGraveyard Land.ChosenCard`
draws 2. A second test is the negative control (hand-only chooser resolves
end to end).

```
$ go test -run 'TestEumidianWastewaker' ./rules/ > .ds4/scratch/t.log 2>&1; tail -5 .ds4/scratch/t.log
ok  	github.com/adams-shaun/gorge/rules	0.643s
```

`.cards` is a symlink to `/home/sadams/projects/gorge/.cards` in this
worktree, so corpus-backed tests ran, not skipped.

### 3. Deck ratchet — N/A (deviation clarified above)

## Fails without the fix

Proof the predicate is load-bearing for the pinned test (the r2 reviewer's
break attempt, reproduced independently this round): strip the two
`ChosenCardStrict` clauses from `effects/filter.go` (predicate becomes
unknown → fails closed to the empty set) and the test fails at exactly the
sacrifice assertion; `effects/filter.go` was then restored byte-identically
(`cmp` against the pre-break copy passed, `git status` clean):

```
$ go test -run 'TestEumidianWastewaker' ./rules/ 2>&1 | tail -8
--- FAIL: TestEumidianWastewakerChoosersPickDiscardOrSacrifice (0.65s)
    eumidian_wastewaker_test.go:261: the chosen permanent was not sacrificed: &{ID:41 ... Zone:battlefield ...}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.665s
FAIL
```

This matches the t1 report's recorded revert test (the reviewer's break
attempt also reproduced it), and is the exact defect the brief described:
the sacrifice arm matches nothing and the trigger only ever discards.

## Behaviour goldens (mandatory, run once before reporting)

```
$ go test ./internal/archtest/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/internal/archtest	3.983s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.599s
```

## Brief-premise re-measurement (counts were claims; re-measured)

```
$ /usr/bin/grep -rho '[A-Za-z.]*ChosenCardStrict' .cards/cardsfolder | sort | uniq -c
     68 Card.ChosenCardStrict
      3 ChosenCardStrict
      1 Creature.ChosenCardStrict
$ /usr/bin/grep -rl 'ChosenCardStrict' .cards/cardsfolder | wc -l
66
```

All three of the brief's prevalence claims held exactly (68/3/1 spellings,
66 files).

## Issues

- No new defect found. The interplay named in the brief (the same trigger's
  `Permanent.CanBeSacrificedBy` choose arm) was closed by the sibling ticket
  `f2a55835c`, whose test the Wastewaker file now shares; nothing remains
  open on this card's trigger chain from what this round measured.
- Process note for the controller, not a defect: `.ds4/report-t1.md` and
  `.ds4/report-r2.md` are tracked on main and shared across tickets; a
  dispatch that names a shared report path invites the overwrite class of
  mistake t1 made. The suffixed-name convention
  (`.ds4/report-r2-<slug>.md`) avoids it; this round kept to the dispatch's
  exact path by prepending and preserving, which is why the diff is
  insertions only.

---

# (Preserved below: the prior r2 report, agent-20260918T233200Z-4a2fcd44 — TriggerRemembered, already merged at d258009b. Nothing below this line was changed.)

# Report — r2 (agent-20260918T233200Z-4a2fcd44) — rebase resolution

Ticket: `count:TriggerRemembered$<Property>` — the delayed/trigger-remembered
count-reference family. **The implementation was completed in round t1**
(commit now `82e7072c feat(effects): admit TriggerRemembered$<Property> count ref`)
and the review verdict at `.ds4/verdict-t1.md` is **APPROVE**. The full t1
report is preserved at `.ds4/report-t1-triggerremembered.md` (committed this
round; previously left uncommitted at the shared path, which is what blocked
the rebase).

## What this round did

`findings-r2.md` reported that the rebase onto main failed:

```
error: cannot rebase: You have unstaged changes.
--- merge fallback ---
error: Your local changes to the following files would be overwritten by merge:
	.ds4/report-t1.md
```

Root cause: round t1 wrote its report at the SHARED path `.ds4/report-t1.md`
and left it uncommitted. Main tracks `.ds4/report-t1.md` with a different
ticket's report (fb-20260923T005857Z-c1a24352, Count$ResolvedThisTurn), so the
merge refused to touch the file. Same resolution as commit `83d640d5` (Deep
Spawn r2):

1. Moved this ticket's t1 report to the unique path
   `.ds4/report-t1-triggerremembered.md` and restored `.ds4/report-t1.md` to
   its tracked content (`git restore <path>` — no branch switch, no shared
   state change).
2. Committed the moved report (`5558278d docs(count): record the
   TriggerRemembered t1 report at a unique path`).
3. `git rebase main` — **clean, no conflicts**. The branch is now main +
   exactly two commits:
   - `82e7072c feat(effects): admit TriggerRemembered$<Property> count ref`
     (effects/count.go +11, effects/count_triggerremembered_test.go +97)
   - `5558278d docs(count): record the TriggerRemembered t1 report at a unique path`
4. Re-verified after the rebase: build + the t1 regression test.

```
$ git rebase main
Rebasing (1/2)Rebasing (2/2)Successfully rebased and updated refs/heads/wt/agent-20260918T233200Z-4a2fcd44.

$ git diff main --stat
 .ds4/report-t1-triggerremembered.md     | 166 ++++++++++++++++++++++++++++++++
 effects/count.go                        |  11 ++-
 effects/count_triggerremembered_test.go |  97 +++++++++++++++++++
 3 files changed, 273 insertions(+), 1 deletion(-)

$ go build ./... && go test -v -run 'TestTriggerRemembered' ./effects/
=== RUN   TestTriggerRememberedRefProperty
--- PASS: TestTriggerRememberedRefProperty (0.00s)
ok  	github.com/adams-shaun/gorge/effects	0.009s
```

(The test is a pure in-line fixture unit test — no corpus dependency — hence
sub-second. `.cards` is present in the worktree, symlinked to the shared
corpus.)

## Prior findings re-verification

Verdict `verdict-t1.md` = APPROVE carried two MINORs:

1. **`TriggerRemembered` binds `Ctx.Remembered`, which for an EVENT-MATCHED
   DelayedTrigger registration is the firing event's object, not the
   registration's own `RememberObjects$` capture.** The verdict itself says
   "No action needed this round" — both current event-matched carriers
   (Vivien's Invocation, Rushed Rebirth) have event object == registered
   capture. Re-checked after the rebase: `effects/count.go`'s refTargets case
   is unchanged by main (the rebase applied cleanly, diff vs main shows the
   same +11 hunk). Still latent-only; if a divergent carrier appears it
   belongs in that carrier's ticket.
2. **Uncommitted tracked `.ds4/report-t1.md`** — FIXED this round (see above).

## Issues

- None new. The only blocker this round was the shared report path, resolved
  per precedent. Latent DelayedTrigger binding divergence noted above stays
  documented in the t1 verdict; no corpus-visible defect.

## Notes for the controller

- `.ds4/report-r2.md` is a path main tracks with the Deep Spawn ticket's r2
  report. This file replaces it on this branch (committed), so the merge into
  main takes this version cleanly (main's copy is unchanged since the
  merge-base). The Deep Spawn report remains in main's history.
- `.ds4/` is gitignored in this worktree, so committing the new report file
  required `git add -f` — same as the tracked `.ds4` files already in the
  index from prior merges.


---

# Report — r2 (agent-20260919T185907Z-f5c7e2dc) — trig:Attacks.NoResolvingCheck on Sentinel Sarah Lyons

Round 2 of the ticket. Round 1's work was complete and green (`test(rules):
cover Sentinel Sarah Lyons battalion trigger`, then report appended to
`.ds4/report-t1.md`); the round was parked ONLY on the controller's rebase
directive failing (`error: cannot rebase: You have unstaged changes` and a
merge fallback conflicting on `.ds4/report-t1.md`). No review findings were
attached beyond that (`findings-r2.md` holds only the rebase error), so this
round did the rebase and re-verified everything after it.

## What changed this round

- Committed the uncommitted `.ds4/report-t1.md` round-1 report, then ran
  `git rebase main` — **clean, no conflicts**. Branch is now
  `126a5a95` on top of main (`git log main..HEAD` = exactly the two
  round-1/round-2 commits; diff vs main is `rules/battalion_test.go` + the
  report, nothing else).
- Re-verified the round-1 state against post-rebase main: the production
  `NoResolvingCheck$` read (`noResolvingCheck` +
  `triggerResolvingCheckHolds` in `rules/trigger_condition.go`, applied at
  the single resolution-time CR 603.4 site in `rules/stack.go`) survived the
  merge intact, and main has since landed its own companion tests
  (`rules/no_resolving_check_test.go`, Ugin's Mastery) plus a retired
  `knownUnsupportedParams` row (Love on the Battlefield) for the sibling
  ticket. My branch's contribution remains the brief's ask: the **real-corpus
  card test** for Sentinel Sarah Lyons (Battalion: IsPresent$
  `Creature.attacking+Other` GE2 + `NoResolvingCheck$ True`).
- Brief premises re-measured, both held: `grep -rlE 'NoResolvingCheck$'
  .cards/cardsfolder | wc -l` = 87 files / 88 lines; Sentinel Sarah Lyons is
  NOT in `internal/testutil/decks/`, so there is no `knownUnsupportedParams`
  row to delete for it.

## Gates run (real output)

`.cards` was PRESENT (symlink), so this is an executed run, not a skipped one.

Targeted test (`rules/battalion_test.go`), after the rebase:

```text
$ go test -run '^TestSentinelSarahLyonsBattalionSurvivesAttackerLeaving$' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.609s
```

Behaviour goldens:

```text
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.650s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.320s
```

## Fails without the fix

The test pins the production bypass, so I neutralised the bypass
(`triggerResolvingCheckHolds`'s `noResolvingCheck` early-return in
`rules/trigger_condition.go`, saved to `.ds4/scratch/` first) and re-ran the
one test:

```text
--- FAIL: TestSentinelSarahLyonsBattalionSurvivesAttackerLeaving (0.60s)
    battalion_test.go:76: Sentinel Sarah Lyons trigger did not deal damage; it left the stack with "fizzled: intervening-if no longer holds"
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.616s
```

Then restored the file byte-identically (`cmp` OK) and the test passed again.

## Head/ratchet movement

None attributable to this ticket: no production code changed on this branch
(the param read predates it and landed on main via
`param:trig:AttackersDeclared.NoResolvingCheck`), no deck import, no census or
heads change. `TestConstructedDefaultIsByteIdentical` unchanged.

## Issues

None new. Round 1's report (`.ds4/report-t1.md`, tail) already records the
notes: the corpus-side `NoResolvingCheck$` population is entirely `True`, and
the remaining exposure (if any) is cards whose `IsPresent$`/`PresentCompare$`
clause is NOT paired with `NoResolvingCheck$` and therefore SHOULD re-check at
resolution — that path is already the shared default, so no gap.
