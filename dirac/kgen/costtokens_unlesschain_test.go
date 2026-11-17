package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnlessPartChainShapes pins unlessPartChain's family: an UnlessCost$ Sac
// with a life and/or draw rider is one instruction carrying the riders; every
// other multi-part unless cost keeps payInstr's one-part refusal.
func TestUnlessPartChainShapes(t *testing.T) {
	lc := &lowerCtx{unlessCost: true}
	sac := &lFilter{Clauses: []lClause{{Types: tCreature, Ctrl: whoYou}}}
	none := manaCost{None: true}

	in, err := lc.unlessPayInstr(lCost{Mana: none, Sac: sac, SacN: 3, PayLife: 3, DrawN: 3}, lRef{}, false)
	if err != nil {
		t.Fatalf("Sac+Life+Draw refused: %v", err)
	}
	b, _ := kop.PaySacRidersB(3, 3)
	if in.CS != "SacrificeRiders" || in.A.N != 3 || in.B.N != b || in.F != sac {
		t.Errorf("Sac+Life+Draw lowered to %s, want SacrificeRiders A=3 B=%d", in, b)
	}
	if life, draw := kop.PaySacRidersFrom(int16(in.B.N)); life != 3 || draw != 3 {
		t.Errorf("riders unpack to life %d draw %d", life, draw)
	}

	refused := []struct {
		name string
		cost lCost
	}{
		{"Life+Draw", lCost{Mana: none, PayLife: 3, DrawN: 3}},
		{"Sac+Life+Mill", lCost{Mana: none, Sac: sac, SacN: 1, PayLife: 3, MillN: 1}},
		{"Sac+Life+Mana", lCost{Mana: manaCost{Generic: 1}, Sac: sac, SacN: 1, PayLife: 3}},
		{"Sac+Life+Discard", lCost{Mana: none, Sac: sac, SacN: 1, PayLife: 3, DiscardN: 1}},
		{"Sac+Life 32", lCost{Mana: none, Sac: sac, SacN: 1, PayLife: 32}},
	}
	for _, tc := range refused {
		if _, err := lc.unlessPayInstr(tc.cost, lRef{}, false); err == nil {
			t.Errorf("%s: lowered, want a refusal", tc.name)
		}
	}

	// A lone Sac is not a chain: it is payInstr's ordinary one-part instruction.
	one, err := lc.unlessPayInstr(lCost{Mana: none, Sac: sac, SacN: 1}, lRef{}, false)
	if err != nil || one.CS != "Sacrifice" {
		t.Errorf("lone Sac lowered to %s (%v), want Sacrifice", one, err)
	}
	// Outside an unless cost nothing chains.
	if _, ok, _ := (&lowerCtx{}).unlessPartChain(lCost{Mana: none, Sac: sac, SacN: 1, PayLife: 3}, lRef{}); ok {
		t.Error("a chain lowered outside an unless cost")
	}
}
