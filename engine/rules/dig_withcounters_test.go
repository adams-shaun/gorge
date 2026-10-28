package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Undercity's Throne of the Dead Three Dig, as inline fixture text.
const digThroneSrc = "Name:ThroneMe\nManaCost:G\nTypes:Sorcery\n" +
	"A:SP$ Dig | DigNum$ 10 | Reveal$ True | ChangeValid$ Creature | DestinationZone$ Battlefield | WithCounters$ P1P1 | WithCountersAmount$ 3 | LibraryPosition2$ 0 | SkipReorder$ True\nOracle:x\n"

// Arbiter of the Ideal's reveal Dig: WithCounters$ MANIFESTATION, no amount.
const digArbiterSrc = "Name:ArbiterMe\nManaCost:G\nTypes:Sorcery\n" +
	"A:SP$ Dig | DigNum$ 1 | Reveal$ True | Optional$ True | ChangeValid$ Artifact,Creature,Land | DestinationZone$ Battlefield | LibraryPosition2$ 0 | WithCounters$ MANIFESTATION\nOracle:x\n"

// Elspeth Resplendent's [-3] Dig: WithCounters$ Shield.
const digShieldSrc = "Name:ShieldMe\nManaCost:G\nTypes:Sorcery\n" +
	"A:SP$ Dig | DigNum$ 7 | ChangeNum$ 1 | Optional$ True | ChangeValid$ Permanent.cmcLE3 | DestinationZone$ Battlefield | WithCounters$ Shield | RestRandomOrder$ True\nOracle:x\n"

// digPowerElk gains its controller life equal to its power as it enters, so
// the life total shows whether the +1/+1 counters were on it at entry.
const digPowerElk = "Name:PowerElk\nTypes:Creature\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigGain\n" +
	"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ X\nSVar:X:Count$CardPower\nOracle:x\n"

// digTopOf puts the named card on top of seat 0's library and returns its id.
func digTopOf(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	var target state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, oid := range e.G.Zone(z, 0) {
			if e.G.Obj(oid).Face().Name == name {
				target = oid
				if z == state.ZHand {
					e.emit(events.Event{Kind: events.MoveZone, Obj: oid, From: state.ZHand, To: state.ZLibrary, Player: 0})
				}
			}
		}
	}
	if target == 0 {
		t.Fatalf("fixture card %s not found", name)
	}
	order := []state.ObjID{target}
	for _, oid := range e.G.Zone(state.ZLibrary, 0) {
		if oid != target {
			order = append(order, oid)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order, Secret: true})
	return target
}

// digCastAndTake casts the fixture and takes option 0 of any Dig ask.
func digCastAndTake(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for %d: %+v", id, d.Options)
	}
	submitChoices(t, e, idx)
	for i := 0; i < 20 && len(e.G.Stack) == 0; i++ {
		submitChoices(t, e, 0)
	}
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KChoose && d.ResumeKind == "dig" {
			submitChoices(t, e, 0)
			continue
		}
		passUntilStackEmpty(t, e, 1)
	}
}

// TestDigWithCountersEntersWithCounters pins api:Dig's WithCounters$ /
// WithCountersAmount$: the battlefield take ENTERS with the counters (CR
// 122.6), so its own enters trigger reads the boosted power.
func TestDigWithCountersEntersWithCounters(t *testing.T) {
	t.Run("Throne of the Dead Three", func(t *testing.T) {
		e, _, id := newFixtureDeck(t, 71, digThroneSrc, digPowerElk)
		addMana(t, e, 0, "G")
		elk := digTopOf(t, e, "PowerElk")
		life := e.G.Players[0].Life
		if e.G.Obj(elk).Zone != state.ZLibrary {
			t.Fatalf("precondition: PowerElk is in %s, want the library", e.G.Obj(elk).Zone)
		}
		digCastAndTake(t, e, id)
		o := e.G.Obj(elk)
		if o.Zone != state.ZBattlefield {
			t.Fatalf("PowerElk is in %s, want the battlefield", o.Zone)
		}
		if got := o.Counter("P1P1"); got != 3 {
			t.Errorf("P1P1 counters = %d, want 3", got)
		}
		if got := e.G.Players[0].Life; got != life+5 {
			t.Errorf("life %d, want %d: the enters trigger did not see power 5 (2 + three counters)", got, life+5)
		}
	})
	t.Run("Arbiter of the Ideal", func(t *testing.T) {
		e, _, id := newFixtureDeck(t, 72, digArbiterSrc, digCreature)
		addMana(t, e, 0, "G")
		elk := digTopOf(t, e, "Elk")
		digCastAndTake(t, e, id)
		o := e.G.Obj(elk)
		if o.Zone != state.ZBattlefield {
			t.Fatalf("Elk is in %s, want the battlefield", o.Zone)
		}
		if got := o.Counter("MANIFESTATION"); got != 1 {
			t.Errorf("MANIFESTATION counters = %d, want 1 (the default amount)", got)
		}
	})
	t.Run("Elspeth Resplendent", func(t *testing.T) {
		e, _, id := newFixtureDeck(t, 73, digShieldSrc, digCreature)
		addMana(t, e, 0, "G")
		elk := digTopOf(t, e, "Elk")
		digCastAndTake(t, e, id)
		o := e.G.Obj(elk)
		if o.Zone != state.ZBattlefield {
			t.Fatalf("Elk is in %s, want the battlefield", o.Zone)
		}
		if got := o.Counter("SHIELD"); got != 1 {
			t.Errorf("SHIELD counters = %d, want 1 (the script's `WithCounters$ Shield`)", got)
		}
		if got := o.Counter("Shield"); got != 0 {
			t.Errorf("internal Shield marker = %d, want 0: `Shield` is the real shield counter, not the regeneration marker", got)
		}
	})
}
