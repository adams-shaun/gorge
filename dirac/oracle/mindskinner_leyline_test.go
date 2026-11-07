package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMindskinnerLeylineLockstep: The Mindskinner's PreventMillOpps damage
// replacement mills cards while the opponent controls Leyline of the Void.
// The kernel's preventMill moves each milled card through placeFromK, whose
// graveyard destination asks exileAnyDest, so each mill is exiled exactly as
// gorge's Moved replacement does; the lockstep must see no divergence and
// the gorge side must actually exile the milled cards.
func TestMindskinnerLeylineLockstep(t *testing.T) {
	st := &dpsStats{}
	hits, exiled, buried := 0, 0, 0
	leylineOnBattlefield := func(g *state.Game) bool {
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if dpsNamed(g, id, "Leyline of the Void") {
					return true
				}
			}
		}
		return false
	}
	check := func(ref *GorgeSide, _ Side) *Divergence {
		st.scan(ref, func(g *state.Game, e *events.Event) {
			if e.Kind == events.DeclareAttackers {
				for _, id := range e.IDs {
					if dpsNamed(g, id, "The Mindskinner") {
						hits++
					}
				}
				return
			}
			// A milled card's library move is logged as its replaced
			// destination: exile with Leyline in play, graveyard without it.
			if e.Kind != events.MoveZone || e.From != state.ZLibrary || !leylineOnBattlefield(g) {
				return
			}
			switch e.To {
			case state.ZExile:
				exiled++
			case state.ZGraveyard:
				buried++
			}
		})
		return nil
	}
	dpsRun(t, "std-dsk", [2][]cardset.Entry{
		{{Name: "The Mindskinner", Count: 6}, {Name: "Leyline of the Void", Count: 6}, {Name: "Island", Count: 28}},
		{{Name: "Swamp", Count: 40}},
	}, st, check)
	if hits == 0 {
		t.Fatal("precondition: no game had The Mindskinner attack")
	}
	if exiled == 0 {
		t.Fatal("no card milled under Leyline of the Void was exiled on the gorge side")
	}
	if buried != 0 {
		t.Fatalf("%d card(s) milled under Leyline of the Void reached the gorge graveyard", buried)
	}
}
