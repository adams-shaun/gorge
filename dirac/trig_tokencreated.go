package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// --- TokenCreatedOnce once per token batch ---
//
// gorge (trigmatch/zone.go tokenCreatedMatches, trigger_match.go
// checkFaceTriggers) matches Mode$ TokenCreatedOnce on each minted token's
// TokenCreate event (ValidToken$ against the token, ValidPlayer$ against its
// creator) and latches the line once per token batch: one effects.Resolve
// (a spell's or ability's whole resolution, nested ones sharing the outer
// bracket); outside a resolution each token is its own batch. A copy token
// (events.CopyToken, DB$ CopyPermanent / Myriad) is minted in the library and
// its Library -> Battlefield MoveZone is its creation (gorge, CR
// 111.1): it fires too, with its owner as the creator. The kernel
// fires TrigTokenCreated as each OpCreateToken(Ex) / amass / copy token enters
// (no slot subscription: a battlefield scan behind genPrep.anyTokCr) and,
// for a Tf2Batch line inside a resolution, skips a line already pending:
// the pending queue holds only this resolution's triggers (they are put on
// the stack before anything resolves).

// fireTokenCreated fires the TokenCreated lines for the token at sl.
//
//go:noinline
func (s *State) fireTokenCreated(k *prep, sl Slot) {
	o, p := s.perms[sl].obj, s.perms[sl].ctrl
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, ls := range ord[:n] {
		s.slotTriggers(k, ls, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigTokenCreated }, func(l trigListener, a *kop.Ability) {
			if a.Trig.Flags2&kop.Tf2Batch != 0 && s.res.active && s.pendingLine(l) {
				return // already queued in this resolution's token batch
			}
			if !whoseOK(a.Trig.Whose, l.ctrl, p) {
				return
			}
			s.objTrigGen(k, kop.TrigTokenCreated, l, a, sl, o, p, 0)
		})
	}
}

// pendingLine: listener l's line is in the pending queue.
func (s *State) pendingLine(l trigListener) bool {
	for i := 0; i < int(s.npend); i++ {
		if p := &s.pend[i]; p.obj == l.obj && p.card == l.card && p.ab == l.ab {
			return true
		}
	}
	return false
}
