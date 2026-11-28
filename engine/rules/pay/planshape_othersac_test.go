package pay

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
)

// TestPaymentPlanOtherSacTierShapes pins the closed shape set of the
// other-sacrifice plan tier (the Ashnod's Altar family): gorge's planner
// admits exactly the productions dirac's kernel mirrors (kop.OtherSacPlanAb
// over the card's FIRST mana ability), so no supported card is offered a
// payment plan the kernel cannot replay. The admitted rows are a fixed
// amount of one colour with no {T} (Ashnod's Altar), and one mana of any
// colour with or without the {T} (Phyrexian Altar, and a single-ability
// stand-in for Lazotep Quarry: the real card's other-sacrifice ability is a
// later one, which the kernel never units -- a known latent limit, not a
// mirrored shape). Every deferred row is a production the kernel defers as
// an open ticket (kop.SacXEachManaAb, kop.SacXRestrManaAb,
// kop.SacTapFixedManaAb / kop.SecondSacTapManaAb) or one no kernel plan
// shape carries at all (dynamic, X, Combo, and an allocation of any-colour
// mana); a {T} plus a fixed colour is Phyrexian Tower's second ability,
// which the kernel's census units only as the card's later mana ability and
// therefore never plans.

func TestPaymentPlanOtherSacTierShapes(t *testing.T) {
	t.Parallel()
	mana := func(params map[string]string) *cards.SA {
		sa := &cards.SA{API: "Mana", Params: map[string]string{"SpellDescription": "x"}}
		for k, v := range params {
			sa.Params[k] = v
		}
		return sa
	}
	const sac = "Sac<1/Creature>"
	costOf := func(t *testing.T, raw string) costvocab.Cost {
		t.Helper()
		c := costvocab.ParseCost(raw)
		if len(c.Sac) != 1 {
			t.Fatalf("cost %q did not parse to exactly one Sac part: %+v", raw, c)
		}
		return c
	}
	for _, tc := range []struct {
		name     string
		raw      string
		params   map[string]string
		want     Tier
		wantCons Consequence
	}{
		// Admitted: the two production arms kop.OtherSacPlanAb honours.
		{"Ashnod's Altar (no tap, fixed colour x2)", sac,
			map[string]string{"Cost": sac, "Produced": "C", "Amount": "2"},
			TierLastResort, Consequence{SacrificeOthers: true}},
		{"Thermopod (no tap, fixed colour x1)", sac,
			map[string]string{"Cost": sac, "Produced": "R", "Amount": "1"},
			TierLastResort, Consequence{SacrificeOthers: true}},
		{"Phyrexian Altar (no tap, any colour x1)", sac,
			map[string]string{"Cost": sac, "Produced": "Any"},
			TierLastResort, Consequence{SacrificeOthers: true}},
		{"any colour x1 (single-ability stand-in)", "T " + sac,
			map[string]string{"Cost": "T " + sac, "Produced": "Any"},
			TierLastResort, Consequence{SacrificeOthers: true}},
		// Deferred: the kernel's open-ticket shapes.
		{"Phyrexian Tower's second (tap, fixed colour x2)", "T " + sac,
			map[string]string{"Cost": "T " + sac, "Produced": "B", "Amount": "2"},
			TierDeferred, Consequence{}},
		{"Master of Dark Rites (spend-restricted)", "T Sac<1/Creature.Other>",
			map[string]string{"Cost": "T Sac<1/Creature.Other>", "Produced": "B", "Amount": "3",
				"RestrictValid": "Spell.Demon,Spell.Cleric,Spell.Vampire"},
			TierDeferred, Consequence{}},
		{"Bolg's Company shape (one of each of two colours)", "T Sac<1/Creature.Other>",
			map[string]string{"Cost": "T Sac<1/Creature.Other>", "Produced": "B R"},
			TierDeferred, Consequence{}},
		{"Furgul's Quag Nurturer (dynamic amount)", "T " + sac,
			map[string]string{"Cost": "T " + sac, "Produced": "G", "Amount": "Sacrificed$CardPower"},
			TierDeferred, Consequence{}},
		{"Illuminor Szeras (X amount)", "T Sac<1/Creature.Other>",
			map[string]string{"Cost": "T Sac<1/Creature.Other>", "Produced": "B", "Amount": "X"},
			TierDeferred, Consequence{}},
		{"The Golden Throne (Combo Any)", "T " + sac,
			map[string]string{"Cost": "T " + sac, "Produced": "Combo Any", "Amount": "3"},
			TierDeferred, Consequence{}},
		{"any colour, two mana (allocation)", sac,
			map[string]string{"Cost": sac, "Produced": "Any", "Amount": "2"},
			TierDeferred, Consequence{}},
		{"two colours, no tap (mana of each)", sac,
			map[string]string{"Cost": sac, "Produced": "W U"},
			TierDeferred, Consequence{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ma := mana(tc.params)
			tier, c, detail, rider := PaymentPlanShapeTierOf(ma, costOf(t, tc.raw))
			if rider {
				t.Fatalf("rider verdict for %q", tc.name)
			}
			if tier != tc.want || c != tc.wantCons || (tc.want == TierDeferred && detail != "source:last_resort") {
				t.Errorf("tier = %v, %+v, %q; want %v, %+v", tier, c, detail, tc.want, tc.wantCons)
			}
		})
	}
}
