package oracle

import "testing"

// CR 701.3d with CR 111.7 and 700.4: when a token bearer leaves the
// battlefield, the attachment becomes unattached from it, but the "from a
// permanent" half of Grafted Wargear's Mode$ Unattached line (ValidObject$
// Permanent) no longer matches: the token ceased to exist at the same
// checkpoint (tail-29 pair 182, decision 630). The kernel queued the
// equipment's Unattached trigger beside the copy token's Dies trigger and
// asked a trigger order, where gorge had already pushed the Dies trigger
// alone.
func TestMBCUnattachedTriggerTokenBearerLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unattached_trigger_token_bearer")
}
