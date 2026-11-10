package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ojer Axonil, Deepest Might's damage tally.
//
// tallyRedNC records the noncombat damage a red source deals, for
// ExRedNCDmg (Temple of Power's "red sources you controlled dealt 4 or more
// noncombat damage this turn"). gorge counts each landed Damage event (post
// replacement and prevention, Amount > 0) from the source's
// DamageDealtThisTurn ledger, matching Card.Red+YouCtrl against the source's
// colours and controller as they were when the hit landed. replDamage is the
// one funnel of noncombat damage whose site passes the source, so the tally
// is taken there once the amount is final. The ReplDmgRaise form of
// ReplDamagePlus lives in replPlus (trig_oneormore.go).

// tallyRedNC adds n noncombat damage to the tally of srcCtrl when the
// source is red now (its colours as the hit lands).
//
//go:noinline
func (s *State) tallyRedNC(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, n int) {
	if s.srcColors(k, srcSl, srcCard)&kop.ColR == 0 {
		return
	}
	t := &s.g.redNC[SeatIx(srcCtrl)]
	*t = uint8(min(int(*t)+n, 255))
}
