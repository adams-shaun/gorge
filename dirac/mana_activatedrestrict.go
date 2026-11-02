package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a mana ability's bare `Activated` RestrictValid$ term
// (Guidelight Optimizer, Purple Dragon Punks). Gorge's
// restrictValidTermMatches (pay/payment.go:606-607) admits every
// activated-ability payment with no card filter; kgen lowers the term as an
// any-card ManaRestrictAb, so restrActMatch admits any ability source in any
// zone. No kernel mechanism is added; this file holds the lockstep mutation.

// DebugManaRestrictActNone is the lockstep mutation switch of
// TestManarestrictMutation: restrSrcAdmits's pdAct arm answers false for an
// any-card ManaRestrictAb.
var DebugManaRestrictActNone bool

// restrActAnyCard: the activated restriction f is the match-anything filter.
func restrActAnyCard(k *prep, f kop.FilterID) bool {
	fl := &k.t.Filters[f]
	return fl.Or == 0 && *fl == kop.Filter{}
}
