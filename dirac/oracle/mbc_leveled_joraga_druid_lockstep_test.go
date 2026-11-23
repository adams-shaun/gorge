package oracle

import "testing"

// Ancient Brontodon ({6}{G}{G}) and Silvos ({3}{G}{G}{G}) beside a levelled
// Joraga Treespeaker (its "{T}: Add {G}{G}"), a counter-bearing Incubation
// Druid (three mana of one colour), two Swamps and a Forest: eight mana pay
// the Brontodon and gorge offers both casts (tail-55 pair 80, decision 726).
func TestMBCAncientBrontodonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "leveled_joraga_beside_reflected_druid_cast_offer")
}
