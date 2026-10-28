package neutral_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// TestNeutralViewMatchesNativeAtCombatTrick pins the spell facts the heuristic's
// PumpInCombat gate reads. The fixture is the choose request at the first
// recorded divergence where the native bot passed a Violent Urge in its
// precombat main and the neutral world cast it: the native view carries the
// literal +1/+0 pump, and the rebuild must carry the same facts or the gate
// cannot recognise the card as a timed pump and holds it nowhere.
func TestNeutralViewMatchesNativeAtCombatTrick(t *testing.T) {
	reg := testcorpus.Registry(t)
	raw, err := os.ReadFile("testdata/combat_trick_violent_urge.json")
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	var p xview.Payload
	if err := json.Unmarshal(q.Decision.Extensions["x_gorge_view_v1"], &p); err != nil {
		t.Fatal(err)
	}
	native, _ := agent.Rebuild(p)
	if native.Step != state.StepMain1.String() {
		t.Fatalf("precondition: fixture step = %q, want the precombat main where the native bot passes", native.Step)
	}
	got, err := neutral.NewBuilder(reg).View(&q.Decision.Observation)
	if err != nil {
		t.Fatal(err)
	}
	// The gate is b.Step == StepDeclareBlockers; main1 is outside it, so a
	// card the board recognises as a timed pump must not be cast here.
	if got.Step != native.Step || got.Phase != native.Phase {
		t.Fatalf("rebuilt step/phase = %q/%q, native %q/%q", got.Step, got.Phase, native.Step, native.Phase)
	}

	find := func(v view.View, name string) (view.CardView, bool) {
		for _, pl := range v.Players {
			if pl.ID != v.Viewer {
				continue
			}
			for _, cv := range pl.Hand {
				if cv.Name == name || cv.CardName == name {
					return cv, true
				}
			}
		}
		return view.CardView{}, false
	}
	const name = "Violent Urge"
	nv, ok := find(native, name)
	if !ok {
		t.Fatalf("precondition: native hand has no %s", name)
	}
	if nv.PumpPower != 1 || nv.PumpToughness != 0 || !nv.PumpOwn || !nv.CombatTrick {
		t.Fatalf("precondition: native %s = +%d/+%d own=%v trick=%v, want +1/+0 own trick", name,
			nv.PumpPower, nv.PumpToughness, nv.PumpOwn, nv.CombatTrick)
	}
	gv, ok := find(got, name)
	if !ok {
		t.Fatalf("rebuilt hand has no %s", name)
	}
	// Compare the facts the heuristic reads into botpolicy.Card, not only the
	// wire fields, so a projection that stops feeding the board fails here.
	nb := heuristic.BoardFromView(native).Cards.Get(nv.ID)
	gb := heuristic.BoardFromView(got).Cards.Get(gv.ID)
	if nb.PumpPower != 1 || nb.PumpToughness != 0 || !nb.PumpOwn || !nb.CombatTrick {
		t.Fatalf("precondition: native board %s = +%d/+%d own=%v trick=%v", name,
			nb.PumpPower, nb.PumpToughness, nb.PumpOwn, nb.CombatTrick)
	}
	if gb.PumpPower != nb.PumpPower || gb.PumpToughness != nb.PumpToughness ||
		gb.PumpOwn != nb.PumpOwn || gb.CombatTrick != nb.CombatTrick ||
		gb.Counter != nb.Counter || gb.Draw != nb.Draw || gb.Token != nb.Token ||
		gb.InstantSpeed != nb.InstantSpeed || gb.Castable != nb.Castable {
		t.Fatalf("rebuilt board %s = +%d/+%d own=%v trick=%v api c/d/t=%v/%v/%v instant=%v castable=%v, native +%d/+%d own=%v trick=%v api c/d/t=%v/%v/%v instant=%v castable=%v",
			name,
			gb.PumpPower, gb.PumpToughness, gb.PumpOwn, gb.CombatTrick, gb.Counter, gb.Draw, gb.Token, gb.InstantSpeed, gb.Castable,
			nb.PumpPower, nb.PumpToughness, nb.PumpOwn, nb.CombatTrick, nb.Counter, nb.Draw, nb.Token, nb.InstantSpeed, nb.Castable)
	}
	// Non-vacuity: the two views otherwise agree on the card's identity facts.
	if gv.SpellAPI != nv.SpellAPI || gb.PrintedName != nb.PrintedName || !slices.Equal(gv.Keywords, nv.Keywords) {
		t.Fatalf("rebuilt view %s = api %q name %q keywords %v, native api %q name %q keywords %v",
			name, gv.SpellAPI, gb.PrintedName, gv.Keywords, nv.SpellAPI, nb.PrintedName, nv.Keywords)
	}
}
