package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestChangeZoneAllAttachmentSweepMatchesBeforeAnyMove: "return enchanted
// creature and all Auras attached to that creature" decides its set once, when
// the effect applies (CR 608.2h), and moves it together. Whichever of the
// creature and its Auras the scan reaches first, all three leave.
func TestChangeZoneAllAttachmentSweepMatchesBeforeAnyMove(t *testing.T) {
	for _, tc := range []struct {
		name  string
		order []string
	}{
		{"source Aura first", []string{"mark", "bear", "favor"}},
		{"creature first", []string{"bear", "mark", "favor"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			mark := h.g.AddObject(mkCard(t, "Name:Mark\nManaCost:U\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"), 0)
			bear := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
			favor := h.g.AddObject(mkCard(t, "Name:Favor\nManaCost:W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"), 0)
			ids := map[string]state.ObjID{"mark": mark.ID, "bear": bear.ID, "favor": favor.ID}
			var zone []state.ObjID
			for _, n := range tc.order {
				h.g.Obj(ids[n]).Zone = state.ZBattlefield
				zone = append(zone, ids[n])
			}
			h.g.SetZone(state.ZBattlefield, 0, zone)
			h.g.Obj(ids["mark"]).AttachedTo = ids["bear"]
			h.g.Obj(ids["favor"]).AttachedTo = ids["bear"]
			sa := &cards.SA{API: "ChangeZoneAll", Params: map[string]string{
				"ChangeType": "Card.EnchantedBy,Aura.AttachedTo Card.EnchantedBy",
				"Origin":     "Battlefield", "Destination": "Hand",
			}}
			effChangeZoneAll(h, &Ctx{Source: ids["mark"], Controller: 0}, sa)
			for _, n := range []string{"mark", "bear", "favor"} {
				if got := h.g.Obj(ids[n]).Zone; got != state.ZHand {
					t.Errorf("%s zone = %v, want hand", n, got)
				}
			}
		})
	}
}
