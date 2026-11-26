package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The ActivationLimit static build arm (was an arm of (*cardBuilder).static
// in build_ab.go): the kind carries the whole record, the kernel's activation
// limiter reads it (static_turnbits.go).
func init() {
	registerStaticBuild("ActivationLimit", cbActivationLimitStatic)
}

func cbActivationLimitStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	st.Kind = kop.StatActivationLimit
	return nil, nil
}
