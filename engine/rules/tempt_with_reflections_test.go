package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTemptWithReflectionsOpponentControlsItsCopy drives the real compiled
// Tempt with Reflections: the caster targets their Memnite and the opponent
// accepts the tempting offer. CR 111.2: the player who creates a token owns
// and controls it, so the opponent's copy is the opponent's Memnite token
// (Controller$ Player.IsRemembered names the iterating opponent), and the
// caster ends with the original plus two copies.
func TestTemptWithReflectionsOpponentControlsItsCopy(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := miscHandsEngine(t, reg,
		[]string{"Tempt with Reflections"}, nil, []string{"Memnite"}, nil)
	addMana(t, e, 0, "UUUU")
	tempt := miscHandObj(t, e, 0, "Tempt with Reflections")
	memnite := battlefieldObjIDByName(e, "Memnite")
	if memnite == 0 {
		t.Fatal("precondition: the caster's Memnite is not on the battlefield")
	}
	submitChoices(t, e, miscCastOption(t, e, tempt))
	accepted := false
	for i := 0; i < 80; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending while resolving Tempt (step %d)", i)
		}
		if d.Kind == decision.KPriority {
			if accepted && countNamedTokens(e, "Memnite", 0)+countNamedTokens(e, "Memnite", 1) >= 3 {
				break
			}
			if accepted && len(e.G.Stack) == 0 {
				break
			}
			passPriority(t, e)
			continue
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "repeat_each_optional" {
			if d.Player != 1 {
				t.Fatalf("Tempt's copy offer went to seat %d, want the opponent seat 1", d.Player)
			}
			yes := -1
			for _, o := range d.Options {
				if o.Kind == "yes" {
					yes = o.Index
				}
			}
			if yes < 0 {
				t.Fatalf("copy offer has no yes option: %+v", d.Options)
			}
			submitChoices(t, e, yes)
			accepted = true
			continue
		}
		if d.Kind == decision.KTarget {
			idx := d.Options[0].Index
			for _, o := range d.Options {
				if o.Obj == memnite {
					idx = o.Index
				}
			}
			submitChoices(t, e, idx)
			continue
		}
		var cs []int
		for j := 0; j < d.Min && j < len(d.Options); j++ {
			cs = append(cs, d.Options[j].Index)
		}
		submitChoices(t, e, cs...)
	}
	if !accepted {
		t.Fatal("seat 1 was never offered Tempt's per-opponent copy")
	}
	if got := countNamedTokens(e, "Memnite", 1); got != 1 {
		t.Errorf("seat 1 controls %d Memnite tokens, want 1 (CR 111.2: the creating opponent controls its copy)", got)
	}
	if got := countNamedTokens(e, "Memnite", 0); got != 2 {
		t.Errorf("seat 0 controls %d Memnite tokens, want 2", got)
	}
	replayCheck(t, e, cfg)
}

// countNamedTokens counts battlefield tokens named name controlled by p.
func countNamedTokens(e *Engine, name string, p state.PlayerID) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o != nil && o.Controller == p && o.IsToken && o.Face() != nil && o.Face().Name == name {
			n++
		}
	}
	return n
}
