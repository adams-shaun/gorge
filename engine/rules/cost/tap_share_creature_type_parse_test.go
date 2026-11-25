package cost

// Weight of Conscience's cost token is the corpus's only literal tapXType
// carrier of Forge's bare share-a-creature-type relation. The relation is a
// property of the paid SET, so ParseCost moves it out of the per-object spec
// into CostPart.ShareCreatureType. Only the literal N form is stripped; the
// dynamic X/Any forms keep the term in the spec and fail closed as before.

import "testing"

func TestParseCostTapXTypeSharesCreatureType(t *testing.T) {
	t.Run("literal form strips the relation", func(t *testing.T) {
		c := ParseCost("tapXType<2/Creature.sharesCreatureTypeWith>")
		if len(c.Unknown) != 0 || len(c.TapPermanent) != 1 {
			t.Fatalf("parsed cost = %+v, want one tapXType part and no Unknown", c)
		}
		part := c.TapPermanent[0]
		if part.N != 2 || part.Spec != "Creature" || !part.ShareCreatureType {
			t.Fatalf("part = %+v, want N=2 Spec=Creature ShareCreatureType=true", part)
		}
		// The relation survives a format/parse round trip: FormatCost spells
		// it back and ParseCost strips it again.
		if got := FormatCost(c); got != "tapXType<2/Creature.sharesCreatureTypeWith>" {
			t.Fatalf("FormatCost = %q, want the Forge spelling", got)
		}
		back := ParseCost(FormatCost(c))
		if len(back.TapPermanent) != 1 || back.TapPermanent[0].Spec != "Creature" || !back.TapPermanent[0].ShareCreatureType {
			t.Fatalf("round trip = %+v, want the literal part with the flag", back)
		}
	})

	t.Run("predicate plus relation keeps the predicate", func(t *testing.T) {
		c := ParseCost("tapXType<2/Creature.Other+sharesCreatureTypeWith>")
		if len(c.TapPermanent) != 1 {
			t.Fatalf("parsed cost = %+v, want one tapXType part", c)
		}
		part := c.TapPermanent[0]
		if part.Spec != "Creature.Other" || !part.ShareCreatureType {
			t.Fatalf("part = %+v, want Spec=Creature.Other ShareCreatureType=true", part)
		}
		if got := FormatCost(c); got != "tapXType<2/Creature.Other+sharesCreatureTypeWith>" {
			t.Fatalf("FormatCost = %q, want the '+' spelling after a predicate", got)
		}
	})

	t.Run("dynamic X form keeps the term and the flag off", func(t *testing.T) {
		c := ParseCost("tapXType<X/Creature.sharesCreatureTypeWith>")
		if len(c.Unknown) != 0 || len(c.TapPermanent) != 1 {
			t.Fatalf("parsed cost = %+v, want one dynamic tapXType part", c)
		}
		part := c.TapPermanent[0]
		if part.Dyn != "X" || part.Spec != "Creature.sharesCreatureTypeWith" || part.ShareCreatureType {
			t.Fatalf("part = %+v, want the X form with the term left in Spec and no flag", part)
		}
	})

	t.Run("dynamic Any form keeps the term and the flag off", func(t *testing.T) {
		c := ParseCost("tapXType<Any/Creature.sharesCreatureTypeWith>")
		if len(c.Unknown) != 0 || len(c.TapPermanent) != 1 {
			t.Fatalf("parsed cost = %+v, want one dynamic tapXType part", c)
		}
		part := c.TapPermanent[0]
		if part.Dyn != "Any" || part.Spec != "Creature.sharesCreatureTypeWith" || part.ShareCreatureType {
			t.Fatalf("part = %+v, want the Any form with the term left in Spec and no flag", part)
		}
	})
}
