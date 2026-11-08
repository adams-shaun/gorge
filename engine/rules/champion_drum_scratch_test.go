package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestScratchChampionDrum(t *testing.T) {
	e, _ := raiseEngine(t, []string{"Champion of the Clachan", "Thoughtweft Imbuer", "Springleaf Drum", "Dream Seizer", "Eclipsed Realms", "Steam Vents"}, nil)
	champ := blightMove(t, e, 0, "Champion of the Clachan", state.ZHand)
	blightMove(t, e, 0, "Thoughtweft Imbuer", state.ZHand)
	blightMove(t, e, 0, "Springleaf Drum", state.ZBattlefield)
	blightMove(t, e, 0, "Dream Seizer", state.ZBattlefield)
	blightMove(t, e, 0, "Swamp", state.ZBattlefield)
	blightMove(t, e, 0, "Steam Vents", state.ZBattlefield)
	for i := 0; i < 5 && e.Pending() != nil && e.Pending().Kind != "priority"; i++ {
		t.Logf("pending %v", e.Pending().Kind)
		break
	}
	blightMove(t, e, 0, "Swamp", state.ZBattlefield)
	t.Logf("pending %+v", e.Pending())
	t.Logf("offered=%v", raiseCastOffered(e, champ))
}
