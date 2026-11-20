package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestExiledWithOwnerMultiExileRefuses: gorge's shared player filter behind
// DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1 matches
// EVERY seat whose exile holds a card exiled with the source and acts on
// each, while the kernel's RefExiledWithOwner is a scalar player ref that
// asks only the first seat in seat order (dirac zone_exiledwith.go
// exiledWithOwnerSeat). The two agree only while at most one seat can match
// -- the source's exile holds at most one linked card. A carrier whose ETB
// exile can move more than one card, or that carries a second exile mover
// (two resolutions can stack two links into two seats), refuses rather than
// lowering a ref the kernel would under-ask. The one-card shape (Unidentified
// Hovership's TargetMax$ 1) stays supported: see
// TestUnidentifiedHovershipExiledWithOwnerLowers.
func TestExiledWithOwnerMultiExileRefuses(t *testing.T) {
	// the ETB exile takes up to two creatures: two seats' exiles can each
	// hold a linked card when their owners differ, and gorge would manifest
	// dread for both owners while the scalar ref asks only the first seat
	two := strings.Replace(exiledWithOwnerSrc, "TargetMax$ 1", "TargetMax$ 2", 1)
	if two == exiledWithOwnerSrc {
		t.Fatal("probe does not spell TargetMax$ 1")
	}
	// a second exile mover (a dies-trigger exile of the vehicle itself, the
	// Promise shape exwCheck allows): even bounded to one card per
	// resolution, two movers can stack two linked cards into two seats
	twoMover := exiledWithOwnerSrc + `T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ TrigExileSelf | TriggerDescription$ dies: exile this.
SVar:TrigExileSelf:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ Self
`
	for _, tc := range []struct{ name, src string }{
		{"two-target ETB exile", two},
		{"a second exile mover", twoMover},
	} {
		reg := testutil.CorpusRegistry(t)
		card, diags := cards.ParseBytes("exiled_with_owner_multi.txt", []byte(tc.src))
		if card == nil {
			t.Fatalf("%s: script did not parse: %v", tc.name, diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("%s: link: %v", tc.name, d)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		refused := false
		for _, rf := range lc.Refusals {
			if strings.Contains(rf.Reason, "ExiledWithSource player ref") {
				refused = true
			}
		}
		if !refused {
			t.Errorf("%s: %s lowered with no ExiledWithSource refusal: %v", tc.name, lc.Name, lc.Refusals)
		}
	}
}
