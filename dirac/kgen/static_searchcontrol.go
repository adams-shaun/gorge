package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The SearchControl static build arm (lower_searchcontrol.go): the kind is
// the whole record; the kernel's search pick reads it (zone_searchcontrol.go).
func init() {
	registerStaticBuild("SearchControl", func(_ *cardBuilder, _ lStatic, st *kop.Static) ([]uint8, error) {
		st.Kind = kop.StatSearchControl
		return nil, nil
	})
}
