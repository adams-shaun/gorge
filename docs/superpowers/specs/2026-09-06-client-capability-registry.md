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

## Open questions

- **#6 and #2**: client-local or engine-modelled? Affects whether they survive
  a reconnect and whether prod round-trips every priority window.
- **Reconnect/resync**: `interface-comparison.md` row 10 records this as *not
  implemented* on the gorge side. Unverified since; re-check before designing
  anything that depends on it.
