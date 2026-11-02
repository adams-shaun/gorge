package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// tokenCopyByStolenIdentity puts name and others on seat 0's battlefield and
// resolves Stolen Identity on name, answering each ask the resolution poses
// with answer (nil: the first option). It returns the engine and the token.
func tokenCopyByStolenIdentity(t *testing.T, name string, others []string, answer func(*decision.Decision) int) (*Engine, state.ObjID) {
	t.Helper()
	e, _ := fdnCensusEngine(t, append([]string{name, "Stolen Identity"}, others...)...)
	for _, o := range others {
		moveByName(t, e, 0, o, state.ZBattlefield)
	}
	src := moveByName(t, e, 0, name, state.ZBattlefield)
	for d := e.Pending(); d != nil && d.Kind != decision.KPriority; d = e.Pending() {
		submitChoices(t, e, 0)
	}
	spell := moveByName(t, e, 0, "Stolen Identity", state.ZHand)
	addMana(t, e, 0, "UUCCCC")
	submitChoices(t, e, plainCastOption(t, e, spell).Index)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		answerTargetAsk(t, e, []state.ObjID{src})
	}
	want := e.G.NextID
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while resolving")
		}
		if d.Kind == decision.KPriority {
			submitPass(t, e)
			continue
		}
		if answer == nil {
			submitChoices(t, e, 0)
		} else {
			submitChoices(t, e, answer(d))
		}
	}
	tok := e.G.Obj(want)
	if tok == nil || !tok.IsToken || tok.Zone != state.ZBattlefield {
		t.Fatalf("%s: the token copy did not enter", name)
	}
	return e, want
}

// CR 707.2, 614.12: Banner of Kinship's as-enters choice is a plain R: line
// with a ChooseType body, and its copy has it. The copy's controller chooses,
// and the copy enters with a counter for each creature of the chosen type.
func TestTokenCopyOfBannerOfKinshipChoosesAndCounts(t *testing.T) {
	bear := func(d *decision.Decision) int {
		for _, o := range d.Options {
			if o.Label == "Bear" {
				return o.Index
			}
		}
		return 0
	}
	e, tok := tokenCopyByStolenIdentity(t, "Banner of Kinship", []string{"Grizzly Bears"}, bear)
	o := e.G.Obj(tok)
	if o.ChosenType != "Bear" {
		t.Errorf("token copy chose %q, want Bear", o.ChosenType)
	}
	if got := o.Counter("FELLOWSHIP"); got != 1 {
		t.Errorf("token copy has %d fellowship counters, want 1", got)
	}
}

// The other body shapes a token copy's own entry line can carry apply as they
// do to the card: a plain line's chain after a choice, a chain after a counter
// placement the entry absorbs, a keyword line's chain, and a keyword line
// whose body is not itself an election.
func TestTokenCopyAppliesItsOwnEntryBodies(t *testing.T) {
	for _, tc := range []struct {
		name  string
		check func(t *testing.T, e *Engine, o *state.Object)
	}{
		{"Slumbering Trudge", func(t *testing.T, e *Engine, o *state.Object) {
			if !o.Tapped {
				t.Error("the chained tap after the counter body did not apply")
			}
		}},
		{"Riptide Replicator", func(t *testing.T, e *Engine, o *state.Object) {
			if o.ChosenColor == "" || o.ChosenType == "" {
				t.Errorf("chosen colour %q and type %q: the chained choice did not apply", o.ChosenColor, o.ChosenType)
			}
		}},
		{"Wingfold Pteron", func(t *testing.T, e *Engine, o *state.Object) {
			if len(o.Counters) == 0 {
				t.Error("the as-enters counter choice did not apply")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, tok := tokenCopyByStolenIdentity(t, tc.name, nil, nil)
			tc.check(t, e, e.G.Obj(tok))
		})
	}
}
