package effects

import "testing"

// TestMayPlayEffectParamsDontGrantZone: the Effect route's free grant reads
// ValidSA$ Spell and MayPlayDontGrantZonePermissions$ True (World War Hulk)
// and nothing else of those keys; the printed route and the plain grant keep
// refusing them.
func TestMayPlayEffectParamsDontGrantZone(t *testing.T) {
	base := func(extra map[string]string) map[string]string {
		m := map[string]string{
			"MayPlay":                "True",
			"MayPlayWithoutManaCost": "True",
			"Affected":               "Creature.Red,Creature.Green",
			"AffectedZone":           "Hand,Graveyard,Library,Exile,Command",
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	both := map[string]string{"ValidSA": "Spell", "MayPlayDontGrantZonePermissions": "True"}
	grant, ok := mayPlayFreeGrantFromLine(base(both))
	if !ok {
		t.Fatal("free grant with ValidSA$ Spell + MayPlayDontGrantZonePermissions$ True was refused")
	}
	if !grant.MayPlayNoZone || !grant.MayPlayFree || !grant.MayPlay {
		t.Fatalf("registered grant = %+v, want MayPlay+MayPlayFree+MayPlayNoZone", grant)
	}
	// The bit alone (Weftwalking-shaped) registers too; ValidSA is optional.
	if g, ok := mayPlayFreeGrantFromLine(base(map[string]string{"MayPlayDontGrantZonePermissions": "True"})); !ok || !g.MayPlayNoZone {
		t.Fatalf("bit-only free grant = %+v ok=%v", g, ok)
	}
	// A plain free grant carries no bit.
	if g, ok := mayPlayFreeGrantFromLine(map[string]string{"MayPlay": "True", "MayPlayWithoutManaCost": "True",
		"Affected": "Card.IsRemembered", "AffectedZone": "Exile"}); !ok || g.MayPlayNoZone {
		t.Fatalf("plain free grant = %+v ok=%v, want no bit", g, ok)
	}
	for _, bad := range []map[string]string{
		{"ValidSA": "Spell", "MayPlayDontGrantZonePermissions": "False"},
		{"ValidSA": "Spell", "MayPlayDontGrantZonePermissions": ""},
		{"ValidSA": "Spell", "MayPlayDontGrantZonePermissions": "true"},
		{"ValidSA": "Spell.Equipment", "MayPlayDontGrantZonePermissions": "True"},
		{"ValidSA": "Activated", "MayPlayDontGrantZonePermissions": "True"},
		{"ValidSA": "", "MayPlayDontGrantZonePermissions": "True"},
		{"ValidSA": "Spell"}, // ValidSA without the bit: a land play is not limited
		{"MayPlayDontGrantZonePermissions": "Yes"},
	} {
		if _, ok := mayPlayFreeGrantFromLine(base(bad)); ok {
			t.Errorf("free grant accepted %v", bad)
		}
	}
	// The plain grant (no MayPlayWithoutManaCost$) and the printed route refuse both keys.
	plain := map[string]string{"MayPlay": "True", "Affected": "Creature", "AffectedZone": "Graveyard", "MayPlayDontGrantZonePermissions": "True"}
	if _, ok := mayPlayGrantFromLine(plain); ok {
		t.Error("plain Effect grant accepted MayPlayDontGrantZonePermissions$")
	}
	if _, _, _, _, ok := MayPlayStaticParams(plain); ok {
		t.Error("printed route accepted MayPlayDontGrantZonePermissions$")
	}
	if _, _, _, _, ok := MayPlayStaticParams(map[string]string{"MayPlay": "True", "Affected": "Creature", "AffectedZone": "Graveyard", "ValidSA": "Spell"}); ok {
		t.Error("printed route accepted ValidSA$")
	}
}
