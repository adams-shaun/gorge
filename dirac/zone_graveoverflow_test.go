package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTokenWipeOverfillsGraveyardStalls: a token that dies waits in its
// owner's graveyard until the next state-based check (ceaseTokensGen), so a
// wipe of a token army can put more objects there at once than the
// MaxDeck-sized array holds. The kernel stalls (StallZone, the capFull
// pattern) instead of indexing past the array: before, placeFrom panicked
// with "index out of range [60] with length 60" (hbot-vs-hbot FDN games
// 12404 and 28769 of a random-deck hunt: 52 and 56 Goblin tokens in
// the graveyard beside 8 and 4 cards).
func TestTokenWipeOverfillsGraveyardStalls(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Llanowar Elves", Count: 1}, {Name: "Swamp", Count: 8}})
	if !sc.s.g.on {
		t.Skip("not a generated table")
	}
	sc.grave(id["Swamp"], 0, 20)
	var toks []Obj
	for i := 0; i < MaxDeck; i++ {
		o := sc.s.newToken(id["Llanowar Elves"], 0)
		sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
		sc.s.perms[sl].damage = 5
		toks = append(toks, o)
	}
	sc.s.npend = 0
	sc.s.sba(sc.k)
	if got := sc.s.StallReason(); got != StallZone {
		t.Fatalf("stall reason %q, want %q", StallName(got), StallName(StallZone))
	}
	for _, o := range toks {
		if sc.onBF(o) {
			t.Fatalf("token %d still on the battlefield", o)
		}
	}
	// a generated table stops at its next step, as every full pool does
	if d := sc.s.Step(); d.Kind != DkNone || !sc.s.over {
		t.Fatalf("game not over after the zone stall: decision kind %d", d.Kind)
	}
}
