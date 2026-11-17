package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 702.46a: Burr Grafter (Soulshift 3) dies and its controller may return a
// Spirit card with mana value 3 or less from their graveyard to hand.
func TestSoulshiftReturnsASpirit(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Burr Grafter", "Elder Pine of Jukai")

	// Precondition: a Spirit card with mana value 3 (Elder Pine of Jukai,
	// 2G) is in seat 0's graveyard, where Soulshift's target search reads.
	// Burr Grafter itself is mana value 4 (3G), so it is not a legal
	// Soulshift 3 target; the Elder Pine is the only one.
	var graveyardSpirit state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Elder Pine of Jukai" {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZGraveyard})
				e.pending = nil
				graveyardSpirit = id
				break
			}
		}
		if graveyardSpirit != 0 {
			break
		}
	}
	if o := e.G.Obj(graveyardSpirit); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("graveyard Spirit precondition: %+v", o)
	}

	carrier := putNamedOnBattlefield(t, e, "Burr Grafter")
	e.emit(events.Event{Kind: events.Damage, Obj: carrier, Amount: 99})
	e.checkStateBased()
	e.priorityRound()
	if o := e.G.Obj(carrier); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Burr Grafter = %+v, want it in the graveyard", o)
	}

	sawOptional, sawTarget := false, false
	for range 8 {
		d := passUntilNonPriority(t, e, 60)
		if d == nil {
			break
		}
		switch d.Kind {
		case decision.KTriggerOptional:
			sawOptional = true
			yes := -1
			for _, o := range d.Options {
				if o.Kind == "yes" {
					yes = o.Index
				}
			}
			if yes < 0 {
				t.Fatalf("optional trigger offers no yes answer: %+v", d.Options)
			}
			submitChoices(t, e, yes)
		case decision.KTarget:
			idx := -1
			for _, o := range d.Options {
				if o.Obj == graveyardSpirit {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("Soulshift target ask does not offer the graveyard Spirit: %+v", d.Options)
			}
			sawTarget = true
			submitChoices(t, e, idx)
		default:
			submitChoices(t, e, 0)
		}
	}
	if !sawOptional {
		t.Fatal("Soulshift offered no optional trigger")
	}
	if !sawTarget {
		t.Fatal("Soulshift offered no Spirit target")
	}
	if o := e.G.Obj(graveyardSpirit); o == nil || o.Zone != state.ZHand {
		t.Fatalf("graveyard Spirit = %+v, want it returned to hand", o)
	}
	replayCheck(t, e, cfg)
}

// The Soulshift carriers the corpus calls unsupported by this keyword alone.
func TestSoulshiftCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Body of Jukai", "Burr Grafter", "Crawling Filth"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
