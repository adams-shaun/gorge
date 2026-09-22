package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestSkirkProspectorOffersSacrificeChoiceWithExtraGoblin uses real corpus
// cards. A second Goblin must widen activation into a legal choice.
func TestSkirkProspectorOffersSacrificeChoiceWithExtraGoblin(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _, ids := realCardEngine(t, reg, 74, "Skirk Prospector", "Goblin Guide")
	prospector, guide := ids[0], ids[1]
	submitChoices(t, e, activateOption(t, e, prospector))
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 || len(d.Options) != 2 {
		t.Fatalf("Prospector decision = %+v, want one-of-two Goblin sacrifice choice", d)
	}
	for _, o := range d.Options {
		if o.Obj != prospector && o.Obj != guide {
			t.Fatalf("non-Goblin sacrifice option: %+v", o)
		}
	}
	submitChoices(t, e, d.Options[0].Index)
	if e.G.Obj(d.Options[0].Obj).Zone != state.ZGraveyard {
		t.Fatalf("chosen Goblin was not sacrificed")
	}
	if e.G.Players[0].Pool[state.MR] != 1 {
		t.Fatalf("Prospector pool = %+v, want one red", e.G.Players[0].Pool)
	}
}

// TestSkirkProspectorBotChoosesLeastValuableGoblin proves the new activation
// ask is answered by the real bot policy with a legal choice.
func TestSkirkProspectorBotChoosesLeastValuableGoblin(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _, ids := realCardEngine(t, reg, 75, "Skirk Prospector", "Goblin Guide")
	prospector := ids[0]
	submitChoices(t, e, activateOption(t, e, prospector))
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("Prospector decision = %+v, want two choices", d)
	}
	intent := newTestBot(1).answer(e, d)
	if err := e.Submit(intent); err != nil {
		t.Fatalf("bot sacrifice answer rejected: %v", err)
	}
	if e.G.Obj(prospector).Zone != state.ZGraveyard {
		t.Fatalf("bot sacrificed %v, want least-valuable Prospector", e.G.Obj(prospector).Zone)
	}
}
