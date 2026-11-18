package rules

// Trigger modes Countered (Baral), MilledOnce (Zellix) and AbilityResolves
// (Tom Bombadil, Narci), each played through a real game on the compiled
// corpus: the printed cards are cast or activated, resolve, and the assertions
// read the resulting state.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// trigAAnswer answers every pending decision for a bounded number of steps
// until the stack and the pending-trigger queue are empty: pass priority,
// say yes to an optional ask when one is offered, else take the first Min
// options.
func trigAAnswer(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over; i++ {
		if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 0 {
			return
		}
		d := e.Pending()
		if d == nil {
			e.priorityRound()
			continue
		}
		sagaAnswer(t, e, d)
	}
	t.Fatalf("trigAAnswer did not settle (stack %d, pending %d)", len(e.G.Stack), len(e.pendingTriggers))
}

// trigACount counts the logged events of kind k from index from on.
func trigACount(e *Engine, from int, k events.Kind, keep func(events.Event) bool) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == k && (keep == nil || keep(ev)) {
			n++
		}
	}
	return n
}

// trigAOrderTop puts ids on top of player p's library, in order, through one
// real LibraryOrder event.
func trigAOrderTop(e *Engine, p state.PlayerID, ids ...state.ObjID) {
	skip := map[state.ObjID]bool{}
	for _, id := range ids {
		skip[id] = true
	}
	order := append([]state.ObjID(nil), ids...)
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		if !skip[id] {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: p, IDs: order})
}

// TestEDHTriggerModesACounteredBaralLoots: Baral, Chief of Compliance loots
// when a spell its controller controls counters a spell. Seat 0 casts Giant
// Growth, then Counterspell on it; when Counterspell resolves Baral's trigger
// draws a card and discards a card.
func TestEDHTriggerModesACounteredBaralLoots(t *testing.T) {
	t.Parallel()
	e, _ := mfEngine(t, []string{"Baral, Chief of Compliance", "Giant Growth", "Counterspell", "Grizzly Bears"}, nil)
	searchMoveByName(t, e, "Baral, Chief of Compliance", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	growth := searchMoveByName(t, e, "Giant Growth", state.ZHand)
	counter := searchMoveByName(t, e, "Counterspell", state.ZHand)
	addMana(t, e, 0, "G")
	lbCast(t, e, growth, bear)
	addManaQuiet(t, e, 0, "UU")
	lbCast(t, e, counter, growth)
	if len(e.G.Stack) != 2 {
		t.Fatalf("stack holds %d objects, want Giant Growth under Counterspell", len(e.G.Stack))
	}
	from := len(e.L.Events)
	trigAAnswer(t, e, 80)

	if z := e.G.Obj(growth).Zone; z != state.ZGraveyard {
		t.Fatalf("Giant Growth is in %v, want the graveyard (countered)", z)
	}
	if n := trigACount(e, from, events.Countered, nil); n != 1 {
		t.Fatalf("%d Countered markers, want 1", n)
	}
	draws := trigACount(e, from, events.Draw, func(ev events.Event) bool { return ev.Player == 0 })
	discards := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.MoveZone && ev.From == state.ZHand && ev.To == state.ZGraveyard &&
			ev.Obj != counter && events.IsDiscard(ev) {
			discards++
		}
	}
	if draws != 1 || discards != 1 {
		t.Fatalf("Baral looted %d draws and %d discards, want 1 and 1", draws, discards)
	}
}

// TestEDHTriggerModesACounteredIgnoresOpponentsCounter: Baral only watches
// spells and abilities its controller controls. Seat 1's Counterspell on a
// seat 0 spell must not loot seat 0.
func TestEDHTriggerModesACounteredIgnoresOpponentsCounter(t *testing.T) {
	t.Parallel()
	e, _ := mfEngine(t, []string{"Baral, Chief of Compliance", "Giant Growth", "Grizzly Bears"}, []string{"Counterspell"})
	searchMoveByName(t, e, "Baral, Chief of Compliance", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	growth := searchMoveByName(t, e, "Giant Growth", state.ZHand)
	counter := searchMoveByNameSeat(t, e, 1, "Counterspell", state.ZHand)
	addMana(t, e, 0, "G")
	addManaQuiet(t, e, 1, "UU")
	lbCast(t, e, growth, bear)
	if d := e.Pending(); d != nil && d.Player == 0 {
		submitPass(t, e)
	}
	if d := e.Pending(); d == nil || d.Player != 1 {
		t.Fatalf("seat 1 not holding priority: %+v", d)
	}
	lbCast(t, e, counter, growth)
	from := len(e.L.Events)
	trigAAnswer(t, e, 80)
	if z := e.G.Obj(growth).Zone; z != state.ZGraveyard {
		t.Fatalf("Giant Growth is in %v, want the graveyard (countered)", z)
	}
	if n := trigACount(e, from, events.Countered, nil); n != 1 {
		t.Fatalf("%d Countered markers, want 1 (the precondition: a counter happened)", n)
	}
	if n := trigACount(e, from, events.Draw, func(ev events.Event) bool { return ev.Player == 0 }); n != 0 {
		t.Fatalf("seat 0 drew %d cards off the opponent's counterspell, want 0", n)
	}
}

// TestEDHTriggerModesAMilledOnceZellixMakesOneHorror: Zellix, Sanity Flayer
// activates ("{1}, {T}: target player mills three cards") on a library whose
// top three are two creatures and a land; the one mill action makes ONE
// Horror, not one per creature card.
func TestEDHTriggerModesAMilledOnceZellixMakesOneHorror(t *testing.T) {
	t.Parallel()
	e, _ := mfEngine(t, []string{"Zellix, Sanity Flayer"}, []string{"Grizzly Bears", "Grizzly Bears"})
	zellix := searchMoveByName(t, e, "Zellix, Sanity Flayer", state.ZBattlefield)
	e.G.Obj(zellix).SummonSick = false
	var bears []state.ObjID
	var land state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 1) {
		o := e.G.Obj(id)
		switch {
		case o.Face().Name == "Grizzly Bears" && len(bears) < 2:
			bears = append(bears, id)
		case o.Face().Name == "Plains" && land == 0:
			land = id
		}
	}
	if len(bears) != 2 || land == 0 {
		t.Fatalf("library lacks two bears and a land: %v %v", bears, land)
	}
	trigAOrderTop(e, 1, bears[0], bears[1], land)
	addMana(t, e, 0, "G")
	horrors := func() int {
		n := 0
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); o != nil && o.IsToken {
				n++
			}
		}
		return n
	}
	before := horrors()
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == zellix {
			idx = o.Index
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no activation option for Zellix: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	for i := 0; i < 6; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			break
		}
		pick := -1
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player == 1 {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("opponent not offered as the mill target: %+v", d.Options)
		}
		submitChoices(t, e, pick)
	}
	from := len(e.L.Events)
	trigAAnswer(t, e, 80)
	milled := 0
	for _, ev := range e.L.Events[from:] {
		if events.IsMill(ev) {
			milled++
		}
	}
	if milled != 3 {
		t.Fatalf("%d cards milled, want 3 (precondition)", milled)
	}
	if got := horrors() - before; got != 1 {
		t.Fatalf("Zellix made %d Horror tokens off a mill of two creature cards, want exactly 1", got)
	}
}

// trigASagaEngine builds a game with Tom Bombadil (or Narci) and two Birth of
// the Imperium Sagas in play, with two more Sagas at the bottom of seat 0's
// library for Tom Bombadil to find.
func trigASagaEngine(t *testing.T, watcher string) (*Engine, []state.ObjID) {
	t.Helper()
	e, _ := mfEngine(t, []string{watcher, "Birth of the Imperium", "Birth of the Imperium",
		"Birth of the Imperium", "Birth of the Imperium"}, nil)
	searchMoveByName(t, e, watcher, state.ZBattlefield)
	sagas := []state.ObjID{
		searchMoveByName(t, e, "Birth of the Imperium", state.ZBattlefield),
		searchMoveByName(t, e, "Birth of the Imperium", state.ZBattlefield),
	}
	var bottom []state.ObjID
	var rest []state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if e.G.Obj(id).Face().Name == "Birth of the Imperium" {
			bottom = append(bottom, id)
		} else {
			rest = append(rest, id)
		}
	}
	if len(bottom) != 2 {
		t.Fatalf("library holds %d Sagas, want 2", len(bottom))
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: append(rest, bottom...)})
	trigAAnswer(t, e, 200) // chapter I of both Sagas
	return e, sagas
}

func trigASagasOnBattlefield(e *Engine) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == "Birth of the Imperium" {
			n++
		}
	}
	return n
}

// TestEDHTriggerModesAAbilityResolvesBombadilFindsASaga: Tom Bombadil triggers
// when a Saga's FINAL chapter ability resolves (chapters I and II do not), and
// its "triggers only once each turn" holds when two Sagas finish together.
func TestEDHTriggerModesAAbilityResolvesBombadilFindsASaga(t *testing.T) {
	t.Parallel()
	e, sagas := trigASagaEngine(t, "Tom Bombadil")
	if n := trigASagasOnBattlefield(e); n != 2 {
		t.Fatalf("%d Sagas in play after chapter I, want 2 (Bombadil must not fire on chapter I)", n)
	}
	sagaAddLore(e, sagas[0], 1) // chapter II only
	trigAAnswer(t, e, 200)
	if n := trigASagasOnBattlefield(e); n != 2 {
		t.Fatalf("%d Sagas in play after chapter II, want 2", n)
	}
	sagaAddLore(e, sagas[0], 1) // chapter III
	sagaAddLore(e, sagas[1], 2) // chapters II and III the same turn
	trigAAnswer(t, e, 400)
	for _, s := range sagas {
		if z := e.G.Obj(s).Zone; z != state.ZGraveyard {
			t.Fatalf("Saga %d is in %v, want the graveyard (final chapter resolved)", s, z)
		}
	}
	if n := trigASagasOnBattlefield(e); n != 1 {
		t.Fatalf("%d Sagas in play, want exactly 1: two final chapters resolved in one turn and Bombadil triggers once each turn", n)
	}
}

// TestEDHTriggerModesAAbilityResolvesNarciDrainsForManaValue: Narci's final
// chapter trigger has each opponent lose, and Narci's controller gain, the
// resolving Saga's mana value (Birth of the Imperium: 5).
func TestEDHTriggerModesAAbilityResolvesNarciDrainsForManaValue(t *testing.T) {
	t.Parallel()
	e, sagas := trigASagaEngine(t, "Narci, Fable Singer")
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	sagaAddLore(e, sagas[0], 2) // chapters II and III
	trigAAnswer(t, e, 300)
	if z := e.G.Obj(sagas[0]).Zone; z != state.ZGraveyard {
		t.Fatalf("Saga is in %v, want the graveyard (final chapter resolved)", z)
	}
	if got := e.G.Players[1].Life; got != life1-5 {
		t.Fatalf("opponent life %d -> %d, want a loss of 5", life1, got)
	}
	if got := e.G.Players[0].Life; got != life0+5 {
		t.Fatalf("controller life %d -> %d, want a gain of 5", life0, got)
	}
}
