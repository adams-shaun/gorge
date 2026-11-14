package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugTrigQualsFlip inverts every trigCmpOK, castTargetsPlayer and
// AttackedTarget$ You answer (the mutation test's lever; test-only).
var DebugTrigQualsFlip bool

// trigCmpOK is a trigger count gate (gorge CompareIntCount for
// ActivatorThisTurnCast$, ComparePresent for ValidAttackersAmount$): have
// against Trigger.N. A bare literal (N>>8 == 0) is the card's default
// operator, >= for the attackers gate (zeroIsGE) and == for the N-th spell
// gate; otherwise N is a kop.DmgGate (the comparison in the high byte).
//
//go:noinline
func trigCmpOK(have int, n int16, zeroIsGE bool) bool {
	var ok bool
	switch {
	case n>>8 != 0:
		ok = kop.DmgGatePass(n, have)
	case zeroIsGE:
		ok = have >= int(n)
	default:
		ok = have == int(n)
	}
	return ok != DebugTrigQualsFlip
}

// castTargetsPlayer is castTargetsAll's player-target arm (gorge
// targetMatchesTargetsValid's MatchesPlayerSpecFrom): the target player p
// matches an FPlayer alternative of the TargetsValid$ filter chain f when
// its controller selector names p relative to the trigger's controller.
//
//go:noinline
func (s *State) castTargetsPlayer(k *prep, f kop.FilterID, p uint8, ctrl uint8) bool {
	return s.castTargetsPlayerRead(k, f, p, ctrl) != DebugTrigQualsFlip
}

func (s *State) castTargetsPlayerRead(k *prep, f kop.FilterID, p uint8, ctrl uint8) bool {
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := &k.t.Filters[f]
		if fl.Flags&kop.FPlayer == 0 {
			continue
		}
		switch fl.Ctrl {
		case kop.CtrlAny:
			return true // a bare Player alternative (Ertha Jo's Player,Creature.inZoneBattlefield)
		case kop.CtrlYou:
			if p == ctrl {
				return true
			}
		case kop.CtrlOpp:
			if p != ctrl {
				return true
			}
		}
	}
	return false
}
