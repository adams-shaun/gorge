package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Gorge-side pins for the two Earthbend behaviours the kernel lockstep's
// corpus cannot isolate: a bounced and replayed land still returns when it
// later dies (gorge keys the registration by object id, not incarnation), and
// two earthbends on one land queue two returns where the second does nothing
// because the card is already back on the battlefield.
//
// Both drive gorge's own api:Earthbend effect (effects.Resolve) on a real
// corpus carrier, then the MoveZone event a Destroy or bounce emits, so the
// return promise and its matching are gorge's, not a restatement.

func ebgNew(t *testing.T, seed uint64) (*GorgeSide, state.ObjID, *cards.SA) {
	t.Helper()
	reg := synthReg(t)
	forest, ok := reg.Lookup("Forest")
	if !ok {
		t.Fatal("Forest not in the corpus")
	}
	lesson, ok := reg.Lookup("Earthbending Lesson")
	if !ok {
		t.Fatal("Earthbending Lesson not in the corpus")
	}
	var deck [40]*cards.Card
	for i := range deck {
		deck[i] = forest
	}
	deck[0] = lesson
	decks := [2][]*cards.Card{deck[:], deck[:]}
	m, err := NewMatchup(reg, [2]string{"earthbend", "opponent"}, decks, 0)
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewGorgeSide(m, seed, Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	forestID := ebgFind(t, g, "Forest", state.ZLibrary)
	lessonID := ebgFind(t, g, "Earthbending Lesson", state.ZLibrary)
	g.E.Emit(events.Event{Kind: events.MoveZone, Obj: forestID, From: state.ZLibrary, To: state.ZBattlefield})
	ob := g.E.G.Obj(lessonID)
	if ob == nil || ob.Face() == nil {
		t.Fatal("Earthbending Lesson has no face")
	}
	for i := range ob.Face().Abilities {
		if ob.Face().Abilities[i].API == "Earthbend" {
			return g, forestID, ob.Face().Abilities[i]
		}
	}
	t.Fatal("Earthbending Lesson has no api:Earthbend ability")
	return nil, 0, nil
}

func ebgFind(t *testing.T, g *GorgeSide, name string, z state.Zone) state.ObjID {
	t.Helper()
	for _, id := range g.E.G.Zone(z, 0) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%s not in seat 0 %v", name, z)
	return 0
}

func ebgEarthbend(t *testing.T, g *GorgeSide, forest state.ObjID, sa *cards.SA) {
	t.Helper()
	src := ebgFind(t, g, "Earthbending Lesson", state.ZLibrary)
	effects.Resolve(g.E, &effects.Ctx{
		Source:     src,
		Controller: 0,
		Targets:    []state.Target{{Obj: forest}},
	}, sa)
}

func ebgDelayed(g *GorgeSide) int {
	n := 0
	for _, d := range g.E.G.Delayed {
		if d.Execute == "__kwEarthbendReturn" {
			n++
		}
	}
	return n
}

// ebgSettle answers priority passes (and any earthbend target ask) until stop
// holds with an empty stack and trigger queue.
func ebgSettle(t *testing.T, g *GorgeSide, stop func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if stop() && len(g.E.G.Stack) == 0 && len(g.E.PendingTriggers()) == 0 {
			return
		}
		c := g.Step()
		switch c.Kind {
		case DNone:
			t.Fatalf("game ended while settling (delayed %d)", ebgDelayed(g))
		case DPriority:
			g.Apply(Pack(ActPass, RefNone, RefNone, 0))
		case DChoose, DTriggerOrder:
			g.Apply(c.Options[0])
		default:
			t.Fatalf("unexpected decision %v while settling: %v", c.Kind, c.Options)
		}
	}
	t.Fatalf("never settled: delayed %d stack %d triggers %d", ebgDelayed(g), len(g.E.G.Stack), len(g.E.PendingTriggers()))
}

func ebgDie(t *testing.T, g *GorgeSide, forest state.ObjID, from, to state.Zone) {
	t.Helper()
	g.E.Emit(events.Event{Kind: events.MoveZone, Obj: forest, From: from, To: to})
}

// TestEarthbendGorgeBounceReplayDeath: an earthbent land bounced to hand,
// replayed, then destroyed still returns. gorge matches the registration by
// object id and does not track incarnation, so the replay must not cancel the
// promise.
func TestEarthbendGorgeBounceReplayDeath(t *testing.T) {
	t.Parallel()

	g, forest, sa := ebgNew(t, 77)
	ebgEarthbend(t, g, forest, sa)
	if n := ebgDelayed(g); n != 1 {
		t.Fatalf("after one earthbend: %d return records, want 1", n)
	}
	ebgDie(t, g, forest, state.ZBattlefield, state.ZHand)
	ebgSettle(t, g, func() bool { return ebgDelayed(g) == 1 })
	if o := g.E.G.Obj(forest); o == nil || o.Zone != state.ZHand {
		t.Fatalf("after the bounce the land is %+v, want it in hand with the promise still live", o)
	}
	ebgDie(t, g, forest, state.ZHand, state.ZBattlefield)
	ebgSettle(t, g, func() bool { return ebgDelayed(g) == 1 })
	if o := g.E.G.Obj(forest); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("after the replay the land is %+v, want it back on the battlefield with the promise still live", o)
	}
	ebgDie(t, g, forest, state.ZBattlefield, state.ZGraveyard)
	ebgSettle(t, g, func() bool { return ebgDelayed(g) == 0 })
	o := g.E.G.Obj(forest)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("bounced/replayed land did not return from the graveyard: %+v", o)
	}
	if !o.Tapped {
		t.Fatal("returned land entered untapped; want tapped")
	}
}

// TestEarthbendGorgeTwoRegistrations: two earthbends on one land queue two
// returns. The first departure returns it; the second return finds the card
// on the battlefield and does nothing. A later departure returns nothing.
func TestEarthbendGorgeTwoRegistrations(t *testing.T) {
	t.Parallel()

	g, forest, sa := ebgNew(t, 78)
	ebgEarthbend(t, g, forest, sa)
	ebgEarthbend(t, g, forest, sa)
	if n := ebgDelayed(g); n != 2 {
		t.Fatalf("after two earthbends: %d return records, want 2", n)
	}
	ebgDie(t, g, forest, state.ZBattlefield, state.ZGraveyard)
	ebgSettle(t, g, func() bool { return ebgDelayed(g) == 0 })
	o := g.E.G.Obj(forest)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("land did not return after two registrations: %+v", o)
	}
	if !o.Tapped {
		t.Fatal("returned land entered untapped; want tapped")
	}
	// The second return resolved after the first had already put the card
	// back, so it did nothing. A second death has no registration left.
	ebgDie(t, g, forest, state.ZBattlefield, state.ZGraveyard)
	ebgSettle(t, g, func() bool { return true })
	if o := g.E.G.Obj(forest); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("a second death returned without a registration: %+v", o)
	}
}
