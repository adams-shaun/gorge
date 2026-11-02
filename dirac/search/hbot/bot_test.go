package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// playSelf plays one vanilla game with the bot answering every decision of
// both seats; check runs before each answer.
func playSelf(t testing.TB, bt *Bot, seed uint64, decks [2]int, check func(s *dirac.State, d *dirac.Decision, i int)) (int8, int) {
	var s dirac.State
	s.Init(v.Table, v.Decks[decks[0]], v.Decks[decks[1]], seed, dirac.Config{})
	var d dirac.Decision
	n := 0
	for ; n < 20000; n++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		i := bt.Pick(&s, &d)
		if check != nil {
			check(&s, &d, i)
		}
		if i < 0 || i >= int(d.N) {
			t.Fatalf("seed %d decision %d (kind %d sub %d): pick %d of %d options", seed, n, d.Kind, d.Sub, i, d.N)
		}
		s.Apply(d.Options[i])
	}
	over, w := s.Terminal()
	if !over {
		t.Fatalf("seed %d: game not over after %d decisions", seed, n)
	}
	return w, n
}

// TestBotPlaysVanillaGames: every answer is one of the options, every game
// ends, the bot is deterministic (the same game twice is the same game).
func TestBotPlaysVanillaGames(t *testing.T) {
	bt := New(NewFacts(v.Table))
	for seed := uint64(1); seed <= 40; seed++ {
		decks := [2]int{int(seed) % len(v.Decks), int(seed/3) % len(v.Decks)}
		w1, n1 := playSelf(t, bt, seed, decks, nil)
		w2, n2 := playSelf(t, bt, seed, decks, nil)
		if w1 != w2 || n1 != n2 {
			t.Fatalf("seed %d: not deterministic (%d/%d vs %d/%d)", seed, w1, n1, w2, n2)
		}
	}
}

// TestPickZeroAllocs: a Pick allocates nothing (the search's hot loop).
func TestPickZeroAllocs(t *testing.T) {
	bt := New(NewFacts(v.Table))
	var states []*dirac.State
	var decs []dirac.Decision
	for seed := uint64(1); seed <= 6; seed++ {
		playSelf(t, bt, seed, [2]int{0, 1}, func(s *dirac.State, d *dirac.Decision, i int) {
			if d.N > 1 && len(states) < 400 {
				c := new(dirac.State)
				c.CopyFrom(s)
				states = append(states, c)
				decs = append(decs, *d)
			}
		})
	}
	if len(states) < 100 {
		t.Fatalf("only %d positions", len(states))
	}
	kinds := map[dirac.DecisionKind]int{}
	for i := range states {
		kinds[decs[i].Kind]++
	}
	allocs := testing.AllocsPerRun(5, func() {
		for i := range states {
			bt.Pick(states[i], &decs[i])
		}
	})
	if allocs != 0 {
		t.Fatalf("Pick allocates: %.1f allocs per %d picks", allocs, len(states))
	}
	t.Logf("%d positions, kinds %v", len(states), kinds)
}

// BenchmarkPick times Pick over positions of vanilla self-play games.
func BenchmarkPick(b *testing.B) {
	bt := New(NewFacts(v.Table))
	var states []*dirac.State
	var decs []dirac.Decision
	for seed := uint64(1); seed <= 6; seed++ {
		playSelf(b, bt, seed, [2]int{0, 1}, func(s *dirac.State, d *dirac.Decision, i int) {
			if d.N > 1 {
				c := new(dirac.State)
				c.CopyFrom(s)
				states = append(states, c)
				decs = append(decs, *d)
			}
		})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		j := i % len(states)
		bt.Pick(states[j], &decs[j])
	}
}
