//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFreePlayTwelveCandidates: a cast-without-paying effect that finds
// twelve candidates (Rishkar's Expertise over a hand of a 100-card deck
// reached eleven in four-seat cmd5 pods; the arrays held eight and stalled
// StallFree) offers all twelve, queues the picked ones in order and casts them.
func TestFreePlayTwelveCandidates(t *testing.T) {
	const n = 12
	if maxFree <= 8 || len((&genState{}).free) != maxFree || len((&genState{}).freeQ) != maxFree {
		t.Fatalf("maxFree %d: the four-seat class must hold more than eight free-cast candidates", maxFree)
	}
	sc, synth, id := m11Scen(t, []kop.Instr{{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: 5}})
	var rem [n]Obj
	for i := range rem {
		rem[i] = sc.obj(id["Opt"], 0)
		sc.s.place(rem[i], kop.ZoneExile, false)
		sc.s.g.rem[i] = rem[i]
	}
	sc.s.g.nrem = n
	m11Start(sc, synth, nil)
	if sc.s.Stalled() {
		t.Fatalf("stalled (%s) posing twelve candidates", StallName(sc.s.StallReason()))
	}
	d := m11FreeAsk(t, sc)
	if d.N != n+1 {
		t.Fatalf("%d options, want twelve cards and done", d.N)
	}
	picks := rem[:9]
	for _, o := range picks {
		d = m11FreeAsk(t, sc)
		m11Choose(t, sc, d, o)
	}
	if int(sc.s.g.nfq) != len(picks) || int(sc.s.g.nfree) != n-len(picks) {
		t.Fatalf("nfq %d nfree %d, want %d queued and %d still offered", sc.s.g.nfq, sc.s.g.nfree, len(picks), n-len(picks))
	}
	for i, o := range picks {
		if sc.s.g.freeQ[i] != o {
			t.Fatalf("freeQ[%d] = %d, want %d (pick order)", i, sc.s.g.freeQ[i], o)
		}
	}
	d = m11FreeAsk(t, sc)
	sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
	if sc.s.Stalled() {
		t.Fatalf("stalled (%s) closing the offer", StallName(sc.s.StallReason()))
	}
	m11Drive(t, sc, func() bool {
		for _, o := range picks {
			if sc.s.objs[o].zone == kop.ZoneExile {
				return false
			}
		}
		return true
	})
	for _, o := range picks {
		if z := sc.s.objs[o].zone; z == kop.ZoneExile {
			t.Fatalf("picked card %d was never cast (still in exile)", o)
		}
	}
	for _, o := range rem[len(picks):] {
		if z := sc.s.objs[o].zone; z != kop.ZoneExile {
			t.Fatalf("unpicked card %d left exile (zone %d)", o, z)
		}
	}
}
