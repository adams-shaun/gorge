package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestHandExileRecordsExiledWith: a hidden pick out of the hand into exile
// records the card as exiled with the resolving source, as Forge's
// changeHiddenOriginResolve does (handleExiledWith), when the source reads
// that set: All Good Things . . .'s "cards exiled with this Saga" counts the
// cards its chapters exiled from the hand. A source that never reads the set
// gets no association, so its event log is unchanged.
func TestHandExileRecordsExiledWith(t *testing.T) {
	for _, tc := range []struct {
		name  string
		svars map[string]string
		want  int32
	}{
		{"reader", map[string]string{"Tally": "ExiledWith$Amount"}, 1},
		{"no reader", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			src := h.g.AddObject(mkCard(t, "Name:Archive Saga\nTypes:Enchantment Saga\nOracle:x\n"), 0)
			src.Zone = state.ZBattlefield
			h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
			card := h.g.AddObject(mkCard(t, "Name:Spare Thought\nTypes:Instant\nOracle:x\n"), 0)
			card.Zone = state.ZHand
			h.g.SetZone(state.ZHand, 0, []state.ObjID{card.ID})
			c := &Ctx{Source: src.ID, Controller: 0, SVars: tc.svars}
			Resolve(h, c, sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Exile | Mandatory$ True"))
			if z := h.g.Obj(card.ID).Zone; z != state.ZExile {
				t.Fatalf("the hand card did not move to exile: zone=%s", z)
			}
			if got, ok := EvalCountOK(h, &Ctx{Source: src.ID, Controller: 0}, "ExiledWith$Amount"); !ok || got != tc.want {
				t.Fatalf("ExiledWith$Amount = (%d, %v), want (%d, true)", got, ok, tc.want)
			}
		})
	}
}
