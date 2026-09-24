package cards

import "testing"

// TestSplitGrantNames pins the exported Add*$ grant-name splitter directly
// (the K:Class: grant path already exercised it indirectly): Forge joins
// several SVar names with " & ", whitespace is trimmed, empty members are
// dropped, and a single name is unchanged.
func TestSplitGrantNames(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"A & B", []string{"A", "B"}},
		{"  A   &   B  ", []string{"A", "B"}},
		{"A &  & B", []string{"A", "B"}},
		{"A", []string{"A"}},
		{"", nil},
		{" & ", nil},
	}
	for _, tc := range cases {
		got := SplitGrantNames(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("SplitGrantNames(%q) = %q, want %q", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("SplitGrantNames(%q) = %q, want %q", tc.in, got, tc.want)
				break
			}
		}
	}
}
