package gamecfg

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
)

// Reader is what the v2 adapter reads from a game in progress.
type Reader interface {
	// Board is read-only.
	Board() *state.Game
	// Chars answers derived facts over Board for view projections.
	Chars() view.Chars
	// Log holds the events and accepted intents so far.
	Log() *events.Log
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
	// Facts is id's characteristics after continuous effects.
	Facts(id state.ObjID) Facts
	// HostTriggerAmount is the causing event's magnitude for a stack ability
	// that reads one (TriggerCount$), or false.
	HostTriggerAmount(id state.ObjID) (int32, bool)
}

// Facts is what an observation reports of an object's characteristics.
type Facts struct {
	Name             string
	Types            []string
	Colors           string // WUBRG letters
	Keywords         []string
	Power, Toughness int32
}

// Readable says why a v2 seat cannot read g, or nil when it can.
func Readable(g game.Game) error {
	if gorgegame.Rules(g) == nil {
		return fmt.Errorf("the %q engine offers no v2 reader", g.Caps().Engine)
	}
	return nil
}

// Over is the Game a v2 seat reads g through. g must be Readable.
func Over(g game.Game, sec *secrets.Game) *Game {
	return Wrap(gorgegame.Rules(g), sec)
}

// engineReader reads a gorge engine.
type engineReader struct{ e *rules.Engine }

func (r engineReader) Board() *state.Game { return r.e.G }

func (r engineReader) Chars() view.Chars { return r.e }

func (r engineReader) Log() *events.Log { return r.e.L }

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

func (r engineReader) Facts(id state.ObjID) Facts {
	d := r.e.Derived(id)
	return Facts{Name: d.Name, Types: d.Types, Colors: d.Colors, Keywords: d.Keywords, Power: d.Power, Toughness: d.Toughness}
}

func (r engineReader) HostTriggerAmount(id state.ObjID) (int32, bool) {
	return rules.HostTriggerAmount(r.e, id)
}
