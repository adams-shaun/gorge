package oracle

import "testing"

// TestAuraFallsOffGrantedColourProtection replays a game where Sword of Forge
// and Frontier is equipped to a Capital Guard wearing the red Aura Hammerhand.
// The Sword grants protection from red, so the Aura is attached to something
// it can no longer legally enchant and goes to the graveyard (CR 702.16c,
// 704.5m) in both engines.
func TestAuraFallsOffGrantedColourProtection(t *testing.T) {
	synthReproLockstep(t, "repro/aura_falls_off_granted_colour_protection", "tail-18", Config{GorgeAutopaySurface: true})
}
