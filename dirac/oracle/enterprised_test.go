package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The U.S.S. Enterprise-D first-time counter lockstep: its CounterAddedOnce
// line gates on FirstTime$ True, so only the turn's first batch of CHARGE
// counters put on it exiles a card and grants the may-play window. Two
// Station activations in one turn (a Spacecraft stationing a second creature)
// must produce one trigger, not two, and the next turn's first Station
// activation must produce another. The lockstep compares every decision,
// including which card is exiled and whether it may be played.
var kenterpriseDecks = [2][]cardset.Entry{
	{
		{Name: "U.S.S. Enterprise-D, Galaxy-Class", Count: 4},
		{Name: "Savannah Lions", Count: 8}, {Name: "Elite Vanguard", Count: 8},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 20}, {Name: "Mountain", Count: 20},
	},
}

// enterprisePolicy takes a land play, cast, special action (Station) or
// activation whenever one is offered, so the games reach Enterprise-D, the
// creatures that crew it and two Station activations in one turn, else plays
// randomly.
func enterprisePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActPlayLand, ActCast, ActSpecial, ActActivate:
				buf = append(buf, a)
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

// enterpriseFiredKey is one Enterprise-D source in one turn: the latch is
// per object and the used records clear at the turn change.
type enterpriseFiredKey struct {
	obj  dirac.Obj
	turn uint16
}

// runEnterprise plays the matchup for seeds 1..seeds and returns the games,
// divergences, stalls and the first-time latch evidence: how many Enterprise-D
// batches fired, how many were refused, how many objects both fired and were
// refused in one turn (two Station activations), and how many objects fired in
// two or more turns.
func runEnterprise(t *testing.T, seeds int, mutate bool) (games, divs, stalls, fired, refused, sameTurn, twoTurns int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("enterprised")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCounterFirstOff
	dirac.DebugCounterFirstOff = mutate
	defer func() { dirac.DebugCounterFirstOff = old }()
	var log []dirac.CounterFirstRec
	dirac.CounterFirstLog = &log
	defer func() { dirac.CounterFirstLog = nil }()

	var decks [2][]*cards.Card
	for s, d := range kenterpriseDecks {
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
	tbl, _, _, err := set.Matchup(kenterpriseDecks[0], kenterpriseDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	const name = "U.S.S. Enterprise-D, Galaxy-Class"
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kenterprise-a", "kenterprise-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := enterprisePolicy(seed)
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		log = log[:0]
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		fire, deny := map[enterpriseFiredKey]bool{}, map[enterpriseFiredKey]bool{}
		for _, r := range log {
			if r.Card != name {
				continue
			}
			key := enterpriseFiredKey{obj: r.Obj, turn: r.Turn}
			if r.Fired {
				fire[key] = true
				fired++
			} else {
				deny[key] = true
				refused++
			}
		}
		objTurns := map[dirac.Obj]map[uint16]bool{}
		for key := range fire {
			if deny[key] {
				sameTurn++
			}
			m := objTurns[key.obj]
			if m == nil {
				m = map[uint16]bool{}
				objTurns[key.obj] = m
			}
			m[key.turn] = true
		}
		for _, turns := range objTurns {
			if len(turns) >= 2 {
				twoTurns++
			}
		}
	}
	return
}

func TestEnterpriseDLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, fired, refused, sameTurn, twoTurns := runEnterprise(t, 32, false)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	t.Logf("%d games: %d Enterprise-D first-time firings, %d refusals, %d objects fired and refused in one turn, %d objects fired on two turns",
		games, fired, refused, sameTurn, twoTurns)
	// The test must reach the precondition it compares: two Station batches
	// in one turn (one fire, one refusal) and a fire on another turn.
	if fired < 2 {
		t.Fatalf("only %d Enterprise-D first-time firings; the games never reached the trigger", fired)
	}
	if refused < 1 {
		t.Fatalf("no Enterprise-D later-batch refusal; the games never put charge counters on it twice")
	}
	if sameTurn < 1 {
		t.Fatalf("no Enterprise-D source fired and was refused in the same turn; no two Station activations in one turn")
	}
	if twoTurns < 1 {
		t.Fatalf("no Enterprise-D source fired on two turns; the games never crossed a turn boundary")
	}
}

// TestEnterpriseDMutation switches the first-time latch off: dirac fires on
// every charge-counter batch while gorge fires only on the turn's first, so
// the lockstep must diverge. A green mutation means the games never reach a
// second batch.
func TestEnterpriseDMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, fired, _, _, _ := runEnterprise(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation fired on every batch in %d games (%d firings) with no divergence", games, fired)
	}
	t.Logf("mutation diverged in %d of %d games (%d firings)", divs, games, fired)
}
