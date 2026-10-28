package rules

// cr-g277: an activated ability whose DB$ Mana sits on a sub-ability of a
// non-Mana head is a mana ability (CR 605.1a). Sacellum Godspeaker's
// "{T}: Reveal any number of creature cards with power 5 or greater from your
// hand. Add {G} for each card revealed this way." must be classified as one,
// stay out of the ordinary ability offer and resolve without the stack.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestSacellumGodspeakerIsManaAbility(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	big := func(name string) *cards.Card {
		return card(t, "Name:"+name+"\nManaCost:5\nTypes:Creature Giant\nPT:5/5\nOracle:x\n")
	}
	small := card(t, "Name:Small Fry\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e := handEngine(t, big("Big One"), big("Big Two"), small)
	id := battlefieldCorpusCard(t, e, 0, lookup(t, reg, "Sacellum Godspeaker"))
	e.G.Obj(id).SummonSick = false

	o := e.G.Obj(id)
	var head *cards.SA
	headIdx := -1
	for i, a := range o.Face().Abilities {
		if a != nil && a.IsActivated() && a.API == "Reveal" {
			head, headIdx = a, i
		}
	}
	if head == nil || cards.ManaChainProduction(head) == nil {
		t.Fatalf("precondition: Godspeaker has no Reveal head reaching a DB$ Mana: %+v", head)
	}
	if !cards.IsManaAbilitySA(head) {
		t.Fatalf("the Reveal head with a DB$ Mana sub is not classified as a mana ability: %q", head.Line)
	}
	if !containsSA(o.Face().ManaAbilities(), head) {
		t.Fatalf("the head is absent from Face.ManaAbilities()")
	}

	e.priorityRound()
	if _, ok := findAbilityOption(e, id, headIdx); ok {
		t.Fatalf("the head is offered as an ordinary ability as well as a mana ability: %+v", e.Pending().Options)
	}
	submitChoices(t, e, activateOption(t, e, id))

	// The reveal ask offers only the two power-5 creatures, the pool is still
	// empty (the production follows the answer) and nothing is on the stack.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("expected the reveal ask over the two power-5 cards, got %+v", d)
	}
	if got := poolString(e.G.Players[0].Pool); got != "" {
		t.Fatalf("pool = %q at the reveal ask, want empty", got)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("the mana activation pushed a stack object: %+v", e.G.Stack)
	}
	submitChoices(t, e, 0, 1)

	if got := poolString(e.G.Players[0].Pool); got != "GG" {
		t.Fatalf("pool = %q after revealing two cards, want GG", got)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("a stack object exists after the mana activation: %+v", e.G.Stack)
	}
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected priority after the activation, got %+v", d)
	}
	if !e.G.Obj(id).Tapped {
		t.Fatalf("Godspeaker is untapped after paying {T}")
	}
}
