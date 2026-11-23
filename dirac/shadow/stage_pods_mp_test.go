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

// The pod verifier: the staging verifier (stage_test.go) at four seats, every
// seat still in the game. Each game is a four-seat lockstep game of the
// vanilla pool; a position is staged from one seat's view with the true hidden
// zones of all four and held to the replayed state.

const (
	podSeats       = 4
	podGames       = 160
	podCombatGames = 240
	// podDepartedGames is the games of the departed-seat verifier: each plays to
	// the first loss, late in the game.
	podDepartedGames = 60
	podModes         = 9
)

// podsEnv is the vanilla pool at four seats. Seeds 0 mod 4 take the first
// stageable position, 1 mod 4 wait for damage or a counter, 2 mod 4 for a
// non-empty stack whose top entry is the active seat's, 3 mod 4 for an
// attacker on a seat other than the one after the active seat.
func podsEnv() *stageEnv {
	env := vanillaEnv()
	env.seats = podSeats
	env.want = func(seed uint64, v *view.View, _ *decision.Decision) bool {
		switch seed % 4 {
		case 1:
			return hasMarks(v)
		case 2:
			n := len(v.Stack)
			return n != 0 && v.Stack[n-1].Controller == v.Active
		case 3:
			return hasBF(v, func(c *view.CardView) bool {
				return c.Attacking && c.AttackingPlayer != nil && int(*c.AttackingPlayer) != (int(v.Active)+1)%podSeats
			})
		}
		return true
	}
	return env
}

// podsDepartedEnv is podsEnv wanting only positions after a seat has left the
// game (a lost player in the view).
func podsDepartedEnv() *stageEnv {
	env := vanillaEnv()
	env.seats = podSeats
	env.want = func(seed uint64, v *view.View, _ *decision.Decision) bool {
		lost := 0
		for _, p := range v.Players {
			if p.Lost {
				lost++
			}
		}
		return lost >= 1+int(seed%2) // odd seeds wait for a second seat to leave
	}
	return env
}

// podCombatEnv is combatEnv at four seats with a ninth mode: the declare
// blockers decision of a defender other than the active seat's next.
func podCombatEnv() *stageEnv {
	env := combatEnv()
	env.seats = podSeats
	base := env.want
	env.want = func(seed uint64, v *view.View, d *decision.Decision) bool {
		if seed%podModes == podModes-1 {
			return d.Kind == decision.KBlockers && int(d.Player) != (int(v.Active)+1)%podSeats
		}
		return base(seed%podModes, v, d)
	}
	return env
}

func newStats() *stageStats {
	return &stageStats{reasons: map[string]int{}, byStep: map[string]int{}, byKind: map[string]int{}}
}

func TestStagePodsMatchReplay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	env := podsEnv()
	st := newStats()
	failed := 0
	for seed := uint64(1); seed <= podGames; seed++ {
		if msg := stageGame(reg, env, seed, st, nil); msg != "" {
			t.Error(msg)
			if failed++; failed >= 5 {
				break
			}
		}
	}
	t.Logf("%d four-seat games: %d positions tried, %d staged (by step %v), %d continuation decisions; unstageable by reason %v", podGames, st.tried, st.staged, st.byStep, st.contSteps, st.reasons)
	t.Logf("first detail of each reason: %v", st.firstDetail)
	t.Logf("staged positions with marks %d, a stack entry %d, an attack on a non-adjacent defender %d", st.marked, st.stacked, st.farAttack)
	if st.staged < podGames*3/4 {
		t.Errorf("only %d of %d games staged a position", st.staged, podGames)
	}
	if st.marked < 20 || st.stacked < 10 || st.farAttack < 20 {
		t.Errorf("the pods exercised too little: %d marked, %d stacked, %d non-adjacent attack positions staged", st.marked, st.stacked, st.farAttack)
	}
}

// TestStagePodsDepartedMatchReplay: positions after a seat has left the game
// stage and match the replayed game, played out to its end. A game costs about
// half a second (the seat leaves late), so it runs podDepartedGames, not
// podGames, to stay inside the test budget.
func TestStagePodsDepartedMatchReplay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	env := podsDepartedEnv()
	st := newStats()
	failed := 0
	for seed := uint64(1); seed <= podDepartedGames; seed++ {
		if msg := stageGame(reg, env, seed, st, nil); msg != "" {
			t.Error(msg)
			if failed++; failed >= 5 {
				break
			}
		}
	}
	t.Logf("%d four-seat games: %d positions tried, %d staged (by step %v), %d continuation decisions; unstageable by reason %v", podDepartedGames, st.tried, st.staged, st.byStep, st.contSteps, st.reasons)
	t.Logf("staged positions with a departed seat %d, with two %d", st.departed, st.departed2)
	if st.departed < 20 || st.departed2 < 5 {
		t.Errorf("the departed pods exercised too little: %d positions with a departed seat, %d with two, want 20 and 5", st.departed, st.departed2)
	}
}

func TestStageCombatPodsMatchReplay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	env := podCombatEnv()
	st := newStats()
	failed := 0
	for seed := uint64(1); seed <= podCombatGames; seed++ {
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
	t.Logf("%d four-seat games: %d tried, %d staged, %d continuation decisions; staged by kind@step %v; unstageable by reason %v", podCombatGames, st.tried, st.staged, st.contSteps, kinds, st.reasons)
	t.Logf("first detail of each reason: %v; blockers poses of a defender after the active seat's next: %d", st.firstDetail, st.farBlock)
	for _, k := range []string{"priority@begin-combat", "priority@declare-attackers", "priority@declare-blockers", "attackers@declare-attackers", "blockers@declare-blockers", "priority@upkeep", "priority@end"} {
		if st.byKind[k] < 15 {
			t.Errorf("only %d staged positions of %s, want 15", st.byKind[k], k)
		}
	}
	if st.farBlock < 15 {
		t.Errorf("only %d staged blockers poses of a defender other than the next seat, want 15", st.farBlock)
	}
}

// podMutations bend each field the pod staging adds: the verifier must fail on
// every one.
func podMutations() map[string]func(*dirac.World) {
	// attackedSeats is the set of seats w's attackers attack.
	attackedSeats := func(w *dirac.World) (n int) {
		var seen [dirac.MaxSeats]bool
		for i := range w.Objs {
			if o := &w.Objs[i]; o.Attacking && !seen[o.AtkSeat] {
				seen[o.AtkSeat] = true
				n++
			}
		}
		return n
	}
	return map[string]func(*dirac.World){
		"seat 3 life off by one": func(w *dirac.World) { w.Life[3]++ },
		"seat 2 library top swapped": func(w *dirac.World) {
			var top []*dirac.WorldObj
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Zone == kop.ZoneLibrary && o.Owner == 2 && o.Pos < 2 {
					top = append(top, o)
				}
			}
			if len(top) == 2 {
				top[0].Pos, top[1].Pos = top[1].Pos, top[0].Pos
			}
		},
		"attacked seat moved": func(w *dirac.World) {
			for i := range w.Objs {
				if w.Objs[i].Blocking {
					return
				}
			}
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Attacking {
					o.AtkSeat = (o.AtkSeat + 1) % w.NSeat
					if o.AtkSeat == w.Active {
						o.AtkSeat = (o.AtkSeat + 1) % w.NSeat
					}
				}
			}
		},
		"blocking defender moved": func(w *dirac.World) {
			if w.Pose != dirac.PoseBlockers || attackedSeats(w) < 2 {
				return
			}
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Attacking && o.AtkSeat != w.BlkDef {
					w.BlkDef = o.AtkSeat
					return
				}
			}
		},
		"priority holder moved": func(w *dirac.World) {
			if len(w.Stack) == 0 && w.Pose == dirac.PosePriority {
				w.Prio = (w.Prio + 1) % w.NSeat
			}
		},
	}
}

func TestStagePodMutationsAreCaught(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	muts := podMutations()
	// the defender mutation needs a blockers pose: the combat env stages them
	runMutations(t, reg, podCombatEnv(), map[string]func(*dirac.World){"blocking defender moved": muts["blocking defender moved"]})
	delete(muts, "blocking defender moved")
	runMutations(t, reg, podsEnv(), muts)
	runMutations(t, reg, podsDepartedEnv(), map[string]func(*dirac.World){"departed seat revived": reviveDeparted})
}

// reviveDeparted puts the objects of every departed seat back in its library, as
// if it had not left. Its life goes back up too: at 0 the staged game's state-based
// actions would make the seat leave again and undo the mutation.
func reviveDeparted(w *dirac.World) {
	for p := uint8(0); p < w.NSeat; p++ {
		if !w.Lost[p] {
			continue
		}
		w.Lost[p], w.Life[p] = false, 20
		n := uint8(0)
		for i := range w.Objs {
			if o := &w.Objs[i]; o.Owner == p && o.Zone == kop.ZoneNone {
				o.Zone, o.Pos = kop.ZoneLibrary, n
				n++
			}
		}
	}
}
