package rules

import (
	"strings"
	"testing"
)

// TestParamCensusReadsNumLoyaltyActGrants: loyaltyAbilityLimit's
// NumLoyaltyAct fold reads Twice$ and Additional$ (CR 606.3's "unless an
// effect says otherwise"); the census must see both, so Kaito, Dancing
// Shadow and the Urza walkers are not reported as carrying an unread
// stat:NumLoyaltyAct.Twice.
func TestParamCensusReadsNumLoyaltyActGrants(t *testing.T) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)
	reads := d.stat["NumLoyaltyAct"]
	if !reads["ValidCard"] {
		t.Fatalf("stat:NumLoyaltyAct read set %v lacks ValidCard -- the bucket itself is not being derived", reads)
	}
	for _, key := range []string{"Twice", "Additional"} {
		if !reads[key] {
			t.Errorf("stat:NumLoyaltyAct read set %v lacks %s -- loyaltyAbilityLimit's read is invisible to the census", reads, key)
		}
	}
}

// TestParamCensusAcceptsRestrictParamsArgument: an Effect-delivered static's
// ce.RestrictParams handed to a named helper is a static Params map, not a
// rot-guard finding.
func TestParamCensusAcceptsRestrictParamsArgument(t *testing.T) {
	t.Parallel()
	s := scanPackages(t)
	s.rotGuard(t)
	if !s.mapAttributed["rules:Engine.numLoyaltyActFold:params"] {
		t.Fatalf("numLoyaltyActFold's params map was never attributed at a call site; mapAttributed has no entry for it")
	}
	for _, e := range s.guardErrs {
		if strings.Contains(e, "RestrictParams") {
			t.Errorf("rot guard rejects a RestrictParams argument: %s", e)
		}
	}
}
