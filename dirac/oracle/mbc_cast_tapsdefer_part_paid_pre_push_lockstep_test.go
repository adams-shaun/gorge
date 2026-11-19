package oracle

import "testing"

// CR 601.2b, 605.1a: beside Manabarbs the untapped land a plain cast needs is
// a Taps-deferred gap whose tap queues a punishing trigger, so gorge's
// potential-play search proves the cast by activating its whole mana plan at
// its asks, before the spell is pushed (the Forest's trigger sits below it).
// The kernel's cast arm had no counterpart of the activation arm's
// payPrefixRest (Mana_resolutionpay.go): the prefix's skip left the
// Taps-deferred unit unpaid, and manaStep's seSpell payment tapped it at
// bsMana, after bsPush (decision 212 of pool tail-34, Spring Cleaning).
func TestMBCCastTapsdeferPartPaidPrePushReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_tapsdefer_part_paid_pre_push")
}
