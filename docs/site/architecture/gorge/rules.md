# The rules core

Package `engine/rules` is where the Comprehensive Rules (CR) live: casting and
the stack, state-based actions, triggers, replacement effects, layers, combat
and zones. Each section names the functions that implement a mechanism and the
CR rules they cite. Read [Events and replay](events.md) and
[Game loop and decisions](game-loop.md) first.

## The shape of the package

Around `engine/rules` sit seven sub-packages that each compute one thing and
never hold an engine:

<figure markdown>
<svg viewBox="0 0 780 330" width="100%" role="img" aria-label="The engine/rules package and its sub-packages, layered: orchestration on top, the resolution kernel, the subsystem packages, characteristic computation, effects, the cost vocabulary, and the core types">
  <rect x="10" y="10" width="760" height="50" rx="6" class="dg-box-accent"/>
  <text x="24" y="32" class="dg-text-bold">rules</text>
  <text x="120" y="32" class="dg-text">turn, step, priority, casting, stack, SBAs, triggers, replacements, layers' lifecycle, Clone</text>
  <text x="120" y="50" class="dg-text-muted">implements effects.Host and every sub-package's Board</text>
  <rect x="10" y="68" width="760" height="40" rx="6" class="dg-box"/>
  <text x="24" y="93" class="dg-text-bold">rules/resolve</text>
  <text x="200" y="93" class="dg-text-muted">checkpoint, answer tape, re-execution of a resolution</text>
  <rect x="10" y="116" width="248" height="56" rx="6" class="dg-box"/>
  <text x="24" y="138" class="dg-text-bold">rules/trigmatch</text>
  <text x="24" y="158" class="dg-text-muted">one matcher per Mode$</text>
  <rect x="266" y="116" width="248" height="56" rx="6" class="dg-box"/>
  <text x="280" y="138" class="dg-text-bold">rules/combat</text>
  <text x="280" y="158" class="dg-text-muted">attack/block legality</text>
  <rect x="522" y="116" width="248" height="56" rx="6" class="dg-box"/>
  <text x="536" y="138" class="dg-text-bold">rules/pay</text>
  <text x="536" y="158" class="dg-text-muted">mana and payment plans</text>
  <rect x="10" y="180" width="504" height="40" rx="6" class="dg-box"/>
  <text x="24" y="205" class="dg-text-bold">rules/chars</text>
  <text x="140" y="205" class="dg-text-muted">the CR 613 layer walk (read-only)</text>
  <rect x="522" y="180" width="248" height="40" rx="6" class="dg-box"/>
  <text x="536" y="205" class="dg-text-bold">rules/scriptfacts</text>
  <rect x="10" y="228" width="504" height="40" rx="6" class="dg-box"/>
  <text x="24" y="253" class="dg-text-bold">effects (+ params)</text>
  <text x="180" y="253" class="dg-text-muted">api: primitives, filters, counts</text>
  <rect x="522" y="228" width="248" height="40" rx="6" class="dg-box"/>
  <text x="536" y="253" class="dg-text-bold">rules/cost</text>
  <text x="640" y="253" class="dg-text-muted">(leaf)</text>
  <rect x="10" y="276" width="760" height="44" rx="6" class="dg-box-muted"/>
  <text x="24" y="303" class="dg-text-bold">events, state, decision, cards</text>
  <text x="300" y="303" class="dg-text-muted">the vocabulary every layer above shares</text>
</svg>
<figcaption>Each band imports only bands below it. The sub-packages read the
game through a narrow <code>Board</code> interface that <code>rules</code>
implements; nothing below the top band emits an event. <code>internal/archtest</code>
pins the imports, and each <code>Board</code>'s method count may only shrink.</figcaption>
</figure>

Inside `rules`, a mechanism's files share a name prefix: `cast*`, `trigger*` and
`triggered_*`, `replace*`/`replacement*`, `layer*`, `mana*`, `combat*`, `sba*`,
`turn*`, and `stack.go`.

## Casting (CR 601)

A cast is a small state machine. `beginCast`
([`engine/rules/cast_begin.go`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/cast_begin.go))
creates a `pendingCast` on `e.cast`; `continueCast` runs the stages in order. A
stage that needs an answer poses a decision and returns; the answer re-enters
`continueCast`, which skips finished stages.

```mermaid
flowchart TD
  A["priority: cast option chosen"] --> B["beginCast<br/>pendingCast on e.cast"]
  B --> C["announcements (CR 601.2b):<br/>gift, kicker counts, replicate, squad,<br/>convoke, X, delve, sacrifice, discard ..."]
  C --> D["pushCast (CR 601.2a):<br/>PutOnStack, spell is on the stack"]
  D --> E["castModeAsk: modes (CR 601.2b)"]
  E --> F["targetAsk (CR 601.2c)<br/>+ sub-ability targets"]
  F --> FE{"a legal choice exists<br/>for every mandatory target?"}
  FE -- no --> AB["abortCast: CR 733.1 reversal"]
  FE -- yes --> G["total cost (CR 601.2f):<br/>commander tax, then increases and reductions,<br/>repriced for the chosen targets"]
  G --> H["payCast: mana-ability window (CR 601.2g),<br/>pay mana and non-mana costs (CR 601.2h)"]
  H -- "cost no longer payable" --> AB
  H --> I["CastInfo events, then<br/>'when you cast' triggers (CR 601.2i)"]
  I --> J["priority to the caster"]
```

- **Targets before payment.** Targets are chosen (`targetAsk`) before anything is
  paid (`payCast`), as CR 601.2c-h requires, and the total cost is recomputed for
  the chosen targets (`repriceForTargets`).
- **Mandatory-target feasibility.** If the mandatory targets cannot all be chosen,
  the cast aborts with `cast aborted: no legal target`. `targetChoiceFeasible` is
  the same predicate the priority offer used, so an offered cast can always
  announce a legal target.
- **Illegal casts are reversed forward, not erased** (CR 733.1). The log is
  append-only, so `abortCast` emits compensating events: the spell moves back
  (`MoveZone` with text `reversed`), `SpellCastReversed` decrements the "spells
  cast this turn" tally `PutOnStack` incremented, faces and choices are restored,
  and the held "when you cast" trigger is dropped. Announced-payment mana taps
  are undone with one `ManaUndo` each. Repeating the same aborted cast twice
  removes the option for that window (CR 733.2), so a bot cannot loop on it.

`CastInfo` records how the spell was cast (X, kicker and alternative-cost flags).
It is emitted only at payment, so an aborted cast leaves no cast-time trace.

## The stack and resolution

`Game.Stack` is a slice of object IDs; the top is last. Spells and abilities are
both arena objects; an ability object carries its `*cards.SA`, its source and
the source's incarnation.

`resolveTop` ([`engine/rules/stack.go`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/stack.go))
resolves the top object:

1. For an ability, check that the source is still the same object (CR 400.7)
   and re-check an intervening "if" condition (CR 603.4).
2. **Re-check targets** (`legalTargets`, CR 608.2b). If every target is gone or
   illegal, the object leaves the stack without resolving ("fizzled: no legal
   targets remain"). A target that left and came back is a new object, caught by
   comparing the incarnation stamped when it was chosen.
3. Emit `Resolve` (for a spell) and call `effects.Resolve`, which walks the
   ability's sub-ability chain (see [Cards and primitives](cards.md#resolution-walking-the-chain)).
4. If a decision is needed mid-way, the resolution kernel takes over
   ([Game loop](game-loop.md#questions-asked-in-the-middle-of-a-resolution)).

## State-based actions (CR 704)

`checkStateBased` ([`engine/rules/sba.go`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/sba.go))
applies state-based actions until none apply (CR 704.3): a fixed-point loop of at
most `maxSBAPasses` (32), after which it emits a `Note` instead of looping. The
main checks, in order (`sba.go` also handles, for example, Role exclusivity,
CR 704.5z, and Start your engines! speed, CR 704.5aa):

| CR | Check |
|---|---|
| 704.5a-c | a player at 0 or less life, who drew from an empty library, or with 10 poison counters loses |
| 704.5q | +1/+1 and -1/-1 counters annihilate |
| 704.5f-h | toughness 0 or less, lethal damage, deathtouch damage |
| 704.5j | the legend rule: the controller chooses which to keep (a `choose` decision with `keep` options) |
| 704.5k | the world rule |
| 704.5i | a planeswalker with 0 loyalty |
| 704.5v | a battle with 0 defense |
| 704.5d-e | tokens and copies in the wrong zone cease to exist |
| 704.5m-n | illegal Auras, Equipment and Fortifications |
| 704.5s | Sagas past their last chapter |
| 704.5t | completed dungeons |

The checks run after every submitted answer (never mid-cast), after draws, after
each combat damage pass and at step boundaries. A quiet-skip (nothing relevant
changed) and a per-permanent prefilter keep this cheap.

## Triggers

<figure markdown>
<svg viewBox="0 0 780 200" width="100%" role="img" aria-label="Trigger flow: an event is logged, checkTriggers matches it against every trigger, matches are queued, sorted APNAP, ordered by each controller, and put on the stack before the next priority">
  <defs>
    <marker id="tr-a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0,0 L10,5 L0,10 z" class="dg-arrow"/></marker>
  </defs>
  <rect x="10" y="40" width="120" height="70" rx="8" class="dg-box"/>
  <text x="70" y="68" text-anchor="middle" class="dg-text-bold">event</text>
  <text x="70" y="88" text-anchor="middle" class="dg-text-muted">logged + applied</text>
  <rect x="160" y="40" width="150" height="70" rx="8" class="dg-box-accent"/>
  <text x="235" y="64" text-anchor="middle" class="dg-text-bold">checkTriggers</text>
  <text x="235" y="84" text-anchor="middle" class="dg-text-muted">interest prefilter,</text>
  <text x="235" y="100" text-anchor="middle" class="dg-text-muted">trigmatch.Match(mode)</text>
  <rect x="340" y="40" width="130" height="70" rx="8" class="dg-box"/>
  <text x="405" y="64" text-anchor="middle" class="dg-text-bold">pending queue</text>
  <text x="405" y="84" text-anchor="middle" class="dg-text-muted">pendingTriggers</text>
  <text x="405" y="100" text-anchor="middle" class="dg-text-muted">(visible in views)</text>
  <rect x="500" y="40" width="130" height="70" rx="8" class="dg-box"/>
  <text x="565" y="64" text-anchor="middle" class="dg-text-bold">APNAP sort</text>
  <text x="565" y="84" text-anchor="middle" class="dg-text-muted">CR 603.3b, then</text>
  <text x="565" y="100" text-anchor="middle" class="dg-text-muted">trigger_order ask</text>
  <rect x="660" y="40" width="110" height="70" rx="8" class="dg-box"/>
  <text x="715" y="64" text-anchor="middle" class="dg-text-bold">stack</text>
  <text x="715" y="84" text-anchor="middle" class="dg-text-muted">TriggerPush,</text>
  <text x="715" y="100" text-anchor="middle" class="dg-text-muted">modes, targets</text>
  <line x1="130" y1="75" x2="158" y2="75" class="dg-line" marker-end="url(#tr-a)"/>
  <line x1="310" y1="75" x2="338" y2="75" class="dg-line" marker-end="url(#tr-a)"/>
  <line x1="470" y1="75" x2="498" y2="75" class="dg-line" marker-end="url(#tr-a)"/>
  <line x1="630" y1="75" x2="658" y2="75" class="dg-line" marker-end="url(#tr-a)"/>
  <text x="235" y="140" text-anchor="middle" class="dg-text-muted">leaves-the-battlefield events are matched</text>
  <text x="235" y="156" text-anchor="middle" class="dg-text-muted">against the board as it was (look-back)</text>
  <text x="565" y="140" text-anchor="middle" class="dg-text-muted">drained by putTriggersOnStack,</text>
  <text x="565" y="156" text-anchor="middle" class="dg-text-muted">just before a player gets priority</text>
</svg>
<figcaption>Triggers are found after every event and placed only when a
player would next receive priority (CR 117.5).</figcaption>
</figure>

- **Matching.** After each event is logged and applied, `checkTriggers`
  ([`engine/rules/trigger_match.go`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/trigger_match.go))
  finds the triggered abilities it made true. The event kind's trigger-interest
  class (`events/kindinfo.go`) limits which faces are examined. The per-mode
  verdict is `trigmatch.Match`: a dense table with one matcher per trigger
  `Mode$`, registered from each mode's file in `rules/trigmatch` (a mode with no
  matcher never fires). Leaves-the-battlefield triggers match against a snapshot
  of the board from before the event (CR 603.10a "look back").
- **Queueing and order.** Matches wait in `pendingTriggers` until
  `priorityRound` calls `putTriggersOnStack`. That sorts them APNAP (active
  player's first, CR 603.3b), asks each controller with two or more for a
  `trigger_order`, chooses modes at placement (CR 603.3c) and pushes each with
  a `TriggerPush` event. Waiting triggers are visible to every seat in the
  view, because they will hit the stack.
- **Optional triggers** ("you may") go on the stack unconditionally; the yes
  or no is asked as the trigger resolves (CR 603.5).
- **Delayed triggers** ("at the beginning of the next end step") are game
  state in `Game.Delayed`, registered by a `DelayedRegister` event and fired
  by `checkDelayedTriggers`.
- **Loop guard.** One trigger fires at most `maxTriggerFires` (256) times for one
  source, and a livelock watcher (`livelock.go`) aborts a game that repeats a
  cycle of events without progress.

## Replacement and prevention effects (CR 614-616)

Replacement effects ("if X would happen, Y happens instead") change an event
**before** it is logged. `Engine.emit` calls `applyReplacements`
([`engine/rules/replacement.go`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/replacement.go))
first; a substituted event is never logged, only the substitute's events are.

```mermaid
sequenceDiagram
  participant Eff as effect
  participant Em as Engine.emit
  participant Rp as applyReplacements
  participant P as affected player
  participant Lg as Log + Apply
  Eff->>Em: MoveZone(creature -> graveyard)
  Em->>Rp: discover applicable replacements (deterministic order)
  alt two or more apply (CR 616.1)
    Rp->>P: replacement decision: which applies first?
    P-->>Rp: choice
    Note over Rp: apply it, re-check what still applies (CR 616.1e)
  end
  Rp-->>Em: rewritten or substituted event
  Em->>Lg: log and fold the result
```

- The replaceable events are the script's `R:Event$` vocabulary (`Moved`,
  `DamageDone`, `Draw`, `GainLife`, `AddCounter`, `CreateToken`, `ProduceMana`,
  `Untap`, `BeginPhase`, ...), most with their own `replacement_*.go` file.
- When several apply, the affected player (the object's controller, or the player
  for a draw) chooses through a `replacement` decision and the event is parked
  until they answer. Applicability is re-evaluated after each rewrite (CR
  616.1e); an optional replacement gets an `apply`/`decline` ask.
- **Prevention is a replacement of the damage event** (CR 615.1): `Prevent$ True`
  replacements, "prevent all damage" keywords and fog effects apply on the damage
  path. Protection (CR 702.16) is checked earlier, at the top of `emit`, and
  records a `Note` ("prevented: protection") instead of the `Damage` event.

## Continuous effects and the layer system (CR 613)

A permanent's current characteristics are its printed values with every active
continuous effect applied in CR 613 order. The engine never stores derived
characteristics in `state.Game`; it computes them on demand from the printed face
plus the effect list.

<figure markdown>
<svg viewBox="0 0 780 470" width="100%" role="img" aria-label="The CR 613 layer stack as implemented: printed face at the bottom, then layers 1 copy, 2 control, 3 text, 4 type, 5 colour, 6 abilities, and layer 7 power and toughness with sublayers 7a to 7e, with current characteristics on top">
  <defs>
    <marker id="ly-a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto"><path d="M0,0 L10,5 L0,10 z" class="dg-arrow-accent"/></marker>
  </defs>
  <rect x="150" y="10" width="480" height="36" rx="6" class="dg-box-solid"/>
  <text x="390" y="33" text-anchor="middle" class="dg-text-on">current characteristics: Engine.Derived(id) / Chars(id)</text>
  <!-- layer 7 -->
  <rect x="150" y="56" width="480" height="150" rx="6" class="dg-box-accent"/>
  <text x="164" y="76" class="dg-text-bold">7  power / toughness</text>
  <text x="440" y="76" class="dg-text-mono">LPT, chars.PT</text>
  <rect x="170" y="84" width="440" height="22" rx="4" class="dg-box"/><text x="182" y="100" class="dg-text">7e switching (SubSwitch)</text><text x="600" y="100" text-anchor="end" class="dg-text-muted">switch P and T</text>
  <rect x="170" y="108" width="440" height="22" rx="4" class="dg-box"/><text x="182" y="124" class="dg-text">7d counters (SubCounters)</text><text x="600" y="124" text-anchor="end" class="dg-text-muted">+1/+1, -1/-1 counters</text>
  <rect x="170" y="132" width="440" height="22" rx="4" class="dg-box"/><text x="182" y="148" class="dg-text">7c modifying (SubModify)</text><text x="600" y="148" text-anchor="end" class="dg-text-muted">+N/+N pumps, anthems</text>
  <rect x="170" y="156" width="440" height="22" rx="4" class="dg-box"/><text x="182" y="172" class="dg-text">7b setting (SubSet)</text><text x="600" y="172" text-anchor="end" class="dg-text-muted">"base P/T becomes 0/1"</text>
  <rect x="170" y="180" width="440" height="22" rx="4" class="dg-box"/><text x="182" y="196" class="dg-text">7a characteristic-defining (SubCDA)</text><text x="600" y="196" text-anchor="end" class="dg-text-muted">"P/T equal to cards in hand"</text>
  <!-- layers 6..1 -->
  <rect x="150" y="214" width="480" height="30" rx="6" class="dg-box"/><text x="164" y="234" class="dg-text-bold">6  abilities</text><text x="300" y="234" class="dg-text-muted">add / remove keywords and abilities; CR 613.8 dependency order</text>
  <rect x="150" y="250" width="480" height="30" rx="6" class="dg-box"/><text x="164" y="270" class="dg-text-bold">5  colour</text><text x="300" y="270" class="dg-text-muted">"is blue", "is all colours"</text>
  <rect x="150" y="286" width="480" height="30" rx="6" class="dg-box"/><text x="164" y="306" class="dg-text-bold">4  type</text><text x="300" y="306" class="dg-text-muted">add / remove types and subtypes (chars.Types)</text>
  <rect x="150" y="322" width="480" height="30" rx="6" class="dg-box"/><text x="164" y="342" class="dg-text-bold">3  text</text><text x="300" y="342" class="dg-text-muted">name changes, text substitution</text>
  <rect x="150" y="358" width="480" height="30" rx="6" class="dg-box"/><text x="164" y="378" class="dg-text-bold">2  control</text><text x="300" y="378" class="dg-text-muted">event-driven: ControlChange, timestamp order</text>
  <rect x="150" y="394" width="480" height="30" rx="6" class="dg-box"/><text x="164" y="414" class="dg-text-bold">1  copy</text><text x="300" y="414" class="dg-text-muted">Object.CopyFace, set by ClonePermanent</text>
  <rect x="150" y="430" width="480" height="30" rx="6" class="dg-box-muted"/><text x="390" y="450" text-anchor="middle" class="dg-text">printed face: cards.Face (Object.Face())</text>
  <!-- side annotations -->
  <line x1="120" y1="456" x2="120" y2="20" class="dg-line-accent" marker-end="url(#ly-a)"/>
  <text x="108" y="240" text-anchor="middle" class="dg-text-muted" transform="rotate(-90 108 240)">applied bottom to top</text>
  <text x="650" y="236" class="dg-text-muted">within a layer:</text>
  <text x="650" y="252" class="dg-text-muted">sublayer, then</text>
  <text x="650" y="268" class="dg-text-muted">timestamp</text>
  <text x="650" y="284" class="dg-text-muted">(stable sort)</text>
</svg>
<figcaption>The layers as implemented in <code>state.Layer</code> and
<code>state.Sublayer</code> (<code>engine/state/continuous.go</code>). The
engine numbers layer 7's sublayers 7a-7e, with counters in their own sublayer
before switching; the current CR text folds counters into 7c and calls
switching 7d. Counters and modifiers both add, so the result is the same.</figcaption>
</figure>

**The effect list.** A `state.ContinuousEffect` has a source, layer and sublayer,
timestamp, an `Affects` filter (the `ValidTgts$` filter language) and the
modification (`AddPower`, `SetPower`, `AddKeywords`, `AddTypes`, `SetName`, ...).
Statics on permanents are rescanned into effects; resolving spells add them
through `Host.AddContinuous`, which emits a `ClockTick` and stamps the effect
with the game clock, so timestamps are part of the log. The registry lives on the
engine (`Engine.continuous`) and is rebuilt identically on replay by
re-execution.

**Ordering.** `active()` ([`engine/rules/layers.go`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/layers.go))
returns the live effects in CR 613 order (layer, sublayer, timestamp) with a
stable sort, so equal keys keep registration order. An effect whose source left
the battlefield drops out (a lord's bonus vanishes when it dies), except one-shot
until-end-of-turn effects. In layer 6, `abilityDependencyOrder` (`rules/chars`)
applies CR 613.8 dependencies: an effect that changes whether another applies
goes first, and a dependency loop falls back to timestamp order.

**The walk.** The read-only computation lives in package `rules/chars`:
`chars.Types` is the layer-4 walk, `chars.Compute` the full layer 3/5/6 walk,
`chars.PT` the layer-7 walk. A caller asks the engine:

```go
func (e *Engine) Derived(id state.ObjID) Derived   // type Derived = effects.Chars
func (e *Engine) Chars(id state.ObjID) *effects.Chars // the effects.Host form
func (e *Engine) Power(id state.ObjID) int32
func (e *Engine) Toughness(id state.ObjID) int32
```

The returned slices (keywords, types) alias the engine's scratch buffers and are
rewritten by the next call; callers copy what they keep. That is what lets
repeated derivation run without allocating.

**Caching.** Every filter match reads keywords and types, so several caches sit
in front of derivation, each keyed on something only an event can move:

| Cache | Keyed on | Why it is safe |
|---|---|---|
| `active()` effect list | log length, continuous-registry version, static build sequence | the board only changes through logged events |
| layer-inert reuse | the same key, ignoring `DecisionAsk`, `DecisionMade` and `Priority` events | those fold nothing a layer reads |
| walk-scoped `Derived` memo | per legal-actions walk, plus log length, registry version and arena size | the walk is a pure read, and derivations inside one walk repeat objects |
| incremental layer 4/5/6 tables | type, colour and keyword grants | rebuilt when their inputs change |

The layer-inert reuse and the `Derived` memo each have a verify mode
(`layerInertVerify`, `derivedMemoVerify`) that recomputes on every hit and panics
on a difference, so a stale entry fails loudly in tests.

## Combat

Combat is "CR 508-510 in miniature"
([`engine/rules/combat.go`](https://github.com/adams-shaun/gorge/blob/main/engine/rules/combat.go)):

```mermaid
sequenceDiagram
  participant AP as Active player
  participant E as Engine
  participant DP as Defending players
  E->>AP: attackers: one option per (attacker, defender) pair
  AP-->>E: chosen pairs
  E->>E: validate (one defender per creature, CR 506.2), DeclareAttackers events
  Note over E: priority round
  E->>DP: blockers: one option per (blocker, attacker) pair
  DP-->>E: chosen pairs
  E->>E: validate (CR 509.1a), DeclareBlockers event
  Note over E: priority round
  alt any first or double strike
    E->>E: first-strike damage pass (CR 510.4), SBAs
    Note over E: priority round between passes
  end
  E->>E: regular damage pass, Damage events, SBAs
  E->>E: end of combat: EndCombatReset
```

- **Legality** (can this creature attack or block that one, menace, "attacks each
  combat if able", goad) is in `rules/combat` as pure predicates over a `Board`.
  The declaration flow, validation and events stay in `rules`.
- **Defenders.** Each attacker may attack any living opponent, planeswalker or
  battle independently; the decision's options are the legal pairs.
- **Damage assignment.** There is no damage assignment order: any division that
  satisfies CR 510.1c is legal, asked as a `choose` decision when the controller
  has a real choice.
- **First strike** is two passes inside the one combat damage step with a
  priority round between them; `actsThisDamageStep` (CR 510.5) decides who deals
  damage in each pass.

## Zones and object identity (CR 400.7)

**The arena.** Every card, token, spell and ability is a `state.Object` in one
dense slice, `Game.Objs`; `Objs[i]` has ID `i+1`, so `ObjID` 0 means "no object".
Zones are lists of IDs indexed by zone and player; the stack is its own list.

**Identity.** An object keeps its `ObjID` for the whole match, in every zone.
CR 400.7 makes a permanent that leaves the battlefield and returns a *new*
object, so each object carries an `Incarnation`, bumped by the `MoveZone` fold
whenever it crosses the battlefield boundary. Anything that must name one
incarnation (a chosen target, a delayed trigger, an ability's source) records the
incarnation with the ID and compares both later. Leaving the battlefield also
resets the object: control returns to its owner, and damage, counters and
attachments are cleared.

```mermaid
stateDiagram-v2
  direction LR
  Hand --> Stack: cast (PutOnStack)
  Stack --> Battlefield: resolves, Incarnation 1 to 2
  Battlefield --> Graveyard: dies, Incarnation 2 to 3
  Graveyard --> Battlefield: reanimated, Incarnation 3 to 4
  note right of Battlefield
    ObjID never changes.
    A target chosen at Incarnation 2
    is illegal at Incarnation 4.
  end note
```

A stable ID plus a counter, rather than a fresh ID per zone change, keeps the
log and views simple and avoids a forwarding table for every cross-zone
reference (the card a trigger remembered, the creature an Aura was cast on). It
also fixes the mapping from each deck card to one object, which dirac's oracle
relies on.

**The game record.** `state.Game`
([`engine/state/game.go`](https://github.com/adams-shaun/gorge/blob/main/engine/state/game.go))
holds the players (life, pool, counters, per-turn tallies), the object arena, the
stack, turn, step, active player, priority holder and pass count, game-over and
winner, extra turns and phases, the monarch and initiative, the timestamp clock,
delayed triggers, and the token and name tables set at genesis. Only the `events`
package may mutate it.

## Mana

Each player's pool is a fixed array, `state.Mana`: one slot per colour plus
colourless (and extra slots for typed mana), alongside restricted mana spendable
only on certain things. Mana abilities resolve immediately without the stack (CR
605) and emit `ManaAdd`; pools empty at the end of each step (CR 500.4) with
`ManaClear`. Payment planning is in
[Cards and primitives](cards.md#costs-and-payment).

## Changing the rules core safely

- Mutate state only by emitting an event (`e.emit`); add a new event kind at
  the end of the list if no existing one fits.
- Read characteristics through `Derived`/`Chars`, never from the printed face.
- Ask through `e.ask` (or `Host.Ask` from an effect), and give the ask a
  deterministic fallback for a host that cannot answer.
- Run the focused tests for the mechanism and `TestHeads`; if a golden head
  moves, find the first diverging event before re-pinning
  ([Events and replay](events.md#golden-heads)).

[Invariants](../invariants.md) lists the rules these follow and what enforces
each one.
