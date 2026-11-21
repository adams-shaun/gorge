package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 601.2g, 605.3b: Academy Ruins' "{1}{U}, {T}: Put target artifact card
// from your graveyard on top of its owner's library" is payable when the only
// {U} on the board is Starfleet Academy's "Federation — {5}, {T}: Choose a
// color. Add an amount of mana of that color equal to the number of creature
// types among non-Borg creatures you control" (X = 1 beside one Bear): the
// Mountains fund the Federation's {5}, its colour ask funds the {U}. This is
// the paid dynamic-amount census gap (Starfleet Academy is refused in dirac's
// table over it, dirac/support/refused/starfleet-academy.json): gorge's
// canonical pricing (PotentialPaymentPlans) must still price the activation
// through its census relaxation and prefix script — that price is what the
// lockstep's canonical priority list offers and what dirac's kernel mirrors
// (its open any-colour production). A game whose canonical pricing dropped
// the activation is exactly the lockstep `options options` divergence the
// evidence loop found at Academy Ruins (ev-dr-mbc-abbb-s000, decision 980).

const sfuRuinsSrc = "Name:Academy Ruins\nManaCost:no cost\nTypes:Legendary Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nA:AB$ ChangeZone | Cost$ 1 U T | TgtPrompt$ Choose target artifact card in your graveyard | ValidTgts$ Artifact.YouCtrl | Origin$ Graveyard | Destination$ Library | SpellDescription$ Put target artifact card from your graveyard on top of your library.\nOracle:x\n"

const sfuStarfleetSrc = "Name:Starfleet Academy\nManaCost:no cost\nTypes:Legendary Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nA:AB$ Mana | PrecostDesc$ Federation — | Cost$ 5 T | Produced$ Any | Amount$ X | SpellDescription$ Choose a color. Add an amount of mana of that color equal to the number of creature types among non-Borg creatures you control.\nSVar:X:Count$Valid Creature.nonBorg+YouCtrl$CreatureType\nOracle:x\n"

const sfuBearSrc = "Name:Grizzly Bears\nManaCost:1G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

const sfuArtifactSrc = "Name:Sol Ring\nManaCost:1\nTypes:Artifact\nOracle:x\n"

// sfuRuinsPlan finds the PotentialPaymentPlans entry of Academy Ruins' {1}{U}
// graveyard-recall ability (pile index 1).
func sfuRuinsPlan(t *testing.T, e *Engine, ruins state.ObjID) (PaymentPlanOutcome, int) {
	t.Helper()
	for _, pp := range e.potentialPlans(0, 16) {
		if pp.Action.Kind == "ability" && pp.Action.Obj == ruins && pp.Action.Ability == 1 {
			return PaymentPlanOutcome{Reason: pp.Reason, Detail: pp.Detail}, len(pp.Script)
		}
	}
	return PaymentPlanOutcome{Reason: "absent"}, 0
}

// sfuRuinsFixture seats seat 0 with Academy Ruins, the named extras and six
// Mountains on the battlefield, an artifact card in its graveyard, and
// re-asks priority at main1.
func sfuRuinsFixture(t *testing.T, starfleet, bear bool) (*Engine, state.ObjID) {
	t.Helper()
	extras := []string{sfuArtifactSrc, sfuArtifactSrc}
	if starfleet {
		extras = append(extras, sfuStarfleetSrc)
	}
	if bear {
		extras = append(extras, sfuBearSrc)
	}
	e, _, ruins := newFixtureDeck(t, 34, sfuRuinsSrc, extras...)
	mv := func(oid state.ObjID, from, to state.Zone) {
		if oid != 0 {
			e.emit(events.Event{Kind: events.MoveZone, Obj: oid, From: from, To: to})
		}
	}
	find := func(name string) state.ObjID {
		for _, cand := range e.G.Zone(state.ZHand, 0) {
			if e.G.Obj(cand).Face().Name == name {
				return cand
			}
		}
		for _, cand := range e.G.Zone(state.ZLibrary, 0) {
			if e.G.Obj(cand).Face().Name == name {
				return cand
			}
		}
		return 0
	}
	mv(find("Academy Ruins"), state.ZHand, state.ZBattlefield)
	if starfleet {
		mv(find("Starfleet Academy"), state.ZHand, state.ZBattlefield)
	}
	if bear {
		if bid := find("Grizzly Bears"); bid != 0 {
			mv(bid, state.ZLibrary, state.ZBattlefield)
		} else {
			putCreature(t, e, 0, sfuBearSrc)
		}
	}
	nm := 0
	for _, cand := range e.G.Zone(state.ZLibrary, 0) {
		if nm >= 6 {
			break
		}
		if e.G.Obj(cand).Face().Name == "Mountain" {
			mv(cand, state.ZLibrary, state.ZBattlefield)
			nm++
		}
	}
	if nm < 6 {
		t.Fatalf("seeded only %d Mountains", nm)
	}
	n := 0
	for _, cand := range e.G.Zone(state.ZLibrary, 0) {
		if n >= 1 {
			break
		}
		if e.G.Obj(cand).Face().Name == "Sol Ring" {
			mv(cand, state.ZLibrary, state.ZGraveyard)
			n++
		}
	}
	if n < 1 {
		t.Fatal("could not seed an artifact card in the graveyard")
	}
	addMana(t, e, 0, "C") // drive to main1, float {C}, re-ask priority
	return e, ruins
}

// TestStarfleetFundedActivationPriced pins the canonical pricing of the
// Federation-funded {1}{U} activation, and its two refusals (no {U} source at
// all; the Federation makes nothing with X = 0).
func TestStarfleetFundedActivationPriced(t *testing.T) {
	t.Parallel()

	e, ruins := sfuRuinsFixture(t, true, true)
	got, script := sfuRuinsPlan(t, e, ruins)
	if got.Reason != "" || script == 0 {
		t.Fatalf("Federation-funded {1}{U} activation not priced: reason %q detail %q script %d steps", got.Reason, got.Detail, script)
	}

	e, ruins = sfuRuinsFixture(t, false, true)
	if got, _ := sfuRuinsPlan(t, e, ruins); got.Reason == "" {
		t.Fatal("the {1}{U} activation was priced with no {U} source on the board")
	}

	e, ruins = sfuRuinsFixture(t, true, false)
	if got, _ := sfuRuinsPlan(t, e, ruins); got.Reason == "" {
		t.Fatal("the {1}{U} activation was priced through a Federation whose X resolves to 0")
	}
}
