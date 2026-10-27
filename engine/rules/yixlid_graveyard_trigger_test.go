package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// yixlidJailerPreconditions asserts the corpus shape the three scenarios below
// depend on: Golgari Brownscale prints the Card.Self graveyard->hand
// ChangesZone line, and Yixlid Jailer's static removes all abilities from
// cards in the graveyard. Without these, a green scenario proves nothing.
func yixlidJailerPreconditions(t *testing.T) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	brownscale, ok := reg.Lookup("Golgari Brownscale")
	if !ok {
		t.Fatal("Golgari Brownscale missing from corpus")
	}
	self := false
	for _, tg := range brownscale.Faces[0].Triggers {
		if tg.Mode == "ChangesZone" && tg.ParamStr(cards.PKOrigin) == "Graveyard" &&
			tg.ParamStr(cards.PKDestination) == "Hand" && tg.ParamStr(cards.PKValidCard) == "Card.Self" {
			self = true
		}
	}
	if !self {
		t.Fatal("precondition: Golgari Brownscale prints no Card.Self ChangesZone Origin$ Graveyard Destination$ Hand line")
	}
	jailer, ok := reg.Lookup("Yixlid Jailer")
	if !ok {
		t.Fatal("Yixlid Jailer missing from corpus")
	}
	remover := false
	for i := range jailer.Faces[0].Statics {
		if jailer.Faces[0].Statics[i].HasParam(cards.PKRemoveAllAbilities) &&
			jailer.Faces[0].Statics[i].ParamStr(cards.PKAffectedZone) == "Graveyard" {
			remover = true
		}
	}
	if !remover {
		t.Fatal("precondition: Yixlid Jailer carries no graveyard-scoped RemoveAllAbilities$ static")
	}
}

// runYixlidScenario runs one inline audit scenario and fails on any scenario
// failure or replay diff.
func runYixlidScenario(t *testing.T, sc string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("scenario: err=%v fails=%v", err, res.Fails)
	}
}

// TestYixlidJailerStopsGraveyardToHandTrigger is the XMage
// LoosingAbilitiesTest#testYixlidJailerAndPutIntoHandEffect row, folded onto
// one player: Cemetery Recruitment returns the Brownscale, but Yixlid Jailer
// ("cards in graveyards lose all abilities", CR 113.6) means the card's
// graveyard->hand trigger no longer exists when it leaves (CR 603.10a looks
// back in time), so p0 gains no life.
func TestYixlidJailerStopsGraveyardToHandTrigger(t *testing.T) {
	t.Parallel()
	yixlidJailerPreconditions(t)
	runYixlidScenario(t, `{"name":"yixlid-stops-brownscale","cr":["113.6","603.10a"],"why":"XMage LoosingAbilitiesTest#testYixlidJailerAndPutIntoHandEffect: under Yixlid Jailer the graveyard->hand trigger is gone","setup":{"p0":{"hand":["Cemetery Recruitment"],"battlefield":["Yixlid Jailer","Swamp","Swamp"],"graveyard":["Golgari Brownscale"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Cemetery Recruitment","mana":"CB","targets":["p0:Golgari Brownscale"]},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":0,"zone":"hand","name":"Golgari Brownscale"},"eq":1},{"life":{"p0":20}}]}`)
}

// TestGolgariBrownscaleGraveyardToHandTrigger is the positive control: the
// same board without Yixlid Jailer, where the Brownscale's own
// "when CARDNAME is put into your hand from your graveyard, you gain 2 life"
// line must fire, gaining p0 2 life. This pins the widened 7e1c80fae
// self-ChangesZone admission: the negative above is the Jailer's ability
// loss, not a setup whose trigger never fires.
func TestGolgariBrownscaleGraveyardToHandTrigger(t *testing.T) {
	t.Parallel()
	yixlidJailerPreconditions(t)
	runYixlidScenario(t, `{"name":"brownscale-gains-life","cr":["603.10a"],"why":"without the Jailer the graveyard->hand trigger fires","setup":{"p0":{"hand":["Cemetery Recruitment"],"battlefield":["Swamp","Swamp"],"graveyard":["Golgari Brownscale"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Cemetery Recruitment","mana":"CB","targets":["p0:Golgari Brownscale"]},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":0,"zone":"hand","name":"Golgari Brownscale"},"eq":1},{"life":{"p0":22}}]}`)
}

// TestYixlidJailerKeepsObserverGraveyardTrigger pins the id == ev.Obj guard at
// the trigger gate: the ability-loss look-back is asked only about the moved
// card itself. Kishla Skimmer, which watches ANY card leave the graveyard, is
// a creature on the battlefield, so the Jailer does not apply to it -- its
// "whenever a card leaves your graveyard" line must still draw Grizzly Bears
// as the Brownscale returns to hand.
func TestYixlidJailerKeepsObserverGraveyardTrigger(t *testing.T) {
	t.Parallel()
	yixlidJailerPreconditions(t)
	runYixlidScenario(t, `{"name":"yixlid-keeps-kishla","cr":["113.6","603.10a"],"why":"the observer's own trigger is not suppressed by the moved card's ability loss","setup":{"p0":{"hand":["Cemetery Recruitment"],"battlefield":["Kishla Skimmer","Yixlid Jailer","Swamp","Swamp"],"graveyard":["Golgari Brownscale"],"library_top":["Grizzly Bears","Silvercoat Lion"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Cemetery Recruitment","mana":"CB","targets":["p0:Golgari Brownscale"]},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":0,"zone":"hand","name":"Golgari Brownscale"},"eq":1},{"count":{"seat":0,"zone":"hand","name":"Grizzly Bears"},"eq":1}]}`)
}
