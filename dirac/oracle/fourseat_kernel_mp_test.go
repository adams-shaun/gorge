//go:build dirac_mp

package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestFourSeatVanillaKernelGenesisAgrees starts four-seat vanilla matchups
// on the kernel through the oracle (KernelMatchupN, RealKernel's InitN) and
// on gorge: at the first decision both projections carry four seats and
// agree on every field, further seats' zone positions included. A short lockstep then runs to its first divergence
// or the end without the harness panicking.
func TestFourSeatVanillaKernelGenesisAgrees(t *testing.T) {
	c, _ := testCorpus(t)
	mk := RealKernel(vanilla.Table, nil, Config{})
	n := len(vanilla.Decks)
	for g := 0; g < 4; g++ {
		labels := make([]string, 4)
		decks := make([][]kop.CardID, 4)
		for i := range decks {
			d := (g + i) % n
			labels[i], decks[i] = fmt.Sprint("vanilla", d), vanilla.Decks[d]
		}
		seed := uint64(4100 + g)
		m, err := KernelMatchupN(c.Reg, vanilla.Table, labels, decks, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if m.NSeat() != 4 {
			t.Fatalf("game %d: %d seats", g, m.NSeat())
		}
		k, err := mk(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		gs, err := NewGorgeSide(m, seed, Config{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		k.Step()
		gs.Step()
		var kp, gp Projection
		k.Project(&kp)
		gs.Project(&gp)
		if kp.Seats.NSeat != 4 || gp.Seats.NSeat != 4 {
			t.Fatalf("game %d: NSeat kernel %d gorge %d, want 4", g, kp.Seats.NSeat, gp.Seats.NSeat)
		}
		if d := DiffProjection(&gp, &kp); len(d) != 0 {
			t.Fatalf("game %d: genesis projections differ (gorge, kernel): %v", g, d)
		}
		for s := 2; s < 4; s++ {
			if sp := kp.seat(s); *sp.life != 20 || *sp.hand != 7 || int(*sp.lib) != len(decks[s])-7 {
				t.Fatalf("game %d seat %d: life %d hand %d library %d", g, s, *sp.life, *sp.hand, *sp.lib)
			}
		}
		CloseSide(k)

		k, err = mk(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{})
		CloseSide(k)
		if res.Div != nil && res.Div.Field == "panic" {
			t.Fatalf("game %d: panic in %s: %s", g, res.Div.What, res.Div.Detail)
		}
		t.Logf("game %d: %d decisions, first divergence %v", g, res.Decisions, res.Div)
	}
}
