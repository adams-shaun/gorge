// Task 19: Defined$ ReplacedCard. A replacement's ReplaceWith$ could not
// name the object the replaced event was about: effects/context.go's Defined
// fell back to Ctx.Targets (nil for a replacement context) for every
// "ReplacedCard" script, so a Rest in Peace / Dryad Militant / Leyline of
// the Void-shaped replacement ("if a card would be put into a graveyard from
// anywhere, exile it instead") relocated NOTHING -- the card it was supposed
// to exile stayed wherever it was, and resolveTop's totality guard
// (ensureLeftTheStack) had to park it in exile as a backstop. This task
// wires the pair: rules/replacement.go sets Ctx.Replaced to the object the
// replaced event was about, and Defined$ ReplacedCard yields it.
//
// Card text below is authored for this test, in the same R:/SVar$ shape as
// the corpus cards that found the bug -- never copied from Forge's
// .cards/cardsfolder.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// restInPeaceSrc is the shipped corpus's Rest in Peace shape: a broad "if a
// card would be put into a graveyard from anywhere, exile it instead"
// replacement, whose ReplaceWith$ names the card it is acting on via
// Defined$ ReplacedCard.
const restInPeaceSrc = `Name:Peace
ManaCost:1 W
Types:Enchantment
R:Event$ Moved | ValidCard$ Card | Destination$ Graveyard | ReplaceWith$ Exile | Description$ If a card would be put into a graveyard from anywhere, exile it instead.
SVar:Exile:DB$ ChangeZone | Defined$ ReplacedCard | Destination$ Exile
Oracle:x
`

const banishedBearSrc = `Name:Bear
ManaCost:1 G
Types:Creature Bear
PT:2/2
Oracle:x
`

const quickBoltSrc = `Name:Bolt
ManaCost:R
Types:Instant
A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3
Oracle:x
`

// TestRestInPeaceShapedReplacementExilesTheReplacedCard is the task's
// regression test. Seed seat 0 with the Peace enchantment plus a bear and a
// bolt, so every object is a real deck card replayFromLog reconstructs (the
// logged Moves below -- Peace hand->battlefield, bear hand->battlefield,
// the exiling Changes -- are what make the eventual replayCheck line up).
//
// Part 1: a creature dealt lethal damage dies, but its "would go to the
// grave" Move matches Peace's replacement and the bear is exiled instead.
// Part 2: an instant cast and resolved also matches ("any card") on its own
// stack->graveyard Move and is exiled by the replacement itself -- the card
// leaves the stack on its own, so ensureLeftTheStack's backstop never has
// to fire (asserted via the absence of its "fully discarded" Note).
func TestRestInPeaceShapedReplacementExilesTheReplacedCard(t *testing.T) {
	e, cfg, peace := newFixtureDeck(t, 150, restInPeaceSrc, banishedBearSrc, quickBoltSrc)

	e.emit(events.Event{Kind: events.MoveZone, Obj: peace, From: state.ZHand, To: state.ZBattlefield})

	bear := putCreature(t, e, 0, banishedBearSrc)
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 5})
	e.checkStateBased()
	if z := e.G.Obj(bear).Zone; z != state.ZExile {
		t.Fatalf("bear in %s, want exile -- Peace's replacement must exile the killed "+
			"card, not let it reach the graveyard and get backstopped", z)
	}

	bolt := addToHand(t, e, 0, quickBoltSrc)
	addMana(t, e, 0, "R")
	castObj(t, e, bolt)
	if z := e.G.Obj(bolt).Zone; z != state.ZExile {
		t.Fatalf("bolt in %s, want exile (the replacement relocated it; no backstop needed)", z)
	}
	if hasNote(e, "fully discarded") {
		t.Fatal("ensureLeftTheStack fired: the replacement should have relocated the card itself")
	}
	replayCheck(t, e, cfg)
}
