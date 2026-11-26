package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCostProgCastRouteRemainder lowers the alternative-cost shape whose only
// part is a pick kop.Cost can already carry beside a spell ability: Lashknife's
// "tap an untapped creature you control rather than pay this spell's mana
// cost" (tapXType<1/Creature>) on an Aura whose spell ability owns the enchant
// target. The kernel poses the tap pick for the alternative cast
// (cast_altfree.go offerAltFreeHand / altTapCost), and the Aura's target ask is
// the spell ability's, so the static lowers like an instant's or sorcery's.
//
// The static's own gate (IsPresent$ Plains.YouCtrl) must survive lowering: the
// alternative cast is offered only while its controller has a Plains.
func TestCostProgCastRouteRemainder(t *testing.T) {
	all, bs := costProgCastBuild(t, []string{"Lashknife"})
	if len(all) != 1 || len(bs) != 1 {
		t.Fatalf("lowered %d cards, built %d, want one of each", len(all), len(bs))
	}
	if got := costProgCastRouteReason(all[0]); got != "" {
		t.Fatalf("Lashknife: refusals %q, want none (the tap alternative needs no kernel pick beyond altTapCost)", got)
	}
	var alt *kop.Ability
	for i := range bs[0].def.Abilities {
		a := &bs[0].def.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost {
			alt = a
			break
		}
	}
	if alt == nil {
		t.Fatal("Lashknife lowered without a StatAlternativeCost ability (the alternative cast would never be offered)")
	}
	if alt.Cost.Flags != kop.CostTapX || alt.Cost.N != 1 {
		t.Fatalf("alternative cost %+v, want one tapped creature (CostTapX N=1)", alt.Cost)
	}
	if alt.Cost.Mana != (kop.ManaCost{}) {
		t.Fatalf("alternative mana %+v, want none (the whole cost is the tap)", alt.Cost.Mana)
	}
	if alt.Static.Cond2 == 0 {
		t.Error("the IsPresent$ Plains.YouCtrl gate is missing from the static")
	}
}

// auraAltScript is an Aura carrying an AlternativeCost static whose Cost$ is
// the argument, beside the Aura's one enchant ability.
func auraAltScript(cost string) string {
	return "Name:Probe Aura\nManaCost:1 W\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ AlternativeCost | ValidSA$ Spell.Self | EffectZone$ All | Cost$ " + cost +
		" | Description$ Probe.\nOracle:Probe.\n"
}

// buildRefusals lowers and builds one inline card and returns its refusal
// reasons joined.
func buildRefusals(t *testing.T, name, src string) string {
	t.Helper()
	card, diags := cards.ParseBytes("probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{name: card}, names: []string{name},
		nDecks: map[string]int{}, reg: testutil.CorpusRegistry(t)}
	all, _, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	return costProgCastRouteReason(all[0])
}

// TestCostProgCastRouteAuraAltShapes: an Aura's enchant ability shares the
// cast with the alternative cost only for the tap-alone shape the lockstep
// run covers (Lashknife). A sacrifice, return, discard or tap-plus-mana
// alternative beside the enchant target stays refused until it is checked.
func TestCostProgCastRouteAuraAltShapes(t *testing.T) {
	if got := buildRefusals(t, "Probe Aura", auraAltScript("tapXType<1/Creature>")); got != "" {
		t.Fatalf("tap-alone Aura alternative refused: %q", got)
	}
	for _, cost := range []string{"Sac<1/Creature>", "Discard<1/Card>", "W tapXType<1/Creature>"} {
		got := buildRefusals(t, "Probe Aura", auraAltScript(cost))
		if !strings.Contains(got, "non-mana part") {
			t.Errorf("Aura alternative %q: refusals %q, want the non-mana-part refusal", cost, got)
		}
	}
}
