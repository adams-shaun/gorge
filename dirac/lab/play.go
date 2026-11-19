package lab

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

// Result is how one game ended.
type Result struct {
	Pair      int      // index of the game's seed in the Batch
	Decks     []string // deck names by seat
	Seed      uint64
	Outcome   Outcome
	Winner    int // winning seat; -1 when Outcome is not Won
	Turns     int
	Decisions int    // decisions answered in the game
	Stall     string // the stall reason when Stalled, else ""
	Class     string // capacity or unmodelled when Stalled
	Err       string // the panic or bad answer when Failed
}

// Play plays g to its end, seat i answering with pols[i], for at most maxSteps
// decisions (counted from the call). It returns how the game ended; a game that
// stalls or runs out of steps is Stalled or Truncated, never Drawn. A policy
// that panics or answers with an action not offered ends the game Failed.
func Play(g *Game, pols []Policy, maxSteps int) (res Result) {
	res = Result{Seed: g.Seed(), Decks: g.su.names, Winner: -1}
	defer func() {
		if e := recover(); e != nil {
			res.Outcome, res.Err = Failed, fmt.Sprint(e)
			res.Turns, res.Decisions = g.Turn(), g.Steps()
		}
	}()
	if len(pols) != g.Seats() {
		panic(fmt.Sprintf("lab: %d policies for %d seats", len(pols), g.Seats()))
	}
	for seat, p := range pols {
		if st, ok := p.(Starter); ok {
			st.Start(g, seat)
		}
	}
	for n := 0; n < maxSteps && !g.Done(); n++ {
		d := g.Decision()
		a := pols[d.Player].Pick(&g.s, d)
		if err := g.Apply(a); err != nil {
			panic(fmt.Sprintf("seat %d: %v", d.Player, err))
		}
	}
	res.Outcome = g.Outcome()
	if res.Outcome == Running {
		res.Outcome = Truncated
	}
	res.Winner = g.Winner()
	res.Stall, res.Class = g.StallReason()
	res.Turns, res.Decisions = g.Turn(), g.Steps()
	return res
}

// Summary tallies a batch. Games = sum of Wins, Draws, Stalls, Truncated and Failed.
// Win rates are wins over all Games, so a stalled or truncated game counts
// against every seat, never as a draw.
type Summary struct {
	Games     int
	Wins      []int // by seat
	Draws     int
	Stalls    int
	Truncated int
	Failed    int
	Turns     int64
	Decisions int64
	StallsBy  map[string]int // stall reason -> count
}

// Rate is a proportion with its 95% Wilson score interval.
type Rate struct{ P, Lo, Hi float64 }

// Wilson is the 95% Wilson score interval of k successes in n trials.
func Wilson(k, n int) Rate {
	if n == 0 {
		return Rate{}
	}
	const z = 1.959963984540054
	N, p := float64(n), float64(k)/float64(n)
	den := 1 + z*z/N
	mid := (p + z*z/(2*N)) / den
	half := z * math.Sqrt(p*(1-p)/N+z*z/(4*N*N)) / den
	return Rate{p, math.Max(0, mid-half), math.Min(1, mid+half)}
}

// WinRate is seat's win rate over all games, with its interval.
func (s *Summary) WinRate(seat int) Rate { return Wilson(s.Wins[seat], s.Games) }

// Summarize tallies results of a game with seats seats.
func Summarize(seats int, rs []Result) Summary {
	s := Summary{Wins: make([]int, seats), StallsBy: map[string]int{}}
	for i := range rs {
		r := &rs[i]
		s.Games++
		s.Turns += int64(r.Turns)
		s.Decisions += int64(r.Decisions)
		switch r.Outcome {
		case Won:
			s.Wins[r.Winner]++
		case Drawn:
			s.Draws++
		case Stalled:
			s.Stalls++
			s.StallsBy[r.Stall]++
		case Truncated:
			s.Truncated++
		default:
			s.Failed++
		}
	}
	return s
}

// Batch plays one game per seed on cfg's decks, seat i answered by pols[i], at
// most maxSteps decisions each, on workers goroutines. Results are in seeds
// order whatever workers is. Each worker plays its own clone of the policies, so
// with more than one worker every policy must be a Cloner. A game that fails is
// a Failed result, not an error.
func Batch(cfg Config, pols []Policy, seeds []uint64, workers, maxSteps int) ([]Result, Summary, error) {
	su, err := Open(cfg)
	if err != nil {
		return nil, Summary{}, err
	}
	defer su.Close()
	if workers < 1 {
		workers = 1
	}
	if workers > len(seeds) {
		workers = max(1, len(seeds))
	}
	teams, err := clones(pols, workers)
	if err != nil {
		return nil, Summary{}, err
	}
	rs := make([]Result, len(seeds))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				rs[i] = Play(su.Game(seeds[i]), teams[w], maxSteps)
				rs[i].Pair = i
			}
		}()
	}
	for i := range seeds {
		next <- i
	}
	close(next)
	wg.Wait()
	return rs, Summarize(su.Seats(), rs), nil
}

// clones is one team of policies per worker: the given one first.
func clones(pols []Policy, workers int) ([][]Policy, error) {
	teams := [][]Policy{pols}
	for w := 1; w < workers; w++ {
		team := make([]Policy, len(pols))
		for i, p := range pols {
			c, ok := p.(Cloner)
			if !ok {
				return nil, errors.New("lab: policy " + fmt.Sprintf("%T", p) + " is not a Cloner: play it with one worker")
			}
			team[i] = c.Clone()
		}
		teams = append(teams, team)
	}
	return teams, nil
}
