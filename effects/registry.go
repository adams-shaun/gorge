// Package effects implements the primitives Forge card scripts reference. It
// reaches the engine through the Host interface, so it never imports rules and
// the dependency graph stays acyclic.
package effects

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Host is everything an effect may do to a game: read it, and propose events.
// Deliberately tiny — an effect that needs more is a sign the primitive is
// doing rules work that belongs in the rules package.
type Host interface {
	// Game returns the live match state for reading. The returned *state.Game
	// must never be written to directly: every state mutation goes through
	// Emit (which routes through events.Apply), which is what keeps the event
	// log a complete description of the match.
	Game() *state.Game
	Emit(events.Event)
	// EmitDamage emits a Damage event and returns the event that actually
	// landed after replacement effects. A prevention returns a non-Damage
	// result; an amount-changing replacement returns Damage with the applied
	// amount. Damage riders (lifelink/deathtouch/commander damage) must consume
	// this result rather than the proposed event.
	EmitDamage(events.Event) events.Event
	// EmitTap taps the permanent obj with the synchronous provenance a Taps
	// trigger reads but the replayed Tap event does not carry: tapper is the
	// player who tapped it (Forge Card.tap's tapper -- the resolving
	// ability's activator, a cost's payer), and entering marks a permanent
	// being given its tapped entry state by a DB$ Tap | ETB$ True replacement
	// body, which CR 603.2e says never "becomes tapped" (Forge TapEffect's
	// ETB branch sets the state without running Taps triggers). The emitted
	// event is exactly Emit(events.Event{Kind: events.Tap, Obj: obj}), so the
	// hash chain is unaffected; rules.Engine keeps the provenance as event
	// context while the event's triggers are matched.
	EmitTap(obj state.ObjID, tapper state.PlayerID, entering bool)
	// Rand is the engine's seeded generator. Effects that need randomness must
	// use it and nothing else, or replay breaks.
	Rand(n int) int
	// ShuffleLibrary returns a Fisher-Yates permutation of order. The engine
	// owns hypothetical shuffle planning here; effects still emit the sole
	// state-mutating Shuffle event with the returned order.
	ShuffleLibrary(state.PlayerID, []state.ObjID) []state.ObjID
	// AddContinuous registers one continuous effect against the CR 613 layer
	// system (rules.Engine.AddContinuous). This is how Pump, PumpAll, Animate
	// and Protection reach the layer system without effects importing rules,
	// which would be an import cycle (effects sits below rules). Task 19c.
	AddContinuous(state.ContinuousEffect)
	// ContinuousNamed reports whether an ACTIVE continuous effect registered
	// by controller carries the given Name — the ask effEffect's Stackable$
	// False dedup makes before it would register a second copy of the same
	// named effect (Wrenn and Six's emblem: a second [-7] activation does not
	// stack a second instance). Implemented by rules.Engine against its
	// continuous-effect registry; the effects test double scans its own
	// recorded slice.
	ContinuousNamed(controller state.PlayerID, name string) bool
	// TypeChoices returns the creature-type option list a mid-resolution
	// ChooseType ask offers its chooser (task ct1) — the SAME list the
	// cast-time "as this enters" type ask builds (rules/etbOptions' "type"
	// arm, which this method's rules implementation calls), so the two asks
	// and the no-ask fallback can never disagree about what a creature-type
	// choice ranges over. A category this build cannot enumerate (Basic
	// Land, Card, ...) yields nil: the asking primitive never asks for one
	// (it records the loud Note and the deterministic fallback), so nil is
	// unreachable through the ask path.
	TypeChoices(chooser state.PlayerID, category string) []decision.Option
	// RegisterControl records one GainControl effect with the lifetime its
	// LoseControl$ names (CR 611.2b "for as long as", CR 514.2 end of turn),
	// so the engine can end it through a ControlChange event the moment that
	// duration ends. A grant with no duration is permanent: it supersedes
	// every earlier control effect on the object (CR 613.7 timestamp order).
	RegisterControl(ControlGrant)
	// LegalTargets returns the targets the rules engine would offer for sa.
	// Redirect effects use this shared census rather than duplicating target
	// legality below rules (protection and continuous restrictions included).
	LegalTargets(chooser state.PlayerID, source state.ObjID, sa *cards.SA) []state.Target
	// RegenerationDisallowed reports whether an Effect-registered
	// CantRegenerate restriction makes id unable to be regenerated (Incinerate's
	// "can't be regenerated this turn"). Consulted by ReplaceDestruction before
	// it would consume a shield, so a banned regeneration is never honoured.
	// Implemented by rules.Engine against its continuous-effect registry; the
	// effects test double reports false (no engine to consult). Task ce1.
	RegenerationDisallowed(id state.ObjID) bool
	// SacrificeBlocked reports whether id is forbidden from being sacrificed
	// at all this turn — an Effect-registered CantSacrifice restriction (Call
	// for Aid's "You can't sacrifice those creatures this turn") or a face
	// CantSacrifice static (the simple Card.Self carriers). Consulted at every
	// sacrifice candidate choke point (effSacrifice's eligible pool and
	// object-target paths, effSacrificeAll, the cast/activation/mana/ward/unless
	// Sac-cost candidate walks) so a blocked permanent is never offered and
	// never taken. Implemented by rules.Engine (rules/layers.go); the effects
	// test double reports false (no engine to consult).
	SacrificeBlocked(id state.ObjID) bool
	// HasKeyword reports a DERIVED keyword — printed or granted by a
	// continuous effect (rules.Engine.HasKeyword). Effects that gate on a
	// keyword (Destroy on Indestructible) must ask this, never the face.
	HasKeyword(id state.ObjID, kw string) bool
	// UmbraArmorAura returns the ObjID of the first attached Aura whose
	// DERIVED keyword set carries "Umbra armor" (CR 702.90), in
	// deterministic AliveFrom(0) × battlefield-slice order, or 0 if bearer id
	// wears none. Derived, never the printed face: Umbra Mystic's and Dog
	// Umbra's layer-6 grants must be seen. Consulted by ReplaceUmbraArmor.
	UmbraArmorAura(id state.ObjID) state.ObjID
	// Power, Toughness and IsCreature are current derived characteristics.
	// Damage/count effects must not read a printed face when layers modify P/T
	// or make a planeswalker a creature.
	Power(id state.ObjID) int32
	Toughness(id state.ObjID) int32
	IsCreature(id state.ObjID) bool
	// CastThisTurn counts the spells cast this turn by anyone, derived from
	// the event log so a replay that rebuilds the game arrives at the same
	// number (Task 17's Count$ThisTurnCast backing — a copy/Storm count
	// must be replay-derivable, never a live-only engine counter).
	CastThisTurn() int
	// SpellsCastThisTurnMatching counts the spells put on the stack this turn
	// whose caster is you (when the Forge spec carries a You* qualifier) or
	// anyone, and whose object matches the spec. Derived from the event log
	// like CastThisTurn, so a replay derives the same number. This is the
	// Count$ThisTurnCast_<spec> backing (the "first/second spell you cast"
	// cost modifiers and triggers).
	SpellsCastThisTurnMatching(you state.PlayerID, spec string) int
	// SpellsCastThisTurnMatchingExcluding is SpellsCastThisTurnMatching with
	// one object's own cast excluded from the count -- the bare !CastSaSource
	// qualifier's engine reading. Every bare-form carrier's oracle says
	// other/another (Hotheaded Giant's "unless you've cast another red spell
	// this turn", Dream Thief's "another blue spell", Storm Entity's "each
	// other spell cast this turn"), and the resolving spell's own
	// PutOnStack is unavoidably in the window when an ETB gate reads the
	// count, so the qualifier is the count's exclusion of its own ctx source.
	// Derived from the event log like SpellsCastThisTurnMatching.
	SpellsCastThisTurnMatchingExcluding(you state.PlayerID, spec string, exclude state.ObjID) int
	// EachSpellCastThisTurnMatching is the ARGUMENTED !CastSaSource forms'
	// engine side (task castprov2): the object ids of the spells put on the
	// stack this turn matching spec (with the same You*-qualifier scoping
	// and the same single-object exclusion as
	// SpellsCastThisTurnMatchingExcluding), in reverse log order (newest
	// first) — the order is irrelevant to the aggregate reads (a sum).
	// Derived from the event log like SpellsCastThisTurnMatching.
	EachSpellCastThisTurnMatching(you state.PlayerID, spec string, exclude state.ObjID) []state.ObjID
	// WasCastFromHandByYou reports whether card obj was cast from ITS OWN
	// CONTROLLER's hand by that controller — the Count$wasCastFromYourHandByYou
	// branch head backing (the Myojin cycle's etbCounter CheckSVar$ gate:
	// "enters with a divinity counter on it if you cast it from your hand")
	// and the Card.wasCastFromYourHandByYou filter predicate the corpus's
	// "if you cast it from your hand" ETB trigger specs read. An ordinary
	// hand-origin cast carries no CastFlags bit (the flags mark alternative
	// costs and origins only), so the answer is derived from the event log:
	// the object's latest PutOnStack event names the cast that put it on the
	// stack, whose From is the zone it was cast FROM and whose Player is the
	// caster. Derived from the log like CastThisTurn, so a replay derives
	// the same answer; a card never put on the stack (cheated into play)
	// reads false.
	WasCastFromHandByYou(obj state.ObjID, p state.PlayerID) bool
	// LifeLostThisTurn reports the total life player p lost THIS TURN — the
	// sum of every LifeChange below zero since the last TurnChange, derived
	// from the event log so a replay derives the same number. This is the
	// Count$LifeOppsLostThisTurn backing (Rakdos, Lord of Riots' cost
	// reduction): the Count$ head sums it over the controller's opponents.
	LifeLostThisTurn(p state.PlayerID) int32
	// LifeGainedThisTurn reports the total life player p GAINED this turn —
	// the sum of every LifeChange above zero since the last TurnChange,
	// derived from the event log so a replay derives the same number. This is
	// the Count$LifeYouGainedThisTurn backing (the "At the beginning of each
	// end step, if you gained 4 or more life this turn" family — Angelic
	// Accord, Resplendent Angel, Valkyrie Harbinger — whose CheckSVar$ gate
	// reads the count), the mirror of LifeLostThisTurn.
	LifeGainedThisTurn(p state.PlayerID) int32
	// TurnsTaken reports how many of the game's turns have begun with p as
	// the active player, INCLUDING the turn in progress when it is p's —
	// Forge's Player.getTurns backing (Serra Avenger's
	// Count$YourTurns: "your first, second, or third turns of the game").
	// Derived from the event log like LifeLostThisTurn, so a replay derives
	// the same number; a turn that begins is one TurnChange event naming p.
	TurnsTaken(p state.PlayerID) int32
	// AttackersThisTurn counts the attackers declared THIS turn — the sum of
	// every DeclareAttackers event's attacker list since the last TurnChange,
	// derived from the event log so a replay derives the same number. This is
	// the Count$AttackersDeclared backing (the Raid family's "attacked this
	// turn" read: Bloodsoaked Champion's CheckSVar$ activation gate and ten
	// ConditionCheckSVar$ bodies).
	AttackersThisTurn() int
	// CommanderIdentityColourCount reports how many colours seat p's
	// commander colour identity names (the WUBRG-ordered union of every
	// commander's Card.ColourIdentity, read off state.Player.Commanders —
	// genesis bookkeeping the replay rebuilds in Config order, so the count
	// is replay-derivable like TurnsTaken). This is the Count$ColorsColorIdentity
	// backing (War Room's fixed "Pay life equal to the number of colors in
	// your commanders' color identity"); an empty identity (no commander,
	// or a colourless one) is a real, resolvable 0.
	CommanderIdentityColourCount(p state.PlayerID) int
	// Ask poses a decision in the middle of a resolution. It sets the host's
	// pending decision, sets the mid-resolution resume state, and returns
	// true. A true return tells the calling effect to stop and wait: the
	// resolution is suspended, and the answered decision re-enters it
	// (rules' Engine.Ask is what runs the resume continuation). false means
	// the host cannot ask now — an effects-package test double, or a
	// rules-internal context with no engine to drive — and the calling
	// effect falls back to its deterministic stand-in (R-9). M2d-2.
	Ask(d *decision.Decision) bool
	// SuspendUnless records the unless-cost outcome of an SA whose BODY
	// suspended on a mid-resolution ask of its own (the gate had already
	// resolved when the body asked): rules re-enters that asking SA with a
	// fresh Ctx, so without this record the gate would re-pose its pay ask
	// and the body would run again from the top — a livelock for any asking
	// body under an UnlessCost$ (Rhystic Study's pay-or-draw, a unless-gated
	// Dig's search). paid is the outcome the suspended pass resolved; the
	// re-entry pass consumes the recorded marker instead of asking again.
	SuspendUnless(sa *cards.SA, paid bool)
	// Suspended reports whether the resolution is currently suspended on a
	// mid-resolution ask — Ask returned true and set the host's resume state,
	// which has not yet been cleared by the answer arriving. effects.Resolve
	// calls it after every sub-ability in a chain so that a suspended ask
	// STOPS the chain rather than running the sub-abilities beneath it (B1: a
	// chained SA such as Thoughtseize's Discard | SubAbility$ DBLoseLife must
	// not fire its SubAbility on the initial pass, before the answer exists,
	// nor again on the resume pass — the resume re-enters at the asking SA
	// and walks the rest of the chain exactly once). An effects-package test
	// double that cannot suspend reports false; the rules engine reports
	// e.resume != nil, which Ask sets and the handled answer clears.
	Suspended() bool
	// SuspendContinuation reports that a Resolve loop has just suspended (its
	// host reported Suspended() after running the sub-ability at sa) and is
	// about to return, so the chain it was walking must resume at sa.Sub once
	// the pending answer and any deeper continuations are done. effects.Resolve
	// calls it at EVERY loop level that suspends, including the innermost
	// (the level whose sa is the pending ask's ResumeSA): a host records the
	// enclosing levels (sa != the pending ResumeSA) as outer continuations and
	// drops the innermost one, because re-entering the pending ask's own SA
	// already walks sa.Sub. A host that never suspends (an effects-package
	// double, where Ask returns false) never sees this call.
	SuspendContinuation(sa *cards.SA)
	// BeginDamageBatch/EndDamageBatch bracket the Damage events one
	// dealDamage-style call deals simultaneously (Forge dealDamage, GameAction
	// AddDamage/triggerDamageDoneOnce): within the bracket, the
	// DamageDealtOnce/DamageDoneOnce triggers latch once per batch per
	// referent (per dealing source / per damaged object) and the queued
	// trigger's referent amount is the batch's accumulated total -- Fireball
	// splitting among three creatures is ONE batch to a DamageDealtOnce
	// trigger on the source, not three. rules.Engine implements both; the
	// effects-package test double reports no-ops. Neither suspends.
	BeginDamageBatch()
	EndDamageBatch()
	// ReplaceEvent applies a ReplaceEffect body's requested change to the
	// event currently being replaced. It is inert outside replacement
	// resolution; rules owns the event and records the resulting delta.
	ReplaceEvent(name, value string, resolved int32)
	// CounterAllowed reports whether a spell or ability may be countered.
	// Counter replacement effects are rules, not a MoveZone replacement: they
	// stop Counter before it emits the move off the stack.
	CounterAllowed(target, cause state.ObjID) bool
	// SuspendRepeat reports that one iteration of a RepeatEach loop suspended
	// at a mid-resolution ask. The host must bind the suspended iteration's
	// Remembered to the pending ask (and to the iteration's own continuation
	// frames) and resume the loop at the next subject once they complete,
	// rather than dropping the remaining subjects (CR 608.2c). The Resolve
	// loop enclosing the RepeatEach reports that SA through
	// SuspendContinuation next; the host drops that report, because the loop
	// frame re-enters the RepeatEach itself and so walks its Sub.
	SuspendRepeat(RepeatSuspension)
	// SuspendCharmRest reports that a cross-mode TargetUnique Charm's mode
	// loop (effCharm's re-entry) suspended mid-mode with chosen modes still
	// to run: sa is the Charm's own SA and rest the remaining chosen mode
	// names in execution order. The host records a continuation that
	// re-enters the Charm with Ctx.Modes = rest once the answered ask's own
	// chain completes — the remaining modes must not run while the
	// suspension is live. The Resolve loop enclosing the Charm reports that
	// same SA through SuspendContinuation next; the host drops that report
	// (the charm frame re-enters the Charm itself), which is why the reporter
	// marks it the way SuspendRepeat marks a RepeatEach.
	SuspendCharmRest(sa *cards.SA, rest []string)
	// SetDamageSource overrides the in-flight damage source for the Damage
	// events the caller is about to emit: the provenance rules' emit-side
	// protection check (CR 702.16d) and DamageDone trigger matching read
	// for every Damage event. It returns the previous override so the
	// caller restores it before returning; zero restores "no override".
	// The override is engine-transient state exactly like the resolution
	// source it wraps: replay re-executes the same setter, and Clone never
	// copies it because an emitter always restores before returning
	// (DealDamage/DamageAll never ask mid-loop, so nothing suspends inside
	// the override window).
	SetDamageSource(id state.ObjID) state.ObjID
	// BatchDepartures declares that the caller is about to emit MoveZone
	// events for every object in ids as one simultaneous destruction batch
	// (CR 704.3): the engine snapshots each object's derived lifelink
	// state NOW, before any of the moves fold, so a later batch member's
	// CR 603.10a departure capture reads the batch's own pre-state rather
	// than whatever an earlier member's departure already stripped (a
	// destroy-all over a lifelink-granting Equipment and its bearer: the
	// bearer's lifelink LKI must not depend on battlefield order). Entries
	// are consumed by the matching departure capture. EndBatchDepartures
	// clears any remaining entry after the effect loop, including a member
	// regeneration kept on the battlefield.
	BatchDepartures(ids []state.ObjID)
	EndBatchDepartures()
}

// RepeatCursor is a RepeatEach loop re-entered after an iteration suspended:
// the subjects captured when the loop started (never re-derived mid-loop),
// the index of the next subject, and the completed iteration's final
// Remembered so the objects that iteration remembered outlive it.
type RepeatCursor struct {
	SA       *cards.SA
	Subjects []state.Target
	Next     int
	Last     []state.Target
	HasLast  bool
}

// RepeatSuspension is what effRepeatEach reports when an iteration asks.
// Body is the suspended iteration's Remembered (the loop subject plus
// anything the iteration remembered before asking); Subject is that
// iteration's current subject (the Imprinted binding); Outer and Chosen are
// the RepeatEach resolution's own bindings, restored when the loop re-enters.
type RepeatSuspension struct {
	RepeatCursor
	Body        []state.Target
	Subject     state.Target
	Outer       []state.Target
	Chosen      []state.Target
	ChosenValid bool
}

// DamageSourceLKI is the pre-departure damage provenance of one object.
// It remains separate from Ctx's own-source fields because DamageSource$ may
// name an object distinct from the resolving spell or ability's source.
type DamageSourceLKI struct {
	Lifelink   bool
	Controller state.PlayerID
}

// Ctx carries the bindings a Forge script refers to during resolution.
type Ctx struct {
	TriggerContext
	Source     state.ObjID
	Controller state.PlayerID
	Targets    []state.Target
	Remembered []state.Target
	// TargetsOffered marks that the resolution's OWN ValidTgts$ targeting was
	// already offered at announcement (rules' resolveTop sets it on both the
	// ability and the spell branch, exactly for the SA the placement ask
	// covered). Without it a Min-0 target the chooser elected ZERO of would
	// look identical to a targeting that was never offered (both leave
	// Ctx.Targets empty), and effChangeZone's mid-resolution ask
	// (changeZoneChosenTargets) would pose the same question twice. A fresh
	// ctx rebuilt by a resume does not carry it -- a deeper sub's targeting
	// was genuinely never offered, which is the ask's real population.
	//
	// Boundary, updated by task mvts1: the flag suppresses only the depth-0
	// entry SA of an effects.Resolve call (chosenTargetsFor's atRoot arm) --
	// the SA the placement/announcement ask actually covered. A deeper sub
	// in the SAME resolution that carries its OWN never-offered ValidTgts$
	// now poses its own ask there (the trigger "when you do" family: Mogg
	// Bombers' DealDamage, Kor Outfitter's Attach); before mvts1 it either
	// inherited the outer targets or moved nothing silently. A nested
	// Resolve entry at depth 0 whose SA is genuinely never-covered (a
	// RepeatEach iteration body) is also suppressed while the flag is set --
	// the conservative direction, same as the pre-mvts1 ChangeZone shape.
	TargetsOffered bool
	// Captured is the part of Remembered the resolution started with because
	// its trigger, delayed trigger or replacement put the event's object there
	// (this engine's stand-in for Forge's separate TriggeredCard), rather than
	// because a Remember* parameter of the resolution chose it. Forge keeps
	// neither in a host's remembered list, so a RepeatEach over players does
	// not carry these into its iterations.
	Captured []state.Target
	// SourceLifelinkLKI is the source permanent's derived lifelink state at
	// the last moment it existed on the battlefield. The validity bit is
	// separate because "it did not have lifelink" is authoritative LKI too.
	// Rules seeds this on independently resolving abilities; damage uses it
	// only after the source has departed, and continues to read the live
	// derived source while it remains a permanent.
	SourceLifelinkLKI      bool
	SourceLifelinkLKIValid bool
	// SourceControllerLKI is the source permanent's controller immediately
	// before it left the battlefield. Move resets Controller to Owner, so an
	// independently resolving lifelink ability needs this companion snapshot
	// to credit its last controller rather than its owner.
	SourceControllerLKI      state.PlayerID
	SourceControllerLKIValid bool
	// DamageSourceLKI preserves lifelink and controller LKI by object id for
	// a distinct DamageSource$ object that left while this resolution waited.
	// Rules transports it with the stack object; DamageSource$ consults it only
	// after that named object is no longer a battlefield permanent.
	DamageSourceLKI map[state.ObjID]DamageSourceLKI
	// Sacrificed carries the last-known-information snapshot of every object
	// this resolving spell/ability sacrificed, as it was at the instant of the
	// sacrifice (state.SacrificedInfo). Built two ways, feeding one field: a
	// cost-paid sacrifice carries it onto the stack object (rules/cast.go
	// commitCast) and resolution loads it here, while an effect-driven
	// sacrifice (effSacrifice with RememberSacrificed$ True) appends here
	// directly so a SubAbility$ chained after it can read it. The
	// Sacrificed$<Property> heads in count.go read it.
	Sacrificed []state.SacrificedInfo
	// ResolvingObj is the stack-object WRAPPER of the spell/ability currently
	// resolving -- rules' e.resolvingObj (resolveTop's ability and spell
	// branches) and rp.obj (resumeResolution) -- set at those two ctx
	// construction sites. For an ability resolution Ctx.Source is the source
	// PERMANENT (Ruling T20-b: Defined$ Self must resolve to something with a
	// face), so a ValidStack qualifier that means "not the ability resolving
	// right now" (Ulalek's `Ability.YouCtrl+otherAbility`) cannot anchor on
	// Source: the permanent is not on the stack and excludes nothing. This
	// field is resolution-scratch like Targets/SVars -- never event-encoded,
	// a replay re-derives the same binding -- and zero on contexts built off
	// the resolution path (hand-built test probes), where ValidStack's
	// otherAbility falls back to Ctx.Source. Never widened.
	ResolvingObj state.ObjID
	// SVars is the resolving card's SVar table, and X the value paid for {X}.
	// Both are bound by the rules package when it builds the context.
	SVars map[string]string
	X     int32
	// TimesKicked is the pending cast's settled multikicker payment count
	// (CR 702.43), seeded by rules' targetBoundCtx when the spell's OWN
	// announcement ask resolves a Count$TimesKicked bound BEFORE payment has
	// stamped the stack object (Comet Storm's TargetMin/Max$ TargetsNum).
	// Everywhere else it is zero and the TimesKicked count head falls back to
	// the source object's stamped field -- the same priority the xPaid head
	// gives ctx.X over the object read.
	TimesKicked int32
	// ChosenNumber is the Effect's SetChosenNumber$ binding (task
	// wildgrowth1): the number the Effect resolved at creation, threaded into
	// a registered replacement's body Ctx by rules' replCtx so the body's
	// Count$ChosenNumber head (evalCountBody) reads the frozen binding rather
	// than re-deriving. Zero wherever nothing bound -- the same number a
	// failed binding degrades to.
	ChosenNumber int32
	// ChosenNumberBound marks a Ctx whose ChosenNumber IS a real
	// SetChosenNumber$ binding (rules' seedEffectReplCtx sets it exactly when
	// the match is effect-created, m.key != ""). It is the Count$ChosenNumber
	// head's verdict: bound means evaluated (the value reads, zero
	// legitimately), unbound means the head is UNRESOLVED so the
	// EvalCountOK consumers keep their pre-wildgrowth fail direction --
	// CheckSVarHolds fails open, a numeric filter RHS (cmcEQX via
	// resolveNumericRHS) never matches -- instead of enforcing a meaningless
	// zero on the Choose-event corpus population (77 files whose binding
	// lives on state.Object.ChosenNumber via effects/choose.go, never on
	// Ctx). A zero binding with the flag set is still bound (torgal with no
	// Dogs); only the flag distinguishes the two.
	ChosenNumberBound bool
	// Host is the engine driving this resolution, bound by effects.Resolve
	// itself (it receives the host as its own parameter, so every walk that
	// can reach a resolution-time filter evaluation has passed through one
	// set here) rather than at every Ctx construction site. Ctx.SpecContext
	// consults it to resolve a numeric filter RHS through the SVar table
	// (EvalCountOK -- Nightmare Unmaking's Creature.powerGTX against
	// SVar:X:Count$ValidHand Card.YouOwn, Whir of Invention's
	// Artifact.cmcLEX against the paid X). It stays nil on contexts that
	// never entered Resolve -- the direct Num/EvalCount probes -- which keeps
	// those read-only and resolver-free exactly as they have always been.
	Host Host
	// numericRHS is the cheap gate SpecContext's resolver install reads:
	// effects.Resolve computes it on entry (a paid X, or any SVar table at
	// all -- the resolver itself decides per name and fails closed on a name
	// with no resolvable body, so the broad flag never widens a match), so
	// the gate at the SpecContext call site is one field read and that call
	// site stays inside the inline budget the warm Derived escape-analysis
	// pin (rules/layers_test.go) enforces. Hand-built contexts (the direct
	// Num/EvalCount probes) leave it false and stay resolver-free.
	numericRHS bool
	// resolvingRHS is the one-level recursion guard on the SVar-body
	// resolution SpecContext installs: an SVar body that itself counts a spec
	// carrying the same numeric RHS (Count$Valid Creature.powerGTX named by
	// the SVar that resolves powerGTX) would otherwise recurse unboundedly
	// through SpecContext -> resolveNumericRHS -> EvalCountOK ->
	// MatchesSpecCtx -> resolveNumericRHS. A re-entrant ask fails closed
	// (never matches), the documented unresolvable-RHS contract. Not
	// event-backed, not state: resolution-scratch like Targets or SVars.
	resolvingRHS bool
	// Replaced is the object the replaced event was about (Defined$ ReplacedCard):
	// the card a "would go to the graveyard from anywhere, exile it instead"
	// replacement is acting ON. Set by rules/replacement.go on the context it
	// builds for a matching ReplaceWith$; zero outside a replacement, and nil for
	// a zero (or gone) object when Defined resolves it. It is context, not state
	// -- it drives the replacement's own resolution but is never itself persisted
	// to the event log.
	Replaced state.ObjID
	// ReplacedPlayer is the player a replaced DRAW event was about — the
	// draw-er (Breathstealer's Crypt draws/reveals/discards "that player",
	// Zur's Weirding's other players pay relative to them). Set only on a
	// Draw replacement's own context, like Replaced; zero outside one.
	ReplacedPlayer state.Target
	// ReplacementTarget, ReplacementSource and
	// ReplacementAmount carry the corresponding roles of an in-flight damage
	// ReplacementAmount carry the corresponding roles of an in-flight damage
	// event. They are resolution context, never persisted state; rules seeds
	// them before resolving ReplaceWith$ so ReplacedTarget/ReplacedSource and
	// ReplaceCount$DamageAmount are available to every replacement body API.
	ReplacementTarget state.Target
	ReplacementSource state.ObjID
	ReplacementAmount int32
	// LKI is the object a zone-change trigger fired for, as it was just
	// before the move (CR 603.10 "look back in time"): Move resets counters,
	// tapped state and damage on the way out, so a "dies" condition such as
	// Undying's "if it had no +1/+1 counters" must read this, not the live
	// object. nil for every other trigger.
	LKI *state.Object
	// LKIPower/LKIToughness are that snapshot's derived battlefield P/T,
	// captured before the move removes continuous effects. The validity bit
	// distinguishes a real zero from a non-battlefield/no-characteristic LKI.
	LKIPower, LKIToughness int32
	LKIPTValid             bool
	// Modes is the answered modal choice on a re-entered mid-resolution
	// resolution (M2d-2): the SVar names of the chosen Choices$ sub-abilities,
	// in execution order. rules' resumeResolution sets it from the recorded
	// answer before re-running the suspended sub-ability, so effCharm's
	// re-entry runs exactly the chosen modes instead of asking again. Nil on
	// the first pass and on any non-modes resume.
	Modes []string
	// UnlessPay is the answered unless-pay choice on a re-entered
	// mid-resolution resolution (M2d-2): "pay" means rules' resumeResolution
	// has already paid the UnlessCost$ from the payer's pool and the asking
	// effect proceeds with its body; "decline" means it proceeds as if the
	// player declined (no effect). "" on the first pass, where the effect
	// poses the ask instead. For Sacrifice's damage-payment shape (Vexing
	// Devil), "pay" additionally means the accepting opponent's Damage event
	// has already been emitted by rules' resume arm — payment events belong
	// to rules, never to the effects layer.
	UnlessPay string
	// Discard is the answered "Mode$ RevealYouChoose" discard choice on a
	// re-entered mid-resolution resolution: the object(s) the caster named
	// to be discarded from the target's hand. rules' resumeResolution sets
	// it from the recorded answer before re-running the suspended sub-ability,
	// so effDiscard's re-entry discards exactly the chosen cards instead of
	// asking again. Nil on the first pass and on any non-discard resume. The
	// ask itself carries the caster as the chooser and the target as the
	// discarder, which is why a plain ObjID is not enough state to rebuild:
	// the two player roles are re-derived from Ctx on re-entry.
	Discard []state.ObjID
	// Choice is the selected card(s) or player(s) from ChooseCard,
	// ChoosePlayer, or ChangeTargets. ChoiceDone distinguishes an answered
	// empty optional choice from its first pass.
	Choice     []state.Target
	ChoiceDone bool
	// TargetsPick is the answered target set of the generic ValidTgts$
	// pre-ask (chosenTargetsFor, posed inside effects.Resolve's dispatch
	// loop for a sub the placement/announcement ask never covered -- the
	// trigger "when you do" family, task mvts1). rules' "tgts" resume arm
	// fills it on the re-entered pass; the pre-ask consumes and clears it
	// (fx42 scoping: each resume builds a fresh Ctx and re-enters exactly
	// the asking SA, so nothing else can be holding it). The answer rides
	// its own resume kind ("tgts") and its own Ctx transport rather than
	// the shared Choice pair so another KChoose primitive resolving under
	// the same SA can never steal it.
	TargetsPick     []state.Target
	TargetsPickDone bool
	// OfferedSA is the SA whose ValidTgts$ targeting the placement or
	// announcement ask actually covered (rules' resolveTop and
	// resumeResolution both set it; chosenTargetsFor skips exactly that SA,
	// matched by SA.Line -- ResolveSVar parses fresh on every call, so
	// pointer identity does not hold between two derivations of the same
	// body, the matching convention rules' charmModeTarget already
	// established).
	OfferedSA *cards.SA
	// PickedTargets is the answering pre-ask's target set, made visible to
	// Defined's ValidTgts$ fallthrough for exactly ONE dispatch (the
	// wrapper clears it when the body returns). It must not be Ctx.Targets:
	// a CLOBBER sub names its parent's target explicitly (Object$
	// ParentTarget, Defined$ Targeted), and overwriting Ctx.Targets would
	// point those referents at the sub's OWN answer instead of the outer
	// target the script meant.
	PickedTargets []state.Target
	// ChoiceTarget is the index of the per-player chooser currently being
	// resumed. It keeps multi-player ChooseCard/ChoosePlayer asks from
	// returning to the first chooser after every answer.
	ChoiceTarget int
	// Chosen holds card/player choices for the remaining resolution chain.
	// Unlike Choice it is not the transport for a pending answer; filters such
	// as Creature.nonChosenCard consult it after ChooseCard has returned.
	Chosen      []state.Target
	ChosenValid bool
	// Repeat is set only on the re-entry of a suspended RepeatEach loop; the
	// RepeatEach whose SA it names consumes and clears it.
	Repeat *RepeatCursor
	// RepeatSubject is the RepeatEach iteration's current subject — what
	// Forge's UseImprinted$ binds as "Imprinted" for the sub-ability the
	// loop resolves (Heroism's attacking red creature, Stench of Evil's
	// destroyed Plains). effRepeatEach sets it per iteration; the suspension
	// machinery carries it through a resumed ask the way loopRemembered
	// carries the iteration's Remembered. Zero outside a loop iteration, and
	// the Imprinted/ImprintedController selectors fail closed on zero.
	RepeatSubject state.Target
	// Sacrifice is an Annihilator sacrifice answer on re-entry.
	Sacrifice []state.ObjID
	// Search is the answered hidden-library KChoose selection on a re-entered
	// ChangeZone resolution. SearchDone distinguishes "answered with no cards"
	// from the first pass; Search preserves the player's answer order. The
	// asking effect consumes and clears both before continuing, so a nested
	// search cannot inherit the outer answer.
	Search     []state.ObjID
	SearchDone bool
	// SearchShuffle is the answered ShuffleNonMandatory$ may-shuffle confirm
	// ("yes"/"no") on a re-entered ChangeZone search; SearchShuffleMoved
	// carries the objects the search's first pass moved, so the re-entry can
	// run the LibraryPosition$ placement after the answered shuffle. Both
	// ride the ask (the moved list via Decision.ResumeMoved, the same
	// runtime-continuation class as ResumeRemembered) and are consumed and
	// cleared at the re-entry's top (fx42 scoping), so a nested search poses
	// its own confirm.
	SearchShuffle      string
	SearchShuffleMoved []state.ObjID
	// AttachOpt is the answered Optional$ True attach election ("yes"/"no")
	// on a re-entered Attach resolution (Ajani's Chosen's "you may attach it
	// to the token", Cori-Steel Cutter's "you may attach this Equipment to
	// it"): "yes" attaches, anything else declines. It rides the ask (the
	// same runtime-continuation class as ResumeRemembered) and is consumed
	// and cleared at the re-entry's top (fx42 scoping), so a nested Attach
	// poses its own ask.
	AttachOpt string
	// PutOpt is the answered Optional$ True put-counter election ("yes"/"no")
	// on a re-entered PutCounter resolution (Talus Paladin's "you may put a
	// +1/+1 counter on CARDNAME", Black Widow's "You may put ... If you
	// don't, ..."): "yes" places the counters through the ordinary path,
	// anything else declines and the chained SubAbility$ still runs. It rides
	// the ask (the same runtime-continuation class as ResumeRemembered) and
	// is consumed and cleared at the re-entry's top (fx42 scoping), so a
	// nested PutCounter poses its own ask.
	PutOpt string
	// Extort is the answered optional {W/B} payment on a re-entered Extort
	// resolution (M2d-2): "pay" means the caster agreed to pay and the drain
	// runs; anything else ("decline", first pass with a host that cannot ask)
	// means no drain. rules' resumeResolution sets it from the recorded answer
	// before re-running the suspended effExtort, and effExtort clears it after
	// reading so a nested Extort below it poses its own ask.
	Extort string
	// Play is the answered card a resolved Play effect chose to play from a
	// zone (CR 701.23): the object the controller selected among the offered
	// candidates. rules' resumeResolution sets it from the recorded answer
	// before re-running the suspended effPlay, which then casts/plays it from
	// its own zone. PlayDone distinguishes "answered (possibly with no card)"
	// from the first pass.
	Play     state.ObjID
	PlayDone bool
	// DrawDone is the number of individual draws a multi-card Draw has already
	// completed. A dredge choice suspends between draws; rules restores this
	// cursor after applying the selected replacement so the enclosing Draw
	// continues rather than restarting or abandoning its remaining cards.
	DrawDone int32
	// Imprint is the selected public-zone ChangeZone card. It is scoped to
	// Imprint$ True so a nested ordinary ChangeZone cannot consume it.
	Imprint     []state.ObjID
	ImprintDone bool
	// Untap is the answered UntapType$ selection. UntapDone distinguishes an
	// answered empty "up to" choice from the first pass and scopes the answer
	// to the Untap primitive that asked.
	Untap     []state.ObjID
	UntapDone bool
	// Dig is the answered Dig look-and-take pick on a re-entered mid-resolution
	// resolution: the object(s) the library's owner picked out of the top
	// DigNum$ window to move to DestinationZone$, in the player's answer
	// order. rules' resumeResolution sets it from the recorded answer before
	// re-running the suspended sub-ability, so effDig's re-entry moves exactly
	// the chosen cards instead of asking again; DigDone distinguishes
	// "answered (possibly with no cards)" from the first pass, and DigTarget
	// identifies the Defined$ target whose library posed that ask. Re-entry
	// skips earlier targets (already processed before suspension), applies the
	// answer at DigTarget, then preserves the former deterministic processing
	// for later targets until per-library chained asks exist. The asking effect
	// consumes and clears all three fields at the top of its own walk (the fx42
	// scoping discipline), so a nested Dig cannot inherit the outer answer.
	Dig       []state.ObjID
	DigDone   bool
	DigTarget int
	// DigUntilMove is the answered DigUntil reveal-until OptionalFoundMove$
	// election (task diguntil1; Songbirds' Blessing's "You may put that card
	// onto the battlefield. If you don't, put it into your hand."): "yes"
	// moves the found card(s) to FoundDestination$, "no" — the decline — to
	// OptionalNoDestination$ when the SA carries one, else the found card
	// joins the revealed pile (RevealedDestination$). rules' resumeResolution
	// sets it from the recorded answer before re-running the suspended
	// sub-ability, and DigUntilMoveDone distinguishes "answered" from the
	// first pass (it also suppresses the reveal Note and the withheld-params
	// Note a re-entry would otherwise re-emit). The asking effect consumes
	// and clears both at the top of its own walk (the fx42 scoping
	// discipline), so a nested DigUntil cannot inherit the outer answer.
	DigUntilMove     string
	DigUntilMoveDone bool
	// CounterDist is the answered DividedAsYouChoose$ PutCounter pick
	// (Vastwood Hydra's death trigger): the recipients the chooser picked out
	// of the Choices$-eligible battlefield creatures, in answer order.
	// rules' resume arm sets it before re-running the suspended sub-ability,
	// so effPutCounter's re-entry distributes the CounterNum$ total over
	// exactly the chosen creatures instead of asking again;
	// CounterDistDone distinguishes "answered, possibly with no creatures"
	// (a MinChoiceAmount$ 0 decline) from the first pass. The asking effect
	// consumes and clears both at the top of its own walk (the fx42 scoping
	// discipline), so a nested PutCounter cannot inherit the outer answer.
	CounterDist     []state.ObjID
	CounterDistDone bool
	// CounterPick is the answered bare-Choices$ PutCounter pick (Promise of
	// Loyalty's vow: the chooser picked the creature(s) — WITHOUT a
	// DividedAsYouChoose$ total, so each chosen creature takes the full
	// CounterNum$) out of the Choices$-eligible battlefield creatures, in
	// answer order. rules' resume arm sets it before re-running the
	// suspended sub-ability, so effPutCounter's re-entry places the counters
	// on exactly the chosen creatures instead of asking again;
	// CounterPickDone distinguishes "answered" from the first pass. The
	// asking effect consumes and clears both at the top of its own walk (the
	// fx42 scoping discipline), so a nested PutCounter cannot inherit the
	// outer answer.
	CounterPick     []state.ObjID
	CounterPickDone bool
	// UnlessNext is the index of the UnlessPayer$ payer whose answered
	// unless-pay choice this re-entry applies (0 on a first pass). The
	// unlessProceed gate (Resolve) consumes and clears it; rules' resume
	// arm copies it off the resume point, where Ask stored the asking
	// decision's ResumeTarget. A decline moves the gate on to payer idx+1,
	// so a multi-payer UnlessPayer$ asks each payer in turn.
	UnlessNext int
	// SacPicks is the answered per-player sacrifice choice on a re-entered
	// Sacrifice resolution: the object(s) the sacrificing player chose to
	// sacrifice, in the player's answer order. SacDone distinguishes
	// "answered (possibly with nothing)" from the first pass and SacTarget
	// identifies the Defined$ target index whose player posed that ask, so
	// re-entry skips targets already processed before suspension and
	// continues asking later targets. The asking effect consumes and clears
	// all three at the top of its own walk (the fx42 scoping discipline), so
	// a nested sacrifice below it poses its own ask instead of inheriting.
	SacPicks  []state.ObjID
	SacDone   bool
	SacTarget int
	// SacOptional is the answered first step of an Optional$ + StrictAmount$
	// sacrifice: "sacrifice" means its player elected the exact batch and
	// "decline" means they did not. It is separate from SacPicks because a
	// KChoose represents a range, while this Forge shape permits only zero or
	// exactly Amount$. SacOptionalTarget identifies that player's target slot.
	SacOptional       string
	SacOptionalTarget int
	// UnlessElected is the answered UnlessType$ election of a Discard carrying
	// UnlessType$ (Thirst for Knowledge's "discard two cards unless you
	// discard an artifact card"): "unless" means the player elected the
	// one-card-of-the-type alternative, "ordinary" the NumCards$ discard. The
	// re-entered effDiscard consumes and clears it (fx42 scoping discipline);
	// it is separate from Discard because the unless arm's own follow-up ask
	// re-uses the ordinary "discard" resume kind for its one-card pick.
	UnlessElected string
	// Arrange is the answered KArrange decision on a re-entered
	// mid-resolution resolution (Ruling J0): true once rules' handleArrange
	// has applied the answered arrangement and emitted the LibraryOrder
	// event, so effRearrangeTopOfLibrary's re-entry lets the resolution
	// continue (the chained SubAbility$ runs) instead of re-asking. False on
	// the first pass, where the effect poses the ask. The arrangement itself
	// lives on the LibraryOrder event, not on Ctx -- the answer shape is
	// applied by the rules handler, unlike Modes/UnlessPay/Discard where the
	// effect re-reads the answer -- so the field is only a done-marker.
	Arrange bool
	// MayShuffle is the answered may-shuffle ask a RearrangeTopOfLibrary
	// carrying MayShuffle$ True (Ponder's "You may shuffle.") poses after its
	// KArrange was applied: "yes" means the player shuffled (rules'
	// arrange_mayshuffle resume arm emitted the Shuffle event before
	// re-entering the effect), "no" means they kept the order. Both values
	// are done-markers: the re-entered pass must not pose the ask again. The
	// field is consumed and cleared by the effect (fx42 scoping discipline).
	MayShuffle string
	// Hideaway holds the selected top-library card while the Hideaway
	// replacement resumes to exile it; HideawayPicked distinguishes that
	// selected answer from the first pass. HideawayArranged marks completion
	// of the following bottom-order KArrange ask.
	Hideaway         state.ObjID
	HideawayPicked   bool
	HideawayArranged bool
	// SoulbondPartner is the optional pairing answer. SoulbondDone makes a
	// declined empty choice distinct from the initial pass.
	SoulbondPartner state.ObjID
	SoulbondDone    bool
	// Myriad is one per-opponent optional token decision. MyriadTarget is the
	// index in the deterministic eligible-opponent list that just answered;
	// MyriadDone distinguishes that answer from the first pass, and
	// MyriadCreate says whether it creates that target's token. Re-entry emits
	// the selected token, then asks the next opponent, so each may choice is
	// independent and no answer is retained by a nested Myriad.
	MyriadTarget int
	MyriadDone   bool
	MyriadCreate bool
	// ManaAmount and ManaType are the in-flight unit of mana a ProduceMana
	// replacement modifies. rules seeds them from a ManaAdd event and then
	// emits the transformed event, so ReplaceMana never writes game state
	// directly and replay records the final mana production normally.
	ManaAmount int32
	ManaType   string
	// ManaChoice is the W/U/B/R/G answer to a choice-valued ReplaceMana
	// body (ReplaceType$ Any, ReplaceColor$ Chosen, ReplaceMana$ Any).
	// Rules parks the ManaAdd and supplies this on resume.
	ManaChoice string
	// HandMove is the answered Origin$ Hand ChangeZone selection.
	HandMove     []state.ObjID
	HandMoveDone bool
	// HandMoveTarget is the index of the per-owner hidden-hand chooser whose
	// ask was answered (rv2b r2: an owner-SELECTED Origin$ Hand ChangeZone --
	// DefinedPlayer$/ValidTgts$ naming the hands -- asks each hand owner in
	// turn). It keeps a resumed answer attached to the exact owner that
	// asked, so owners before the cursor (already answered on earlier
	// passes) are skipped and owners after it continue the chain, the same
	// continuation effDig's DigTarget carries. Consumed and cleared at the
	// top of the walk with HandMove/HandMoveDone (fx42 scoping).
	HandMoveTarget int
	// HiddenPick is the answered Hidden$ True public-origin pick (hiddenpick1):
	// the chooser picked which of the ChangeType$-eligible cards in the
	// origin zone(s) move to Destination$. HiddenPickDone distinguishes
	// "answered, possibly with no cards" from the first pass; HiddenPick
	// preserves the player's answer order. effHiddenPick consumes and
	// clears both at the top of its walk (fx42 scoping), so a nested pick
	// cannot inherit the outer answer.
	HiddenPick     []state.ObjID
	HiddenPickDone bool
	// HiddenPickTarget is the index of the fetch player whose hidden-pick ask
	// was answered, the same continuation HandMoveTarget carries: owners
	// before the cursor are skipped on re-entry, owners after it continue
	// the chain. Consumed and cleared with the pair above.
	HiddenPickTarget int
	// DefinedLibraryMove is the answered Optional$ True choice for an
	// object-valued Defined$ fetch list from Origin$ Library. "yes" moves the
	// list; "no" leaves it in place. It is consumed by
	// moveDefinedLibraryObjects before a nested fetch list can inherit it.
	DefinedLibraryMove string
	// RevealOpt is the answered RevealOptional$ yes/no on a re-entered
	// mid-resolution reveal (task fb-3f1cc033, the Delver of Secrets
	// PeekAndReveal shape): "yes" means the peeking player chose to reveal
	// (the Note is emitted, RememberRevealed$ fires) and "no" means they
	// declined (no Note, Remembered unchanged). "" on the first pass, where
	// the effect poses the ask (or, when the host cannot ask, falls back to
	// the mandatory reveal — the same R-9 degradation Scry/Surveil carry).
	// effReveal consumes and clears it before continuing, so a nested
	// RevealOptional$ peek in the same walk poses its own ask (fx42
	// scoping).
	RevealOpt string
	// ChosenType is the answered mid-resolution ChooseType pick (task ct1):
	// the creature type the chooser picked out of the TypeChoices list, set
	// by rules' "choosetype" resume arm before the suspended sub-ability is
	// re-run. effChooseType's re-entry emits the one Choose event the
	// fallback would have emitted, with the answered type instead, so the
	// downstream Card.ChosenType readers see exactly the shape they already
	// read. A valid answer is never empty (the option list's last resort is
	// "Human"), so non-empty IS the answered marker, and the asking effect
	// consumes and clears it at the top of its walk (the fx42 scoping
	// discipline), so a nested ChooseType cannot inherit the outer answer.
	ChosenType string
	// LookAck is the answered bare-look "Continue" ack (lookack, task
	// fb-20260917T232325Z-35cfca4b): the looker acknowledged the private
	// look a NoReveal$ / mandatory-Look$ Reveal-family effect is about to
	// record, so the Secret Note lands below the modal instead of streaming
	// past ungated. There is no decline — the ask paces the look, it does
	// not permit it — so the resume arm sets it on ANY answer, together with
	// LookAckTarget: the decision's ResumeTarget, the index of the Defined$
	// target whose ack was answered. effReveal consumes and clears BOTH at
	// the top of its own walk (fx42 scoping): targets before LookAckTarget
	// were fully processed on the pass that suspended and are skipped,
	// LookAckTarget itself emits without re-asking, and every LATER bare
	// look in the walk poses its own ack — the per-target cursor (the
	// DigTarget pattern) is what keeps a multi-target bare look (Case the
	// Joint's "look at the top card of each player's library", Defined$
	// Player) terminating with exactly one Continue per target instead of
	// re-asking the earlier targets' notes unboundedly.
	LookAck bool
	// LookAckTarget is the Defined$ target index whose look_ack was answered
	// (the decision's ResumeTarget). Meaningful only while LookAck is set;
	// consumed and cleared with it.
	LookAckTarget int
	// DrawOpt is the answered OptionalDecider$ yes/no on a re-entered
	// mid-resolution Draw (Mystic Remora, Rhystic Study): "yes" draws and
	// "no" declines, the same two-way answer the RevealOpt ask poses. ""
	// on the first pass, where effDraw poses the ask (or, when the host
	// cannot ask, keeps the pre-ask mandatory draw — the R-9 degradation).
	// Consumed and cleared before the draw loop, so a nested optional draw
	// in the same walk poses its own ask (fx42 scoping).
	DrawOpt string
	// LastRoll/LastRollName carry the result of a DB$ RollDice this same
	// resolution just made (effects/dice.go), under the SVar name its
	// ResultSVar$ parameter named (usually "Result" or "X"). evalCountExpr's
	// SVar$ head resolves a body of the form "SVar$<name>" against them, so
	// a chained sub's own SVar body (Velukan Dragon's
	// "SVar:X:SVar$Result/Minus.1") and a ConditionCheckSVar$ can read the
	// roll. Zero/"" on any resolution that did not roll, and the values are
	// never persisted -- a roll that suspends and resumes loses them, the
	// same per-resolution lifetime every other Ctx field has. RollPubs is
	// the general form of the same publication (both are read through
	// effects.dice.go's rollPublished, and this slot stays the primary
	// result's mirror for the existing readers).
	LastRoll     int32
	LastRollName string
	// RollPub is one name→value publication a DB$ RollDice of this
	// resolution made, beyond the primary ResultSVar$ slot above:
	// ChosenSVar$/OtherSVar$ (the Endeavor cycle's choose-one-result), and
	// the MaxRollsResults$/EvenOddResults$ counts ("MaxRolls",
	// "EvenResults", "OddResults" -- Luck Bobblehead). Read by Name's
	// bare-name fallback and evalCountExpr's SVar$ head through
	// rollPublished, and by Ctx.SpecContext's numeric-RHS resolver, so a
	// chained sub's filter spec (Valiant Endeavor's Creature.powerGEX,
	// Arcane Endeavor's Instant.cmcLEY) reads the roll too. Never persisted
	// across a suspension -- the chosen/other publications are rebuilt from
	// the answered decision on the roll resume (Ctx.RollResults/RollPick),
	// the same per-resolution lifetime as LastRoll.
	RollPubs []RollPub
	// RollResults/RollPick/RollDone carry the ANSWERED choose-one-result ask
	// on a re-entered mid-resolution RollDice (rules/resolution.go's "roll"
	// arm): RollResults is the per-die results the asking first pass rolled
	// (carried verbatim on the decision and the resume point), RollPick the
	// dice the player picked (each entry a roll Option's Index), RollDone
	// the answered marker. The re-entered effRollDice publishes
	// ChosenSVar$ = the sum of the picked dice's results and OtherSVar$ =
	// the sum of the rest, then lets Resolve chain the SubAbility$; it
	// consumes and clears all three at its top (the fx42 scoping
	// discipline), so a nested RollDice below this walk poses its own ask.
	RollResults []int32
	RollPick    []int
	RollDone    bool
}

// RollPub is one name→value publication (see Ctx.RollPubs).
type RollPub struct {
	Name  string
	Value int32
}

type Effect func(h Host, c *Ctx, sa *cards.SA)

// atomicMap is a copy-on-write string-keyed map. Writes are rare — native
// primitives register themselves from init() and the only other writer is
// M3's plugin tier overriding one at runtime — so they pay the cost of taking
// a mutex and copying the snapshot. Reads are the hot path: Resolve does one
// lookup per effect resolution, on every match, in its own goroutine, so
// readers do a single atomic load and never block or contend with a writer or
// each other. A writer never mutates a map a reader might already hold: it
// always builds a fresh map and swaps the pointer.
type atomicMap[V any] struct {
	mu  sync.Mutex
	ptr atomic.Pointer[map[string]V]
}

func newAtomicMap[V any]() *atomicMap[V] {
	a := &atomicMap[V]{}
	m := map[string]V{}
	a.ptr.Store(&m)
	return a
}

func (a *atomicMap[V]) load() map[string]V { return *a.ptr.Load() }

// set installs or replaces one entry.
func (a *atomicMap[V]) set(key string, val V) {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := *a.ptr.Load()
	next := make(map[string]V, len(old)+1)
	for k, v := range old {
		next[k] = v
	}
	next[key] = val
	a.ptr.Store(&next)
}

// setAll installs or replaces several entries as a single atomic publish.
func (a *atomicMap[V]) setAll(kv map[string]V) {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := *a.ptr.Load()
	next := make(map[string]V, len(old)+len(kv))
	for k, v := range old {
		next[k] = v
	}
	for k, v := range kv {
		next[k] = v
	}
	a.ptr.Store(&next)
}

// delete removes the given keys, if present.
func (a *atomicMap[V]) delete(keys ...string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := *a.ptr.Load()
	next := make(map[string]V, len(old))
	for k, v := range old {
		next[k] = v
	}
	for _, k := range keys {
		delete(next, k)
	}
	a.ptr.Store(&next)
}

type effectRegistrySnapshot struct {
	byName map[string]Effect
	byCode []Effect
}

type effectRegistry struct {
	mu  sync.Mutex
	ptr atomic.Pointer[effectRegistrySnapshot]
}

func newEffectRegistry() *effectRegistry {
	r := &effectRegistry{}
	r.ptr.Store(&effectRegistrySnapshot{
		byName: map[string]Effect{},
		byCode: make([]Effect, int(cards.APICodeCount)),
	})
	return r
}

func (r *effectRegistry) load() *effectRegistrySnapshot { return r.ptr.Load() }

func (r *effectRegistry) set(name string, effect Effect) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.ptr.Load()
	next := &effectRegistrySnapshot{
		byName: make(map[string]Effect, len(old.byName)+1),
		byCode: append([]Effect(nil), old.byCode...),
	}
	for key, registered := range old.byName {
		next.byName[key] = registered
	}
	next.byName[name] = effect
	if code := cards.APICodeForName(name); code != cards.APIUnknown {
		next.byCode[int(code)] = effect
	}
	r.ptr.Store(next)
}

func (r *effectRegistry) delete(names ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.ptr.Load()
	next := &effectRegistrySnapshot{
		byName: make(map[string]Effect, len(old.byName)),
		byCode: append([]Effect(nil), old.byCode...),
	}
	for key, registered := range old.byName {
		next.byName[key] = registered
	}
	for _, name := range names {
		delete(next.byName, name)
		if code := cards.APICodeForName(name); code != cards.APIUnknown {
			next.byCode[int(code)] = nil
		}
	}
	r.ptr.Store(next)
}

var registry = newEffectRegistry()

// Register installs an implementation for a Forge API name. Called from init
// functions in this package; re-registering replaces, which is what lets the
// plugin tier in M3 override a native primitive. Safe to call concurrently
// with Resolve and Supported (and with itself).
func Register(api string, fn Effect) { registry.set(api, fn) }

func unregister(apis ...string) { registry.delete(apis...) }

// Supported reports the primitive set this build implements, in the same
// prefixed form cards.Face.Primitives uses, so it feeds straight into
// cards.Registry.Coverage.
func Supported() map[string]bool {
	reg := registry.load()
	non := supportedNonAPI.load()
	out := make(map[string]bool, len(reg.byName)+len(non))
	for k := range reg.byName {
		out["api:"+k] = true
	}
	for k := range non {
		out[k] = true
	}
	return out
}

// supportedNonAPI holds keyword, trigger, static and replacement primitives,
// which are implemented in rules rather than as effect functions. Tasks 18-20
// fill it in.
var supportedNonAPI = newAtomicMap[bool]()

// RegisterNonAPI records a keyword, trigger, static or replacement primitive as
// implemented. The name must carry its prefix, e.g. "kw:Flying". Safe to call
// concurrently with Resolve and Supported (and with itself).
func RegisterNonAPI(prefixed ...string) {
	kv := make(map[string]bool, len(prefixed))
	for _, p := range prefixed {
		kv[p] = true
	}
	supportedNonAPI.setAll(kv)
}

const maxChain = 32

// Resolve runs an ability and every sub-ability chained beneath it.
func Resolve(h Host, c *Ctx, sa *cards.SA) {
	if c != nil {
		c.Host = h
		c.numericRHS = c.X != 0 || len(c.SVars) > 0
	}
	reg := registry.load()
	for d := 0; sa != nil && d < maxChain; d, sa = d+1, sa.Sub {
		// Condition* gate (task fb-3f1cc033): a sub whose supported condition
		// is evaluated and not met is skipped and the chain continues — the
		// per-SA read the corpus's own gated pairs rely on (Gruesome
		// Discovery's morbid pair: the outer gated EQ0, the inner — its
		// SubAbility — gated bare-Morbid; the "instead" branch only runs
		// because the walk continues past a denial). A chain payload that
		// must not run after its gated parent is kept out by its own
		// population: the DigUntil's DB$ Play reads only what the chain
		// remembered (effPlay's trigger-capture exclusion), never the
		// triggering event's capture. An unresolved shape (supported=false)
		// runs unconditionally, the documented pre-gate behaviour — see
		// conditions.go for the exact boundary and the counts behind it. A
		// RepeatEach re-entered at its loop cursor already passed its gate
		// when the loop began; its remaining iterations are part of that
		// same resolution.
		resumingLoop := c.Repeat != nil && c.Repeat.SA == sa
		if !resumingLoop {
			if met, supported := conditionMet(h, c, sa); supported && !met {
				continue
			}
		}
		var fn Effect
		if code := sa.CompiledAPI(); code != cards.APIUnknown && int(code) < len(reg.byCode) {
			fn = reg.byCode[int(code)]
		}
		if fn == nil {
			fn = reg.byName[sa.API]
		}
		if fn == nil {
			// Unimplemented primitives must be loud but harmless: deck-build
			// validation is supposed to have caught this already.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented API " + sa.API})
			continue
		}
		// UnlessCost$ gate: every API with an UnlessCost$ pays (or declines)
		// before its body runs. This is the one shared unless-cost path —
		// the gate poses the pay decision, rules' resume arm charges the
		// cost, and the re-entry applies the orientation. UnlessResolveSubs$
		// (Forge's AbilityUtils.handleUnlessCost) then gates the SubAbility$
		// walk on the pay outcome: absent/'Always' resolves the subs either
		// way, WhenPaid only when the cost was paid, WhenNotPaid only when it
		// was not. A gate that skips BOTH the body and the subs ends this
		// SA's chain entirely — Forge returns from handleUnlessCost without
		// resolveSubAbilities, so the enclosing chain stops here too.
		runBody, paid := true, false
		// Suspended() is widened by the cumulative-upkeep/triggered-cost
		// payment windows (rules' Suspended() counts e.cumulative and
		// e.triggerCost): a mana ability resolving INSIDE one of those windows
		// must still dispatch, so only a suspension the gate itself caused —
		// the ask poseUnlessAsk posed — stops the loop here. Compare against
		// the pre-gate state instead of the raw predicate.
		wasSuspended := h.Suspended()
		if strings.TrimSpace(sa.Params["UnlessCost"]) != "" {
			runBody, paid = unlessProceed(h, c, sa)
		}
		if !wasSuspended && h.Suspended() {
			// The gate posed the unless-pay ask and suspended the
			// resolution: stop here exactly as an asking effect body
			// would. The resume re-enters THIS SA (the ask's ResumeSA),
			// where the gate consumes the answer and the loop walks
			// sa.Sub — so this loop's own continuation is dropped, like
			// any asking loop's (SuspendContinuation's innermost rule).
			h.SuspendContinuation(sa)
			return
		}
		if !runBody {
			// The body is skipped (paid on an unswitched shape, or every
			// payer declined on a switched one). The Sub chain walks only
			// when UnlessResolveSubs$ says so for this pay outcome.
			if !unlessSubsRun(sa, paid) {
				return
			}
			continue
		}
		// The generic ValidTgts$ pre-ask (task mvts1): an SA the placement/
		// announcement ask never covered -- a sub at depth >= 2 of a trigger's
		// Execute chain (the "when you do" family: Mogg Bombers' DealDamage,
		// Kor Outfitter's Attach, Rhino's second PutCounter) -- poses its own
		// target ask here, before its body reads Defined's ValidTgts$
		// fallthrough. The SA the placement ask covered (Ctx.OfferedSA) is
		// skipped; an ANSWERED ask re-enters this same SA (the pending
		// frame's ResumeSA), so the consumption inside chosenTargetsFor runs
		// before any skip could suppress it. API$ ChangeZone is left to
		// effChangeZone's own mid-resolution ask (changeZoneChosenTargets),
		// which the closed ChangeZone slice owns.
		if ts, done := chosenTargetsFor(h, c, sa, d == 0); done {
			if ts == nil {
				// The ask was posed and suspended the resolution: stop here
				// exactly as an asking body would. The ask's ResumeSA is THIS
				// SA, so the pending frame re-enters it (the innermost rule),
				// the "tgts" arm fills Ctx.TargetsPick, and the re-entered
				// pass consumes the answer and dispatches with it visible to
				// Defined for this SA.
				h.SuspendContinuation(sa)
				return
			}
			c.PickedTargets = ts
			fn(h, c, sa)
			c.PickedTargets = nil
		} else {
			fn(h, c, sa)
		}
		imprint(h, c, sa)
		if strings.EqualFold(sa.Params["ClearImprinted"], "True") && c.Source != 0 {
			h.Emit(events.Event{Kind: events.Imprint, Obj: c.Source, Text: "clear"})
		}
		if h.Suspended() {
			// A sub-ability in this chain posed a mid-resolution ask and
			// suspended the resolution: do NOT descend into the rest of the
			// chain. The B1 bug was that this loop kept walking sa.Sub
			// unconditionally, so a chained SA ran its SubAbility$ on the
			// initial pass (before the answer existed) AND again when the
			// answered decision re-entered at the asking SA — Thoughtseize's
			// Discard | SubAbility$ DBLoseLife lost 4 life instead of 2. The
			// resume re-enters at THIS asking SA (rules' resumeResolution),
			// which re-runs the asking effect to apply the answer and then
			// continues walking sa.Sub exactly once.
			// Report this loop's suspension point to the host so a NESTED ask
			// (an ask posed from inside this loop's own effect, e.g. the mode
			// a Charm runs) does not lose the chain this loop was still
			// carrying — fx32's defect. The host keeps the enclosing levels as
			// outer continuations and drops this one when it is the asking
			// loop's own level, which re-enters sa.Sub itself.
			h.SuspendContinuation(sa)
			// The gate had already resolved when the body asked: record the
			// outcome so the answer's re-entry pass consumes it instead of
			// re-posing the pay ask (the asking-body-under-UnlessCost$
			// livelock — Rhystic Study's pay-or-draw was the live carrier).
			if strings.TrimSpace(sa.Params["UnlessCost"]) != "" {
				h.SuspendUnless(sa, paid)
			}
			return
		}
		// UnlessResolveSubs$ also gates the sub walk when the body RAN: Forge
		// resolves the subs iff (paid && WhenPaid-or-default) or
		// (!paid && WhenNotPaid-or-default), independent of the orientation —
		// a paid unswitched body both runs AND suppresses a WhenNotPaid chain.
		if strings.TrimSpace(sa.Params["UnlessCost"]) != "" && !unlessSubsRun(sa, paid) {
			return
		}
	}
}
