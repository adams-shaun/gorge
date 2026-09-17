package cards

import "testing"

func TestProducedCountsReusesNormalization(t *testing.T) {
	for _, tc := range []struct {
		src string
		max float64
	}{
		{"W U", 1},
		{"{W} {U}", 3},
	} {
		t.Run(tc.src, func(t *testing.T) {
			allocs := testing.AllocsPerRun(100, func() {
				counts, any := ProducedCounts(tc.src)
				if counts != [6]int32{1, 1, 0, 0, 0, 0} || any {
					t.Fatalf("ProducedCounts(%q) = %v, %v", tc.src, counts, any)
				}
			})
			if allocs > tc.max {
				t.Fatalf("ProducedCounts(%q) allocated %.0f objects; budget %.0f", tc.src, allocs, tc.max)
			}
		})
	}
}
