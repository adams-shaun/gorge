package kop

import (
	"sort"
	"testing"
)

// TestRefValuesUnique: no two Ref constants share a value. The kernel decodes
// a ref by value (resolve.go playerRef, exprs.go), and TestKopOrdinalsUnchanged
// is keyed by name, so a reused ordinal would otherwise read one ref as another
// unnoticed (RefResChosenPlayer once took RefTaxedAttacker's 38).
func TestRefValuesUnique(t *testing.T) {
	cs := typedConsts(t, "kop.Ref")
	if len(cs) < 2 {
		t.Fatalf("found %d Ref constants; the scan missed the enum", len(cs))
	}
	names := make([]string, 0, len(cs))
	for n := range cs {
		names = append(names, n)
	}
	sort.Strings(names)
	seen := map[int64]string{}
	for _, n := range names {
		v := cs[n]
		if o, dup := seen[v]; dup {
			t.Errorf("%s and %s share value %d", o, n, v)
		}
		seen[v] = n
	}
}
