package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestInitNStartRefusesNonPermanent: an instant or sorcery is never a
// permanent (CR 304.4, 307.4), so a start listing one on the battlefield
// panics, as gorge's StartSpec.Validate refuses it.
func TestInitNStartRefusesNonPermanent(t *testing.T) {
	tbl, _, _, _ := startTable()
	_, decks := startGame(tbl, 1, nil)
	if decks[0][5] != v.LightningBolt || tbl.Cards[v.LightningBolt].Types&kop.TInstant == 0 {
		t.Fatalf("precondition: deck card 5 is not the instant Lightning Bolt")
	}
	defer func() {
		if recover() == nil {
			t.Fatalf("an instant on the battlefield did not panic")
		}
	}()
	startGame(tbl, 1, &Start{Seats: []StartSeat{{Battlefield: []int{5}}}})
}
