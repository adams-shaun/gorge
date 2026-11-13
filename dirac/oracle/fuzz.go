package oracle

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Pairing is one deck pair and its game seed.
type Pairing struct {
	Index int
	A, B  string
	Seed  uint64
	// More is a pod's further decks, seats 2 and up (PodAt); empty for a pair.
	More [MaxSeats - 2]string
}

// Decks is the pairing's decks in seat order.
func (p Pairing) Decks() []string {
	out := []string{p.A, p.B}
	for _, n := range p.More {
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

// PodsFrom is PairingsFrom for games of seats seats: at two seats it is
// PairingsFrom, and past two each pod is PodAt.
func PodsFrom(pool []PoolDeck, first, n int, seed uint64, seats int) []Pairing {
	if seats <= 2 {
		return PairingsFrom(pool, first, n, seed)
	}
	out := make([]Pairing, 0, n)
	if len(pool) < seats {
		return out
	}
	for p := first; p < first+n; p++ {
		out = append(out, PodAt(pool, p, seed, seats))
	}
	return out
}

// PodAt is pod p of seats distinct decks from pool (len(pool) >= seats > 2):
// a pure function of (seed, p), drawn from the generator PairingAt uses, the
// seats' decks in seat order, then the seed.
func PodAt(pool []PoolDeck, p int, seed uint64, seats int) Pairing {
	r := rand.New(rand.NewPCG(seed, uint64(p)))
	idx := make([]int, len(pool))
	for i := range idx {
		idx[i] = i
	}
	names := make([]string, seats)
	for s := range names {
		k := s + r.IntN(len(idx)-s)
		idx[s], idx[k] = idx[k], idx[s]
		names[s] = pool[idx[s]].Name
	}
	out := Pairing{Index: p, A: names[0], B: names[1], Seed: r.Uint64()}
	copy(out.More[:], names[2:])
	return out
}

// Pairings draws n deck pairs from pool exactly as gorge self-play draws its
// jobs: pair p's decks and seed are a pure function of (seed, p).
func Pairings(pool []PoolDeck, n int, seed uint64) []Pairing {
	return PairingsFrom(pool, 0, n, seed)
}

// PairingsFrom is Pairings starting at pair index first (-first-pair).
func PairingsFrom(pool []PoolDeck, first, n int, seed uint64) []Pairing {
	out := make([]Pairing, 0, n)
	if len(pool) < 2 {
		return out
	}
	for p := first; p < first+n; p++ {
		out = append(out, PairingAt(pool, p, seed))
	}
	return out
}

// PairingAt is pair p of PairingsFrom's sequence on its own (pool must hold
// at least two decks): a caller that needs one pairing at a time (a self-play runner's
// schedule) makes it without holding the whole list.
func PairingAt(pool []PoolDeck, p int, seed uint64) Pairing {
	r := rand.New(rand.NewPCG(seed, uint64(p)))
	i := r.IntN(len(pool))
	j := r.IntN(len(pool) - 1)
	if j >= i {
		j++
	}
	return Pairing{Index: p, A: pool[i].Name, B: pool[j].Name, Seed: r.Uint64()}
}

// FuzzConfig is one fuzz run.
type FuzzConfig struct {
	Run       RunConfig
	Mulligans int
	// Policy builds the policy of one game from its seed (nil: RandomPolicy).
	Policy func(seed uint64) Policy
	// StopAtFirst stops the run at the first divergence (in pair order).
	StopAtFirst bool
	// Shrink shrinks the first divergence's repro with this many reruns
	// (0: no shrink).
	Shrink int
	// Workers plays games in parallel (0 or 1: serially).
	Workers int
	// Coverage collects each clean game's coverage (WithCoverage) into
	// FuzzSummary.Coverage, merged in pair order.
	Coverage bool
	// GameTimeout is the per-game watchdog: a game with no result after this long,
	// twice (one retry absorbs a host stall), is a hang and the worker moves on.
	// Its goroutine cannot be stopped, so it leaks until the process exits; with
	// StopOnHang the run stops feeding new pairs and the caller re-executes the
	// rest elsewhere. 0: no watchdog.
	GameTimeout time.Duration
	// StopOnHang stops the feed at the first recorded hang, as StopAtFirst stops
	// it at the first divergence: the played pairs are the prefix before
	// FuzzSummary.SkipAt (Games + Errors when nothing was skipped; both count
	// every attempted pair, a hang counts in Games), and the pairs from SkipAt on
	// belong to the caller's next attempt, not to this leaking process. It also
	// stops the feed at a StopAtFirst divergence: the jobs a worker receives
	// after that stop are then skipped too, which changes a divergence run's
	// games count slightly (by the pairs a worker received past the stop).
	StopOnHang bool
}

// FuzzSummary is a fuzz run's totals.
type FuzzSummary struct {
	Games, Diverged, Over, Stalled, Errors int
	Decisions                              int
	Hung                                   int       // counted in Games too
	Hangs                                  []Pairing // the hung pairs, in pair order
	Wall                                   time.Duration
	ByClass                                map[string]int // divergences by class ("projection Cards[].Zone")
	First                                  *Repro         // the first divergence's repro (shrunk when asked)
	FirstPair                              *Pairing
	Stats                                  Stats
	Coverage                               *Coverage // FuzzConfig.Coverage's merge (nil: off)
	// SkipAt is the lowest pair index a worker received after the feed stopped
	// (StopOnHang; len(pairs) when nothing was skipped): the played pairs are
	// the prefix before it, so the caller's next attempt starts at SkipAt and
	// no pair is played twice or lost between attempts.
	SkipAt int
}

// Fuzz plays every pairing in lockstep against the side factory mk.
// progress, when non-nil, is called after each game (serialised).
func Fuzz(c *Corpus, pairs []Pairing, mk SideFactory, fc FuzzConfig, progress func(i int, p Pairing, r *Result)) FuzzSummary {
	t0 := time.Now()
	s := FuzzSummary{ByClass: map[string]int{}}
	pol := fc.Policy
	if pol == nil {
		pol = RandomPolicy
	}
	workers := fc.Workers
	if workers < 1 {
		workers = 1
	}
	type done struct {
		i    int
		m    *Matchup
		res  Result
		err  error
		cov  *Coverage
		hung bool
	}
	var (
		mu      sync.Mutex
		stopped atomic.Bool
		firstAt = -1
		first   done
		covs    []*Coverage
		hung    []int
		skipAt  = len(pairs) // the lowest index a worker received after the feed stopped
	)
	if fc.Coverage {
		covs = make([]*Coverage, len(pairs))
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if fc.StopOnHang && stopped.Load() {
					// Received after the feed stopped: the caller's next attempt
					// plays it. Record the lowest such index, so the played pairs
					// stay an exact prefix (a worker can be descheduled between
					// receiving a job and checking the stop, below a played one).
					mu.Lock()
					if i < skipAt {
						skipAt = i
					}
					mu.Unlock()
					continue
				}
				p := pairs[i]
				// The game owns everything it writes (d, its coverage), so one the
				// watchdog gave up on touches nothing shared.
				play := func() (d done) {
					defer func() {
						if r := recover(); r != nil {
							d.err = fmt.Errorf("panic: %v", r)
						}
					}()
					d.i = i
					d.m, d.err = c.MatchupN(p.Decks(), fc.Mulligans)
					if d.err == nil {
						var k Side
						if k, d.err = mk(d.m, p.Seed); d.err == nil {
							rc, gp := fc.Run, pol(p.Seed)
							var cov *Coverage
							if covs != nil {
								rc = WithCoverage(rc, d.m, &cov)
							}
							d.res = Lockstep(d.m, p.Seed, k, gp, rc)
							if cov != nil && d.res.Clean() {
								cov.Games, cov.Decisions, cov.Fallbacks = 1, d.res.Decisions, PolicyFallbacks(gp)
								d.cov = cov
							}
						}
					}
					return d
				}
				d := WithinTimeoutRetry(fc.GameTimeout, true, play, done{i: i, hung: true})
				if d.cov != nil {
					covs[i] = d.cov
				}
				mu.Lock()
				if d.hung {
					s.Games++
					s.Hung++
					s.ByClass["hang"]++
					hung = append(hung, i)
					if fc.StopOnHang {
						stopped.Store(true)
					}
				} else if d.err != nil {
					s.Errors++
				} else {
					s.Games++
					s.Decisions += d.res.Decisions
					s.Stats.Add(d.res.Stats)
					switch {
					case d.res.Div != nil:
						s.Diverged++
						s.ByClass[d.res.Div.fieldClass()]++
						if firstAt < 0 || i < firstAt {
							firstAt, first = i, d
						}
						if fc.StopAtFirst {
							stopped.Store(true)
						}
					case d.res.Over:
						s.Over++
					default:
						s.Stalled++
					}
					if progress != nil {
						progress(i, p, &d.res)
					}
				}
				mu.Unlock()
			}
		}()
	}
	for i := range pairs {
		if stopped.Load() {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	s.SkipAt = skipAt
	if covs != nil {
		s.Coverage = NewCoverage()
		for _, cv := range covs {
			s.Coverage.Merge(cv)
		}
	}
	sort.Ints(hung)
	for _, i := range hung {
		s.Hangs = append(s.Hangs, pairs[i])
	}
	if firstAt >= 0 {
		p := pairs[firstAt]
		rep := NewRepro(first.m, p.Seed, fc.Run, &first.res)
		if fc.Shrink > 0 {
			rep = Shrink(c, rep, mk, fc.Shrink)
		}
		s.First, s.FirstPair = rep, &p
	}
	s.Wall = time.Since(t0)
	return s
}

// WithinTimeoutRetry runs play on its own goroutine and returns its result, or
// hung when it has none after timeout. With retry set, a first attempt that times
// out is played once more under a fresh watchdog of the same timeout: a host
// stall (swap, load) past a wall-clock watchdog is not a property of the game, so
// hung is returned only when both attempts time out. Without retry (a run whose
// shared diagnostic counters a stalled attempt would keep writing) one attempt
// decides. A timed-out goroutine cannot be stopped, so each stalled attempt leaks
// until it returns; play must touch only its own game so a stalled attempt cannot
// disturb its retry. A zero timeout waits for the first attempt and never retries.
func WithinTimeoutRetry[T any](timeout time.Duration, retry bool, play func() T, hung T) T {
	if timeout <= 0 {
		return play()
	}
	attempts := 1
	if retry {
		attempts = 2
	}
	for attempt := 0; attempt < attempts; attempt++ {
		ch := make(chan T, 1)
		go func() { ch <- play() }()
		t := time.NewTimer(timeout)
		select {
		case r := <-ch:
			t.Stop()
			return r
		case <-t.C:
		}
	}
	return hung
}
