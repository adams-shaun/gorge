# Human divergence: the human player as the bot's teacher

Status: the design as written on 2026-10-10, before implementation. Every
`path:line` below was read at `6a157c31f`, and anything not read from code is
marked **INFERRED** or **HYPOTHESIS**. Nothing here changes rules behaviour, adds an event kind, or touches
`events.Apply`.

## 0. One-paragraph summary

A human plays a vs-bot table and opts in. At match end the server replays the
match and asks a **shadow bot** (a fresh, deterministically seeded instance of
the table's own hosted policy) what it would have answered at every decision
the human made. Every place where the human's answer **A** and the shadow's
answer **B** differ *materially* becomes a **divergence**, captured in the
existing feedback snapshot format (`match.json` + `log.json`, replayable by
`cmd/repro`) plus a `divergences.jsonl` sidecar. Offline, a triage command
rebuilds the human seat's own observation stream, and the hindsight evaluator
(the `hindsight` tool, since retired) rolls both A and B to game end on sampled
hidden-information worlds. Each divergence lands in one of four buckets:
**human_better** (a bot defect), **bot_better** (a human error, or a hold or
plan the teacher cannot value), **indistinguishable** (split into
hidden-information and reasoning gaps by an omniscient arm that never leaves
the triage tool), or **untriaged**. The decided buckets are exported as
`label-v1` records, so the existing DPO trainer (private, outside this repo)
turns them into preference pairs with no code change. Human choices are
never imitation targets. In a post-game review, and only for divergences the
player is shown, the player answers which information they used, from a
checklist of the policy encoder's feature families plus free text. The bot's
choice is revealed only after they commit that answer. Free-text answers are
clustered into candidate **symbols**: named, computable predicates over the
redacted view. A symbol counts as trainable when it is computable from the seat's view
and measurably separates the buckets.

## 1. Goals and non-goals

Goals:

1. Capture a replayable, privacy-safe record of every material human/bot
   disagreement on opted-in vs-bot tables, reusing the feedback capture
   format so `cmd/repro` and `feedback.EngineAt` work on it unchanged.
2. Triage each disagreement offline with the search teacher, into buckets with
   a pre-registered statistical rule.
3. Feed teacher-decided disagreements to the existing DPO path as preference
   pairs `(state, winner ≻ loser)`. Human answers never become imitation targets.
4. Ask the human *why* without anchoring them, in a way that maps onto the
   policy encoder's feature families, and turn the free text into candidate
   encoder features.

Non-goals (v1):

- Human-vs-human tables. Their `log.json` would hold another *person's*
  hidden cards, so v1 captures only tables with exactly one human seat (§10).
- Live, mid-game triage. In-game prompts are specified (§8.4) but deferred
  (§12), because they need a live teacher and a human-seat feed that the host
  does not build today (`server/host/botenv.go:54` skips `*HumanSeat` when creating
  feeds).
- A Go-side pairwise loss in `bots/policynet`. §7.5 lists it as a follow-up.
- Any change to what the bot plays during the match. The shadow is never
  submitted.

## 2. What already exists (verified)

| Need | Existing mechanism | Where |
|---|---|---|
| Replayable snapshot of a table's match | `Registry.SnapshotForFeedback(id, seat)`: `match.json` (`FeedbackMatch`), `log.json` (`FeedbackLog`), seat-redacted `view.json`. It reads under the locks and projects outside them. | `server/host/feedback.go:169`, `:37`, `:126`, `:145` |
| Snapshot of the *current or last* match only | it picks `t.cur`, else `t.history` tail, else the last archived sidecar | `server/host/feedback.go:176-196` |
| Writing the snapshot beside a report, size cap, status line | `feedbackStore.captureSnapshot`, `maxFeedbackSnapshotBytes = 64 MiB`, `report.json` `snapshot` field | `server/gorged/feedback.go:265`, `:75`, `:98` |
| Durable feedback dir outside the wiped `-dir` | `-feedback` flag; empty value disarms the endpoint | `cmd/gorged/main.go:271`, `:449-460` |
| Load a snapshot dir back to `(Log, Config)` and replay to intent n | `feedback.Load`, `feedback.EngineAt` | `testkit/testutil/feedback/feedback.go:165`, `:436` |
| Rebuild one seat's observation stream from a log | `searchseat.RebuildFeed(cfg, l, n, actor)`: captures at every intent boundary and records the actor's logged answers | `bots/searchseat/feed.go:57` |
| Honest teacher over sampled worlds | `searchprobe.TeacherIntentChoice`, with paired `WinsOverBaseline`/`LossesToBaseline` against candidate 0 | `bots/searchprobe/teacher.go:147`, `:73-89` |
| Adaptive A/B evaluation with CIs, honest and omniscient arms | `hindsight.Evaluate(candidates, source, seed, opts)`: blocks of common-random-number worlds, stops when the top two Wilson intervals separate; `PairedDiff95`; `ClearMargin` (≥ 0.10 and paired lower bound > 0); `Seed` | the retired `hindsight` evaluator |
| Honest world source with redeal fallback; omniscient arm | `evaluateSampled`, `evaluateOmniscient` | the retired `hindsight` command |
| Sampler starvation is real | A hindsight smoke: 73 of 77 branched decisions (94.8%) were `no_world` before the redeal fallback existed | an archived hindsight measurement (`docs/design/ARCHIVE.md`) |
| Training record the DPO path reads | `LabelRecord` "label-v1" (schema 2), with `Candidates` (index 0 = bot), `TeacherChoice`, `Margin`, `View` (seat view, `Decision` nil), `Board` (traceboard), `RescoreValues` | the private `searchteacher` training command |
| Go reader of the same records | `policynet.labelRecord` mirror, `AcceptedLabelSchemaVersions = {1,2,3}`, `Load` | `bots/policynet/loader.go:37`, `:53`, `:154` |
| DPO pair construction (off-repo) | `build_pairs`: Q = (8·values + 64·rescore)/72, chosen = argmax Q, rejected = candidates ≥ `--gap` (default 0.06) below; records without `rescore` are skipped | the private DPO trainer (read 2026-10-10) |
| Prompt rendering and game-level split (off-repo) | `render.to_example` (game key = `pair/game_index/seed`), `prep.py` splits by game | the private DPO trainer |
| Human seat, caretaker | `HumanSeat` with a deterministic caretaker bot. A timed-out or cancelled decision is answered by the caretaker, and `caretakerFires` counts each such answer | `server/host/humanseat.go:23`, `:40`, `:228` |
| How hosted bots are built | `bots.New(name, bots.Options{Seed, AutoPayMana, Deps, DecisionDeadlineMS, ...})`, wrapped by `NewBotPolicySeatWithDeps`. The served deadline is 2000 ms | `bots/registry.go:104`, `:45`; `server/host/bot_policy.go:53`, `:69` |
| Seat dispatch order for one decision | HumanSeat → `EnvSeat && WantsEnv` (Env with honest root) → `BoardSeat` → View | `server/host/match.go:449-516`, `server/host/botenv.go` (envData doc) |
| Honest-root seed derivation | `bots.RootSeed(seatSeed, seq)` | `bots/env.go:53` |
| Bot state across decisions | `seat.Bot` carries an RNG plus scratch buffers, and no other cross-decision state | `bots/heuristic/bot.go:30-70` |
| Bot board reads only seat-legal facts | `BoardFromGameInto` doc: public facts plus the seat's own hand and graveyard | `bots/botpolicy/combat.go:115-145` |
| Match-end hook | `Options.OnMatchEnd`. It runs on the match goroutine, **must not call back into the Registry or block**, and `cmd/gorged` does not set it today | `server/host/registry.go:40-47`, `:210-213`; `server/host/snapshot.go:65` |
| Undo truncation hook | `OnRewindFunc(t, k, toIntent, headSeq)`, `Registry.Undo` | `server/host/undo.go:73`, `:159` |
| Historical seat view | `GET /api/tables/{t}/matches/{k}/view?seq=N&seat=S` → `ViewAtSeat` | `server/host/httpapi/handler.go:95`, `server/host/viewat.go:66` |
| Seat fence | `claimForTable`: 401 without a claim, 403 for another table | `server/host/httpapi/rest.go:113` |
| vs-bot game creation | `CreateGameRequest{format, human_deck, bot_deck, bot_policy, mulligans}` | `server/host/httpapi/rest.go:383` |
| Client match scrubber | DVR cursor over the match's events (`dvr.ts`); finished `/m/:match` is a spectator replay | `web/src/lib/dvr.ts`, `web/src/routes/Table.svelte:53-75` |
| Client auto-answers | `autopilot.ts` passes priority for the player | `web/src/lib/autopilot.ts:8` |
| Policy encoder feature families | v1 dense and sparse (`EncodeState`), mz rows (`mzState`), own-library (`ownLibState`), diagnostic hidden-info sets refused for checkpoints | `bots/policynet/policynet.go:157`, `bots/policynet/features.go:13-87`, `:263`, `bots/policynet/ownlib.go:41` |
| Layering fences | `host`, `server/host/httpapi`, `cmd/gorged` must not depend on `testkit/testutil`. Only allow-listed packages may import `time` | `internal/archtest/arch_test.go:150-160`, `:115-142` |
| Seat privacy rules | credentials bind table and seat; every seat route goes through `claimForTable` | `docs/agents/runtime-behavior.md:67-76` |

What the measured history says (maintainers' records, **not re-measured here**):
an imitation policy trained on 17lands human choices scored above the bot on a
staged benchmark, and lost in play when used as a cast veto (−1.23 pts, CI
excluding zero). "Agreement with humans again failed to predict wins." That
record is the reason for this design's central rule: **a human answer reaches
training only through the teacher's verdict, never on its own.**

## 3. Definitions

- **Human decision.** An intent index `n` in a match's log where the pending
  decision's `Player` is the human seat. The answer must have been accepted
  from a human submit: not by the caretaker (`server/host/humanseat.go:228`), not by the
  client's autopilot (§5.2), and not later rewound by an undo.
- **Shadow answer B(n).** The intent a *fresh* instance of the shadow policy
  returns for that decision, given exactly the inputs the host's dispatch
  would hand that policy (§5.4). Seeded by
  `ShadowSeed(matchSeed, seat, n)`, with no wall-clock deadline. B is
  therefore a pure function of `(match.json, log.json, n, policy)`, and
  the triage tool recomputes it and checks it against the recorded B.
- **Divergence.** A human decision where `A ≠ B` *materially* (§5.5).
- **Teacher.** `hindsight.Evaluate` over honest sampled worlds, with the
  frozen default bot as the rollout policy for every seat
  (`bots/searchprobe/teacher.go:211-222`).

## 4. Architecture

```
live match (host, opted-in vs-bot table)
  └─ per-match provenance ledger: caretaker / client-auto marks per intent,
     truncated on undo                                   [host]
match end → OnMatchEnd (non-blocking enqueue)            [cmd/gorged]
  └─ divergence worker (one match at a time, background)
       ├─ Registry.SnapshotMatch(id, k, seat)  → match.json, log.json
       ├─ Registry.ShadowScan(id, k, seat, policy) → divergences.jsonl
       └─ <divergence-dir>/<match-id>/
review UI (seat-authorised) ── GET list (no B) ── POST answer → reveals B
       └─ answers.jsonl
offline:  cmd/divergence triage  → triage.jsonl
          cmd/divergence export-labels → label-v1 JSONL (DPO)
          cmd/divergence export-reasons → reasons JSONL  → clustering → symbols
```

All new server-side state is host-side metadata of the same kind as
`m.snaps`. None of it is replay input, none of it is an event, and a table
without opt-in runs byte-identically to today.

## 5. Capture

### 5.1 Opt-in and scope

- Server: a new `-divergence <dir>` flag in `cmd/gorged`. Empty disarms
  everything, the same shape as `-feedback` (`cmd/gorged/main.go:449-460`). The
  directory lives outside `-dir`, for the reason `-feedback` does: deploys wipe
  `-dir`.
- Game: `CreateGameRequest` gains `teach_bot bool`, carried into a new
  `TableConfig.TeachBot` with a `json:"teach_bot,omitempty"` tag, so existing
  sidecars decode unchanged. The client shows it as an explicit opt-in on the
  play-vs-bot form. The default is off.
- Eligible tables: `TeachBot` set, `len(Humans) == 1`, and the server armed.
  Anything else is never scanned.

### 5.2 The live provenance ledger (the only hot-path change)

The match log alone cannot say whether an intent at a human seat was a
deliberate human choice. Three cases must be excluded, and only the host knows
about them:

1. **Caretaker answers.** `HumanSeat.caretakerFires` increments on a timeout
   or cancel (`server/host/humanseat.go:228`). The ledger marks intent `n` as
   caretaker when the count moved across the answer.
2. **Client autopilot answers.** The client auto-passes
   (`web/src/lib/autopilot.ts`). The client adds `?auto=1` to the intent POST
   when autopilot produced the intent. The handler forwards it as a boolean
   into a new `Registry.SubmitIntentFrom(id, k, player, in, auto bool)`, or an
   options struct. **Not** into `decision.Intent`: intents are recorded in the
   log, and this flag must stay out of replay input.
3. **Undo.** On rewind to `toIntent` the ledger truncates every entry
   `≥ toIntent`. This happens at the same site that fires `OnRewind`
   (`server/host/undo.go:73`).

Representation: one `[]uint8` per match indexed by intent (bit 0 caretaker,
bit 1 auto), appended inside the existing `attempt` exclusive section
(`server/host/match.go`, the `m.locked` block that calls `m.e.Submit`). It costs one
append per intent and allocates only on slice growth. A refused submit appends
nothing. The ledger is always built, because it costs nothing and keeps the
table config out of the hot loop. It is read only by `ShadowScan`.

### 5.3 Match-end capture

`OnMatchEndFunc` must not re-enter the Registry or block
(`server/host/registry.go:40-47`). So the gorged hook only does a non-blocking send of
`(table, k)` to a bounded channel (capacity 64; overflow increments a dropped
counter, logged once a minute). A single worker goroutine owns the rest:

1. `Registry.SnapshotMatch(id, k, seat)`: a **new** variant of
   `SnapshotForFeedback` that names match `k` explicitly through
   `r.lookup(id, k)` (`server/host/viewat.go:165`). The current function snapshots
   `t.cur` (`server/host/feedback.go:176-196`). By the time the worker runs, a
   single-shot vs-bot table has no next match, but naming `k` removes the race
   for every table shape. Body: the existing function's copy section, factored
   so that both entry points share it. The `view.json` half is skipped
   (`seat == nil`), because the end-of-game view is useless here and the review
   UI asks for historical views by `seq`.
2. `Registry.ShadowScan(id, k, seat, policy)` (§5.4).
3. Write `<dir>/<match-id>/`, with match-id = `<UTC ts>-<table>-m<k>`, generated
   by the server as `feedbackID` is (`server/gorged/feedback.go:403`):
   - `match.json`, `log.json`: byte-for-byte the feedback shapes, so
     `go run ./cmd/repro <dir>` verifies the replay (exit 0) unchanged.
   - `report.json`: `{"received", "snapshot": "captured: ...", "text": "divergence capture"}`,
     so `feedback.Load` finds its optional status line
     (`testkit/testutil/feedback/feedback.go:104-111`).
   - `divergences.jsonl` (§5.6).
   - Nothing at all when the scan found zero divergences. The directory is
     created only when there is something to keep.
   - The same 64 MiB cap and the same semantic-partial rule as
     `captureSnapshot` (`server/gorged/feedback.go:298-346`). A partial capture
     (`tokens_unread`) is written but marked, and triage skips it as
     `untriaged: snapshot_partial`.

Cost **HYPOTHESIS**: for a non-search shadow policy the scan is one replay of
the match (milliseconds) plus one heuristic `Decide` per human decision. For a
search policy (`bots.Lookup(p).Search`, `server/host/registry.go:370`) each shadow
decision is a full search. The worker therefore takes the registry's
search-slot gate (`searchGateFor`, `server/host/registry.go:366`) around each searched
shadow decision, so it never competes with live tables beyond `SearchSlots`.
This cost is to be measured: scan wall time per match for `bot` and for one
search policy.

### 5.4 The shadow answer

`server/divergence/shadow` (new; it may import `bots`, `seat`, `searchseat`,
`botpolicy`, `view`, `rules`, and never `host` or `testkit/testutil`):

```go
// Answer returns the intent policy would give at e's pending decision for
// seat, built exactly as host's dispatch would build that seat's input.
func Answer(e *rules.Engine, feed *searchseat.Feed, setup searchprobe.PublicGame,
    policy string, seed uint64, autoPayMana bool, deps bots.Deps) (decision.Intent, error)
```

- A fresh `bots.New(policy, bots.Options{Seed: seed, AutoPayMana: autoPayMana,
  Deps: deps, DecisionDeadlineMS: 0})` for every call. Deadline 0 is the
  determinism contract (`server/host/bot_policy.go:57-69` and `server/host/match.go`'s
  `BotUnboundedDecisions` comment: a deadline bail-out makes two runs diverge).
  Fresh per call: `seat.Bot`'s only cross-decision state is its RNG
  (`bots/heuristic/bot.go:30-31`), so a per-decision seed loses nothing. Any policy whose
  answer depends on state from earlier decisions is **INFERRED** absent.
  The parity test (§11, shadow parity risk) is the check.
- Dispatch mirrors `projectNextData` (`server/host/match.go:449-516`) in the same
  order. If the seat is an `EnvSeat` and `WantsEnv(&dc)`, build the Env as
  `m.envData` does: the feed, `searchseat.HonestRoot` with seed
  `bots.RootSeed(seed, d.Seq)` (`bots/env.go:53`), and the Board. Else a
  `BoardSeat` gets `botpolicy.BoardFromGame`. Else a View gets
  `view.ProjectForControlled(..., view.Seat, controlledSeats, &dc)`. The
  payment extension is ensured when the policy is a `PaymentPlanConsumer` or
  `autoPayMana` is set, as the host does.
- `ShadowSeed(matchSeed uint64, seat state.PlayerID, n int) uint64`
  is a splitmix avalanche of the three, the same construction as
  `hindsight.Seed` (the retired `hindsight` tool), with its own
  constants.
- The engine `e` is a **replay walk's** engine, never the live one, and
  `Answer` must not mutate it. HonestRoot builds a redeal, and `EnsurePaymentActions`
  is a pure derived read (see `bots/searchseat/feed.go:72-84`).

`Registry.ShadowScan` copies `(m.cfg, m.e.L.Clone(), ledger)` under the match
lock, as `SnapshotForFeedback` does (`server/host/feedback.go:201-281`), then outside
the lock runs one `replay.Walk` over the copy. When the policy is an EnvSeat it
also keeps one `searchseat.Feed` for the human seat, calling `Observe` at every
boundary and `RecordAnswer` for the seat's logged intents, which is the
`RebuildFeed` loop (`bots/searchseat/feed.go:57-92`). At each eligible
human decision it calls `shadow.Answer`. The walk visits a boundary **before**
its Submit (`feed.go:68-73`), which is exactly the state the human answered.

### 5.5 Materiality: which differences are divergences

`server/divergence/equiv.go`, `Material(d *decision.Decision, a, b decision.Intent) (bool, string)`:

- Equal `Choices` (as sets, or as sequences for ordered kinds): not material.
- Kinds that are never material in v1: a priority decision whose two answers are
  both bare mana-source activations (`Kind == "activate" && Cost == ""`; the
  same test as `bareManaSources`, `bots/searchseat/searchseat.go:260`); and
  differences confined to `Intent.Payment`/`Announce` while the chosen
  option is the same (a payment plan is not a strategic choice).
- Name-equivalence: two answers whose chosen options map to the same
  canonical tuple `(option.Kind, object name, ability index, mode, amount,
  target object name or player)` are equivalent, for example tapping the
  left Mountain instead of the right one. Object names are taken from the
  decision's own options plus the walk engine's objects, which is
  the seat's own knowledge at that decision.
- Everything else is material. The reason string (`"cast-vs-pass"`,
  `"attack-set"`, `"block-set"`, `"target"`, `"mode"`, `"other"`) is
  recorded and drives the review ordering (§8.2).

The predicate is deliberately conservative toward *material*: a false
"material" costs one triage, and a false "equivalent" loses data silently.

### 5.6 `divergences.jsonl` record

One JSON object per line, sorted by intent index. The writer is
`server/divergence/record.go`:

```json
{"record_type":"divergence-v1","schema_version":1,
 "id":"<match-id>/<n>","table":"t7","match":1,"seat":0,
 "intent":118,"event_seq":2291,"decision_seq":604,"turn":9,"kind":"attackers",
 "policy":"bot","shadow_seed":1234567890123,
 "options":[ ...decision.Option as offered to the human, PaymentActions stripped... ],
 "human":{"choices":[0,2],"label":"Attack with Grizzly Bears, Hill Giant"},
 "shadow":{"choices":[2],"label":"Attack with Hill Giant"},
 "material":"attack-set"}
```

`event_seq` is the last event before the decision was answered (`before-1`
in the play loop), so the review UI can open
`view?seq=<event_seq>&seat=<seat>`. The labels are rendered by the
`choiceSummary` routine `cmd/repro` already uses (`cmd/repro/main.go:391`),
moved into `server/divergence` so that host can use it.

## 6. Offline triage

### 6.1 Command

`cmd/divergence triage -in '<dir>/*' -out triage.jsonl [-block 16 -max-rollouts 128 -attempts N -redeal -omniscient -workers W]`

New command. It may import `testkit/testutil/feedback`: only host, httpapi
and gorged are fenced from testutil (`internal/archtest/arch_test.go:158-160`).
It must not import `time` unless it is added to the allowlist
(`arch_test.go:115-138`), so wall-clock measurement stays out of the records,
as in the `searchteacher` command.

For each match directory:

1. `feedback.Load(dir)` → `(l, cfg, meta)`. If `meta.Report` starts with `partial`,
   every divergence becomes `untriaged: snapshot_partial`.
2. **One** replay walk per match, not one per divergence. A new
   `searchseat.WalkFeed(cfg, l, actor, stops []int, visit func(e *rules.Engine, f *Feed, n int) error)`
   is `RebuildFeed`'s loop with a visit callback at each requested intent
   index, so the cost is O(game) per match rather than O(game × divergences).
   The visitor must not retain `e` or `f`. It clones what it needs.
3. At each stop:
   - Recompute `B' = shadow.Answer(...)` with the recorded `shadow_seed`. If
     `B' ≠ B`, the result is `untriaged: shadow_diverged`. This is the triage
     analogue of `cmd/repro`'s DIVERGED: a corpus pin move, a policy change, or
     a determinism defect.
   - Candidates: `[B, A]`, with the shadow at index 0, the "recorded bot answer"
     convention `hindsight.Evaluate` assumed (the tool is retired). Each is turned into
     observer-local actions with `Collector.IntentActions(d, in)`
     (`bots/searchprobe/action.go:48`).
   - **Honest arm**: `hindsight.Evaluate` with an honest `WorldSource` built
     exactly as the retired `hindsight` command's `evaluateSampled`: seed `hindsight.Seed(base,
     matchHash, n, block)`, with `Redeal` on by default.
   - **Omniscient arm** (`-omniscient`, default on): `evaluateOmniscient`'s
     source (the retired `hindsight` command), repeated clones of the real walk
     engine with `Clairvoyant: true`. Its output **never** reaches an export or
     the UI (§10).
4. Write one `triage-v1` record per divergence: bucket, sub-bucket, both
   `hindsight.Evaluation`s, the options used, and provenance (`git rev-parse
   HEAD`, `FORGE_REF`). `testkit/testutil/feedback` already shells out to git
   for the root.

### 6.2 Buckets (pre-registered)

Let `rA, rB` be the honest-arm win rates of A and B, and `[lo, hi]` the
paired 95% interval of A − B: `PairedDiff95` of A's paired wins and losses
against candidate 0, which is `OptionResult[1].PairedDifferenceLow/Hi`
(`hindsight.go:224-237`).

| bucket | rule | meaning |
|---|---|---|
| `human_better` | `rA − rB ≥ 0.10` and `lo > 0` (this is `ClearMargin`, `hindsight.go:344`) | the teacher sides with the human: **bot defect** |
| `bot_better` | `rB − rA ≥ 0.10` and `hi < 0` | the teacher sides with the bot: **human error, or a hold or plan the teacher cannot value** (§6.3) |
| `indistinguishable` | the evaluation completed at `MaxRollouts` (or the CIs separated without clearing 0.10), and neither rule fired | sub-bucketed below |
| `untriaged` | `SamplerStatus != "ok"` (`no_world`, `rollout_not_terminal`), `shadow_diverged`, `snapshot_partial`, a replay divergence, or a candidate that does not map | excluded from all exports; counted |

Sub-buckets of `indistinguishable`, from the omniscient arm:

- `hidden_info`: the omniscient arm clears in either direction. The right answer
  depended on hidden cards. The honest sampler could not tell, and the human may
  have had a better prior (tells, deck knowledge) than the sampler's.
- `reasoning_or_tie`: the omniscient arm does not clear either. Either a true
  tie, or a difference that the teacher's rollouts to game end with the bot as
  rollout policy cannot resolve.

The 0.10 threshold and the CI rule are `ClearMargin`'s rule, reused
unchanged so the label semantics match the existing hindsight work. Changing
them is a maintainer decision, recorded before a run.

### 6.3 The teacher's bias is asymmetric, and the export respects it

Every rollout continues with the frozen heuristic bot for *both* seats
(`teacher.go:211-222`). A human move that only pays off under a multi-turn
plan the bot would not carry on is therefore undervalued.

- `human_better` is **robust**: A beat B even though the bot played the
  continuation after A. These are the highest-precision pairs.
- `bot_better` is **biased against plans and holds**. These pairs are exported,
  but each carries a `plan_flag` set when the human's elicitation (§8) ticked
  `turn_plan` or `opp_behaviour`. The default DPO export excludes flagged
  `bot_better` pairs (`-include-flagged` overrides). This is an analytic
  filter on an already teacher-decided pair. It never promotes a human answer
  into a label.

### 6.4 Budgets and caps

Triage is an offline batch job and runs under the heavy-job discipline (one
heavy job at a time, inside a systemd scope). Each *test* of the command fits
the per-test budget in AGENTS.md (4 GB, 4 vCPU, 1 minute) by using
`-block 4 -max-rollouts 8` on a short fixture.

## 7. Training use

### 7.1 Export to `label-v1` (DPO input)

`cmd/divergence export-labels -triage triage.jsonl -out human.labels.jsonl[.gz] [-buckets human_better,bot_better] [-include-flagged] [-include-ties]`

One record per exported divergence, using the `LabelRecord` JSON keys
(from the `searchteacher` command). The mapping:

| field | value |
|---|---|
| `record_type`, `schema_version` | `"label-v1"`, `2` |
| `pair` | `"human:" + <match-id>`, a prefix no generator emits today, so consumers can include or exclude by prefix |
| `game_index`, `seed` | `match`, the match seed |
| `decision_sequence`, `seat`, `kind`, `turn` | from the divergence |
| `board` | `traceboard.Project(botpolicy.BoardFromGame(e.G, e, seat))` on the walk engine, the same projection `teach` records |
| `view` | the seat view with `Decision` nil and `Round` filled, the same function as `seatView` (`labels.go:121-126`) |
| `options` | the decision's `Options` (no `PaymentActions`) |
| `candidates` | `[{choices: B, bot: true, value: rB, worlds: n}, {choices: A, bot: false, value: rA, worlds: n}]` |
| `teacher_choice` | `1` for `human_better`, `0` for `bot_better`, `0` for ties (only with `-include-ties`) |
| `bot_index`, `margin` | `0`, `max(rA, rB) − rB` (the writer's definition, `labels.go:58-72`) |
| `worlds`, `attempts`, `accepted`, `horizon` | from the honest evaluation; `horizon` 0 (rollouts run to game end) |
| `outcome`, `outcome_known` | the human seat's real result in that match |
| `rescore_values` | `[rB, rA]` again, so `dpo.py`'s `build_pairs` (which skips records without `rescore`) reads Q = r. The duplication is documented in the record as `rescore_worlds: 0` |
| additive keys (ignored by every current reader) | `"source":"human-divergence"`, `"human_index":1`, `"bucket"`, `"sub_bucket"`, `"plan_flag"`, `"divergence_id"`, `"checklist"` (ids only, never free text) |

Effect in `dpo.py` with its default `--gap 0.06`: every exported
`human_better` record yields `A ≻ B`, and every `bot_better` record yields
`B ≻ A`, because the bucket rule's 0.10 already exceeds the gap. That is the
requested `(state, A ≻ B)` pair for bot defects, plus the mirror pair for human
errors, with **no change to the off-repo trainer**. `prep.py` splits by
`pair/game_index/seed`, which is one human match, so no match leaks across the
train/eval split.

`policynet.Load` reads the same file as ordinary teacher labels. `teacher_choice`
is the teacher's verdict, so the Go policy net also never trains on a raw
human answer. A test pins this.

### 7.2 What is never exported

- `untriaged` records; `indistinguishable` records unless `-include-ties`
  (and then with `teacher_choice` 0, so they teach "keep the bot", never "copy
  the human"); omniscient-arm values; `log.json` content; free text; player
  names (`FeedbackMatch.PlayerNames`).

### 7.3 Gating

Pre-registered before the first training run that uses human pairs:
the DPO adapter trained on `searchteacher` pairs plus human pairs, against the
same recipe without human pairs, on the existing held-out evaluation
(`dpo-pipeline.sh` eval on the fresh `t1` set) and on the 64-world regret
measure the program already uses. The human pairs are adopted only if the
paired difference's lower bound is above zero. Volume **HYPOTHESIS**: the
number of `human_better` divergences per human game is unknown. A
simulated human measures the rate first (§12), and the first
real games measure it again.

### 7.4 Why not imitation

Spelled out because the request invites it: an imitation target teaches
"what humans do", and the measured history says that does not turn
into wins (§2). A preference pair whose winner the teacher chose teaches "what
wins under the teacher". The human's contribution is *search guidance*: the
human proposed a candidate the bot never considered, and the teacher verified
it. In `searchteacher`, a candidate the bot never considered cannot be
scored at all, because `searchseat.candidates` builds the set from the bot's
own answer (`bots/searchseat/searchseat.go:445-511`). Human divergences
are therefore a new source of candidates as well as of labels.

### 7.5 Follow-ups (not v1)

- `policynet.LoadPreferences`: a Bradley–Terry pairwise target beside
  `PPO`/`Visits` (`bots/policynet/loader.go:122-126`), so the Go net can
  use the pairs directly.
- Feeding human candidate As back into `searchseat` as extra root candidates
  (a "human prior widen"), measured with the existing `-prior-widen` machinery.

## 8. Elicitation

### 8.1 Principles

1. **No anchoring.** The bot's answer B and the triage verdict are not present
   in any client-side state until the player has committed their reasons. This
   is enforced server-side (the list endpoint never returns B) and client-side
   (a pure state machine, `list → selected → answering → revealed →
   reconsidered`, whose pre-commit states have no field for B, pinned by a unit
   test).
2. **Only redacted information.** Everything shown is either the seat's own
   historical view (`ViewAtSeat`) or the seat's own offered options and
   choices.
3. **Low volume.** Post-game review is the default surface. A review shows at
   most 5 divergences per match, chosen by §8.2.
4. **Opt-in, skippable, never blocking play.**

### 8.2 Which divergences are shown

Before any triage data exists, divergences are ranked by `material` reason
(cast-vs-pass > attack-set > block-set > target > mode > other), then by
turn, latest first. Once `triage.jsonl` history exists, a per-(kind, material,
policy) estimate `P(decided)` (the share of past divergences of that class
that landed in `human_better` or `bot_better`) ranks first. This is the
**confidence** signal the request asks for. It is a data-driven prior over
divergence classes, which is honest about the fact that the heuristic bots
expose no calibrated confidence of their own (**INFERRED** from `seat.Bot`'s
fields, `bots/heuristic/bot.go:30-70`: no score is retained). It is computed offline by
`cmd/divergence prior` and shipped to the server as a small JSON file
(`-divergence-prior <path>`). Without the file the ranking falls back to the
material order.

### 8.3 Flow (post-game review)

The match has finished. The client still holds the seat claim
(`/t/<id>?seat=N&token=…`), and `claimForTable` checks only the table
(`server/host/httpapi/rest.go:113-128`), so the seated client can call seat routes
after game over.

1. The client calls `GET /api/tables/{t}/matches/{k}/divergences?seat=S`
   (seat-fenced). It receives up to 5 items:
   `{id, turn, kind, event_seq, options, human_label, human_choices, answered}`.
   **No shadow field.**
2. The player picks one. The client opens the DVR at `event_seq`, using
   `view?seq=&seat=` for the board as the player saw it, and highlights the
   player's own choice in the option list.
3. The question form:
   - "Which information did you use?": the checklist in §8.5 (multi-select,
     at least one tick or the free text required).
   - "Anything else?": free text, capped at 2 KiB.
   - "How sure were you?": 1–5.
4. `POST /api/tables/{t}/matches/{k}/divergences/{id}/answer` with
   `{seat, checklist[], text, confidence}`. The response is the reveal:
   `{shadow_label, shadow_choices}`. Answering again returns 409 with the same
   reveal (idempotent, first answer wins).
5. After the reveal: "Knowing the bot would have chosen this, would you
   change your answer?" (yes / no / unsure).
   `POST .../reconsider {seat, value}`. This post-reveal self-assessment is
   recorded beside the triage verdict. It is analytics only, never a label.

`answers.jsonl` in the match's divergence directory gets one line per answer
and per reconsider: `{divergence_id, checklist, text, confidence,
reconsider}`. No wall-clock time is written, and the server's sequence order
is the file order.

Triage verdicts are not shown to the player in v1. They arrive hours later,
from an offline batch. Later work may surface "the teacher agreed with you"
on a review page, as honest-arm results only.

### 8.4 In-game chip (deferred)

An opt-in play setting, off by default. When the live shadow (a later host
change: the shadow runs asynchronously after a human Submit, on a copy of the
parked View or Board) finds a material divergence whose class `P(decided)` is
at least 0.5 (**HYPOTHESIS** threshold), a non-modal chip appears in the
feed. Clicking it runs the same flow as §8.3, with the board shown from the
historical view. At most 2 chips per match, and never while the player holds a
pending decision. The bot's choice is still revealed only after the answer.
Revealing it mid-game teaches the player during the game, which contaminates
their later decisions as data. This mode must therefore tag every later
divergence in that match `post_reveal: true`.

### 8.5 Checklist = encoder feature families

Each item maps to the encoder rows it corresponds to, so a tick can be compared
with what the policy can see. "Playable" means computable from the seat's
redacted view, which is the test `FeatureSet.Diagnostic` enforces for
checkpoints (`bots/policynet/features.go:84-87`).

| id | label shown | encoder rows today | playable |
|---|---|---|---|
| `my_hand` | My hand / what I could cast | v1 hand bag; `handCard("hand", ...)` mv, castable, api, kw (`features.go:363`, `:422-452`) | yes |
| `my_mana` | My available mana | `mz|me|avail` (`features.go:388`) | yes |
| `opp_mana` | Opponent's untapped mana | `mz|opp|avail` (`features.go:393`) | yes |
| `creatures` | Creatures in play: size, keywords, tapped/sick | `perm("bf-me"/"bf-opp")` rows (`features.go:351`, `:356`) | yes |
| `combat_math` | Combat math / lethal | canattack/canblock aggregates, `mz|me|lethal-unblocked` (`features.go:395-415`) | yes |
| `life` | Life totals | `mz|lifediff`, `me|life`, `opp|life` (`features.go:405-407`) | yes |
| `stack` | What's on the stack | `mz|stack|*` (`features.go:368-380`) | yes |
| `graveyards` | Graveyards / exile | v1 graveyard and exile bags (`policynet.go:219`, `:240-256`) | yes |
| `my_library` | What's left in my library | `mz-ownlib` (`bots/policynet/ownlib.go:41`) | yes (`mz-ownlib` only) |
| `timing` | Turn, phase, timing | v1 dense step/phase/active/priority (`policynet.go:159-177`) | yes |
| `card_text` | A specific card's rules text | card-name hash rows only. Rules text reaches only the LLM renderer | partial |
| `opp_hand_guess` | What I think the opponent is holding | **none playable**. Only the diagnostic `mz-opphand`, which reads the real hand (`features.go:463`) | no (belief) |
| `opp_behaviour` | What the opponent did or didn't do earlier | **none**. Every encoder is history-free | no (history) |
| `opp_deck` | The opponent's deck / archetype | **none** in the encoder. The sampler's pool uses the decklist | no |
| `turn_plan` | My plan for the next turns | **none** (planning, not a feature) | n/a |
| `other` | Something else (free text) | | |

The table also reads as a diagnostic matrix. A tick on a *playable* family on
a `human_better` divergence says the information was available and the gap is
weighting or reasoning. A tick on a non-playable family says the encoder is
missing a family, which is the input to §9.

## 9. From free text to candidate symbols

A **symbol** is a named, deterministic function `func(v view.View, seat
state.PlayerID) float32` over the seat's redacted view, or over the seat's
own observation history (the `Feed` frames), with a family and a status
(`proposed`, `implemented`, `adopted`, `rejected`).

Pipeline:

1. `cmd/divergence export-reasons` writes one line per answered divergence:
   `{divergence_id, bucket, sub_bucket, kind, checklist, text, confidence,
   reconsider}`. The view is not included; it is reachable by id. This is the
   only export that carries free text, and it stays on the training box,
   never in the repo.
2. Clustering is off-repo, in the DPO trainer's environment (it already holds the model
   weights and torch). Steps: embed the texts, cluster them, then have the LLM
   name each cluster and **propose a predicate in terms of view fields** (for
   example "opponent kept ≥ 2 untapped Islands and passed with ≥ 2 cards in
   hand"). This step is research tooling; the repo gets its output, not the
   code.
3. A maintainer reviews a cluster and files it as a symbol entry in
   `server/divergence`. A registry entry is code, so it is
   deterministic, testable, and runs inside the redaction boundary.
4. `cmd/divergence symbols-eval -triage ... -symbols name1,name2` evaluates
   each symbol over the triaged divergences on the walk engine's seat view:
   - **separation**: the AUC of the symbol value for the cluster's divergences
     against the rest, and for `human_better` against `bot_better`;
   - **coverage**: the fraction of `indistinguishable/hidden_info` divergences
     where the symbol fires (the hidden-info family's proxy);
   - **honesty**: the symbol is evaluated on the seat view only. A symbol
     that needs `view.Omniscient` is refused at registration.
5. A symbol with separation AUC ≥ 0.65 on held-out matches (**HYPOTHESIS**
   threshold, to be set before the first evaluation) becomes a candidate encoder
   row. It is added as a **new appended `FeatureSet` ordinal**, never by
   changing an existing set (`features.go:56-58`: "ordinals are part of what a
   checkpoint's feature set names, never renumbered"). The value test that
   adopts it is the existing train-and-gate loop. That is "trainable-symbol
   identification": a symbol is trainable when it is honest, separates the
   buckets, and the net trained with it beats the net without it on the
   held-out gate.

History-based symbols (`opp_behaviour`) need the feed's frames rather than a
single view. They are valid, because the seat observed those frames, but they
need an encoder that reads history. That is a known gap, recorded here and not
solved here.

## 10. Privacy

Rules, each with its enforcement:

| rule | enforcement |
|---|---|
| Only the consenting human's seat is analysed and exported | `TeachBot` opt-in, `len(Humans) == 1`, and `ShadowScan(seat)` takes the human seat only |
| `log.json` is omniscient (it is replay input, as for feedback) and stays server-side and offline | it is never served by any endpoint. Only the triage tool reads it, and only to replay |
| Anything shown to the player is seat-redacted | the board comes from `ViewAtSeat` (`server/host/viewat.go:66`, seat visibility). Options and choices are the seat's own offered options. B comes from a policy whose inputs are the seat's view, board or honest root (`bots/botpolicy/combat.go:115-124`, `bots/env.go:34-41`) |
| No clairvoyant output leaves triage | the omniscient arm's values are written only to `triage.jsonl` (an offline file). every evaluation in a triage record carries an `arm` field (`honest` or `omniscient`), and `export-labels` reads only `arm == "honest"` values. It refuses any record whose exported values carry another arm. This mirrors the label writer's own oracle marker, which a consumer filters on (`labels.go:76-82`). The host never links the private companion module's clairvoyant package (`arch_test.go:163-175`), and nothing here changes that |
| Seat routes are fenced | new routes go through `claimForTable` plus a `seatFromQuery`-style check that the claim's seat equals `seat` (`server/host/httpapi/rest.go:113-150`) |
| Training records are redacted | `view` is the seat view with the decision nil, and `board` is the traceboard projection, the same contract as `LabelRecord.View` (`labels.go:44-50`). A test asserts no opponent hand card name appears in any exported record, mirroring the label writer's documented property |
| Free text stays out of training records | only checklist ids enter `export-labels`. Free text goes only to `export-reasons`, kept on the training box |
| No player identity | `PlayerNames` is dropped from every export. The `match.json` on disk keeps it, as feedback does |
| The bot's choice is not revealed before the answer | the list response has no shadow field, and the client state machine cannot hold it pre-commit, both pinned by tests |

## 11. Invariants, risks, open questions

- **Determinism.** No new event, no new `events.Kind`, no write to
  `state.Game` outside `events`. The ledger and the divergence store are
  host-side metadata, like `m.snaps`. `TestHeads` and golden replays must be
  untouched.
- **Hot path.** One byte appended per intent in an already-exclusive section.
  The scan is post-match, background, one match at a time.
- **Shadow parity risk.** If `shadow.Answer`'s dispatch drifts from
  `projectNextData`, B stops being "what the bot would do". A parity test
  pins it: a host match whose bot seat is a per-decision `ShadowSeed`-seeded
  instance must yield **zero** divergences when that seat is shadowed.
- **Sampler starvation.** A hindsight smoke measured 94.8% `no_world` before the redeal
  fallback. `-redeal` defaults on, `untriaged` is a first-class bucket, and its
  rate is a reported metric. If it stays high on human games, the work
  stalls on the sampler, not on this design.
- **Teacher bias** (§6.3) is the largest *semantic* risk. Prefer
  `human_better` pairs, and flag `bot_better` pairs that the player explained
  with a plan.
- **Open:** whether the shadow should be the table's policy (v1) or a fixed
  reference policy, so that divergences are comparable across tables. v1 records
  `policy` per record, so both views can be computed later.
- **Open:** CR 723 control effects. When the human controls another
  player's turn, `d.Player` is the controller (`server/host/match.go:519-526`), and
  those decisions are attributed to the human seat. v1 marks them
  `material: "controlled"` and does not triage them.

## 12. Order of the work

The capture core (`server/divergence`) comes first, then the host's provenance
ledger and the match-end snapshot and shadow scan, the capture worker and opt-in
field, the seat-fenced API, and the post-game review. Triage needs the core and
the `shadow` package, the exports need triage, the opt-in form needs the worker's
request field, and the symbol registry needs the exports.

The first volume number for §7.3 comes from a simulated human: a different hosted
policy in the human slot through the `Seats` option, over 20 games against `bot`,
reporting divergences per game by `material` class.

The live shadow and in-game chip (§8.4) wait for §7.3's first gate. They need the
asynchronous shadow on the parked View or Board copy after a human Submit, a
human-seat feed when the policy is an EnvSeat (today `newMatchFeeds` skips humans,
`server/host/botenv.go:54`), a push frame for the chip, and `post_reveal` tagging.
The symbol registry (§9) refuses omniscient symbols.

## 13. Pre-registered first measurement

After capture, triage and the exports exist and the first 20 opted-in human games are captured:

1. The divergence rate per human decision, by material class.
2. The bucket shares, with the `untriaged` share broken down by reason. **Kill
   criterion:** `untriaged > 70%` means the sampler is the bottleneck, and the
   work pauses until the sampler is fixed.
3. The `human_better` count. **Kill criterion:** zero `human_better` in 20 games
   with fewer than 30% untriaged means the human-as-teacher hypothesis, for this
   bot and these players, is not worth the UI cost. In that case keep only the
   capture and triage half, as a bot-defect miner.
4. The checklist tick distribution for `human_better` against `bot_better`, and
   the share of ticks on non-playable families (§8.5). This is the first input
   to §9.
