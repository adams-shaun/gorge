package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// S:Mode$ MustBlock | ValidCreature$ <filter> ("CARDNAME blocks each combat
// if able", CR 509.1a; gorge combat.MustBlockCandidates): each creature
// matching Filter, resolved with the static's controller and source, is a
// required blocker this combat. ValidCard$ is never read there, so a card
// spelling it must not lower: gorge's matcher would bind nothing and the
// kernel's blockDuty would. The kernel reads it live in blockDuty beside the
// ExtBlocksIfAble mark (dirac/static_mustblock.go), so the lowering carries
// only the filter; the shared gates (Condition$, IsPresent$) ride Cond2.
func init() {
	staticMode("MustBlock", (*staticCtx).modeMustBlock)
}

func (sc *staticCtx) modeMustBlock() ([]lStatic, bool, error) {
	sp := sc.sp
	var err error
	if _, ok := sp.get("ValidCreature"); ok {
		if sc.st.F, err = sc.readF("ValidCreature"); err != nil {
			return nil, true, err
		}
	}
	if _, ok := sp.get("ValidCard"); ok {
		return nil, true, refuse("MustBlock ValidCard %q", sp.str("ValidCard"))
	}
	sp.consume("ValidCreature", "Secondary")
	return nil, false, nil
}

// mustBlockAbility builds the MustBlock static (kop.StatMustBlock): Filter is
// ValidCreature$, read on the battlefield.
func (cb *cardBuilder) mustBlockAbility(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	st := &ka.Static
	var err error
	if st.Cond2, err = cb.cond(s.Cond); err != nil {
		return ka, nil, err
	}
	st.OvGate = s.OvGate
	st.Kind = kop.StatMustBlock
	if s.F != nil {
		if st.Filter, err = cb.filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	}
	return ka, nil, nil
}
