package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The any-number divided-counters lockstep: Picnic Ruiner's
// Stolen Goodies ("distribute three +1/+1 counters among any number of target
// creatures you control", TargetMin$ 0) and Elusive Otter's Grove's Bounty
// (X counters over 0..X targets). A zero-target cast resolves and puts
// nothing; a division is announced from two targets up (CR 601.2d, 608.2b).
// Every other literal-N Min-0 carrier the arm lifts is in the deck too --
// Storm the Seedcore (spell), Verdurous Gearhulk and Jugan, the Rising Star
// (enters/dies triggers) and Myojin of Towering Might (activated ability) --
// so a root-declaration regression shows up here. Rhino, Terrible Trampler
// (a SubAbility$ divided PutCounter) stays refused by kgen.

var kdivctranyDecks = [2][]cardset.Entry{
	{
		{Name: "Picnic Ruiner", Count: 4}, {Name: "Elusive Otter", Count: 4}, {Name: "Storm the Seedcore", Count: 3},
		{Name: "Verdurous Gearhulk", Count: 2}, {Name: "Jugan, the Rising Star", Count: 2}, {Name: "Myojin of Towering Might", Count: 2},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Llanowar Elves", Count: 5}, {Name: "Forest", Count: 16},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Terror", Count: 2}, {Name: "Mountain", Count: 17}, {Name: "Swamp", Count: 17},
	},
}

type kdivctranyStats struct {
	games, divs int
	goodies     [3]int // Stolen Goodies casts with 0, 1 and 2+ targets
	bounty      int    // Grove's Bounty casts with X >= 2 over 2+ targets
	bountyAny   int    // Grove's Bounty stack objects
	storm       int    // Storm the Seedcore casts
	gearhulk    int    // Verdurous Gearhulk enters triggers
	jugan       int    // Jugan, the Rising Star dies triggers
	myojin      int    // Myojin of Towering Might activated abilities
	gearhulkAll int
	juganAll    int
	myojinAll   int
}

// kdivctranyShape is a stack object's name, whether it is an ability (not a
// spell), its targets and X as last seen on the stack (its targets are chosen
// after it first appears).
type kdivctranyShape struct {
	name  string
	spell bool
	n     int
	x     int32
}

// kdivctranyCheck records each divided-carrier stack object from the gorge
// side; kdivctranyTally counts them once the game is over.
func kdivctranyCheck(seen map[state.ObjID]*kdivctranyShape) func(*GorgeSide, Side) *Divergence {
	return func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		for _, id := range g.Stack {
			o := g.Obj(id)
			if o == nil {
				continue
			}
			// Triggered and activated ability objects carry no face of their
			// own; name them through the source permanent (or card) instead.
			n := ""
			if o.Face() != nil {
				n = o.Face().Name
			} else if src := g.Obj(o.Source); src != nil && src.Face() != nil {
				n = src.Face().Name
			}
			switch n {
			case "Stolen Goodies", "Grove's Bounty", "Storm the Seedcore",
				"Verdurous Gearhulk", "Jugan, the Rising Star", "Myojin of Towering Might":
			default:
				continue
			}
			sh := seen[id]
			if sh == nil {
				sh = &kdivctranyShape{name: n, spell: o.Ability == nil}
				seen[id] = sh
			}
			sh.n, sh.x = len(o.Targets), o.X
		}
		return nil
	}
}

func (st *kdivctranyStats) tally(seen map[state.ObjID]*kdivctranyShape) {
	for _, sh := range seen {
		switch sh.name {
		case "Stolen Goodies":
			st.goodies[min(sh.n, 2)]++
		case "Grove's Bounty":
			st.bountyAny++
			if sh.x >= 2 && sh.n >= 2 {
				st.bounty++
			}
		case "Storm the Seedcore":
			st.storm++
		case "Verdurous Gearhulk":
			st.gearhulkAll++
			if !sh.spell {
				st.gearhulk++
			}
		case "Jugan, the Rising Star":
			st.juganAll++
			if !sh.spell {
				st.jugan++
			}
		case "Myojin of Towering Might":
			st.myojinAll++
			if !sh.spell {
				st.myojin++
			}
		}
	}
}

func runDivctrany(t *testing.T, decksIn [2][]cardset.Entry, seeds int, mutate func()) kdivctranyStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("divctrany")
	if err != nil {
		t.Fatal(err)
	}
	oldN, oldM := dirac.DebugDivNOneEach, dirac.DebugDivMaxVMinus1
	defer func() { dirac.DebugDivNOneEach, dirac.DebugDivMaxVMinus1 = oldN, oldM }()
	if mutate != nil {
		mutate()
	}
	var decks [2][]*cards.Card
	for s, d := range decksIn {
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
	tbl, _, _, err := set.Matchup(decksIn[0], decksIn[1])
	if err != nil {
		t.Fatal(err)
	}
	var st kdivctranyStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdivctrany-a", "kdivctrany-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[state.ObjID]*kdivctranyShape{}
		res := Lockstep(m, seed, k, kdivctranyPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: kdivctranyCheck(seen)})
		CloseSide(k)
		st.tally(seen)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			if mutate == nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) && mutate == nil {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
	}
	return st
}

// kdivctranyPolicy plays lands, then casts whenever it can; every other pick
// (targets, how many, X, the division) is random.
func kdivctranyPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			// an X above 7 would outgrow the kernel's division arrays (a loud
			// StallXTgt), so X is announced up to 7
			for _, a := range c.D.Options {
				if xc, _ := chooseCode("x"); a.Kind() != ActChoose+CanonKind(xc) || a.Mode() <= 7 {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestDivctranyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runDivctrany(t, kdivctranyDecks, 256, nil)
	t.Logf("%d games, %d divergences; Stolen Goodies casts with 0/1/2+ targets: %d/%d/%d; Grove's Bounty stack objects %d, X>=2 over 2+ targets %d; other literal carriers: Storm %d, Gearhulk %d (all %d), Jugan %d (all %d), Myojin %d (all %d)",
		st.games, st.divs, st.goodies[0], st.goodies[1], st.goodies[2], st.bountyAny, st.bounty, st.storm, st.gearhulk, st.gearhulkAll, st.jugan, st.juganAll, st.myojin, st.myojinAll)
	if st.divs != 0 {
		t.Fatalf("%d divergences", st.divs)
	}
	for i, n := range st.goodies {
		if n < 3 {
			t.Errorf("only %d Stolen Goodies casts with %d targets (2 = two or more)", n, i)
		}
	}
	if st.bounty < 3 {
		t.Errorf("only %d Grove's Bounty casts with X >= 2 over 2+ targets", st.bounty)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Storm the Seedcore casts", st.storm},
		{"Verdurous Gearhulk enters triggers", st.gearhulk},
		{"Jugan, the Rising Star dies triggers", st.jugan},
		{"Myojin of Towering Might activations", st.myojin},
	} {
		if c.n < 3 {
			t.Errorf("only %d %s", c.n, c.name)
		}
	}
}

// TestDivctranyMutation: one counter per target (the announced shares lost),
// and a DivMaxV total of X-1, must each diverge.
func TestDivctranyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mu := range []struct {
		name string
		set  func()
	}{
		{"one-each", func() { dirac.DebugDivNOneEach = true }},
		{"x-minus-1", func() { dirac.DebugDivMaxVMinus1 = true }},
	} {
		st := runDivctrany(t, kdivctranyDecks, 64, mu.set)
		if st.divs == 0 {
			t.Fatalf("%s: mutation ran %d games with no divergence", mu.name, st.games)
		}
		t.Logf("%s: diverged in %d of %d games", mu.name, st.divs, st.games)
	}
}
