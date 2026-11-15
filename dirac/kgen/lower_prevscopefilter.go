package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// scopeFiltered lowers the ValidTarget$ / ValidSource$ of a scoped prevention
// line (effectPreventScope) when one of them is not a bit form: the damage
// source is matched against sf (a card filter), the recipient against the
// player bits (You, Opponent, Player) or tf (a card filter over permanents).
// A half that is a bit form (tok / sok) keeps its bits, a colour list excepted. fb always carries
// kop.PrevScopeFiltered, which tells the kernel to read sf and tf from the
// Effect's source card (dirac/repl_prevscopefilter.go).
func (lc *lowerCtx) scopeFiltered(vt, vs string, tb int, tok bool, sb int, sok bool) (fb int, sf, tf *lFilter, err error) {
	fb = kop.PrevScopeFiltered
	if tok {
		fb |= tb
	} else {
		f, err := lc.filter(vt)
		if err != nil {
			return 0, nil, nil, err
		}
		switch f.Players {
		case plYou:
			fb |= kop.PrevScopePlayer
		case plOpp:
			fb |= kop.PrevScopeOppPlayer
		case plBoth:
			fb |= kop.PrevScopeAnyPlayer
		}
		if len(f.Clauses) > 0 {
			c := *f
			c.Players = 0
			tf = &c
		}
	}
	if sok && sb>>kop.PrevScopeColourShift == 0 {
		// a colour list stays out of fb: those bits are the recipient bits here, and
		// the kernel reads no colours for a filtered record, so it goes through the filter
		fb |= sb
		return fb, nil, tf, nil
	}
	// the damage source may be "unblocked" (Snag): F2Unblocked reads a battlefield attacker
	old := lc.unblockedOK
	lc.unblockedOK = true
	f, err := lc.filter(vs)
	lc.unblockedOK = old
	if err != nil {
		return 0, nil, nil, err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return 0, nil, nil, refuse("Effect replacement ValidSource$ %q", vs)
	}
	return fb, f, tf, nil
}

// scopeFilterAgrees refuses a card whose program already holds a filtered
// scoped prevention that differs from in: the kernel finds a record's
// filters from the Effect's source card, so one card carries one pair.
func (lc *lowerCtx) scopeFilterAgrees(in lInstr) error {
	if in.C&kop.PrevScopeFiltered == 0 {
		return nil
	}
	same := func(o lInstr) bool {
		return o.C == in.C && o.F.String() == in.F.String() && o.F2.String() == in.F2.String()
	}
	progs := [][]lInstr{lc.ab.Prog}
	for _, a := range lc.out.Abilities {
		if a != lc.ab {
			progs = append(progs, a.Prog)
		}
	}
	for _, p := range progs {
		for _, o := range p {
			if o.Op == "PreventDamageScope" && o.C&kop.PrevScopeFiltered != 0 && !same(o) {
				return refuse("Effect replacement: two scoped preventions with different filters")
			}
		}
	}
	return nil
}
