package rules

// Forge's card property is spelled `copiedSpell` (CardProperty.java's
// `property.startsWith("copiedSpell")`), and `!copiedSpell` is its negation
// (Card.hasProperty strips the '!'). Stonebinder's Familiar
// (`Card.!token+!copiedSpell`) and The War Doctor
// (`Card.Other+!token+!copiedSpell`) use that spelling; The Heron Moon uses
// gorge's generic `nonCopiedSpell`. All three must count a real card put into
// exile and refuse a copy.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCopiedSpellSpellingsAgree: every corpus spelling of the copy property
// is a recognised predicate, and the three negated forms agree on a real
// card (matches) and on a copy (refused).
func TestCopiedSpellSpellingsAgree(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	src := battlePerm(t, e, 0, corpusCardText(t, "s/stonebinders_familiar.txt"))
	real := battlePerm(t, e, 1, corpusCardText(t, "g/grizzly_bears.txt"))
	copied := battlePerm(t, e, 1, corpusCardText(t, "g/grizzly_bears.txt"))
	e.G.Obj(copied).IsCopy = true
	for _, spec := range []string{"Card.!token+!copiedSpell", "Card.Other+!token+!copiedSpell", "Card.!token+nonCopiedSpell", "Card.copiedSpell"} {
		if u := effects.UnknownPredicates(spec); len(u) != 0 {
			t.Errorf("%s: unknown predicates %v", spec, u)
		}
	}
	for _, spec := range []string{"Card.!token+!copiedSpell", "Card.Other+!token+!copiedSpell", "Card.!token+nonCopiedSpell"} {
		if !e.matchesSpec(spec, real, e.specCtx(src, 0)) {
			t.Errorf("%s must match a card that is not a copy", spec)
		}
		if e.matchesSpec(spec, copied, e.specCtx(src, 0)) {
			t.Errorf("%s must refuse a copy", spec)
		}
	}
	if !e.matchesSpec("Card.copiedSpell", copied, e.specCtx(src, 0)) || e.matchesSpec("Card.copiedSpell", real, e.specCtx(src, 0)) {
		t.Errorf("Card.copiedSpell must match exactly the copy")
	}
}

// TestNegatedCopiedSpellTriggersCountExiledCard: on seat 0's turn a Grizzly
// Bears in the opponent's hand is exiled. Stonebinder's Familiar ("whenever
// one or more cards are put into exile during your turn") gets a +1/+1
// counter, and The War Doctor ("one or more other cards are put into exile
// from anywhere") a time counter; each card runs on its own board (zall
// harness, changeszoneall_test.go).
func TestNegatedCopiedSpellTriggersCountExiledCard(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	for _, c := range []struct{ name, counter string }{
		{"Stonebinder's Familiar", "P1P1"},
		{"The War Doctor", "TIME"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e, _ := zallEngine(t, reg, []string{c.name}, []string{"Grizzly Bears"})
			src := zallEnter(t, e, 0, c.name)
			zallDrain(t, e)
			var bear state.ObjID
			for _, id := range e.G.Zone(state.ZHand, 1) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
					bear = id
					break
				}
			}
			if bear == 0 {
				t.Fatal("fixture: no Grizzly Bears in the opponent's hand")
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZHand, To: state.ZExile})
			zallDrain(t, e)
			if n := e.G.Obj(src).Counter(c.counter); n != 1 {
				t.Fatalf("%s has %d %s counters after a nontoken card was exiled on its controller's turn, want 1 (!copiedSpell never matched)", c.name, n, c.counter)
			}
		})
	}
}
