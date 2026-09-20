package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

func init() { Register("RollPlanarDice", effRollPlanarDice) }

// effRollPlanarDice implements AB$ RollPlanarDice (CR 901.3; 1 corpus
// carrier, Fractured Powerstone — the whole measured population, since no
// plane deck exists in this engine and no other card rolls the planar die).
// The effect itself is only the announcement: it emits the proposed
// PlanarRoll event carrying Amount$ (default 1 — the corpus carrier names
// none), and rules' replacement dispatch owns everything after it: the
// CR 616.1 planar-dice replacements apply to the held event (Ichor
// Elixir's "roll that many plus one and ignore one"), then the dispatch
// rolls the rewritten count through the engine rng, notes each die and
// logs the record with the kept results. A count below 1 rolls nothing and
// emits nothing — the "one or more" scope Ichor's own text names.
//
// The faces (1-4 blank, 5 planeswalk, 6 chaos — CR 901.3a) have NO engine
// consequence: there is no plane deck to planeswalk away from and no plane
// chaos ability to trigger, so the roll is recorded and nothing more. Any
// future plane support keys off this event, never off the die-roll Notes.
func effRollPlanarDice(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "Amount", 1)
	if n < 1 {
		return
	}
	h.Emit(events.Event{Kind: events.PlanarRoll, Player: c.Controller, Obj: c.Source, Amount: n})
}
