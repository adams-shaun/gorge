package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// S:Mode$ IgnoreLandWalk ("creatures with swampwalk can be blocked as though
// they didn't have swampwalk", Crevasse, Great Wall, Lord Magnus, Street
// Savvy): gorge combat.landwalkIgnored skips a landwalk line for a blocker
// while an active static names it. ValidKeyword$ Landwalk:<spec> names the
// line by its land spec (absent: every landwalk line), ValidBlocker$ filters
// the would-be blocker against the static's source (absent: any blocker).
// The static carries the blocker filter in F and the line's land filter in
// Landwalk (kop.Static.Landwalk, as the grant statics carry theirs); the kernel
// (dirac/static_ignorelandwalk.go) compares it with each link of the
// attacker's landwalk filter.
func init() {
	staticMode("IgnoreLandWalk", (*staticCtx).modeIgnoreLandWalk)
}

func (sc *staticCtx) modeIgnoreLandWalk() ([]lStatic, bool, error) {
	sp := sc.sp
	sp.consume("Secondary")
	var err error
	if _, ok := sp.get("ValidBlocker"); ok {
		if sc.st.F, err = sc.readF("ValidBlocker"); err != nil {
			return nil, true, err
		}
	}
	if kw, ok := sp.get("ValidKeyword"); ok {
		head, param, _ := strings.Cut(strings.TrimSpace(kw), ":")
		if head != "Landwalk" {
			return nil, true, refuse("IgnoreLandWalk ValidKeyword$ %q", kw)
		}
		f, err := sc.lc.landwalkFilter(kw, param)
		if err != nil {
			return nil, true, err
		}
		if len(f.Clauses) != 1 {
			// the kernel compares one landwalk line (one Or-chain link) with the
			// attacker's links; gorge compares the whole spec string
			return nil, true, refuse("IgnoreLandWalk ValidKeyword$ %q names more than one land clause", kw)
		}
		sc.st.Landwalk = f
	}
	return nil, false, nil
}

// ignoreLandWalkAbility builds the IgnoreLandWalk static (kop.StatIgnoreLandWalk):
// Filter the blocker filter, Landwalk the land filter of the line it names.
func (cb *cardBuilder) ignoreLandWalkAbility(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	st := &ka.Static
	var err error
	if st.Cond2, err = cb.cond(s.Cond); err != nil {
		return ka, nil, err
	}
	st.OvGate = s.OvGate
	st.Kind = kop.StatIgnoreLandWalk
	if s.F != nil {
		if st.Filter, err = cb.filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	}
	if s.Landwalk != nil {
		if st.Landwalk, err = cb.filter(s.Landwalk, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	}
	return ka, nil, nil
}
