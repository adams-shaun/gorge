package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Read ahead (CR 702.155, dirac/kw_readahead.go) in lockstep: the Elder
// Dragon War's shape, a three-chapter Saga whose chapters each change a life
// total or the hand, under a synthetic name (built in process through
// kgen.TableFromCards). The ask poses the chapter the Saga enters on (one
// option per chapter); the policy answers the lowest, the highest or the
// middle by seed, so a Saga that enters on chapter I (all three chapters
// still ahead), II and III (the later chapters only: the entry turn triggers
// no earlier one, CR 702.155a) are all played. Replenish returns the
// sacrificed Sagas to the battlefield with no spell on the stack, which is
// the queued ask's entry (a cast Saga is the pre-entry ask's); Naturalize
// destroys one early.

const readAheadProbeSrc = `Name:Read Ahead Probe
ManaCost:2
Types:Enchantment Saga
K:Read ahead
K:Chapter:3:DBLose,DBGain,DBDraw
SVar:DBLose:DB$ LoseLife | Defined$ Player.Opponent | LifeAmount$ 1 | SpellDescription$ Each opponent loses 1 life.
SVar:DBGain:DB$ GainLife | Defined$ You | LifeAmount$ 2 | SpellDescription$ You gain 2 life.
SVar:DBDraw:DB$ Draw | Defined$ You | NumCards$ 1 | SpellDescription$ Draw a card.
`

func readAheadDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{{Name: "Read Ahead Probe", Count: 12}, {Name: "Replenish", Count: 4}, {Name: "Plains", Count: 24}},
		{{Name: "Naturalize", Count: 6}, {Name: "Savannah Lions", Count: 8}, {Name: "Plains", Count: 13}, {Name: "Forest", Count: 13}},
	}
}

// readAheadStats counts what the ask posed and what the Sagas entered with.
type readAheadStats struct {
	asks, low, mid, high int
	first                [4]int               // Sagas first seen on the battlefield with 1, 2, 3 lore counters
	fromYard             int                  // of them, those that entered from a graveyard (the queued ask)
	seen                 map[state.ObjID]bool // this game's Sagas already counted (object ids repeat across games)
}

// readAheadPolicy plays castActivatePlayedPolicy and answers a read-ahead ask
// with the lowest, the highest or the middle chapter by seed.
func readAheadPolicy(seed uint64, played map[string]int, st *readAheadStats) Policy {
	base := castActivatePlayedPolicy(seed, played)
	code, _ := chooseCode("readahead")
	kind := ActChoose + CanonKind(code)
	return PolicyFunc(func(c *PickCtx) Canon {
		n := len(c.D.Options)
		if n > 0 && c.D.Options[0].Kind() == kind {
			st.asks++
			switch seed % 3 {
			case 0:
				st.low++
				return c.D.Options[0]
			case 1:
				st.high++
				return c.D.Options[n-1]
			}
			st.mid++
			return c.D.Options[n/2]
		}
		return base.Pick(c)
	})
}

// check records each Saga's lore count the first time it is on the battlefield.
func (st *readAheadStats) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	seen := st.seen
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != "Read Ahead Probe" || seen[id] {
				continue
			}
			seen[id] = true
			if n := loreOf(o); n >= 1 && n < len(st.first) {
				st.first[n]++
				if o.EnteredFrom == state.ZGraveyard {
					st.fromYard++
				}
			}
		}
	}
	return nil
}

// readAheadRun plays seeds 1..seeds; mutate runs before the games. It returns
// how many seeds diverged (report: each is an error, else the first stops).
func readAheadRun(t *testing.T, reg *cards.Registry, seeds uint64, report bool, st *readAheadStats, played map[string]int, mutate func()) int {
	t.Helper()
	decks := readAheadDecks()
	set, gd := sweepcapBuild(t, reg, readAheadProbeSrc, decks)
	tbl := sweepcapTable(t, set, decks)
	if mutate != nil {
		mutate()
	}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"readahead-a", "readahead-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		st.seen = map[state.ObjID]bool{}
		res := Lockstep(m, seed, k, readAheadPolicy(seed, played, st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: st.check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	return div
}

// TestReadAheadLockstep: the Saga plays identically in gorge and dirac over 60
// seeds, and the pool entered it on chapter I, II and III, from the stack and
// from a graveyard.
func TestReadAheadLockstep(t *testing.T) {
	reg := synthReg(t)
	var st readAheadStats
	played := map[string]int{}
	if div := readAheadRun(t, reg, 60, true, &st, played, nil); div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	if played["Read Ahead Probe"] == 0 {
		t.Fatal("the Saga was never cast")
	}
	if st.asks == 0 || st.low == 0 || st.mid == 0 || st.high == 0 {
		t.Errorf("the pool never reached every answer: %+v", st)
	}
	for n := 1; n <= 3; n++ {
		if st.first[n] == 0 {
			t.Errorf("no Saga entered with %d lore counters: %+v", n, st)
		}
	}
	if st.fromYard == 0 {
		t.Errorf("no Saga entered from a graveyard (the queued ask): %+v", st)
	}
	t.Logf("%+v", st)
}

// TestReadAheadMutation: with the entry-turn chapter gate dropped (a Saga that
// enters on chapter III triggers I and II too) some seed diverges, so the
// lockstep does observe the gate. Test-only global: never parallel.
func TestReadAheadMutation(t *testing.T) {
	reg := synthReg(t)
	defer func() { dirac.DebugReadAheadNoGate = false }()
	var st readAheadStats
	if div := readAheadRun(t, reg, 60, false, &st, map[string]int{}, func() { dirac.DebugReadAheadNoGate = true }); div == 0 {
		t.Fatal("no seed diverged with the gate dropped: the pool does not enter a Saga past chapter I")
	}
}
