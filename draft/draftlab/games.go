package draftlab

import (
	"fmt"
	"math/rand/v2"
	"sync"
)

// Games plays ms[i] with seed
// rand.New(rand.NewPCG(seed, 0x1AB6A3<<32|uint64(i))).Uint64(), up to
// Options.Workers at once. Results are in input order; on failure the error
// of the lowest failing index is returned. No result depends on the worker
// count or on scheduling.
func (l *Lab) Games(ms []Matchup, seed uint64) ([]GameResult, error) {
	return l.runGames(len(ms), func(i int) (GameResult, error) {
		gs := rand.New(rand.NewPCG(seed, gamesSaltBase<<32|uint64(i))).Uint64()
		return l.PlayGame(ms[i].A, ms[i].B, gs)
	})
}

// runGames runs n independent game jobs with at most Options.Workers at once
// and returns their results in index order. Each job writes only its own
// index of a preallocated slice, so no result depends on scheduling. On
// failure the lowest failing index's error is returned, wrapped with the
// index (errors.Is still reaches the cause).
func (l *Lab) runGames(n int, fn func(i int) (GameResult, error)) ([]GameResult, error) {
	out := make([]GameResult, n)
	errs := make([]error, n)
	workers := l.opts.Workers
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i], errs[i] = fn(i)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("draftlab: game %d: %w", i, err)
		}
	}
	return out, nil
}
