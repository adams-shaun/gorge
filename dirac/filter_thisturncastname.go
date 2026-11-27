package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwSharesNameThisTurnCast is kop.FwSharesNameThisTurnCast: the candidate
// shares a printed name with a spell cast this turn (gorge's
// `sharesNameWith ThisTurnCast` referent, the cast ledger's objects of any
// caster). The name identity is fwSharesName's; the referents are the cast
// ring's pushes (cast_castledger.go), every seat's, a reversed cast (CR 733.1)
// skipped as gorge's events.CastReversals skips it. The table that reads the
// word records the ring (tableReadsCastNames).
//
//kmanifest:fword FwSharesNameThisTurnCast
func (s *State) fwSharesNameThisTurnCast(c *fctx, _ *kop.FLit) bool {
	if c.fd {
		return false
	}
	cn := NoCard
	if c.perm() {
		cn = s.nameCardAt(c.sl)
	} else if c.c != NoCard {
		cn = s.nameKey(c.c)
	}
	if cn == NoCard {
		return false
	}
	for p := uint8(0); p < MaxSeats; p++ {
		for _, r := range s.castPushes(p) {
			if r.prov&castReversedPush != 0 {
				continue
			}
			if rn := s.objName(r.obj - 1); rn != NoCard && rn == cn {
				return true
			}
		}
	}
	return false
}

// tableReadsCastNames reports a table whose filter programs read the cast
// ring through FwSharesNameThisTurnCast: gen.go turns the ring on for it.
func tableReadsCastNames(t *kop.Table) bool {
	for i := range t.FilterProgs {
		for _, l := range t.FilterProgs[i].Lits {
			if l.W == kop.FwSharesNameThisTurnCast {
				return true
			}
		}
	}
	return false
}
