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

// The PersistentMana$ True lockstep: R/G attackers whose
// Attacks trigger adds mana that survives every step boundary until the turn
// ends (Brazen Collector, Savage Ventmaw, beside a Firebending Student's red
// that ends with combat) and G/W Branch of Vitu-Ghazi turned face up, with
// instants that spend the kept mana in a later step.

var persistDecks = [2][]cardset.Entry{
	{ // A: R/G; Branch of Vitu-Ghazi too, since gorge offers a face-down cast only when the floating pool pays it (a Ventmaw's six kept units)
		{Name: "Brazen Collector", Count: 4}, {Name: "Savage Ventmaw", Count: 4}, {Name: "Firebending Student", Count: 2},
		{Name: "Branch of Vitu-Ghazi", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Lightning Strike", Count: 3},
		{Name: "Giant Growth", Count: 3}, {Name: "Youthful Knight", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
	{ // B: G/W
		{Name: "Branch of Vitu-Ghazi", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Air Elemental", Count: 2}, {Name: "Giant Growth", Count: 3}, {Name: "Youthful Knight", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 9},
	},
}

type persistStats struct {
	adds     int // persistent units added (a rise of pm between decisions)
	spent    int // games where a persistent unit was spent in a later step than it was made
	together int // games where firebending and persistent red floated together
	branch   int // Branch of Vitu-Ghazi turned face up (two units of one colour in one step)
}

// persistProbe wraps a seeded random policy biased to attack and to cast
// instants while persistent mana floats.
type persistProbe struct {
	st          *persistStats
	base        Policy
	r           *rand.Rand
	rs          *realSide
	tbl         *kop.Table
	prev        [dirac.MaxSeats][kop.NumManaKinds]uint8
	born        [dirac.MaxSeats]kop.Step
	spent, both bool
}

func (p *persistProbe) pick(c *PickCtx) Canon {
	pm := p.rs.s.PersistMana()
	fb := p.rs.s.FirebendRed()
	step := p.rs.s.StepNow()
	any := false
	for i := range pm {
		var tot, prev int
		for col := range pm[i] {
			tot += int(pm[i][col])
			prev += int(p.prev[i][col])
		}
		if tot > prev {
			p.st.adds++
			for col := range pm[i] {
				if tot-prev == 2 && int(pm[i][col])-int(p.prev[i][col]) == 2 {
					p.st.branch++
				}
			}
			p.born[i] = step
		}
		if tot != 0 {
			any = true
		}
		if tot < prev && step != p.born[i] && !p.spent {
			p.spent = true
			p.st.spent++
		}
		if tot != 0 && fb[i] != 0 && pm[i][kop.R] != 0 && !p.both {
			p.both = true
			p.st.together++
		}
	}
	p.prev = pm
	var atk, casts, face, lands []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActSpecial:
			if a.Mode()&0x0f == kfacedownTurnUp {
				face = append(face, a) // Branch of Vitu-Ghazi turned face up
			}
		case ActCastFace:
			if a.Mode() == faceCodeDisguised {
				face = append(face, a) // cast face down
			}
		case ActPlayLand:
			// keep Branch of Vitu-Ghazi in hand to be cast face down: play
			// another land when there is one
			if p.tbl.Cards[p.rs.s.ObjCard(dirac.Obj(a.Src()))].Name != "Branch of Vitu-Ghazi" {
				lands = append(lands, a)
			}
		case ActAttack:
			atk = append(atk, a)
		case ActCast:
			casts = append(casts, a)
		}
	}
	a := p.base.Pick(c)
	if a.Kind() == ActPlayLand && len(lands) > 0 && p.tbl.Cards[p.rs.s.ObjCard(dirac.Obj(a.Src()))].Name == "Branch of Vitu-Ghazi" && p.r.IntN(8) != 0 {
		a = lands[p.r.IntN(len(lands))]
	}
	switch {
	case len(face) > 0 && p.r.IntN(2) != 0:
		a = face[p.r.IntN(len(face))]
	case len(atk) > 0 && p.r.IntN(4) != 0:
		a = atk[p.r.IntN(len(atk))]
	case len(casts) > 0 && any && p.r.IntN(3) != 0:
		a = casts[p.r.IntN(len(casts))]
	}
	return a
}

func persistSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("persist")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range persistDecks {
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
	return reg, set, decks
}

// persistRun plays seeds 1..seeds and returns whether a game diverged, the
// kernel capacity stall count and the probe's tallies.
func persistRun(t *testing.T, seeds uint64, report bool) (diverged bool, stalls int, st persistStats) {
	t.Helper()
	reg, set, decks := persistSetup(t)
	tbl, _, _, err := set.Matchup(persistDecks[0], persistDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"persist-a", "persist-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &persistProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x706d616e)), rs: k.(*realSide), tbl: tbl}
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

func TestPersistLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := persistRun(t, 256, true)
	t.Logf("256 games, persist %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.spent < 40 {
		t.Errorf("games with a persistent unit spent in a later step: %d, want at least 40", st.spent)
	}
	if st.branch < 5 {
		t.Errorf("Branch of Vitu-Ghazi turned face up: %d, want at least 5", st.branch)
	}
	if st.together < 10 {
		t.Errorf("games with firebending and persistent red floating together: %d, want at least 10", st.together)
	}
}

// TestPersistMutation: a step boundary that keeps no persistent units must
// make the lockstep diverge.
func TestPersistMutation(t *testing.T) {
	dirac.DebugPersistEmpty = true
	defer func() { dirac.DebugPersistEmpty = false }()
	if diverged, _, _ := persistRun(t, 64, false); !diverged {
		t.Fatal("DebugPersistEmpty: no divergence in 64 seeds")
	}
}
