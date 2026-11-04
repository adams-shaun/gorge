package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The resolving-spell mana lockstep: a DB$ Mana in a
// spell's program adds ordinary mana to the pool mid-resolution, exactly as a
// triggered DB$ Mana does (gorge effMana is the one resolver; a spell is not a
// mana ability, CR 605.1a). The decks carry Konstrari Charm (its mana mode),
// Rapturous Moment and Narset's Rebuke, plus cheap spells the floating mana
// can pay in the same step, so the tally sees a spell's mana cast something.
var kdbmanaDecks = [][2][]cardset.Entry{{
	// U-R: Rapturous Moment and Narset's Rebuke; Wind Drake is also the
	// Charm's flying-damage target in the other deck.
	{
		{Name: "Rapturous Moment", Count: 4}, {Name: "Narset's Rebuke", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
	// R-G: Konstrari Charm's mana mode beside cheap green and red bodies.
	{
		{Name: "Konstrari Charm", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 6},
	},
}, {
	// the same two decks with the seats swapped, so each deck is on the play
	{
		{Name: "Konstrari Charm", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Rapturous Moment", Count: 4}, {Name: "Narset's Rebuke", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
}}

// kdbmanaTally counts on gorge's side: each carrier's resolutions, the
// Charm's mana-mode resolutions, and casts or activations in the same step
// while the pool still held a produced colour.
type kdbmanaTally struct {
	games, charmMana, follow int
	res                      map[string]int
}

func newKdbmanaTally() *kdbmanaTally {
	return &kdbmanaTally{res: map[string]int{}}
}

// kdbmanaProduced is the pool a carrier's DB$ Mana adds, by gorge's Mana
// indices (W U B R G C). OK is false for a non-carrier.
func kdbmanaProduced(name string) (m [6]bool, ok bool) {
	switch name {
	case "Konstrari Charm":
		m[state.MC] = true // Produced$ C Amount$ 3
	case "Rapturous Moment":
		m[state.MU], m[state.MR] = true, true // U U R R R
	case "Narset's Rebuke":
		m[state.MU], m[state.MR], m[state.MW] = true, true, true // U R W
	default:
		return m, false
	}
	return m, true
}

// kdbmanaPending is one carrier spell observed on gorge's stack.
type kdbmanaPending struct {
	obj     state.ObjID
	name    string
	ctrl    state.PlayerID
	modes   []string
	onStack bool
}

// kdbmanaFloat is the most recent carrier resolution for a seat: the step it
// happened in and the colours it added.
type kdbmanaFloat struct {
	ok   bool
	turn int32
	step state.Step
	col  [6]bool
}

// kdbmanaWatch follows the gorge side's stack between decisions. A carrier
// that leaves the stack for the graveyard resolved (a reversed cast returns
// to its owner's hand and is not counted); its controller then has its mana
// in the pool for the rest of the step.
type kdbmanaWatch struct {
	tal     *kdbmanaTally
	pending []kdbmanaPending
	float   [2]kdbmanaFloat
}

func kdbmanaHasMode(modes []string, want string) bool {
	for _, m := range modes {
		if m == want {
			return true
		}
	}
	return false
}

func kdbmanaHeld(pool *state.Mana, m [6]bool) bool {
	for i, want := range m {
		if want && pool[i] > 0 {
			return true
		}
	}
	return false
}

func (w *kdbmanaWatch) observe(g *GorgeSide) {
	if g == nil || w == nil || w.tal == nil {
		return
	}
	game := g.E.G
	onStack := func(id state.ObjID) bool {
		for _, s := range game.Stack {
			if s == id {
				return true
			}
		}
		return false
	}
	live := w.pending[:0]
	for _, p := range w.pending {
		ob := game.Obj(p.obj)
		if onStack(p.obj) || ob != nil && ob.Zone == state.ZStack {
			p.onStack = true
			// A modal spell is pushed while its cast-time mode answer is
			// still pending: the first observation can carry no modes, and
			// the answer lands on the same stack object before resolution.
			if ob != nil && len(ob.ChosenModes) > 0 {
				p.modes = append(p.modes[:0], ob.ChosenModes...)
			}
			live = append(live, p)
			continue
		}
		if !p.onStack {
			continue
		}
		if ob != nil && ob.Zone == state.ZGraveyard {
			w.tal.res[p.name]++
			if p.name == "Konstrari Charm" && kdbmanaHasMode(p.modes, "DBMana") {
				w.tal.charmMana++
			}
			m, _ := kdbmanaProduced(p.name)
			w.float[p.ctrl] = kdbmanaFloat{ok: true, turn: game.Turn, step: game.Step, col: m}
		}
	}
	w.pending = live
	for _, id := range game.Stack {
		ob := game.Obj(id)
		if ob == nil || ob.Face() == nil {
			continue
		}
		if _, ok := kdbmanaProduced(ob.Face().Name); !ok {
			continue
		}
		seen := false
		for _, p := range w.pending {
			if p.obj == id {
				seen = true
				break
			}
		}
		if seen {
			continue
		}
		w.pending = append(w.pending, kdbmanaPending{
			obj: id, name: ob.Face().Name, ctrl: ob.Controller,
			modes: append([]string(nil), ob.ChosenModes...), onStack: true,
		})
	}
}

// kdbmanaPolicy is the random policy with the watch on the front: when it
// picks a cast or activation by the seat whose carrier resolved this step and
// the pool still holds one of that carrier's colours, it counts the follow-up.
func kdbmanaPolicy(seed uint64, w *kdbmanaWatch) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		w.observe(c.G)
		a := rnd.Pick(c)
		if c.G != nil && c.D.Kind == DPriority && (a.Kind() == ActCast || a.Kind() == ActActivate) {
			p := c.D.Player
			if p < 2 {
				f := w.float[p]
				g := c.G.E.G
				if f.ok && f.turn == g.Turn && f.step == g.Step && kdbmanaHeld(&g.Players[p].Pool, f.col) {
					w.tal.follow++
				}
			}
		}
		return a
	})
}

// kdbmanaRun plays seeds 1..n of every matchup. It fails on each divergence
// when report is set, else returns at the first (the mutation wants one).
func kdbmanaRun(t *testing.T, n uint64, report bool, tal *kdbmanaTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dbmana")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kdbmanaDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"db-a", "db-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			w := &kdbmanaWatch{tal: tal}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, kdbmanaPolicy(seed, w), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					return div
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

func TestDbmanaLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	tal := newKdbmanaTally()
	kdbmanaRun(t, 256, true, tal)
	t.Logf("%d games: Rapturous Moment %d, Narset's Rebuke %d, Konstrari Charm %d (mana mode %d), same-step follow-ups %d",
		tal.games, tal.res["Rapturous Moment"], tal.res["Narset's Rebuke"], tal.res["Konstrari Charm"], tal.charmMana, tal.follow)
	for _, n := range []string{"Rapturous Moment", "Narset's Rebuke", "Konstrari Charm"} {
		if tal.res[n] < 20 {
			t.Errorf("%s resolved %d times, want at least 20", n, tal.res[n])
		}
	}
	if tal.charmMana < 20 {
		t.Errorf("Konstrari Charm's mana mode resolved %d times, want at least 20", tal.charmMana)
	}
	if tal.follow < 20 {
		t.Errorf("%d casts/activations followed a carrier in the same step with its mana, want at least 20", tal.follow)
	}
}

// TestDbmanaMutation: with a spell's AddMana adding nothing, the lockstep
// diverges within 64 seeds.
func TestDbmanaMutation(t *testing.T) {
	dirac.DebugDbManaResolve = true
	defer func() { dirac.DebugDbManaResolve = false }()
	if n := kdbmanaRun(t, 64, false, newKdbmanaTally()); n == 0 {
		t.Fatal("mutation: no seed diverged in 64 seeds")
	}
}
