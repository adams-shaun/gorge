# Merge-conflict resolution — cli-20260923T060000Z-ctms-tag

## State found

`git status` was CLEAN — no rebase or merge was in flight (the daemon's failed
attempt had rolled back). The branch carried the two approved commits
(`d1c2d6f2` fix + `17c1c42d` review-proof) on top of base `62ae4746`; main
(`b88ad124`) was ahead. I reproduced the integration with `git merge main`,
which hit exactly the one conflict the daemon saw (`effects/misc.go`), resolved
it, and committed the merge as `c0dccbed`.

## Conflicted file: effects/misc.go (one hunk, in `effMana`)

What each side wanted:

- **Branch (d1c2d6f2, reviewed fix):** replaced the inline snow/typed-mana tag
  read (walk `state.TypedManaTags` over the source face's types) with the one
  shared helper `tag, snow := ManaProducerTag(h, c.Source)` — the point of the
  fix is that effMana, effManaReflected and rules/cumulative.go's AddMana all
  read the tag through ONE helper. The branch kept the base's old
  `TriggersWhenSpent$` comment block ("UNRESTRICTED provenance batch …
  restriction encoding wins").
- **Main (82d3ba68 "fix(rules): fire mana-spent riders on ability
  activations"):** kept the inline block byte-for-byte but replaced that same
  old `TriggersWhenSpent$` comment with a newer one ("retained alongside any
  spend restriction … the spend path dispatches the named rider after the
  payment completes"). Main's real change in that commit is rules-side
  (rules/cast.go, rules/stack.go — SpellAbilityCast riders, ability-activation
  capture); those files merged cleanly.

Resolution — both intents are independent and compatible:

- Kept the **branch's** `tag, snow := ManaProducerTag(h, c.Source)`: the
  reviewed fix, load-bearing for the cleanly-merged companions
  (`effects/mana_reflected.go:265`, `rules/cumulative.go:612`,
  `state/ids.go`). Dropping it would have broken the fix's own tests.
- Kept **main's** newer `TriggersWhenSpent$` comment verbatim: it is main's
  later deliberate change to those lines, and the code below the conflict
  (`provenanceOnly := triggersWhenSpent != "" && restriction == "" &&
  noCounter == ""`) is identical on both sides, so the comment is the only
  divergence there.

The inline main-side block was deleted as part of the resolution — it is the
code the reviewed fix deliberately extracted into the helper, not an
independent main change (verified: `git show 62ae4746:effects/misc.go` shows
the base already had it, and the 62ae4746→main diff never touches it).

## Commands run (real output)

- `git merge main` → `CONFLICT (content): Merge conflict in effects/misc.go`
  (the only conflicted file; `git diff --name-only --diff-filter=U` listed
  just it).
- `gofmt -l effects/misc.go` → clean; `go build ./effects/ ./rules/` → clean;
  no conflict markers remain.
- `git commit --no-edit` → merge `c0dccbed`; `git status --short` → clean.
- Ratchets (brief's command, widened with the branch's own new tests):
  `go test ./rules -run 'TestManaReflectedProducerTagsItsSourceType|TestCumulativeUpkeepAddManaTagsItsSourceType|TestCastTotalManaSpentGrammarCountsModelledAndFailsClosed|TestManaReflectedCaveSourceTagsItsReflectedMana|TestManaReflectedAttributionIsTheAbilitySourceNotTheReflectedSet|TestProducedShapeTagsItsSourceNotTheReflectedLand|TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.857s`. Corpus was present
  (`.cards` symlink found at worktree creation): cross-checked by running
  `TestEveryRepoDeckIsFullySupported -v`, which read **979 distinct repo-deck
  cards** and reports "4 of 979 … not fully supported" PASS — that standing is
  main's merged state (the deck set grew since the AGENTS.md 3-of-791 note),
  not something this resolution moved.
- `go test ./effects/` → `ok … 2.694s` (the edited package, once).
- Behaviour goldens: `go test ./internal/archtest/` → `ok … 4.264s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok … 1.279s` — the pinned bot split did NOT move, so the merge changed no
  repo-deck behaviour the bench exercises.

## Unsure about / notes

- Main's newer comment says TriggersWhenSpent is "retained alongside any spend
  restriction", but the shared `provenanceOnly` line still requires
  `restriction == ""` — the comment slightly overstates the code, on MAIN's
  side, unchanged by me (integration, not redesign).
- No registry/ratchet table edits were needed: the branch registers no new
  `Mode$` matcher and closes no ratchet row (its commit message explicitly
  leaves the castfilter1/2 AGENTS.md row to the sibling ctms-refhead ticket).

## Issues

None new. The merge itself surfaced no defect.
