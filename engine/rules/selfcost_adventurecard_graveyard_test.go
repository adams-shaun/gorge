package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// hearthLike is a Hearth Elemental-shaped card: the printed creature face
// carries an EffectZone$ All ReduceCost on itself whose Amount$ X counts
// "cards in your graveyard that are instant cards, sorcery cards, and/or
// have an Adventure" (Forge's Count$ValidGraveyard Instant.YouOwn,
// Sorcery.YouOwn,Card.AdventureCard+YouOwn), and the card has an ALTERNATE
// Adventure face. hearthLikeNoAdv drops the Adventure face and the
// Adventure-card count term -- the Tolarian Terror shape that already works.
const hearthLike = "Name:Test Elemental\nManaCost:5 R\nTypes:Creature Elemental\nPT:4/5\nAlternateMode:Adventure\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ X | EffectZone$ All | Description$ costs less\n" +
	"SVar:X:Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn,Card.AdventureCard+YouOwn\nOracle:x\n" +
	"\nALTERNATE\n\n" +
	"Name:Stoke Test\nManaCost:1 R\nTypes:Sorcery Adventure\n" +
	"A:SP$ Draw | NumCards$ 2\nOracle:y\n"

const hearthLikeNoAdv = "Name:Test Elemental\nManaCost:5 R\nTypes:Creature Elemental\nPT:4/5\n" +
	"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ X | EffectZone$ All | Description$ costs less\n" +
	"SVar:X:Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn\nOracle:x\nOracle:y\n"

const advFiller = "Name:Filler Sorcery\nManaCost:R\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"

// The card's OWN ReduceCost must price its printed cast at offer time
// (CR 601.2f), whatever the Amount$ count reads. Hearth Elemental in a
// std-woe lockstep game (pair 173, seed 434573) lost its printed cast
// option: gorge charged the full {5}{R} where {X} with two qualifying
// graveyard cards made it {3}{R}, so only the kernel offered the cast.
func TestSelfCostStaticCountsGraveyardAdventureCards(t *testing.T) {
	setup := func(src string, grave func(e *Engine)) (*Engine, state.ObjID) {
		e := mayPlayBase(t)
		id := handCard(e, card(t, src), 0)
		grave(e)
		addMana(t, e, 0, "RRRG")
		return e, id
	}
	// printedCast reports the PRINTED-face cast option (Mode empty; the
	// Adventure face's offer is Kind cast Mode adventure_alt).
	printedCast := func(e *Engine, id state.ObjID) bool {
		for _, o := range e.Pending().Options {
			if o.Kind == "cast" && o.Obj == id && o.Mode == "" {
				return true
			}
		}
		return false
	}
	twoSorceries := func(e *Engine) {
		graveCard(e, card(t, advFiller), 0, 0)
		graveCard(e, card(t, advFiller), 0, 0)
	}
	emptyGrave := func(e *Engine) {}

	// Control: the two-term count on a plain creature already reduces
	// {5}{R} to {3}{R}.
	if e, id := setup(hearthLikeNoAdv, twoSorceries); !printedCast(e, id) {
		t.Fatalf("control: cast not offered with the two-term count and {R}{R}{R}{G} funded: %+v", e.Pending().Options)
	}
	// The full count on the Adventure-shaped card must reduce the same way:
	// the printed creature cast is still offered (CR 715.1 -- the Adventure
	// face does not take the printed cast away).
	if e, id := setup(hearthLike, twoSorceries); !printedCast(e, id) {
		t.Fatalf("cast not offered: the Adventure shape kept the full {5}{R}: %+v", e.Pending().Options)
	}
	// Precondition for the reductions: with an empty graveyard the {5}{R} is
	// unaffordable on {R}{R}{R}{G}, so no cast may be offered.
	if e, id := setup(hearthLike, emptyGrave); printedCast(e, id) {
		t.Fatalf("cast offered with an empty graveyard: the reduction is not the only reason the cast is affordable")
	}
}

// On the autopay surface the payment planner prices each offered plain cast,
// and a cast it declines ("insufficient") is dropped from the options. The
// planner must price the reduced cost the same read the offer gate composed:
// four untapped Mountains pay {3}{R} of the two-reduced {5}{R}.
func TestSelfCostPlannerPricesGraveyardAdventureCount(t *testing.T) {
	e := mayPlayBase(t)
	id := handCard(e, card(t, hearthLike), 0)
	graveCard(e, card(t, advFiller), 0, 0)
	graveCard(e, card(t, advFiller), 0, 0)
	for i := 0; i < 4; i++ {
		moveByName(t, e, 0, "Mountain", state.ZBattlefield)
	}
	addMana(t, e, 0, "")
	out := e.PlanCastPayment(0, decision.PlannedCast{Object: id, Face: 0, Origin: "hand"})
	if out.Plan == nil || out.Reason != "" {
		t.Fatalf("planner declined the reduced cast: plan=%v reason=%q detail=%q", out.Plan, out.Reason, out.Detail)
	}
	// Precondition: with an empty graveyard the raw {5}{R} stays beyond four
	// Mountains' mana, and the planner must decline it the same way.
	e2 := mayPlayBase(t)
	id2 := handCard(e2, card(t, hearthLike), 0)
	for i := 0; i < 4; i++ {
		moveByName(t, e2, 0, "Mountain", state.ZBattlefield)
	}
	addMana(t, e2, 0, "")
	out2 := e2.PlanCastPayment(0, decision.PlannedCast{Object: id2, Face: 0, Origin: "hand"})
	if out2.Plan != nil || out2.Reason == "" {
		t.Fatalf("planner priced the raw {5}{R} off four Mountains: plan=%v reason=%q", out2.Plan, out2.Reason)
	}
}
