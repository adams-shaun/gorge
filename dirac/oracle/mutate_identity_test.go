package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMutateTopMergeHandsTheSpellInstanceToTheUnderCard pins identity.sync
// across a mutate top merge: the object gorge mints for the demoted card takes
// the resolving spell's instance, where the kernel parks that card, and binds
// no instance of its own. It holds both when the pile is still on the
// battlefield at the next sync and when it has already left.
func TestMutateTopMergeHandsTheSpellInstanceToTheUnderCard(t *testing.T) {
	for _, leaves := range []bool{false, true} {
		name := "pile stays"
		if leaves {
			name = "pile left before the sync"
		}
		t.Run(name, func(t *testing.T) {
			g := state.NewGame([]string{"a", "b"})
			named := func(n string) *cards.Card { return &cards.Card{Faces: []*cards.Face{{Name: n}}} }
			bear := g.AddObject(named("Bear"), 0).ID
			spell := g.AddObject(named("Mutator"), 0).ID
			var log []events.Event
			apply := func(ev events.Event) {
				events.Apply(g, ev)
				log = append(log, ev)
			}
			apply(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZLibrary, To: state.ZBattlefield})
			apply(events.Event{Kind: events.MoveZone, Obj: spell, From: state.ZLibrary, To: state.ZStack})
			var id identity
			id.sync(g, log)
			bearInst, spellInst := id.ref(g, bear), id.ref(g, spell)
			if bearInst == RefNone || spellInst == RefNone || len(id.obj) != 2 {
				t.Fatalf("precondition: instances bear=%d spell=%d of %d", bearInst, spellInst, len(id.obj))
			}

			apply(events.Event{Kind: events.Mutate, Obj: bear, IDs: []state.ObjID{spell}, Text: "top", Amount: 1})
			pile := g.Obj(bear)
			if len(g.Objs) != 3 || len(pile.MergedCards) != 1 || pile.MergedCards[0].Obj == spell {
				t.Fatalf("precondition: the merge minted no under-card: objs=%d pile=%+v", len(g.Objs), pile.MergedCards)
			}
			under := pile.MergedCards[0].Obj
			if leaves {
				apply(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
				if o := g.Obj(under); o.Zone != state.ZGraveyard || len(g.Obj(bear).MergedCards) != 0 {
					t.Fatalf("precondition: the pile did not split: under in %v", o.Zone)
				}
			}
			id.sync(g, log)

			if len(id.obj) != 2 {
				t.Errorf("instances = %d, want 2: the under-card binds none of its own", len(id.obj))
			}
			if got := id.ref(g, under); got != spellInst {
				t.Errorf("under-card instance = %d, want the spell's %d", got, spellInst)
			}
			if int(spellInst) < len(id.obj) && id.obj[spellInst] != under {
				t.Errorf("instance %d names object %d, want the under-card %d", spellInst, id.obj[spellInst], under)
			}
			if got := id.ref(g, spell); got != RefNone {
				t.Errorf("spent spell object instance = %d, want none", got)
			}
			if got := id.ref(g, bear); got != bearInst {
				t.Errorf("survivor instance = %d, want %d", got, bearInst)
			}
		})
	}
}
