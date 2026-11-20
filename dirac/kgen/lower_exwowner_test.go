package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// exiledWithOwnerDropETB replaces the probe's ETB exile (trigger plus body)
// with `repl`: the probes below must leave exactly one exile mover, so the
// mover-count guard cannot refuse them and a refusal can only come from the
// resolution guard.
func exiledWithOwnerDropETB(t *testing.T, repl string) string {
	t.Helper()
	old := `T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigChangeZone | TriggerDescription$ enters: exile up to one creature of toughness 5 or less.
SVar:TrigChangeZone:DB$ ChangeZone | TargetMin$ 0 | TargetMax$ 1 | ValidTgts$ Creature.toughnessLE5 | TgtPrompt$ Pick a creature of toughness 5 or less. | Origin$ Battlefield | Destination$ Exile`
	if !strings.Contains(exiledWithOwnerSrc, old) {
		t.Fatal("probe does not spell the ETB exile block")
	}
	return strings.Replace(exiledWithOwnerSrc, old, repl, 1)
}

// TestExiledWithOwnerMultiExileRefuses: gorge's shared player filter behind
// DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1 matches
// EVERY seat whose exile holds a card exiled with the source and acts on
// each, while the kernel's RefExiledWithOwner is a scalar player ref that
// asks only the first seat in seat order (dirac zone_exiledwith.go
// exiledWithOwnerSeat). The two agree only while at most one seat can match
// -- the source's exile holds at most one linked card, once per source
// object. A carrier whose ETB exile can move more than one card, that
// carries a second exile mover (two movers can stack two links into two
// seats), or that holds its one bounded mover in an ability that can resolve
// more than once for one source object (each resolution stacks another link
// into another seat), refuses rather than lowering a ref the kernel would
// under-ask. The one-card shape (Unidentified Hovership's TargetMax$ 1)
// stays supported: see TestUnidentifiedHovershipExiledWithOwnerLowers.
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
	// resolution and once per object, two movers can stack two linked cards
	// into two seats
	twoMover := exiledWithOwnerSrc + `T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ TrigExileSelf | TriggerDescription$ dies: exile this.
SVar:TrigExileSelf:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ Self
`
	// a repeatable activated exile (the review's probe): one mover bounded
	// to one card, but the ability can resolve again while the link stands,
	// so two activations stack two linked cards into two seats
	activated := exiledWithOwnerDropETB(t, `A:AB$ ChangeZone | Cost$ 2 | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Creature.toughnessLE5 | TgtPrompt$ Pick a creature of toughness 5 or less. | SpellDescription$ Exile target creature.`)
	// a repeatable trigger ("whenever a nontoken creature dies, exile
	// this", kgen's supported spelling for a dies trigger over other
	// cards): the trigger fires again for each other card that dies,
	// stacking a linked card into its owner's exile each time
	repTrig := exiledWithOwnerDropETB(t, `T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.Other | Execute$ TrigExileIt | TriggerDescription$ whenever a nontoken creature dies, exile this.
SVar:TrigExileIt:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | Defined$ Self`)
	for _, tc := range []struct{ name, src, want string }{
		{"two-target ETB exile", two, ""},
		{"a second exile mover", twoMover, ""},
		{"a repeatable activated exile", activated, "non-triggered ability"},
		{"a repeatable dies trigger", repTrig, "fires again"},
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
				if tc.want != "" && !strings.Contains(rf.Reason, tc.want) {
					t.Errorf("%s: refusal %q does not name %q", tc.name, rf.Reason, tc.want)
				}
				refused = true
			}
		}
		if !refused {
			t.Errorf("%s: %s lowered with no ExiledWithSource refusal: %v", tc.name, lc.Name, lc.Refusals)
		}
	}
}
