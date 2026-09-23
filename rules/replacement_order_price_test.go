package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// An unresolved ReplaceCounter body cannot be elected ahead of a real one.
// The corpus currently prices both of these cards; a copied body with an
// unresolved Amount models a missing SVar without changing the compiled corpus.
func TestAddCounterOrderExcludesUnpriceableBody(t *testing.T) {
	reg := sharedCorpus(t)
	e := newSeats(t, 2)
	e.pending = nil
	scales := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Hardened Scales"))
	branching := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Branching Evolution"))
	bear := onBoard(t, e, 0, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	for _, id := range []state.ObjID{scales, branching, bear} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: %d must be on the battlefield", id)
		}
	}
	ev := events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 1}
	first := &e.G.Obj(scales).Face().Repls[0]
	second := &e.G.Obj(branching).Face().Repls[0]
	if !e.replacementMatches(*first, scales, ev) || !e.replacementMatches(*second, branching, ev) {
		t.Fatal("precondition: both corpus replacement lines must match the proposed counter placement")
	}
	// Copy the corpus's second replacement without mutating any game or card
	// state. The first card still prices to 2, while this body's missing SVar
	// has no amount to apply and must not be an order option.
	unpriced := *second
	body := *second.With
	body.Params = map[string]string{"Amount": "NoSuchSVar"}
	unpriced.With = &body
	good := replMatch{id: scales, repl: first}
	bad := replMatch{id: branching, repl: &unpriced}
	if n, ok := e.priceAddCounterBody(ev, good, ev.Amount); !ok || n != 2 || n == ev.Amount {
		t.Fatalf("precondition: Hardened Scales prices 1 to 2, got %d, %v", n, ok)
	}
	if n, ok := e.priceAddCounterBody(ev, bad, ev.Amount); ok || n != ev.Amount {
		t.Fatalf("precondition: missing SVar must be unpriceable, got %d, %v", n, ok)
	}
	rewritten, handled := e.applyAddCounterReplacements(ev, []replMatch{good, bad})
	if handled || rewritten.Amount != 2 || e.Pending() != nil || len(e.replChoices) != 0 {
		t.Fatalf("unpriceable body was offered or affected the counter: event=%+v handled=%v pending=%+v queue=%+v", rewritten, handled, e.Pending(), e.replChoices)
	}
}
