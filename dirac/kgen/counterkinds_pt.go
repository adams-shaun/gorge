package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The P/T counter kinds beyond +1/+1, -1/-1 and -2/-1 (CR 122.1a): the
// kernel's ptGen adds each kind's power and toughness (dirac/ctr_ptkinds.go).
func init() {
	for name, k := range map[string]kop.CounterKind{
		"P1P0": kop.CtrP1P0, // a +1/+0 counter
		"P0P1": kop.CtrP0P1, // a +0/+1 counter
		"P1P2": kop.CtrP1P2, // a +1/+2 counter
		"P2P2": kop.CtrP2P2, // a +2/+2 counter
		"M0M1": kop.CtrM0M1, // a -0/-1 counter
		"M0M2": kop.CtrM0M2, // a -0/-2 counter
		"M2M2": kop.CtrM2M2, // a -2/-2 counter
	} {
		counterKinds[name] = k
	}
}
