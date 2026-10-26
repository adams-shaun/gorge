package rules

// CR 107.4f: a Phyrexian pip is paid with one mana of its colour or 2 life;
// CR 119.4 gates the life face on the life total. An activation is offered
// whenever SOME face mix is payable, not only while life covers every pip.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// souleaterOffer builds a main-phase engine with Immolating Souleater on the
// battlefield for seat 0, optionally an untapped Mountain, and the given life.
// It returns the engine and the Souleater's id.
func souleaterOffer(t *testing.T, mountain bool, life int32) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e := crAbortEngine(t, reg, "ur-delver", "Immolating Souleater")
	id := crAbortMove(t, e, 0, "Immolating Souleater", state.ZBattlefield)
	crActivationSA(t, e, id, "Pump", "RP")
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if oid != id && e.G.Obj(oid).Face().Name != "Immolating Souleater" {
			t.Fatalf("fixture: unexpected permanent %s on seat 0's battlefield", e.G.Obj(oid).Face().Name)
		}
	}
	if mountain {
		crAbortMove(t, e, 0, "Mountain", state.ZBattlefield)
	}
	if d := life - e.G.Players[0].Life; d != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: d})
	}
	if got := e.G.Players[0].Life; got != life {
		t.Fatalf("fixture: life = %d, want %d", got, life)
	}
	e.askPriority(0)
	return e, id
}

func souleaterOffered(e *Engine, id state.ObjID) (int, bool) {
	if d := e.Pending(); d != nil {
		for _, opt := range d.Options {
			if opt.Kind == "ability" && opt.Obj == id {
				return opt.Index, true
			}
		}
	}
	return 0, false
}

// souleaterAtPriority reports whether d is a priority decision, i.e. the
// activation's pip and payment asks are over.
func souleaterAtPriority(d *decision.Decision) bool {
	for _, o := range d.Options {
		if o.Kind == "pass" {
			return true
		}
	}
	return false
}

func TestPhyrexianActivationOfferedAtLowLife(t *testing.T) {
	t.Run("one life with a Mountain pays the colour face", func(t *testing.T) {
		e, id := souleaterOffer(t, true, 1)
		idx, ok := souleaterOffered(e, id)
		if !ok {
			t.Fatalf("CR 107.4f: {R/P} activation not offered at 1 life with an untapped Mountain: %+v", e.Pending().Options)
		}
		crAbortAnswer(t, e, "Immolating Souleater", idx)
		// The payment window may ask for the face and/or the Mountain; answer
		// the colour face and any mana source until the ability is on the stack.
		for i := 0; i < 6; i++ {
			d := e.Pending()
			if d == nil || souleaterAtPriority(d) {
				break
			}
			pick := 0
			for _, o := range d.Options {
				if o.Kind == "pay_life" {
					continue
				}
				pick = o.Index
				break
			}
			crAbortAnswer(t, e, "Immolating Souleater", pick)
		}
		if d := e.Pending(); d == nil || !souleaterAtPriority(d) || len(e.G.Stack) == 0 {
			t.Fatalf("activation did not complete onto the stack; stack=%v pending=%+v", e.G.Stack, d)
		}
		for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(oid); o.Face().Name == "Mountain" && !o.Tapped {
				t.Errorf("the Mountain is untapped: the {R} face was not paid with its mana")
			}
		}
		if got := e.G.Players[0].Life; got != 1 {
			t.Errorf("life = %d, want 1 (the pip is paid with {R})", got)
		}
	})
	t.Run("one life with no red source is not offered", func(t *testing.T) {
		e, id := souleaterOffer(t, false, 1)
		if _, ok := souleaterOffered(e, id); ok {
			t.Fatal("CR 119.4: {R/P} offered at 1 life with no red source")
		}
	})
	t.Run("two life with no red source pays two life", func(t *testing.T) {
		e, id := souleaterOffer(t, false, 2)
		idx, ok := souleaterOffered(e, id)
		if !ok {
			t.Fatalf("CR 107.4f: {R/P} activation not offered at 2 life: %+v", e.Pending().Options)
		}
		crAbortAnswer(t, e, "Immolating Souleater", idx)
		for i := 0; i < 6; i++ {
			d := e.Pending()
			if d == nil || souleaterAtPriority(d) {
				break
			}
			pick := 0
			for _, o := range d.Options {
				if o.Kind == "pay_life" {
					pick = o.Index
				}
			}
			crAbortAnswer(t, e, "Immolating Souleater", pick)
		}
		// Paying the 2 life takes the seat to 0, so the game ends there
		// (CR 704.5a) and no priority decision follows; the charge is the
		// observable.
		if got := e.G.Players[0].Life; got != 0 {
			t.Errorf("life = %d, want 0 (2 life paid)", got)
		}
	})
}
