# Forge-hosted Commander pods with gorge in shadow

Date: 2026-11-17. Base: `origin/main` at `27ce1998e2`.
Outcome: **design only.** No harness code yet. Section 9 lists the first six tickets.
A capped local smoke (section 8) ran the gorge half that already exists.

## 1. Goal

We have a stored list of Commander decks, so we no longer need to poll for new ones.
The plan is to play 4-seat Commander games with Forge as the host, mirror every decision
Forge makes into gorge (shadow mode), and, where its tables allow, into dirac in lockstep
with gorge. Every engine difference goes to disk without stopping the game. The diffs then
feed a deduplicated, root-cause-grouped defect backlog. Dirac evidence gathering keeps going
offline at full speed, with no Forge in the loop.

## 2. What exists (verified at the base, `INFERRED` where not read)

**Forge as a game host** (forge-oracle, `forge-oracle/src/main/java/forge/oracle/`):

- `GameReplay.java` is the headless whole-game driver: Forge AI against Forge AI, one
  JSON line in and one row out. It is limited to **two seats**
  (`GameRequest.java:103-104`, "two seats are required"), **Constructed**
  (`GameReplay.java:108`, `new GameRules(GameType.Constructed)`) and two fixed seat
  references (`:114`). Turn and wall caps already exist (`Deadline`, `:250`). Forge's game
  log is printed only when the `BRIDGE_GAMELOG` env var is set (`:127`), and the driver
  writes no decision log.
- `BridgeController.java:50-62` is a `PlayerControllerAi` subclass. Forge holds the game and
  asks an external binary for gorge's *policy* answer on priority (`:719`), attackers
  (`:995`), blockers (`:1095`) and targets (`:1180`). Any other question falls back to
  Forge's AI and is counted. It is head-to-head play over one Forge state. **No gorge engine
  runs**, so it is not a shadow. Its override list is still the right map for where a
  recorder must hook.
- `ScriptedController.java:82` goes the opposite way: it scripts gorge's decisions into
  Forge. `SnapshotWriter.java:45` writes Forge state snapshots in the oracle's shape.
- Forge supports Commander natively: `GameType.Commander`
  (`forge-game/.../GameType.java:36`) and `RegisteredPlayer.forCommander`
  (`RegisteredPlayer.java:135`).

**The cross-engine farm** (gorge-compliance `cmd/xengine`, `oracle/xengine/`) is
**gorge-led**. gorge plays explore-bot games (`oracle/xengine/game/play.go:75`). Each tape
is lowered to an oracle scenario that the Forge and XMage followers replay
(`cmd/xengine/main.go:1-39`). `classify` buckets each difference, and `adjudicate` writes
`tickets/<cluster>.md` drafts (`main.go:39`). `dside` lockstep-plays a game first with
dirac when a dirac table holds its cards. The decks are generated 2-seat Standard-set decks.
There are no Commander games, no pods and no decklists.

**The gorge-side shadows:**

- `server/divergence` (`doc.go:1-17`, `shadow/shadow.go:50 Answer`) is a **policy** shadow:
  a seeded bot answers the decision a human answered. It is not an engine shadow.
- `dirac/cmd/sbv2dirac/sbv2/server.go:6-12` is the real engine-shadow pattern. The kernel
  leads, a `GorgeSide` follows action by action, and an external agent's pick is translated
  to a canonical action (`GorgeSide.CanonOfIntent`) and applied to both. A Forge-led
  follower takes the same shape with Forge as the external leader.

**Lockstep and replay:**

- `dirac/oracle` (`doc.go`, `lockstep.go:278 Lockstep`, `:291 LockstepLed`) compares option
  lists and state projections at every decision. A divergence (`lockstep.go:109`) carries
  `What` and `Field` and a field class with indices stripped (`:222 fieldClass`).
  `Implicates` (`:145`) names the suspect cards. kfuzz drives it, and `khbot` measures the
  kernel bot. `dplay` (`dirac/cmd/dplay/main.go:1-4`) plays dirac alone.
- **Dirac seats two players**: `dirac/seats_default.go:5` (`MaxSeats = 2`) and `:38`
  (`MaxObj = 236` card instances). The oracle's matchup holds 4 (`dirac/oracle/setup.go:75`),
  but kfuzz `-seats 3|4` runs only `-kernel gorge` (`kfuzz/main.go:230`). `-cmd-mini` cuts
  Commander decks down to fit the 8-bit refs (`:235`). `dirac/docs/support.md:36` says "no
  4-seat game is in the evidence runs."
- The replay formats are:
  - `engine/replay`: (Config, Log), with Config-only randomness.
  - `server/matchrecord`: `gorge-match-1`, which has a `Chance` field (`record.go:64-68`) that
    `Verify` refuses (`:200`).
  - `dirac/oracle` Repro `koracle-repro-2`: up to 4 seats, Commander, forced `Chance`, but
    `Canon` actions carry 8-bit refs (`repro.go:16-58`).
  - The chance-aware intent replay already exists as `verifyLog` (`dirac/oracle/replay.go:306`:
    `rules.NewHypothetical`, then intents, then `replay.FirstDivergence`).
- gorge can adopt an outside engine's shuffles and draws: `rules.NewHypotheticalChancePlanned`
  takes a `ShufflePlanner` callback at every library shuffle and a `ChancePlanner` for every
  other draw (`engine/rules/chance.go:52-90`). An intent can carry an explicit mana plan
  (`engine/decision/decision.go:1371`, `payment_plan.go:188 PaymentPlan.Activations`).
  The dirac kernel records chance (`dirac/oracle/realkernel.go:123`) but takes none forced
  (`INFERRED`: no forced-chance input found).
- `cmd/mtgsim` already plays gorge-hosted 4-seat Commander pods from a deck directory with
  `-verify` (`main.go:53,56,62,64`). It writes no match record.

**Companion-module operations** (not in this tree):

- A Commander pod runner with three legs: `pod4` (mtgsim 4-seat `-verify`), `duel` (1v1
  Commander) and `lock` (kfuzz 1v1 on the `mbc` kernel). It runs each game under
  `timeout 900` and `nice 19`, and keeps a ledger with ok, refused, harness and FAIL rows.
- The evidence loop: it works out the set of cards that still need evidence, deals targeted
  synthetic pools, credits cards from clean games, and files at most 4 tickets a cycle for
  each first-divergence signature.
- The farm and hunt ticks pull results home every 10 minutes.

The pod runner already plays Commander decks and can point at the stored list with no new
code.

## 3. Pipeline (Forge-led)

1. **Deal.** Choose 4 decks from the stored manifest that gorge fully supports, with a
   seeded shuffle by seed (the pod runner's rule). Order the seats by seed.
2. **Host.** Forge's `GameReplay` gets a 4-seat Commander request:
   `"format":"commander"`, with each seat's `commander` (and `partner`) named, and
   `max_turns` and `max_ms` caps. Seats are played by Forge AI or the recorder's random mode
   (section 6).
3. **Record.** Every seat's controller is a `RecordingController` (a `PlayerControllerAi`
   subclass hooked where `BridgeController` overrides). It calls the parent to decide, then
   appends one JSONL line to a `forge-shadow-1` log:
   - `header`: decks in library-build order with per-card Forge ids, commanders, seed,
     Forge ref and caps.
   - `shuffle`: the seat and the resulting order, written at the opening and at every
     shuffle.
   - `start`: who plays first.
   - `mull`: keep, or the cards put on the bottom.
   - `ask`: n, seat, turn, step, kind (priority, attack, block, target, mode, choose,
     optional, order, sacrifice and so on), Forge's offered options (the playable list the
     bridge already builds), and the answer by Forge card id, ability index and targets,
     plus the mana sources it tapped.
   - `rand`: coin, die or random choice, with its bound and value.
   - `snap`: a `SnapshotWriter` snapshot at the start of every step with an empty stack.
   - `end`: winner and reason.
4. **Shadow.** The gorge follower (`xengine fshadow`) reads the log, either streaming or
   after the game. It builds gorge's Config from the header, with ObjIDs bound to Forge ids
   by (seat, decklist index). It starts `NewHypotheticalChancePlanned`, whose shuffle planner
   returns Forge's logged order and whose chance planner returns the `rand` values. It
   answers each gorge decision with the logged Forge answer translated to a gorge `Intent`,
   paying with an explicit `PaymentPlan` of the sources Forge tapped. At every `snap` it
   compares gorge's state with Forge's through the oracle comparison that `classify` already
   uses. Two kinds of difference are recorded:
   - **Soft**, which never breaks sync: the option sets differ (an option one side offers and
     the other does not), but Forge's chosen answer is on gorge's list.
   - **Hard**: Forge's answer is not on gorge's list, a snapshot differs, gorge poses a
     different decision kind, or gorge errors or ends the game differently.
5. **On a hard divergence**, write `divergence-<n>.json` with deck ids, seed, the log offset,
   the gorge record prefix (a match record with `Chance`), both states' diff, the implicated
   cards and the decision kind. Forge keeps hosting to the end because it does not depend on
   gorge, and the whole log is kept. gorge **re-syncs** at the next step boundary where the
   stack is empty: it lowers Forge's snapshot to an oracle scenario setup and starts gorge
   from it (`engine/rules/oracle_export.go:48 RunOracleScenarioJSON`'s setup path). The
   follower then resumes, with every later finding tagged `resynced:<k>`. Re-syncs are
   capped at 3 per game. A setup the scenario grammar cannot express (a nonempty stack,
   lingering effects) ends the shadow there.
6. **dirac in the pod: not available.** With `MaxSeats = 2` and `MaxObj = 236`, a 4-seat,
   400-card game cannot run in dirac. The game row records `dirac=n/a(seats)` and no
   lockstep is attempted. dirac evidence for the same decks comes from step 7.
7. **dirac offline.** For each pod, the 6 deck pairs play 1v1 Commander duels in kfuzz
   lockstep, gorge-led, at full speed with no Forge in the loop. They run on an `edh` pool
   built from the same manifest (the `cmd5` registration pattern, `dirac/oracle/pools.go:42`),
   with the random and heuristic policies. A duel is 200 cards plus tokens, inside 236; a
   capacity stall is counted, as everywhere. Clean games credit cards through the evidence
   loop's writer. A G|D divergence goes through the same signature and backlog as F|G, and
   collection continues because `-keep-going` is the existing behaviour.

## 4. Offline replay format (reuse)

**Chosen: `gorge-match-1` with `Chance`.** It records the follower's gorge game as Match
plus Log (intents) plus the chance transcript the planners produced.
- Intents use ObjIDs, so any seat count and any deck size fit. `koracle-repro-2`'s 8-bit
  `Canon` cannot hold a 400-card pod.
- The format already exists and already has the `Chance` field.
- The only missing piece is a chance-aware verify: lift `verifyLog` into
  `matchrecord.VerifyChance` (ticket 4). Replaying is then a pure-gorge operation at engine
  speed.

For a **Forge-hosted duel** (2 seats), the same record converts to a `koracle-repro-2`
(`Chance`, `Commander`, `Actions` derived by `CanonOfIntent`). That replays the exact Forge
game in G|D lockstep **once the kernel accepts forced chance**, which it does not today. The
recommended interim path is step 7's seeded duels. The `forge-shadow-1` log itself is kept as
the source of truth, so a follower fix can re-shadow old games without Forge.

## 5. Backlog output

**Signature** (one string, hashed to 12 hex characters for the file name):
`pair | what | kind | field | cards`.
- `pair`: `F|G`, `G|D` or `F|G:resynced`.
- `what`: `options`, `answer-missing`, `snapshot`, `kind`, `terminal` or `error`.
- `kind`: gorge's `decision.Kind`.
- `field`: the first differing field with indices stripped, as `fieldClass` does
  (`Battlefield[].Tapped`, `Life[]`).
- `cards`: up to 2 implicated card names, sorted. A token becomes `token:<name>`. The names
  come from `Implicates` for G|D. For F|G they come from the one-sided options, else the
  first card field, else the last sourced action.

**Class census.** Signatures are grouped into a **root-cause class** by
`what | kind | field | primitive`, where `primitive` is the IR primitive of the implicated
ability (Forge API or keyword, from the card's IR). One brief covers a whole class, with
every signature and example listed. This extends `xengine adjudicate`'s clusters with an
`fshadow` input instead of adding a second writer.

**Rank.** Each class is scored as the sum, over its signatures, of
`games x decks-in-manifest-holding-the-card x paper`:
- `paper` is 1 for a card legal in paper Commander.
- `paper` is 0.1 for a digital or non-tournament card: an `A-` name, Perpetual, Scheme,
  Plane, Attraction, or an Un- set.

Harness classes (an answer with no mapping, a setup we cannot express) are ranked separately,
as follower tickets, never as gorge defects.

**Brief template** (what the orchestrator files later through agentctl):

```
Title: <class>: <one-line symptom>             Repo: gorge | gorge-compliance
Suspect: <file:function> from the primitive's handler, plus the playbook class row
Repro first: <in-worktree command replaying the shortest record prefix>, then a red engine
  test (go test -run '^TestX$' ./engine/rules under the 4G scope)
Evidence: N games, M decks, signatures [..], first diff (both values), Forge log offset
Corroboration: XMage or Forge rule text for the expected behaviour (CR cite)
Acceptance: the repro replays clean; the class census rows for this primitive are clean
Not in scope: other primitives in the census
```

## 6. Seat policy and bounds

- **Forge-hosted (F|G):** every seat is Forge AI, with the AI profile varied per seat
  (Default, Reckless, Cautious; `INFERRED` from Forge's AI profiles). One seat in four runs
  the recorder's **random** mode: a seeded uniform pick over the options the recorder already
  lists, and the parent AI for anything unlisted. The random seat rotates by seed. gorge's
  heuristic is **not** a Forge seat here: its answers would come from gorge, so there is
  nothing to shadow. It already covers pods on the gorge-hosted leg.
- **gorge-hosted pods** (the existing `pod4` leg) with `mtgsim -policies` mixes of heuristic,
  random and explore: a self-replay check and a second deck exerciser.
- **Duels (G|D):** kfuzz random and heuristic, on both surfaces, with mulligans 0 and 1, as
  the evidence loop runs them.
- **Bounds:**
  - Forge: `max_turns` 60 (15 rounds of four seats; the smoke game ended on turn 35) and
    `max_ms` 600000 (both through `Deadline`), with JVM `-Xmx2g`.
  - Follower: 20000 decisions.
  - Every game: `timeout 900`.
  - Each slot: 4 GB.
  - A capped game is a stall row, not a finding.

## 7. Resources

The work runs on the farm box (metal2) as its own user unit, `gorge-xedh`, at Nice 19 with
CPUWeight 1 and IOSchedulingClass idle. It is capped at CPUQuota 800% and MemoryMax 16G, and
runs 2 slots, rising to 4 if the idle-core reserve allows. Each slot runs one JVM and one
follower.
- Its directory is `/scratch/xedh/` (kit, decks, run, out, STOP). It never touches
  `/scratch/gate*`, `fy-gate`, `farm`, `hunt` or `evloop`.
- It builds its own Forge kit from forge-oracle at the pinned ref, into `/scratch/xedh/kit`.
- The existing hunt tick gains one pull of `/scratch/xedh/out/` (games.tsv, divergences,
  records, logs) every 10 minutes into its pulled directory. backlog-scope reads the
  signatures from there.
- The step-7 duels go to the evidence loop as staged cycles (the pod runner's `evcycle`
  pattern), not as a second loop.
- Locally, only a 1-game smoke runs, under the 4G/400% scope.

## 8. Smoke result (local, capped)

The 4 precons `AnimatedArmy_BLC`, `FamilyMatters_BLC`, `PeaceOffering_BLC` and
`SquirreledAway_BLC` (MTGJSON, from the manifest's precon directory) ran through
`mtgsim -seats 4 -format commander -seed 1 -verify`. The game took 1 s: 1993 intents,
35 turns, winner FamilyMatters, with a poison loss and two life losses, and `replay OK`. This
was gorge-hosted; no Forge leg exists yet. kfuzz `-kernel mbc` on two of their 1v1 pairs
refused both (`card "Bello, Bard of the Brambles" is not in the pool`), which confirms the
missing `edh` pool.

## 9. First tickets (Suspect and Repro first)

1. **forge-oracle: 2-4 seat Commander requests.**
   - Suspect: `GameRequest.java:103`, `GameReplay.java:108,114`.
   - Repro: a 4-seat request fails with "two seats are required".
   - Change: `GameType.Commander`, `forCommander`, a seat list.
2. **forge-oracle: `RecordingController` that writes `forge-shadow-1`.**
   - Suspect: `BridgeController.java:719,995,1095,1180`, the override map; the shuffle and
     random hooks are `INFERRED`.
   - Repro: a 2-seat recorded game in `DriverContractTest` style writes a header, asks and an
     end, and two runs give the same bytes.
3. **gorge-compliance: `xengine fshadow`, the follower MVP.**
   - Scope: priority, land, cast, pass, attack, block, target, mulligan, start, explicit
     `PaymentPlan`, snapshot compare. It stops at the first hard divergence.
   - Suspect: `engine/rules/chance.go:68`, `sbv2/server.go:6-12`.
   - Repro: a golden 2-seat vanilla log replays clean, and a log with one doctored answer
     reports `answer-missing`.
4. **gorge: chance-aware match records.**
   - Change: `matchrecord.VerifyChance` (lift `dirac/oracle/replay.go:306`) plus
     `mtgsim -record DIR`.
   - Suspect: `server/matchrecord/record.go:200`.
   - Repro: a test that records a `NewHypothetical` game and fails `Verify` today.
5. **gorge/dirac: an `edh` lockstep pool from the manifest's supported precons.**
   - Pattern: the `cmd5` registration (`pools.go:42`, `testdata/slices/cmd5`).
   - Repro: the section 8 kfuzz refusal.
   - Depends on the manifest landing.
6. **gorge-compliance: signature, class census and brief writer.**
   - Change: the section 5 rules added to `adjudicate`.
   - Repro: two synthetic findings with one primitive produce one brief, and a digital card
     ranks 10x lower.

Before any ticket, point the existing pod runner's deck directory at the manifest's supported
precons. That gets `pod4` and `duel` running today, and `lock` once ticket 5 lands.

## 10. Decisions (chosen) and risks

- **dirac in a 4-seat pod:** none. Use 1v1 duels offline (step 7). A 4-seat dirac build is
  a separate kernel program.
- **The follower stops or re-syncs:** the MVP stops at the first hard divergence. Turn-boundary
  re-sync comes in phase 2, with tagged findings and a cap of 3.
- **Card identity:** bind by (seat, decklist index) at genesis. Risk: Forge may build the
  command zone first or assign ids differently (`INFERRED`). Ticket 2's header records the
  ids, so the binding is a lookup, not a guess.
- **Chance alignment:** only semantic draws are logged (shuffle, coin, die, random pick).
  Risk: gorge draws in an order Forge does not. Such a mismatch is a harness class, fixed in
  the follower.
- **Mana payment:** follow Forge's sources with an explicit plan. A plan gorge refuses is
  ranked as a `payment` class (a likely real gorge defect when the sources were legal).
- **Box:** metal2, the farm box, not metal, which is for gates. **Decks:** fully supported
  decks only. Unsupported cards stay in the Commander coverage report's blocker list.
- **Corpus skew:** Forge's card scripts come from its own fork commit, and gorge's come from
  `FORGE_REF`. Each row records both, and a mismatch is a preflight failure, not a finding.
