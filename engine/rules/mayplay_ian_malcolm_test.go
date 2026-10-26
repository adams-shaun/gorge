package rules

// mayplay_ian_malcolm_test.go pins Ian Malcolm, Chaotician's whole cast-time
// text end to end (CR 601.2, CR 611.3). Printed: "Whenever a player draws
// their second card each turn, that player exiles the top card of their
// library. During each player's turn, that player may cast a spell from among
// the cards they don't own exiled with Ian Malcolm, Chaotician, and mana of
// any type can be spent to cast it." The exile is the Drawn trigger's effect
// (it is how cards get exiled WITH Ian), not a rider on the cast: the grant
// is `MayPlayPlayer$ ActivePlayer` over `Card.nonLand+ExiledWithSource+!OwnedBy
// ActivePlayer` with MayPlayIgnoreType$ and nothing fires when a card is cast
// through it. Every exile here is made by Ian's real trigger.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ianGame is a two-seat game with Ian on seat 0's battlefield, stopped at
// turn 3 (seat 0) main 1. Seat 0 holds Savannah Lions and a Grizzly Bears;
// seat 1 holds a Grizzly Bears. It returns the engine, its Config, the ids
// by seat and name, and Ian's id.
func ianGame(t *testing.T) (*Engine, Config, [2]map[string][]state.ObjID, state.ObjID) {
	t.Helper()
	e, cfg, ids := mpRiderGame(t,
		[]string{"Savannah Lions", "Grizzly Bears"}, []string{"Grizzly Bears"},
		[]string{"Ian Malcolm, Chaotician"}, nil)
	driveToTurn3Main(t, e)
	return e, cfg, ids, ids[0]["Ian Malcolm, Chaotician"][0]
}

// ianExileSecondDraw has p draw their second card of the turn (Ian's Drawn
// trigger, Number$ 2) and settles the trigger. When pick is non-zero that hand
// card is first put second from the top of p's library, so the card Ian exiles
// is pick; otherwise the library is left as is. It returns the exiled card and
// asserts it is exiled with Ian.
func ianExileSecondDraw(t *testing.T, e *Engine, p state.PlayerID, ian, pick state.ObjID) state.ObjID {
	t.Helper()
	if pick != 0 {
		e.emit(events.Event{Kind: events.MoveZone, Obj: pick, From: state.ZHand, To: state.ZLibrary, Player: p})
		lib := slices.DeleteFunc(slices.Clone(e.G.Zone(state.ZLibrary, p)), func(id state.ObjID) bool { return id == pick })
		e.emit(events.Event{Kind: events.LibraryOrder, Player: p, IDs: slices.Concat(lib[:1], []state.ObjID{pick}, lib[1:])})
	}
	lib := e.G.Zone(state.ZLibrary, p)
	want := lib[1]
	if pick != 0 && want != pick {
		t.Fatalf("precondition: second library card %d, want the picked %d", want, pick)
	}
	e.drawCard(p)
	mpRiderSettle(t, e)
	if o := e.G.Obj(want); o.Zone != state.ZExile || o.ExiledWith != ian {
		t.Fatalf("precondition: second draw did not exile card %d with Ian (zone %s, ExiledWith %d)", want, o.Zone, o.ExiledWith)
	}
	return want
}

// ianOffers reports whether the pending priority decision offers any
// cast/play of id (a land would be a "play" option).
func ianOffers(e *Engine, id state.ObjID) bool {
	d := e.Pending()
	if d == nil {
		return false
	}
	for _, o := range d.Options {
		if o.Obj == id && (o.Kind == "cast" || o.Kind == "play") {
			return true
		}
	}
	return false
}

// ianAt drives to turn/seat main 1, gives that seat mana and returns at its
// priority.
func ianAt(t *testing.T, e *Engine, turn int32, seat state.PlayerID, mana string) {
	t.Helper()
	mpRiderDrive(t, e, turn, seat, state.StepMain1)
	addManaQuiet(t, e, seat, mana)
	if e.G.Active != seat || e.G.Priority != seat {
		t.Fatalf("precondition: active %d priority %d, want seat %d", e.G.Active, e.G.Priority, seat)
	}
}

// The beneficiary is the ACTIVE player, not Ian's controller: a card seat 0
// owns and Ian exiled is cast by seat 1 on seat 1's turn, and a card seat 1
// owns is cast by seat 0 on seat 0's turn. (A card is never offered to its own
// owner: TestIanMalcolmExcludesOwnCards.)
func TestIanMalcolmActivePlayerMayCastOwnerForeignExile(t *testing.T) {
	e, cfg, ids, ian := ianGame(t)
	lions := ids[0]["Savannah Lions"][0]
	bears := ids[1]["Grizzly Bears"][0]

	// Turn 3 (seat 0): seat 0's second draw exiles seat 0's Lions.
	if got := ianExileSecondDraw(t, e, 0, ian, lions); got != lions {
		t.Fatalf("exiled %d, want Lions %d", got, lions)
	}
	// Turn 4 (seat 1): seat 1's second draw exiles seat 1's Bears.
	ianAt(t, e, 4, 1, "R")
	if got := ianExileSecondDraw(t, e, 1, ian, bears); got != bears {
		t.Fatalf("exiled %d, want Bears %d", got, bears)
	}
	if e.G.Obj(lions).Owner == e.G.Obj(bears).Owner {
		t.Fatalf("precondition: both exiled cards owned by seat %d", e.G.Obj(lions).Owner)
	}
	e.priorityRound()
	if !ianOffers(e, lions) {
		t.Fatalf("seat 1 (active, not Ian's controller) not offered seat 0's exiled Lions: %v", mpRiderLabels(e))
	}
	if ianOffers(e, bears) {
		t.Fatalf("seat 1 offered its own exiled Bears")
	}

	// Turn 5 (seat 0): the same grant now serves seat 0 for the card seat 1 owns.
	ianAt(t, e, 5, 0, "RR")
	if !ianOffers(e, bears) {
		t.Fatalf("seat 0 (active) not offered seat 1's exiled Bears: %v", mpRiderLabels(e))
	}
	if ianOffers(e, lions) {
		t.Fatalf("seat 0 offered its own exiled Lions")
	}
	replayCheck(t, e, cfg)
}

// Casting through the grant has no rider: nothing leaves either library and
// no exile follows the cast.
func TestIanMalcolmGrantHasNoCastRider(t *testing.T) {
	e, cfg, ids, ian := ianGame(t)
	lions := ids[0]["Savannah Lions"][0]
	ianExileSecondDraw(t, e, 0, ian, lions)
	ianAt(t, e, 4, 1, "R")

	libBefore := [2][]state.ObjID{slices.Clone(e.G.Zone(state.ZLibrary, 0)), slices.Clone(e.G.Zone(state.ZLibrary, 1))}
	exileBefore := [2]int{len(e.G.Zone(state.ZExile, 0)), len(e.G.Zone(state.ZExile, 1))}
	marker := len(e.L.Events)
	if len(libBefore[1]) == 0 {
		t.Fatalf("precondition: seat 1's library is empty")
	}
	if !ianOffers(e, lions) {
		t.Fatalf("precondition: Lions not offered to seat 1: %v", mpRiderLabels(e))
	}
	mpRiderCastResolve(t, e, lions)

	if o := e.G.Obj(lions); o.Zone != state.ZBattlefield || o.Controller != 1 || o.Owner != 0 {
		t.Fatalf("cast Lions zone %s controller %d owner %d, want battlefield, 1, 0", o.Zone, o.Controller, o.Owner)
	}
	for p := range libBefore {
		if !slices.Equal(libBefore[p], e.G.Zone(state.ZLibrary, state.PlayerID(p))) {
			t.Fatalf("seat %d library changed by the cast: %v -> %v", p, libBefore[p], e.G.Zone(state.ZLibrary, state.PlayerID(p)))
		}
	}
	// Lions left exile 0 (it was cast); nothing was exiled in its place.
	if got := [2]int{len(e.G.Zone(state.ZExile, 0)), len(e.G.Zone(state.ZExile, 1))}; got != [2]int{exileBefore[0] - 1, exileBefore[1]} {
		t.Fatalf("exile sizes %v after the cast, want %v", got, [2]int{exileBefore[0] - 1, exileBefore[1]})
	}
	for _, ev := range e.L.Events[marker:] {
		if ev.Kind == events.MoveZone && ev.To == state.ZExile {
			t.Fatalf("an exile followed the cast: %+v", ev)
		}
	}
	replayCheck(t, e, cfg)
}

// !OwnedBy ActivePlayer and Card.nonLand: a card is not offered to its owner
// on their own turn, and a land exiled with Ian is offered to nobody.
func TestIanMalcolmExcludesOwnCards(t *testing.T) {
	t.Run("nonland owned by the active player", func(t *testing.T) {
		e, cfg, ids, ian := ianGame(t)
		lions := ids[0]["Savannah Lions"][0]
		ianExileSecondDraw(t, e, 0, ian, lions)
		addManaQuiet(t, e, 0, "RRRR")
		if e.G.Active != 0 || e.G.Obj(lions).Owner != 0 {
			t.Fatalf("precondition: active %d, Lions owner %d", e.G.Active, e.G.Obj(lions).Owner)
		}
		if ianOffers(e, lions) {
			t.Fatalf("Lions offered to its owner on their own turn: %v", mpRiderLabels(e))
		}
		// The same card is offered to the other seat on its turn, so the
		// withholding above is the owner test and not a dead grant.
		ianAt(t, e, 4, 1, "R")
		if !ianOffers(e, lions) {
			t.Fatalf("precondition: Lions not offered to seat 1: %v", mpRiderLabels(e))
		}
		replayCheck(t, e, cfg)
	})
	t.Run("land exiled with Ian", func(t *testing.T) {
		e, cfg, ids, ian := ianGame(t)
		lions := ids[0]["Savannah Lions"][0]
		// Turn 3: Lions is exiled (offered to seat 1 next turn). Turn 5: seat
		// 0's natural second card, a Mountain, is exiled with Ian too.
		ianExileSecondDraw(t, e, 0, ian, lions)
		mpRiderDrive(t, e, 5, 0, state.StepMain1)
		land := ianExileSecondDraw(t, e, 0, ian, 0)
		if !e.G.Obj(land).Face().IsLand() {
			t.Fatalf("precondition: exiled card %q is not a land", e.G.Obj(land).Face().Name)
		}
		ianAt(t, e, 6, 1, "RR")
		if !ianOffers(e, lions) {
			t.Fatalf("precondition: Lions not offered to seat 1: %v", mpRiderLabels(e))
		}
		if ianOffers(e, land) {
			t.Fatalf("exiled land offered to seat 1: %v", mpRiderLabels(e))
		}
		replayCheck(t, e, cfg)
	})
}

// MayPlayIgnoreType$: the exiled Bears ({1}{G}) is offered with only {R}{R},
// while a Bears in hand is not.
func TestIanMalcolmAnyManaType(t *testing.T) {
	e, cfg, ids, ian := ianGame(t)
	bears1 := ids[1]["Grizzly Bears"][0]
	ianAt(t, e, 4, 1, "")
	ianExileSecondDraw(t, e, 1, ian, bears1)
	handBears := ids[0]["Grizzly Bears"][0]
	ianAt(t, e, 5, 0, "RR")
	if o := e.G.Obj(handBears); o.Zone != state.ZHand || o.Owner != 0 {
		t.Fatalf("precondition: seat 0's Bears in %s", o.Zone)
	}
	if e.G.Players[0].Pool.Total() != 2 {
		t.Fatalf("precondition: pool %v, want exactly RR", e.G.Players[0].Pool)
	}
	if ianOffers(e, handBears) {
		t.Fatalf("precondition: hand Bears castable from {R}{R}: %v", mpRiderLabels(e))
	}
	o, ok := mpRiderOffer(e, bears1)
	if !ok {
		t.Fatalf("exiled Bears not offered for {R}{R}: %v", mpRiderLabels(e))
	}
	submitChoices(t, e, o.Index)
	e.resolveTop()
	e.priorityRound()
	if g := e.G.Obj(bears1); g.Zone != state.ZBattlefield || g.Controller != 0 {
		t.Fatalf("Bears zone %s controller %d, want battlefield under seat 0", g.Zone, g.Controller)
	}
	replayCheck(t, e, cfg)
}
