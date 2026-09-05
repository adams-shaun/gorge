# Final fix-wave re-review — gorge, base 5db64f2 → head 2fe9c72

### Finding Verdicts

**C1 (Critical) — every stack exit guarded — ADDRESSED (code), with an
Important test-coverage gap (see probe 2).**

`ensureLeftTheStack` is defined at `rules/stack.go:349` and called at six
sites — the five the review listed plus the ETB block it replaces:

| site (HEAD) | exit | `to` |
|---|---|---|
| `rules/stack.go:137` | `askTarget` "countered: no legal targets" | `ZGraveyard` |
| `rules/stack.go:216` | ability object, `ValidTgts$` fizzle | `ZExile` |
| `rules/stack.go:257` | ability object, normal CR 608.2m exit | `ZExile` |
| `rules/stack.go:291` | spell, resolution-time fizzle | `ZGraveyard` |
| `rules/stack.go:305` | permanent ETB (was the inline Task 29 block) | `ZGraveyard` |
| `rules/stack.go:310` | instant/sorcery, normal resolution | `ZGraveyard` |

The helper body is the reviewer's own (nil/zone check → save/set/restore
`e.applyingReplacement` → `Note` → `MoveZone`), and the ETB `Note` string is
byte-identical to BASE's (re-concatenated across different line breaks only) —
`rules/replacement_updated_test.go` is not in the diff at all, and both Task 29
guard tests pass unedited.

Both regressions exist in the new `rules/stack_totality_test.go` (198 lines):
`TestInstantResolvingUnderAGraveyardReplacementDoesNotStickOnTheStack:110`
(`ValidCard$ Instant,Sorcery`, `ReplaceWith$ … Defined$ ReplacedCard`) and
`TestAbilityObjectResolvingUnderAnExileReplacementDoesNotStickOnTheStack:172`
(`Destination$ Exile | ValidCard$ Card`). Each asserts `passes == 2`, the
terminal zone, exactly one `Note`, exactly one `Resolve`, and a further pending
decision. Mutation checks re-run independently: each goes RED when *its own*
call is deleted (probe 2).

**I2 (Important) — RedactEvents deep-copies before any branch — ADDRESSED.**
`view/redact.go:81-92` adds, unconditionally, at the top of the loop body
before the `if e.Secret` branch:

```go
e.IDs = append([]state.ObjID(nil), e.IDs...)
e.Pairs = append([][2]state.ObjID(nil), e.Pairs...)
```

Mutation test `TestRedactEventsDoesNotAliasTheEngineLog`
(`view/view_test.go:538`) asserts `Head()` unchanged, `Head() ==
HeadAt(len(Events))`, and `replay.Replay` still verifies. Independently
re-measured on a real repo-deck game (probe 3): PASS at HEAD, FAIL at BASE
(`base=82b98d56d46a6ca1 HeadAt=f4a7c5192d91ca92`).

**T23-z (Important, plan-mandated) — IDs/Pairs filtered on the zone-move
kinds — ADDRESSED.** `view/redact.go:124-125`, inside `case events.MoveZone,
events.Draw, events.PutOnStack:`, adds `e.IDs = filterVisible(g, e.IDs,
viewer)` and `e.Pairs = filterVisiblePairs(g, e.Pairs, viewer)`.
Behaviour-neutrality independently re-measured (probe 3): **0** of 21 non-test
emitters of those kinds set `IDs` or `Pairs`, no `.Kind =` assignment form
exists, and on a 16 692-event game across 3 viewers the filter changed **0**
events.

**Sweep — T21-i 11-row table applied verbatim — ADDRESSED.** All 11 land
exactly (probe 5). `T21-d` ×2 (`rules/combat.go:334`,
`rules/combat_test.go:472`) and `T21-f` ×2 (`rules/combat_test.go:27`,
`rules/layers_test.go:12`) are untouched. The whole-diff citation churn is
exactly 11 `+`/11 `−` `T21-*` lines and nothing else: no other `Ruling X-y`
citation changed anywhere in the diff.

**Sweep — `rules/trigger.go:968` → M-6 — ADDRESSED.** `:968` now reads
"Review finding M-6 (Task 29 fix round 1)"; the real I-3 block is intact at
`:989`. Label source confirmed at `task-29-rereview.md:110-119`.

**Sweep — `view/view_test.go` orphan doc removed — ADDRESSED.** The three
lines are gone; `TestRedactEventsStripsNoteCarryingAnotherSeatsLibraryIDs` no
longer appears anywhere in the tree.

**Sweep — `rules/turn.go` drainerSrc misquote — ADDRESSED.**
`rules/turn.go:253-256` now reads "an upkeep self-drain (`Mode$ Phase | Phase$
Upkeep`) trigger", matching `rules/trigger_order_test.go:28-32`'s actual
`drainerSrc` (`T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigDrain`) verbatim.

**Sweep — `cards/parse_test.go` ≤ 100 chars — ADDRESSED.** The reflowed
comment now occupies `cards/parse_test.go:116-126`, longest 77 chars. (The
file's `wc -L` is 104, from `:8` (104) and `:54` (103) — both pre-existing at
BASE and outside the brief's single named line.)

**Sweep — `fix1_test.go` → `genesis_replay_test.go` — ADDRESSED.** Rename
present, package-internal, no import changes, `go build ./...` green.

**Sweep — stale comments rewritten — 5 of 6 ADDRESSED, 1 NOT ADDRESSED.**

- `rules/trigger.go:817` — ADDRESSED and correct: `dealCombatDamage` is at
  `rules/combat.go:222`, and `events.Event` (verified field-by-field) carries
  no combat/noncombat discriminator, so the kept half is true.
- `rules/statics.go:167` — ADDRESSED and correct: `blockRestricted` is called
  from `rules/combat.go:80` inside `canBlock`.
- `rules/layers.go:80` — ADDRESSED and correct: `EndOfTurnCleanup` is called
  from `rules/combat.go:438` inside `cleanupStep`.
- `rules/statics_test.go:181-185` — ADDRESSED and correct: `askBlockers`
  (`rules/combat.go:167`) and `handleBlockers` (`:206`) are implemented and
  `rules/stubs.go` no longer exists.
- `rules/legal.go:66-70` — ADDRESSED: restated as T19c-b's parked limitation.
- **`rules/genesis_replay_test.go:396-402` — NOT ADDRESSED.** The stale claim
  was replaced by a *new* false claim. It says the fixture card "has no printed
  `A:AB$ Mana` line at all, so legal.go's 'activate' case finds zero entries in
  `o.Face().ManaAbilities()` and its resolveAbility loop never runs." Measured
  at HEAD: `cards.ApplyIntrinsics` (`cards/intrinsic.go:12-32`) grants every
  `Basic Land Mountain` an intrinsic `AB$ Mana | Produced$ R`, and `card()`
  (`rules/turn_test.go:20`) calls it — so `ManaAbilities()` returns **1**, the
  `resolveAbility` loop **does** run, one `ManaAdd R 1` is emitted, and the
  pool goes **0 → 1**. The test three lines below the comment `t.Fatalf`s if
  the `activate` option is *absent*, which requires exactly the opposite of
  what the comment asserts. Severity: Minor (comment-only), but the sweep item
  is not discharged.

**Sub-item 1b — no `Defined$` coverage key set — CONFIRMED, nothing changed.**
Read end to end: `cmd/forgec/main.go:90` calls `r.Coverage(effects.Supported())`;
`cards/primitive.go:11-40` builds coverage keys **only** as `"api:"+sa.API`,
`"trig:"+t.Mode`, `"stat:"+s.Mode`, `"repl:"+r.Event`, `"kw:"+KeywordHead(k)` —
no parameter *value* is ever extracted; `effects/registry.go:126-138`
(`Supported`) returns `api:` keys from the registry plus `supportedNonAPI`
(populated by `RegisterNonAPI` at `rules/trigger.go:1057-1060` — where
`"repl:Moved"` lives — `rules/combat.go:453`, `rules/statics.go:216`).
`effects.Defined` (`effects/context.go:16`) is a runtime resolver with a
documented fallback for unmodeled forms, not a coverage set. There is no
`validDefinedKeys`-style set to add `ReplacedCard` to. `make report` is
unchanged: `cards: 33667  playable: 15265 (45.3%)`.

**Judgment call 1 — `events/` is comment-only — CONFIRMED.**
`git diff 5db64f2..2fe9c72 --stat -- events/ rules/sba.go` = `events/apply_test.go
| 2 +-`, `events/event.go | 2 +-`; `rules/sba.go` is 0. Both changed lines are a
single word inside a `//` comment (`T21-a` → `T21-e`). Stripping comments,
`events/event.go`, `events/apply_test.go`, `events/log.go` and `events/apply.go`
are **byte-identical md5s** across the two revisions. Structurally: the `Kind`
`const` block (identifiers and order), the `Event` struct, and `func (e Event)
Append` all diff **IDENTICAL**. Ruling F-5 is satisfied.

**Judgment call 2 — `replay.Divergence.Short` — ADDRESSED, additive, honest,
untested.**
- *Additive-only*: the field is added to `replay.Divergence` only (nothing on
  `events.Event`; the `Event` struct is byte-identical, above). Every
  `Divergence` literal in the tree is keyed (`cmd/mtgsim/main_test.go:70`,
  `replay/replay.go:129,250,253`), so no construction site breaks. Both new
  branches (`Divergence.Error()` and `cmd/mtgsim/main.go`'s
  `printReplayOutcome`) are `case`s added to existing dispatch; the `Missing`
  and default wordings are unchanged.
- *No panic*: `l.Events[len(e.L.Events)]` at `replay/replay.go:129` is safe —
  `compare` (`:246-257`) returns `Missing` for any `i >= len(l.Events)` and runs
  after the first `Advance` and after every `Submit`, so reaching line 129 with
  a mismatch implies `len(e.L.Events) < len(l.Events)`. Exercised (probe 4).
- *mtgsim output*: the Short branch is only reachable on a divergence; `make
  sim`'s 20 games produce output **byte-identical** to a `git archive 5db64f2`
  build (probe 4).
- *Doc honesty*: the `Short` field doc is accurate ("Got is meaningless in that
  case"). One gap: the struct's pre-existing `Want, Got` paragraph
  (`replay/replay.go:46-49`) still says only "…zero events.Event when `Missing`
  is true; use `Missing`, never a zero-value check on Want" and was not extended
  to say `Got` is the zero event when `Short` is true. Minor.
- *Untested*: no test anywhere sets or asserts `Short`. Minor coverage gap; the
  wave's own report claims only that pre-existing tests stayed green, which is
  true.

**Judgment call 3 — the rename landed in the C1 commit — CONFIRMED
organisational only.** `git show --stat 10a62c6` shows
`rules/{fix1_test.go => genesis_replay_test.go} | 0` (pure rename, zero
content). The file's only content change — 11 lines, the M5 comment rewrite —
is entirely in the sweep commit `2fe9c72`. No content crossed a commit boundary.

---

### Probe Results

**Probe 1 — the review's own C1 reproduction shapes, public API only, at HEAD
vs BASE.** Scratch module `probe` outside the repo, `replace
github.com/adams-shaun/gorge => /home/sadams/projects/gorge` (HEAD) and
`=> <scratch>/copy-base` (a `git archive 5db64f2` copy). Card text authored for
the probe in the corpus's R:/SVar$ shape; deterministic policy (`play_land` >
`activate` > `cast` > `pass`), seed 3.

| shape | rev | intents | events | turn | over | stack | guard Notes | terminal zone |
|---|---|---|---|---|---|---|---|---|
| (a) instant under `ValidCard$ Instant,Sorcery` graveyard replacement, `ReplaceWith$ … Defined$ ReplacedCard` | **BASE** | 300 000 (budget) | 1 500 023 | 1 | **false** | `[Zap Lite]` | 0 | STACK ×1 |
| (a) | **HEAD** | **2 003** | 12 212 | 68 | **true** | `[]` | **19** (all `:310`) | **graveyard ×19**, hand ×1 |
| (b) ability object under `Destination$ Exile \| ValidCard$ Card` | **BASE** | 300 000 (budget) | 1 500 034 | 3 | **false** | `[<ability>]` | 0 | — |
| (b) | **HEAD** | **3 287** | 20 386 | 68 | **true** | `[]` | **514** (all `:216`/`:257`) | — |

BASE reproduces the review's signature exactly (`over=false turn=1`,
`events=500023` at the review's 100 000-intent budget = 1 500 023 at 300 000).
At HEAD both reach the next decision *and* run to `over=true` (decking), stack
empty, exactly one guard `Note` per swept object.

**Probe 2 — mutation, one call deleted at a time, in a `git archive 2fe9c72`
scratch copy (the checkout was never edited).**

| deleted site | `stack_totality_test.go` ×2 + Task 29 guard tests ×2 | whole `go test ./...` |
|---|---|---|
| `:137` askTarget-countered | 4/4 PASS — **unpinned** | **all green** |
| `:216` ability fizzle | 4/4 PASS — **unpinned** | **all green** |
| `:257` ability resolve | **RED** `TestAbilityObjectResolvingUnderAnExileReplacement…` | FAIL |
| `:291` spell fizzle | 4/4 PASS — **unpinned** | **all green** |
| `:305` permanent ETB | **RED** ×2 (`TestPermanentSpellWhoseEntryIsFullyReplaced…`, `TestTotalityGuardSurvivesABroadGraveyardReplacement`) | FAIL |
| `:310` instant/sorcery | **RED** `TestInstantResolvingUnderAGraveyardReplacement…` | FAIL |

So the two new tests' own mutation checks are genuine, and the ETB site is
still doubly pinned — but **three of six sites are pinned by nothing in the
repo**. Two of them are demonstrably load-bearing and reachable:

- **`:216` is independently Critical-class.** Deleting only `:216` in the HEAD
  copy leaves `go test ./...` fully green, yet the public-API probe hangs:
  `intents=100000 events=400074 turn=3 over=false stack=[<ability>]`. Shape that
  pins it: a permanent whose trigger's `Execute$` SVar carries `ValidTgts$` (e.g.
  `SVar:TrigZap:DB$ DealDamage | ValidTgts$ Creature | NumDmg$ 1`) — `checkTriggers`
  never records targets, so `spec != ""` with zero legal targets takes CR 608.2b's
  fizzle to `ZExile` — resolving with a `Destination$ Exile | ValidCard$ Card`
  replacement (`ReplaceWith$ … Defined$ ReplacedCard`) on the battlefield.
  Measured at HEAD: the `:216` guard fires **514** times in that game.
- **`:137` is reachable but accidentally backstopped.** A targeted instant
  (`A:SP$ DealDamage | ValidTgts$ Creature`) cast with no creature anywhere,
  under the graveyard replacement, hangs at BASE (`intents=300000 turn=1
  over=false stack=[Zap Target]`) and fires the `:137` guard **19** times at
  HEAD. Deleting `:137` alone does not hang, because the spell then survives to
  `resolveTop`, where the *same* `ValidTgts$` spec re-fizzles into `:291`. The
  redundancy is accidental, not designed or documented. Shape that pins it:
  assert exactly one `Note` whose `Text` names "countered: no legal targets", at
  cast time, before any priority round drains the stack.
- **`:291` is unpinned and unexercised.** No probe I built reached it — every
  "no legal target" spell was caught earlier at `:137`. Its distinct shape is a
  spell whose targets are legal at cast time and illegal at resolution (two
  damage spells in flight at one 1/1; the second resolves first and kills it),
  under a graveyard replacement. That is the only shape in which `:291` is not
  backstopped by `:137`.

**Probe 3 — `RedactEvents` aliasing on a real repo-deck game.** Test written
inside the scratch copy (`view/` package, `internal/testutil` repo decks:
`death-n-taxes` vs `dimir-tempo`, seed 7, 2 869 intents, **16 692 events**,
turn 100, `over=true`).

- Events carrying `IDs`/`Pairs`: **99**; of those on `MoveZone`/`Draw`/`PutOnStack`: **0**.
- Mutating every returned `IDs`/`Pairs` slot — viewer 0 (**155** slots), viewer
  1 (**161**), spectator index 9 (**95**): `Head()` unchanged and `HeadAt(len)`
  still equal at **`82b98d56d46a6ca1`**; `replay.Replay(e.L, cfg)` returns nil.
- Same probe against a `git archive 5db64f2` copy: **FAILS** on the first viewer
  — `base=82b98d56d46a6ca1 HeadAt=f4a7c5192d91ca92`. Not vacuous.
- T23-z neutrality: static scan of every `events.Event{…}` literal in the tree
  — **47** with Kind `MoveZone`/`Draw`/`PutOnStack` (21 non-test:
  `rules/stack.go` ×8, `effects/zone.go` ×5, `effects/cardflow.go` ×4,
  `rules/sba.go` ×2, `effects/misc.go` ×1, `rules/legal.go` ×1; 26 test) —
  **0 set `IDs` or `Pairs`**, and **0** `.Kind = MoveZone|Draw|PutOnStack`
  assignment forms. Dynamically, on the same 16 692-event log across 3 viewers,
  the new filter changed **0** events.

**Probe 4 — gates on the real checkout** (working tree, index and HEAD
unchanged throughout: `git status --porcelain` empty at start and end,
`HEAD=2fe9c722bfde5e2794c35c29ef50b9fe751b522c`; only gitignored `bin/` was
rebuilt by `make`).

| gate | result |
|---|---|
| `make lint` | `gofmt -l .` empty, `go vet ./...` clean, **exit 0** |
| `CGO_ENABLED=0 go build ./...` | OK |
| `go test -count=1 ./...` | all 12 packages green, **15.98 s** wall |
| `go test -race -count=1 ./rules/ ./view/ ./replay/` | `rules 142.795s`, `view 1.023s`, `replay 49.513s`, **0 races**, **142.92 s** wall |
| `make sim` | 20 games, **20/20 `replay OK`**, exit 0, **1.66 s** wall |
| `make sim` output vs a `git archive 5db64f2` build | **byte-identical** (sole diff is `make`'s own recipe echo line, absent from the direct binary run) |
| `make report` | **`cards: 33667  playable: 15265 (45.3%)`** — exact |
| `git ls-files \| grep -ci '\.txt$'` | **0** |
| `cards.TestNoForgeScriptsTracked` | **PASS** (runs, not skipped) |

Acceptance chain heads (`go test ./rules/ -run
'TestRepoDecksPlayAtEverySeatCount|TestRepoDeckGamesReplayExactly' -v`):

| seats | intents | events | turns | winner | chain | required | match |
|---|---|---|---|---|---|---|---|
| 2 | 345 | 1994 | 15 | death-n-taxes | `7705a6505954f6cd` | `7705a6505954f6cd` | ✓ |
| 4 | 1188 | 6210 | 35 | mono-black-aggro | `2d5589b31c4853cd` | `2d5589b31c4853cd` | ✓ |
| 6 | 2410 | 11800 | 51 | mono-green-stompy | `bf4012092fdad38b` | `bf4012092fdad38b` | ✓ |
| 8 | 3800 | 17788 | 61 | mono-green-stompy | `01b9f48c1b6dc135` | `01b9f48c1b6dc135` | ✓ |

`TestRepoDeckGamesReplayExactly`, 5 seeds, all `replay OK`:
`14fcd780b8791e5e` / `f57f72c786bc53ad` / `d7b8de12dd00c449` /
`69e808f5e005c0fd` / `b2ba742268c7d0db`. **No head moved.**

The five Task 22 pins, all PASS, with their pinned numbers quoted from the
assertions:

```
--- PASS: TestDestroyLethalDamageDoesNotAmplifyWhenAReplacementKeepsThePermanent
      "life gained after one Submit = %d, want exactly 2"   /  "log grew by %d events ... want exactly 6"
--- PASS: TestRemovePermanentsDoesNotAmplifyWhenAReplacementKeepsThePermanent
      "life gained after one Submit = %d, want exactly 1"   /  "log grew by %d events ... want exactly 5"
--- PASS: TestNoPendingDecisionWithAZeroLifePlayerNotYetLost              (CR 704.5a outstanding: 0)
--- PASS: TestNoPendingDecisionWithAZeroLifePlayerNotYetLostViaRemovalSweep (CR 704.5a outstanding: 0)
--- PASS: TestLethalDamageIsRetriedWhenTheReplacementsControllerIsEliminatedMidCall  (rules/sba_test.go:606)
      doomed -> graveyard, guardian off the battlefield
--- PASS: TestRemovalSweepFiringsDoNotScaleWithAnUnrelatedDeathChain
      "blocked sweep fired %d times, want exactly 2" across chains {0, 1, 5, 20, 60}
ok  github.com/adams-shaun/gorge/rules  0.015s
```

Additional M12 runtime check (scratch module, public API): a recording longer
than the replay yields `Seq=233 Short=true Missing=false Want.Kind=draw
Got.Kind=game_start`, `Error() = "replay diverged at event 233: replay ended
there; the recorded log continues with draw"`; the mirror direction still
yields `Seq=230 Missing=true Got.Kind=priority`; an empty recording yields
`Missing` at Seq 0. **No panic in any direction.**

**Probe 5 — T21-i, row by row.** All 15 `T21-*` citations in Go source at HEAD:

| file:line | cites | table says | ✓ |
|---|---|---|---|
| `events/event.go:93` | T21-e | T21-e | ✓ |
| `rules/turn.go:33` | T21-e | T21-e | ✓ |
| `events/apply_test.go:803` | T21-e | T21-e | ✓ |
| `rules/combat_test.go:521` | T21-e | T21-e | ✓ |
| `rules/combat.go:156` | T21-c | T21-c | ✓ |
| `rules/combat_test.go:443` | T21-c | T21-c | ✓ |
| `rules/combat.go:303` | T21-a | T21-a | ✓ |
| `rules/combat.go:384` | T21-a | T21-a | ✓ |
| `rules/combat_test.go:373` | T21-a | T21-a | ✓ |
| `rules/combat.go:346` | T21-b | T21-b | ✓ |
| `rules/combat_test.go:408` | T21-b | T21-b | ✓ |
| `rules/combat.go:334` | T21-d | *(untouched)* | ✓ |
| `rules/combat_test.go:472` | T21-d | *(untouched)* | ✓ |
| `rules/combat_test.go:27` | T21-f | *(untouched)* | ✓ |
| `rules/layers_test.go:12` | T21-f | *(untouched)* | ✓ |

Whole-diff citation churn: exactly 11 added and 11 removed `T21-*` lines, and
nothing else. The only other ruling-ID *additions* are new prose, not relabels:
`T19c-b` (`rules/legal.go:69`), `T23-z` (`view/redact.go:118`), `T26-a`
(`rules/stack.go:334`, attributing the original guard to Task 29 — confirmed
correct against `progress.md:4144`). No citation outside the table changed.

**Probe 6 — hygiene.** `wc -L cards/parse_test.go` = **104**, but the changed
comment (now `:116-126`) is at most **77** chars; the two >100 lines (`:8` 104,
`:54` 103) are byte-identical at BASE. `git diff 5db64f2..2fe9c72 --stat --
.cards` → **empty**. `git ls-files | grep -ci '\.txt$'` → **0**.

---

### New Breakage in the Fix Diff

- **Minor — `rules/genesis_replay_test.go:396-402`**: a newly-written comment
  that is false. Measured: `ManaAbilities()` = 1 (intrinsic, `cards/intrinsic.go:26-31`),
  the `resolveAbility` loop runs, one `ManaAdd R 1` is emitted, pool 0 → 1. The
  test's own `t.Fatalf("no activate option…")` at `:430` requires the opposite of
  what the comment claims. See the sweep verdict above for the exact fix.
- **Minor — `rules/stack.go:316-317` and `rules/stack_totality_test.go:2-9`**:
  both say "resolveTop's five exits". There are **six** guarded exits, and one of
  them (`:137`) is in `askTarget`, not `resolveTop`. The test header then writes
  "The other four (…)" and lists five. Arithmetic/attribution only; no behaviour.
- **Minor — `replay/replay.go:46-49`**: the `Want, Got` doc paragraph was not
  extended for the new `Short` case (it still speaks only of `Missing`), even
  though the same commit adds a second condition under which `Got` is the zero
  event.
- **Minor — `replay.Divergence.Short` is untested**: no test in the tree sets or
  asserts it, despite `printReplayOutcome` having been extracted specifically so
  a hand-built `*replay.Divergence` can drive it (`cmd/mtgsim/main.go:181-184`).
  Verified working by an out-of-repo probe instead.
- **Cosmetic — `rules/turn.go:255-257`**: the reflow left a short ragged line
  ("…only priority -- so" / "their draw step is still entered"). `gofmt` clean.

No new Critical. No behavioural regression: every measured number that existed
before this wave is unchanged (four chain heads, five replay chains, 20 sim
chains, `make report`, the five Task 22 pins), and the two commits that touch
running code (`10a62c6`, `215af2f`) are both proven live by BASE/HEAD
counterfactuals.

---

### Out-of-Scope Observations

- The mutual backstopping between `:137` and `:291` (probe 2) means the six
  guards are not six independent safety nets; a future reader deleting either
  will see a green suite. Worth one sentence in `ensureLeftTheStack`'s doc.
- The review's own after-merge recommendation #1 (a `resolveTop` never returns
  with `id` still in `e.G.Stack` invariant in the fuzz gate) would have pinned
  all three unpinned sites at once, and remains the cheapest fix for the gap
  probe 2 found.
- `rules/trigger.go` is still 1062 lines (Ruling F-4 correctly respected — not
  split in this wave).
- `cards/parse_test.go:8` (104 chars) and `:54` (103) remain over 100; both
  pre-date this wave and neither was named in the brief.
- The wave's report flags the `events/` gate as "not literally empty" and
  explains why; Ruling F-5 already resolves that, and the diff is provably
  comment-only (byte-identical code md5s). No action needed.

---

### Verdict

**Merge gate: Findings remain open.**

Two items, both cheap:

1. **Important — three of the six `ensureLeftTheStack` call sites are pinned by
   nothing** (`rules/stack.go:137`, `:216`, `:291`). Deleting `:216` alone leaves
   the entire suite green while a reachable, ordinary-card game hangs at
   `intents=100000 over=false turn=3 stack=[<ability>]`.
   *Exact fix*: add to `rules/stack_totality_test.go` one test per site, each
   mutation-checked against its own call —
   (a) `:216` — a permanent whose trigger `Execute$` SVar carries `ValidTgts$`
   (`SVar:TrigZap:DB$ DealDamage | ValidTgts$ Creature | NumDmg$ 1`), its ability
   object resolving with `exileBlockingReplacementSrc` on the battlefield;
   (b) `:137` — a targeted instant (`A:SP$ DealDamage | ValidTgts$ Creature`)
   cast with no creature anywhere, under `graveyardBlockingSpellsReplacementSrc`,
   asserting one `Note` naming "countered: no legal targets" at cast time;
   (c) `:291` — two damage instants aimed at the same 1/1, the second resolving
   first, so the first fizzles at resolution under the graveyard replacement.
   Each must assert a bounded pass count, the terminal zone, and exactly one
   `Note`. (Alternatively, the review's recommendation #1 invariant in the fuzz
   gate pins all three at once.)

2. **Minor — `rules/genesis_replay_test.go:396-402` states a false reason.**
   *Exact fix*: replace with the measured truth — "`resolveAbility` is fully
   implemented (Task 14) and this activate really does add {R} to the pool:
   `Basic Land Mountain`'s mana ability is intrinsic (`cards.Face.ApplyIntrinsics`,
   `cards/intrinsic.go`) even though the fixture script has no printed `A:` line,
   so `ManaAbilities()` returns one entry and one `ManaAdd` is emitted. This test
   simply does not assert the pool — it is about the activate/replay path
   (`Passes`, `Priority`, `Tapped`)."

Everything else is addressed and every gate is green at the required numbers.
Neither open item touches running code; both are additive.
