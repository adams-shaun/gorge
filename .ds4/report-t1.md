# Report — agent-20260923T065617Z-9b7a6efa: CR 704.5k world-rule SBA

## Outcome

`STATUS=DONE`. The world rule (CR 704.5k) is implemented, sharing the legend
rule's (CR 704.5j) one parked-batch controller-choice channel. Six new tests
in `rules/world_rule_test.go` cover it; every one fails with the non-test fix
reverted. All brief gates pass.

Commit: `26f2e09d0fdc25ba9a5fe368364dc2be20bf7d56` on branch
`wt/agent-20260923T065617Z-9b7a6efa` (base `374a568b`).

Workspace facts: `.cards` symlink was ALREADY present in this worktree
(`ls -la .cards` → `… -> /home/sadams/projects/gorge/.cards`), so the
corpus-backed tests ran rather than skipped. `.ds4/` contents were present.

## What changed and why (per file)

- **`cards/face.go`** — added `func (f *Face) IsWorld() bool { return
  f.hasType("World") }` beside `IsLegendary()`. `TypeWorld` and
  `typeMaskFor("World")` already existed; only the accessor was missing.

- **`rules/sba.go`** — the bulk.
  - Renamed `legendGroup` → `sbaGroup` (it now describes a set for either
    rule) and gave `legendBatch` a `rule sbaRule` field (`sbaLegend` /
    `sbaWorld`). The rule derives the replay-visible strings: `keepCounter()`
    (`"legend_keep"` / `"world_keep"`) and `departureText()` (`"legend rule"` /
    `"world rule"`).
  - `parkLegendChoice` → `parkSBAChoice(rule, group, dead)`,
    `askLegendChoice` → `askSBAChoice`, `legendAnswer` → `sbaAnswer`,
    `applyLegendBatch` → `applySBABatch`. ONE park/ask/CR 800.4a-decline/
    settle path for both rules; the batch carries which rule parked it. The
    ask prompt is intentionally rule-neutral (it names the duplicate's own
    name, which is what the seat chooses between), so legend asks are
    byte-identical to before.
  - `legendGroups()` / `worldGroups()` are thin wrappers over the new
    `duplicateGroups(rule)` — the ONE duplicate-set scan (per controller,
    same printed name, ≥2 members, battlefield scan order, deterministic;
    membership maps never iterated).
    - The legend half keeps its printed-`IsLegendary()` pre-filter and its
      `IgnoreLegendRule` exemption, unchanged.
    - The world half reads the DERIVED type list alone. There is no
      `IgnoreWorldRule` static in the corpus (measured:
      `/usr/bin/grep -rn IgnoreWorldRule .cards/cardsfolder` → nothing), so
      none is collected.
  - `sbaSupertypeUnderLayers(e, id, rule)` is the shared derived-type read;
    `legendaryUnderLayers` / `worldUnderLayers` are pinned-rule wrappers.
  - `destroyLethalDamage`: after the legend park, the world set is parked on
    the same atomic channel (`parkSBAChoice(sbaWorld, groups[0], dead)`);
    when a decision is outstanding it defers (`e.sbaUnquiet = true`) rather
    than applying under the ask, exactly as the legend half does. A pass that
    gathers both parks the legend set first; the legend answer's Submit tail
    re-scans and parks the world set.

- **`rules/layers.go`** — extracted `anyLayer4TypeEffect()` out of
  `typeCharacteristics`' existing fast-path scan (behaviour identical) so the
  duplicate scan can tell when the derived list may differ from the printed
  one. With no layer-4 type effect active, `typeCharacteristics` returns the
  printed list, so a printed-World pre-filter is exact; only when a layer-4
  effect is live does the world scan read every permanent derived-side.

- **`rules/turn.go`, `rules/engine.go`, `rules/clone.go`,
  `rules/sbaquiet.go`** — generalized the flow-marker dispatch
  (`e.sbaAnswer`), the `Engine.legendBatch` comment, the Clone deep-copy
  comment and the quiet-skip comments to say "duplicate-permanent" rather
  than "legend". `legendBatch`'s field name is kept (brief: one home; clone
  and sbaquiet already key on it).

- **`rules/legend_sba_combat_test.go`** — one-line rename
  `e.askLegendChoice()` → `e.askSBAChoice()` (the departed-controller test
  calls the ask directly). No assertion changed.

- **`rules/world_rule_test.go`** (new) — the six tests.

## Deviations from the brief (with reasons)

1. **The brief's layer-strip test premise is false.** The brief says a World
   supertype can be "STRIPPED by a layer-4 static (the `RemoveCardTypes$` /
   `AddTypes$` shape `fortifySBALandStripper` uses)". It cannot:
   `RemoveCardTypes$` keeps supertypes (`rules/layers.go` around the
   `ce.RemoveCardTypes` block: it keeps `isSupertype(t)`), and `World` is in
   `supertypeWords` (`rules/layers.go`). The only supertype-specific stripper
   is `NonLegendary$`/`RemoveLegendary`, which drops Legendary only. So no
   grammar in this build removes World.
   I re-measured this rather than assuming: `supertypeWords =
   {"Basic","Legendary","Ongoing","Snow","World"}`, `RemoveCardTypes` keeps
   exactly those, and `RemoveLegendary` drops only `"Legendary"`.
   Consequence: the layer test is written in the ADD direction instead —
   two same-named non-World enchantments made World by `AddTypes$ World`. The
   test asserts the printed faces are NOT World while the derived lists ARE,
   so a printed-face-only scan cannot pass. **To keep both directions
   correct** (and because the brief's core demand is "read the derived list,
   not the printed face"), the world scan is derived-authoritative: with a
   layer-4 effect live it never trusts the printed word, so a future
   World-stripper would work too. The legend scan is untouched.

2. **The brief suggested a pass-loop `worldRuleStep` in `checkStateBased`.**
   I instead put the world park inside `destroyLethalDamage`, immediately
   after the legend park. This is the cleaner integration: it shares the
   same-pass `dead` slice and the same atomic park/ask, which is exactly what
   the brief said to carry ("carrying the same-pass lethal `dead` the same
   way"). A pass that gathers both rules parks the legend set first and the
   world set on the next scan, which the orthogonality test pins.

Nothing else deviates. No `events.Kind`/`events.Event` change; `"world_keep"`
and `"world rule"` are additive string values on existing kinds. No
`IgnoreWorldRule` static. No `web/` change. No AGENTS.md "Known
approximations" edit (there was no row for the world rule; nothing to
delete). No row added or grown.

## Gates — exact commands and real output

Targeted tests (brief's Done-means pattern, widened to cover the new tests),
with the corpus present:

```
$ go test -run 'TestWorldRule|TestWorldAndLegend|TestLegend' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.467s
```

`internal/archtest` (no allowlist edits):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	2.678s
```

Byte-identical botbench split (passes unchanged, as expected — no repo deck
carries a World card):

```
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.635s
```

Format + generated types:

```
$ gofmt -l cards/face.go rules/sba.go rules/layers.go rules/turn.go rules/engine.go rules/clone.go rules/sbaquiet.go rules/world_rule_test.go rules/legend_sba_combat_test.go
(no output)

$ go run ./cmd/gentypes -check
(no output, exit 0)
```

Also ran (targeted, per budget): `go test -run
'TestWorldRule|TestWorldAndLegend|TestLegend|TestIgnoreLegendRule|TestSBA|TestFortification'
./rules/` → `ok … 0.491s`. Daemon gates (TestHeads, `make sim`, acceptance
ratchet, `go vet ./...`, `make report`, CR conformance) deliberately not run
in the seat.

## Fails without the fix

Reverted the non-test change (disabled the world block in
`destroyLethalDamage`: `if groups := e.worldGroups(); false && len(groups) > 0`),
ran the new tests, restored `rules/sba.go` byte-identically (`cmp` clean):

```
$ go test -run 'TestWorldRule|TestWorldAndLegend' ./rules/
--- FAIL: TestWorldRuleAsksControllerWhichDuplicateToKeep (0.00s)
    world_rule_test.go:110: no decision pending: the world rule did not ask its controller
--- FAIL: TestWorldRuleBotAnswerKeepsBattlefieldOrderFirst (0.00s)
    world_rule_test.go:144: no decision pending: the world rule did not ask its controller
--- FAIL: TestWorldRuleIsPerController (0.00s)
    world_rule_test.go:199: no decision pending: the world rule did not ask its controller
--- FAIL: TestWorldRuleSinglePermanentPosesNothing (0.01s)
    world_rule_test.go:247: no decision pending: the world rule did not ask its controller
--- FAIL: TestWorldRuleReadsDerivedSupertype (0.00s)
    world_rule_test.go:284: no decision pending: the world rule did not ask its controller
--- FAIL: TestWorldAndLegendRulesAreOrthogonal (0.00s)
    world_rule_test.go:326: pending decision priority Min 1 Max 1, want a KChoose Min 1 Max 1
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.026s
FAIL
```

All six fail. The two "nothing happens" tests (`…IsPerController`,
`…SinglePermanentPosesNothing`) carry positive controls inside the same test
(seat 0 IS asked when it controls a duplicate; a second copy makes the lone
permanent's ask arrive), so they fail too — they cannot pass with the whole
rule unregistered. Restored file verified with `cmp rules/sba.go
.ds4/scratch/sba.go.bak` → identical, and every other changed file `cmp`-clean
against its backup.

Each test additionally asserts its own precondition: objects on
`ZBattlefield`, `IsWorld()` on the printed face, names equal, controllers
distinct where scoping is claimed, and (layer test) printed faces NOT World
while derived lists ARE World.

## Test list (new tests added)

`rules/world_rule_test.go`:
1. `TestWorldRuleAsksControllerWhichDuplicateToKeep`
2. `TestWorldRuleBotAnswerKeepsBattlefieldOrderFirst`
3. `TestWorldRuleIsPerController`
4. `TestWorldRuleSinglePermanentPosesNothing`
5. `TestWorldRuleReadsDerivedSupertype`
6. `TestWorldAndLegendRulesAreOrthogonal`

## Head / ratchet movement

None expected and none observed through the gates I ran. No repo deck carries
a World card (`/usr/bin/grep -rlE "Types:.*World" .cards/cardsfolder | wc -l`
measured **26**, and no hit across `internal/testutil/decks/*.json`), so
chain heads and the acceptance/param ratchets should not move; the daemon
runs TestHeads at the merge gate. `cmd/botbench`'s pinned split is unchanged.
If TestHeads moves, it is not attributable to this change and should be
bisected (none of the 26 World cards is in a repo deck, measured now).

## Issues (found, not fixed)

1. **No layer-4 grammar can remove the World supertype.** The brief assumed
   `RemoveCardTypes$` strips it; it keeps all supertypes, and there is no
   `RemoveWorld`-style parameter. Impact: a hypothetical layer-4 "your World
   permanents aren't world permanents" effect would be read by
   `typeCharacteristics` (it would drop World from the derived list) and the
   world scan would then correctly not group them — so the engine is correct;
   the gap is purely that the test cannot exercise the strip direction with
   current card text. No corpus carrier exists. If a future Forge/gorge
   grammar adds supertype removal, `TestWorldRuleReadsDerivedSupertype` can
   add a strip-direction case with no engine change. Not ledgible to AGENTS.md
   (frozen, and it is not debt).

2. **`Face.IsWorld()` has no rules-side caller outside the derived-type
   string read** — `worldUnderLayers` compares the string `"World"` (shared
   with the generic `sbaSupertypeUnderLayers`), and the world pre-filter uses
   `o.Face().IsWorld()`. So `IsWorld()` IS used (the perf pre-filter), but if
   a reviewer looks for a `worldUnderLayers` call to `IsWorld()`, it is not
   there by design. Not a defect; noted for clarity.

3. **`Config.Commanders` de-duplicates same-named commanders.** While
   building the per-controller test I found that
   `commanderGame(t, seed, FormatConstructed, 40, [][]string{{src, src}, …})`
   leaves only ONE entry in seat 0's command zone for two identical card
   sources (`index out of range [1] with length 1`). Not part of this ticket
   and not a rules bug (a Commander deck cannot legally run two same-named
   commanders), but it is a fixture trap: a future test that fields two
   same-named permanents through `commanderGame` will hit it. I worked around
   it with `newFixtureDeckWithOpponentCard`. Worth a note in the helper's
   doc if anyone else trips on it; not filed as a separate ticket because it
   is a test-helper ergonomics issue, not an engine behaviour, and the workaround
   is one line.

No CR-lane test is warranted by this ticket (it implements, and closes, the
CR 704.5k gap).

## Open concerns

None blocking. The one thing a reviewer should sanity-check is the deliberate
derived-authoritative world scan (deviation 1): it is strictly more correct
than the legend half's printed pre-filter, and it does not touch the legend
path.
