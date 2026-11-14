package oracle

import "testing"

// TestCastOptionsParityLockstep replays MBC draft games whose first
// difference was a cast the kernel offered at priority and gorge did not.
// Each recording diverged at its last action before the kernel learned the
// rule its subtest names, and now plays through it and 200 decisions on.
// Each recording's pool is its MBC draft split, built from its own decks.
func TestCastOptionsParityLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		// Kavu Primarch (Kicker {4}, Convoke) with lands and no pool: gorge's
		// potential planner declines every credit-bearing kicked cast
		// (cost:credit), so only a floating pool that pays the kicked total
		// offers it (CR 702.33, 702.51a); the kernel offered it from the lands.
		"kicked_convoke_cast_lands_only",
		"kicked_convoke_cast_lands_only_full",
		// Ashaya's Enduring Bond with no legendary creature or planeswalker
		// on its caster's side: CR 205.4e withholds the cast, from the hand
		// and through a play permission alike.
		"legendary_sorcery_without_legend",
		"legendary_sorcery_without_legend_mayplay",
		// Voyager Drake (Multikicker {U}) payable only with a depletion
		// land's counter mana: gorge prices the plain cast by its composed
		// plan alone, which never runs the prefix search that taps such a
		// source, so the plain cast is not offered.
		"multikicker_plain_cast_needs_depletion_land",
		"multikicker_plain_cast_needs_depletion_skerry",
		// Eldrazi Spawn tokens beside Path of Annihilation's granted "{T}: Add
		// one mana of any color": gorge's cast planner leaves the grant out
		// of its census but keeps each token's printed sacrifice for {C}, a
		// last-resort unit, so a cast that sacrifice pays is planned (CR
		// 602.2b, 605.1a). The kernel dropped the tokens from that census.
		"spawn_sac_mana_beside_granted_tap_planned",
		"spawn_sac_mana_beside_granted_tap_planned_aura",
		// The same tokens for a kicked cast, which gorge offers on its
		// potential walk: an untapped Spawn taps for the granted colour and
		// is then sacrificed for {C}, two units, while a tapped one only
		// sacrifices (CR 602.2b, 605.1a). The kernel joined both into one.
		"kicked_cast_taps_granted_spawn_then_sacrifices",
		"kicked_cast_taps_granted_spawn_then_sacrifices_second",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name) // on its own pool's table and recorded surface
		})
	}
}
