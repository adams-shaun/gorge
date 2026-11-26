package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The Override static build arm (was an arm of (*cardBuilder).static in
// build_ab.go): a layer-7 copy effect over Filter (kop.StatOverride; the
// kernel's overrides_gen.go). Not cb.staticOverride, the Continuous grant's
// RemoveKeyword$ override.
func init() {
	registerStaticBuild("Override", cbOverrideStatic)
}

func cbOverrideStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatOverride
	if s.F != nil {
		fid, err := cb.filter(s.F, kop.ZoneBattlefield)
		st.Filter = fid
		if err != nil {
			return nil, err
		}
	}
	i, err := cb.override(s.Override)
	if err != nil {
		return nil, err
	}
	st.Override = uint16(i + 1)
	// the chosen land-type setter's read (Convincing Mirage's AddType$
	// ChosenType): the static reads its source's perm.chosen
	// (act_choosetype.go chosenGrant); any other static kind's AddChosen
	// lowering is the Continuous case's
	st.AddChosen = s.AddChosen
	return nil, nil
}
