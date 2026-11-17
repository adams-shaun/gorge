package kgen

import "testing"

// TestEffectTriggersAttackerBlockedForms: an end-of-turn DB$ Effect emblem on Mode$ AttackerBlocked.
func TestEffectTriggersAttackerBlockedForms(t *testing.T) {
	checkForms(t, []formCase{{card: "Barreling Attack", form: "EOT emblem on AttackerBlocked", lir: "CreateEmblem"}})
}
