// Thundermane Dragon drives the whole fixture: "You may cast creature spells
// with power 4 or greater from the top of your library. If you cast a
// creature spell this way, it gains haste until end of turn." Its SpellCast
// trigger's Animate grant is registered on the SPELL while it is on the stack
// (the grant's move sweep is anchored to the zone it was made in), and
// CR 400.7a carries a grant to a permanent spell onto the permanent that
// spell becomes. The arms:
//
//   - a library-top cast resolves and the creature has haste until end of
//     turn: the grant carried off the stack, then expired at the turn's
//     cleanup (Probe Colossus);
//   - a cast countered with a real counterspell leaves the graveyard card
//     carrying nothing, so casting it again the same turn cannot inherit the
//     countered grant's haste (Probe Titan): the recast through a graveyard
//     may-play permission gets its own fresh haste from Thundermane's
//     trigger, and that fresh grant must carry just the same. Thundermane's
//     own grant is library-top only, so the recast permission is a synthetic
//     graveyard grant, the Conduit of Worlds shape.
//
// Thundermane, the counterspell and the graveyard may-play grant are the only
// non-probe cards; every object is a deck card moved by logged events, so
// replayCheck validates the flow.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestThundermaneGrantsHasteToTheLibraryCast(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	thunder := mayPlayAfterStackCard(t, "Thundermane Dragon")
	p1 := card(t, "Name:Probe Colossus\nManaCost:R\nTypes:Creature\nPT:4/4\nOracle:x\n")
	p2 := card(t, "Name:Probe Titan\nManaCost:R\nTypes:Creature\nPT:4/4\nOracle:x\n")
	counter := card(t, "Name:Probe Counter\nManaCost:U U\nTypes:Instant\nA:SP$ Counter | ValidTgts$ Spell\nOracle:x\n")
	door := card(t, "Name:Grave Door\nTypes:Enchantment\nS:Mode$ Continuous | Affected$ Card.Creature+YouOwn | AffectedZone$ Graveyard | MayPlay$ True\nOracle:x\n")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{thunder, p1, p2, counter, door}, nil)

	find := func(name string) state.ObjID {
		t.Helper()
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, 0) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
					return id
				}
			}
		}
		t.Fatalf("card %q not found in seat 0's hand or library", name)
		return 0
	}
	tid := find("Thundermane Dragon")
	moveByName(t, e, 0, "Thundermane Dragon", state.ZBattlefield)
	did := find("Grave Door")
	moveByName(t, e, 0, "Grave Door", state.ZBattlefield)
	a1 := find("Probe Colossus")
	if o := e.G.Obj(a1); o.Zone != state.ZLibrary {
		moveByName(t, e, 0, "Probe Colossus", state.ZLibrary)
	}
	a2 := find("Probe Titan")
	if o := e.G.Obj(a2); o.Zone != state.ZLibrary {
		moveByName(t, e, 0, "Probe Titan", state.ZLibrary)
	}
	cid := find("Probe Counter")
	if o := e.G.Obj(cid); o.Zone != state.ZHand {
		moveByName(t, e, 0, "Probe Counter", state.ZHand)
	}
	// Thundermane's MayPlay grant is over the library TOP: put the probes
	// there through a logged LibraryOrder (a plain MoveZone appends to the
	// bottom); Probe Colossus first, Probe Titan right behind it.
	order := []state.ObjID{a1, a2}
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if id != a1 && id != a2 {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order})
	if o := e.G.Obj(tid); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Thundermane not on the battlefield (precondition): %+v", o)
	}
	if o := e.G.Obj(did); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Grave Door not on the battlefield (precondition): %+v", o)
	}
	if o := e.G.Obj(cid); o == nil || o.Zone != state.ZHand {
		t.Fatalf("Probe Counter not in hand (precondition): %+v", o)
	}
	if top := e.G.Zone(state.ZLibrary, 0); len(top) < 2 || top[0] != a1 || top[1] != a2 {
		t.Fatalf("probes not on top of the library (precondition): %v", top)
	}

	// The library-cast arm: the cast resolves (its Static$ True SpellCast
	// trigger resolved off the stack first), and CR 400.7a carries the haste
	// grant onto the permanent the spell became.
	// Mana for the whole turn arrives through logged ManaAdd events so
	// replayCheck can rebuild the pool (a direct pool write is not logged).
	addMana(t, e, 0, "RRRRUU")
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("priority = %+v, want priority for seat 0", d)
	}
	idx := landGrantOptionIndex(d, "cast", a1)
	if idx < 0 {
		t.Fatalf("Thundermane's MayPlay grant did not offer the top-of-library 4/4: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(a1); o.Zone != state.ZBattlefield {
		t.Fatalf("library-cast probe zone = %s, want battlefield", o.Zone)
	}
	if !e.HasKeyword(a1, "Haste") {
		t.Fatal("the library-cast creature never gained haste: the grant registered on the stack spell was swept away when the spell resolved")
	}

	// The countered arm: the second probe casts (it is the library top now),
	// its trigger resolves off the stack, and the grant is live on the spell
	// while it waits (precondition); then a real counter sends the spell to
	// the graveyard without it resolving. The grant must END on that
	// stack-to-graveyard move: the graveyard card carries nothing.
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority after the first cast: %+v", d)
	}
	idx = landGrantOptionIndex(d, "cast", a2)
	if idx < 0 {
		t.Fatalf("the second top-of-library cast was not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	if len(e.pendingTriggers) > 0 {
		e.putTriggersOnStack()
		answerTriggerOrders(t, e)
	}
	if len(e.G.Stack) > 1 {
		e.resolveTop() // the trigger only, leaving the probe spell on the stack
	}
	if o := e.G.Obj(a2); o.Zone != state.ZStack {
		t.Fatalf("second probe spell zone = %s, want stack (precondition)", o.Zone)
	}
	if !e.HasKeyword(a2, "Haste") {
		t.Fatal("the trigger's haste grant is not live on the stack spell (precondition)")
	}

	addMana(t, e, 0, "U")
	cd := castByName(t, e, 0, "Probe Counter")
	if cd == nil {
		t.Fatal("Probe Counter cast option not offered")
	}
	submitChoices(t, e, cd.Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target decision for the counter: %+v", d)
	}
	cidx := -1
	for _, o := range d.Options {
		if o.Obj == a2 {
			cidx = o.Index
		}
	}
	if cidx < 0 {
		t.Fatalf("stack spell %d not offered as the counter's target: %+v", a2, d.Options)
	}
	submitChoices(t, e, cidx)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(a2); o.Zone != state.ZGraveyard {
		t.Fatalf("countered probe zone = %s, want graveyard (precondition)", o.Zone)
	}
	if e.HasKeyword(a2, "Haste") {
		t.Fatal("the haste grant survived the countered spell's stack-to-graveyard move (CR 400.7)")
	}

	// The recast: the countered card is cast again from the graveyard the same
	// turn, through Grave Door's permission. The fresh SpellCast trigger
	// grants haste to the new spell, and CR 400.7a must carry THAT grant onto
	// the permanent the spell becomes.
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority after the counter: %+v", d)
	}
	idx = landGrantOptionIndex(d, "cast", a2)
	if idx < 0 {
		t.Fatalf("the recast of the countered card was not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(a2); o.Zone != state.ZBattlefield {
		t.Fatalf("recast probe zone = %s, want battlefield", o.Zone)
	}
	if !e.HasKeyword(a2, "Haste") {
		t.Fatal("the recast creature never gained haste: the grant registered on the stack spell was swept away when the spell resolved")
	}

	// End of turn: both grants are until end of turn and expire at the turn's
	// cleanup step. Settle the end step, then pass through the turn boundary
	// to the next turn's upkeep and confirm the haste is gone.
	turn, active := e.G.Turn, e.G.Active
	ateotDriveToStep(t, e, turn, active, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 20)
	}
	ateotDriveToStep(t, e, turn+1, state.PlayerID((int(active)+1)%2), state.StepUpkeep)
	for _, id := range []state.ObjID{a1, a2} {
		if e.HasKeyword(id, "Haste") {
			t.Fatalf("the haste grant survived end-of-turn cleanup on %d", id)
		}
	}
	replayCheck(t, e, cfg)
}
