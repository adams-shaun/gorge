package kop

import "testing"

// TestFrozenSentinels: each "one past the iota list" sentinel is fixed since
// the manifest cutover, and no other constant of its type takes its value:
// a new code is a manifest entry above it, never an append that would land
// on the sentinel. NumCounterKinds3 is the counter kinds' cover (moved once,
// 81 to 242).
func TestFrozenSentinels(t *testing.T) {
	for _, c := range []struct {
		typ, name string
		want      int64
	}{
		{"kop.Op", "OpCount", 156},
		{"kop.TrigEvent", "TrigEventCount", 51},
		{"kop.ReplKind", "ReplCount", 41},
		{"kop.StaticKind", "StatCount", 21},
		{"kop.ExprKind", "ExCount_", 62},
		{"kop.CondKind", "CondCount_", 13},
		{"kop.CounterKind", "NumCounterKinds3", 253},
	} {
		consts := typedConsts(t, c.typ)
		if got, ok := consts[c.name]; !ok || got != c.want {
			t.Errorf("%s = %d (declared %v), want the frozen %d", c.name, got, ok, c.want)
		}
		for n, v := range consts {
			// CtrMode is the Gate.Ctr sentinel at the cover's value: IsCounterKind
			// excludes it, so it is not a counter kind taking the sentinel.
			if n != c.name && n != "CtrMode" && v == c.want {
				t.Errorf("%s %s = %d takes frozen %s's value: a new code is a manifest entry (dirac/manifest)", c.typ, n, v, c.name)
			}
		}
	}
}
