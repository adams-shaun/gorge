package dirac

import (
	"testing"

	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

func TestDeterminism(t *testing.T) {
	for g := 0; g < 20; g++ {
		run := func() []Projection {
			s := NewConfig(v.Table, v.Decks[g%4], v.Decks[(g+1)%4], uint64(1000+g), Config{Mulligans: 1})
			pol := NewRandomPolicy(uint64(g))
			var out []Projection
			var d Decision
			for i := 0; i < 5000; i++ {
				s.StepInto(&d)
				var p Projection
				s.Project(&p)
				out = append(out, p)
				if d.Kind == DkNone {
					break
				}
				s.Apply(pol.Pick(&d))
			}
			return out
		}
		a, b := run(), run()
		if len(a) != len(b) {
			t.Fatalf("game %d: lengths %d vs %d", g, len(a), len(b))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("game %d: projection %d differs", g, i)
			}
		}
	}
}

// TestCloneIsAssignment: a copied State continues identically.
func TestCopyFromIsACopy(t *testing.T) {
	s := NewConfig(v.Table, v.Decks[0], v.Decks[1], 77, Config{})
	pol := NewRandomPolicy(5)
	PlayRandom(s, &pol, 40)
	c := new(State)
	c.CopyFrom(s)
	p1, p2 := pol, pol
	PlayRandom(s, &p1, 1<<20)
	PlayRandom(c, &p2, 1<<20)
	var a, b Projection
	s.Project(&a)
	c.Project(&b)
	if a != b || !a.Over {
		t.Fatalf("clone diverged or game not over")
	}
}

func TestZeroAllocsPerGame(t *testing.T) {
	var s State
	g := uint64(0)
	allocs := testing.AllocsPerRun(50, func() {
		g++
		s.Init(v.Table, v.Decks[g%4], v.Decks[(g/4)%4], g, Config{Mulligans: 2})
		pol := NewRandomPolicy(g)
		PlayRandom(&s, &pol, 1<<20)
	})
	if allocs != 0 {
		t.Fatalf("allocs per game = %v, want 0", allocs)
	}
}

// TestSoak plays 10k random games with every deck pairing, bounded steps.
func TestSoak(t *testing.T) {
	var s State
	n := 10000
	if testing.Short() {
		n = 1000
	}
	maxDec, maxTurn := 0, 0
	for g := 0; g < n; g++ {
		s.Init(v.Table, v.Decks[g%4], v.Decks[(g/4)%4], uint64(g)*0x9e37, Config{Mulligans: uint8(g % 4), NoElide: g%5 == 0})
		pol := NewRandomPolicy(uint64(g) ^ 0xabcdef)
		dec, done := PlayRandom(&s, &pol, 50000)
		if !done {
			t.Fatalf("game %d did not end in 50000 decisions (turn %d)", g, s.turn)
		}
		if over, _ := s.Terminal(); !over {
			t.Fatalf("game %d not terminal", g)
		}
		if dec > maxDec {
			maxDec = dec
		}
		if int(s.turn) > maxTurn {
			maxTurn = int(s.turn)
		}
	}
	t.Logf("%d games: max decisions %d, max turns %d", n, maxDec, maxTurn)
}

func BenchmarkGame(b *testing.B) {
	var s State
	b.ReportAllocs()
	dec := 0
	for i := 0; i < b.N; i++ {
		g := uint64(i)
		s.Init(v.Table, v.Decks[g%4], v.Decks[(g/4)%4], g, Config{Mulligans: 2})
		pol := NewRandomPolicy(g)
		n, _ := PlayRandom(&s, &pol, 1<<20)
		dec += n
	}
	b.ReportMetric(float64(dec)/float64(b.N), "decisions/game")
}

// BenchmarkTransition is the cost of one Step+Apply pair (a decision
// answered), measured over whole games, game setup included.
func BenchmarkTransition(b *testing.B) {
	var s State
	var d Decision
	pol := NewRandomPolicy(1)
	g := uint64(0)
	s.Init(v.Table, v.Decks[0], v.Decks[1], g, Config{Mulligans: 2})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.StepInto(&d)
		if d.Kind == DkNone {
			g++
			s.Init(v.Table, v.Decks[g%4], v.Decks[(g/4)%4], g, Config{Mulligans: 2})
			s.StepInto(&d)
		}
		s.Apply(pol.Pick(&d))
	}
}

// BenchmarkTransitionNoElide is BenchmarkTransition posing every decision.
func BenchmarkTransitionNoElide(b *testing.B) {
	var s State
	var d Decision
	pol := NewRandomPolicy(1)
	g := uint64(0)
	cfg := Config{Mulligans: 2, NoElide: true}
	s.Init(v.Table, v.Decks[0], v.Decks[1], g, cfg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.StepInto(&d)
		if d.Kind == DkNone {
			g++
			s.Init(v.Table, v.Decks[g%4], v.Decks[(g/4)%4], g, cfg)
			s.StepInto(&d)
		}
		s.Apply(pol.Pick(&d))
	}
}
