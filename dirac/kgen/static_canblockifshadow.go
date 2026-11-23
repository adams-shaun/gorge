package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// S:Mode$ CanBlockIfShadow (ValidBlocker$ <filter>, ValidAttacker$ <filter>;
// Aether Web, Wall of Diffusion; CR 702.28b): a blocker matching Filter can
// block a shadow attacker matching Filter2 as though the shadow rule were
// absent (gorge combat.shadowBlockLifted). Filter is ValidBlocker$, Filter2
// ValidAttacker$; an absent filter matches any creature. The kernel reads it
// live in the shadow branch of the block check, gated by useCanBlkShadow
// (dirac/static_canblockifshadow.go).
func init() {
	staticMode("CanBlockIfShadow", (*staticCtx).modeCanBlockIfShadow)
}

func (sc *staticCtx) modeCanBlockIfShadow() ([]lStatic, bool, error) {
	sp := sc.sp
	var err error
	if _, ok := sp.get("ValidBlocker"); ok {
		if sc.st.F, err = sc.readF("ValidBlocker"); err != nil {
			return nil, true, err
		}
	}
	if _, ok := sp.get("ValidAttacker"); ok {
		if sc.st.F2, err = sc.readF("ValidAttacker"); err != nil {
			return nil, true, err
		}
	}
	sp.consume("ValidBlocker", "ValidAttacker", "Secondary")
	return nil, false, nil
}

// canBlockIfShadowAbility builds the CanBlockIfShadow static
// (kop.StatCanBlockIfShadow): Filter ValidBlocker$, Filter2 ValidAttacker$,
// both read on the battlefield.
func (cb *cardBuilder) canBlockIfShadowAbility(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	st := &ka.Static
	var err error
	if st.Cond2, err = cb.cond(s.Cond); err != nil {
		return ka, nil, err
	}
	st.OvGate = s.OvGate
	st.Kind = kop.StatCanBlockIfShadow
	if s.F != nil {
		if st.Filter, err = cb.filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	}
	if s.F2 != nil {
		if st.Filter2, err = cb.filter(s.F2, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	}
	return ka, nil, nil
}
