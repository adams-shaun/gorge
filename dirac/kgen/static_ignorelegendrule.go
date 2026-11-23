package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// S:Mode$ IgnoreLegendRule (ValidCard$ <filter>, optional IsPresent$/
// PresentCompare$ gate; Mirror Gallery, Council of Reeds, Brothers Yamazaki;
// CR 704.5j): a permanent matching Filter is exempt from the legend rule
// while the gate holds (gorge legendGroups skips exempt candidates from the
// duplicate scan). The kernel reads it live in legendGroup's candidate walk,
// gated by useLgExempt (dirac/static_ignorelegendrule.go).
func init() {
	staticMode("IgnoreLegendRule", (*staticCtx).modeIgnoreLegendRule)
}

func (sc *staticCtx) modeIgnoreLegendRule() ([]lStatic, bool, error) {
	sp := sc.sp
	var err error
	if _, ok := sp.get("ValidCard"); ok {
		if sc.st.F, err = sc.readF("ValidCard"); err != nil {
			return nil, true, err
		}
	}
	sp.consume("ValidCard", "Secondary")
	return nil, false, nil
}

// ignoreLegendRuleAbility builds the IgnoreLegendRule static
// (kop.StatIgnoreLegendRule): Filter ValidCard$, read on the battlefield.
func (cb *cardBuilder) ignoreLegendRuleAbility(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	st := &ka.Static
	var err error
	if st.Cond2, err = cb.cond(s.Cond); err != nil {
		return ka, nil, err
	}
	st.OvGate = s.OvGate
	st.Kind = kop.StatIgnoreLegendRule
	if s.F != nil {
		if st.Filter, err = cb.filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	}
	return ka, nil, nil
}
