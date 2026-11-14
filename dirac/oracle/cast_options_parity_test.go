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
		// The same Voyager Drake beside Gemhide Sliver's granted "{T}: Add one
		// mana of any color" (a Sliver is a source only through its grant):
		// gorge's planner census has no granted source, so the composed plan
		// cannot pay the four mana, and no prefix search runs to tap the grant
		// for a plain Multikicker cast. The kernel counted the Sliver and
		// offered the cast.
		"multikicker_plain_cast_needs_granted_mana",
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
		// Nyxborn Hydra bestowed for {G}{G} at X 0 with one green land and
		// Wall of Roots: the counter-cost {G} is a census gap, and gorge's
		// prefix search activates it for a bestowed cast as for any other
		// potential play, then offers the cast (CR 702.103a, 601.2g).
		"bestow_cast_takes_counter_mana_prefix",
		"bestow_cast_takes_counter_mana_prefix_full",
		// A targeting spell beside an opponent's battlefield RaiseCost with a
		// ValidTarget$ (Kasmina, Enigmatic Mentor; Esior, Wardwing
		// Familiar): gorge's cast planner declines the witness whatever the
		// targets, so the autopay surface does not offer the cast from the
		// lands (CR 601.2f). The kernel priced it as a planned cast.
		"board_target_raise_declines_planned_cast",
		"board_target_raise_declines_planned_cast_commander",
		// Axebane Guardian's X mana in any combination of colours: gorge's
		// cast planner has no colour list for a Combo Any, so it is no unit
		// of its census, and a creature only that mana pays is not offered
		// on the autopay surface (CR 605.1a, 601.2g).
		"combo_any_evaluated_mana_unplanned",
		"combo_any_evaluated_mana_unplanned_second",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name) // on its own pool's table and recorded surface
		})
	}
}
