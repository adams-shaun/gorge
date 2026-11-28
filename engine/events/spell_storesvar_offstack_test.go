package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSpellStoreSVarClearedOffStack pins the CR 400.7 clear in move(): a spell
// that leaves the stack for anything but the battlefield is a new object, so
// the api:StoreSVar numbers its resolution wrote (Object.RuntimeSVars, the
// overlay the ChangeNum$ X reads go through) die with the stack entry. The
// carrier shape is Disorienting Choice's keep/exile tally: a spell that stored
// X = 2 and returns to the graveyard, hand, exile or library must present a
// clean store when that same object is cast again; a permanent spell keeps the
// store onto the battlefield (the ETB "if it was kicked" family reads
// cast-time values off the permanent).

const (
	storesvarProbeSorcery  = "Name:Choice\nManaCost:3 G\nTypes:Sorcery\nOracle:x\n"
	storesvarProbeCreature = "Name:Golem\nManaCost:3\nTypes:Creature Golem\nPT:1/1\nOracle:x\n"
)

// storesvarCast stores X = 2 on a fresh copy of src: the card goes to seat 0's
// hand, is put on the stack and takes one StoreSVar write. The store and the
// stack zone are checked, so a caller's assertion about the exit is about the
// clear, not about a missing write.
func storesvarCast(t *testing.T, src string) (*state.Game, state.ObjID) {
	t.Helper()
	g, id := gameWithOneCardSrc(t, src)
	Apply(g, Event{Kind: PutOnStack, Obj: id, From: state.ZHand, To: state.ZStack})
	Apply(g, Event{Kind: StoreSVar, Obj: id, Text: "X", Amount: 2})
	o := g.Obj(id)
	if o == nil {
		t.Fatal("spell object gone")
	}
	if o.Zone != state.ZStack {
		t.Fatalf("spell zone = %v, want the stack", o.Zone)
	}
	if o.RuntimeSVars["X"] != 2 {
		t.Fatalf("StoreSVar precondition: RuntimeSVars = %v, want X = 2", o.RuntimeSVars)
	}
	return g, id
}

func TestSpellStoreSVarClearedOffStack(t *testing.T) {
	for _, to := range []state.Zone{state.ZGraveyard, state.ZHand, state.ZExile, state.ZLibrary} {
		g, id := storesvarCast(t, storesvarProbeSorcery)
		Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: to})
		o := g.Obj(id)
		if o == nil {
			t.Fatalf("stack->%v: object gone", to)
		}
		if o.RuntimeSVars != nil {
			t.Errorf("stack->%v: RuntimeSVars = %v, want the store cleared (CR 400.7)", to, o.RuntimeSVars)
		}
	}
	// The control: the stack->battlefield path keeps the store (the same
	// clear must not fire for a resolving permanent spell).
	g, id := storesvarCast(t, storesvarProbeCreature)
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: state.ZBattlefield})
	o := g.Obj(id)
	if o == nil {
		t.Fatal("stack->battlefield: object gone")
	}
	if o.RuntimeSVars["X"] != 2 {
		t.Errorf("stack->battlefield: RuntimeSVars = %v, want X = 2 kept onto the permanent", o.RuntimeSVars)
	}
}
