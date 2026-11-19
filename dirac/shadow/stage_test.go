package shadow_test

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The verifier of NewFromWorld (the only place gorge's own state enters
// staging): for each of stageGames lockstep games of a vanilla and keyword
// pool, at a random decision k, stage from the seat's view of gorge's
// game with the true hidden zones as the world, and require the projection
// equal to the prefix-replayed dirac state's. Both are then played to the
// game's end on one shared random policy, with every decision and every
// projection equal.

const stageGames = 200

// stagePool is vanilla's lands, its creatures without an ability and Hungry
// Ghoul, whose one ability puts a +1/+1 counter on itself (Llanowar Elves and
// Jeskai Student have others and are left out). Damage comes from combat.
func stagePool() (lands, creatures []kop.CardID) {
	for id := vanilla.Plains; id <= vanilla.Forest; id++ {
		lands = append(lands, id)
	}
	for id := vanilla.SavannahLions; id <= vanilla.TundraWall; id++ {
		creatures = append(creatures, id)
	}
	creatures = append(creatures, vanilla.HungryGhoul, vanilla.HungryGhoul, vanilla.HungryGhoul)
	return
}

func stageDeck(r *rand.Rand) []kop.CardID {
	lands, creatures := stagePool()
	var d []kop.CardID
	for i := 0; i < 17; i++ {
		d = append(d, lands[r.IntN(len(lands))])
	}
	for i := 0; i < 23; i++ {
		d = append(d, creatures[r.IntN(len(creatures))])
	}
	return d
}

func stageCfg() dirac.Config {
	c := dirac.LockstepConfig(dirac.SurfaceFull)
	c.NoElide = true
	return c
}

// stageEnv is one pool a verifier plays: its table, how a deck is drawn, how
// the lockstep kernel is built, and which positions a game waits for.
type stageEnv struct {
	tbl    *kop.Table
	deck   func(r *rand.Rand) []kop.CardID
	kernel oracle.SideFactory
	// want reports whether a staged position of game seed is worth
	// verifying (a game that wants none takes the first stageable one).
	want func(seed uint64, v *view.View) bool
}

func vanillaEnv() *stageEnv {
	return &stageEnv{
		tbl: vanilla.Table, deck: stageDeck, kernel: oracle.RealKernel(vanilla.Table, nil, oracle.Config{}),
		want: func(seed uint64, v *view.View) bool { return seed%2 == 0 || hasMarks(v) }, // odd games wait for damage or a counter
	}
}

type stageStats struct {
	tried, staged int
	reasons       map[string]int
	byStep        map[string]int
	contSteps     int
	damaged       int               // staged positions with a damaged creature
	countered     int               // staged positions with a +1/+1 counter
	attached      int               // staged positions with an attached Aura or Equipment
	tokens        int               // staged positions with a token
	firstDetail   map[string]string // the first error text of each reason, for a log
	deadTok       int               // positions skipped: a token had left the view after the last one shown
}

var errDone = &oracle.Divergence{What: "stage-done"}

// stageGame plays game seed to a random decision and verifies staging at the
// first stageable one from there on. It returns a failure text or "".
func stageGame(reg *cards.Registry, env *stageEnv, seed uint64, st *stageStats, mut func(*dirac.World)) string {
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
	kstate, _, _ := oracle.KernelStateOf(k)
	want := 2 + r.IntN(160) // the decision to stage at, or the first stageable after it
	n, fail := 0, ""
	check := func(ref *oracle.GorgeSide, ker oracle.Side) *oracle.Divergence {
		n++
		if n < want {
			return nil
		}
		d := ref.Pending()
		if d == nil || d.Kind != "priority" {
			return nil
		}
		viewer := state.PlayerID(r.IntN(2))
		g := ref.E.G
		v := view.ProjectFor(g, ref.E, viewer, view.Seat, d)
		// the hidden zones as the harness knows them: the one place gorge's own state is read
		hidden := shadow.Hidden{}
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.Zone(state.ZHand, p) {
				hidden.Hand[p] = append(hidden.Hand[p], card(ref, g, id))
			}
			for _, id := range g.Zone(state.ZLibrary, p) {
				hidden.Library[p] = append(hidden.Library[p], card(ref, g, id))
			}
		}
		if !env.want(seed, &v) {
			return nil
		}
		st.tried++
		in := &shadow.Input{
			View: v, Hidden: hidden, Tbl: env.tbl, Cfg: stageCfg(), Seed: seed,
			ObjOf: func(id int) (uint8, bool) { return uint8(ref.InstRef(state.ObjID(id))), true },
		}
		w, err := shadow.WorldFromView(in)
		var staged *dirac.State
		if err == nil {
			var kp dirac.Projection
			kstate.Project(&kp)
			if len(w.Objs) != int(kp.NObj) {
				st.deadTok++ // a token left the view after the last shown one: the replayed game holds one more object
				return nil
			}
			if mut != nil {
				mut(&w)
			}
			staged, err = dirac.NewFromWorld(&w)
			if err == nil {
				st.count(&w)
			}
		}
		if err != nil {
			var u dirac.ErrUnstageable
			if !errors.As(err, &u) {
				fail = "not an ErrUnstageable: " + err.Error()
				return errDone
			}
			st.reasons[u.Reason]++
			if st.firstDetail == nil {
				st.firstDetail = map[string]string{}
			}
			if _, ok := st.firstDetail[u.Reason]; !ok {
				st.firstDetail[u.Reason] = u.Detail
			}
			return nil
		}
		st.staged++
		st.byStep[v.Step]++
		fail = compareAndPlay(kstate, staged, seed, n, st)
		return errDone
	}
	res := oracle.Lockstep(m, seed, k, oracle.RandomPolicy(seed), oracle.RunConfig{MaxDecisions: 4000, MaxTurns: 60, Check: check})
	if res.Div != nil && res.Div != errDone {
		return fmt.Sprintf("seed %d: the lockstep itself diverged: %s", seed, res.Div)
	}
	return fail
}

// count tallies the features a staged world holds, once per position.
func (st *stageStats) count(w *dirac.World) {
	var dmg, ctr, att, tok bool
	for i := range w.Objs {
		o := &w.Objs[i]
		dmg, ctr, att, tok = dmg || o.Damage != 0, ctr || o.P1P1 != 0, att || o.Attached, tok || o.Token
	}
	for _, f := range []struct {
		on bool
		n  *int
	}{{dmg, &st.damaged}, {ctr, &st.countered}, {att, &st.attached}, {tok, &st.tokens}} {
		if f.on {
			*f.n++
		}
	}
}

// hasMarks reports whether a battlefield card of v carries damage or counters.
func hasMarks(v *view.View) bool {
	for _, p := range v.Players {
		for i := range p.Battlefield {
			if c := &p.Battlefield[i]; c.Damage != 0 || len(c.Counters) != 0 {
				return true
			}
		}
	}
	return false
}

func card(ref *oracle.GorgeSide, g *state.Game, id state.ObjID) shadow.Card {
	return shadow.Card{Obj: uint8(ref.InstRef(id)), Name: g.Obj(id).Face().Name}
}

// compareAndPlay checks the staged state against the replayed one, then plays both out.
func compareAndPlay(orig *dirac.State, staged *dirac.State, seed uint64, at int, st *stageStats) string {
	cp := new(dirac.State)
	cp.CopyFrom(orig)
	var a, b dirac.Decision
	var pa, pb dirac.Projection
	rr := rand.New(rand.NewPCG(seed, 0x706c6179))
	for step := 0; ; step++ {
		cp.StepInto(&a)
		staged.StepInto(&b)
		cp.Project(&pa)
		staged.Project(&pb)
		pa.Draws, pb.Draws = 0, 0 // the staged game was not dealt, so it has drawn nothing
		if pa != pb {
			return fmt.Sprintf("seed %d, decision %d, step %d after: projections differ: %s", seed, at, step, diff(&pa, &pb))
		}
		if a.Kind != b.Kind || a.Player != b.Player || a.N != b.N || a.Options != b.Options {
			return fmt.Sprintf("seed %d, decision %d, step %d after: decisions differ: %v p%d %v / %v p%d %v", seed, at, step, a.Kind, a.Player, a.Options[:a.N], b.Kind, b.Player, b.Options[:b.N])
		}
		if a.Kind == dirac.DkNone {
			oa, wa := cp.Terminal()
			ob, wb := staged.Terminal()
			if oa != ob || wa != wb {
				return fmt.Sprintf("seed %d: terminal %v/%d vs %v/%d", seed, oa, wa, ob, wb)
			}
			return ""
		}
		if step > 6000 {
			return fmt.Sprintf("seed %d: no end after %d steps", seed, step)
		}
		st.contSteps++
		o := a.Options[rr.IntN(int(a.N))]
		cp.Apply(o)
		staged.Apply(o)
	}
}

// diff names the top-level fields of two projections that differ, and the first differing object.
func diff(a, b *dirac.Projection) string {
	var out []string
	va, vb := reflect.ValueOf(a).Elem(), reflect.ValueOf(b).Elem()
	for i := 0; i < va.NumField(); i++ {
		if !reflect.DeepEqual(va.Field(i).Interface(), vb.Field(i).Interface()) {
			f := va.Type().Field(i).Name
			if f == "Objs" {
				for j := range a.Objs {
					if a.Objs[j] != b.Objs[j] {
						out = append(out, fmt.Sprintf("Objs[%d] %+v vs %+v", j, a.Objs[j], b.Objs[j]))
						break
					}
				}
				continue
			}
			out = append(out, fmt.Sprintf("%s %v vs %v", f, va.Field(i).Interface(), vb.Field(i).Interface()))
		}
	}
	return strings.Join(out, "; ")
}

// TestStageMatchesReplay is the acceptance verifier (see the file comment).
func TestStageMatchesReplay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	st := &stageStats{reasons: map[string]int{}, byStep: map[string]int{}}
	failed := 0
	for seed := uint64(1); seed <= stageGames; seed++ {
		if msg := stageGame(reg, vanillaEnv(), seed, st, nil); msg != "" {
			t.Error(msg)
			if failed++; failed >= 5 {
				break
			}
		}
	}
	t.Logf("marks: %d staged positions held damage, %d held counters", st.damaged, st.countered)
	t.Logf("%d games: %d positions tried, %d staged (by step %v), %d continuation decisions; unstageable by reason %v", stageGames, st.tried, st.staged, st.byStep, st.contSteps, st.reasons)
	if st.damaged < 20 || st.countered < 20 {
		t.Errorf("the pool exercised too little: %d damaged and %d counter positions staged", st.damaged, st.countered)
	}
	if st.staged < stageGames*9/10 {
		t.Errorf("only %d of %d games staged a position", st.staged, stageGames)
	}
}

// TestStageMutationsAreCaught: staging that drops a feature must fail the
// verifier, so the verifier is checking each of them.
func TestStageMutationsAreCaught(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cardsettest.Require(t, "mbc")
	muts := map[string]func(*dirac.World){
		"tapped dropped": func(w *dirac.World) {
			for i := range w.Objs {
				w.Objs[i].Tapped = false
			}
		},
		"sick dropped": func(w *dirac.World) {
			for i := range w.Objs {
				w.Objs[i].Sick = false
			}
		},
		"life off by one":     func(w *dirac.World) { w.Life[1]++ },
		"land drop forgotten": func(w *dirac.World) { w.LandsPlayed[w.Active] = 0 },
		"library top swapped": func(w *dirac.World) {
			var top []*dirac.WorldObj
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Zone == kop.ZoneLibrary && o.Pos < 2 {
					top = append(top, o)
				}
			}
			if len(top) >= 4 {
				top[0].Pos, top[1].Pos = top[1].Pos, top[0].Pos
				top[2].Pos, top[3].Pos = top[3].Pos, top[2].Pos
			}
		},
		"graveyard order reversed": func(w *dirac.World) {
			var n [2]uint8
			for i := range w.Objs {
				if w.Objs[i].Zone == kop.ZoneGraveyard {
					n[w.Objs[i].Owner]++
				}
			}
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Zone == kop.ZoneGraveyard {
					o.Pos = n[o.Owner] - 1 - o.Pos
				}
			}
		},
		"damage dropped": func(w *dirac.World) {
			for i := range w.Objs {
				w.Objs[i].Damage = 0
			}
		},
		"counters dropped": func(w *dirac.World) {
			for i := range w.Objs {
				w.Objs[i].P1P1 = 0
			}
		},
		"turn off": func(w *dirac.World) { w.Turn += 2 },
	}
	runMutations(t, reg, vanillaEnv(), muts)
	cenv := cardsEnv(t)
	runMutations(t, reg, cenv, cardsMutations(cenv.tbl))
}

// runMutations requires the verifier to fail for each mutation within 120 games of env.
func runMutations(t *testing.T, reg *cards.Registry, env *stageEnv, muts map[string]func(*dirac.World)) {
	t.Helper()
	for name, mut := range muts {
		caught := false
		st := &stageStats{reasons: map[string]int{}, byStep: map[string]int{}}
		for seed := uint64(1); seed <= 120 && !caught; seed++ {
			func() {
				defer func() {
					if recover() != nil {
						caught = true
					}
				}()
				caught = stageGame(reg, env, seed, st, mut) != ""
			}()
		}
		if !caught {
			t.Errorf("mutation %q passed the verifier in 120 games", name)
		}
	}
}
