package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Great Divide Guide shape: each land and Ally its controller controls
// has "{T}: Add one mana of any color." The granted mana abilities are real
// mana (CR 605.1a-d): a priority cast's affordability proof counts them like
// any other free tap ability, so a cast whose cost they can fund is offered.
const (
	ggcGuide = "Name:Great Divide Guide\nManaCost:1 G\nTypes:Creature Human Scout Ally\nPT:2/3\n" +
		"S:Mode$ Continuous | Affected$ Land.YouCtrl,Ally.YouCtrl | AddAbility$ AnyMana | Description$ x\n" +
		"SVar:AnyMana:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ x\nOracle:x\n"
	ggcGran = "Name:Gran-Gran Shaped\nManaCost:U\nTypes:Creature Human Peasant Ally\nPT:1/2\nOracle:x\n"
	// 4GG: six mana, two of them green.
	ggcFlopsie = "Name:GGC Flopsie\nManaCost:4 G G\nTypes:Creature Ape Goat\nPT:4/4\nOracle:x\n"
)

func ggcIsland() string { return "Name:GGC Island\nTypes:Land Island\nOracle:x\n" }

// With the Guide on the battlefield, six untapped mana sources (four lands
// and two Allies, every one of them able to make any colour) pay 4GG: the
// cast is offered at priority.
func TestGuideGrantCensusOffersSixManaCast(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9404, ggcFlopsie)
	toMain1(t, e)
	onBoardReady(t, e, 0, ggcGuide)
	onBoardReady(t, e, 0, ggcGran)
	for i := 0; i < 4; i++ {
		onBoard(t, e, 0, ggcIsland())
	}
	ids := setHandSrcs(t, e, 0, ggcFlopsie)
	d := ppPriority(t, e)
	found := false
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == ids[0] {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no cast option for the 4GG creature, options: %+v", d.Options)
	}
}

// The control: without the grant only the four Islands' {U} exists -- five
// mana short of 4GG with no green, so the cast is not offered.
func TestGuideGrantCensusWithoutGrantDeclinesCast(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9404, ggcFlopsie)
	toMain1(t, e)
	onBoardReady(t, e, 0, ggcGran)
	for i := 0; i < 4; i++ {
		onBoard(t, e, 0, ggcIsland())
	}
	ids := setHandSrcs(t, e, 0, ggcFlopsie)
	d := ppPriority(t, e)
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == ids[0] {
			t.Fatalf("cast offered without a granter, options: %+v", d.Options)
		}
	}
}

// priority options are checked through the shared decision type; keep the
// import used even if the option walk's shape changes.
var _ = decision.KPriority
var _ = state.ZHand

func TestZZGuideCensusDebug(t *testing.T) {
	e, _, _ := newFixtureDeck(t, 9404, ggcFlopsie)
	toMain1(t, e)
	onBoardReady(t, e, 0, ggcGuide)
	onBoardReady(t, e, 0, ggcGran)
	for i := 0; i < 4; i++ {
		onBoard(t, e, 0, ggcIsland())
	}
	setHandSrcs(t, e, 0, ggcFlopsie)
	d := ppPriority(t, e)
	units := e.paymentPlanManaUnits(0)
	for _, u := range units {
		t.Logf("unit id=%d free=%d", u.ID, u.FreeCount)
		for _, a := range u.Alts {
			t.Logf("   alt counts=%v amt=%d any=%v", a.Counts, a.Amt, a.Any)
		}
	}
	t.Logf("pool=%v", e.G.Players[0].Pool)
	pm := e.PotentialMana(0)
	t.Logf("PotentialMana=%v", pm)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		mas := e.availableManaAbilities(0, id)
		t.Logf("board id=%d name=%s abilities=%d", id, o.Face().Name, len(mas))
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		t.Logf("pp kind=%s obj=%d reason=%q detail=%q script=%d plan=%v", pp.Action.Kind, pp.Action.Obj, pp.Reason, pp.Detail, len(pp.Script), pp.Plan != nil)
	}
	census := e.paymentPlanCensusOf(0, nil)
	t.Logf("census complete=%v relaxable=%v gaps=%v fees=%v", census.complete, census.relaxable, census.gaps, census.fees())
	_ = d
}

