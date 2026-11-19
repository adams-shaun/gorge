package dirac

import "testing"

// CR 602.2b: the twin cut mirrors gorge's potential play kind "ability": a
// printed activation and a gained one, but not a granted one, which gorge
// offers as the separate Kind "granted" and whose prefix search keeps the
// older node walk. The encoding is the only thing that tells them apart, so
// this pins it.
func TestIsAbilityPlayMirrorsGorgeKind(t *testing.T) {
	gained := MakeAction(AkActivate, 1, 2, gainedMode|3)
	granted := MakeAction(AkActivate, 1, 2, 0x80|3)
	if gained.Mode() == granted.Mode() {
		t.Fatalf("precondition: gained %#x and granted %#x modes must differ", gained.Mode(), granted.Mode())
	}
	for _, tc := range []struct {
		name string
		a    Action
		want bool
	}{
		{"printed", MakeAction(AkActivate, 1, 0, 0), true},
		{"printed-ability-3", MakeAction(AkActivate, 1, 0, 3), true},
		{"gained", gained, true},
		{"granted", granted, false},
		{"cast", MakeAction(AkCast, 1, 0, 0), false},
		{"pass", MakeAction(AkPass, 0, 0, 0), false},
	} {
		if got := isAbilityPlay(tc.a); got != tc.want {
			t.Errorf("%s: isAbilityPlay = %v, want %v", tc.name, got, tc.want)
		}
	}
}
