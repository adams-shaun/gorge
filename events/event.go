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
	NumKinds = int(MyriadCleanup) + 1
)

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
	"pair", "myriad_copy", "myriad_cleanup"}

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
	bit  uint16
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
}

// FlagsFrom parses a comma-separated flag list (CastInfo.Counter's shape)
// into a CastFlags byte. Unrecognized names are silently ignored, the same
// totality stance as everywhere else in this package: a stray or future
// flag name in an untrusted log must not make this panic.
func FlagsFrom(s string) uint16 {
	var f uint16
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
func FlagsString(f uint16) string {
	var parts []string
	for _, fn := range flagNames {
		if f&fn.bit != 0 {
			parts = append(parts, fn.name)
		}
	}
	return strings.Join(parts, ",")
}
