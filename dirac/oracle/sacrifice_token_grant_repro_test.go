package oracle

import "testing"

// TestSacrificeTokenGrantShortfallRepro pins pair 248 of dr-mbc-bb (Mothdust
// Changeling implicated): paying Ebony Fly's {4} roll with a Forest pair, an
// Eldrazi Spawn's sacrifice and Gemhide Sliver's grant. gorge's census counts
// the sacrifice-only token, so its grant prefix taps one Sliver; the kernel's
// census left the token to its last-resort phase and the prefix tapped a
// second Sliver (Mothdust Changeling, a changeling Sliver) instead.
func TestSacrificeTokenGrantShortfallRepro(t *testing.T) {
	stdReproNamed(t, "sacrifice_token_shortfall_takes_a_second_grant")
}
