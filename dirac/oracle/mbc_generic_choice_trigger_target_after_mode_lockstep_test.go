package oracle

import "testing"

// CR 603.3d with CR 601.2c: a triggered ability's controller chooses its
// targets when putting it on the stack, and the GenericChoice's own choice
// is an instruction followed at resolution (CR 608.2d; 701aa4756f moved it
// there), so the target ask comes first and the modes ask at resolution.
// Inspirit, Flagship Vessel's granted trigger is a GenericChoice whose own
// ValidTgts$ ("up to one other target artifact") sits on the choice, not
// inside either mode, and its chosen mode's Defined$ Targeted reads that
// ability target. The recording first pinned a shape where gorge asked the
// mode at placement and then resolved without ever offering the target, so
// the kernel's target ask (then decision 134, tail-38 focus game) had no
// gorge counterpart; it is re-aimed to the resolution ask (target at
// decision 133, mode at 136).
//
// The same table holds the recordings added with the gate-xcount and Orgg
// fixes; one MBC table per set, so later causes add rows here.
//
// CR 107.4a with CR 601.2g-h: Baldur's Gate's "{2}, {T}: Add X mana of any
// one color, where X is the number of other Gates you control" produces
// nothing while its controller owns no other Gate (here X is 0). Two lockstep
// divergences followed. The kernel's filter-gap census listed the zero
// ability, so its prefix search tapped the Gate for a fee it did not pay and
// funded a filter from the Gate where gorge taps the Mountain (first
// recording); and its window treated the zero ability as a source of the owed
// colour, tapping the Gate for a {C} it could not spend (second recording).
// Both were recorded off the tail-51 focus pool, pair 3.
//
// CR 508.1 with Orgg's "can't attack if defending player controls an untapped
// creature with power 3 or greater": the Orgg recording is the old tail-38
// game (pair 72) where the defender's Ancient Ooze is an untapped 0/0 printed
// CDA whose layer-derived power is 6. gorge's UnlessDefender$ matcher read the
// printed face (0) instead of the derived power, so it offered Orgg where the
// kernel correctly withheld it.
func TestMBCTail38ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "generic_choice_trigger_target_after_mode")
	stdReproNamed(t, "gate_xcount_zero_leaves_fixed_mana_floating")
	stdReproNamed(t, "gate_xcount_zero_fee_window_over_tap")
	stdReproNamed(t, "orgg_unless_defender_cda_power")
}
