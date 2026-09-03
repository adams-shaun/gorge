// Package events defines every way state can change and is the only package
// permitted to mutate state.Game. The log is therefore a complete description
// of a match, and replay is state reconstruction rather than re-simulation.
package events

import (
	"encoding/binary"

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
)

var kindNames = [...]string{"game_start", "shuffle", "move_zone", "draw",
	"life", "damage", "tap", "untap", "step", "turn", "priority", "stack_push",
	"stack_resolve", "mana_add", "mana_clear", "counter", "declare_attackers",
	"declare_blockers", "player_lost", "game_over", "decision_ask",
	"decision_made", "note", "land_played", "targets_chosen", "flip_face",
	"clock_tick"}

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
