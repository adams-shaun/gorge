package shadow_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestStageStackPriorityAfterResolution finds the priority round a view cannot
// settle: the active seat passed over the other seat's top entry after a
// resolution, so the holder has one pass, but the same view also occurs right
// after that entry's controller activated it with no pass. Staging must refuse
// it as priority instead of guessing.
func TestStageStackPriorityAfterResolution(t *testing.T) {
	if testing.Short() {
		// A search over whole lockstep games (about 9 s of the package's 38). The
		// pull request fast job passes -short; the merge batch runs it in full.
		t.Skip("whole-game lockstep search; the merge batch runs it without -short")
	}
	reg := testutil.CorpusRegistry(t)
	env := vanillaEnv()
	matches := 0
	for seed := uint64(1); seed <= 400 && matches < 10; seed++ {
		if msg := stagePrioGame(reg, env, seed, &matches); msg != "" {
			t.Fatalf("seed %d: %s", seed, msg)
		}
	}
	t.Logf("%d matched positions, all refused as %q", matches, dirac.ReasonPriority)
	if matches < 3 {
		t.Fatalf("found only %d matched positions", matches)
	}
}

// stagePrioGame plays one lockstep game and stages every matched priority
// position until matches reaches 10. It returns a failure text or "".
func stagePrioGame(reg *cards.Registry, env *stageEnv, seed uint64, matches *int) string {
	r := rand.New(rand.NewPCG(seed, 0x5748))
	decks := [2][]kop.CardID{env.deck(r), env.deck(r)}
	m, err := oracle.KernelMatchup(reg, env.tbl, [2]string{"a", "b"}, decks, nil, 0)
	if err != nil {
		return err.Error()
	}
	k, err := env.kernel(m, seed)
	if err != nil {
		return err.Error()
	}
	defer oracle.CloseSide(k)
	var prev *decision.Decision
	var prevView view.View
	var prevActive state.PlayerID
	fail := ""
	check := func(ref *oracle.GorgeSide, _ oracle.Side) *oracle.Divergence {
		d := ref.Pending()
		if d == nil || d.Kind != "priority" {
			prev = nil
			return nil
		}
		g := ref.E.G
		v := view.ProjectFor(g, ref.E, d.Player, view.Seat, d)
		if prev != nil && prev.Player == prevActive && len(v.Stack) != 0 && sameStack(v.Stack, prevView.Stack) {
			top := v.Stack[len(v.Stack)-1]
			if top.Controller == d.Player && d.Player != v.Active {
				*matches++
				hidden := shadow.Hidden{}
				for p := state.PlayerID(0); p < 2; p++ {
					for _, id := range g.Zone(state.ZHand, p) {
						hidden.Hand[p] = append(hidden.Hand[p], card(ref, g, id))
					}
					for _, id := range g.Zone(state.ZLibrary, p) {
						hidden.Library[p] = append(hidden.Library[p], card(ref, g, id))
					}
				}
				in := &shadow.Input{
					View: v, Hidden: hidden, Tbl: env.tbl, Cfg: stageCfg(), Seed: seed,
					ObjOf: func(id int) (dirac.Obj, bool) { return dirac.Obj(ref.InstRef(state.ObjID(id))), true },
				}
				staged, err := shadow.StageFromView(in)
				var u dirac.ErrUnstageable
				if !errors.As(err, &u) || u.Reason != dirac.ReasonPriority || staged != nil {
					fail = fmt.Sprintf("match %d: staged %v, error %v, want %q", *matches, staged != nil, err, dirac.ReasonPriority)
					return errDone
				}
				if *matches >= 10 {
					return errDone
				}
			}
		}
		prev, prevView, prevActive = d, v, v.Active
		return nil
	}
	res := oracle.Lockstep(m, seed, k, oracle.RandomPolicy(seed), oracle.RunConfig{MaxDecisions: 4000, MaxTurns: 60, Check: check})
	if fail != "" {
		return fail
	}
	if res.Div != nil && res.Div != errDone {
		return fmt.Sprintf("the lockstep itself diverged: %s", res.Div)
	}
	return ""
}

func sameStack(a, b []view.StackView) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}
