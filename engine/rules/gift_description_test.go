package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// giftAskFor casts spell and returns the pending CR 702.174a gift ask.
func giftAskFor(t *testing.T, e *Engine, spell state.ObjID, mana string) *decision.Decision {
	t.Helper()
	addMana(t, e, 0, mana)
	submitChoices(t, e, castCardOption(t, e, spell).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 ||
		d.Options[0].Kind != "gift_decline" || d.Options[1].Kind != "gift_promise" {
		t.Fatalf("gift ask not posed after casting: %+v", d)
	}
	if d.Options[1].Player != 1 {
		t.Fatalf("precondition: gift_promise names seat %d, want 1", d.Options[1].Player)
	}
	return d
}

func assertGiftAskText(t *testing.T, e *Engine, d *decision.Decision, prompt, promise string) {
	t.Helper()
	if d.Prompt != prompt {
		t.Fatalf("gift prompt = %q, want %q", d.Prompt, prompt)
	}
	if d.Options[0].Label != "Don't promise a gift" {
		t.Fatalf("decline label = %q, want unchanged", d.Options[0].Label)
	}
	want := "Promise " + pay.SeatName(e.G, 1) + " " + promise
	if d.Options[1].Label != want {
		t.Fatalf("promise label = %q, want %q", d.Options[1].Label, want)
	}
}

// TestGiftDescriptionNamesTheCard: Dewdrop Cure's GiftAbility carries
// GiftDescription$ a card, so the ask says what is promised.
func TestGiftDescriptionNamesTheCard(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 7301, corpusCardText(t, "d/dewdrop_cure.txt"))
	d := giftAskFor(t, e, spell, "CCW")
	assertGiftAskText(t, e, d, "Promise a gift (a card)?", "a card")
}

// TestGiftDescriptionNamesTheTreasure: Blooming Blast promises a Treasure.
func TestGiftDescriptionNamesTheTreasure(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 7302, corpusCardText(t, "b/blooming_blast.txt"))
	bearPermanent(t, e, 1)
	d := giftAskFor(t, e, spell, "CR")
	assertGiftAskText(t, e, d, "Promise a gift (a Treasure)?", "a Treasure")
}

// TestGiftDescriptionAbsentKeepsText: a Gift face whose GiftAbility lacks
// GiftDescription$ keeps the generic text byte for byte.
func TestGiftDescriptionAbsentKeepsText(t *testing.T) {
	t.Parallel()
	const src = `Name:Plain Gift Test
ManaCost:W
Types:Sorcery
K:Gift
SVar:GiftAbility:DB$ Draw | Defined$ Promised
A:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ You gain 1 life.
`
	e, _, spell := newFixtureDeck(t, 7303, src)
	d := giftAskFor(t, e, spell, "W")
	assertGiftAskText(t, e, d, "Promise a gift?", "a gift")
}
