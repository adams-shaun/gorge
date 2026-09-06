# Client capability registry — what the UI will need, and what supports it today

**Living document.** Add a row when a UI need is identified; move it when the
support lands. This is the list a design works against, so a feature is not
"planned" until it appears here with its support classified.

**Product shape (settled 2026-09-06):** a **player seat** is the primary
surface — one person acting on their own seat — but it requires **full-table
awareness**: every opponent's board and the whole event stream, not just the
player's own corner. In a 4-seat game that is the sizing constraint on every
layout decision.

Source material: `docs/superpowers/reports/2026-09-03-m0-m1/ui-inspiration.md`
(25 numbered, screenshot-cited recommendations) and `interface-comparison.md`
(14-row protocol comparison). Item numbers below are that survey's.

## How to read the support column

| | |
|---|---|
| **shipped** | built and on main |
| **client** | every fact needed is already on the wire; pure rendering work |
| **wire** | the engine knows it, the protocol does not carry it |
| **engine** | the engine does not model or ask it at all |

The distinction matters for sequencing: a **client** item can be designed and
built at any time; a **wire** or **engine** item blocks its UI until a
server-side task lands, and those tasks belong in the engine backlog, not the
design.

---

## Needs engine work

| Item | What the UI wants | Why it is blocked |
|---|---|---|
| **Combat damage distribution** (user request, 2026-09-06) | When one attacker is blocked by several creatures, the attacking player orders the blockers and assigns damage. | **There is no damage-assignment decision kind.** The kinds are priority, target, attackers, blockers, mulligan, modes, trigger_order, trigger_optional, choose. `dealCombatDamage` assigns automatically. Worse, the implicit order is taken from the order the blocker options were chosen in (`rules/combat.go:209`), and `KBlockers` is asked of the **defender** (`rules/combat.go:202`) — so the blocking player currently decides how the attacker's damage is distributed. CR 510.1c gives that ordering to the attacking player. Needs a new decision kind asked of the attacker, plus a rules fix. |
| **#19 Mana payment** | Show the *remaining* cost as mana symbols with the spell named, auto-tap by default, allow manual tapping, always offer Cancel. | `legalActions` offers one `"activate"` option per permanent — "Tap <name> for mana" — which taps **the permanent's whole mana ability set together** (`rules/legal.go:139`). There is no choice of which mana a dual produces, no remaining-cost figure on the wire, no auto-tap, and no cancel. `PlayerView.pool` exists, so what is *in* the pool is visible; what is still *owed* is not. |
| **#6 Auto-yield manager** | List every muted ability in full text, per-row *Remove Yield*, a global *Disable All*, set-a-yield from an object's context menu. | Yields are per-player persistent state the engine does not model. Doable client-side as a local filter, but then it is per-browser and invisible to a reconnect. Decide which before designing it. |
| **#2 Per-step stop toggles** | A 12-cell colour strip, one column per player, green = stop / red = skip, with MTGO's tuned defaults. | Same question as #6: client-side auto-pass round-trips every priority window to the server and back. Cheap on a LAN fixture, wasteful in prod. Server-side stops are an engine feature. |

## Needs wire work

| Item | What the UI wants | Gap |
|---|---|---|
| **#8 Interpolate bound parameters** | `[Gained Amount: 4]`, `[Zone Change … Traveler's Amulet (90)]`, `- targeting Cathodion [e74]` inside the stack entry's text. | `StackView` carries `text` (the ability text) and `targets[]`, but no resolved parameter bindings. The engine computes them at resolution; the wire never sees them. |

## Client-only — buildable today, nothing blocking

Every fact these need is already on the wire.

| Item | Note |
|---|---|
| **#1** Default to stopping; ship the cancel with the skip | pure UX |
| **#3** Held-modifier full control | pure UX |
| **#4** Four-line status block | `view.turn/step/phase/priority`, stack length — **shipped** |
| **#5** Primary button labelled by state | **shipped**; resolves by option `kind`, never position (FL-101) |
| **#7** Type-banded stack entries | **shipped** — `StackTile.svelte` renders `kind-{stack.kind}` |
| **#9** Short identity token per object (`[436]`) | `CardView.id` / `StackView.id` |
| **#10** Emphasise top of stack by contrast | **shipped** (M2e-4) |
| **#11** Stack depth always visible, panel open or closed | `view.stack.length` |
| **#12 / #16** Combat arrows — attacker→defender, targeting arcs | **unblocked 2026-09-06**: `Option.Attacker` is on the wire (U0); `StackView.targets[]` already carried the spell bindings |
| **#13** Pending-trigger tray | `PendingView{source, controller, label, optional, decider}` is fully present. **The survey calls this a genuine gap no surveyed platform fills — gorge already has the data and has not drawn it.** Highest differentiator-per-effort item on the list. |
| **#14** Trigger-order transfer widget | `KTriggerOrder` |
| **#15** Optional triggers: Decline / Take Action | `KTriggerOptional` |
| **#17** Combat sub-step named in the prompt | `view.step` |
| **#18** Prompts as text over the board, never a modal | **shipped** (M2e-4) |
| **#20** Persistent rules transcript in a right rail | **shipped** — `Transcript.svelte` |
| **#21** Ring freshly-changed permanents | event stream |
| **#22** Group permanents by type into sub-rows | `CardView.types` |
| **#23–25** Spectator layout, four hands as text, per-seat identity bars | `view.Project` with omniscient visibility |

## Cross-cutting, already settled

- **Base path** — the client mounts anywhere via `<meta name="gorge-base">`; no
  router rewrite needed (U0).
- **The client stays rules-ignorant.** `decision.Validate` is the whole
  contract; the client renders `view.decision.options` verbatim and posts
  indices. The legacy mtgplay client had 8 rules-inference sites. Never
  reintroduce one.
- **Never default to the last option.** The final option on every priority
  decision is `concede` (FL-101). Resolve by `kind`, never by position.

## The label/discriminator gap — one class, five fields

**This is the single biggest obstacle to the "client must not guess" rule**, and
it is one root cause, not five bugs.

`decision.Option` and `decision.Decision` carry a human-readable `Label` and
hide the machine-readable discriminator behind it:

| Field | `json` | What the client cannot do without it |
|---|---|---|
| `Option.Ability` | `-` | Anchor an ability popup to the right ability on the card. It gets `kind:"ability"` and a label, and nothing tying it to a specific ability. |
| `Option.Mode` | `-` | Tell a kicked / surged / flashback / miracle cast from an ordinary one. |
| `Option.AltCostIndex` | `-` | Tell which of several costs a `cast` option pays. |
| `Option.Amount` | `-` | Know what X value an `x` option represents; the index is its position, not its value. |
| `Decision.Source` | `-` | Name the source object in the prompt — survey #18, "always name the source". |

`Option.Attacker` was the sixth and was fixed in U0. **The remaining five should
be fixed the same way, as one task**, before UI is built on top of label
parsing. A client that reads `"Cast Bolt (kicked)"` to discover the mode is
doing rules inference by string-match, which is precisely the property
`interface-comparison.md` row 4 records gorge as having and the legacy client
as lacking (8 inference sites).

`ResumeKind` and `ResumeSA` are genuine engine internals and stay hidden.

## Needs wire work (added 2026-09-06)

| Item | What the UI wants | Gap |
|---|---|---|
| **Equipment/Auras ride under the attachee** | An attachment rendered beneath the permanent it modifies, clearly visible. | `state.Object.AttachedTo` exists (`state/object.go:84`) and is **not on `CardView`**. The client cannot tell what is attached to what. Same shape as `Option.Attacker`. |
| **Game-level counters (storm count, etc.)** | "Storm count: 3" and its siblings, rendered as board state. | Storm is computed inside `effects/count.go` from the log (`Count$ThisTurnCast`). No game-level count reaches the wire; a client would have to re-derive it by counting events — guessing. |
| **Card-anchored ability shortcuts** | Tap-for-effect shortcuts on the card, with a popup to pick which ("tap for red", "pay 2 life, sacrifice, draw"). | Needs `Option.Ability` above. Options exist and are legal; they simply cannot be attached to the card that owns them. |

## Needs engine work (added 2026-09-06)

| Item | Why it is blocked |
|---|---|
| **Scry X** | Not registered. `grep Register` in `effects/` has `Mill` and no `Scry`. |
| **"View Z from the top, put N back in any order"** | `effRearrangeTopOfLibrary` (`effects/cardflow.go:210`) looks at the top N and **emits a Note with the order unchanged** — the reorder is never asked. Already listed as a known approximation in `AGENTS.md`. |
| **Auto mode: autopay mana** | There is no autopay. `legalActions` offers one `activate` per permanent that taps its **whole mana ability set together** (`rules/legal.go:139`); there is no remaining-cost figure and no choice of which mana a dual produces. |
| **Auto mode: skip priority intelligently** | "Do not pause unless cards can be played and another player has something on the stack; on your own turn, pass each phase." Client-side this round-trips every priority window; engine-side it is real backend work. Same open question as #2/#6. |

## Client-only (added 2026-09-06)

| Item | Support |
|---|---|
| **Mulligan view** | `KMulligan`, with keep/mulligan and bottoming option kinds. Reachable in served games since M2e-5. |
| **Mana pool rendering** | `PlayerView.pool` — `Record<string, number>`. |
| **+X/+Y counters vs actual strength** | **Already solved and worth knowing.** `CardView.power`/`toughness` are **derived**, read through `ch.Power(id)`, not printed fields — counters and effects are already applied. `CardView.counters` ships alongside, so "3/3 (2/2 +1/+1)" needs no inference. |
| **Ability/keyword icons (deathtouch, double strike)** | `CardView.keywords` is **derived** too, so *granted* keywords appear, not just printed ones. |
| **Identical-card stacking** | Group by `printing` — no 100 separate zombie tokens. |
| **Graveyard / exile stack viewers** | `PlayerView.graveyard`, `.exile`. |
| **Targeting and attack effect overlays** | `StackView.targets[]` plus `Option.Attacker` (U0). |
| **Hover for art and oracle text** | `CardView.printing{name,set,number}` is explicitly "the identity a client resolves an image by". **Resolve art and oracle text from mtgbld's Scryfall catalog, not the gorge wire** — gorge compiles Forge scripts and deliberately ships no card text, so adding oracle text to this wire would raise a licensing question that the catalog already answers. |

## Open questions

- **#6 and #2**: client-local or engine-modelled? Affects whether they survive
  a reconnect and whether prod round-trips every priority window.
- **Reconnect/resync**: `interface-comparison.md` row 10 records this as *not
  implemented* on the gorge side. Unverified since; re-check before designing
  anything that depends on it.
- **Auto vs manual mode** is two features wearing one name: "autopay mana" is an
  engine feature, "skip priority intelligently" is a stops/yield policy. They
  can ship independently and should be scoped separately.
- **Where does auto-mode policy live?** Client-local is cheap and does not
  survive a reconnect; engine-side survives and avoids a round trip per
  priority window. Same decision as #2 and #6 — answer it once for all three.
