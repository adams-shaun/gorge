package oracle

import "testing"

// CR 605.3b with CR 602.2: Mercurial Pretender enters as a copy of Flamestick
// Courier and carries the granted "{2}{U}{U}: Return this creature to its
// owner's hand". With one Island untapped, three Mountains, a Sandstone Needle
// and a Signpost Scarecrow ("{2}: add one mana of any colour") the second {U}
// is the Scarecrow paid with two red, so the bounce is affordable and priority
// offers it (tail-45 pair 220, seed 2, decision 556).
func TestMBCMercurialPretenderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_bounce_funded_by_fee_filter")
}
