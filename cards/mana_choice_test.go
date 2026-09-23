package cards

import "testing"

func TestProducedCountsReportsManaChoices(t *testing.T) {
	for _, tc := range []struct {
		produced string
		want     [6]int32
	}{
		{"Any", [6]int32{1, 1, 1, 1, 1, 0}},
		{"Combo Any", [6]int32{1, 1, 1, 1, 1, 0}},
		{"Combo R G", [6]int32{0, 0, 0, 1, 1, 0}},
		{"Chosen", [6]int32{1, 1, 1, 1, 1, 0}},
		{"ComboChosen", [6]int32{1, 1, 1, 1, 1, 0}},
		{"Combo R Chosen", [6]int32{1, 1, 1, 1, 1, 0}},
	} {
		got, any := ProducedCounts(tc.produced)
		if got != tc.want || !any {
			t.Errorf("ProducedCounts(%q) = %v, %v; want %v, true", tc.produced, got, any, tc.want)
		}
	}
}
