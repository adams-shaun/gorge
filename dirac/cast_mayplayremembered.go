package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// mayPlayRemembered is OpMayPlay over the remembered set (B=1: the
// Effect's ExileOnMoved$ lifetime, exact for one remembered card only): gorge's Effect
// registers its static with the whole Remembered list (effectRemembered),
// so every remembered card still in zone C gets the permission, in
// remember order.
//
//go:noinline
func (s *State) mayPlayRemembered(k *prep, in *kop.Instr, ctrl uint8) {
	set, n := &s.g.rem, int(s.g.nrem)
	if in.Obj == kop.RefChosen {
		set, n = &s.g.chosen, int(s.g.nchosen) // the imprint (act_exilelandchoose.go)
	}
	if in.B.N&kop.MayPlayExileOnMoved != 0 && n > 1 {
		// ExileOnMoved$: gorge ends the whole effect when one remembered
		// card leaves; per-card permissions cannot follow that
		s.capFull(StallExileOnMoved)
		return
	}
	headed := false // the first limited record heads the grant (cast_mayplaylimit.go)
	for i := 0; i < n; i++ {
		o := set[i]
		if s.objs[o].zone != kop.Zone(in.C) {
			continue
		}
		if in.B.N&kop.MayPlayNonLand != 0 && k.t.Cards[s.objs[o].card].Types&kop.TLand != 0 {
			continue
		}
		if in.B.N&kop.MayPlayInstSorc != 0 && k.t.Cards[s.objs[o].card].Types&(kop.TInstant|kop.TSorcery) == 0 {
			continue
		}
		if s.g.nmay >= maxMay {
			s.capFull(StallMayPlay)
			return
		}
		mk := uint8(in.B.N)&kop.MayPlayKinds | uint8(in.B.N>>8)&kop.MayPlayAdvKind
		s.g.may[s.g.nmay] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: ctrl, until: grantUntil(uint8(in.A.N), mk, &headed), kind: mk, turn: s.turn}
		if in.B.N&kop.MayPlayOwner != 0 {
			s.partitionRecord(&s.g.may[s.g.nmay], o, ctrl, int(in.B.N)>>kop.MayPlayRaiseShift&7) // cast_whileexiled.go
		}
		s.g.nmay++
	}
}
