package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Firebending lockstep (CR 702.189): constructed R/W and R/G decks over
// the generated firebend pool, so every attack of a firebending carrier adds
// its persistent red and the red survives the declare-blockers, first-strike
// and regular damage steps (spent by Shock and Lightning Strike mid-combat)
// until the end-of-combat step ends, exactly as gorge's foldStepChange and
// ManaClear leave it.

var firebendDecks = [2][]cardset.Entry{
	{ // A: R/W
		{Name: "Fire Sages", Count: 3}, {Name: "Firebending Student", Count: 3}, {Name: "Sun Warriors", Count: 2},
		{Name: "Rough Rhino Cavalry", Count: 2}, {Name: "Jeong Jeong, the Deserter", Count: 2}, {Name: "Uncle Iroh", Count: 1},
		{Name: "Shock", Count: 3}, {Name: "Lightning Strike", Count: 3}, {Name: "Youthful Knight", Count: 2},
		{Name: "Iroh's Demonstration", Count: 1}, {Name: "Hill Giant", Count: 1},
		{Name: "Mountain", Count: 12}, {Name: "Plains", Count: 5},
	},
	{ // B: R/B, Marsh Gas (-2/-0) over a Student's attack trigger takes its power below 0
		{Name: "Fire Sages", Count: 2}, {Name: "Firebending Student", Count: 4}, {Name: "Rough Rhino Cavalry", Count: 2},
		{Name: "Uncle Iroh", Count: 1}, {Name: "Shock", Count: 1}, {Name: "Lightning Strike", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Marsh Gas", Count: 4}, {Name: "Disfigure", Count: 1},
		{Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 8},
	},
}

type firebendStats struct {
	adds       int // firebending red added (a rise of fbR between decisions)
	survived   int // games where firebending red floated past a step boundary
	marsh      int // Marsh Gas cast over an attack trigger
	stackCasts int // spells cast with an attack trigger on the stack
}

// firebendProbe wraps a seeded random policy biased to attack and to spend the
// floating red on instants in combat, counting what the lockstep exercised.
type firebendProbe struct {
	st     *firebendStats
	base   Policy
	r      *rand.Rand
	rs     *realSide
	tbl    *kop.Table
	prev   [dirac.MaxSeats]uint8
	born   [dirac.MaxSeats]kop.Step
	seen   bool
	gameOK bool
}

func (p *firebendProbe) pick(c *PickCtx) Canon {
	fb := p.rs.s.FirebendRed()
	step := p.rs.s.StepNow()
	for i := range fb {
		if fb[i] > p.prev[i] {
			p.st.adds++
			p.born[i] = step
		}
		if fb[i] != 0 && step != p.born[i] && !p.gameOK {
			p.gameOK = true
			p.st.survived++
		}
	}
	p.prev = fb
	var atk, casts []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActAttack:
			atk = append(atk, a)
		case ActCast:
			casts = append(casts, a)
		}
	}
	a := p.base.Pick(c)
	switch {
	case len(atk) > 0 && p.r.IntN(4) != 0:
		a = atk[p.r.IntN(len(atk))]
	case len(casts) > 0 && step >= kop.StepAttackers && fb != [dirac.MaxSeats]uint8{} && p.r.IntN(3) != 0:
		a = casts[p.r.IntN(len(casts))]
	case len(casts) > 0 && step == kop.StepAttackers && p.rs.s.StackLen() > 0:
		// a firebending trigger on the stack: answer it, preferring Marsh Gas
		// (-2/-0 takes a Student's power below 0 before the amount is read)
		a = casts[p.r.IntN(len(casts))]
		for _, c := range casts {
			if p.tbl.Cards[p.rs.s.ObjCard(dirac.Obj(c.Src()))].Name == "Marsh Gas" {
				a = c
				p.st.marsh++
			}
		}
		p.st.stackCasts++
	}
	return a
}

func firebendSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	return firebendSetupDecks(t, firebendDecks)
}

// firebendSetupDecks is firebendSetup over any two decks of the firebend pool.
func firebendSetupDecks(t *testing.T, firebendDecks [2][]cardset.Entry) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("firebend")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range firebendDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(firebendDecks[0], firebendDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

// firebendRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies. report logs the first
// divergence as a test error (the mutation run expects one and stays quiet).
func firebendRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st firebendStats) {
	t.Helper()
	reg, set, tbl, decks := firebendSetup(t)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"firebend-a", "firebend-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &firebendProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x66626e64)), rs: k.(*realSide), tbl: tbl}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestFirebendLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := firebendRun(t, 256, true)
	t.Logf("256 games, firebend %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.adds < 40 {
		t.Errorf("firebending adds: %d, want at least 40", st.adds)
	}
	if st.marsh < 5 {
		t.Errorf("Marsh Gas casts over an attack trigger: %d, want at least 5", st.marsh)
	}
	if st.survived < 10 {
		t.Errorf("games with firebending red floating past a step boundary: %d, want at least 10", st.survived)
	}
}

// TestFirebendMutation: a step boundary that keeps no firebending red must
// make the lockstep diverge.
func TestFirebendMutation(t *testing.T) {
	dirac.DebugFirebendEmpty = true
	defer func() { dirac.DebugFirebendEmpty = false }()
	if diverged, _, _ := firebendRun(t, 64, false); !diverged {
		t.Fatal("DebugFirebendEmpty: no divergence in 64 seeds")
	}
}
