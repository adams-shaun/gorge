package feat

import "testing"

// The ids MageZero's Python vocab.feature_id gives (and its findings doc's
// decoded table: PassAbility 679880620, Cast Faerie Seer 2082092948).
func TestMZXFeatureIDMatchesMageZero(t *testing.T) {
	for s, want := range map[string]int32{
		"PassAbility": 679880620, "PLAYER": 499558809, "ABILITY": 1229966541, "name": 1391785934,
		"Faerie Seer": 2110090788, "Cast Faerie Seer": 2082092948, "{T}: Add {U}.": 2023366677,
		"LifeTotal": 142245513, "": 1614623469,
	} {
		if got := MZXFeatureID(s); got != want {
			t.Errorf("MZXFeatureID(%q) = %d, want %d", s, got, want)
		}
	}
}
