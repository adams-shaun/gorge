package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOracleSetupKeywordCounterKind pins the setup-side counter-kind fold. A
// title-cased keyword counter restated by a scenario (the spelling Forge and
// XMage use) must be placed under the ENGINE's kind, "Flying", not the
// snapshot comparison key "FLYING": the card filter counters_EQ0_Flying reads
// the exact kind, so before this fold Inventive Wingsmith's intervening-if saw
// no counter and put its trigger on the stack. A scenario spelling in another
// case (flying) must fold to the same kind. The pre-existing +1/+1 and
// non-keyword kinds must keep their present spelling.
func TestOracleSetupKeywordCounterKind(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const bear = "p0:Grizzly Bears"

	setup := func(t *testing.T, counters map[string]int32) (*state.Game, *state.Object) {
		t.Helper()
		sc := oracleScenario{
			Name: "setup-keyword-counter",
			Setup: map[string]oracleSeat{
				"p0": {
					Battlefield: []string{"Grizzly Bears"},
					Counters:    map[string]map[string]int32{"Grizzly Bears": counters},
					Library:     []string{"Island", "Island", "Island"},
				},
				"p1": {Library: []string{"Island", "Island", "Island"}},
			},
		}
		fails, transcript, run := runOracleScenario(reg, sc)
		if len(fails) > 0 {
			t.Fatalf("scenario failed: %v\ntranscript:\n%s", fails, strings.Join(transcript, "\n"))
		}
		id, ok := run.refs[bear]
		if !ok {
			t.Fatalf("setup did not bind %s", bear)
		}
		o := run.e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("%s is not on the battlefield after setup: %+v", bear, o)
		}
		return run.e.G, o
	}
	matches := func(g *state.Game, o *state.Object, spec string) bool {
		// You is the object's controller (seat p0), Source the object itself.
		return effects.MatchesObjectCtx(g, spec, o, effects.SpecContext{You: 0, Source: o.ID})
	}

	for _, spelling := range []string{"Flying", "flying", "FLYING"} {
		t.Run(spelling, func(t *testing.T) {
			g, o := setup(t, map[string]int32{spelling: 1})
			// Precondition: the counter is on the object under the engine's
			// kind, so the filter below has a counter to read.
			if got := o.Counter("Flying"); got != 1 {
				t.Errorf("setup placed %q: Counter(\"Flying\") = %d, want 1 (kinds %v)", spelling, got, o.Counters)
			}
			if matches(g, o, "Creature.counters_EQ0_Flying") {
				t.Errorf("counters_EQ0_Flying matched with one Flying counter on the object")
			}
			if !matches(g, o, "Creature.counters_GE1_Flying") {
				t.Errorf("counters_GE1_Flying did not match with one Flying counter on the object")
			}
		})
	}

	t.Run("p1p1-and-charge-unchanged", func(t *testing.T) {
		_, o := setup(t, map[string]int32{"+1/+1": 2, "CHARGE": 1})
		if got := o.Counter("P1P1"); got != 2 {
			t.Errorf("setup placed +1/+1: Counter(\"P1P1\") = %d, want 2 (kinds %v)", got, o.Counters)
		}
		if got := o.Counter("CHARGE"); got != 1 {
			t.Errorf("setup placed CHARGE: Counter(\"CHARGE\") = %d, want 1 (kinds %v)", got, o.Counters)
		}
		if got := o.Counter("+1/+1"); got != 0 {
			t.Errorf("setup placed +1/+1 under the raw spelling: Counter(\"+1/+1\") = %d, want 0", got)
		}
	})
}
