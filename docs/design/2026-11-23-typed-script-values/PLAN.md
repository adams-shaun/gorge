# cardc: mtgish as the card compiler front end, proven end to end on a side branch

Status: plan, for the operator. Not committed.
Bases: gorge origin/main `86b960f2c5`; gorge-compliance origin/main `285a7038`; mtgish data measured at the
current compliance pin `153451e44d` (`.mtgish/153451e44d…/mtgish.lines.json`). A claim marked INFERRED was not read
from code or data.

Operator decision (11-23): "mtgish as the basis for our new card compiler front end and unified input to engines …
on a side-branch for now proving out the functionality e2e." This plan implements that. The Forge-script compiler
(`engine/cards`) stays on main unchanged.

## 1. Pipeline

```
i5jb/mtgish (MIT)            cardc (new, committed code + schema)                       engines
mtgish.lines.json  --fe-->  typed card IR (engine/cardc/ir)  --be:gorge-->  *cards.Card  --> gorge rules
(.mtgish/<ref>/,             Ref / Filter / Value / Cond core,                  (today's ABI)  --> kgen -> dirac
 gitignored)                 Program, Cost, Duration, Keyword
                                  ^
                                  +-- later: Forge-script front end into the same IR (cross-check, section 7)
```

1. **Source.** mtgish's typed tree, `mtgish.lines.json` at `MTGISH_REF`. It is one JSON object per card, every Rust
   enum a tagged union (`{"_Action":"SpellDealsDamage","args":[…]}`). At `153451e` it holds 33,529 lines, 32,850
   distinct names, 7,180 distinct `Enum.Variant`s over 310 enums (measured, `scratchpad/typedir/mvar.py`). gorge
   gets the same pin as compliance: `MTGISH_REF` = `1553384` once gorge-compliance PR #373 lands (open today).
2. **IR.** A typed card IR in `engine/cardc/ir`, stdlib only. Its core is four leaf families. Their shapes are taken
   from oracle-grammar's schema 192 (MIT; shapes only, no code), so a later oracle-grammar front end maps onto the
   same nodes:

   | family | cardc node | mtgish enums it is read from | oracle-grammar shape it follows |
   |---|---|---|---|
   | references | `ir.Ref{Kind RefKind; Slot uint8; Of *Ref; Proj Proj; LKI bool}` (object and player refs: Self, Target(slot), EventSubject, AttachedTo(ref), ControllerOf/OwnerOf(ref), ExiledWith(src), Chosen, Remembered-as-receipt, Union) | `Permanent`, `Player`, `Spell`, `Card`, `Target`, `DamageRecipient`, `Ref_*` args | `ObjectRef` (59 variants), `PlayerRef` (51) |
   | filters | `ir.Filter{All/Any/Not []Filter; Type, Subtype, Color, Controller *Ref, Owner *Ref, Zone, Counters, Cmp *Value, Word FilterWord}` | `Permanents` (354 corpus variants), `Players`, `Spells`, `Cards`, `CardsIn*`, `Exilable` | `ObjectFilter` (137) |
   | values | `ir.Value{Kind ValueKind; N int32; Of *Filter; Ref *Ref; Prop Prop; Args [2]*Value; Op ArithOp}` (Constant, X, Count(filter), Property(ref, prop), Add/Sub/Mul, Half-round) | `GameNumber` (425), `CounterType` amounts | `Expr` (145) |
   | conditions | `ir.Cond{Kind CondKind; All/Any/Not []Cond; L, R *Value; Cmp CmpOp; Filter *Filter; Ref *Ref}` (Compare, Exists(filter), PlayerIs, WasCastFrom, Turn, …) | `Condition` (235), `CheckHasable` | `Condition` (121) |

   Abilities (`Spell`, `Activated`, `Triggered`, `Static`, `Replacement`, `Keyword`), `Program` (ordered `Action`
   list with `If/Else` and receipts), `Cost`, `Duration` and `ManaProduce` sit around them. Each node keeps the mtgish
   variant path it came from (`Src string`, for refusal messages and the ledger), never Oracle text.
3. **Gorge binding.** The backend `engine/cardc/gorgebe` lowers IR to `*cards.Card`, the struct gorge's rules and
   kgen already read (`engine/cards/ir.go:11` `SA{Kind, API, Params, Sub}`). It emits script lines in gorge's own
   vocabulary (API names, `ParamKey` names, `engine/cards/params.go:28`) and runs them through the existing load path
   (`cards.ParseBytes`, `engine/cards/parse.go:21`, then linking and `ParamSet` binding, `engine/cards/link.go:243`).
   So the proof changes no engine code: both card sources meet at one ABI, and a divergence is a card-definition
   difference by construction. Native IR reading by the engines comes after the proof (section 7).
4. **What is committed.** Only cardc code, the IR schema, per-variant lowering code, tests that compile from
   `.mtgish/` at test time, and registers that hold card names and class tokens (as compliance's `known/*.tsv` do).
   Never committed: mtgish data, any per-card IR dump or golden, any generated script text. All of that is derived
   from Oracle text and lives in gitignored `.mtgish/` and `.cards/cardc/`. cardc never copies Forge script text: the
   per-card fallback (section 5) loads the Forge IR from `.cards` at run time, by reference.

## 2. What to lift from gorge-compliance `oracle/mtgish`

Read at `285a7038`. 15,292 lines; built 10-20/10-21 (217 of its commits are on those two days).

| piece | lines | lift | why |
|---|---:|---|---|
| `node.go` (tagged-union reader: `tagged`, `variantOf`, `argsOf`, `argN`, `field`) | 162 | **as code**, into `engine/cardc/mtgish/node.go` | stdlib only (`sort`, `strconv`, `strings`); the generic reader cardc needs |
| `load.go` (`Card`, `decodeCard`, the face list, the name index) | 135 | **as code** | stdlib only; face order and the shared-field fallback are already right |
| `vocab.go` (type words, mana symbols) | 167 | **as code** | stdlib only |
| `exclude.go` + `known/excluded.tsv` (digital, Un-, non-game faces) | 137 | **as code + data** | the same population rules apply |
| `typeindex.go` | n/a | **no**: it imports `engine/cards` for gorge's type index; cardc's backend has that directly | |
| `MTGISH.lock`, the Makefile `mtgish-fetch` recipe, the boundary guard | n/a | **as recipe**, in `cardc.mk` | same pin, same sha256 check, same gitignore |
| `map_*.go` (21 normaliser files), `effects_mtgish.go`, `from_mtgish.go` | ~11,000 | **as a reference index, not code** | they reduce trees to summary tokens (lossy). What carries over is the variant knowledge: which mtgish variant is which gorge verb (2,612 of 7,180 variant names appear as literals in them), and their tests' tricky shapes. Each fe ticket cites the map file for its enum |
| `from_gorge.go`, `effects_gorge.go` (gorge `cards.Card` to summary) | 1,642 | **no lift; run in place** | CC-30 can point `cmd/mtgishxc` at a cardc registry for a coarse second diff, from compliance, without copying |
| ledger `known/T1-T4.tsv` | 1,399 rows | **as data** (names + class only) | PR #373: 329 mtgish-bug rows at `1553384`, T4 agreement 97.4%. These seed the gap register (section 5). The `forge-bug` rows (11 with Doctor Strange) seed the forge-bug class. The `equivalent` notes name the canonicalisations the static diff must apply: "if/else merged vs one sub-ability per branch" (17), "two conditional SAs vs one IfElse" (16), "one action over several targets vs one sub-ability each" (16) |

Scale for the proof slice. The table counts distinct `Enum.Variant`s at `153451e`; "named" means the variant name
appears as a literal in compliance's non-test mtgish code.

| slice | cards | missing in mtgish | variants | named in compliance |
|---|---:|---:|---:|---:|
| FDN pool (`dirac/cardset/pools/fdn/cards.txt`) | 286 | 0 | 530 | 233 |
| BLB pool | 276 | 0 | 616 | 269 |
| 12 Legacy acceptance decks (`testkit/testutil/decks.go:65`) | 136 | 1 (Delver of Secrets: DFC name join) | 394 | 205 |
| **FDN + Legacy 12** | 421 | 1 | **680** | n/a |
| all 34 repo decks | 1,362 | 21 (all `A // B` or DFC names) | 1,445 | 615 |

The largest enums in FDN + Legacy 12 are `Action` 106, `Rule` 62, `Permanents` 45, `GameNumber` 28, `Trigger` 28,
`Cost` 17, `Condition` 15, `Target` 15 and `PlayerEffect` 15. The front end is about 680 variant lowerings for the
proof, against 7,180 for the corpus.

## 3. The side branch

- **Branch:** `cardc` in gorge, cut from origin/main. Tickets use `agentctl issue add --target-branch cardc`
  (`agentctl/cli.py:721`).
  - PRs to `cardc` are outside the Mergify queue (`.mergify.yml:8` and `:54` match `base=main`) and outside
    `ci.yml`'s `pull_request` branches (`:43`, main and develop).
  - The cardc gate is each seat's precheck plus `make cardc-check` (CC-00), merged into `origin/cardc` by the lander.
    INFERRED: agentctl's lander merges a `target_branch` ticket without a main-queue PR. CC-00 verifies this before
    any other ticket is queued.
- **Layout:**

  ```
  engine/cardc/ir/       node schema, printer, equality, JSON codec   (no imports outside stdlib)
  engine/cardc/mtgish/   front end: reader (lifted) + fe_<enum>.go    (imports ir)
  engine/cardc/canon/    IR canonicalisation, the one alias home      (imports ir)
  engine/cardc/gorgebe/  backend to *cards.Card + registry builder    (imports ir, engine/cards)
  engine/cardc/lockstep/ record/replay harness, divergence ledger     (imports rules, replay; test + cmd use only)
  cmd/cardc/             compile | report | diff | lockstep
  cardc.mk               mtgish-fetch, cardc-check, cardc-lockstep    (one `include` line in Makefile)
  ```

- **Main's behaviour is untouched by construction:** no package on main imports `engine/cardc`. CC-00 adds
  `TestNothingImportsCardc` in `internal/archtest`: only `engine/cardc/**` and `cmd/cardc` may import it. So
  - golden heads (`engine/rules/testdata/heads`), set heads, wire goldens and `scripts/dirac-gate.sh` stay unmoved;
  - the IR cache key does not move: `cacheVersion` stays 8 (`engine/cards/registry.go:338`), and `engine/cards`
    source is not edited, so `CompilerFingerprint` (`engine/cards/fingerprint.go:20`) is unchanged.
  - cardc's own compiled output gets its own cache, `.cards/cardc/ir-<sha(cardc sources, MTGISH_REF)>`, beside
    gorge's.
- **Edits outside the new directories** are confined to CC-00:
  - the Makefile `include`;
  - `.gitignore` (`.mtgish/`, `.cards/cardc/`);
  - `TestPublicBoundary` (`internal/archtest/public_boundary_test.go:339`): it fails on a tracked `.mtgish` file, a
    tracked `.cards/cardc` file, or a committed `*.lines.json`;
  - `TestDependencyOrderHolds` (`internal/archtest/arch_test.go:204`, order list `:361`): registers the cardc
    packages.

  These are new forbidden edges and new packages, not raised ratchets. `internal/codeshape`'s strings ratchet does
  not cover `engine/cardc` (its scopes are `internal/codeshape/strings_ratchet_test.go:31`), and the branch does not
  add it.
- **Staying current:** one rolling ticket, `cardc-follow` (the `kfollow` pattern), merges origin/main into `cardc`
  every 6 hours and re-runs `make cardc-check`. It is a merge, not a rebase, because many seats branch from
  `cardc`. A red follow is P0 for the cardc lane only. The only expected conflicts are in the four CC-00 hook files.

## 4. The end-to-end proof

**Slice:** the FDN pool and the 12 Legacy acceptance decks.
- FDN games are the committed dirac slice: `dirac/oracle/testdata/slices/fdn/pairs.tsv`, 1,500 pairs over the
  2,841 decks in `decks.tar.gz`.
- Legacy games are the acceptance games at 2, 4, 6 and 8 seats (`engine/rules/acceptance_game.go:49`
  `AcceptanceConfig`, `:97` `PlayAcceptance`).
- Commander: `cmd4`, the four-seat Foundations Commander pod. It is **stage 2** of the proof, because its 100-card
  decks reach beyond the FDN pool (pods are four seats, never two).
- BLB is the second slice, after FDN passes.

**Harness (CC-31).** It uses gorge's replay contract (`engine/replay/replay.go:62` `Replay`, `:288`
`FirstDivergence`): a replay re-runs the engine on the recorded intents with the Config's decks.
1. **Forward:** play a game on the Forge registry (`botpolicy.GameBot`, the acceptance bot) and record its log.
   Rebuild the same decks from the cardc registry, card for card in the same order, and `Replay`. A refused intent
   or a `*Divergence` is a divergence, named at its first event.
2. **Reverse:** play on the cardc registry and replay on Forge. This catches lines the Forge game never took.
3. Swapping card definitions under a recorded log is outside `Replay`'s documented contract, which expects "the same
   *cards.Card pointers". CC-31's first test plays one deck through both registries with identical definitions, to
   show the swap itself diverges nowhere. INFERRED until that test passes.

Each test chunk is ≤ 100 games, to fit the one-minute test budget (the precedent is
`TestCompiledFilterMatchesTextualOracleChunk00-07`, `engine/effects/filter_compiled_oracle_test.go:311`). The whole
slice runs as `make cardc-lockstep SLICE=fdn`.

**Static diff first (CC-30).** `cardc diff` compares each card's cardc `*cards.Card` with its Forge one:
- ability multiset by kind and API;
- target specs, amounts and trigger modes, after the canonicalisations the compliance ledger already lists.

It is cheap and runs before any game, so most gaps surface without a game.

**Classes.** Every divergence gets a ledger row: `engine/cardc/testdata/ledger/<card>.tsv`, one file per card, no
text, held both ways like `TestMtgishLedger`.

| class | meaning | action |
|---|---|---|
| `mtgish-bug` | mtgish's tree is wrong against Oracle text | local patch or fallback; file upstream (section 5) |
| `cardc-gap` | mtgish is right; cardc's front end, canon or backend loses it | fix cardc. **Must be 0 at the bar** |
| `forge-bug` | the Forge script is wrong against Oracle text, and cardc is right | record it; fix gorge's Forge path on main or the fork (`gorge` branch) in a normal main ticket |
| `equivalent` | same game meaning, different event order (for example, ability order in a prompt) | canonicalise in the backend until the streams match; the class may hold rows only while the fix is queued |

**Pass bar (FDN stage):**
1. **Compile:** at least 95% of the 421 slice cards compile from mtgish with no refusal and no fallback. The rest sit
   on the fallback register, each with a class. Nothing is unclassified.
2. **Lockstep:** 0 divergences without a ledger row over the 1,500 FDN pairs at 2 seats (forward and reverse), plus
   the Legacy acceptance games at 2/4/6/8 seats.
3. **Ledger:** `cardc-gap` 0 and `equivalent` 0.
4. **Coverage:** every cardc-compiled card in the slice plays in at least `EvidenceMinPlays` = 3 divergence-free games
   (`dirac/oracle/evidence.go:11`, the same bar dirac uses).
5. **Stage 2:** `cmd4` at 4 seats replays divergence-free with cardc as the source of every card it covers.

**Forge as the independent cross-check.** Once a card passes, it has two independent readings that agree in lockstep:
Oracle text through mtgish, and Forge's script. Today xproof's Forge-as-oracle shares gorge's script, so a script bug
agrees with itself. A disagreement between the two readings is exactly the forge-bug or mtgish-bug signal xproof
lacks. CC-40 adds a `reading-agreement` column per card to the proof page input.

## 5. Gaps and how cardc handles them

Known mtgish classes. The rows come from the compliance ledger at `153451e`; PR #373 gives the totals at `1553384`.

| class | rows at 153451e | where it bites | cardc handling |
|---|---:|---|---|
| "up to N" read as exactly N (i5jb/mtgish#3) | 68 | target counts: a min-0 target becomes mandatory | **patch**: the tree cannot show it, so the patch is keyed by card name from the ledger's mtgish#3 rows. It sets `Target.Min = 0`. A name-only register, no text |
| morph: first line duplicated, face-up trigger lost (mtgish#1) | 34 + 18 | morph and megamorph cards | **detect + patch**: drop the duplicate rule and rebuild the turned-face-up trigger from the Morph keyword node. Fallback if a game diverges |
| Class level costs (the ClassAbility capture) | 32 | Class enchantments | **fallback** until fixed upstream (no Class in FDN; BLB has some) |
| haste dropped (mtgish#2) | 13 | granted haste | **patch** by name |
| MDFC back-face cost | 9 | MDFCs | **patch** from the face's `ManaCost` |
| no-refusal policy | n/a | mtgish never refuses; a wrong tree is silent | the static diff and lockstep are the detectors. cardc's front end **refuses** any variant it has no lowering for (`refuse("Action.X")`), so the gap is always named |
| rules meaning carried in AI fields | n/a | `AILogic$ Random` on charms (`engine/effects/charm_params.go:79`); `FriendOrFoe` on AssignGroup (`engine/effects/assigngroup_params.go:19`) | the backend emits these from IR facts (random mode choice), never from AI data |

The policy has three tiers, in order:
1. **Upstream:** file each class on i5jb/mtgish (our #1 to #4 are open). Bump `MTGISH_REF` with compliance when one
   closes. Patches whose rows disappear are deleted in the bump.
2. **Local patch:** IR-level only, in `engine/cardc/canon/patch_<class>.go`, keyed by a name list in
   `engine/cardc/testdata/patches/<class>.txt`. A patch is a structural edit with no text. The patch register only
   shrinks after the proof.
3. **Per-card fallback:** `engine/cardc/testdata/fallback/<card>.txt` names the class. The registry builder then
   serves that card's Forge `*cards.Card` from `.cards`. Fallback cards do not count toward the 95% compile bar, and
   the register only shrinks after the proof. Tokens fall back to Forge's `tokenscripts` by name during the proof,
   and are counted.

## 6. Tickets

Every ticket targets `cardc`. "Done means" always ends with `make test-targeted`, and `make test-smart` before DONE.
A missing `.mtgish` is `ENV`, never green. Sonnet implements every ticket. Opus reviews the four schema-defining
tickets (CC-01, CC-11, CC-12, CC-20), per the 11-19 model rule.

Each enum family owns its files:
- front end: `fe_<enum>.go` and `ir/<family>.go`;
- backend: `be_<group>.go`.

Lowerings register themselves from their own files (the atomdiff precedent in the operand-algebras design §6.2
K1), so no shared switch is edited twice. Ledger, patch and fallback registers are one file per card or class, so
no two seats append to one file.

| id | title | files | depends | est. seat-days | Done means |
|---|---|---|---|---:|---|
| CC-00 | branch, hooks, gate | `cardc.mk`, Makefile (`include`), `.gitignore`, `internal/archtest/{public_boundary,arch}_test.go`, `oracle`-style `MTGISH.lock` at `engine/cardc/MTGISH.lock`; verify agentctl lands to `cardc` | none | 0.3 | `make mtgish-fetch` verifies sha256; `make test-one PKG=./internal/archtest RUN='^(TestPublicBoundary\|TestNothingImportsCardc\|TestDependencyOrderHolds)$'` PASS; one no-op ticket lands on `cardc` |
| CC-01 | IR schema | `engine/cardc/ir/*.go` (card, ability, program, ref, filter, value, cond, cost, duration; printer, `Equal`, JSON codec) | CC-00 | 0.75 | `make test-one PKG=./engine/cardc/ir RUN=.` PASS: print/parse round trip and Equal over hand-built nodes, with no card data |
| CC-02 | mtgish reader + report | `engine/cardc/mtgish/{node,load,vocab,exclude}.go` (lifted), `refuse.go`, `cmd/cardc` `report` | CC-01 | 0.4 | `cardc report -slice fdn` lists every variant and its refusal count; DFC and split names join (Delver and the 21 deck misses resolve); `make test-one PKG=./engine/cardc/mtgish RUN=.` PASS |
| CC-10 | fe: characteristics and keywords | `fe_typeline.go`, `fe_manacost.go`, `fe_pt.go`, `fe_keyword.go` | CC-02 | 0.4 | slice: characteristics equal the Forge card's for 100% of non-refused faces (test) |
| CC-11 | fe: references | `ir/ref.go`, `fe_permanent.go`, `fe_player.go`, `fe_spell.go`, `fe_target.go` | CC-02 | 0.5 | every slice `Permanent/Player/Spell/Card/Target/DamageRecipient` variant lowers or refuses by name |
| CC-12 | fe: filters | `ir/filter.go`, `fe_permanents.go`, `fe_players.go`, `fe_cards.go` | CC-11 | 0.6 | as CC-11 for `Permanents/Players/Spells/Cards/CardsIn*/Exilable` |
| CC-13 | fe: values | `ir/value.go`, `fe_gamenumber.go`, `fe_countertype.go` | CC-12 | 0.4 | as CC-11 for `GameNumber/CounterType` |
| CC-14 | fe: conditions | `ir/cond.go`, `fe_condition.go` | CC-13 | 0.4 | as CC-11 for `Condition/CheckHasable` |
| CC-15a/b | fe: actions (A-L / M-Z, the compliance c1/c2 split) | `ir/action.go` (15a), `fe_action_a.go`, `fe_action_b.go` | CC-11..14 | 2 × 0.6 | all 106 slice `Action` variants lower or refuse by name |
| CC-16a/b | fe: abilities (spell/activated/triggered + `Trigger`; statics, `LayerEffect`, replacements, `Cost`) | `fe_rule_a.go`, `fe_trigger.go`, `fe_rule_b.go`, `fe_cost.go` | CC-15a | 2 × 0.6 | all 62 slice `Rule` and 28 `Trigger` variants lower or refuse; `cardc report` refusals ≤ 5% of slice cards |
| CC-20 | be: card skeleton + registry | `engine/cardc/gorgebe/{card,registry,fallback}.go` | CC-10, CC-01 | 0.75 | `cardc compile -slice fdn` builds a `*cards.Registry` (tokens via Forge); `cards.ParseBytes` diags 0 on every emitted card |
| CC-21a/b | be: family printers (Ref → `Defined$`/target spellings; Filter → `Valid*`; Value → `Count$`/SVar; Cond → `Condition*`) | `be_ref.go`, `be_filter.go` (21a); `be_value.go`, `be_cond.go` (21b) | CC-20, CC-11..14 | 2 × 0.6 | every printed spelling is one gorge reads: `knownDefinedTargets` known (the `TestDefinedSelectorCensus` reader, `engine/effects/defined_selector_census_test.go`), `cards.ValueHead` in `modelledValueHeads` (`engine/effects/value_heads.go:20`), and filters with no `csUnknown` predicate (`engine/effects/filter_compiled.go:117`) |
| CC-22a/b | be: actions to API + params | `be_action_a.go`, `be_action_b.go` | CC-15a/b, CC-21a/b | 2 × 0.6 | every lowered slice action emits a registered API (`effects.Supported()`, `engine/effects/registry.go:692`) |
| CC-23 | be: triggers, statics, replacements, costs | `be_trigger.go`, `be_static.go`, `be_repl.go`, `be_cost.go` | CC-16a/b, CC-21a/b | 0.75 | every lowered slice ability emits a supported `trig:/stat:/repl:` mode |
| CC-30 | static diff | `cmd/cardc` `diff`, `engine/cardc/lockstep/static.go` | CC-22a/b, CC-23 | 0.5 | `cardc diff -slice fdn` classifies every card as equal, differs or refused; differing cards get ledger rows |
| CC-31 | lockstep harness | `engine/cardc/lockstep/{record,replay,chunk_test}.go`, `cardc.mk` target | CC-20 | 0.6 | identity-swap test (section 4) PASS; `make cardc-lockstep SLICE=fdn` prints `pass/diverged/refused` per pair |
| CC-32 | divergence ledger + ratchet | `engine/cardc/testdata/ledger/`, `ledger_test.go` | CC-30, CC-31 | 0.4 | `make test-one PKG=./engine/cardc/lockstep RUN='^TestCardcLedger$'` fails both ways |
| CC-33 | gap register: patches + fallback | `engine/cardc/canon/patch_*.go`, `testdata/patches/`, `testdata/fallback/` (seeded from compliance's mtgish-bug rows) | CC-32 | 0.4 | each section 5 class has a patch or fallback; registers only shrink (test) |
| CC-34..39 | burn-down waves, one class family each | per class | CC-33 | 6 × 0.5 | pass bar 1-4 met on FDN |
| CC-40 | reading-agreement output | `cmd/cardc` `agree` (a TSV for the proof page) | CC-34..39 | 0.3 | TSV lists every slice card with `agree/disagree/fallback` |
| cardc-follow | rolling: merge main into `cardc` | none outside the merge | CC-00 | ~0.05/day | `make cardc-check` green after each merge |

**Collisions:**
- The hubs are CC-01 (schema) and CC-20 (registry). Every later ticket depends on one of them.
- After CC-01, each family ticket extends only its own `ir/<family>.go`.
- CC-15a creates `ir/action.go`, and 15b only appends variants in `fe_action_b.go`.
- Front-end tickets (CC-10..16) and backend tickets (CC-21..23) touch disjoint directories, so waves 1 and 2 overlap
  once CC-20 lands.
- The only files shared with main are CC-00's four hook files, and only `cardc-follow` touches them afterwards.
- Burn-down tickets each own one class's files. Ledger rows are one file per card, so two waves never edit one file.
- Peak useful parallelism is about 8 seats in wave 1 (CC-10..14, 15a/b, 16a), 6 in wave 2 and 6 in burn-down. 10-15
  seats is more than enough.

**Estimate: about 15 seat-days of tickets, about 21 with rework. About 7-9 calendar days at 10-15 seats.** The
critical path is CC-00 → 01 → 02 → 11 → 12 → 13 → 14 → 15a → 22a → 30 → 32 → 33 → burn-down, about 13 tickets.

The basis:
- **The compliance mtgish cross-check** (`oracle/mtgish`, 15,292 lines) mapped 2,612 variant names corpus-wide to
  summary grain and reached a ledgered whole-corpus diff in 2 days. 217 commits are dated 10-20/10-21 (git log,
  gorge-compliance). cardc needs 680 variants at engine grain for the proof: fewer variants, a finer target.
- **W4 on 10-03** converted every string-code family in effects and rules to named enums: 46 commits, 1,172 file
  changes, +36,587 / −20,584 lines, in one day (git log `--grep='^W4'`). It shows the pipeline's rate on mechanical
  sweeps.
- **The dsd tickets** (operand-algebras design) are this pipeline's latest comparable seat work. Medium tickets took
  2.3-10.5 h from dispatch to merge, median about 3.5 h: T04 2.6, T11 2.3, T12 3.1, T13b 4.0, T13c 9.2, T17b 3.1,
  T17c 5.4, T21 10.5 (agentctl ledger events). 8 of the 25 were superseded. Most of those were high-complexity
  representation tickets: T06 and T07 died after 11 local rounds. Hence the +35% rework allowance, and Opus review on
  the schema tickets.
- This is weeks, not months. The proof is one slice. The unbounded part is the burn-down, and it is bounded by the
  pass bar, not by the corpus.

## 7. After the proof

1. **BLB, then the Standard sets the setheads cover** (`engine/rules/testdata/setheads/games.txt`). Each set is one
   slice ticket plus burn-down. A set passes on the same bar as FDN.
2. **kgen consumes cardc, two steps.**
   1. **No kgen change:** `kgen` already reads `*cards.Card` (361 kgen files import `engine/cards`). Point it at the
      cardc registry with a `-cardc` flag. Then:
      - diff `kgen -lir` per card against the Forge registry's (`dirac/kgen/main.go:36`);
      - run the dirac lockstep (kfuzz) on the FDN slice with both engines reading cardc.

      A card dirac supports must still play identically to gorge (invariant 13): here both engines read one source.
   2. **Native:** kgen lowers IR families directly. `ir.Ref` lowers to SetExpr, which revives the operand-algebras
      ports that stalled on parsing text: `dsd-t07-defined-grammar`, superseded after 11 local rounds on PR #2795,
      and T08 and T10 behind it. `ir.Filter` lowers to `PredProg` and FilterWords (T11 and T13 are merged), and
      `ir.Value`/`ir.Cond` to `kop.Expr` (`dsd-t16-expr-conditions`, human_needed today). Each is a refiled dsd ticket
      against IR nodes instead of strings.
      - Evidence keys stay put for every card whose LIR block is byte-identical (`dirac/kgen/lirkey.go:16`; OD-5's
        additive rule).
      - A card whose LIR moves carries its re-evidence in the same PR.
3. **The Forge front end into the same IR.** The Forge-script parsers move into `engine/cardc/forgefe`. They are
   gorge's existing ones, read at `86b960f2c5`:
   - `cards.ValueHead`, `engine/cards/value_heads.go:36`;
   - `definedSpec`, `engine/effects/context.go:684`, and `definedSpecCodes`, `:3258`;
   - `compileSpec`, `engine/effects/filter_compiled.go:285`;
   - the condition readers, `engine/effects/conditions.go:296` and `engine/rules/trigger_condition.go:258`.

   The cross-check then compares IR to IR, which is exact and static, before any game. That ends the "4 places per
   count head" duplication on the engine side once the engines read IR natively.
4. **Promotion to main:** one PR from `cardc`, after the operator signs off. All must hold:
   - **(a)** FDN, BLB and `cmd4` pass the bar.
   - **(b)** With cardc as the source of every card it covers, `TestHeads`, `TestSetHeads` and
     `TestRepoDeckGamesReplayExactly` are green **without** re-pinning: golden heads, set heads and wire goldens are
     byte-identical.
   - **(c)** `scripts/dirac-gate.sh` is unedited and green.
   - **(d)** `cardc-gap` rows are 0, and the patch and fallback registers are recorded as shrink-only ratchets.
   - **(e)** On main, source selection is per card: a card is served by cardc only when it is on the
     lockstep-clean list, else by Forge. Adding that selector to `cards.Card` changes the IR cache schema, so that PR
     bumps `cacheVersion` 8 → 9 (`engine/cards/registry.go:338`) and documents it beside the version 2-8 notes there.
   - **(f)** The licensing boundary still holds: `TestPublicBoundary` refuses `.mtgish` and cardc output.
   - **(g)** The operator signs off on the archtest additions.

   Default-on for a format follows when that format's clean list reaches the Forge path's coverage.

## 8. Risks

- **mtgish emits wrong trees instead of refusing.** The 11-23 oracle-grammar evaluation
  (`scratchpad/oracle-grammar-eval.md`) found oracle-grammar refuses where mtgish guesses. The static diff and
  lockstep are cardc's only detectors, so a card is never promoted on compile success alone. The IR keeps
  oracle-grammar's shapes so a second front end can be added without a schema change.
- **Event-order equivalences may be common** (prompt order, ability order). If `equivalent` rows exceed about 10% of
  the slice after CC-30, the backend emits Forge's ability order. The bar does not relax.
- **Upstream pace:** i5jb/mtgish fixed 127 rows between `153451e` and `1553384` (PR #373). Each pin bump is a cardc
  ticket: re-run report, diff and ledger. It is the same work compliance does today.
