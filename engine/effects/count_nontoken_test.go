package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestThisTurnEnteredNonTokenCountsOnlyNontokenPermanents: a spec's nonToken
// property is CR 111's "nontoken" -- the permanents that entered this turn
// without being tokens -- not an unknown word the matcher fails closed on
// (Spock, Voice of Reason's draw counted nothing).
func TestThisTurnEnteredNonTokenCountsOnlyNontokenPermanents(t *testing.T) {
	h := newHost(t, 2)
	g := h.g
	land := g.AddObject(mkCard(t, "Name:Dune\nTypes:Land\nOracle:x\n"), 0)
	bear := g.AddObject(mkCard(t, "Name:Cub\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"), 0)
	scout := g.AddObject(mkCard(t, "Name:Scout\nTypes:Creature Human\nPT:1/1\nOracle:x\n"), 0)
	scout.IsToken = true
	for _, o := range []*state.Object{land, bear, scout} {
		o.Zone = state.ZBattlefield
		g.Entered = append(g.Entered, state.ZoneEntry{Obj: o.ID, To: state.ZBattlefield, From: state.ZStack, PermanentCard: !o.IsToken, WasCreature: o != land})
	}
	c := &Ctx{Controller: 0}
	for body, want := range map[string]int32{
		"Count$ThisTurnEntered_Battlefield_Permanent.nonToken+YouCtrl": 2,
		"Count$ThisTurnEntered_Battlefield_Permanent.YouCtrl":          3,
		"Count$ThisTurnEntered_Battlefield_Creature.nonToken":          1,
	} {
		if got, ok := EvalCountOK(h, c, body); !ok || got != want {
			t.Errorf("%s = (%d, %v), want (%d, true)", body, got, ok, want)
		}
	}
}
