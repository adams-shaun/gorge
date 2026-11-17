package rules

// CR 107.4f/119.4: {R/P} is paid with {R} or 2 life. A seat at 1 life that
// controls no source of {R} (an untapped Island, a tapped land, colourless
// mana floating) can pay neither face, so the activation is not offered.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestPhyrexianColourFaceOffer(t *testing.T) {
	for _, tc := range []struct {
		name        string
		island      bool
		plains      bool // tapped
		floatC      int32
		wantOffered bool
	}{
		{"island only", true, false, 0, false},
		{"island and tapped second land", true, true, 0, false},
		{"island, tapped second land, floating colourless", true, true, 1, false},
		{"floating colourless only", false, false, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := testutil.CorpusRegistry(t)
			e := crAbortEngine(t, reg, "ur-delver", "Immolating Souleater")
			id := crAbortMove(t, e, 0, "Immolating Souleater", state.ZBattlefield)
			crActivationSA(t, e, id, "Pump", "RP")
			if tc.island {
				crAbortMove(t, e, 0, "Island", state.ZBattlefield)
			}
			if tc.plains {
				pl := crAbortMove(t, e, 0, "Island", state.ZBattlefield)
				e.emit(events.Event{Kind: events.Tap, Obj: pl})
				if !e.G.Obj(pl).Tapped {
					t.Fatal("fixture: land is not tapped")
				}
			}
			if tc.floatC > 0 {
				e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: tc.floatC})
			}
			if d := 1 - e.G.Players[0].Life; d != 0 {
				e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: d})
			}
			if got := e.G.Players[0].Life; got != 1 {
				t.Fatalf("fixture: life = %d, want 1", got)
			}
			e.askPriority(0)
			if _, ok := souleaterOffered(e, id); ok != tc.wantOffered {
				t.Fatalf("CR 107.4f: {R/P} offered = %v at 1 life with no {R} source, want %v; pool=%v", ok, tc.wantOffered, e.G.Players[0].Pool)
			}
		})
	}
}
