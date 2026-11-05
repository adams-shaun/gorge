package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// playVsRandom plays one game: seat searchSeat searches, the other seat
// picks uniformly at random. It returns the winner.
func playVsRandom(sr *Searcher, seed uint64, searchSeat uint8, decks [2]int) (int8, int) {
	var s dirac.State
	s.Init(v.Table, v.Decks[decks[0]], v.Decks[decks[1]], seed, dirac.Config{})
	pol := dirac.NewRandomPolicy(seed ^ 0x1234)
	var d dirac.Decision
	searched := 0
	for i := 0; i < 5000; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if d.Player == searchSeat {
			r := sr.Search(&s, seed^uint64(searchSeat)*0x51)
			if r.Stats.Searched > 0 {
				searched++
			}
			r.Play(&s)
			continue
		}
		s.Apply(pol.Pick(&d))
	}
	_, w := s.Terminal()
	return w, searched
}

func TestSearchDeterministic(t *testing.T) {
	sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 7, dirac.Config{})
	pol := dirac.NewRandomPolicy(3)
	var d dirac.Decision
	checked := 0
	for i := 0; i < 400 && checked < 20; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if d.Player == 0 {
			a := *sr.Search(&s, 99)
			b := *sr.Search(&s, 99)
			if a.Visits != b.Visits || a.Choice != b.Choice || a.Keys != b.Keys {
				t.Fatalf("decision %d: searches differ", i)
			}
			if a.Stats.Searched > 0 {
				checked++
			}
			a.Play(&s)
			continue
		}
		s.Apply(pol.Pick(&d))
	}
	if checked == 0 {
		t.Fatalf("no searched decision")
	}
}

func TestSearchBeatsRandom(t *testing.T) {
	opts := DefaultOptions()
	if testing.Short() {
		opts.Sims = 20
	}
	sr := NewSearcher(opts, Uniform{}, Heuristic{})
	wins, games := 0, 200
	for g := 0; g < games; g++ {
		seat := uint8(g & 1)
		w, _ := playVsRandom(sr, uint64(g)+1, seat, [2]int{g % 5, (g / 5) % 5})
		if w == int8(seat) {
			wins++
		}
	}
	t.Logf("search (stub evaluator, %d sims) vs random: %d/%d", opts.Sims, wins, games)
	if wins*100 < 60*games {
		t.Fatalf("search won %d/%d < 60%%", wins, games)
	}
}

func TestSearchZeroAllocs(t *testing.T) {
	sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 11, dirac.Config{})
	pol := dirac.NewRandomPolicy(5)
	var d dirac.Decision
	for i := 0; i < 60; i++ { // into the game
		s.StepInto(&d)
		s.Apply(pol.Pick(&d))
	}
	for {
		s.StepInto(&d)
		if d.Player == 0 && d.Kind == dirac.DkPriority {
			break
		}
		s.Apply(pol.Pick(&d))
	}
	allocs := testing.AllocsPerRun(5, func() { sr.Search(&s, 1) })
	if allocs != 0 {
		t.Fatalf("allocs per search = %v", allocs)
	}
}

// TestNodeCacheIsExact: over fixed worlds the node cache changes only the
// cost counters, never the result (azmcts TestNodeCacheIsExact).
func TestNodeCacheIsExact(t *testing.T) {
	o := DefaultOptions()
	o.CachedWorlds = 4
	plain := NewSearcher(o, Uniform{}, Heuristic{})
	o.NodeCache = 64
	cached := NewSearcher(o, Uniform{}, Heuristic{})
	rs := roots(40)
	resumes := 0
	for i := range rs {
		a := *plain.Search(rs[i], uint64(i))
		b := *cached.Search(rs[i], uint64(i))
		if a.Visits != b.Visits || a.Choice != b.Choice || a.Q != b.Q || a.Avail != b.Avail {
			t.Fatalf("root %d: cached search differs: %v vs %v", i, a.Visits[:a.N], b.Visits[:b.N])
		}
		resumes += b.Stats.NodeResumes
	}
	if resumes == 0 {
		t.Fatalf("the node cache never resumed")
	}
}

// TestClosedListDeal: a closed-list world deals the opponent's hidden cards
// from the belief corpus, not from its real list.
func TestClosedListDeal(t *testing.T) {
	o := DefaultOptions()
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	sr.Belief = NewBelief(v.Table, [][]dirac.CardID{v.Decks[2]})
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 4, dirac.Config{})
	var d dirac.Decision
	s.StepInto(&d)
	sr.seat = 0
	sr.redeal(&s, 1, 2)
	in := map[dirac.CardID]bool{}
	for _, c := range v.Decks[2] {
		in[c] = true
	}
	for o := 0; o < s.NumObjs(); o++ {
		ob := dirac.Obj(o)
		if s.ObjOwner(ob) == 1 && s.Hidden(ob, 0) && !in[s.ObjCard(ob)] {
			t.Fatalf("hidden card %d of the opponent is not from the belief list", s.ObjCard(ob))
		}
	}
}

func TestHeuristicLeafSearchBeatsRandom(t *testing.T) {
	opts := DefaultOptions()
	opts.Sims = 50
	sr := NewSearcher(opts, HeuristicLeaf{}, Heuristic{})
	wins, games := 0, 100
	for g := 0; g < games; g++ {
		seat := uint8(g & 1)
		w, _ := playVsRandom(sr, uint64(g)+7, seat, [2]int{g % 5, (g / 5) % 5})
		if w == int8(seat) {
			wins++
		}
	}
	t.Logf("search (heuristic leaf, %d sims) vs random: %d/%d", opts.Sims, wins, games)
	if wins*100 < 60*games {
		t.Fatalf("search won %d/%d", wins, games)
	}
}

// Under gorge's either-or shape (Config.GorgeAltAdd) an unpayable part
// undoes the cast: the search prunes that edge as a no-op instead of
// looping on it, and self-play over the decks holding Eaten Alive ends.
func TestNoOpEdgesPruned(t *testing.T) {
	opts := DefaultOptions()
	opts.Sims = 40
	noops := 0
	// Swamps, Forests, Eaten Alive and bears to sacrifice
	var deck []dirac.CardID
	for i := 0; i < 40; i++ {
		switch {
		case i < 12:
			deck = append(deck, v.Swamp)
		case i < 18:
			deck = append(deck, v.Forest)
		case i < 30:
			deck = append(deck, v.EatenAlive)
		default:
			deck = append(deck, v.GrizzlyBears)
		}
	}
	for g := uint64(0); g < 12; g++ {
		var srs [2]*Searcher
		for i := range srs {
			srs[i] = NewSearcher(opts, Uniform{}, Heuristic{})
		}
		var s dirac.State
		s.Init(v.Table, deck, v.Decks[0], g, dirac.Config{GorgeAltAdd: true})
		var d dirac.Decision
		over := false
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				over = true
				break
			}
			r := srs[d.Player].Search(&s, g*13+uint64(i))
			noops += r.Stats.NoOps
			r.Play(&s)
		}
		if !over {
			t.Fatalf("game %d did not end in 3000 decisions", g)
		}
	}
	t.Logf("%d no-op edges pruned", noops)
	if noops == 0 {
		t.Fatal("no no-op edge met: the test no longer poses an unpayable part")
	}
}
