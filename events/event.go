// Package events defines every way state can change and is the only package
// permitted to mutate state.Game. The log is therefore a complete description
// of a match, and replay is state reconstruction rather than re-simulation.
package events

import (
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/state"
)

type Kind uint8

const (
	GameStart Kind = iota
	Shuffle
	MoveZone
	Draw
	LifeChange
	Damage
	Tap
	Untap
	StepChange
	TurnChange
	Priority
	PutOnStack
	Resolve
	ManaAdd
	ManaClear
	CounterChange
	DeclareAttackers
	DeclareBlockers
	PlayerLost
	GameOver
	DecisionAsk
	DecisionMade
	Note
	LandPlayed
	// TargetsChosen carries a spell or ability's chosen targets. Appended here
	// (Ruling T14-b) rather than inserted near PutOnStack/Resolve so every
	// earlier kind's numeric value, and therefore the hash chain and the
	// golden replays Tasks 9-13 already locked in, is unaffected.
	//
	// Amount is the discriminator between the two target shapes: 0 means
	// object targets (read from IDs), 1 means a player target (read from
	// Player). An empty IDs with a zero Player would otherwise be
	// indistinguishable from "player 0 was targeted" -- PlayerID 0 is a real
	// seat as well as the zero value.
	TargetsChosen
	// FlipFace changes an object's active face. Task 18's SetState primitive
	// is the only source of this event. Appended here rather than inserted
	// nearer Tap/Untap (Ruling T18-a, following TargetsChosen's own T14-b
	// precedent) so every earlier Kind's numeric value -- and therefore the
	// hash chain and any golden replay already locked in -- is unaffected.
	//
	// Amount carries the destination FaceIdx directly, not a delta: SetState
	// resolves Mode$ (Transform/Flip/...) to a concrete index itself, so
	// Apply does not need to know the object's current face to act on it.
	FlipFace
	// ClockTick advances Game.Clock by one. Ruling T19-a: rules.AddContinuous
	// used to increment g.Clock with a direct field write to stamp a
	// zero-Timestamp ContinuousEffect, the same bug class Ruling T11-a fixed
	// for Passes/Priority -- Object.Timestamp is assigned from Clock whenever
	// a permanent enters the battlefield (see Move below), so a live game
	// that advances Clock outside Apply leaves every later Timestamp off by
	// one in a fresh reconstruction folded from the log alone. AddContinuous
	// now emits this and reads the result instead of writing Clock itself.
	// It carries no fields: the tick's only effect is the increment, so
	// nothing else needs to survive the encode/decode round trip. Appended
	// here (following TargetsChosen's T14-b and FlipFace's T18-a precedent)
	// so every earlier Kind's numeric value, the hash chain, and any golden
	// replay already locked in are unaffected.
	ClockTick
	// TriggerPush creates a triggered ability's stack object and places it
	// on the stack, in one event. Ruling T20-a: a live rules.Engine used to
	// mint this object with a direct, unlogged Game.AddObject call and then
	// emit a plain MoveZone naming its (already-assigned) ObjID -- but a log
	// replayed with no Engine behind it never learns that ID exists, so
	// events.Move's "if o == nil { return }" guard silently no-ops and the
	// replayed stack permanently diverges from the live one. This event
	// carries what Apply needs to recreate the object itself: Player is its
	// controller, Obj is the permanent whose trigger fired (Object.Source),
	// Amount is that permanent's Face().Triggers index (so Apply re-derives
	// the *cards.SA to run -- a raw pointer cannot be logged, only the data
	// that lets it be found again), and IDs is the triggering event's
	// Remembered object(s). No new Event field was added: Player/Obj/Amount/
	// IDs already exist and are reused, exactly as the brief's Ruling
	// requires, so the hash chain and every earlier golden replay are
	// unaffected by this Kind's own shape -- only its ordinal, appended here
	// after ClockTick, is new.
	TriggerPush
	// EndCombatReset clears every object's IsAttacking/BlockedBy fields
	// when Obj is zero. Nonzero Obj removes only that permanent from combat,
	// retaining zero blocker-list tombstones so attackers stay blocked.
	// Ruling T21-e (Task 21 fix round 1): rules.setStep used to do this with
	// a direct loop over e.G.Objs when entering StepEndCombat or
	// StepCleanup, instead of emitting anything -- a violation of "all state
	// mutation goes through events.Apply" that was structurally present
	// before Task 21 but stayed inert (IsAttacking/BlockedBy were never
	// actually set to a non-zero value by anything) until real combat made
	// it observable: a log-only reconstruction, having never been told
	// combat ended, kept a surviving attacker/blocker marked IsAttacking/
	// BlockedBy forever, while the live game correctly cleared both.
	// Carries no fields (same shape as ClockTick): the reset, applied to
	// every object in the arena, is its whole effect. Appended here,
	// following TriggerPush's own precedent, so every earlier Kind's
	// ordinal, the hash chain, and any golden replay already locked in are
	// unaffected.
	EndCombatReset
	// CastInfo records how a spell was cast: Amount is the value chosen for
	// {X} (state.Object.X), and Counter is a comma-separated list of flag
	// names (FlagsFrom/FlagsString) folded into state.Object.CastFlags. Task
	// 4. Since the paid-{X} Ctx binding (CR 107.3i), commitCast's ability arm
	// also emits it on the AbilityPush-minted stack object to record the {X}
	// an activated ability's Cost$ was paid with -- Obj is that ability
	// object, never the source permanent -- so Amount's meaning is "the {X}
	// value paid for this cast or activation". Appended here, after
	// EndCombatReset, following every prior Kind's
	// own append-only precedent, so no earlier ordinal, hash chain or golden
	// replay is affected.
	CastInfo
	// Choose records an answer to an "as this enters/resolves, choose ..."
	// effect. Counter discriminates the shape ("name", "type", "number");
	// Text carries a chosen name or creature type, Amount a chosen number.
	// An unrecognized Counter is a no-op, not an error, the same totality
	// stance as every other case in Apply's switch.
	Choose
	// TokenCreate mints a token onto the battlefield. Player is its owner
	// and controller; Text is the key into Game.Tokens (a Forge token
	// script stem, e.g. "r_1_1_goblin") -- token definitions live on the
	// game, set at genesis, never in the event itself, so Apply stays a
	// pure function of (g, e) with no card data smuggled through Text.
	TokenCreate
	// StackCopy duplicates the spell or ability named by Obj (which must
	// currently be on the stack) and places the copy on top of the stack.
	// Player is the copy's controller (CR 707.10a: a copy's controller is
	// whoever the copy effect says, not necessarily the original's).
	StackCopy
	// Attach records or clears what Aura/Equipment permanent Obj is
	// attached to. IDs holds zero or one entries: empty detaches (sets
	// AttachedTo to 0), one attaches to that id.
	Attach
	// AbilityPush creates an activated ability's stack object and places it
	// on the stack, in one event -- the same shape TriggerPush already
	// uses for triggered abilities, for the same reason (Ruling T20-a): a
	// log-only replay must be able to recreate the object itself, not rely
	// on a live Engine's direct, unlogged AddObject call. Player is the
	// ability's controller, Obj is the source permanent, Amount is that
	// permanent's Face().Abilities index, and IDs is whatever the
	// activation remembered (mirrors TriggerPush's own IDs usage).
	AbilityPush
	// ModeChosen records a KModes answer: spell announcement (CR 601.2b),
	// triggered-ability placement (CR 603.3c), or a mid-resolution mode or
	// unless-pay choice. Obj is the stack object, Player the chooser, and Text
	// the chosen option labels (csv, in execution order). It is a marker like
	// Note: the engine caches placement/announcement names or carries a
	// resolution answer in its continuation, while Apply remains inert. The
	// log lets replay re-derive the same branch. Appended
	// here, after AbilityPush, following every prior Kind's own append-only
	// precedent, so no earlier ordinal, hash chain or golden replay is
	// affected (M2d-2).
	ModeChosen
	// CmdDamage records a commander dealing combat damage to a player (CR
	// 903.10, Task m33). Player is the damaged player, Obj the source
	// commander (whose Object ID is stable across zone moves), and Amount
	// the damage that actually landed (prevented/replaced damage is never
	// emitted). events.Apply folds it into that player's cumulative
	// commander-damage tally (state.Player.CmdDamage), keyed by the source
	// commander's match-wide dense index derived from g.Players[].Commanders.
	// Carried as a dedicated event rather than derived from the existing
	// Damage events because those do not record which commander the source
	// was -- this event is what lets a reconstruction starting from the log
	// alone rebuild the per-commander tally. Appended after ModeChosen,
	// following every prior Kind's append-only precedent, so no earlier
	// ordinal, hash chain or golden replay is affected. Only ever emitted in
	// a Commander-format game.
	CmdDamage
	// DelayedRegister records a delayed-trigger registration (CR 603.7,
	// effects.effDelayedTrigger's Mode$ Phase branch). A delayed trigger is
	// registered during one resolution and fires later -- in general a
	// different turn -- so the registration is game state, folded here into
	// state.Game.Delayed so a log-only replay rebuilds the same set. Event
	// field reuse: Obj is the source object (whose face's SVar table carries
	// the Execute$ ability), Player the controller, Step the phase to fire
	// in, Counter the Execute$ SVar name, IDs the Remembered object(s)
	// captured at registration (PlayerRef-encoded, as TriggerPush does), and
	// Text the Forge Phase$ string for description -- or, for an event-matched
	// (non-phase) registration, "<Mode$ value>:<trigger SVar name>", the
	// encoding events.Apply's DelayedRegister case decodes into
	// state.DelayedTrigger's EventMode/Trigger pair. Appended here, after
	// CmdDamage, following every prior Kind's own append-only precedent, so
	// no earlier ordinal, hash chain or golden replay is affected.
	DelayedRegister
	// DelayedPush mints a delayed triggered ability's stack object and fires
	// it -- the "goes on the stack like any other triggered ability" half of
	// a Mode$ Phase delayed trigger, created when the registered phase is
	// entered. It is a sibling of TriggerPush/AbilityPush and exists for the
	// same reason (Ruling T20-a): the ability object is minted inside Apply,
	// so a log-only replay creates the exact object a live game did. Unlike
	// TriggerPush (whose Ability re-derives from a face Triggers index), a
	// delayed trigger's Ability is an SVar-named sub-ability on the source's
	// face, so the Execute$ name is carried in Counter and Apply resolves it
	// via cards.ResolveSVar. Obj is the source, Player the controller,
	// Amount the registration's ID (state.DelayedTrigger.ID) so Apply can
	// remove exactly the registration it fired, and IDs the Remembered
	// objects (PlayerRef-encoded). Firing is one-shot: Apply's case both
	// mints the object and drops the registration. Appended after
	// DelayedRegister, following every prior Kind's append-only precedent.
	DelayedPush
	// LibraryOrder records a player having set a complete new order on
	// their library -- the state change behind a library-arranging effect
	// (RearrangeTopOfLibrary, Ponder; later Scry/Surveil/Dig) and the
	// discard half of a bottoming ask. Player names whose library, IDs the
	// COMPLETE new order (the top cards reordered, then the untouched
	// remainder beneath them). Secret is always true: the exact order of a
	// hidden zone must not leak to another seat, so view redaction drops the
	// payload for anyone but Player.
	//
	// Appended here, after DelayedPush, following the named precedents of
	// TargetsChosen (Ruling T14-b), FlipFace (T18-a) and ClockTick (T19-a)
	// rather than inserted anywhere earlier, so every earlier Kind's numeric
	// value, and therefore the hash chain and every golden replay already
	// locked in, is unaffected.
	LibraryOrder
	// ExtraTurn records one grant or consumption of an extra turn (CR
	// 500.7). Player is the seat the turn belongs to and Amount the delta:
	// an api:AddTurn effect emits +NumTurns$, and the turn structure (rules
	// advanceStep) emits -1 exactly when it hands that seat a repeat turn
	// instead of moving to the next living seat. The state
	// (state.Game.ExtraTurns) is folded here, so a log-only reconstruction
	// arrives at the same pending counts the live game held. When the
	// granting ability carries Forge's ExtraTurnDelayedTrigger$ (Final
	// Fortune's "At the beginning of that turn's end step, you lose the
	// game"), Counter names the Execute$ SVar and Obj the granting source,
	// and this case ALSO registers the delayed trigger for the extra turn's
	// end step: a delayed registration whose MinTurn is the extra turn's
	// number (g.Turn+1 at grant time), so the ordinary delayed-trigger
	// firing skips the CURRENT turn's end step and fires exactly once, in
	// the granted turn. Appended here, after LibraryOrder, following every
	// prior Kind's append-only precedent, so no earlier ordinal, hash chain
	// or golden replay is affected.
	ExtraTurn
	// DoorUnlock records the unlock of an Enchantment Room's locked door
	// (CR 309.5): the player paid the locked half's mana cost as a sorcery.
	// Obj is the room permanent; Apply sets its Unlocked flag, which makes
	// the alternate face's rules text live (rules' trigger/static/ability
	// scans) and is what a Mode$ UnlockDoor trigger matches against. The
	// unlock trigger itself is queued rules-side on this event (rules
	// checkTriggers), so this event is the whole state delta. Appended
	// after ExtraTurn, same append-only precedent.
	DoorUnlock
	// SpeedChange records one increment of a seat's speed (CR 702.163,
	// "Start your engines!"). Player is the seat whose speed rises and
	// Amount the delta (always +1 today; the engine caps the grant at max
	// speed 4 and at once per turn before ever emitting). Folded into
	// state.Player.Speed here, so a reconstruction rebuilds it. Appended
	// after DoorUnlock, same append-only precedent.
	SpeedChange
	// MonarchChange gives the designation to Player. It is a state transition,
	// not a Note, so conditional "if you're the monarch" triggers replay from
	// the same state as the live match. Appended after LibraryOrder to preserve
	// every prior event ordinal.
	MonarchChange
	// ControlChange transfers control of a permanent or a stack object. Obj is
	// the controlled object and Player its new controller. It is deliberately a
	// distinct event: control is neither ownership nor a zone change, and a
	// replay must retain it when the object later moves.
	ControlChange
	// CardToken mints a battlefield token that is a copy of the card object Obj
	// names. Appending after main's existing events preserves their ordinals.
	CardToken
	// KeywordTriggerPush mints a mandatory keyword-provided triggered ability.
	KeywordTriggerPush
	// Goad applies CR 701.38's attack requirement to Obj. Player is the
	// goading player, Text its Forge duration, IDs[0] its source, and Amount
	// the target's controller plus one. Amount -1 removes every relationship.
	Goad
	// PlayerCounterChange changes a counter on Player. Counter names the kind
	// and Amount the delta; Ward's AddCounterYou<.../POISON> is its first use.
	PlayerCounterChange
	// Imprint updates one source-card association. Obj is the source and IDs
	// are the cards to add: ordinary text is Forge's imprintedCards list,
	// Text "exiled-with" is its distinct exiledCards list, Text
	// "until-host-leaves" is ChangeZone's Duration$ UntilHostLeavesPlay
	// association (Amount carries the zone the cards were exiled from, which
	// their return moves them back to; rules sweepExileReturn consumes it),
	// and Text "clear" clears only imprintedCards. Reusing Text avoids
	// changing Event's layout.
	// It is append-only so prior event ordinals and replay hashes stay stable.
	Imprint
	// StartingPlayerChange records CR 103.1's starting-player designation.
	// It is emitted by genesis's toss resolution (folded without appending at
	// genesis, since genesis is replayed from Config) and by an opening-hand
	// effect such as Impatient Iguana, rather than being inferred from
	// TurnChange, because Count$StartingPlayer must read its result before
	// turn one. Appended after main's kinds so the merge preserves main's
	// ordinals (log.json serializes kind numerically; a committed fixture's
	// replay pins them).
	StartingPlayerChange
	// Pair records a Soulbond pairing (CR 702.103): Obj is the pairing
	// permanent and IDs[0] is its chosen partner. Appended after
	// StartingPlayerChange (the merge kept main's kinds at their main
	// ordinals), following the same append-only precedent, so all earlier
	// Kinds keep their numeric values and the hash chain and golden replays
	// are unaffected.
	Pair
	// MyriadCopy records one Myriad (CR 702.109) attacker token: a copy of
	// the attack-creature Obj that enters tapped and attacking the opponent
	// named by Player. Appended after Pair, same append-only precedent.
	MyriadCopy
	// MyriadCleanup exiles every Myriad token still on the battlefield as the
	// end-of-combat step ends (CR 702.109a). Its event-sourced arena scan keeps
	// live play and log replay in lockstep without adding one delayed trigger
	// per token.
	MyriadCleanup
	// GrantTriggerPush mints a static-grant's triggered ability (AddTrigger$
	// on a Mode$ Continuous static, e.g. Hearthhull's "STATION 8+ Whenever you
	// sacrifice a land") and places it on the stack, in one event. It mirrors
	// DelayedPush's shape -- the Ability is not a face Triggers index but the
	// granted trigger's Execute$ SVar-named body -- minus the registration
	// bookkeeping: a granted trigger is consumed by nothing and lives exactly
	// as long as its granting static. The Execute$ body lives on the GRANTOR's
	// face (the card carrying the static), while Obj is the AFFECTED
	// recipient; for a cross-object grant the grantor's object id rides
	// Amount, and Apply resolves the name from the grantor's SVar table when
	// it is set (0 = the historical self-grant shape, resolved from the
	// affected object's own table). Rules' queue gate links the body from the
	// same table, so a replay reproduces the same stack object. Appended after
	// MyriadCleanup, following every prior Kind's append-only precedent, so no
	// earlier ordinal, hash chain or golden replay is affected.
	GrantTriggerPush
	// ManaActivate records one activation of an AB$ Mana ability. It is a
	// MARKER like Note: effMana's own ManaAdd events carry the mana that
	// landed but never attribute it to a source object (Event.Obj is unset
	// on them), and populating it would re-encode every existing ManaAdd
	// event and move every golden replay -- so the marker, emitted only by
	// resolveManaAbility for abilities that actually carry an
	// ActivationLimit$ parameter (Vivi Ornitier's "only once each turn"),
	// is what lets rules' activationLimitReached event-log scan count mana
	// activations by source and ability index, the same way it already
	// counts AbilityPush for non-mana abilities. Obj is the source permanent,
	// Player the activator, Amount the ability's index in the face's
	// Abilities slice. Appended here, after GrantTriggerPush, following every
	// prior Kind's own append-only precedent, so no earlier ordinal, hash
	// chain or golden replay is affected.
	ManaActivate
	// TokenAttacks marks one token that entered the battlefield TAPPED AND
	// ATTACKING (Mobilize, Kari Zev's "tapped and attacking" monkey -- the
	// TokenAttacking$ True rider). It is NOT a mint: events.Apply's
	// TokenCreate case already made the object, and Obj here is that
	// already-existing battlefield token, Player its controller and IDs[0]
	// the defender it attacks. MyriadCopy must not be reused for this: it
	// mints a copy of the SOURCE card and flags IsMyriad, which
	// MyriadCleanup exiles at end of combat -- wrong semantics for a script
	// token a Sacrifice at the next end step owns. Appended here, after
	// ManaActivate, following every prior Kind's own append-only precedent,
	// so no earlier ordinal, hash chain or golden replay is affected.
	TokenAttacks
	// XChange records a mid-resolution effect rewriting the {X} a stack
	// object was cast or activated with (Unbound Flourishing's "double the
	// value of X", Glava's "the value of X becomes 5" -- DB$ ChangeX). Obj
	// is the stack object whose {X} was rewritten, Amount the new value. A
	// plain CastInfo could not carry this: its Apply case resets CastFlags
	// from Counter unconditionally (wiping Kicked/Flashback on a flagged X
	// spell) and would shadow adventure's first-CastInfo backward log scan,
	// and neither FlagConverged nor FlagReplicated may alias a real X
	// value. Appended here, after TokenAttacks, following every prior
	// Kind's own append-only precedent, so no earlier ordinal, hash chain
	// or golden replay is affected.
	XChange
	// NoteNumber records a number a trigger's Execute$ body NOTED onto a
	// card (DB$ Pump | NoteNumber$ <expr> -- Lupine Harbingers' exile
	// trigger noting Count$YourTurns, the corpus's one NoteNumber$
	// carrier): Obj is the card, Amount the noted value, and Apply folds it
	// into Object.NotedNumber for Count$NotedNumber to read at the later
	// ETB. A plain Note could not carry this: it is transcript text with no
	// numeric payload and no Apply behaviour, and the value must be
	// event-backed so a replay derives the identical count. Appended here,
	// after XChange, following every prior Kind's own append-only
	// precedent, so no earlier ordinal, hash chain or golden replay is
	// affected.
	NoteNumber
	// ExtraPhase records one Forge AddPhaseEffect message (DB$ AddPhase:
	// "after this phase, there is an additional combat phase"; 56 corpus SA
	// lines). Three forms, split on Amount, mirroring the ExtraTurn
	// precedent one level up: +1 is a grant (Step the splice point
	// AfterStep -- the phase after which the extra phase is inserted, either
	// the parsed AfterPhase$ or, when omitted, the phase the grant resolved
	// in so the fold agrees with the live splice; IDs[0] the extra phase's
	// entry step, IDs[1] an explicit FollowedBy$ resume point when present;
	// Counter the forwarded Execute$ SVar name; Text the ExtraPhaseRiders
	// marker when the granting SA carries ExtraPhaseDelayedTrigger$); -1
	// CONSUMES one grant at the turn boundary it splices at -- the queue
	// entry is marked consumed and, when it carries the delayed rider, the
	// one-shot delayed trigger registers HERE (MinTurn = the current turn:
	// the extra phase begins in this turn, unlike an extra turn's Turn+1);
	// -2 COMPLETES one consumed grant when the walk leaves the extra
	// phase's last step. The fold lives in state.Game.ExtraPhases (cleared
	// at TurnChange), and the consumer is rules/turn.go's advanceStep tail.
	// Appended after NoteNumber (main's own later append), still after every
	// earlier Kind, so no earlier ordinal, hash chain or golden replay is
	// affected.
	ExtraPhase
	// CopyToken mints a battlefield token that is a copy of the CARD object
	// Obj names (DB$ CopyPermanent: Flamerush Rider, Molten Echoes, the
	// populate family -- task copyp1). Like MyriadCopy it only MINTS the
	// object, in the untracked ZLibrary state AddObject leaves it in; the
	// caller follows with a genuine MoveZone, so the copy's battlefield
	// entry stays a ChangesZone-matchable event every "a creature enters"
	// trigger observes (the CardToken shape folds its own move, which the
	// Myriad comment above records as entry-invisible). Player is the copy's
	// controller and Amount is the entry-state rider bitmask the
	// CopyToken* constants name; bit CopyTokenAttacking takes the defender
	// it attacks from IDs[0] (a player number, the MyriadCopy/TokenAttacks
	// precedent). Appended after ExtraPhase, still after every earlier
	// Kind, so no earlier ordinal, hash chain or golden replay is affected.
	CopyToken
	// Exert records CR 702.100's exert election (task exert1): Obj is the
	// permanent the controller exerted and Player is the controller at exert
	// time. Amount >= 0 is the exert itself; Amount == -1 is the
	// consumed-at-use marker rules/turn.go's untap-step scan emits when it
	// passes an exerted permanent -- CR 702.100b's "won't untap during your
	// next untap step" window closes there, so the fold clears the object's
	// skip flag. Appended here, after CopyToken, following every prior Kind's
	// own append-only precedent, so no earlier ordinal, hash chain or golden
	// replay is affected.
	Exert
	// PlanarRoll records one planar-dice roll (CR 901.3, task rollplanar1):
	// Player is the roller, Obj the rolling source, Amount the number of
	// dice rolled AFTER any planar-dice replacement rewrote the count, IDs
	// the KEPT results (1-4 blank, 5 planeswalk, 6 chaos) in roll order and
	// Counter the ignored-result count the replacement wrote ("" = none).
	// It is an Apply no-op marker: no plane deck exists in this build, so a
	// roll has no state to fold — the log line IS the record (one event per
	// roll action), and replay re-derives the same rolls from the seeded rng.
	// Appended here, after Exert, following every prior Kind's own
	// append-only precedent, so no earlier ordinal, hash chain or golden
	// replay is affected.
	PlanarRoll
	// Explore records one completed explore action (CR 701.35a, task
	// explore1): Obj is the exploring permanent, Player its controller
	// (whose library was explored), IDs[0] the card the process revealed,
	// and Amount the outcome -- 1 when the revealed card was a land and
	// went to its owner's hand, 0 when it was a nonland (the +1/+1 counter
	// went on the explorer and the card went back on top or into the
	// graveyard per the LCI wording the corpus spells out). It is an Apply
	// no-op marker, exactly like PlanarRoll: the explore's own state changes
	// are their own MoveZone/CounterChange events, and the record is what
	// trig:Explores matches and what makes the explore trigger- and
	// replay-visible. Appended here, after PlanarRoll, following every prior
	// Kind's own append-only precedent, so no earlier ordinal, hash chain or
	// golden replay is affected.
	Explore
	// CombatRetarget re-points an already-attacking creature at a new defender
	// mid-combat (api:ChangeCombatants's Attacking$ True shape -- Misleading
	// Signpost, Portal Mage, Windshaper Planetar): Obj is the attacker, Player
	// the NEW defender. It deliberately is NOT DeclareAttackers, whose Apply
	// case increments AttacksThisTurn and would refire every Attacks trigger --
	// a reselect changes no declaration, only which seat the existing attack
	// is pointed at (CR 506.3b: only within the attacker's controller's own
	// combat, which is why the same event also clears BlockedBy: Forge's
	// removeFromCombat + addAttacker leaves the old blockers behind, and the
	// re-pointed attack is unblocked -- the blocker lists of the OLD blockers
	// are attacker-side only, so clearing BlockedBy is the whole unlink).
	// Appended here, after Explore, following every prior Kind's own
	// append-only precedent, so no earlier ordinal, hash chain or golden
	// replay is affected.
	CombatRetarget
	// RingTemptsYou records one "the Ring tempts you" action (CR 701.54a:
	// each time the Ring tempts you, choose a creature you control; it
	// becomes your Ring-bearer). Player is the tempted seat, Obj the
	// designated Ring-bearer (0 when the player controls no creature — CR
	// 701.54d: the "Whenever the Ring tempts you" trigger still fires when
	// the actions complete even if some were impossible), and Amount the
	// NEW tempt count, carried as a replay-visible marker. Apply folds the
	// count increment and the designation; the designation's two clears (a
	// control change, CR 701.54b, and the permanent leaving the battlefield,
	// CR 400.7/701.54e) are derived in Apply's own ControlChange and
	// MoveZone cases, so no second event is needed. Appended here, after
	// CombatRetarget, following every prior Kind's own append-only
	// precedent, so no earlier ordinal, hash chain or golden replay is
	// affected.
	RingTemptsYou
	// RingEmblemPush mints one of the Ring emblem's four level abilities
	// (CR 701.54c), which are engine-side abilities with no corpus script
	// text and no object in any zone -- the temptation count folded by
	// RingTemptsYou is their only state (state.Player.RingTempted). Player
	// is the emblem's owner (the tempted seat), Amount the level (1..4) and
	// Counter the canonical "__ring:<level>" payload events.Apply rebuilds
	// the ability from, exactly as the granted ward/afflict
	// KeywordTriggerPush payloads are rebuilt ("the same DB$ ... a printed
	// trigger would have carried"). Obj carries the Ring-bearer the firing
	// event named (0 for a level whose body needs no bearer). The mint lives
	// in Apply because a direct unlogged Game.AddObject call would name an
	// ObjID a log-only replay never learns about (Ruling T20-a). Appended
	// here, after RingTemptsYou, following every prior Kind's own
	// append-only precedent, so no earlier ordinal, hash chain or golden
	// replay is affected.
	RingEmblemPush
	// GrantAbilityPush mints an activated ability GRANTED to one permanent
	// by another (CR 613.1f): a printed `S:Mode$ Continuous | Affected$
	// <spec> | AddAbility$ <SVar>` static (Ichormoon Gauntlet's "Planeswalkers
	// you control have [0]: Proliferate", a lord granting an activated
	// ability, an Equipment granting "{T}: deal 1 damage") whose grantor is
	// the static's source and whose recipient is the affected permanent.
	// Obj is the ability's own SOURCE -- the RECIPIENT, so the minted stack
	// object's Source (and every `Defined$ Self`/`CARDNAME` body that reads
	// it) is the affected permanent, not the granting static. Counter names
	// the SVar body on the GRANTOR's face; IDs[0] is the granting object id,
	// resolved from there by Apply. The DelayedPush precedent is why this is
	// a distinct Kind rather than an overload: DelayedPush resolves its body
	// from e.Obj and decodes e.IDs into the ability's Remembered set, which
	// is exactly wrong for a cross-object grant. Self-grants (Animate, the
	// max-speed static) still mint through DelayedPush, byte-identically.
	// Appended after RingEmblemPush (main's own later append), still after
	// every earlier Kind, so no earlier ordinal, hash chain or golden replay
	// is affected.
	GrantAbilityPush
	// Investigate records one completed investigate action (CR 701.36a, task
	// investtrig1): Player is the investigating seat and Obj the resolving
	// source permanent (0 for a source-less body). It is an Apply no-op
	// marker, exactly like Explore: the investigate's own state change (the
	// Clue token mint) is its own TokenCreate event that precedes this one,
	// and the record is what trig:Investigated matches ("whenever you
	// investigate" — Erdwal Illuminator, Val, Marooned Surveyor). The marker
	// is separate from the mint so a plain Clue-token creation (DB$ Token |
	// TokenScript$ c_a_clue_draw, no Investigate) never fires an investigate
	// trigger. Appended after GrantAbilityPush (the branch's own append, moved
	// one ordinal by the merge with main's GrantAbilityPush), still after
	// every earlier Kind, so no earlier ordinal, hash chain or golden replay
	// is affected.
	Investigate
	// BlessingChange records a seat GAINING the city's blessing (CR
	// 702.131d, task ascend1): Player is the seat. It is one-way -- Apply
	// sets the latch and nothing ever clears it (CR 702.131a: "for the
	// rest of the game") -- and the grant's continuous re-check lives in
	// the rules emitter (rules/ascend.go), so Apply folds the bit plainly.
	// Appended after Investigate, still after every earlier Kind, so no
	// earlier ordinal, hash chain or golden replay is affected.
	BlessingChange
	// ClonePermanent folds a DB$ Clone copy basis onto an existing permanent
	// (CR 613.1a's layer-1 copy): Obj is the object that BECOMES the copy,
	// IDs[0] is the object copied FROM, Text is the copy's NewName$ (empty
	// keeps the copied face's name), and Counter is "gain-this-ability" when
	// the GainThisAbility$ True rider applies. An event with no IDs (or a
	// zero id) CLEARS the copy -- the expiry and leave-the-battlefield path.
	// Appended after BlessingChange, still above NumKinds, so no earlier ordinal, hash
	// chain or golden replay is affected.
	ClonePermanent
	// Mutate records one mutate-spell resolution (CR 702.140): Obj is the
	// TARGET permanent that survives and becomes the mutated pile, IDs[0] is
	// the mutate card's object (the resolving spell), and Text is "top" when
	// the mutating card is placed on top or "under" when it is placed beneath
	// the target (CR 702.140b's choice). Apply folds the pile: the survivor's
	// Card/FaceIdx always describe the top card and every under-card lands in
	// its MergedCards (top-of-pile first), each parked in ZCeased, and
	// TimesMutated advances by Amount. It is the provenance both the
	// trig:Mutates fire and Count$TimesMutated read, so a replay rebuilds the
	// pile identically. Appended here, after ClonePermanent -- the last Kind
	// main holds -- following every prior Kind's own append-only precedent,
	// so no earlier ordinal, hash chain or golden replay is affected.
	Mutate
	// MergedTriggerPush mints a mutated pile's UNDER-CARD triggered ability
	// (CR 702.140d: the permanent has all abilities of the cards beneath
	// it, including their "whenever this creature mutates" triggers). Obj
	// is the pile (the triggering source), Player the controller, Counter
	// the Execute$ SVar name (kept as the log's readable provenance and as
	// a consistency check), Amount the PAIR (under-card pile index, that
	// face's own Triggers index) packed by MergedTriggerAmount, and IDs the
	// Remembered capture the ordinary trigger push encodes.
	//
	// Apply mints the ability from THAT face's own compiled trigger -- the
	// TriggerPush shape, f.Triggers[idx].Effect -- never from a by-name
	// SVar walk. Two reasons, both load-bearing:
	//   - the top face may define the same SVar name with a different body
	//     (Cubwarden and Everquill Phoenix both name their token SVar
	//     TrigToken), so a top-first by-name walk steals the under-card's
	//     body; and
	//   - cards.ResolveSVar parses a FRESH *SA on every call, so an ability
	//     minted that way has no pointer identity with the compiled
	//     cards.Trigger.Effect. Every consumer that recovers a resolving
	//     ability's owning trigger does so by that pointer
	//     (findTriggerForAbilityFace), so a fresh parse silently disabled
	//     the OptionalDecider$ gate, the intervening-if recheck, the
	//     ResolvedLimit$ count, the ability's label and the merged-face
	//     SVar table at resolution. Minting the compiled pointer is what
	//     makes an under-card trigger an ordinary trigger everywhere else.
	//
	// It is a sibling of DelayedPush/GrantTriggerPush -- the minting shape
	// is GrantTriggerPush's (no registration consumed) -- appended here,
	// after Mutate, following every prior Kind's own append-only precedent,
	// so no earlier ordinal, hash chain or golden replay is affected.
	MergedTriggerPush
	// Discover records one completed discover action (CR 701.57, task
	// trigdisc1): Player is the discovering seat and Obj the resolving source
	// permanent (0 for a source-less body). It is an Apply no-op marker,
	// exactly like Explore/Investigate: the discover's own state changes (the
	// exiles, the reveal Notes, the chosen card's move) are their own events
	// that precede this one, and the record is what trig:Discover matches
	// ("whenever you discover" -- Val, Marooned Surveyor, Curator of Sun's
	// Creation). The marker is ONE per completed discover action, never one
	// per exiled card. Appended after MergedTriggerPush, still after every
	// earlier Kind, so no earlier ordinal, hash chain or golden replay is
	// affected.
	Discover
	// Seek records one completed seek action (Forge's Alchemy seek, task
	// trigdisc1): Player is the seeking seat and Obj the resolving source
	// permanent (0 for a source-less body). It is an Apply no-op marker,
	// exactly like Discover, and is what trig:SeekAll matches ("whenever you
	// seek one or more cards" -- Vexyr, Ich-Tekik's Heir; Val, Marooned
	// Surveyor; Lurker in the Deep). One marker per seek ACTION, never one
	// per sought card -- a seek of three cards is one marker and one
	// trigger -- and the emitter's contract (api:Seek, still unimplemented)
	// is to emit only when the seek actually found at least one card, so the
	// oracle's "one or more cards" holds. Appended after Discover, still
	// after every earlier Kind, so no earlier ordinal, hash chain or golden
	// replay is affected.
	Seek
	// Connive records one completed connive action (CR 702.59, task connive1):
	// Obj is the conniving permanent, Player its controller, IDs the cards it
	// discarded in discard order, and Amount the number of NONLAND cards
	// among them (the +1/+1 counters that went on the conniver). It is an
	// Apply no-op marker, exactly like Explore: the connive's own state
	// changes (the draws, the discards, the counter) are their own events
	// that precede this one, and the record is what trig:Connives matches
	// (Iron Monger Sadistic Tycoon, Glorious Purpose, Ultron Unlimited).
	// One marker per completed connive ACTION, never one per discarded card.
	// Appended after Seek, still after every earlier Kind, so no earlier
	// ordinal, hash chain or golden replay is affected.
	Connive
	// Enlist records one CR 702.160 enlist action (the `K:Enlist` keyword,
	// task enlist1): Obj is the ATTACKING creature that enlisted (the
	// trigger's source for Mode$ Enlisted, so ValidCard$ Card.Self matches
	// it) and IDs[0] the nonattacking creature it tapped, Player the
	// attacker's controller. Apply folds the per-combat stamp the
	// enlistedThisCombat filter predicate reads; the tap is its own Tap
	// event and the +X/+0 is a rules-registered continuous pump, so this
	// event is the enlist action's canonical record and the Mode$ Enlisted
	// trigger's carrier. Appended here, after Connive, following every prior
	// Kind's own append-only precedent, so no earlier ordinal, hash chain
	// or golden replay is affected.
	Enlist
	// NumKinds is the number of defined Kind constants, one past the last
	// (state.Zone's numZones, next package over, is the same shape). It
	// exists for the scans that must visit every kind: view's
	// Describe-coverage test used to bound its loop with a kind NAME
	// (EndCombatReset) that silently stopped being the last Kind, so eight
	// kinds landed past the loop and were never described; bounding by
	// NumKinds instead means a Kind appended here is covered by
	// construction, with no edit to the scan. It must stay AFTER the last
	// Kind: appending a Kind below it would renumber every later ordinal
	// and corrupt the hash chain, so new kinds always go above it.
	NumKinds = int(Enlist) + 1
)

// mergedTriggerShift is the width MergedTriggerPush's Amount gives the
// under-card's own Triggers index; the pile index sits above it. Both are
// small non-negative card-script indices (a pile is a handful of cards, a
// face a handful of T: lines), so 16 bits each is far beyond any real value
// and the packing stays inside int32 with room to spare.
const mergedTriggerShift = 16

// MergedTriggerAmount packs a MergedTriggerPush's Amount: mergedIdx is the
// under-card's position in the pile's MergedCards (top-of-pile first) and
// trigIdx is that under-card face's own Triggers index. A negative or
// oversized index yields -1, which MergedTriggerIndexes reports as invalid
// so Apply degrades the push to a no-op rather than minting a wrong ability.
func MergedTriggerAmount(mergedIdx, trigIdx int) int32 {
	if mergedIdx < 0 || trigIdx < 0 ||
		mergedIdx >= 1<<mergedTriggerShift || trigIdx >= 1<<mergedTriggerShift {
		return -1
	}
	return int32(mergedIdx)<<mergedTriggerShift | int32(trigIdx)
}

// MergedTriggerIndexes unpacks MergedTriggerAmount. ok is false for a value
// this build cannot read (a negative Amount -- a tampered or truncated log),
// which every caller treats as "mint nothing".
func MergedTriggerIndexes(amount int32) (mergedIdx, trigIdx int, ok bool) {
	if amount < 0 {
		return 0, 0, false
	}
	return int(amount >> mergedTriggerShift), int(amount & (1<<mergedTriggerShift - 1)), true
}

// CopyToken's Amount rider bitmask (DB$ CopyPermanent's entry-state
// riders, folded in Apply so replay derives the identical object):
// TokenTapped$ True, TokenAttacking$ True, and AtEOT$ ExileCombat -- the
// latter flags the copy IsMyriad so the existing end-of-combat cleanup
// (MyriadCleanup, CR 702.109a) exiles it with the same semantics every
// Myriad token already had: end-of-combat exile, battlefield only.
const (
	CopyTokenTapped      int32 = 1
	CopyTokenAttacking   int32 = 2
	CopyTokenExileCombat int32 = 4
)

// ExtraPhaseRiders is the rider payload an api:AddPhase grant forwards for
// its ExtraPhaseDelayedTrigger$ pair (Moraug's "at the beginning of that
// combat, untap all creatures you control"), Text-encoded on the ExtraPhase
// event (Ruling T20-a's field-reuse precedent -- the event gains no field):
// "DELAY=<step ordinal>" and "VP=<ValidPlayer$ value>", joined with "|".
// The delayed phase cannot ride the IDs slice beside the entry/FollowedBy
// steps: an absent rider and the zero Step (untap) would be
// indistinguishable, so the riders live in Text and the IDs slots stay
// unambiguous (IDs[0] the entry step, IDs[1] an explicit FollowedBy$ only).
type ExtraPhaseRiders struct {
	HasDelayedPhase bool
	DelayedPhase    state.Step
	ValidPlayer     string
}

const (
	extraPhaseDelayKey = "DELAY="
	extraPhaseVPKey    = "VP="
)

// EncodeExtraPhaseRiders writes the rider payload as the canonical Text
// marker. Deterministic key order (DELAY first), so the same riders always
// encode identically.
func EncodeExtraPhaseRiders(r ExtraPhaseRiders) string {
	var parts []string
	if r.HasDelayedPhase {
		parts = append(parts, extraPhaseDelayKey+strconv.FormatInt(int64(r.DelayedPhase), 10))
	}
	if r.ValidPlayer != "" {
		parts = append(parts, extraPhaseVPKey+r.ValidPlayer)
	}
	return strings.Join(parts, "|")
}

// DecodeExtraPhaseRiders reads the rider payload back; the zero value (no
// riders) for any other Text.
func DecodeExtraPhaseRiders(text string) ExtraPhaseRiders {
	var r ExtraPhaseRiders
	for _, part := range strings.Split(text, "|") {
		if v, ok := strings.CutPrefix(part, extraPhaseDelayKey); ok {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 && state.Step(n).Valid() {
				r.HasDelayedPhase, r.DelayedPhase = true, state.Step(n)
			}
			continue
		}
		if v, ok := strings.CutPrefix(part, extraPhaseVPKey); ok {
			r.ValidPlayer = strings.TrimSpace(v)
		}
	}
	return r
}

// kindNames is declared with NumKinds's length, never [...] inferred, so
// kindNames and the enum cannot drift apart: a Kind added without a name (or
// a name added without a Kind) is a compile error, the same lockstep
// zoneNames has with numZones.
var kindNames = [NumKinds]string{"game_start", "shuffle", "move_zone", "draw",
	"life", "damage", "tap", "untap", "step", "turn", "priority", "stack_push",
	"stack_resolve", "mana_add", "mana_clear", "counter", "declare_attackers",
	"declare_blockers", "player_lost", "game_over", "decision_ask",
	"decision_made", "note", "land_played", "targets_chosen", "flip_face",
	"clock_tick", "trigger_push", "end_combat_reset", "cast_info", "choose",
	"token_create", "stack_copy", "attach", "ability_push", "mode_chosen", "commander_damage",
	"delayed_register", "delayed_push", "library_order", "extra_turn", "door_unlock", "speed_change",
	"monarch_change", "control_change", "card_token", "keyword_trigger_push", "goad", "player_counter", "imprint", "starting_player_change",
	"pair", "myriad_copy", "myriad_cleanup", "grant_trigger_push", "mana_activate", "token_attacks",
	"x_change", "note_number", "extra_phase", "copy_token", "exert", "planar_roll", "explore", "combat_retarget", "ring_tempts_you", "ring_emblem_push", "grant_ability_push", "investigate", "blessing_change", "clone_permanent", "mutate", "merged_trigger_push",
	"discover", "seek", "enlist"}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "unknown"
}

// ExtraTurnSkipUntapText is the canonical Text marker on an ExtraTurn grant
// whose Forge AddTurn source carries SkipUntap$ True. Text is already part of
// Event's encoded union, so it preserves this turn-specific rider without a
// schema change.
const ExtraTurnSkipUntapText = "extra turn; skip untap"

// Event is a state delta. The field set is a flat union so encoding stays
// allocation-free and an external consumer needs no engine code to read it.
// manaRestrictionPrefix marks a ManaAdd event whose added (or spent) mana is
// governed by RestrictValid$. Text is otherwise unused by ManaAdd, so this
// preserves the event wire shape while keeping the provenance replayable.
const manaRestrictionPrefix = "mana-restriction:"

// ManaRestrictionText encodes a RestrictValid$ constraint on a ManaAdd event.
// The optional " @<source-id>" segment (appended only when the producing
// source is known and nonzero, so every historical encoding stays
// byte-identical) records WHICH permanent's ability produced the batch — the
// source-relative predicates a Valid value may carry (Cavern of Souls'
// ChosenType) are resolved against it, never against the card being paid for.
func ManaRestrictionText(valid string, source state.ObjID) string {
	if source == 0 {
		return manaRestrictionPrefix + valid
	}
	return manaRestrictionPrefix + valid + " @" + strconv.FormatUint(uint64(source), 10)
}

// The AddsNoCounter$ suffixes. They are appended AFTER the optional source
// segment, and only when the producing ability carries AddsNoCounter$, so
// every historical encoding stays byte-identical. " nc" is the plain flag
// (Cavern of Souls); " nc!Permanent" is Forge's conditional
// AddsNoCounter$ !Permanent (Boseiju: the spell must not be a permanent).
const (
	manaRestrictionNC        = " nc"
	manaRestrictionNCNotPerm = " nc!Permanent"
)

// ManaRestrictionTextNC is ManaRestrictionText for a batch whose producing
// ability also carries AddsNoCounter$. cond is "True" (the plain flag) or
// "NotPermanent" (AddsNoCounter$ !Permanent); an empty cond encodes the plain
// historical shape.
func ManaRestrictionTextNC(valid string, source state.ObjID, cond string) string {
	base := ManaRestrictionText(valid, source)
	switch cond {
	case "NotPermanent":
		return base + manaRestrictionNCNotPerm
	case "True":
		return base + manaRestrictionNC
	default:
		return base
	}
}

// ManaRestrictionFromText returns the constraint carried by a restricted
// ManaAdd event, with the producing source id when the encoding carries one
// (0 otherwise) and the AddsNoCounter$ condition when one is encoded (""). It
// deliberately accepts no aliases: ordinary historical ManaAdd events must
// remain unrestricted. A bare empty Valid with a condition still counts as a
// restriction batch (the batch is unrestricted spend-wise but carries the
// can't-be-countered provenance).
func ManaRestrictionFromText(text string) (string, state.ObjID, string, bool) {
	valid, ok := strings.CutPrefix(text, manaRestrictionPrefix)
	if !ok || valid == "" {
		return "", 0, "", false
	}
	cond := ""
	if s, found := strings.CutSuffix(valid, manaRestrictionNCNotPerm); found {
		cond, valid = "NotPermanent", s
	} else if s, found := strings.CutSuffix(valid, manaRestrictionNC); found {
		cond, valid = "True", s
	}
	if _, tail, found := strings.Cut(valid, " @"); found {
		head, _, _ := strings.Cut(valid, " @")
		if n, err := strconv.ParseUint(tail, 10, 64); err == nil {
			return head, state.ObjID(n), cond, true
		}
	}
	return valid, 0, cond, true
}

type Event struct {
	Seq     uint64           `json:"seq"`
	Kind    Kind             `json:"kind"`
	Player  state.PlayerID   `json:"player,omitempty"`
	Obj     state.ObjID      `json:"obj,omitempty"`
	From    state.Zone       `json:"from,omitempty"`
	To      state.Zone       `json:"to,omitempty"`
	Amount  int32            `json:"amount,omitempty"`
	Step    state.Step       `json:"step,omitempty"`
	Counter string           `json:"counter,omitempty"`
	Text    string           `json:"text,omitempty"`
	IDs     []state.ObjID    `json:"ids,omitempty"`
	Pairs   [][2]state.ObjID `json:"pairs,omitempty"`
	// Secret marks an event whose payload is hidden information. View
	// projection redacts it for everyone but Player.
	Secret bool `json:"secret,omitempty"`
}

// Append writes a compact, deterministic encoding of e to dst. Every field is
// included and length-prefixed where variable, so two different events can
// never encode identically.
func (e Event) Append(dst []byte) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], e.Seq)
	dst = append(dst, b[:]...)
	dst = append(dst, byte(e.Kind), byte(e.Player), byte(e.From), byte(e.To), byte(e.Step))
	binary.LittleEndian.PutUint32(b[:4], uint32(e.Obj))
	dst = append(dst, b[:4]...)
	binary.LittleEndian.PutUint32(b[:4], uint32(e.Amount))
	dst = append(dst, b[:4]...)
	if e.Secret {
		dst = append(dst, 1)
	} else {
		dst = append(dst, 0)
	}
	dst = appendStr(dst, e.Counter)
	dst = appendStr(dst, e.Text)
	binary.LittleEndian.PutUint32(b[:4], uint32(len(e.IDs)))
	dst = append(dst, b[:4]...)
	for _, id := range e.IDs {
		binary.LittleEndian.PutUint32(b[:4], uint32(id))
		dst = append(dst, b[:4]...)
	}
	binary.LittleEndian.PutUint32(b[:4], uint32(len(e.Pairs)))
	dst = append(dst, b[:4]...)
	for _, p := range e.Pairs {
		binary.LittleEndian.PutUint32(b[:4], uint32(p[0]))
		dst = append(dst, b[:4]...)
		binary.LittleEndian.PutUint32(b[:4], uint32(p[1]))
		dst = append(dst, b[:4]...)
	}
	return dst
}

func appendStr(dst []byte, s string) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(len(s)))
	dst = append(dst, b[:]...)
	return append(dst, s...)
}

// flagNames pairs each CastFlags bit with the name CastInfo's Counter field
// carries for it, in a fixed order -- so FlagsString(FlagsFrom(s)) is
// canonical (a determinism requirement: this order must never depend on map
// iteration).
var flagNames = [...]struct {
	name string
	bit  uint64
}{
	{"kicked", state.FlagKicked},
	{"surged", state.FlagSurged},
	{"flashback", state.FlagFlashback},
	{"miracle", state.FlagMiracle},
	// Appended at the end, keeping the historic four first: a flag list is
	// canonicalised in THIS table order, so appending new flags after the
	// old ones keeps FlagsString(FlagsFrom(s)) stable for every name the
	// original table already knew.
	{"evoked", state.FlagEvoked},
	{"dashed", state.FlagDashed},
	{"overloaded", state.FlagOverloaded},
	{"warped", state.FlagWarped},
	{"buyback", state.FlagBuyback},
	{"harmonize", state.FlagHarmonize},
	{"suspend", state.FlagSuspend},
	{"escaped", state.FlagEscaped},
	{"mayplay", state.FlagMayPlay},
	// The and/or Kicker's index bits (Wastescape Battlemage's two
	// independently optional kickers). Appended at the end per the table's
	// own ordering rule.
	{"kicked 1", state.FlagKicked1},
	{"kicked 2", state.FlagKicked2},
	// The can't-be-countered provenance of a spell paid with AddsNoCounter$
	// mana (Cavern of Souls). Appended at the end per the table's own
	// ordering rule.
	{"ncount", state.FlagNoCounter},
	// The Adventure spell face's cast (CR 714.3a). Appended at the end per
	// the table's own ordering rule.
	{"adventure", state.FlagAdventure},
	// The Replicate keyword's payment provenance (CR 702.55a); the payment
	// COUNT rides the same CastInfo's Amount. Appended at the end per the
	// table's own ordering rule.
	{"replicated", state.FlagReplicated},
	// Converge's spend provenance (CR 107.4f-family); the distinct-colour
	// COUNT rides the same CastInfo's Amount. Appended at the end per the
	// table's own ordering rule.
	{"converged", state.FlagConverged},
	// The Bestow keyword's alternative-cost cast (CR 702.114a); the flag is
	// the provenance rules' resolution reader uses to substitute the
	// synthesized Aura attach spell. Appended at the end per the table's
	// own ordering rule.
	{"bestowed", state.FlagBestowed},
	// Multikicker's payment provenance (CR 702.43); the TIMES-KICKED COUNT
	// rides the same CastInfo's Amount. Appended at the end per the table's
	// own ordering rule.
	{"multikicked", state.FlagMultikicked},
	// Foretell's cast provenance (CR 702.126a): set by BOTH provenance
	// markers -- the {2} face-down hand exile (rules' payCast foretell
	// branch, which emits its own CastInfo) and the later foretell-cost
	// cast from exile (modeFlags). Appended at the end per the table's own
	// ordering rule.
	{"foretold", state.FlagForetold},
	// Conspire's tap provenance (CR 702.78a): the flag is what the keyword
	// expansion's copy trigger reads through Count$Conspired, so a declined
	// Conspire emits no flag and resolves like the plain cast. Appended at
	// the end per the table's own ordering rule.
	{"conspired", state.FlagConspired},
	// Convoke's creature provenance (CR 702.66, task connive1): the flag is
	// what Defined$ Convoked reads -- the creatures tapped to help pay for
	// the cast ride the pay-time CastInfo's IDs. Emitted only for a face
	// whose SVar table or abilities reference the selector (rules/cast.go's
	// faceWantsConvoked), so every unrelated convoke cast stays
	// byte-identical. Appended at the end per the table's own ordering rule.
	{"convoked", state.FlagConvoked},
	// The total-mana-spent capture (task castprov1): a face whose SVar
	// table reads the Count$CastTotalManaSpent head stamps its pay-time
	// CastInfo with the flag, so the Amount folds into Object.ManaSpent
	// instead of overwriting X. Appended at the end per the table's own
	// ordering rule.
	{"manaspent", state.FlagManaSpent},
	// The SNOW-unit part of the total-mana-spent capture (task castfilter1):
	// a face whose SVar table reads the filtered Count$CastTotalManaSpent Snow
	// head stamps its pay-time CastInfo with this flag too, so the Amount
	// folds into Object.ManaSnowSpent instead of overwriting X or the
	// unfiltered total. Appended at the end per the table's own ordering
	// rule.
	{"manasnowspent", state.FlagManaSnowSpent},
	// The TREASURE-/CAVE-/DESERT-sourced parts of the total-mana-spent
	// capture (task castfilter2): a face whose SVar table reads the filtered
	// Count$CastTotalManaSpent Treasure/Cave/Desert head stamps its pay-time
	// CastInfo with these flags too, so each Amount folds into its own
	// Object field instead of overwriting X, the total, or an earlier tag.
	// Appended at the end per the table's own ordering rule.
	{"manatreasurespent", state.FlagManaTreasureSpent},
	{"manacavespent", state.FlagManaCaveSpent},
	{"manadesertspent", state.FlagManaDesertSpent},
	// The DB$ Play ReplaceGraveyard$ Exile rider (task replplay1): the Play
	// SA's own provenance stamps its pay-time CastInfo with this flag, so
	// spellRestZone/spellFizzleZone send the played card to exile instead
	// of the graveyard. Appended at the end per the table's own ordering
	// rule.
	{"replacegraveyard", state.FlagReplaceGraveyard},
	// The Aftermath half's cast (CR 702.85a): the flag is what the resolution
	// reader (spellRestZone) and the fizzle reader (spellFizzleZone) read to
	// exile the card instead of the graveyard. Appended at the end per the
	// table's own ordering rule.
	{"aftermath", state.FlagAftermath},
	// Mutate's cast provenance (CR 702.140a), the placement choice riding
	// FlagMutatedTop beside it. Appended at the end per the table's own
	// ordering rule.
	{"mutated", state.FlagMutated},
	{"mutated top", state.FlagMutatedTop},
	// The Squad keyword's payment provenance (CR 702.66); the payment COUNT
	// rides the same CastInfo's Amount. Appended at the end per the table's
	// own ordering rule.
	{"squadpaid", state.FlagSquadPaid},
	// The Fuse cast (CR 702.101b) of a non-Room Split card: one spell
	// resolving both halves. Appended at the end per the table's own
	// ordering rule.
	{"fused", state.FlagFused},
}

// FlagsFrom parses a comma-separated flag list (CastInfo.Counter's shape)
// into a CastFlags word. Unrecognized names are silently ignored, the same
// totality stance as everywhere else in this package: a stray or future
// flag name in an untrusted log must not make this panic.
func FlagsFrom(s string) uint64 {
	var f uint64
	for _, part := range strings.Split(s, ",") {
		for _, fn := range flagNames {
			if strings.TrimSpace(part) == fn.name {
				f |= fn.bit
			}
		}
	}
	return f
}

// FlagsString is FlagsFrom's inverse: a canonical, fixed-order csv of the
// flag names set in f.
func FlagsString(f uint64) string {
	var parts []string
	for _, fn := range flagNames {
		if f&fn.bit != 0 {
			parts = append(parts, fn.name)
		}
	}
	return strings.Join(parts, ",")
}
