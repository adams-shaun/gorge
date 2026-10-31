# Draft events: a 4-player draft pod around the engine

Status: accepted 2026-10-13, base `main@bf71ba0c5`.

Comments under `draft/` cite this document as `spec §N`, and name its phases (E0 to
E5, in sections 5 and 7). It records the design as accepted, so where it and the code
differ the code is right. For how things work today, read
[the architecture page](../site/architecture/draft.md) and
[the user guide](../site/guide/draft.md). The differences that matter:

- **Set data.** Set files are now generated from trimmed MTGJSON booster data with
  two collations (section 7, E5-a), not from the XMage-derived manifests of
  section 4.2.
- **Phases.** E0 to E3 and E5-a to E5-d shipped. E4 shipped as drafter tuning; its
  learned ratings and pick assistant are not in this repository. E5-e, hosted 8-seat
  and sealed events for humans, was not built: a hosted event is 4 seats, and 8-seat,
  sealed and Swiss run in `cmd/draftsim` and `draft/draftlab`.
- **Ratings fallback.** A set with no rating file uses the heuristic of section 4.6
  (BLB today); there is no self-play fallback.
- **Wire types** live in `server/protocol/draftevent.go`, not `event.go`. Section 7's
  external-agent wire is documented in
  [`docs/site/api/gorge-agent.md`](../site/api/gorge-agent.md).

## 1. Scope

A 4-player draft pod: four seats, each a human or a bot persona, open three
packs, pick one card and pass, build a 40-card deck from the pool plus free
basic lands, then play a round robin of 1v1 matches. Bots take part in all
three phases.

Out of scope until later: 8-seat pods, Pick-Two, ghost drafters, sealed,
cube, constructed Bo3, an Arena-style gauntlet, Swiss, pick timers for
multi-human pods, leak-free search bots.

### Decisions

| # | Question | Decision |
|---|---|---|
| Q1 | First set | **FDN** for E0–E2, **BLB** right after (both Level B). |
| Q2 | Pod shape | 4 seats, pick one, 3 packs, pass left/right/left. |
| Q3 | Collation | Simplified Play Booster (§5). No foils, The List, Special Guests. The spec format must be able to express full sheets later. |
| Q4 | Set data source | Hand-authored gorge spec + the XMage-derived set manifests (MIT), now in gorge-compliance. Replaced by MTGJSON in E5-a. **No Forge `res/editions` fetch, no Forge data committed.** |
| Q5 | Pick ratings | 17Lands public dumps where available (CC BY 4.0, attributed), gorge self-play GIH as the fallback for other Level B sets, an IR heuristic as the floor. |
| Q6 | Match format | Bo1 in E0–E2; Bo3 with sideboarding in E3. No Bo1 hand smoother. |
| Q7 | Bots in events | Only policies with `bots.Info.Env == false`. Env policies read every seat's real decklist (`server/host/botenv.go`) and are refused at any human table. |
| Q8 | Human identity | Invite link carrying a seat token + a typed display name. Timeout during the draft auto-picks; a game never auto-replaces a human (caretaker only if `ThinkTimeout` is set). |
| Q9 | Restart | The event resumes from its log. A game in progress at restart is aborted and replayed from scratch with a new seed; both facts are logged. |
| Q10 | Reveal | Other seats' picks and decks are hidden until COMPLETE, then revealed. Bot-vs-bot games are watchable live. |
| Q11 | Layout | New pure `draft/limited/` and `draft/event/` packages, plus an `eventhost` runtime package, the one package of the stack on archtest's `time` allow-list. |
| Q12 | Refactor | After E2, as its own ticket: move play-vs-bot onto events and `createGame` out of `cmd/gorged`. |
| Q14 | Training data | Draft logs export in 17Lands `draft_data` column shape from E0. |

## 2. Vocabulary

**Event** › **Round** › **Series** (best-of-N between two seats) › **Game**. A
host "match" k on a table is one Game. A **pod** is the event's four seats. A
**persona** is a bot entrant: a Drafter, a Builder and a Player (a `bots`
policy name). A **CardRef** is one card instance in an event (§4.1).

## 3. Invariants for the new packages

Every package under `draft/limited/` and `draft/event/`, and `cmd/draftsim`:

- **Pure.** No `time`, `os` file access (except `cmd/draftsim`), network,
  `rules`, `state`, `events`, `effects`, `host`, `view`, `seat` or `bots`
  imports in `draft/limited/*` or `event`. They may import `cards` and `deck`.
  `cmd/draftsim` is the only E0 package that touches the engine. Each package
  carries a test that fails on a forbidden import, so these packages need no
  `internal/archtest` allow-list entry.
- **Deterministic.** All randomness is `math/rand/v2` seeded from a `uint64`
  (`rand.New(rand.NewPCG(seed, salt))`). No ranging over a map where order can
  reach output, a log entry or a pick. Same seed and same inputs give a
  byte-identical log.
- **Event state changes only through `event.Apply(entry)`.** The log is
  append-only. Replaying the log reproduces the state exactly.
- **Packs are logged when opened** (the full CardRef list), so a replay
  survives a later change to set data.

## 4. Packages and contracts (E0)

### 4.1 `limited` (package root): shared types

```go
type Rarity byte // 'C','U','R','M','L' (basic/land-slot), 'S' special
type CardRef struct {
    ID     string // "c117": unique within an event, assigned in pack-open order
    Name   string
    Set    string // printing set code, for art lookup later
    Number string // collector number of the printing
    Rarity Rarity
}
```

Colour and mana value come from the `cards.Registry` by `Name` when needed;
`CardRef` itself stays small and serialisable.

### 4.2 `draft/limited/setdata`: per-set booster spec

One embedded JSON file per set, `draft/limited/setdata/sets/<SET>.json`, generated
by a `go generate` program from the set's XMage-derived manifest plus a
small hand-authored header, then committed. Shape:

```json
{
  "code": "FDN", "name": "Foundations", "pack_size": 14,
  "main_max": 271,
  "sheets": {
    "common":   [{"name": "...", "number": "...", "rarity": "C"}, ...],
    "uncommon": [...], "rare": [...], "mythic": [...],
    "land":     [...]
  },
  "slots": [
    {"sheet": "common", "count": 7, "balance_colours": true},
    {"sheet": "uncommon", "count": 3},
    {"rare_mythic": {"rare_weight": 2, "mythic_weight": 1}, "count": 1},
    {"sheet": "land", "count": 1},
    {"wildcard": {"common": 0.571, "uncommon": 0.417, "rare_mythic": 0.012}, "count": 2}
  ],
  "source": "FDN manifest (XMage-derived, MIT) + gorge header"
}
```

A C/U/R/M sheet card is a manifest card with at least one collector number
`<= main_max`, bucketed by manifest rarity. The `land` sheet is named
explicitly in the hand-authored header and its cards are removed from the
other sheets. Measured on main@bf71ba0c5:

- **FDN**, `main_max` 271: 90 C, 101 U, 60 R, 20 M. The land sheet is the 10
  common gain lands (Bloodfell Caves, Blossoming Sands, Dismal Backwater,
  Jungle Hollow, Rugged Highlands, Scoured Barrens, Swiftwater Cliffs,
  Thornwood Falls, Tranquil Cove, Wind-Scarred Crag; numbers 259–271), leaving
  80 commons. Evolving Wilds (262) stays a common.
- **BLB**, `main_max` 261: 81 C, 99 U, 60 R, 20 M. The land sheet is the five
  basics (manifest rarity LAND, numbers 262–281).

`rare_weight 2 : mythic_weight 1` over 60 R / 20 M gives a mythic in 1 of 7
rare slots, the published Play Booster rate. The header (code, name,
`main_max`, land-sheet rule, slot list) is hand-authored from public product
facts; no Forge file is read or copied.

**Support test:** every card on every sheet resolves in the corpus registry
and `reg.Unsupported(c, effects.Supported())` is empty for it (the same check
`cmd/cardfuzz` uses). That test lives
in the package's external test package, so the `effects` import does not
reach the pure package.

### 4.3 `draft/limited/booster`: pack generation

`booster.Open(spec, rng) []Card` (or equivalent) fills the slots in order:

- **No duplicate names within a pack**: resample the slot.
- **Colour balance** on a `balance_colours` slot: after drawing the commons,
  if some colour of W/U/B/R/G has no mono-coloured common, replace a common
  of the most-represented colour with one of the missing colour. Repeat until
  balanced or 5 tries. Colour is read from the card IR. The colour lookup is
  passed in as a function so the package stays pure.
- **Wildcard** draws its rarity by the given weights, then a card of that
  rarity (rare/mythic by the rare_mythic weights).

The stats test opens 10,000 packs from a fixed seed and asserts:
14 cards each, no duplicate names, mythic share of the rare slot in
0.13–0.16, every pack's commons cover all five colours, and identical output
for the same seed.

### 4.4 `draft/limited/draft`: the pod

- Seats 0..3. Packs 1 and 3 pass **left**, meaning seat `i` passes to
  `(i+1)%4`. Pack 2 passes right, to `(i+3)%4`.
- Each seat holds a **FIFO queue** of packs. A seat may pick whenever its
  queue is non-empty. After a pick the remainder goes to the next seat's
  queue. Pack round r+1 opens only when every seat has emptied round r.
- **Deterministic regardless of interleaving:** a pack visits seats in a
  fixed order, and a seat's pick depends only on its earlier picks, so any
  order of `Pick` calls that respects the queues gives the same pools. A test
  must prove this by driving the same seeded draft in two different
  interleavings and comparing pools and logs.
- API sketch: `New(seed, packs [4][3][]CardRef)`, `Next(seat) (PickView, bool)`,
  `Pick(seat, pickNo, id) error` (stale pickNo → `ErrStale`, a card not in
  the pack → `ErrNotInPack`), `Done() bool`, `Pool(seat) []CardRef`.
- Packs come from the caller (generated by booster with the event seed), so
  `draft` does not depend on `setdata`.

### 4.5 `draft/limited/build`: decks

- A deck is `{Main []string (CardRef IDs from the seat's pool), Basics [5]int (W,U,B,R,G)}`.
- Valid when every ID is in the pool, there are no duplicate IDs, and
  `len(Main) + sum(Basics) >= 40`. There is no maximum and no copy limit
  (MTR 7.1). Basics are free and unlimited.
- `AutoLands(main, colourOf) [5]int`: fill up to 40 with basics split by the
  coloured-pip share of the non-land main cards, using largest remainder.
  Colourless decks split evenly over nothing, so they return zero.
- `ToDeckFile(pool, deck, name) deck.File` produces the engine-loadable deck,
  with basics as ordinary entries.

### 4.6 `draft/limited/pickbot`: Drafter and Builder

```go
type PickView struct {
    Set           string
    Round, PickNo int
    Pack          []limited.CardRef
    Picks         []limited.CardRef   // own pool so far
    Seen          [][]limited.CardRef // packs seen earlier
}
type Drafter interface{ Pick(v PickView, r *rand.Rand) string } // a card ID in v.Pack
type Builder interface{ Build(set string, pool []limited.CardRef) build.Deck }
```

A small registry maps names to Drafters and Builders (`rating-colour`,
`best23`), mirroring `bots.Register`.

**Ratings.** `draft/limited/pickbot/ratings/FDN.json` (committed with this spec) is
17Lands GIH WR for all 286 FDN cards, covering all 271 main-set cards. It is
CC BY 4.0 and carries its attribution in the file. The UI shows that
attribution from E2. For a set with no rating file, use the IR heuristic:
rarity base (C 0, U 2, R 5, M 6), plus a creature term from P/T against mana
value, plus small bonuses for evasion keywords and for removal-like spell
types. A test pins its ordering on a handful of named FDN cards.

**Drafter `rating-colour` v1.** The score for card c at pick p is
`rating(c) + k(p) * colourFit(c, picks)`.
- `rating` is centred: `(gih_wr - 0.55) * 100` for rated sets, and the
  heuristic on the same rough scale otherwise.
- `colourFit` is the mean, over c's colours, of that colour's share of the
  seat's rating-weighted picks so far. Colourless cards get 0.5 of the
  maximum.
- `k` ramps linearly from 0 at pack 1 pick 1 to its full value by pack 1
  pick 10, then holds. The full value is `ColourFitMax`; the default is 8
  (E4-2: any k from 6 to 12 measured the same, and 8 commits the drafter to
  its colours — at the old 4 the pools spread across all five).
- Ties are broken with the seeded rng.
- The constants live in one struct, tuned with draftsim (§4.8).

**Builder `best23` v1.**
1. Choose the colour pair that maximises the summed rating of its best 23
   playable cards (cards whose colours are a subset of the pair, plus
   colourless).
2. Take those 23, preferring a creature count of at least 17 and capping
   mana value 6+ at 3.
3. Add 17 basics via `AutoLands`.
4. If fewer than 23 playables exist, fill with the best off-colour cards and
   say so in the deck name.

The human "Auto" lands button and "Reset to bot suggestion" use these same
functions.

### 4.7 `event`: lifecycle, pairings, standings

- Phases: `LOBBY → DRAFT → BUILD → ROUND(1..3) → COMPLETE`.
- Log entries:
  - `EventCreated{Seed, Set, Seats[4]{Kind, Name, Persona}}`
  - `PackOpened{Round, Seat, Cards}`
  - `Picked{Round, PickNo, Seat, ID, Auto}`
  - `DeckSaved{Seat, Rev, Deck}`
  - `Ready{Seat, Rev}`
  - `SeriesStarted{Round, A, B, BestOf}`
  - `GameEnded{Round, A, B, Game, Winner *int, Head}`
  - `SeriesEnded{Round, A, B, Score [2]int}`
  - `EventEnded{Standings}`
- `Apply` rejects an entry illegal in the current phase.
- **Round robin** by the circle method with seat 0 fixed: R1 0–3, 1–2; R2
  0–2, 3–1; R3 0–1, 2–3.
- **Standings** (MTR Appendix C):
  - Match points: win 3, draw 1, loss 0.
  - MW% = points / (3 × rounds), floored at 0.33. GW% is the same over games.
  - OMW% and OGW% are the means of the opponents' floored values.
  - Order: points, then OMW%, then GW%, then OGW%, then seat index.
- Games are played by the caller. `event` exposes
  `PendingSeries() []SeriesSpec` and takes results as `GameEnded` entries,
  so `event` never imports the engine. `cmd/draftsim` plays them with
  `rules` in E0; `eventhost` will play them through `host.Registry` in E1.
- **Export:** `DraftDataCSV(log, w)` writes one row per pick in 17Lands
  `draft_data` shape: `expansion, event_type ("gorge_pod"), draft_id,
  pack_number, pick_number, pick`, then `pack_card_<name>` and `pool_<name>`
  count columns over the set's sheet names, in sorted order.

### 4.8 `cmd/draftsim`: the headless pod

`draftsim -set FDN -seed N -events K -players lethal-pressure,bot,forge-heuristic,lethal-pressure [-verify] [-draftlog out.csv]`

1. Four bot personas (Drafter `rating-colour`, Builder `best23`, the named
   Player policies) run an event end to end.
2. Games are played with `rules` the way `cmd/mtgsim` does, using
   `bots.New(policy, ...)` per seat. Use `botpolicy.MulliganLands` for 40-card
   decks where the policy exposes it. If the chosen policy cannot take it, say
   so rather than hack it in.
3. **The tool refuses any policy whose `bots.Info.Env` is true.**
4. `-verify` re-runs the event from its own log and compares event logs and
   every game's chain head.
5. Output is one line per game plus final standings, as with `mtgsim`.

**E0 exit:**
- `draftsim -verify` passes for 5 seeds.
- The booster stats test passes.
- One deck-quality measurement is run by hand: bot-built decks against 17Lands
  wr60 FDN decks at a fixed Player, using a 31.5k-deck corpus on local storage.

## 5. Later phases

- **E1 · hosted events.**
  - `eventhost` runs events inside gorged: seat tokens, JSONL persistence
    under `<dir>/events/<id>/`, resume per Q9, and SSE frames
    (`event_state`, `draft_pack`, `pod_progress`, `series_update`).
  - `/api/events/...` routes in `server/host/httpapi`; wire types in
    `server/protocol/event.go` followed by `make gentypes`.
  - Event decks are served through a deck loader chained in front of the
    catalogue, with ids `ev/<event>/<seat>/<rev>`.
  - A per-seat bot policy field on `TableConfig`.
- **E2 · UI.** `web/src/routes/Event.svelte` with Create, Draft, Build and Hub
  views; tap-to-move on phones; 17Lands attribution.
- **E3 · Bo3.**
  - A host series hook: the next game's decks and the starting-player
    chooser, posed as the existing `starting_player` decision to the loser
    (MTR 2.2).
  - Sideboarding between games as event actions.
  - Multi-human invites and pick timers.
- **E4 · better bots.** Learned per-set ratings (17Lands or self-play), the
  pick assistant, and drafters trained on exported draft logs.

## 6. Licensing

- Forge files (`res/editions`, `res/draft/*.rnk`) are GPL-3.0. We read them
  only for facts and never commit them.
- Draftmancer and XMage are MIT.
- 17Lands public datasets are CC BY 4.0 and need attribution. The 17Lands
  website's card-data pages fall under 17Lands' separate usage guidelines and
  are not used here.

## 7. E5: training-facing extensions (feedback 2026-10-13)

External feedback asked for genuine pack distributions, 8-agent drafts,
1 human vs 7 agents, sealed (bot- and human-built), and Drafter / Builder /
Player components that a separate training system can compose into
non-linear workflows (e.g. random-deck play, freeze the player, build, then
draft). Decisions:

| # | Question | Decision |
|---|---|---|
| E5-Q1 | Who runs the game loop for external players | gorge drives games; an external model sits behind a seat adapter |
| E5-Q2 | Process boundary | compiled Go for now (no RPC service); external agents later via a seat/bot (and Drafter/Builder) adapter to an external interface |
| E5-Q3 | Collation | selectable: MTGJSON `play` (paper) or `play-arena`; default `play-arena` (ratings are 17Lands Arena data) |
| E5-Q4 | Cards on bonus sheets | FDN and BLB start; their 20 SPG special guests (1.5% of packs) are all engine-supported; 18 lack XMage verdicts and get hand oracle scenarios (E5-a3) |
| E5-Q5 | Post-draft structure for 1 human vs 7 agents | deferred (Swiss vs an Arena-style 7-wins/3-losses gauntlet) |
| E5-Q6 | Priority | E3 first; E5 behind it at normal priority |

Phases: **E5-a** MTGJSON-backed packs (inputs committed under
`draft/limited/setdata/mtgjson/`, MIT; `scripts/mtgjson_trim.py`), **E5-b** pod size
as a parameter (4 | 8), sealed supply (6 packs → pool), Swiss pairing,
8-pod and sealed in draftsim, **E5-c** a Go composition API for training
workflows (open packs / draft / build / play with named built-in agents and
seeds, artifacts + 17Lands-shaped CSVs), **E5-d** external-agent adapters
(Drafter, Builder; batched Player), **E5-e** hosted 1 human + 7 bots and
sealed for humans.
