package cost

import "testing"

// TestTapXTypeSharesCreatureTypeParse pins Forge's literal Weight of
// Conscience cost token: the bare sharesCreatureTypeWith term is a relation
// between the tapped creatures, so the parser moves it out of the per-object
// spec into CostPart.SameCreatureType (the ordinary matcher would read it
// against the Aura source, which has no creature type). The description is
// kept for the prompt, FormatCost spells the predicate back, and an ordinary
// tapXType is unaffected.
func TestTapXTypeSharesCreatureTypeParse(t *testing.T) {
	const token = "tapXType<2/Creature.sharesCreatureTypeWith/creatures that share a creature type>"
	c := ParseCost(token)
	if len(c.Unknown) != 0 || len(c.TapPermanent) != 1 {
		t.Fatalf("%q parsed to %+v", token, c)
	}
	p := c.TapPermanent[0]
	if p.N != 2 || p.Spec != "Creature" || !p.SameCreatureType || p.Desc != "creatures that share a creature type" {
		t.Fatalf("part = %+v, want N=2 Spec=Creature SameCreatureType=true with its description kept", p)
	}
	formatted := FormatCost(c)
	if formatted != token {
		t.Fatalf("FormatCost = %q, want %q", formatted, token)
	}
	back := ParseCost(formatted)
	if !back.Equal(&c) {
		t.Fatalf("%q re-parsed to %+v, want %+v", formatted, back, c)
	}
	plain := ParseCost("tapXType<2/Creature>")
	if len(plain.TapPermanent) != 1 || plain.TapPermanent[0].SameCreatureType {
		t.Fatalf("plain tapXType<2/Creature> parsed to %+v", plain.TapPermanent)
	}
	if !plain.Equal(&Cost{TapPermanent: []CostPart{{N: 2, Spec: "Creature"}}}) {
		t.Fatalf("plain tapXType<2/Creature> = %+v", plain)
	}
}
