package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugVulcanWrongHolder is the mutation hook (TestVulcanMindMeldMutation):
// the opponent-exiler permission goes to the controller and the
// controller-exiler permission to the opponent, as a kernel that gave every
// exiled card to the wrong player would.
var DebugVulcanWrongHolder bool

// DebugMayPlayTargetLands is the mutation hook (TestSilasGraveLockstepMutation):
// the single-object OpMayPlay grant ignores MayPlayNonLand, so a targeted
// artifact land in the graveyard gets a play permission it must not have.
var DebugMayPlayTargetLands bool

// mayPlayRemembered is OpMayPlay over the remembered set (B=1: the
// Effect's ExileOnMoved$ lifetime, exact for one remembered card only): gorge's Effect
// registers its static with the whole Remembered list (effectRemembered),
// so every remembered card still in zone C gets the permission, in
// remember order.
//
//go:noinline
func (s *State) mayPlayRemembered(k *prep, in *kop.Instr, ctrl uint8) {
	set := s.g.rem[:s.g.nrem]
	if in.Obj == kop.RefChosen {
		set = s.g.chosen[:s.g.nchosen] // the imprint (act_exilelandchoose.go)
	} else if in.Obj == kop.RefImprinted {
		set = s.g.imp[:s.g.nimp] // Dig Imprint$ (zone_digimprint.go)
	}
	n := len(set)
	if in.B.N&kop.MayPlayExileOnMoved != 0 && n > 1 {
		// ExileOnMoved$: gorge ends the whole effect when one remembered
		// card leaves; per-card permissions cannot follow that
		s.capFull(StallExileOnMoved)
		return
	}
	if in.B.N&kop.MayPlayOpponent != 0 != DebugVulcanWrongHolder {
		ctrl = s.nextAlive(ctrl) // the permission is the exiler's: the controller's opponent
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
		if s.mayFull() {
			s.capFull(StallMayPlay)
			return
		}
		mk := uint8(in.B.N)&kop.MayPlayKinds | uint8(in.B.N>>8)&kop.MayPlayAdvKind
		*s.mayAt(int(s.g.nmay)) = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: ctrl, until: grantUntil(uint8(in.A.N), mk, &headed) | payLifeMark(in.B.N), kind: mk | payLifeKind(in.B.N), turn: mayWord(s.turn)}
		if in.B.N&kop.MayPlayOwner != 0 {
			s.partitionRecord(s.mayAt(int(s.g.nmay)), o, ctrl, int(in.B.N)>>kop.MayPlayRaiseShift&7) // cast_whileexiled.go
		}
		s.g.nmay++
	}
}
