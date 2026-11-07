package azmcts

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// holdWalkEnv is a one-submit walk over a clairvoyant world of e, rooted at
// e's pending decision d, the searching seat actor, the hold armed by hold
// (nil: off). oppNodes makes the other seats' decisions points.
func holdWalkEnv(t *testing.T, e *rules.Engine, d *decision.Decision, actor state.PlayerID, seed uint64, hold *upkeepHold, oppNodes bool) (*engineEnv, *Stats) {
	t.Helper()
	obs := searchprobe.NewCollector(actor)
	st := new(Stats)
	cfg := &walkConfig{
		kinds: AllKinds(), limit: DefaultOptions().Limit, maxSteps: 1, envSeed: seed, actor: actor,
		root: &Point{}, rootDec: d, stats: st, enumBoard: &boardScratch{hold: hold},
	}
	if oppNodes {
		cfg.oppNodes = true
		cfg.oppForks = make([]*searchprobe.Forker, len(e.G.Players))
		for p := range cfg.oppForks {
			if pl := state.PlayerID(p); pl != actor {
				cfg.oppForks[p] = searchprobe.NewCollector(pl).Forker()
			}
		}
	}
	src, err := newTestClairvoyant(e, obs)
	if err != nil {
		t.Fatal(err)
	}
	w, err := src.World(0)
	if err != nil {
		t.Fatal(err)
	}
	env, err := newEngineEnv(w, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return env, st
}

// TestAzmctsHoldUpkeepWalkPassesTheBot: at the searching seat's in-walk
// decision in its own upkeep window, a walk bot whose answer is the held
// instant cast plays pass (the kernel's holdWalk), not the cast, and the
// decision counts in HeldUpkeepWalk. Off, the same walk's bot answer is
// that cast (candidate 0 of the point it makes).
func TestAzmctsHoldUpkeepWalkPassesTheBot(t *testing.T) {
	e, castAt := upkeepWindow(t)
	d := e.Pending()
	obj := d.Options[castAt].Obj
	seed := uint64(0)
	for s := uint64(1); s < 200 && seed == 0; s++ {
		env, _ := holdWalkEnv(t, e, d, d.Player, s, nil, false)
		pt, err := env.advance()
		if err != nil {
			t.Fatal(err)
		}
		if pt != nil && env.cur.Seq == d.Seq && len(env.cands) > 0 && reflect.DeepEqual(env.cands[0].in.Choices, []int{castAt}) {
			seed = s
		}
	}
	if seed == 0 {
		t.Fatal("precondition: no walk seed whose bot answer is the instant cast")
	}
	env, st := holdWalkEnv(t, e, d, d.Player, seed, &upkeepHold{seat: d.Player}, false)
	if _, err := env.advance(); err != nil {
		t.Fatal(err)
	}
	if o := env.e.G.Obj(obj); o == nil || o.Zone != state.ZHand {
		t.Fatalf("on: the walk played the held instant (object %d left the hand)", obj)
	}
	if st.HeldUpkeepWalk == 0 || st.HeldUpkeepOpp != 0 {
		t.Fatalf("on: HeldUpkeepWalk %d, HeldUpkeepOpp %d; want >0, 0", st.HeldUpkeepWalk, st.HeldUpkeepOpp)
	}
}

// TestAzmctsHoldUpkeepWalkOpponentNeedsMirror: under OpponentNodes the
// opponent's own-upkeep window is held only with the mirror (upkeepHold.opp,
// Search's OpponentNodes && OpponentMirror): unmirrored, its point still
// offers the instant cast and nothing counts; mirrored, no candidate casts it
// and the decision counts in HeldUpkeepOpp.
func TestAzmctsHoldUpkeepWalkOpponentNeedsMirror(t *testing.T) {
	e, castAt := upkeepWindow(t)
	d := e.Pending()
	actor := 1 - d.Player
	for _, mirror := range []bool{false, true} {
		env, st := holdWalkEnv(t, e, d, actor, 7, &upkeepHold{seat: actor, opp: mirror}, true)
		pt, err := env.advance()
		if err != nil {
			t.Fatal(err)
		}
		if !mirror {
			if pt == nil || !pt.Opp || env.cur.Seq != d.Seq || !castsOption(env.cands, castAt) {
				t.Fatalf("unmirrored: point %v, candidates %v; want an opponent point casting option %d", pt != nil, choicesOf(env.cands), castAt)
			}
			if st.HeldUpkeepOpp != 0 || st.HeldUpkeepWalk != 0 {
				t.Fatalf("unmirrored: HeldUpkeepOpp %d, HeldUpkeepWalk %d; want 0, 0", st.HeldUpkeepOpp, st.HeldUpkeepWalk)
			}
			continue
		}
		if pt != nil && env.cur.Seq == d.Seq && castsOption(env.cands, castAt) {
			t.Fatalf("mirrored: candidates %v still cast option %d", choicesOf(env.cands), castAt)
		}
		if st.HeldUpkeepOpp == 0 || st.HeldUpkeepWalk != 0 {
			t.Fatalf("mirrored: HeldUpkeepOpp %d, HeldUpkeepWalk %d; want >0, 0", st.HeldUpkeepOpp, st.HeldUpkeepWalk)
		}
	}
}
