package gamecfg

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// Reader is what the v2 adapter reads from a game in progress.
type Reader interface {
	// Board is read-only.
	Board() *state.Game
	// Pending is nil when nothing is asked.
	Pending() *decision.Decision
	// Probe submits ins to an independent copy and returns that copy, or the
	// first refusal. The receiver does not move. The copy holds the real hidden
	// zones, so no outcome may shape a decision through their contents.
	Probe(ins ...decision.Intent) (Reader, error)
	Toughness(id state.ObjID) int32
	HasKeyword(id state.ObjID, kw string) bool
	// ObservedAbilities lists id's non-mana activated abilities in the order
	// v2's activate_ability.ability_index names them.
	ObservedAbilities(p state.PlayerID, id state.ObjID) []decision.Option
}

// engineReader reads a gorge engine.
type engineReader struct{ e *rules.Engine }

func (r engineReader) Board() *state.Game { return r.e.G }

func (r engineReader) Pending() *decision.Decision { return r.e.Pending() }

func (r engineReader) Probe(ins ...decision.Intent) (Reader, error) {
	c, err := probe(r.e, ins...)
	if err != nil {
		return nil, err
	}
	return engineReader{c}, nil
}

func (r engineReader) Toughness(id state.ObjID) int32 { return r.e.Toughness(id) }

func (r engineReader) HasKeyword(id state.ObjID, kw string) bool { return r.e.HasKeyword(id, kw) }

func (r engineReader) ObservedAbilities(p state.PlayerID, id state.ObjID) []decision.Option {
	return rules.ObservedAbilities(r.e, p, id)
}
