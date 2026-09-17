package rules

import "testing"

// A warm parse can allocate its token storage, but must not rebuild the
// immutable brace-normalization table on every offer-time cost check.
func TestParseCostReusesNormalization(t *testing.T) {
	for _, tc := range []struct {
		src string
		max float64
	}{
		{"U", 2},
		// Replacer owns both a byte buffer and the normalized string;
		// splitCostTokens owns a token buffer and a one-element slice.
		{"{U}", 4},
	} {
		t.Run(tc.src, func(t *testing.T) {
			allocs := testing.AllocsPerRun(100, func() {
				c := ParseCost(tc.src)
				if c.Colored[1] != 1 || c.CMC() != 1 {
					t.Fatalf("ParseCost(%q) = %+v; want one blue", tc.src, c)
				}
			})
			if allocs > tc.max {
				t.Fatalf("ParseCost(%q) allocated %.0f objects; budget %.0f", tc.src, allocs, tc.max)
			}
		})
	}
}
