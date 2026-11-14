package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// exileXFixedLower lowers one synthetic card and returns its lowering.
func exileXFixedLower(t *testing.T, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("exilexfixed_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Exile X Fixed Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false)
}

const exileXFixedCard = `Name:Exile X Fixed Probe
ManaCost:1 U
Types:Creature Merfolk
PT:1/1
A:AB$ Draw | Cost$ T ExileFromGrave<X/Card> | NumCards$ 1 | SpellDescription$ Draw a card.
SVar:X:%s
Oracle:{T}, Exile cards from your graveyard equal to the number of opponents you have: Draw a card.
`

// TestExileXFixedCountLowersAsOneOpponent: an activated ability's
// ExileFromGrave<X/Spec> whose SVar X is PlayerCountOpponents$Amount (Araumi
// of the Dead Tide) is a fixed count in gorge (pay.FixLifeXCost drops the
// announcement), so it lowers as CostExileGrave of one card on a two-seat
// card, not the announced N 0 form. Any other non-xPaid SVar and the xPaid
// announcement on an activated ability stay refused.
func TestExileXFixedCountLowersAsOneOpponent(t *testing.T) {
	lc := exileXFixedLower(t, strings.Replace(exileXFixedCard, "%s", "PlayerCountOpponents$Amount", 1))
	if len(lc.Refusals) != 0 {
		t.Fatalf("refused: %v", lc.Refusals)
	}
	if !lc.TwoSeat {
		t.Error("the opponents' count read as one must mark the card two-seat")
	}
	var c *lCost
	for i := range lc.Abilities {
		if lc.Abilities[i].Kind == akActivated {
			c = &lc.Abilities[i].Cost
		}
	}
	if c == nil {
		t.Fatal("no activated ability lowered")
	}
	if c.ExileG == nil || c.ExileGN != 1 || c.ExileGX {
		t.Errorf("cost = ExileG %v N %d X %v, want the fixed one-card exile", c.ExileG != nil, c.ExileGN, c.ExileGX)
	}

	for _, body := range []string{"Count$ValidGraveyard Card.YouOwn", "Count$xPaid", "PlayerCountPlayers$Amount"} {
		lc := exileXFixedLower(t, strings.Replace(exileXFixedCard, "%s", body, 1))
		if keclReason(lc.Refusals) == "" {
			t.Errorf("SVar X %q on an activated ability lowered, want refused", body)
		}
	}
}
