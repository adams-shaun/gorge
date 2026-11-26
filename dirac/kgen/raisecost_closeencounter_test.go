package kgen

import (
	"testing"
)

// TestRaiseCostCloseEncounterLowers: the T18c RaiseCost computed-or-picked
// carrier lowers. Close Encounter's own additional cost is a ChooseCard pick
// over two zones — `ChooseCard<1/Creature.YouCtrl+inZoneBattlefield;Creature.
// YouOwn+inZoneExile+warped>` (gorge's costvocab.CloseEncounterChooseSpec) —
// and its spell body's X reads the chosen object's power at resolution
// (SVar X = RevealedCards$CardPower, gorge's cast-cost PAID "Revealed" list).
// The raise rides the spell's own additional cost field (kop.Cost.CloseEnc)
// and the body's X lowers to the power of the object the cast chose
// (rChosenEnc: the cast's chosen-cost object).
func TestRaiseCostCloseEncounterLowers(t *testing.T) {
	all, bs := costProgCastBuild(t, []string{"Close Encounter"})
	if len(all) != 1 || len(bs) != 1 {
		t.Fatalf("lowered %d cards, built %d, want one of each", len(all), len(bs))
	}
	if got := costProgCastRouteReason(all[0]); got != "" {
		t.Fatalf("Close Encounter refusals %q, want none", got)
	}
	sp := spellAbility(bs[0].def)
	if sp == nil {
		t.Fatalf("Close Encounter built with no AbSpell ability")
	}
	c := sp.Cost // an instant's additional cost rides its AbSpell (build.go)
	if !c.CloseEnc {
		t.Fatalf("Close Encounter additional cost %+v, want the CloseEnc choose part", c)
	}
	if c.N != 0 || c.Flags != 0 {
		t.Fatalf("Close Encounter additional cost N %d flags %v, want N 0 and no flag: the ask owes exactly one pick (kernel cost_closeenc.go)", c.N, c.Flags)
	}
	// the spell body's X: the DealDamage amount reads the chosen object's
	// power (RevealedCards$CardPower)
	var found bool
	for _, a := range all[0].Abilities {
		if a.Kind != akSpell {
			continue
		}
		for _, in := range a.Prog {
			if in.Op == "DealDamage" && in.A.Kind == vPowerOf && in.A.Ref == (lRef{Kind: rChosenEnc}) {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("Close Encounter's DealDamage X does not read rChosenEnc's power")
	}
}

// TestRaiseCostCloseEncounterRefuses: the part is the printed Close Encounter
// shape only, and the body read is that spell's own cost. gorge's cost parser
// admits the CloseEncounterChooseSpec alone (cost/token_match.go) and the
// kernel records one chosen object, so a second spec, another count and a
// body read without the cost all stay refused.
func TestRaiseCostCloseEncounterRefuses(t *testing.T) {
	raise := func(cost string) string {
		return "Name:Probe CE\nManaCost:1 G\nTypes:Instant\n" +
			"S:Mode$ RaiseCost | ValidCard$ Card.Self | Activator$ You | Type$ Spell | Cost$ " + cost +
			" | EffectZone$ All | Description$ Probe.\n" +
			"A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ X | SpellDescription$ Probe.\n" +
			"SVar:X:Revealed" + "Cards$CardPower\nOracle:Probe.\n"
	}
	for _, cost := range []string{
		"ChooseCard<1/Card.YouCtrl>",                       // not the printed two-zone spec
		"ChooseCard<2/" + closeEncSpec + ">",               // two picks: the kernel records one
		"ChooseCard<1/Creature.YouCtrl+inZoneBattlefield>", // the battlefield arm alone
	} {
		if got := buildRefusals(t, "Probe CE", raise(cost)); got == "" {
			t.Errorf("raise %q lowered", cost)
		}
	}
	// the body read without the choose cost reads nothing the kernel records
	if got := buildRefusals(t, "Probe CE",
		"Name:Probe CE\nManaCost:1 G\nTypes:Instant\n"+
			"A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ X | SpellDescription$ Probe.\n"+
			"SVar:X:Revealed"+"Cards$CardPower\nOracle:Probe.\n"); got == "" {
		t.Errorf("RevealedCards$CardPower lowered without the choose cost")
	}
}
