package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Overflow Submatrix and Storied Veteran each give a graveyard cast a
// finality counter: the Submatrix's SpellCast trigger for a cast through any
// may-play permission (Forge's Spell.MayPlaySource), the Veteran's stack-zone
// trigger for its own cast (CR 603.2, 614.12). The Holodeck lets either player
// cast any permanent card from the graveyard once a turn, so Subcommander T'Pol
// (flash) and the Veteran are cast from the graveyard beside both permissions:
// each cast gets one counter per grant that covers it, and the Veteran in the
// graveyard is offered its own cast beside the Holodeck's once-per-turn
// permission (the unlimited own grant is the offer gorge's payment builder
// plans, so the cast is priced, not ambiguous, with an empty pool).
func TestFinalityGraveyardCastLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "The Holodeck", Count: 4}, {Name: "Overflow Submatrix", Count: 4}, {Name: "Storied Veteran", Count: 4},
		{Name: "Subcommander T'Pol", Count: 4}, {Name: "Collation Trimatrix", Count: 3}, {Name: "Tenacious Drone", Count: 3},
		{Name: "Impossible Choice", Count: 2}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
	}
	b := []cardset.Entry{
		{Name: "Storied Veteran", Count: 4}, {Name: "Overflow Submatrix", Count: 3}, {Name: "Subcommander T'Pol", Count: 4},
		{Name: "The Holodeck", Count: 3}, {Name: "Collation Trimatrix", Count: 3}, {Name: "Tenacious Drone", Count: 3},
		{Name: "Data, New Life", Count: 2}, {Name: "Test of Priorities", Count: 2}, {Name: "Impossible Choice", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
	}
	fixtureLockstep(t, "std-trk", [2][]cardset.Entry{a, b}, 300)
}
