package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCR701_34aProliferateTwiceIsTwoProliferations: "proliferate twice" is
// two proliferate actions (CR 701.34a, each a fresh "choose any number"),
// not one batch of +2. Contagion Engine's {4},{T} ability poses TWO asks:
// the first takes the carrier (+1), the second takes nothing; and Scheming
// Aspirant's "whenever you proliferate" fires once per proliferation, so the
// life swing is 4.
func TestCR701_34aProliferateTwiceIsTwoProliferations(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Scheming Aspirant", "Contagion Engine", "Grizzly Bears")
	aspirant := searchMoveByName(t, e, "Scheming Aspirant", state.ZBattlefield)
	aspirantWithProliferateTrigger(t, e, aspirant)
	engineID := searchMoveByName(t, e, "Contagion Engine", state.ZBattlefield)
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	carrier := putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 1)
	e.priorityRound()
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life

	addMana(t, e, 0, "CCCCCC")
	opt, ok := findAbilityOption(e, engineID, 0)
	if !ok {
		t.Fatalf("CR 701.34a Contagion Engine seq %d: proliferate ability not offered", len(e.L.Events))
	}
	submitChoices(t, e, opt.Index)
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("CR 701.34a Contagion Engine seq %d: decision = %+v, want the first proliferate ask", len(e.L.Events), d)
	}
	carrierOpt := -1
	for i, o := range d.Options {
		if o.Obj == carrier {
			carrierOpt = i
		}
	}
	if carrierOpt < 0 {
		t.Fatalf("CR 701.34a Contagion Engine seq %d: carrier not offered: %+v", len(e.L.Events), d.Options)
	}
	// First proliferation: the carrier.
	d = answerProliferate(t, e, d, carrierOpt)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		kind, resume := "none", ""
		if d != nil {
			kind, resume = string(d.Kind), d.ResumeKind
		}
		t.Fatalf("CR 701.34a Contagion Engine seq %d: after the first answer the pending decision is %s/%q (carrier P1P1 %d); want the SECOND proliferate ask",
			len(e.L.Events), kind, resume, e.G.Obj(carrier).Counter("P1P1"))
	}
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 2 {
		t.Fatalf("CR 701.34a Contagion Engine seq %d: carrier P1P1 = %d after the first proliferation, want 2 (+1)", len(e.L.Events), got)
	}
	// Second proliferation: choose nothing. Both Aspirant triggers then wait
	// for the resolution to finish (CR 603.3) and may pose an order ask.
	answerProliferate(t, e, d)
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 2 {
		t.Errorf("CR 701.34a Contagion Engine seq %d: carrier P1P1 = %d, want 2 (the second proliferation chose nothing)", len(e.L.Events), got)
	}
	if n := countProliferateMarkers(e); n != 2 {
		t.Errorf("CR 701.34a Contagion Engine seq %d: %d Proliferate markers, want 2 (one per proliferation)", len(e.L.Events), n)
	}
	if got := e.G.Players[1].Life; got != life1-4 {
		t.Errorf("CR 701.34a Scheming Aspirant seq %d: seat 1 life = %d, want %d (the trigger fires once per proliferation)", len(e.L.Events), got, life1-4)
	}
	if got := e.G.Players[0].Life; got != life0+4 {
		t.Errorf("CR 701.34a Scheming Aspirant seq %d: seat 0 life = %d, want %d", len(e.L.Events), got, life0+4)
	}
	replayCheck(t, e, cfg)
}
