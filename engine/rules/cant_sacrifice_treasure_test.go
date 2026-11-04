package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Quark, Humble Proprietor: "You can't sacrifice Treasure tokens." A Treasure's
// mana ability has a sacrifice in its cost, so with Quark out the Treasure
// can't be activated at all (CR 602.2b, 601.2h: a cost that can't be paid
// can't be begun). Another permanent's Treasure, and Quark's controller's
// other artifacts, are untouched.
func TestQuarkWithholdsOwnTreasureManaAbility(t *testing.T) {
	t.Parallel()
	for _, withQuark := range []bool{false, true} {
		e := handEngineTokens(t, corpusAlternativeCard(t, "Quark, Humble Proprietor"))
		quark := e.G.Zone(state.ZHand, 0)[0]
		if withQuark {
			placeOnBattlefield(t, e, quark)
		}
		e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_a_treasure_sac"})
		e.emit(events.Event{Kind: events.TokenCreate, Player: 1, Text: "c_a_treasure_sac"})
		var mine, theirs state.ObjID
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range e.G.Zone(state.ZBattlefield, p) {
				if o := e.G.Obj(id); o != nil && o.IsToken && faceHasType(o, "Treasure") {
					if p == 0 {
						mine = id
					} else {
						theirs = id
					}
				}
			}
		}
		if mine == 0 || theirs == 0 {
			t.Fatal("precondition: both seats need a Treasure token")
		}
		if got := e.SacrificeBlocked(mine, true); got != withQuark {
			t.Fatalf("quark=%v: own Treasure cost sacrifice blocked = %v", withQuark, got)
		}
		if got := e.SacrificeBlocked(mine, false); got != withQuark {
			t.Fatalf("quark=%v: own Treasure effect sacrifice blocked = %v", withQuark, got)
		}
		if e.SacrificeBlocked(theirs, true) || e.SacrificeBlocked(theirs, false) {
			t.Fatalf("quark=%v: an opponent's Treasure must stay sacrificeable", withQuark)
		}
		if withQuark && e.SacrificeBlocked(quark, false) {
			t.Fatal("Quark himself is no Treasure token")
		}
		e.pending = nil
		e.priorityRound()
		d := e.Pending()
		if d == nil {
			t.Fatal("no priority decision")
		}
		offered := false
		for _, o := range d.Options {
			if o.Kind == "activate" && o.Obj == mine {
				offered = true
			}
		}
		if offered == withQuark {
			t.Fatalf("quark=%v: Treasure mana ability offered = %v", withQuark, offered)
		}
	}
}
