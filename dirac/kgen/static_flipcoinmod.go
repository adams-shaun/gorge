package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The FlipCoinMod static build arm (was an arm of (*cardBuilder).static in
// build_ab.go): the kind carries the whole record, the kernel's coin-flip
// modifier reads it (static_turnbits.go).
func init() {
	registerStaticBuild("FlipCoinMod", cbFlipCoinModStatic)
}

func cbFlipCoinModStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatFlipCoinMod
	return nil, nil
}
