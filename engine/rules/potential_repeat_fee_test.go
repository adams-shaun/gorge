package rules

import "testing"

// repeatFeeMasterPlan returns the priced verdict of Thunderscape Master's
// "{B}{B}, {T}" ability on a board of untapped, unsick permanents, and whether
// the potential walk lists the play at all.
func repeatFeeMasterPlan(t *testing.T, board []string) (PotentialPlan, bool) {
	t.Helper()
	e := boundBoard(t, append([]string{"Thunderscape Master"}, board...), nil)
	for _, pp := range e.PotentialPaymentPlans(0) {
		if o := e.G.Obj(pp.Action.Obj); pp.Action.Kind == "ability" && pp.Action.Ability == 0 && o != nil && o.Face() != nil && o.Face().Name == "Thunderscape Master" {
			return pp, true
		}
	}
	return PotentialPlan{}, false
}

// CR 605.3a: Bog Initiate's "{1}: Add {B}." has no {T} and no limit, so a
// payment activates it once per other source: a Mountain and a Forest make
// {B}{B} through it twice. The potential bound counted its {B} once and
// dropped Master's {B}{B} before any search ran (the kernel offered it).
func TestRepeatableFeeOnlyManaFundsTwoPips(t *testing.T) {
	if _, listed := repeatFeeMasterPlan(t, []string{"Mountain", "Wirewood Lodge"}); listed {
		t.Fatal("precondition: {B}{B} must be out of the bound without the converting creature")
	}
	pp, listed := repeatFeeMasterPlan(t, []string{"Bog Initiate", "Mountain", "Wirewood Lodge"})
	if !listed || pp.Reason != "" || len(pp.Script) == 0 {
		t.Fatalf("Master's {B}{B} is payable through two activations of Bog Initiate: listed %v reason %q detail %q script %d", listed, pp.Reason, pp.Detail, len(pp.Script))
	}
	if pp, listed := repeatFeeMasterPlan(t, []string{"Bog Initiate", "Mountain"}); listed && pp.Reason == "" {
		t.Fatal("one land funds one activation: {B}{B} is out of reach")
	}
}

// CR 605.3a, 601.2g: with Energy Refractor's fee-only "{2}: Add one mana of any
// color" also on the board, Bog Initiate's "{1}: Add {B}." line that alone
// funds Master's {B}{B} is still offered.
func TestRepeatableFeeOnlyBesideRefractorFundsTwoPips(t *testing.T) {
	pp, listed := repeatFeeMasterPlan(t, []string{"Energy Refractor", "Bog Initiate", "Mountain", "Wirewood Lodge"})
	if !listed || pp.Reason != "" || len(pp.Script) == 0 {
		t.Fatalf("Master's {B}{B} is payable through two activations of Bog Initiate beside a Refractor: listed %v reason %q detail %q script %d", listed, pp.Reason, pp.Detail, len(pp.Script))
	}
}
