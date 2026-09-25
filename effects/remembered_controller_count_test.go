package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// rememberedControllerBoard builds a four-seat game with three battlefield
// objects: obj0 controlled by seat 0, obj1 by seat 1 and obj2 by seat 1 too
// (the same controller twice, so distinct-controller semantics can be told
// from per-object counting). It returns the game and the three ids.
func rememberedControllerBoard(t *testing.T) (*state.Game, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	g := state.NewGame(names(4))
	card := mkCard(t, "Name:Remembered\nTypes:Creature\nPT:1/1\nOracle:x\n")
	obj0 := g.AddObject(card, 0).ID
	obj1 := g.AddObject(card, 1).ID
	obj2 := g.AddObject(card, 1).ID
	// PRECONDITION: the two controllers really differ and obj1/obj2 really
	// share a controller, or the distinct-count assertion proves nothing.
	if g.Obj(obj0).Controller != 0 || g.Obj(obj1).Controller != 1 || g.Obj(obj2).Controller != 1 {
		t.Fatalf("fixture controllers = %d,%d,%d, want 0,1,1",
			g.Obj(obj0).Controller, g.Obj(obj1).Controller, g.Obj(obj2).Controller)
	}
	return g, obj0, obj1, obj2
}

// TestPlayerCountRememberedControllerAmountCountsDistinctObjectControllers
// pins Forge's PlayerCountRememberedController$Amount: the DISTINCT
// controllers of the remembered OBJECTS. Tempt with Mayhem reads it as
// "plus an additional time for each opponent who copied the spell this way",
// so two remembered objects sharing one controller must count once, a
// remembered PLAYER entry (not an object) must not count at all, and an
// object id that no longer resolves must not invent a seat.
func TestPlayerCountRememberedControllerAmountCountsDistinctObjectControllers(t *testing.T) {
	g, obj0, obj1, obj2 := rememberedControllerBoard(t)
	h := &fakeHost{g: g}
	c := &Ctx{
		Controller: 0,
		Remembered: []state.Target{
			{Obj: obj0},
			{Obj: obj1},
			{Obj: obj2},                 // same controller as obj1: one seat
			{Player: 2, IsPlayer: true}, // a remembered player is not an object
			{Obj: 9999},                 // an unresolvable id contributes nothing
		},
	}
	got, ok := EvalCountOK(h, c, "Count$PlayerCountRememberedController$Amount")
	if !ok {
		t.Fatal("PlayerCountRememberedController$Amount reported UNRESOLVED; want the modelled group count")
	}
	// Distinct controllers of resolvable objects = {0, 1} = 2. Per-object
	// would be 3; remembering the player too would be 4.
	if got != 2 {
		t.Fatalf("PlayerCountRememberedController$Amount = %d, want 2 (distinct resolvable object controllers)", got)
	}
	// The /Op suffix that Tempt carries must still compose on top.
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountRememberedController$Amount/Plus.1"); !ok || got != 3 {
		t.Fatalf("PlayerCountRememberedController$Amount/Plus.1 = (%d, %v), want (3, true)", got, ok)
	}
	// An empty remembered set is a modelled zero, not an unresolved head.
	if got, ok := EvalCountOK(h, &Ctx{Controller: 0}, "Count$PlayerCountRememberedController$Amount"); !ok || got != 0 {
		t.Fatalf("empty PlayerCountRememberedController$Amount = (%d, %v), want (0, true)", got, ok)
	}
}

// TestPlayerCountRememberedControllerScalarProperties pins the scalar
// property reads over the same distinct controller set (Eradicate / Sowing
// Salt / Splinter's NumInHand/NumInLib): the hand and library sizes of the
// remembered objects' controllers, summed once per distinct controller.
func TestPlayerCountRememberedControllerScalarProperties(t *testing.T) {
	g, obj0, obj1, obj2 := rememberedControllerBoard(t)
	// Hands: the remembered objects are controlled by seats 0 and 1; give
	// them different sizes so a wrong membership shows up as a wrong sum.
	filler := mkCard(t, "Name:Filler\nTypes:Sorcery\nOracle:x\n")
	hand := func(p state.PlayerID, n int) {
		ids := make([]state.ObjID, 0, n)
		for i := 0; i < n; i++ {
			ids = append(ids, g.AddObject(filler, p).ID)
		}
		g.SetZone(state.ZHand, p, ids)
	}
	hand(0, 5)
	hand(1, 3)
	if len(g.Zone(state.ZHand, 0)) != 5 || len(g.Zone(state.ZHand, 1)) != 3 {
		t.Fatalf("fixture hands = %d,%d, want 5,3", len(g.Zone(state.ZHand, 0)), len(g.Zone(state.ZHand, 1)))
	}
	h := &fakeHost{g: g}
	c := &Ctx{
		Controller: 0,
		Remembered: []state.Target{{Obj: obj0}, {Obj: obj1}, {Obj: obj2}},
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountRememberedController$CardsInHand"); !ok || got != 8 {
		t.Fatalf("PlayerCountRememberedController$CardsInHand = (%d, %v), want (8, true): seats 0+1 once each", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountRememberedController$CardsInLibrary"); !ok || got != 0 {
		t.Fatalf("PlayerCountRememberedController$CardsInLibrary = (%d, %v), want (0, true)", got, ok)
	}
	// A property the group does not model stays unresolved (fail closed).
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountRememberedController$HighestLifeTotal"); ok {
		t.Fatalf("PlayerCountRememberedController$HighestLifeTotal reported EVALUATED as %d; want unresolved", got)
	}
}

// TestPlayerCountRememberedControllerHasPropertyOpponent pins Faerie Slumber
// Party's `HasPropertyOpponent`: the remembered objects' DISTINCT controllers
// that match the shared player grammar's Opponent clause, relative to the
// resolving controller (one per opponent who controlled a remembered
// creature, not one per creature).
func TestPlayerCountRememberedControllerHasPropertyOpponent(t *testing.T) {
	g, obj0, obj1, obj2 := rememberedControllerBoard(t)
	h := &fakeHost{g: g}
	// obj1 and obj2 share seat 1; both are opponents of seat 0.
	c := &Ctx{Controller: 0, Remembered: []state.Target{{Obj: obj0}, {Obj: obj1}, {Obj: obj2}}}
	got, ok := EvalCountOK(h, c, "Count$PlayerCountRememberedController$HasPropertyOpponent")
	if !ok {
		t.Fatal("PlayerCountRememberedController$HasPropertyOpponent reported UNRESOLVED")
	}
	// Controllers {0, 1}; opponents of seat 0 = {1} = 1. Per-object would be 2.
	if got != 1 {
		t.Fatalf("HasPropertyOpponent (controller 0) = %d, want 1 (distinct opponent controllers)", got)
	}
	// Seen from seat 1, the same controllers {0,1}: opponents are {0} = 1.
	// This proves the filter is relative to the resolving controller.
	c.Controller = 1
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountRememberedController$HasPropertyOpponent"); !ok || got != 1 {
		t.Fatalf("HasPropertyOpponent (controller 1) = (%d, %v), want (1, true)", got, ok)
	}
}

// TestCopyControllerForRemembered pins CopySpellAbility's Controller$
// Remembered selector (Tempt with Mayhem's per-opponent copy): a remembered
// PLAYER entry names the copy's controller directly (the RepeatEach loop
// binds its current subject as Remembered), a remembered OBJECT names its
// controller, and an empty or unresolvable binding fails closed so the
// caller keeps the resolving controller.
func TestCopyControllerForRemembered(t *testing.T) {
	g, obj0, obj1, _ := rememberedControllerBoard(t)
	// obj0 is controlled by seat 0, obj1 by seat 1 -- different, so the
	// precedence between a remembered player and a remembered object is
	// observable.
	if g.Obj(obj0).Controller == g.Obj(obj1).Controller {
		t.Fatal("fixture objects share a controller; the precedence assertion below cannot fail")
	}
	// A remembered player wins over the objects remembered before it: this
	// is Tempt's RepeatEach shape (prior copies first, the current subject
	// last).
	c := &Ctx{Controller: 3, Remembered: []state.Target{
		{Obj: obj0}, {Obj: obj1}, {Player: 2, IsPlayer: true},
	}}
	if got, ok := copyControllerFor(g, c, "Remembered"); !ok || got != 2 {
		t.Fatalf("copyControllerFor(Remembered) = (%d, %v), want (2, true) -- the remembered player", got, ok)
	}
	if got, ok := copyControllerFor(g, c, "RememberedController"); !ok || got != 2 {
		t.Fatalf("copyControllerFor(RememberedController) = (%d, %v), want (2, true)", got, ok)
	}
	// With no remembered player, the first resolvable remembered object's
	// controller is the answer.
	c = &Ctx{Controller: 3, Remembered: []state.Target{{Obj: obj0}, {Obj: obj1}}}
	if got, ok := copyControllerFor(g, c, "Remembered"); !ok || got != 0 {
		t.Fatalf("copyControllerFor(Remembered) over objects = (%d, %v), want (0, true)", got, ok)
	}
	// An unresolvable object id fails closed: the caller keeps its own
	// controller rather than inventing a seat.
	c = &Ctx{Controller: 3, Remembered: []state.Target{{Obj: 9999}}}
	if got, ok := copyControllerFor(g, c, "Remembered"); ok || got != 0 {
		t.Fatalf("copyControllerFor(unresolvable Remembered) = (%d, %v), want (0, false)", got, ok)
	}
	// An empty binding fails closed too.
	if got, ok := copyControllerFor(g, &Ctx{Controller: 3}, "Remembered"); ok || got != 0 {
		t.Fatalf("copyControllerFor(empty Remembered) = (%d, %v), want (0, false)", got, ok)
	}
}
