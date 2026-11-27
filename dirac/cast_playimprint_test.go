package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFreePlayImprintPlayed: OpCastFree's C bit 15 (kgen's ImprintPlayed$
// flag) records the begun card in the resolution's imprinted set at the cast
// commit, the read Rashmi and Ragavan's ConditionDefined$ Imprinted gate and
// its MayPlay both make in the same chain.
func TestFreePlayImprintPlayed(t *testing.T) {
	c := uint16(kop.ZoneExile) | castFreeImprint
	sc, synth, id := m11Scen(t, []kop.Instr{{
		Op:  kop.OpCastFree,
		Obj: kop.RefRemembered,
		C:   int16(c),
	}})
	o := sc.obj(id["Opt"], 0)
	sc.s.place(o, kop.ZoneExile, false)
	sc.s.g.rem[0] = o
	sc.s.g.nrem = 1
	src := sc.perm(synth, 0)
	m11Start(sc, synth, func(e *stackEntry) { e.obj = src })
	d := m11FreeAsk(t, sc)
	m11Choose(t, sc, d, o)
	m11Drive(t, sc, func() bool { return sc.s.objs[o].zone == kop.ZoneStack })
	if sc.s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("the picked card zone = %d, want the stack (the cast never committed)", sc.s.objs[o].zone)
	}
	if sc.s.g.nimp != 1 || sc.s.g.imp[0] != o {
		t.Fatalf("imprinted set %v (n %d), want [%d]: the free cast did not record ImprintPlayed$", sc.s.g.imp[:sc.s.g.nimp], sc.s.g.nimp, o)
	}
}
