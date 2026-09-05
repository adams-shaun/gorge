# Client↔engine interface: mtgplay (A, production) vs gorge (B, in development)

Read-only survey, 2026-09-04. A = `/home/sadams/projects/mtgbld`, B = `/home/sadams/projects/gorge`.

## Executive summary

- A is a **12-frame, 11-prompt-kind ad-hoc JSON protocol** over WebSocket, spoken directly between the browser and the JVM (`mtgserve` is not in the decision path except a test-mode reverse proxy: `mtgserve/cmd/server/main.go:438-447`). B is **two Go types** — `view.View` down, `decision.Intent` up — with **8 declared decision kinds (6 reachable)** and no transport at all yet.
- A identifies options by **heterogeneous domain ids** (card UUID, ability `originalId`, target UUID, mode string, `blocker_id`/`attacker_id` pairs) across **17 distinct reply shapes** landing in **one unkeyed `BlockingQueue`** drained at **14 blocking `take()` sites**. B identifies every option by **one integer index** validated by one 20-line function (`decision/decision.go:90-111`).
- **No sequence numbers exist anywhere in A.** Stale/duplicate-answer defence is a client-side boolean (`10-cards.js:791`) with a user-clickable "Re-enable input" escape. B stamps every `Decision` with `Seq = len(log.Events)` and rejects a mismatched seq, wrong player, wrong count, out-of-range index, or duplicate index.
- I counted **8 places** where A's browser JS computes rules state the engine never stated (blocker legality, tapped filtering, board reconstruction from a cached earlier frame, auto-pass triviality classification, priority exclusivity). B's client is asked to know nothing: costs, targets, phases and blocker→attacker binding are all server-side-only fields (`decision.Option.Attacker`, `.AltCostIndex`, both `json:"-"`).
- Hidden information: A redacts with **one boolean** (`GameSnapshot.playerSummary(..., revealHand)`), applied only to priority frames; `event` frames are broadcast byte-identical to both seats. B redacts in a dedicated package with a documented three-rule, state-aware event redactor plus a per-seat projection.
- **Two concrete A leaks found**: vendored patch `0015` fixes Chrome Mox offering *every* player's hand (the Swing client masked it, JSON serialization exposed it); and `GET /api/matches/{id}/snapshots` serves live god-view snapshots (every hand revealed) to **any seat in the match**, mid-game, with no completion gate (`mtgserve/internal/matches/handlers.go:696-742`).
- A silently decides two things the rules give to a player: **simultaneous-trigger order** (`chooseTriggeredAbility` returns `abilities.get(0)`) and **replacement-effect choice** (`return 0`). B makes both explicit decision kinds (`KTriggerOrder`, `KTriggerOptional`) and exposes the pending-trigger queue to every seat (requirement R3).
- Replay: A cannot reproduce a match from the wire (fresh shuffle, no seed, `MatchReplay` is explicitly "crash repro", not fidelity). B has seed + intents + events + a SHA-256 rolling chain with `HeadAt(n)` prefix verification.
- Seat count: A hard-codes 2 (`final Seat[] seats = new Seat[2]`, `1 - slot` arithmetic, `SpikeGame` is a `TwoPlayerDuel` port). B is N-seat by construction (`PlayerID uint8`, alive-ring `AliveFrom`/`NextAlive`, spectator projection), with one documented M1 shortcut in combat.
- A is far ahead on everything that is not the protocol: auth, reconnect, concede, undo, mulligan, modes, X-spells, mana payment, crash reporting, replays, AI seats, ~31k cards.

## Comparison table

| # | Dimension | A — mtgplay/browser | B — gorge | Consequence for a client |
|---|---|---|---|---|
| 1 | Message vocabulary | 12 server frame `type`s, 11 `prompt.kind`s, 5 priority option kinds, 17 reply shapes, no schema | 1 `View` (with optional embedded `Decision`) down, 1 `Intent` up; 8 `Kind` constants (6 issued), 11 option `Kind` strings | A: a client is 11 renderers + 17 hand-written reply builders. B: one renderer + one `{seq, choices:[i]}` |
| 2 | Option identity | card UUID / ability `originalId` / target UUID / mode string / blocker+attacker UUID pair, per kind | `Option.Index` int, always; `Obj`/`Player` echoed for highlighting only | A: the client must know which id field each kind wants. B: it sends integers |
| 3 | Answer validation | per-kind, ad hoc; re-resolved against `getPlayable(...)`; illegal blocks silently dropped by `declareBlocker` | one `Decision.Validate`: seq, player, count∈[Min,Max], in-range, distinct | A: an illegal answer can fail invisibly. B: it is refused with a reason |
| 4 | Rules knowledge in client | 8 inference sites found (see §2) | none; `Validate` is the whole contract | A: mechanics touch both sides. B: new mechanics are server-only |
| 5 | Hidden information | one `revealHand` boolean at prompt-build; `event` frames unredacted and broadcast to both seats | `view.Project` (hidden zone → count) + `view.RedactEvents` (3 state-aware rules) + leak property tests | A: redaction correctness rests on each call site. B: it is a package with tests |
| 6 | Known leaks | patch `0015` (Chrome Mox, fixed in-engine); live god-view snapshots readable by the opposing seat over HTTP | `Object.Remembered` deliberately never projected; spectator index gated | A: the leak surface extends past the WS protocol |
| 7 | State model | full snapshot on `priority` only; `me`-only on target/choose_card; nothing on combat prompts; plus a separate seq'd `event` stream | full `View` with every decision; non-nil-list discipline; redactable event log alongside | A: the client caches and merges frames. B: every frame is self-sufficient |
| 8 | Stack / pending triggers | stack list on priority frames; pending triggers **not observable** (auto-ordered) | `StackView` + `PendingView` public to every seat (R3), with targets and card text | A: a player cannot see what is about to trigger. B: they can |
| 9 | Sequencing / idempotency | none on the wire; one unkeyed queue, 14 `take()` sites; client-side lock only | `Decision.Seq`, `Intent.Seq`, hard reject on mismatch / no-pending | A: a duplicate answer resolves the *next* prompt. B: it errors |
| 10 | Reconnect / resync | server re-sends `lastPromptSent`; client backfills the log via `GET /events?since=N`; no auto-reconnect | not implemented (no transport) | A works today; B has nothing to compare |
| 11 | Triggers & choices | mulligan, targets, modes, "may" (via `yes_no`) explicit; **trigger order and replacement effects silently auto-picked** | trigger order and optional triggers explicit; **mulligan and modes declared but never asked** | Different gaps, opposite ends |
| 12 | Determinism / replay | no seed, fresh shuffle, best-effort action replay, god-view snapshot scrubber | seed + intents + events + SHA-256 chain, `HeadAt(n)`; genesis (decks/names) is *not* logged | A: replays approximate. B: they verify |
| 13 | Multiplayer | 2 hard-coded (`Seat[2]`, `1 - slot`, `SpikeGame`); `opponents` is already an array | N seats end to end; one M1 shortcut (single defending player in combat) | A: 4-player needs a protocol change. B: it needs a combat widening |
| 14 | Extensibility | new kind = new string + new JS branch + new reply shape; `hello.version` sent but never checked | new kind = a `Kind` const + ask/handle; envelope generated from Go types (D5); `events.Kind` ordinals append-only (chain) | A: silent drift is possible. B: drift is a compile error or a chain mismatch |

---

## 1. Message vocabulary

### A — server → client

Twelve frame types, enumerated from the client's dispatch (`mtgserve/internal/views/templates/play/scripts/90-bootstrap.js:261-405`) and cross-checked against the Java emitters:

| `type` | Emitter |
|---|---|
| `hello` (`{version:"0.3"}`) | `mtgplay/.../MtgPlayServer.java:142` |
| `matched` (`{slot, matchId}`) | `MtgPlayServer.java:231` |
| `waiting` (`{missing_slot}`) | `MtgPlayServer.java:243` |
| `partner_joined` (`{slot}`) | `MtgPlayServer.java:1018` |
| `prompt` | `WebSocketPlayer.java` ×11 kinds |
| `reveal` (`{title, cards[]}`) | `WebSocketPlayer.java:429` |
| `event` (`{seq, kind, payload, turn, phase, actor_slot, actor_name}`) | `MtgPlayServer.java:1095` |
| `toast` (`{level, text}`) | `WebSocketPlayer.java:1947-1953` |
| `error` (`{code, message}`) | `MtgPlayServer.java:1282` |
| `opponent_disconnected` | `MtgPlayServer.java:847` |
| `game_over` (`{winner, turns}`) | doc'd `MtgPlayServer.java:60` |
| `game_crashed` (`{error, turn}`) | `MtgPlayServer.java:983` |

Eleven `prompt.kind`s, all built in `WebSocketPlayer.java`:

`choose_card` (:519, :655), `target` (:761), `mulligan` (:883), `yes_no` (:964), `mode` (:1040, :1085, :1541), `pay_mana` (:1328), `announce_x` (:1453), `attackers` (:1601), `blockers` (:1699), `multi_amount` (:1783), `priority` (:1979). Plus an internal client-inbound sentinel `__refresh__` (:361, consumed at :1937) that never crosses the wire.

Inside a `priority` frame, five option kinds: `play_land` (:2034, :2051, :2143), `cast` (:2098), `activate` (:2156), `undo` (:2200), `pass` (:2208).

### A — client → server

Every inbound frame goes to `WebSocketPlayer.feedChoice` (:266) and is pushed onto a single unkeyed `LinkedBlockingQueue<JsonObject>` (:110). Seventeen shapes, distinguished only by which field happens to be present:

```
{kind:"pass"}                                   {kind:"play_land", card_id}
{kind:"cast", card_id}                          {kind:"activate", ability_id}
{kind:"undo"}                                   {target_id}          // "" = decline
{card_ids:[...]} | {card_id}                    {attacker_ids:[...], defenders:{atk:def}}
{blocks:[{blocker_id, attacker_id}]}            {action:"keep"|"mulligan"}
{answer:bool} | {yes:bool}                      {mode_id}
{ability_id}            // pay_mana, no `kind`  {cancel:true}        // pay_mana abort
{x:N}                                           {amounts:[N,...]}
{type:"prefs", manualMana:bool}                 // intercepted before the queue, :243
```

Field-name evidence: `WebSocketPlayer.java` lines 564, 574, 826, 935, 983-984, 1061, 1371-1375, 1476, 1566-1568, 1657, 1668, 1737, 1814, 2241-2244, 2362. Send sites: `10-cards.js:172,826,915`, `30-targeting.js:52,162,191`, `40-combat.js:106,451`, `50-prompts.js:30,47,96,104,257,266,323,367,436,515,636`, `60-render.js:118,602,626,643,685`, `90-bootstrap.js:147,714`.

**How answers are validated.** There is no common validator. Each kind re-resolves the client's id against the engine's *current* legal set:

- `cast`/`activate` walk `getPlayable(game, true, Zone.ALL, false)` looking for a matching id and return `PriorityResult.FAILED` on no match (`:2318-2356`, `:2377-2395`), which triggers a re-prompt plus a `toast`.
- `target` re-checks `target.canTarget(getId(), picked, source, game)` and loops on a mandatory prompt if the client answers empty (`:826-833`).
- `blockers` does **not** validate: every `{blocker_id, attacker_id}` pair is fed straight to `declareBlocker` (`:1745`), which drops an illegal block with no message back to the client.
- Malformed JSON is logged and ignored (`:288`).

### B — the whole vocabulary

`decision/decision.go` is 129 lines and is the entire client-facing contract. Package doc, lines 1-3: *"the engine asks a Decision listing every legal Option, and the client answers with an Intent naming option indices. No rules knowledge crosses the wire."*

Eight `Kind` constants (`:14-42`): `priority`, `target`, `attackers`, `blockers`, `mulligan`, `modes`, `trigger_order`, `trigger_optional`. **Six are actually issued** by the engine — `rules/turn.go:206`, `rules/stack.go:110`, `rules/combat.go:116`, `rules/combat.go:197`, `rules/trigger.go:562`, `rules/trigger.go:618`. `KMulligan` and `KModes` are declared and handled by the bot's fallback but nothing constructs them (grep across `rules/` finds no `KMulligan`/`KModes`; `effects/misc.go:143` notes modes as future work).

Eleven option `Kind` strings across those six: `play_land`, `cast`, `activate`, `pass` (`rules/legal.go:19-75`); `player`, `permanent` (`rules/stack.go:120-131`); `attacker` (`rules/combat.go:114`); `block` (`rules/combat.go:189`); `trigger` (`rules/trigger.go:568`); `yes`, `no` (`rules/trigger.go:621-623`).

Wire shapes:

```go
// decision/decision.go:46-86
type Option struct {
    Index int `json:"index"`; Kind, Label string
    Obj state.ObjID `json:"obj,omitempty"`; Player state.PlayerID `json:"player"`
    Attacker state.ObjID `json:"-"`     // server-side only
    AltCostIndex int     `json:"-"`     // server-side only
}
type Decision struct { Seq uint64; Player; Kind; Prompt string; Min, Max int; Options []Option
                       Source state.ObjID `json:"-"` }   // server-side only
type Intent struct { Seq uint64; Player state.PlayerID; Choices []int }
```

Validation is one function, `decision.go:90-111` — seq equality, player equality, `Min ≤ len(Choices) ≤ Max`, every index in `[0, len(Options))`, no duplicates. Its comment states the design intent: *"Everything a client can get wrong is caught here, which is what lets the client stay rules-ignorant."* `Engine.Submit` (`rules/engine.go:182-201`) additionally rejects when the game is over or no decision is pending.

**Consequence for a client:** A needs 11 renderers and 17 reply builders, each with its own id convention and its own failure mode. B needs one renderer over `View.Decision.Options` and one reply builder emitting `{seq, player, choices:[i]}`.

---

## 2. Rules knowledge required of the client

### A — eight places the browser or the frame builder infers what it was not told

1. **Blocker legality is never sent.** `selectBlockers` ships `available_blockers` = `getAvailableBlockers(game)` and `attackers` = every attacker (`WebSocketPlayer.java:1689-1723`) — a flat cross product with no statement of which blocker may legally block which attacker. The client renders every attacker in every blocker's dropdown (`40-combat.js:429-437`) and its own comment concedes the engine is silent: *"The engine doesn't tell us which attackers have vigilance and we don't need to know"* (`40-combat.js:398-402`). An illegal pairing is dropped inside `declareBlocker` (`:1745`) with no feedback.
2. **The client filters tapped blockers itself**: `if (b.tapped) continue;` (`40-combat.js:412`) — a rules fact (CR 509.1a) applied client-side even though the engine already knows it.
3. **Combat prompts carry no board.** `attackers`/`blockers` frames have no `me`/`opponents`, so the client reconstructs the battlefield from the *cached previous priority frame*: `const prior = LAST_PRIORITY_BY_SLOT[ACTIVE_SLOT]` (`40-combat.js:25-30`). Stale by construction if anything changed since.
4. **The client classifies which priority frames are "trivial" and auto-passes them.** `60-render.js:673-690` and `90-bootstrap.js:63-68` / `86-92` treat `pass`, `undo`, mana `activate`, and user-"ignored"-source `activate` as non-decisions and fire `{kind:"pass"}` on a timer. That is a rules judgement (that tapping mana with nothing to spend it on is never right) taken by the UI.
5. **The client infers that priority is exclusive** and discards the other seat's pending prompt as stale on that basis: `LAST_PROMPT_BY_SLOT[ACTIVE_SLOT] = null` with the comment *"Priority is exclusive — the active slot's prompt is now stale"* (`90-bootstrap.js:319-321`).
6. **The client builds its own playability index** from the option list and decorates board cells `.playable`: `PLAYABLE_BY_CARD_ID` / `ACTIVATABLE_BY_SOURCE_ID` (`60-render.js:342-356`).
7. **Duplicate-submission prevention is client-side only** — `AWAITING_ENGINE` (`10-cards.js:791-819`), with a "Re-enable input" button that lets the user clear it (`:812`).
8. **Card characteristics beyond name/P/T/tapped are not on the wire.** `GameSnapshot.playerSummary` (`GameSnapshot.java:149-310`) emits no keywords, no mana cost, no oracle text; the client re-resolves each card **by name** against mtgserve's Scryfall catalogue (`10-cards.js:149,395,995` → `/api/cards/resolve?name=`). Card identity therefore round-trips through a *name string*, not the engine's id.

A ninth, engine-side: `applyPriorityChoice`'s `cast` branch has to re-derive the DFC main-card id because the priority emitter and the `SpellAbility`'s own `sourceId` disagree (`WebSocketPlayer.java:2093-2103`, issue #105) — an id-identity problem the protocol pushes onto both ends.

### B — what `Validate` rejects, and what the client is never asked to know

`Decision.Validate` rejects exactly five things (`decision.go:91-109`): seq mismatch, wrong player, choice count outside `[Min, Max]`, index out of range, duplicate index. `Chosen` (`:117-128`) is all-or-nothing on range.

Never asked of the client:

- **Legality.** `rules/legal.go:16-77` enumerates it: sorcery-speed gate (`:10-12`), land drop count (`:30`), `castRestricted` (`:35`), instant-speed-or-Flash (`:38`), `adjustedCost(...).CanPay(pool)` (`:42`), each affordable alternative cost as its own option (`:45-55`), untapped + non-restricted mana abilities (`:58-71`), `pass` last *"so a client can safely default to the final option"* (`:74`).
- **Costs.** Which cost a `cast` option pays is `Option.AltCostIndex`, `json:"-"` (`decision.go:57-65`).
- **Targets.** `askTarget` (`rules/stack.go:108-142`) enumerates legal players and `effects.MatchesSpec`-passing permanents; zero legal targets is resolved server-side as CR 608.2b (`:132-140`), never surfaced.
- **Which attacker a blocker blocks.** `Option.Attacker`, `json:"-"` (`decision.go:54-56`); the option's `Label` reads "X blocks Y" and the index carries the binding (`rules/combat.go:186-192`).
- **Phase/step semantics.** `View.Step` and `View.Phase` are strings; `phaseOf` groups the twelve steps into five phases *"a client actually cares about"* (`view/view.go:242-258`).
- **Trigger ordering direction.** Spelled out in the prompt text itself: *"the one you choose first is put on the stack first, and so resolves last"* (`rules/trigger.go:568-569`), duplicated in the `KTriggerOrder` doc comment (`decision.go:28-34`).

The bot is the proof: `seat/bot.go:79-169` answers all eight kinds reading only `d.Options`, `d.Min`, `d.Max` and one field of the view (`v.Phase`, used solely to decide when tapping mana is worthwhile — `:45-48`).

---

## 3. Hidden information

### A

Redaction is one boolean parameter. `GameSnapshot.playerSummary(p, game, revealHand)` emits `hand[]` only when `revealHand` is true (`GameSnapshot.java:203-213`); the priority builder passes `true` for self (`WebSocketPlayer.java:1986`) and `false` for each opponent (`:1993`). Library contents are never emitted at all. Everything else in the summary — battlefield, graveyard, exile, command zone, mana pool, counters, commander damage — is sent to both seats.

Where redaction is **not** applied:

- `event` frames are built once and sent to slot 0 and slot 1 verbatim (`MtgPlayServer.java:1099-1102`). Payloads include `discard{card_name}` and `dies{card_name}` (`GameLogWatcher.java:204-212`, `:186-202`) — public facts in Magic, so no leak found there, but the channel has no redaction mechanism if a future emitter carries a private one.
- `reveal` frames are per-seat by construction (pushed from `lookAtCards` on the seat that looked — `WebSocketPlayer.java:403-441`).
- Non-priority prompts embed `me` with `revealHand=true` (`:817`, `:539`) — correct, since they only go to that seat.

Two concrete leaks:

1. **Fixed in-engine**: `mtgplay/patches/0015-issue-1-chrome-mox-imprint-own-hand.patch` — Chrome Mox's imprint used the no-collection `choose` overload, whose `TargetCard.possibleTargets(Zone.HAND)` scans `getPlayersInRange(...)`, so *"in mtgbld's web serialization the picker exposed opponents' hand cards. (XMage's Swing client happens to restrict the view to the controller, hiding the latent bug upstream.)"* The fix constrains the choice at the engine, not the wire — i.e. the protocol has no defence of its own against a card that offers a hidden zone.
2. **Open**: `GET /api/matches/{id}/snapshots` returns the god-view snapshots — `GameSnapshot.build(game)` calls `playerSummary(p2, game, true)` for **every** seat (`GameSnapshot.java:117-121`), emitted once per priority entry (`WebSocketPlayer.java:1902-1907`). The handler authorises **any seat in the match** and applies **no match-state gate** (`mtgserve/internal/matches/handlers.go:696-742`), and `Store.ListSnapshots` has no state filter (`internal/matches/snapshots.go:70-77`). The opposing seat can poll the endpoint during a live match and read the other player's hand. (The anonymous share path is correctly gated: `store.go:719` requires `state = 'completed'`.)

### B

Redaction is a package with a stated contract. `view/view.go:1-14`: *"a hidden zone contributes a count and nothing else unless the viewer owns it, another seat's decision is never attached."*

- Zones: `ZLibrary` and `ZHand` are `Hidden()` (`state/ids.go:32`); `Project` emits `LibrarySize`/`HandSize` for all, and populates `Hand`/`Pool` only when `p.ID == viewer` (`view.go:208-224`).
- `null` vs `[]` is load-bearing: a hidden hand marshals `null`, the viewer's own empty hand marshals `[]`; `omitempty` is deliberately absent so the two can never collapse (`view.go:92-106`).
- `Decision` is attached only when `d.Player == viewer`, and as a copy so an in-process seat cannot corrupt the engine's pending decision (`view.go:228-235`).
- A viewer index naming no seat is a spectator; the decision check is explicitly guarded rather than relying on the natural gate (`view.go:193-199`).
- `Object.Remembered` is never projected anywhere, because it can name a hidden-zone object (`view.go:395-399`).
- Event redaction, `view/redact.go:78-125`, applies three rules to a *copy*: (1) a `Secret` event is reduced to its shape for non-owners via an **allowlist of surviving fields**, so a future emitter that starts carrying `Pairs` is covered automatically (`:24-35`); (2) hidden→hidden zone moves lose their `Obj`; (3) every other kind has `Obj`/`IDs`/`Pairs` filtered by `visibleTo`, which resolves each id against actual game state rather than trusting the event's own `Player` field. The doc explains why state-awareness is required: a `TriggerPush`'s `Player` is the trigger's controller, which is not the remembered card's owner (`:52-63`). `Note` is exempt as the engine's explicit "tell everyone" channel; the one private Note is marked `Secret` at its emitter instead (`:41-50`).
- Tests: `view/view_test.go:73` (only the viewer sees their hand/pool), `:180` (library never appears in any projection), `:208` (decision attached only to owner), `:366`, `:419`, `:688` (measured `TriggerPush` leak closure).

**Consequence for a client:** in A, redaction correctness is a per-call-site property that a new card can silently violate. In B it is a chokepoint with property tests, and the projection is the only way a client-facing layer reads state at all.

---

## 4. State model delivered to the client

### A

- **Full snapshot, but only on `priority`.** `buildPriorityPrompt` (`WebSocketPlayer.java:1976-2216`) emits `turn`, `phase`, `step`, `my_turn`, `me`, `opponents[]`, `stack[]`, `options[]`.
- **Partial or absent elsewhere.** `target` and `choose_card` carry `me` only, added specifically so the hand does not go stale mid-modal (`:817`, `:539` — issues #125/#129). `attackers`/`blockers`/`mulligan`/`yes_no`/`mode`/`pay_mana`/`announce_x`/`multi_amount` carry no board at all; the client merges from cache (`60-render.js:317-318`, `40-combat.js:25-30`).
- **A separate delta channel.** `event` frames carry `seq`, `kind`, `payload`, `turn`, `phase`, `actor_slot`, `actor_name` (`MtgPlayServer.java:1085-1094`), persisted to `match_events` with `unique(match_id, seq)` so retries are idempotent (`internal/matches/events.go:44-60`). Nine event kinds from the watcher: `step_change`, `etb`, `attacks`, `blocks`, `dies`, `discard`, `life_change` (`GameLogWatcher.java:73-98`) plus `play_land`, `cast`, `tap_mana`, `activate`, `mulligan`, `kept_hand` from the player (`WebSocketPlayer.java:2280`, `:2337`, `:2426`, `:2447`, `:938-943`). This is a *display log*, not a state delta stream — the board is never rebuilt from it.
- **Stack**: `id`, `name`, `controller`, `source_id`, `card_name` (`:2003-2023`). Targets on stack objects are **not** sent.
- **Pending triggers: not observable.** There is no frame for them, and no decision — see §6.
- **Identity**: XMage UUIDs, with a documented remap needed for DFCs (`getMainCard()`, `:2093-2103`, `:2321-2332`). The client keys hand/battlefield by these ids and re-resolves display data by *name*.

### B

`view.View` (`view/view.go:46-73`) is complete for every decision: `viewer`, `turn`, `step`, `phase`, `active`, `priority`, `over`, `draw`, `winner *PlayerID` (nil-able so seat 0 winning is distinguishable from "still going" — `:54-59`), `players[]`, `stack[]`, `pending[]`, `decision`.

- `PlayerView` (`:84-106`): seat, name, life, lost, three zone sizes, battlefield/graveyard/exile as `CardView[]`, hand + pool for the viewer only.
- `CardView` (`:112-131`): id, name, types, tapped, **derived** power/toughness/keywords (read through `Chars`, never printed fields — `:277-300`), damage, attacking, counters, controller **and** owner, summon-sick.
- `StackView` (`:134-145`): id, kind (`spell`/`ability`), name, **`text`** (the card's own `TriggerDescription$`/`SpellDescription$`, falling back to Oracle — `:342-393`), controller, source, **`targets[]`**, and the card for a spell.
- `PendingView` (`:157-163`): source, controller, label, optional, decider — the trigger queue in the order it will hit the stack.
- **R3** is stated as a requirement and carried in the type docs: *"everything that WILL hit the stack, once its controller has ordered it or its decider has accepted it, must be observable too — not only what already has"* (`:37-40`); `Stack` and `Pending` are *"Public for every seat — R3"* (`:67`, `:70`). Integration tests drive it through a real engine (`view_test.go:1119`, `:1261`).
- Every public list is non-nil so a client never has to treat `null` and `[]` alike (`:61-64`, Ruling T23-u).
- Identity: `state.ObjID uint32` in a dense arena; `AddObject` assigns once at genesis (`rules/engine.go:95`) and zone moves are `MoveZone` events that do not re-mint (`events/apply.go`), so an object id is stable across zone changes by construction.

**Consequence for a client:** in A, rendering a non-priority prompt correctly requires having seen and cached an earlier priority frame. In B every frame stands alone.

---

## 5. Sequencing, idempotency, resync

### A

- **No sequence number on any prompt.** `grep -n 'prompt_id\|promptId\|"seq"' WebSocketPlayer.java` returns nothing. The `seq` that exists belongs to the `event` log only (`MtgPlayServer.java:1069-1075`).
- **One unkeyed queue.** `incomingChoices` (`WebSocketPlayer.java:110`) is drained at 14 blocking `take()` sites (lines 557, 686, 820, 929, 977, 1056, 1121, 1365, 1470, 1561, 1647, 1728, 1805, 1923). Nothing correlates a frame with the prompt it answers.
- **Duplicate/out-of-order answers**: an extra frame stays queued and satisfies the *next* prompt. The only defence is `AWAITING_ENGINE` in the browser (`10-cards.js:791-819`), added for issue #12 (*"the JS happily forwarded all of them and the engine got 5 duplicate cast frames"*), and the user can clear it with a button (`:812`).
- **Stale-answer rejection**: only implicit — `cast`/`activate` fail to re-resolve against the current `getPlayable` set, producing `FAILED` → toast → re-prompt (`:1893-1951`). `blockers` has no such re-resolution.
- **Timeouts**: removed. `take()` blocks indefinitely; abandonment is handled by the mtgserve idle sweeper (`cmd/server/main.go:466-470`) and mtgplay's own abandonment timer (`MtgPlayServer.java:660-664`). The comment at `WebSocketPlayer.java:114-119` records the change from the old 60 s auto-pass. (`docs/protocol.md:159` still documents the removed 60 s timeout.)
- **Reconnect**: `lastPromptSent` is cached on send (`:226-228`) and re-sent on re-attach (`MtgPlayServer.java:253` → `:1022`); it is cleared when a choice is fed (`:283-285`) so a reconnect does not resurface an answered prompt. The socket is hot-swapped while the engine thread keeps blocking on the same queue (`swapSocket`, `:159-163`). The client backfills the log with `GET /api/matches/{id}/events?since=N` on open (`90-bootstrap.js:236-245`). There is **no client auto-reconnect** — `sock.onclose` only sets a status string (`:247-250`).

### B

- `Engine.ask` stamps `d.Seq = uint64(len(e.L.Events))` and then emits a `DecisionAsk` event, so the seq is that event's own index — unique and monotonic (`rules/engine.go:167-171`).
- `Submit` (`:182-201`): rejects if the game is over, rejects if `e.pending == nil` (*"no decision pending"*), then `d.Validate(in)`. Only then does it append the intent to `L.Intents`, emit `DecisionMade`, clear `pending`, `handle`, run state-based actions and `Advance`.
- A **duplicate** answer therefore fails with "no decision pending"; a **stale** answer fails with `intent seq N, pending decision seq M`; a **wrong-seat** answer fails with `intent from player X, decision is for player Y`.
- Reconnect/replay: not implemented — there is no transport. The primitives exist (`Log.HeadAt(n)`, `Log.Intents`), and the spec's data flow step 4 lists *"out-of-range, duplicate, wrong-player and wrong-sequence are all rejected"* as the contract.
- One resilience detail worth noting: `releasePendingDecisionOfDepartedPlayer` (`rules/sba.go:474`) exists so a decision owed by an eliminated seat cannot wedge the match.

---

## 6. Triggers and choices

| Choice | A | B |
|---|---|---|
| Mulligan | **explicit** — `kind:"mulligan"` with hand, `mulligans_taken`, `free_mulligans`, `cards_to_bottom`; London bottoming reuses the target path (`WebSocketPlayer.java:860-946`) | **absent** — `KMulligan` declared (`decision.go:19`) but `rules.New` deals 7 and begins the turn (`engine.go:103`, `:141`) |
| Modes | **explicit** — `kind:"mode"`, single-mode auto-resolved (`:1524-1570`); also reused for `Choice` subclasses: colours, creature types, alternative costs (`:1036-1130`) | **absent** — `KModes` declared, never issued (`effects/misc.go:143` marks it future) |
| Targets | **explicit** — `kind:"target"` with `min_targets`/`max_targets`/`mandatory`, one `target_id` per reply, re-prompted on a mandatory empty answer (`:735-850`) | **explicit** — `KTarget`, `Min=Max=1`; multi-target is noted as pending `TargetMin`/`TargetMax` (`rules/stack.go:108-110`) |
| Optional ("may") triggers | **explicit but indirect** — surfaced as `chooseUse` → `kind:"yes_no"` (`:952-985`) | **explicit and first-class** — `KTriggerOptional`, two options, "yes" first, *"There is no default: an unanswered optional trigger never reaches the stack, and neither does a declined one"* (`decision.go:36-41`; `rules/trigger.go:616-641`) |
| Simultaneous trigger order (CR 603.3b) | **decided silently by the engine** — `chooseTriggeredAbility` returns `abilities.get(0)` with the comment *"Until we expose an interactive 'order your triggers' prompt … pick the first available"* (`:1500-1521`) | **explicit** — `KTriggerOrder`, `Min == Max == n` so `Validate`'s distinct-in-range rule already means "a permutation", no new wire format (`decision.go:21-34`; `rules/trigger.go:561-575`); re-checked before applying (`frontIsTheOfferedGroup`, `:600-614`); resumable mid-drain via `orderedTriggers` (`rules/engine.go:45-57`) |
| Replacement effect choice | **decided silently** — `chooseReplacementEffect` returns `0` (`:1494-1497`) | not reachable as a decision; replacements apply through `applyReplacements` (`rules/trigger.go:916`) with no player choice yet |
| Mana payment | **explicit fallback** — auto-tap first (`:1225-1268`), `kind:"pay_mana"` when auto-tap can't cover it, including delve special actions (`:1270-1394`); user-facing `manualMana` toggle | not surfaced — `payMana` is server-side (`rules/stack.go:92-104`); mana abilities are `activate` options |
| X / distributions | **explicit** — `announce_x` (`:1453`), `multi_amount` (`:1783`) with server-side bounds validation and auto-distribute fallback | absent |
| Attack target choice | one defender assumed; client may pin per-attacker via `defenders{}` (`:1664-1676`) | one defending player in M1, `NextAlive(active)`, with `Option.Player` already carrying it so widening is additive (`rules/combat.go:9-16`, `:103-116`) |

---

## 7. Determinism and replay

### A — a match cannot be reproduced from what crossed the wire

- No seed is sent, stored, or referenced anywhere in the protocol.
- `MatchReplay`'s own header: *"The current iteration is 'crash repro' — it does NOT replay the captured event stream verbatim"* (`MatchReplay.java:33-45`); it walks the same deck pair with headless auto-play, with a 300 s wall-clock cap (`:66`).
- `ScriptedReplayPlayer` is the closer attempt and lists its own limits (`ScriptedReplayPlayer.java:29-42`): *"The starting library is freshly shuffled per game, so the opening hand won't match the original match"*; *"Engine-internal prompts (target picks, attackers/blockers, modes, yes/no, choose_card) aren't captured in the event log"*; only `mulligan`, `kept_hand`, `play_land`, `cast` are drivable (`:49-52`).
- What *is* durable: the `match_events` log (idempotent on `(match_id, seq)`) and per-priority god-view `match_snapshots`, driving a scrubber UI (`replay_detail.html`) and a shareable, completion-gated public replay (`store.go:719`).
- No hash, no chain, no drift detection.

### B — replay is the design premise

- `Log{Seed, Events, Intents}` with a rolling SHA-256 chain seeded from the match seed (`events/log.go:50-75`); `Append` folds each event's compact binary encoding into the chain and defensively copies `IDs`/`Pairs` *"so a caller mutating its own slice afterwards cannot retroactively rewrite a logged event and desync Head from HeadAt"* (`:92-99`).
- `Head()` and `HeadAt(n)` (`:113-144`) make "playback to N" verifiable against a full log.
- `Event.Append` (`events/event.go:147-178`) is a total, length-prefixed encoding — *"two different events can never encode identically."*
- `events.Kind` ordinals are append-only, and every added kind says so in its own comment (`:39-49` TargetsChosen, `:50-59` FlipFace, `:60-73` ClockTick, `:74-91` TriggerPush, `:92-107` EndCombatReset) — because the chain and the golden replays depend on them.
- Determinism discipline: engine RNG from `Config.Seed` (`rules/engine.go:75`); the bot's RNG is separate and explicitly *"never math/rand's global functions and never the engine's rng: a match's outcome must be a pure function of (engine seed, bot seed)"* (`seat/bot.go:29-34`). The plan's global constraints forbid `time.Now`, global `math/rand`, and map-range in any order-affecting path.
- **The one honest caveat, stated at the top of the engine**: *"The event log is NOT a complete match description by itself. Genesis … runs before the log exists … it can never recover deck contents or player names. A faithful replay needs the original Config together with the log, not the log alone"* (`rules/engine.go:5-12`).
- Spec-level evidence (POC, not this code): identical chains across replay, playback-to-N and resume-from-N, and byte-identical chains between native and wasm effect implementations (spec §Determinism and replay, §D2).

---

## 8. Multiplayer

### A — two seats are baked into the interface

- `final Seat[] seats = new Seat[2]` (`MtgPlayServer.java:652`).
- Partner arithmetic `1 - slot` at `:244` (`missing_slot`), `:843` (`opponent_disconnected`), `:1015` (`partner_joined`).
- Game construction indexes `seats[0]`/`seats[1]` directly (`:864`, `:881-882`, `:906-907`, `:920-921`, `:938`).
- `emitEvent` broadcasts to slot 0 and slot 1 only (`:1101-1102`).
- `attachAiSeat` rejects `slot < 0 || slot > 1` (`:776`).
- `SpikeGame` is *"an in-tree port of XMage's `mage.game.TwoPlayerDuel`"*, kept in-tree because `Mage.Server.Plugins` is not compiled (`SpikeGame.java:13-18`).
- The one forward-looking piece: `opponents` is an array *"so multi-player formats can plug in later"* (`WebSocketPlayer.java:1988-1990`), and the client already renders opponent tabs and a multi-defender chooser (`60-render.js:235`, `40-combat.js:151-153, 360-390`).

### B — N seats throughout, one M1 shortcut

- `PlayerID uint8` (`state/ids.go:4`); `state.NewGame(cfg.Names)` sizes zones from the name list (`rules/engine.go:73`).
- Turn order and priority walk an alive ring: `AliveFrom`, `AliveCount`, `NextAlive` (`state/game.go:113-131`), used for pass counting (`rules/legal.go:84`) and priority handoff (`:98`).
- Genesis begins with the first seat still alive, not seat 0 (`rules/engine.go:135-141`), and degrades on a zero-seat or over-decked config rather than panicking (`:80-134`).
- `view.Project` iterates `g.Players` and supports an out-of-range viewer as a spectator (`view/view.go:193-226`).
- **Shortcut**: `askAttackers` sets one defending player for the whole combat — `defender := e.G.NextAlive(p)` (`rules/combat.go:103`). The file header names it explicitly as an M1 simplification and notes `decision.Option.Player` already carries the value, so *"widening this later is additive, not a rewrite"* (`combat.go:9-16`).
- Spec target: *"4+ players as a first-class concept, not a 2-player engine widened later"*; the POC ran 2/4/6/8 seats to completion with no seat-count-specific code.

---

## 9. Extensibility

**A — adding a decision kind** means: a new `prompt.kind` string and builder in `WebSocketPlayer.java`, a new `render*` branch in `60-render.js:325-336`, a new reply shape, and a new read in whichever `take()` site owns it. Nothing is generated; nothing is validated; the two sides are kept in step by hand. **Adding a visible field** is additive and safe (old clients ignore unknown JSON keys), and the codebase uses that freely — `is_token`, `token_color`, `attacking_defender_id`, `commander_damage_taken`, `is_mana` were all added this way (`GameSnapshot.java:223-259`, `:180-202`; `WebSocketPlayer.java:2172`).

What breaks quietly:
- The protocol carries a version (`hello.version = "0.3"`, `MtgPlayServer.java:142`) but **the client never checks it** — it only logs it (`90-bootstrap.js:263`). A breaking change is silent.
- `docs/protocol.md` is stale: it documents version `0.2`, a client `join` frame with an inline deck, and a 60 s choice timeout with auto-pass. The code is `0.3`, identity comes from the session cookie plus the URL (*"no more `join` frame"*, `MtgPlayServer.java:64-66`), and the timeout was removed (`WebSocketPlayer.java:114-119`). `docs/xmage-integration.md`'s "Game Flow" step 2 repeats the stale `join` step. The doc also omits five live prompt kinds (`choose_card`, `pay_mana`, `announce_x`, `multi_amount`, and the `mode` reuse for `Choice`) and five frame types (`partner_joined`, `reveal`, `event`, `toast`, `game_crashed`).
- `{type:"concede"}` is sent by the client after a bug report (`90-bootstrap.js:714`, commented *"same path as the Concede button"*) but **mtgplay has no handler for it**: `feedChoice` intercepts only `type:"prefs"` (`WebSocketPlayer.java:243-248`) and pushes everything else onto the choice queue, where `applyPriorityChoice` defaults a missing `kind` to `"pass"` (`:2241`). The Concede button itself uses the HTTP route (`70-state.js:495-498` → `mtgserve/internal/matches/handlers.go:480`). So the two "same path" concessions are not the same path.

**B — adding a decision kind** is a `Kind` constant plus an `ask`/`handle` pair in `rules` plus a `switch` arm in `Engine.handle` (`rules/turn.go:279-293`). `Validate` needs no change — `KTriggerOrder` is the worked example: *"Min == Max == len(Options) … so Validate's existing 'N distinct in-range indices' rule already means 'a permutation' and no new wire format is needed"* (`decision.go:21-26`). A client that renders `Prompt` + `Options[].Label` and posts indices handles a new kind with **no code change at all**; only kind-specific *affordances* need work. The bot's `clamp` (`seat/bot.go:178-206`) makes even an unknown kind answerable within `[Min, Max]`.

**Adding a visible field** in B means adding it to `View`/`CardView`/`StackView`; per D5 the envelope is *"generated from Go types so the client and server cannot drift."* What breaks: nothing on the view side, but `events.Kind` ordinals cannot be reordered or inserted (hash chain + golden replays), and the existing code enforces that by convention with a comment on every appended kind.

---

## 10. What A has that B lacks

Concrete, all verified in code. B has none of these.

**Transport and session**
1. A WebSocket transport at all. gorge has no `net/http`, no websocket, and imports `encoding/json` in exactly one file (`cards/fetch.go`); `cmd/` contains only `forgec`.
2. Authenticated seat resolution: cookie → `mtgserve:/internal/whoami` → `/internal/matches/{id}/seat`, with a shared-secret header gate (`MtgPlayServer.java:35-47`, `:73-81`, `:113-121`).
3. Reconnect with hot socket swap and last-prompt replay (`:159-163`, `:253`, `:1022`).
4. Solo mode — one browser tab holding both seats, with automatic focus switching (`90-bootstrap.js:310-330`).
5. Partner presence: `waiting`, `partner_joined`, `opponent_disconnected`.
6. Abandonment handling: per-match abandonment timestamp and sweeper (`MtgPlayServer.java:660-672`), plus mtgserve's idle-match sweeper (`cmd/server/main.go:466-470`).

**Decisions and play**
7. Mulligan (London, configurable free mulligans) — declared in B, never asked.
8. Modal spells and generic `Choice` picking (colours, creature types, alternative costs) — declared in B, never asked.
9. `announce_x`, `multi_amount`, `pay_mana`, `choose_card`, `reveal`/look-at-cards.
10. Undo, via engine bookmarks, on priority frames (`WebSocketPlayer.java:2196-2207`, `:2274-2280`) — with a documented gap (`#99`, no undo inside `pay_mana`/`choose_card`/`yes_no`).
11. Per-player runtime preferences over the wire (`{type:"prefs", manualMana}`) with a live prompt-refresh sentinel (`:243-248`, `:361`, `:1937`).
12. Concede — over HTTP (`mtgserve/internal/matches/handlers.go:480-535`).
13. AI seats attached over HTTP with difficulty and a trained policy, gated by deck vocabulary (`MtgPlayServer.java:769-818`). B has `seat.Bot`, in-process only.

**Operations**
14. Structured, durable game log with idempotent `(match_id, seq)` persistence and `?since=N` backfill.
15. God-view snapshot store, replay scrubber UI, and shareable public replay tokens (completion-gated).
16. Crash surfacing: `game_crashed` frames with a one-click "Report this crash" that pre-fills an issue (`90-bootstrap.js:361-397`); a 500-entry per-match frame log served over the internal HTTP port for triage (`MtgPlayServer.java:676-679`, `:334-340`); a vendored watchdog capping the SBA/trigger loop (`patches/0001`).
17. Soft failure: `toast` frames plus an idempotent re-prompt loop when the engine cannot honour a click (`:1907-1951`) — B's `Submit` returns an error with nowhere yet to send it.
18. Format support already on the wire: starting life per format, command zone, commander damage, tokens with colour, player counters (poison/energy/…), exile, attachments.
19. ~31,818 XMage card classes behind `CardRegistry`, versus gorge's curated-pool pipeline (M4 target 1,657 cards).

**Neither has**: chat, clocks/timers, spectators of a *live* match (A has anonymous shared replays of completed matches; B has a spectator *projection* — `view.go:193-199` — but no transport), sideboarding (`WebSocketPlayer.sideboard` is a stub, `:2521-2522`), or draft.

---

## Open questions I could not settle from the code

1. **B's transport is entirely unwritten.** D5 says "WebSocket + versioned JSON envelope … generated from Go types", but no envelope type, version field, framing, push-vs-poll model, or auth exists in the repo. In particular: `seat.Seat.Decide(ctx, View, Decision)` is a synchronous call taking a view *by value* — how a non-asked seat receives updates (view push? redacted event stream? both?) is not implemented. The spec's own open question 4 ("in-process or its own service") is explicitly deferred to M2.
2. **Whether B intends to ship the event stream to clients at all.** `view.RedactEvents` exists and is thoroughly tested, which implies yes, but nothing calls it outside tests, and `View` alone would suffice for rendering.
3. **Why `KMulligan` and `KModes` are declared but unreachable** — placeholders for M1 completion, or a leftover from an earlier scope? `seat/bot.go:68` treats them as "any kind added later", which reads like the former.
4. **A's object-identity guarantees across zone changes.** The client keys by UUID and the DFC main-card remap (`#105`) shows the ids are not uniformly stable across the engine's own views of a card, but I found no statement of the general rule (does a permanent keep its card's UUID when it enters the battlefield, and again when it dies?). Not determinable without reading vendored XMage, which is out of scope here.
5. **Whether the live-snapshot exposure (§3) is intentional.** The handler comment says "Auth-gated: only users who hold a seat in the match", which is satisfied — but the payload is god-view and the match need not be over. I found no issue, test or comment acknowledging the mid-match case.
6. **Whether `{type:"concede"}` over the WS was ever meant to be handled** in mtgplay, or whether `90-bootstrap.js:714` is simply dead code that silently passes priority instead of conceding.
7. **`hello.version` semantics.** Nothing reads it; there is no negotiation, no minimum-version check, and no changelog tying `0.2 → 0.3` to the frame-shape changes.
8. **Whether A's `blockers` frame ever intends to carry legality.** The client comment reasons its way *around* the absence rather than flagging it, so it is unclear whether the silent-drop behaviour is known and accepted or simply unexamined.
