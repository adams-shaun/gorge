package effects

import "testing"

// animatePlayGrant reads an Animate staticAbilities$ body that grants a land
// permission from the graveyard or an extra land play, and nothing else.
func TestAnimatePlayGrant(t *testing.T) {
	ce, ok := animatePlayGrant(staticLineParams{"Mode": "Continuous", "Affected": "Land.YouOwn", "MayPlay": "True",
		"AffectedZone": "Graveyard", "EffectZone": "Battlefield", "Description": "x"})
	if !ok || !ce.MayPlay || ce.Affects != "Land.YouOwn" || ce.AffectedZone != "Graveyard" {
		t.Fatalf("may-play body: ok=%v ce=%+v", ok, ce)
	}
	ce, ok = animatePlayGrant(staticLineParams{"Mode": "Continuous", "Affected": "You", "AdjustLandPlays": "1", "Description": "x"})
	if !ok || ce.AdjustLandPlays != 1 || ce.Affects != "You" {
		t.Fatalf("extra land body: ok=%v ce=%+v", ok, ce)
	}
	for name, p := range map[string]staticLineParams{
		"another effect zone": {"Mode": "Continuous", "Affected": "Land.YouOwn", "MayPlay": "True", "AffectedZone": "Graveyard", "EffectZone": "Graveyard"},
		"a gated permission":  {"Mode": "Continuous", "Affected": "Land.YouOwn", "MayPlay": "True", "AffectedZone": "Graveyard", "CheckSVar": "X"},
		"no zone":             {"Mode": "Continuous", "Affected": "Land.YouOwn", "MayPlay": "True"},
		"a P/T body":          {"Mode": "Continuous", "Affected": "Card.Self", "SetPower": "3"},
		"unlimited lands":     {"Mode": "Continuous", "Affected": "You", "AdjustLandPlays": "Unlimited"},
	} {
		if _, ok := animatePlayGrant(p); ok {
			t.Errorf("%s: read as a play grant", name)
		}
	}
}
