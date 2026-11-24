package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 614.12 and 707.2: Pirated Copy entering as a copy of Flycatcher Giraffid
// enters with the copied "enters with your choice of a vigilance counter or a
// reach counter" replacement, so its controller makes that choice too.
func TestCloneEntersWithCopiedCounterKindChoice(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := searchEngine(t, reg, "Flycatcher Giraffid", "Pirated Copy")
	giraffid := searchMoveByName(t, e, "Flycatcher Giraffid", state.ZBattlefield)
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		// The template's own entry asked its counter choice; answer it.
		submitChoices(t, e, 0)
	}
	pirate := searchMoveByName(t, e, "Pirated Copy", state.ZHand)
	if po := e.G.Obj(pirate); po == nil || po.Zone != state.ZHand {
		t.Fatalf("Pirated Copy fixture not in hand: %+v", po)
	}
	addMana(t, e, 0, "CCCCU")
	castCardByName(t, e, pirate)
	d := passUntilAsk(t, e)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("no copy election (%v)", d != nil)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "clone" && o.Obj == giraffid {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the Giraffid is not offered as a template")
	}
	submitChoices(t, e, idx)
	d = passUntilAsk(t, e)
	if d == nil {
		t.Fatal("no decision after the copy election")
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "counter" || d.Options[0].Label != "Vigilance" {
		t.Fatalf("the copy was not asked the copied counter-kind choice (kind %v)", d.Kind)
	}
	submitChoices(t, e, 1)
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(pirate)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("copy not on the battlefield: %+v", o)
	}
	if got := o.Counter("Reach"); got != 1 {
		t.Fatalf("copy's Reach counters = %d, want 1 (counters %v)", got, o.Counters)
	}
	if got := o.Counter("Vigilance"); got != 0 {
		t.Fatalf("copy's Vigilance counters = %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}
