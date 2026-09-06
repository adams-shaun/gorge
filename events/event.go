// Package events defines every way state can change and is the only package
// permitted to mutate state.Game. The log is therefore a complete description
// of a match, and replay is state reconstruction rather than re-simulation.
package events

import (
	"encoding/binary"
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
	// EndCombatReset clears every object's IsAttacking/BlockedBy fields.
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
	// 4. Appended here, after EndCombatReset, following every prior Kind's
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
	// ModeChosen records the answer to a mid-resolution modal decision
	// (effects.Host.Ask, decided by rules' handleModes -- the KModes
	// decision effCharm poses over its Choices$, or the unless-pay yes/no
	// effCopySpellAbility poses over its UnlessCost$). Obj is the stack
	// object whose resolution was suspended, Player the chooser, and Text
	// the chosen option labels (csv, in execution order). It is a marker
	// like Note: the choice itself lives on the suspended resolution's Ctx
	// when the engine re-enters it, so Apply records nothing on state --
	// the event exists only to carry the choice into the log, which is
	// what makes a replay able to re-derive the same branch. Appended
	// here, after AbilityPush, following every prior Kind's own append-only
	// precedent, so no earlier ordinal, hash chain or golden replay is
	// affected (M2d-2).
	ModeChosen
)

var kindNames = [...]string{"game_start", "shuffle", "move_zone", "draw",
	"life", "damage", "tap", "untap", "step", "turn", "priority", "stack_push",
	"stack_resolve", "mana_add", "mana_clear", "counter", "declare_attackers",
	"declare_blockers", "player_lost", "game_over", "decision_ask",
	"decision_made", "note", "land_played", "targets_chosen", "flip_face",
	"clock_tick", "trigger_push", "end_combat_reset", "cast_info", "choose",
	"token_create", "stack_copy", "attach", "ability_push", "mode_chosen"}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "unknown"
}

// Event is a state delta. The field set is a flat union so encoding stays
// allocation-free and an external consumer needs no engine code to read it.
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
	bit  uint8
}{
	{"kicked", state.FlagKicked},
	{"surged", state.FlagSurged},
	{"flashback", state.FlagFlashback},
	{"miracle", state.FlagMiracle},
}

// FlagsFrom parses a comma-separated flag list (CastInfo.Counter's shape)
// into a CastFlags byte. Unrecognized names are silently ignored, the same
// totality stance as everywhere else in this package: a stray or future
// flag name in an untrusted log must not make this panic.
func FlagsFrom(s string) uint8 {
	var f uint8
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
func FlagsString(f uint8) string {
	var parts []string
	for _, fn := range flagNames {
		if f&fn.bit != 0 {
			parts = append(parts, fn.name)
		}
	}
	return strings.Join(parts, ",")
}
