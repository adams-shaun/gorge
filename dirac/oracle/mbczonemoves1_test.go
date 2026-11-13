package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The MBC zone-moves-1 family (kgen census, spelling by spelling). Each
// spelling that lowers has a fixture-pool lockstep game here that plays one of
// its cards and fails without the lowering.

// Elda, Conjurer of Spectacle: "The first nonlegendary creature spell you cast
// each turn has demonstrate" is an AffectedZone$ Stack Demonstrate grant behind
// a CheckSVar$ X / SVarCompare$ EQ0 gate (X = Count$ThisTurnCast_Creature.YouCtrl+nonLegendary).
// gorge reads the gate against the casts PRIOR to the in-flight one
// (rules hasKeywordPriorCasts), so the first such spell poses the demonstrate
// election and the second, or a legendary one, does not.
var mbcZoneMoves1Elda = [2][]cardset.Entry{
	{
		{Name: "Elda, Conjurer of Spectacle", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Giant Spider", Count: 2},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6}, {Name: "Giant Spider", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 12},
	},
}

// Silverquill Lecturer's ungated "Creature spells you cast have demonstrate" is
// the same grant without the gate: every creature spell its controller casts
// poses the election.
var mbcZoneMoves1Lecturer = [2][]cardset.Entry{
	{
		{Name: "Silverquill Lecturer", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Savannah Lions", Count: 8},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 9},
	},
	mbcZoneMoves1Elda[1],
}

// Siphon Insight (Brainstealer Dragon's grant: "you may play those cards for as
// long as they remain exiled, and you may spend mana as though it were mana of
// any color"): a blue-black deck exiles a green or red card of the other deck's
// top two and casts it with Islands and Swamps. Without MayPlayIgnoreColor$ the
// colored pip cannot be paid.
var mbcZoneMoves1Insight = [2][]cardset.Entry{
	{
		{Name: "Siphon Insight", Count: 8}, {Name: "Island", Count: 16}, {Name: "Swamp", Count: 16},
	},
	{
		{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 12}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

// mbcZoneMoves1Run plays md and its seat-swapped twin over seeds 1..seeds with
// a policy that answers every yes/no ask yes and casts whenever it can
// (kdemoPolicy). mutate, when set, edits each kernel table first; check, when
// set, is called after every decision. It returns the divergent games.
func mbcZoneMoves1Run(t *testing.T, md [2][]cardset.Entry, seeds uint64, mutate func(*kop.Table), check func(mi int) func(*GorgeSide, Side) *Divergence) (div int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbczonemoves1")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md := range [][2][]cardset.Entry{md, {md[1], md[0]}} {
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
		if mutate != nil {
			mutate(tbl)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"mbc-a", "mbc-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30}
			if check != nil {
				rc.Check = check(mi)
			}
			res := Lockstep(m, seed, k, kdemoPolicy(seed), rc)
			CloseSide(k)
			if d := res.Div; d != nil {
				div++
				t.Logf("matchup %d seed %d: diverged at decision %d: %s %s %v %s", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail)
			}
		}
	}
	return div
}

// TestMBCZoneMoves1Lowers: each lowered spelling plays decision for decision.
func TestMBCZoneMoves1Lowers(t *testing.T) {
	// Elda's gated Demonstrate grant lowers, and the grant is really read:
	// the kernel mints demonstrate copies.
	t.Run("Elda", func(t *testing.T) {
		t.Parallel()
		var minted int
		div := mbcZoneMoves1Run(t, mbcZoneMoves1Elda, 24, nil, func(int) func(*GorgeSide, Side) *Divergence {
			tal := &kdemoTally{live: map[int]uint8{}}
			return func(g *GorgeSide, k Side) *Divergence {
				d := tal.check(g, k)
				minted = max(minted, tal.minted[0]+tal.minted[1])
				return d
			}
		})
		if div != 0 {
			t.Fatalf("%d divergent games", div)
		}
		t.Logf("%d demonstrate copies minted at most in one game", minted)
		if minted < 1 {
			t.Fatalf("no demonstrate copy was minted")
		}
	})
	t.Run("Lecturer", func(t *testing.T) {
		t.Parallel()
		var minted int
		div := mbcZoneMoves1Run(t, mbcZoneMoves1Lecturer, 24, nil, func(int) func(*GorgeSide, Side) *Divergence {
			tal := &kdemoTally{live: map[int]uint8{}}
			return func(g *GorgeSide, k Side) *Divergence {
				d := tal.check(g, k)
				minted = max(minted, tal.minted[0]+tal.minted[1])
				return d
			}
		})
		if div != 0 {
			t.Fatalf("%d divergent games", div)
		}
		t.Logf("%d demonstrate copies minted at most in one game", minted)
		if minted < 1 {
			t.Fatalf("no demonstrate copy was minted")
		}
	})
	// Siphon Insight's permanent may-play carries MayPlayIgnoreColor$.
	t.Run("MayPlayIgnoreColor", func(t *testing.T) {
		t.Parallel()
		cast := 0
		div := mbcZoneMoves1Run(t, mbcZoneMoves1Insight, 24, nil, func(mi int) func(*GorgeSide, Side) *Divergence {
			seen := map[int]bool{}
			return func(_ *GorgeSide, k Side) *Divergence {
				r, ok := k.(*realSide)
				if !ok {
					return nil
				}
				// the green and red cards are the other deck's: the Siphon deck
				// controls one only by casting what it exiled
				for i := 0; i < int(r.kp.NObj); i++ {
					o := &r.kp.Objs[i]
					if o.Zone == kop.ZoneBattlefield && o.Ctrl&1 == uint8(mi) && o.Owner&1 != uint8(mi) && !seen[i] {
						seen[i] = true
						cast++
					}
				}
				return nil
			}
		})
		if div != 0 {
			t.Fatalf("%d divergent games", div)
		}
		t.Logf("%d exiled cards put onto the battlefield (cast with off-colour mana, or played as lands)", cast)
		if cast < 4 {
			t.Fatalf("%d exiled cards were put onto the battlefield, want at least 4", cast)
		}
	})
}

// TestMBCZoneMoves1Mutations proves the lockstep reads each lowering: with the
// stack grants ignored the kernel never poses gorge's demonstrate election;
// with the grant's gate dropped it poses one for every nonlegendary creature
// spell, not the first; with the any-colour rider dropped it cannot cast the
// exiled card.
func TestMBCZoneMoves1Mutations(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugStackKwOff = true
	div := mbcZoneMoves1Run(t, mbcZoneMoves1Elda, 24, nil, nil)
	dirac.DebugStackKwOff = false
	if div == 0 {
		t.Error("ignoring the stack grants left the lockstep clean")
	}
	gated := 0
	div = mbcZoneMoves1Run(t, mbcZoneMoves1Elda, 24, func(tb *kop.Table) {
		for i := range tb.Cards {
			for j := range tb.Cards[i].Abilities {
				if a := &tb.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatStackKw && a.Static.Cond2 != 0 {
					a.Static.Cond2 = 0
					gated++
				}
			}
		}
	}, nil)
	if gated == 0 {
		t.Error("no gated stack grant in the table to drop")
	}
	if div == 0 {
		t.Error("dropping the grant's gate left the lockstep clean")
	}
	dropped := 0
	div = mbcZoneMoves1Run(t, mbcZoneMoves1Insight, 24, func(tb *kop.Table) {
		for i := range tb.Cards {
			for j := range tb.Cards[i].Abilities {
				prog := tb.Cards[i].Abilities[j].Prog
				for x := range prog {
					if in := &prog[x]; in.Op == kop.OpMayPlay && in.B.N&kop.MayPlayIgnoreColor != 0 {
						in.B.N &^= kop.MayPlayIgnoreColor
						dropped++
					}
				}
			}
		}
	}, nil)
	if dropped == 0 {
		t.Error("no MayPlayIgnoreColor may-play in the table to drop")
	}
	if div == 0 {
		t.Error("dropping the any-colour rider left the lockstep clean")
	}
}
