// A tax on attacking a player prices the player pair alone (CR 508.1g:
// Propaganda's "can't attack you unless ..." charges the creatures attacking
// its controller). attackOffersCompute used to skip the whole creature when
// its player pair was priced out, so a creature that could not pay to attack
// the player could not attack that player's planeswalker either, though no
// tax applied to that attack.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPricedOutPlayerPairKeepsWalkerPair: Ghostly Prison and a planeswalker
// on seat 0, a creature and no mana on seat 1. Attacking seat 0 costs {2}
// and is not offered; attacking the planeswalker is free and is.
func TestPricedOutPlayerPairKeepsWalkerPair(t *testing.T) {
	t.Parallel()
	cfg := Config{Seed: 719, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}}
	e := New(cfg)
	onBoardCard(t, e, 0, mshCorpusCard(t, "Ghostly Prison"))
	walker := onBoardCard(t, e, 0, card(t, targetWalkerFixture))
	bear := onBoardReady(t, e, 1, "Name:Taxed Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	// PRECONDITION: seat 1 has no mana, so the {2} player pair is priced out,
	// and the planeswalker is seat 0's, untaxed by Ghostly Prison's Target$ You.
	if n := e.attackBudget(1); n != 0 {
		t.Fatalf("precondition: seat 1's attack budget = %d, want 0", n)
	}
	if c := e.attackPairCharge(bear, 0, 0); c.mana != 2 {
		t.Fatalf("precondition: the player pair charges %+v, want {2}", c)
	}
	if c := e.attackPairCharge(bear, 0, walker); !c.zero() {
		t.Fatalf("precondition: the planeswalker pair charges %+v, want nothing", c)
	}
	var player, atWalker bool
	for _, of := range e.attackOffers() {
		if of.id != bear {
			continue
		}
		player = player || of.battle == 0
		atWalker = atWalker || of.battle == walker
	}
	if player {
		t.Fatal("the priced-out player pair is offered")
	}
	if !atWalker {
		t.Fatal("the free planeswalker pair is not offered beside the priced-out player pair")
	}
}
