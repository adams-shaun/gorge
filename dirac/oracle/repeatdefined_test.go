package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRepeatDefinedLockstep plays each RepeatDefined$ card that blocks decks
// decision by decision against gorge: Tainted Pact's
// Pump RememberObjects$ Imprinted / Imprinted count loop and Demonlord
// Belzenlok's DigUntil ImprintFound$+RememberFound$ / Imprinted$Amount loop.
// A library-to-exile move is the proof the card's body actually resolved (no
// other card in the pool digs); without it the lockstep would pass vacuously.
func TestRepeatDefinedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cases := []struct {
		name  string
		decks [2][]cardset.Entry
	}{
		{"Tainted Pact", [2][]cardset.Entry{
			{{Name: "Tainted Pact", Count: 12}, {Name: "Grizzly Bears", Count: 8}, {Name: "Swamp", Count: 20}},
			{{Name: "Grizzly Bears", Count: 8}, {Name: "Swamp", Count: 32}},
		}},
		{"Demonlord Belzenlok", [2][]cardset.Entry{
			{{Name: "Demonlord Belzenlok", Count: 10}, {Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 20}},
			{{Name: "Grizzly Bears", Count: 8}, {Name: "Swamp", Count: 32}},
		}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var cur *GorgeSide
			seen, dug := 0, 0
			check := func(g *GorgeSide, _ Side) *Divergence {
				if g != cur {
					cur, seen = g, 0
				}
				for ; seen < len(g.E.L.Events); seen++ {
					e := g.E.L.Events[seen]
					if e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZExile {
						dug++
					}
				}
				return nil
			}
			sonRun(t, "repeatdefined", [][2][]cardset.Entry{c.decks}, 30, true, nil, check)
			if dug == 0 {
				t.Fatalf("no library-to-exile move: %s never resolved", c.name)
			}
			t.Logf("%s: %d library-to-exile moves", c.name, dug)
		})
	}
}
