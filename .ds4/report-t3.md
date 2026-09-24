# Report — agent-20260923T065617Z-9b7a6efa (fix round t3): restore shared report history

## Outcome

`STATUS=DONE`. The review's one MAJOR (`.ds4/report-t1.md` replaced the
accumulated shared report with this ticket's report, deleting thousands of
unrelated lines) is FIXED: `.ds4/report-t1.md` is restored from `main`
byte-exact and this ticket's round-t1 report is prepended above a separator,
so the diff against `main` is additions only — **257 insertions, 0 deletions**.

The review's MINOR (World permanents grouped by printed name) was fixed in the
previous round (`b902ed0d`, `worldGroupsByNameFree`) and re-verified here;
the new regression `TestWorldRuleGroupsByNameFree` still fails with that fix
reverted (pasted below).

Commit: `b902ed0d` is the code fix; this round's commit is the report/docs
restore.

## Finding dispositions

- **[MAJOR] `.ds4/report-t1.md` replaced, history lost** — FIXED.
  `git show main:.ds4/report-t1.md` is 3429 lines and is exactly the content
  `bbf4f6432^` held. `.ds4/report-t1.md` is now:

  ```
  <this ticket's round-t1 report, 252 lines>
  ---
  # Reports appended below are from other tickets on the shared report file (preserved verbatim from main):
  <main's 3429 lines, byte-exact>
  ```

  Verified: `tail -n 3429 .ds4/report-t1.md | sha256sum` ==
  `git show main:.ds4/report-t1.md | sha256sum` ==
  `c2e4f27ae578dcd06830860cfb734fc525767d364e0715f52f94458b6c79f0df`, and
  `git diff --numstat main -- .ds4/report-t1.md` → `257	0`. Follows the
  established preserve convention (model commits `6bc24448e`, `472d095d`).

  `.ds4/report-t2.md` was already correct (round t2 was APPENDED, not
  replaced: `git diff --numstat main -- .ds4/report-t2.md` → additions only).
  One sentence in that report claimed the 252-line `report-t1.md` was
  "preserved verbatim", which was false when written; it is corrected to point
  at the t3 restore.

- **[MINOR] `duplicateGroups` grouped World permanents by printed name** —
  was FIXED in round t2 (`b902ed0d`). Re-verified this round: reverting the
  dispatch makes `TestWorldRuleGroupsByNameFree` FAIL (pasted below).

## What changed and why (per file)

- **`.ds4/report-t1.md`** — restored main's full shared history byte-exact,
  with this ticket's round-t1 report prepended above the standard separator +
  "preserved verbatim from main" marker.
- **`.ds4/report-t2.md`** — one sentence corrected (see above); the round-t2
  report body otherwise unchanged. Diff vs main remains additions only.
- **`.ds4/report-t3.md`** — this file.
- No Go source, test or `cards/` change in this round. The t2 code fix
  (`rules/sba.go` `worldGroupsByNameFree`) is untouched and verified.

## No other drive-by change

`git status` this round shows only `.ds4/report-t1.md` and
`.ds4/report-t2.md` modified (plus this new report). No code hunk, no test
hunk, no AGENTS.md edit.

## Gates — exact commands and real output

Targeted tests (brief's Done-means pattern; legend + world, corpus present):

```
$ go test -run 'TestWorldRule|TestWorldAndLegend|TestLegend' -v ./rules/ 2>&1 | tail
--- PASS: TestLegendRuleAsksControllerWhichDuplicateToKeep (0.00s)
--- PASS: TestLegendRuleSettlesEachDuplicateSetInTurn (0.00s)
--- PASS: TestLegendRuleKeptSurvivorKeepsLethalDamagePath (0.00s)
--- PASS: TestLegendRuleBotAnswerKeepsBattlefieldOrderFirst (0.00s)
--- PASS: TestLegendRuleSurvivesFirstStrikeCombatTail (0.00s)
--- PASS: TestLegendRuleSurvivesRegularCombatTail (0.00s)
--- PASS: TestLegendRuleReposesDisplacedAsk (0.00s)
--- PASS: TestLegendRuleDepartedControllerDeclines (0.00s)
--- PASS: TestLegendarySpacecraftCommanderLegalityFollowsPTBox (0.46s)
--- PASS: TestLegendarySpacecraftWithPTIsSeatedAsCommander (0.00s)
--- PASS: TestWorldRuleGroupsByNameFree (0.00s)
--- PASS: TestWorldRuleAsksControllerWhichDuplicateToKeep (0.00s)
--- PASS: TestWorldRuleBotAnswerKeepsBattlefieldOrderFirst (0.00s)
--- PASS: TestWorldRuleIsPerController (0.00s)
--- PASS: TestWorldRuleSinglePermanentPosesNothing (0.00s)
--- PASS: TestWorldRuleReadsDerivedSupertype (0.00s)
--- PASS: TestWorldAndLegendRulesAreOrthogonal (0.00s)
ok  	github.com/adams-shaun/gorge/rules	0.506s
```

`internal/archtest` (no allowlist edits):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)
```

Byte-identical botbench split:

```
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)
```

Format + generated types:

```
$ gofmt -l cards/face.go rules/sba.go rules/layers.go rules/turn.go rules/engine.go rules/clone.go rules/sbaquiet.go rules/world_rule_test.go rules/legend_sba_combat_test.go
(no output)

$ go run ./cmd/gentypes -check
(no output, exit 0)
```

`.cards` was ALREADY present as a symlink to
`/home/sadams/projects/gorge/.cards` (checked first), so the corpus tests ran
rather than skipped.

## Fails without the fix

The t2 code fix is what makes the round-t2 regression fail. Reverted just the
dispatch in `rules/sba.go` (`if rule == sbaWorld { return
e.worldGroupsByNameFree() }` removed, so the world half falls into the
name-keyed legend body), ran the test, then restored `rules/sba.go` from
`.ds4/scratch/sba.go.bak` (`cmp` clean):

```
$ go test -run 'TestWorldRuleGroupsByNameFree' ./rules/
--- FAIL: TestWorldRuleGroupsByNameFree (0.00s)
    world_rule_test.go:130: no decision pending: the world rule did not ask its controller
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.004s
FAIL
```

Restore verified: `cmp rules/sba.go .ds4/scratch/sba.go.bak` → clean, and
`git status --short` shows `rules/sba.go` unmodified. The full round-t1
"fails without the fix" output (all six world tests) is in
`.ds4/report-t1.md`'s `## Fails without the fix` section.

## Head / ratchet movement

None. This round changed no Go source. The t2 fix and the round-1
implementation are untouched, and no repo deck carries a World card
(`/usr/bin/grep -rlE "Types:.*World" .cards/cardsfolder | wc -l` = 26; no
hit across `internal/testutil/decks/*.json`), so TestHeads / acceptance /
param ratchets should not move. Daemon runs those at the merge gate.

## Issues (found, not fixed)

None new this round. The round-t1 report's `## Issues` section (the layer-4
supertype-removal grammar gap, the `Config.Commanders` same-name de-dup
fixture trap) still stands and is preserved in `.ds4/report-t1.md`.

## Open concerns

None blocking. The reviewer should confirm
`git diff --numstat main -- .ds4/report-t1.md` shows `0` deletions, which is
the exact failure the MAJOR named.
