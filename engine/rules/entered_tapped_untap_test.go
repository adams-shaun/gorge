package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// An effect that puts a land onto the battlefield tapped loses to "lands you
// control enter untapped" (CR 614.12), and only for that replacement's own
// controller.
func TestEffectTappedEntryYieldsToEntersUntapped(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	for _, tc := range []struct {
		name   string
		holder state.PlayerID
		has    bool
		tapped bool
	}{
		{"no replacement", 0, false, true},
		{"own Spelunking", 0, true, false},
		{"opponent's Spelunking", 1, true, true},
	} {
		e, _ := searchEngine(t, reg, "Rampant Growth")
		if tc.has {
			onBoardCard(t, e, tc.holder, mustCorpusCard(t, reg, "Spelunking"))
		}
		id := searchMoveByName(t, e, "Rampant Growth", state.ZHand)
		addMana(t, e, 0, "GC")
		submitChoices(t, e, plainCastOption(t, e, id).Index)
		d := passUntilNonPriority(t, e, 30)
		if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" {
			t.Fatalf("%s: pending = %+v, want the search", tc.name, d)
		}
		land := d.Options[0].Obj
		submitChoices(t, e, 0)
		o := e.G.Obj(land)
		if o.Zone != state.ZBattlefield || o.Tapped != tc.tapped {
			t.Errorf("%s: land in %s, tapped %v; want battlefield, tapped %v", tc.name, o.Zone, o.Tapped, tc.tapped)
		}
	}
}
