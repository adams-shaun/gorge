package oracle

import "testing"

// Six evloop cards that re-blocked every cycle on one projection field each
// (Cards.Power, Cards.Counters, Cards.Chars.Subtypes, Life). Each recording
// is the bad game's prefix up to its first divergence.

// CR 604.3 with CR 613.4: Angry Mob's two characteristic-defining statics are
// conditioned (Condition$ PlayerTurn / NotPlayerTurn). The kernel applied both
// unconditionally, the 2/2 one last, so on its controller's turn it was 3/3
// under a +1/+1 anthem where gorge (whose CDA whitelist leaves a conditioned
// CDA to the battlefield static scan, which reads the condition) had 4/4
// (tail-09 pair 91, decision 373).
func TestEvloopAngryMobConditionedCDALockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "conditioned_cda_pt_reads_its_turn_condition")
}

// Phyrexian Ingester's +X/+Y reads its imprinted creature. While a granted
// upkeep trigger of the Ingester resolved (Vile Consumption), the kernel's
// static read took the resolving entry's empty imprint set instead of the
// Ingester's kept one and dropped to 3/3; gorge kept 5/5 (dr-mbc-aaab pair
// 52, decision 725).
func TestEvloopPhyrexianIngesterImprintPTLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "imprint_static_pt_survives_own_trigger_resolution")
}

// CR 707.9b: Dino DNA's copy is "a 6/6 green Dinosaur creature"
// (RemoveCardTypes$ + RemoveSubTypes$). The kernel replaced the card types but
// only added Dinosaur to the copied creature's subtypes, so a Thoughtpicker
// Witch copy stayed a Human Wizard (tail-28 pair 93, decision 485).
func TestEvloopDinoDNACopySubtypesLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copy_exception_replaces_copied_subtypes")
}

// CR 702.44a with CR 702.43a: Modular—Sunburst enters with a +1/+1 counter
// for each colour spent. gorge's Modular expansion inlined Count$Converge in
// CounterNum$, where the cast's converge gate does not look, so it recorded
// no colours and Arcbound Wanderer died as a 0/0; the kernel counted two
// (tail-46 pair 97, decision 392).
func TestEvloopArcboundWandererSunburstLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "modular_sunburst_counts_colours_spent")
}

// Bogardan Phoenix's dies trigger returns it and puts a death counter on
// Defined$ TriggeredCard. gorge read the returned card as a departed source
// and put no counter, so the Phoenix returned on every death; the kernel (and
// XMage's BogardanPhoenixEffect) puts the counter (tail-76 pair 86, decision
// 670).
func TestEvloopBogardanPhoenixDeathCounterLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dies_return_puts_counter_on_returned_card")
}

// Time Reaper puts a face-up exiled card on the bottom of its owner's library
// with RememberChanged$, then gains 3 life if it did (ConditionDefined$
// Remembered). The kernel moved the card but remembered nothing, so it gained
// no life where gorge did (tail-62 pair 8, decision 941).
func TestEvloopTimeReaperRememberedLibraryMoveLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_to_library_bottom_remembers_for_if_you_do")
}
