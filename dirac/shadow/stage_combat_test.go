package shadow_test

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The verifier of combat staging: each game of the vanilla and keyword pool
// waits, from a random decision on, for one kind of position (its mode, the
// seed mod combatModes): a priority round at the beginning of combat, the
// declare attackers step with attackers declared, the declare blockers step
// with blocks declared, the declare attackers decision, the declare blockers
// decision, the first decision of a turn and the last. The position is staged
// from the deciding seat's view and held to the replayed state, then both are
// played to the game's end.

const MaxObjs = dirac.MaxObj

const (
	combatGames    = 320
	combatModes    = 8
	combatMinKind  = 20
	modeBegin      = 0 // priority at the beginning of combat
	modeAtkPrio    = 1 // priority with attackers declared
	modeBlkPrio    = 2 // priority with blocks declared
	modeAtkPose    = 3 // the declare attackers decision
	modeBlkPose    = 4 // the declare blockers decision
	modeFirst      = 5 // the first decision of a turn
	modeLast       = 6 // the last decision of a turn
	modeBlkPrioAny = 7 // priority at the declare blockers step, blocks or none
)

func hasCombat(v *view.View, f func(*view.CardView) bool) bool { return hasBF(v, f) }

// combatEnv is the vanilla pool with the combat decisions staged.
func combatEnv() *stageEnv {
	env := vanillaEnv()
	env.combat = true
	env.want = func(seed uint64, v *view.View, d *decision.Decision) bool {
		prio := d.Kind == decision.KPriority
		switch seed % combatModes {
		case modeBegin:
			return prio && v.Step == "begin-combat"
		case modeAtkPrio:
			return prio && v.Step == "declare-attackers" && hasCombat(v, func(c *view.CardView) bool { return c.Attacking })
		case modeBlkPrio:
			return prio && v.Step == "declare-blockers" && hasCombat(v, func(c *view.CardView) bool { return c.Blocked })
		case modeAtkPose:
			return d.Kind == decision.KAttackers
		case modeBlkPose:
			return d.Kind == decision.KBlockers
		case modeFirst:
			return prio && v.Step == "upkeep" && v.Turn >= 2
		case modeLast:
			return prio && v.Step == "end" && v.Priority != v.Active
		}
		return prio && v.Step == "declare-blockers"
	}
	return env
}

// TestStageCombatMatchesReplay stages mid-turn and combat positions at every
// decision kind it admits and requires each to equal the replayed state.
func TestStageCombatMatchesReplay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	env := combatEnv()
	st := &stageStats{reasons: map[string]int{}, byStep: map[string]int{}, byKind: map[string]int{}}
	failed := 0
	for seed := uint64(1); seed <= combatGames; seed++ {
		if msg := stageGame(reg, env, seed, st, nil); msg != "" {
			t.Error(msg)
			if failed++; failed >= 5 {
				break
			}
		}
	}
	var kinds []string
	for k, n := range st.byKind {
		kinds = append(kinds, k+"="+itoa(n))
	}
	sort.Strings(kinds)
	t.Logf("%d games: %d tried, %d staged, %d continuation decisions; staged by kind@step %v; unstageable by reason %v", combatGames, st.tried, st.staged, st.contSteps, kinds, st.reasons)
	t.Logf("first detail of each reason: %v", st.firstDetail)
	for _, k := range []string{"priority@begin-combat", "priority@declare-attackers", "priority@declare-blockers", "attackers@declare-attackers", "blockers@declare-blockers", "priority@upkeep", "priority@end"} {
		if st.byKind[k] < combatMinKind {
			t.Errorf("only %d staged positions of %s, want %d", st.byKind[k], k, combatMinKind)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// combatMutations drop or bend each combat feature of a staged world: the
// verifier must fail on every one.
func combatMutations() map[string]func(*dirac.World) {
	each := func(f func(o *dirac.WorldObj)) func(*dirac.World) {
		return func(w *dirac.World) {
			for i := range w.Objs {
				f(&w.Objs[i])
			}
		}
	}
	return map[string]func(*dirac.World){
		"attackers dropped": each(func(o *dirac.WorldObj) {
			if o.Attacking {
				o.Attacking, o.Blocked = false, false
			}
		}),
		"blocks dropped": each(func(o *dirac.WorldObj) { o.Blocking, o.Blocked = false, false }),
		// two blockers of one attacker trade their places in its block order
		"block order swapped": func(w *dirac.World) {
			var first [MaxObjs]*dirac.WorldObj
			for i := range w.Objs {
				o := &w.Objs[i]
				if !o.Blocking {
					continue
				}
				if f := first[o.Blocks]; f == nil {
					first[o.Blocks] = o
				} else if f.BlockOrd != 0 {
					f.BlockOrd, o.BlockOrd = o.BlockOrd, f.BlockOrd
					first[o.Blocks] = &dirac.WorldObj{}
				}
			}
		},
		"wrong step": func(w *dirac.World) {
			if w.Step == kop.StepBlockers {
				w.Step = kop.StepAttackers
				for i := range w.Objs {
					w.Objs[i].Blocking, w.Objs[i].Blocked = false, false
				}
			}
		},
		"pose dropped": func(w *dirac.World) { w.Pose = dirac.PosePriority },
	}
}
