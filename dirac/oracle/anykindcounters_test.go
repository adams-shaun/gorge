package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Any-kind counter replacements: Loading Zone ("twice that
// many of each of those kinds", no ValidCounterType$, ValidCard$ creature /
// Spacecraft / Planet you control) and Doc Samson, Super Psychiatrist ("that
// many plus one of each of those kinds", ValidSource$ You as the putter,
// ValidObject$ a permanent you control), beside counter makers of several
// kinds (+1/+1, flying, lifelink, charge, supply).
//
// The "can't become untapped" aura Frozen in Ice is anyuntapblocked_test.go
// Blossombind's CantPutCounter static is
// cantputcounterrepl_test.go. "Lands you control enter untapped" (The
// Wandering Minstrel, Spelunking) and gorge's tap-vs-untap order ask
// (replacement_updated.go) are entersuntappedorder_test.go.

var krepl3Decks = [][2][]cardset.Entry{
	{ // M1: Loading Zone with Spacecraft, Wingspan Mentor and the supply maker against removal
		{
			{Name: "Loading Zone", Count: 4}, {Name: "Atmospheric Greenhouse", Count: 4}, {Name: "Stocking the Pantry", Count: 3},
			{Name: "Wingspan Mentor", Count: 4}, {Name: "Grizzly Bears", Count: 5}, {Name: "Adagia, Windswept Bastion", Count: 2},
			{Name: "Forest", Count: 9}, {Name: "Island", Count: 9},
		},
		{
			{Name: "Duskfang Mentor", Count: 4}, {Name: "Void Beckoner", Count: 3}, {Name: "Walking Corpse", Count: 4},
			{Name: "Gray Ogre", Count: 3}, {Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 3},
			{Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 7},
		},
	},
	{ // M2: Doc Samson with the same counter makers and Jace Beleren (a loyalty cost under the putter gate) against Ajani, Caller of the Pride (the opponent's +1/+1 counter on any creature, the Samson player's included) and removal
		{
			{Name: "Doc Samson, Super Psychiatrist", Count: 4}, {Name: "Atmospheric Greenhouse", Count: 3}, {Name: "Stocking the Pantry", Count: 3},
			{Name: "Wingspan Mentor", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Adagia, Windswept Bastion", Count: 1},
			{Name: "Jace Beleren", Count: 3}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
		},
		{
			{Name: "Duskfang Mentor", Count: 4}, {Name: "Void Beckoner", Count: 3}, {Name: "Walking Corpse", Count: 4},
			{Name: "Ajani, Caller of the Pride", Count: 3}, {Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 3},
			{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 7},
		},
	},
}

// krepl3Stats is what the gorge-side event log says the replacements decided:
// counters placed on a permanent whose controller has the source in play.
type krepl3Stats struct {
	lzCtr     int // a counter placement under Loading Zone
	lzDoubled int // ... of two or more +1/+1 counters at once (Greenhouse's one, doubled)
	lzCharge  int // ... a Spacecraft's charge counters (station)
	lzNonP1   int // ... of a kind other than +1/+1 (flying)
	dsCtr     int // a counter placement under Doc Samson
	dsNonP1   int // ... of a kind other than +1/+1 (flying, supply)
	dsPlus    int // ... of two +1/+1 counters at once (Greenhouse's one, plus one)
	dsOpp     int // ... a lone +1/+1 counter on a creature of the Doc Samson player's: put by the opponent (Ajani), so ValidSource$ You leaves it unreplaced
	dsLoyal   int // ... three loyalty counters at once on a planeswalker: Jace's [+2] cost plus one (the cost path)
}

type krepl3Tally struct{ evAt int }

func (p *krepl3Tally) scan(ref *GorgeSide, st *krepl3Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.CounterChange || e.Amount <= 0 {
			continue
		}
		o := g.Obj(e.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		p1 := e.Counter == "P1P1"
		if battlefieldHasName(g, o.Controller, "Loading Zone") {
			st.lzCtr++
			if p1 && e.Amount >= 2 {
				st.lzDoubled++
			}
			if e.Counter == "CHARGE" {
				st.lzCharge++
			}
			if !p1 {
				st.lzNonP1++
			}
		}
		if battlefieldHasName(g, o.Controller, "Doc Samson, Super Psychiatrist") {
			st.dsCtr++
			if !p1 {
				st.dsNonP1++
			}
			if p1 && e.Amount >= 2 {
				st.dsPlus++
			}
			if p1 && e.Amount == 1 {
				st.dsOpp++
			}
			if e.Counter == "LOYALTY" && e.Amount == 3 {
				st.dsLoyal++
			}
		}
	}
	p.evAt = len(evs)
}

// krepl3Policy plays at random among casting, activating and playing lands
// (so the counter makers are reached) and attacks whenever it can.
func krepl3Policy(seed uint64, st *krepl3Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x7265706c330a)) // "repl3"
	var buf []Canon
	var tl krepl3Tally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		switch c.D.Kind {
		case DAttackers:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		case DPriority:
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

// runAnykindcounters plays the constructed matchups, each in both seatings, and
// returns the games, divergences, stalls and the replacement counts. mutate
// inverts the kernel's any-kind match (the mutation must diverge).
func runAnykindcounters(t *testing.T, seeds int, mutate bool) (games int, divs, stalls [2]int, st [2]krepl3Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("anykindcounters")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugReplShapes3Flip
	dirac.DebugReplShapes3Flip = mutate
	defer func() { dirac.DebugReplShapes3Flip = old }()
	for mi, md0 := range krepl3Decks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
			}
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
			for seed := uint64(1); seed <= uint64(seeds); seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"krepl3-a", "krepl3-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				g, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				res := LockstepLed(m, seed, g, k, krepl3Policy(seed, &st[mi]), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls[mi]++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
					}
				}
				if d := res.Div; d != nil {
					divs[mi]++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestAnyKindCounterReplLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 per matchup
	games, divs, stalls, st := runAnykindcounters(t, 96, false)
	t.Logf("%d games, %v divergences, %v stalls", games, divs, stalls)
	t.Logf("M1 %+v", st[0])
	t.Logf("M2 %+v", st[1])
	if divs != [2]int{} || stalls != [2]int{} {
		t.Fatalf("%v divergences, %v stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"counters under Loading Zone", st[0].lzCtr}, {"doubled +1/+1 placements under Loading Zone", st[0].lzDoubled},
		{"charge counters under Loading Zone", st[0].lzCharge}, {"non-+1/+1 counters under Loading Zone", st[0].lzNonP1},
		{"counters under Doc Samson", st[1].dsCtr}, {"non-+1/+1 counters under Doc Samson", st[1].dsNonP1},
		{"plus-one +1/+1 placements under Doc Samson", st[1].dsPlus},
		{"opponent-put (unreplaced) +1/+1 counters on Doc Samson's creatures", st[1].dsOpp},
		{"loyalty-cost placements plus one under Doc Samson", st[1].dsLoyal},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestAnyKindCounterReplMutation inverts the kernel's any-kind match (Loading Zone then
// doubles nothing, Doc Samson's putter gate reads the wrong player): each
// matchup must diverge.
func TestAnyKindCounterReplMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runAnykindcounters(t, 32, true)
	for mi, d := range divs {
		if d == 0 {
			t.Errorf("matchup %d: mutation inverted the any-kind match with no divergence", mi+1)
		}
	}
	t.Logf("mutation diverged %v in %d games", divs, games)
}
