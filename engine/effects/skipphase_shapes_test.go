package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffSkipPhaseLowersTheThreeShapes: the param shapes of Avizoa (Step$
// Untap), Fatigue (Step$ Draw), Blinding Angel (Phase$ BeginCombat alone),
// Elfhame Sanctuary (Step$ Draw, Duration$ EndOfTurn), Moment of Silence
// (Duration$ NextThisTurn) and False Peace compile to a grant, not the Note;
// shapes beyond them still do not.
func TestEffSkipPhaseLowersTheThreeShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		params string
		step   state.Step
		scope  state.ObjID
		ok     bool
	}{
		{"Avizoa", "Defined=You|Step=Untap", state.StepUntap, state.SkipScopeNext, true},
		{"Fatigue", "ValidTgts=Player|Step=Draw", state.StepDraw, state.SkipScopeNext, true},
		{"Blinding Angel", "Defined=TriggeredTarget|Phase=BeginCombat", state.StepBeginCombat, state.SkipScopeNext, true},
		{"Elfhame Sanctuary", "Defined=You|Step=Draw|Duration=EndOfTurn", state.StepDraw, state.SkipScopeTurn, true},
		{"Moment of Silence", "ValidTgts=Player|Phase=BeginCombat|Duration=NextThisTurn", state.StepBeginCombat, state.SkipScopeTurn, true},
		{"False Peace", "ValidTgts=Player|Phase=BeginCombat|Start=FromNext|Duration=UntilTheEndOfTargetedNextTurn", state.StepBeginCombat, 0, true},
		{"Step Upkeep", "Defined=You|Step=Upkeep", 0, 0, false},
		{"Untap this turn", "Defined=You|Step=Untap|Duration=EndOfTurn", 0, 0, false},
		{"Phase Main", "Defined=You|Phase=Main1", 0, 0, false},
		{"Offset start", "Defined=You|Phase=BeginCombat|Start=FromNext", 0, 0, false},
	}
	for _, c := range cases {
		params := map[string]string{}
		for _, kv := range strings.Split(c.params, "|") {
			k, v, _ := strings.Cut(kv, "=")
			params[k] = v
		}
		p := compileSkipPhaseParams(&cards.SA{API: "SkipPhase", Params: params})
		if p.ok != c.ok || (c.ok && (p.step != c.step || p.scope != c.scope)) {
			t.Errorf("%s: compiled %+v, want ok=%v step=%v scope=%d", c.name, p, c.ok, c.step, c.scope)
		}
		if !c.ok && p.desc == "" {
			t.Errorf("%s: an unmodelled shape must say which for the Note", c.name)
		}
	}
}
