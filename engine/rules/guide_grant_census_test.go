package rules

import (
	"testing"
)

// The Great Divide Guide shape: each land and Ally its controller controls
// has "{T}: Add one mana of any color." The granted abilities are real mana
// (CR 605.1a-d), but the planner census holds printed mana abilities only,
// so the cast is priced through the potential-planner's script surface: a
// printed census that covers the cost's colours but not its count leaves the
// granted sources as census gaps, and potentialPrefixScript finds the prefix
// that taps one for the missing unit.
const (
	ggcGuide = "Name:Great Divide Guide\nManaCost:1 G\nTypes:Creature Human Scout Ally\nPT:2/3\n" +
		"S:Mode$ Continuous | Affected$ Land.YouCtrl,Ally.YouCtrl | AddAbility$ AnyMana | Description$ x\n" +
		"SVar:AnyMana:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ x\nOracle:x\n"
	ggcGran = "Name:Gran-Gran Shaped\nManaCost:U\nTypes:Creature Human Peasant Ally\nPT:1/2\nOracle:x\n"
	// 4GG: six mana, two of them green.
	ggcFlopsie = "Name:GGC Flopsie\nManaCost:4 G G\nTypes:Creature Ape Goat\nPT:4/4\nOracle:x\n"
)

func ggcIsland() string { return "Name:GGC Island\nTypes:Land Island\nOracle:x\n" }

func ggcForest() string { return "Name:GGC Forest\nTypes:Land Forest\nOracle:x\n" }

// With the Guide on the battlefield, the printed mana (a Forest's {G}, four
// Islands' {U}) covers the cast's colours but falls one mana short of 4GG;
// the granted abilities are the planner census's gaps, and the prefix search
// taps the Guide for the missing green: PotentialPaymentPlans prices the
// cast with that scripted prefix (pair 619's shape).
func TestGuideGrantCensusOffersSixManaCast(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9404, ggcFlopsie)
	toMain1(t, e)
	onBoardReady(t, e, 0, ggcGuide)
	onBoardReady(t, e, 0, ggcGran)
	onBoard(t, e, 0, ggcForest())
	for i := 0; i < 4; i++ {
		onBoard(t, e, 0, ggcIsland())
	}
	ids := setHandSrcs(t, e, 0, ggcFlopsie)
	ppPriority(t, e) // pose the live priority decision; the search branches on its activations
	pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", ids[0])
	if pp.Reason != "" || len(pp.Script) == 0 {
		t.Fatalf("cast plan reason %q detail %q script %+v, want payable with a scripted prefix", pp.Reason, pp.Detail, pp.Script)
	}
}

// The control: without the grant only the printed mana exists (the Forest's
// {G} and the four Islands' {U}, five short of 4GG's six with the census
// complete, no gaps to relax or search), so the planner proves the cast
// unpayable.
func TestGuideGrantCensusWithoutGrantDeclinesCast(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9404, ggcFlopsie)
	toMain1(t, e)
	onBoardReady(t, e, 0, ggcGran)
	onBoard(t, e, 0, ggcForest())
	for i := 0; i < 4; i++ {
		onBoard(t, e, 0, ggcIsland())
	}
	ids := setHandSrcs(t, e, 0, ggcFlopsie)
	ppPriority(t, e)
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "cast" && pp.Action.Obj == ids[0] && pp.Reason == "" {
			t.Fatalf("cast payable without a granter: reason %q detail %q script %+v", pp.Reason, pp.Detail, pp.Script)
		}
	}
}
