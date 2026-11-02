package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two Leylines of the Void compete for what their controller's opponent loses. A
// casualty of the same state-based check that does not compete leaves at once, and
// the passes that run on under the first order ask unattach its Equipment, while an
// opposing Aura on it stays attached until its own contested move is answered.
func TestAttachmentsUnderAParkedOrderAskLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-dsk")
	leylines := []cardset.Entry{{Name: "Leyline of the Void", Count: 8}, {Name: "Conductive Machete", Count: 6},
		{Name: "Appendage Amalgam", Count: 10}, {Name: "Swamp", Count: 16}}
	bears := []cardset.Entry{{Name: "Frantic Strength", Count: 10}, {Name: "Shrewd Storyteller", Count: 14},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 8}}
	build := func(es []cardset.Entry) (out []*cards.Card) {
		for _, e := range es {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				out = append(out, cd)
			}
		}
		return out
	}
	tbl, _, _, err := set.Matchup(leylines, bears)
	if err != nil {
		t.Fatal(err)
	}
	unattached, stays := 0, 0
	for seed := uint64(1); seed <= 64; seed++ {
		m, err := NewMatchup(reg, [2]string{"leylines", "bears"}, [2][]*cards.Card{build(leylines), build(bears)}, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		at := 0
		check := func(g *GorgeSide, _ Side) *Divergence {
			if d := g.E.Pending(); d != nil && d.Kind == decision.KReplacement {
				for _, e := range g.E.L.Events[at:] {
					if e.Kind == events.Unattached {
						unattached++
					}
				}
				for i := range g.E.G.Objs {
					o := &g.E.G.Objs[i]
					if b := g.E.G.Obj(o.AttachedTo); o.Zone == state.ZBattlefield && b != nil && b.Zone != state.ZBattlefield {
						stays++
					}
				}
			}
			at = len(g.E.L.Events)
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
	}
	t.Logf("%d unattached and %d attached to a departed host under an order ask", unattached, stays)
	if unattached == 0 || stays == 0 {
		t.Fatalf("the fixture never reached it: %d unattached, %d stayed", unattached, stays)
	}
}
