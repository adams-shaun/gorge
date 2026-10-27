package effects

import "testing"

// TestMayPlayStaticParamsReadsPrintedGateKeys pins the printed-route whitelist
// for the keys the static walk already evaluated (IsPresent$ family) and the
// MayLookAt$ look the layers.go look branch registers from the same line.
func TestMayPlayStaticParamsReadsPrintedGateKeys(t *testing.T) {
	printed := func(params map[string]string) bool {
		_, _, _, _, ok := MayPlayStaticParams(params)
		return ok
	}
	cases := []struct {
		name   string
		params map[string]string
		want   bool
	}{
		{"real Case line", map[string]string{
			"MayPlay": "True", "Affected": "Land.TopLibrary+YouCtrl", "AffectedZone": "Library",
			"IsPresent": "Card.Self+IsSolved", "MayLookAt": "You", "Description": "x"}, true},
		{"indomitable gate", map[string]string{
			"MayPlay": "True", "IsPresent": "Pirate.YouCtrl+tapped", "PresentCompare": "GE3"}, true},
		{"blank IsPresent", map[string]string{"MayPlay": "True", "IsPresent": ""}, false},
		{"blank PresentCompare", map[string]string{"MayPlay": "True", "IsPresent": "Zombie", "PresentCompare": " "}, false},
		{"blank PresentZone is the battlefield", map[string]string{"MayPlay": "True", "IsPresent": "Zombie", "PresentZone": ""}, true},
		{"MayLookAt", map[string]string{"MayPlay": "True", "MayLookAt": "You"}, true},
		{"blank MayLookAt", map[string]string{"MayPlay": "True", "MayLookAt": ""}, false},
		{"CheckSVar stays refused", map[string]string{"MayPlay": "True", "CheckSVar": "X"}, false},
		{"free cast stays refused", map[string]string{"MayPlay": "True", "MayPlayWithoutManaCost": "True"}, false},
	}
	for _, c := range cases {
		if got := printed(c.params); got != c.want {
			t.Errorf("%s: MayPlayStaticParams ok = %v, want %v", c.name, got, c.want)
		}
	}
	// The Effect route has no gate evaluator and no look branch: both stay refused.
	for _, params := range []map[string]string{
		{"MayPlay": "True", "IsPresent": "Card.Self+IsSolved"},
		{"MayPlay": "True", "PresentCompare": "GE3"},
		{"MayPlay": "True", "MayLookAt": "You"},
	} {
		if _, ok := mayPlayGrantFromLine(params); ok {
			t.Errorf("Effect route accepted %v", params)
		}
	}
}
