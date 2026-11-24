package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestFarmGigglingSkitterspike(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	spikeCard := mustCorpusCard(t, reg, "Giggling Skitterspike")
	trig := corpusTriggerMode(t, spikeCard, "BecomesTarget")
	if got := trig.Params["ValidSource"]; got != "Spell" {
		t.Fatalf("corpus fixture drift: Skitterspike's ValidSource$ = %q, want Spell", got)
	}

	// Leg 1: an ACTIVATED ability targeting Skitterspike must not trigger it.
	e, ids := becomesTargetOnceBoard(t, reg,
		[]string{"Giggling Skitterspike", "Prodigal Pyromancer"}, nil)
	spike, pyro := ids["Giggling Skitterspike"], ids["Prodigal Pyromancer"]
	e.G.Active, e.G.Priority = 0, 0
	e.askPriority(0)
	opt := abilityOption(t, e, pyro, 0)
	submitChoices(t, e, opt.Index)
	submitChoices(t, e, targetOptionIdx(t, e, spike))
	passUntilStackEmpty(t, e, 40)
	if n := skitterspikeTriggerPushes(e, spike); n != 0 {
		t.Fatalf("Prodigal Pyromancer's activated ability pushed Skitterspike's becomes-target trigger %d time(s), want 0 (CR 112.1, 113.3b, 115.1c: an ability is not a spell)", n)
	}

	// Leg 2 (positive control): a SPELL targeting Skitterspike does trigger it.
	e2, ids2 := targetingBoard(t, reg,
		[]string{"Giggling Skitterspike"},
		[]string{"Lightning Strike"})
	spike2, strike := ids2["Giggling Skitterspike"], ids2["Lightning Strike"]
	e2.G.Players[1].Pool[state.MR] = 3
	castStrikeAt(t, e2, strike, spike2, 1)
	passUntilStackEmpty(t, e2, 40)
	pushes := 0
	for _, ev := range e2.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == spike2 {
			pushes++
		}
	}
	if pushes == 0 {
		t.Fatal("Lightning Strike targeting Skitterspike pushed no becomes-target trigger, want at least one (CR 112.1: a card on the stack is a spell)")
	}
}
