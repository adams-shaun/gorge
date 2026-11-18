package oracle

import "testing"

// A Burning Earth payment through a Taps-deferred Snowfield Sinkhole
// (Plains Swamp): the land's two intrinsic one-colour abilities make its
// deferral a prefix script gap, and the script plays the gap's activation
// before the play's ability. Gorge's scriptOrderAnswers hoists the play's
// needed colours only where an ask option carries a ManaSymbol, and a
// multi-ability source's "choose a mana ability" options never do
// (engine/rules/mana_activation.go sets it on the Combo-flattened and
// stage-2 colour options only), so gorge keeps the ask's engine order: the
// Sinkhole's Plains-side ability first, the Swamp-side colour coming from
// the next source's autopay. The kernel's scriptEngineAnswer mirrored
// needFirst unconditionally in that arm, floating the play's needed colour
// from the Sinkhole itself and leaving the pools one mana apart after the
// Ninjutsu activation this game pays for (decision 438 of 438: the kernel
// held one colour of the play's two for the other). The mirror now skips
// the hoist for a multi-ability fold (dirac/mana_multicolour.go), whose
// abilities kgen orders W U B R G C — mask order, i.e. the ask's own order.
// On the synth tail-71 pool the game came from (focus Hearth Kami, Ninjutsu
// cast), the 41-pair sweep runs absolute clean with the fix.
func TestMBCTapsDeferredMultiAbilityAskOrderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_multi_ability_ask_order")
}
