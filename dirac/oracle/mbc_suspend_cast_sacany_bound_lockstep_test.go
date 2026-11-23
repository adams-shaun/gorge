package oracle

import "testing"

// TestMBCT45PowerArmorReprosLockstep replays the synthetic tail-64 game the
// evidence loop recorded: at p1's priority, Inevitable Betrayal sits in
// p1's hand with its suspend timing open ({1}{U}{U}, three time counters,
// CR 702.62a) and p1's mana is two Islands short of it but one Urza, Lord
// High Artificer activation short of its potential bound: the untapped
// artifacts bound the tap-another ability's repeats ("Tap an untapped
// artifact you control: Add {U}", potential_repeat_fee.go
// addPotentialTapRepeat), so gorge's PotentialMana admits the payment and
// offers the suspend special action beside the plain cast. The kernel's
// closed sacrifice-any bound never priced the action at all --
// grantPlayCost had a plot arm (grantPlotCost) but none for the suspend
// action -- so its bound check (sacPotPays) read the play unpriced and the
// script walk dropped the candidate (divergence at decision 361: options
// options, gorge's menu held the "suspend" cast the kernel's did not).
func TestMBCT45PowerArmorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "suspend_cast_sacany_bound")
}
