package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTinybonesHasCardsInHand pins Tinybones, Trinket Thief's activated
// ability (Defined$ Player.Opponent+HasCardsInHand_Card_EQ0): each opponent
// whose hand holds no card loses the life, an opponent holding a card does
// not, and the controller never does.
func TestTinybonesHasCardsInHand(t *testing.T) {
	const line = "DB$ LoseLife | LifeAmount$ 10 | Defined$ Player.Opponent+HasCardsInHand_Card_EQ0"
	for _, tc := range []struct {
		name  string
		hands [3]int // cards in hand for seats 0..2
		want  [3]int32
	}{
		{"empty-handed opponent loses, holder does not", [3]int{1, 0, 1}, [3]int32{20, 10, 20}},
		{"both opponents empty", [3]int{0, 0, 0}, [3]int32{20, 10, 10}},
		{"nobody loses when every opponent holds a card", [3]int{0, 1, 3}, [3]int32{20, 20, 20}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 3)
			card := mkCard(t, "Name:Filler\nTypes:Creature\nPT:1/1\nOracle:x\n")
			src := h.g.AddObject(card, 0)
			src.Zone = state.ZBattlefield
			for p := range state.PlayerID(3) {
				var ids []state.ObjID
				for range tc.hands[p] {
					o := h.g.AddObject(card, p)
					o.Zone = state.ZHand
					ids = append(ids, o.ID)
				}
				h.g.SetZone(state.ZHand, p, ids)
				if got := len(h.g.Zone(state.ZHand, p)); got != tc.hands[p] {
					t.Fatalf("precondition: seat %d hand = %d, want %d", p, got, tc.hands[p])
				}
				if h.g.Players[p].Life != 20 {
					t.Fatalf("precondition: seat %d life = %d, want 20", p, h.g.Players[p].Life)
				}
			}
			c := &Ctx{Source: src.ID, Controller: 0}
			Resolve(h, c, sa(t, line))
			got := []int32{h.g.Players[0].Life, h.g.Players[1].Life, h.g.Players[2].Life}
			if !slices.Equal(got, tc.want[:]) {
				t.Errorf("life = %v, want %v", got, tc.want)
			}
		})
	}
}
