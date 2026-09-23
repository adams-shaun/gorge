package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestConvokedAmountReadsTheCorpusHeads pins Forge's `Convoked$Amount` count
// head (CR 702.66) on the two REAL corpus carriers, read through their own
// compiled SVar table rather than a hand-written body:
//
//   - Knight-Errant of Eos: `SVar:X:Convoked$Amount`, the plain head;
//   - Ancient Imperiosaur: `SVar:X:Convoked$Amount/Twice`, the same head with
//     the /Twice arithmetic Forge writes INTO the SVar body (no `Count$`
//     prefix, so evalCountExprOK's generic op peel does not see it -- the
//     head's own optional /Op split must handle it).
//
// Both read Object.Convoked, the source-object provenance the pay-time
// FlagConvoked CastInfo folds and the stack->battlefield move preserves, so
// the same source reads the same set before and after the spell resolves.
func TestConvokedAmountReadsTheCorpusHeads(t *testing.T) {
	reg := testutil.CorpusRegistry(t)

	knight, ok := reg.Lookup("Knight-Errant of Eos")
	if !ok {
		t.Fatalf("corpus has no Knight-Errant of Eos")
	}
	imperiosaur, ok := reg.Lookup("Ancient Imperiosaur")
	if !ok {
		t.Fatalf("corpus has no Ancient Imperiosaur")
	}

	// Precondition: the compiled SVar bodies are exactly the shapes under
	// test -- the test can never pass by reading a different expression.
	if got := knight.Faces[0].SVars["X"]; got != "Convoked$Amount" {
		t.Fatalf("precondition: Knight-Errant SVar X = %q, want Convoked$Amount", got)
	}
	if got := imperiosaur.Faces[0].SVars["X"]; got != "Convoked$Amount/Twice" {
		t.Fatalf("precondition: Ancient Imperiosaur SVar X = %q, want Convoked$Amount/Twice", got)
	}

	g := state.NewGame([]string{"you", "them"})
	// Two creatures really convoked the cast, plus one with NO convoke, so a
	// count of 2 is distinguishable from "everything on the battlefield".
	convokers := []struct{ name string }{
		{"Grizzly Bears"}, {"Grizzly Bears"},
	}
	convokedIDs := make([]state.ObjID, 0, len(convokers))
	for _, c := range convokers {
		o := corpusObject(t, reg, g, c.name)
		convokedIDs = append(convokedIDs, o.ID)
	}
	corpusObject(t, reg, g, "Grizzly Bears") // never convoked

	src := g.AddObject(knight, 0)
	src.Zone = state.ZBattlefield
	if len(src.Convoked) != 0 {
		t.Fatalf("precondition: fresh source already has Convoked %v", src.Convoked)
	}
	// The provenance the head reads: the captured convoked ids.
	src.Convoked = convokedIDs
	if len(src.Convoked) != 2 {
		t.Fatalf("fixture precondition: Convoked = %v, want 2 ids", src.Convoked)
	}

	h := &fakeHost{g: g}
	c := &Ctx{Source: src.ID, Controller: 0, SVars: map[string]string{"X": knight.Faces[0].SVars["X"]}}

	// Read through the same Num->SVar indirection the real Dig/CounterNum
	// parameters use: Amount$ X resolves c.SVars["X"].
	amount := sa(t, "SP$ Dig | Amount$ X")
	if got := Num(h, c, amount, "Amount", -1); got != 2 {
		t.Fatalf("Num Amount$ X (Knight-Errant SVar) = %d, want 2", got)
	}

	// The /Twice body: two counters per convoked creature.
	c.SVars["X"] = imperiosaur.Faces[0].SVars["X"]
	if got := Num(h, c, amount, "Amount", -1); got != 4 {
		t.Fatalf("Num Amount$ X (Ancient Imperiosaur /Twice SVar) = %d, want 4", got)
	}

	// The `Count$`-prefixed spelling composes through evalCountExprOK's
	// generic op peel too -- the same 4, not a doubled 8.
	if got, ok := EvalCountOK(h, c, "Count$Convoked$Amount/Twice"); !ok || got != 4 {
		t.Fatalf("Count$Convoked$Amount/Twice literal = (%d,%v), want (4,true)", got, ok)
	}
}

// TestConvokedAmountEmptyAndAbsentAreEvaluatedZero pins the modelled-head
// contract: a cast with no convoke and an absent source both read a
// LEGITIMATE zero (ok=true), never the unresolvable verdict the dispatch
// fallthrough gives. Knight-Errant's "up to X" Dig gate and Ancient
// Imperiosaur's ETB replacement both fail closed at 0, so mistaking the zero
// for "the head did nothing" is exactly the reported bug.
func TestConvokedAmountEmptyAndAbsentAreEvaluatedZero(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	knight, ok := reg.Lookup("Knight-Errant of Eos")
	if !ok {
		t.Fatalf("corpus has no Knight-Errant of Eos")
	}
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	src := g.AddObject(knight, 0)
	src.Zone = state.ZBattlefield

	// Present source, empty provenance: a plain cast.
	c := &Ctx{Source: src.ID, Controller: 0, SVars: map[string]string{"X": "Convoked$Amount"}}
	if got, ok := EvalCountOK(h, c, "Convoked$Amount"); !ok || got != 0 {
		t.Fatalf("empty provenance = (%d,%v), want (0,true)", got, ok)
	}

	// Absent source: no object at the id.
	c.Source = state.ObjID(99999)
	if g.Obj(c.Source) != nil {
		t.Fatalf("fixture precondition: source %d exists", c.Source)
	}
	if got, ok := EvalCountOK(h, c, "Convoked$Amount"); !ok || got != 0 {
		t.Fatalf("absent source = (%d,%v), want (0,true)", got, ok)
	}
	for _, body := range []string{"Convoked$Amount/Unmodelled", "Count$Convoked$Amount/Unmodelled"} {
		if got, ok := EvalCountOK(h, c, body); ok {
			t.Fatalf("unknown operator %q = (%d,%v), want unresolved", body, got, ok)
		}
	}

	// An unknown property fails closed (the unresolvable verdict), so the
	// head never silently counts for a shape it does not model.
	if _, ok := EvalCountOK(h, c, "Convoked$CardPower"); ok {
		t.Fatalf("Convoked$CardPower must fail closed, got ok=true")
	}
}
