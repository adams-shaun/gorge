package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 702.43a, first half: Arcbound Worker (0/0, Modular 1) enters with a +1/+1
// counter, so it is a 1/1 and survives state-based actions.
func TestModularEntersWithCounters(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Arcbound Worker", "Arcbound Bruiser")
	worker := putNamedOnBattlefield(t, e, "Arcbound Worker")
	e.checkStateBased()
	o := e.G.Obj(worker)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Arcbound Worker = %+v, want it on the battlefield", o)
	}
	if got := o.Counter("P1P1"); got != 1 {
		t.Fatalf("Arcbound Worker P1P1 = %d, want 1 (Modular 1)", got)
	}
	bruiser := putNamedOnBattlefield(t, e, "Arcbound Bruiser")
	if got := e.G.Obj(bruiser).Counter("P1P1"); got != 3 {
		t.Fatalf("Arcbound Bruiser P1P1 = %d, want 3 (Modular 3)", got)
	}
	replayCheck(t, e, cfg)
}

// CR 702.43a, second half: when a Modular permanent dies, its controller may put
// a +1/+1 counter on target artifact creature for each +1/+1 counter it had.
func TestModularDiesMayMoveItsCounters(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, answer := range []int{0, 1} {
		t.Run(map[int]string{0: "yes", 1: "no"}[answer], func(t *testing.T) {
			e, cfg := proliferateEngine(t, reg, "Arcbound Ravager", "Arcbound Worker")
			ravager := putNamedOnBattlefield(t, e, "Arcbound Ravager")
			worker := putNamedOnBattlefield(t, e, "Arcbound Worker")
			putCountersOn(t, e, 0, "Arcbound Ravager", "P1P1", 2)
			if got := e.G.Obj(ravager).Counter("P1P1"); got != 3 {
				t.Fatalf("precondition: Arcbound Ravager P1P1 = %d, want 3", got)
			}
			e.emit(events.Event{Kind: events.Damage, Obj: ravager, Amount: 99})
			e.checkStateBased()
			e.priorityRound()
			if o := e.G.Obj(ravager); o == nil || o.Zone != state.ZGraveyard {
				t.Fatalf("Arcbound Ravager = %+v, want graveyard", o)
			}
			for range 8 {
				d := passUntilNonPriority(t, e, 60)
				if d == nil {
					break
				}
				switch d.Kind {
				case decision.KTriggerOptional:
					submitChoices(t, e, answer)
				default:
					submitChoices(t, e, 0)
				}
			}
			passUntilStackEmpty(t, e, 60)
			want := int32(1)
			if answer == 0 {
				want = 4
			}
			if got := e.G.Obj(worker).Counter("P1P1"); got != want {
				t.Fatalf("Arcbound Worker P1P1 = %d, want %d after answer %d", got, want, answer)
			}
			replayCheck(t, e, cfg)
		})
	}
}
