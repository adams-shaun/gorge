// searchprobe runs the non-production history-conditioned calibration. It does
// not train a model, change a production bot, or claim strength from 500 games.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/internal/testutil"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, progress io.Writer) error {
	fs := flag.NewFlagSet("searchprobe", flag.ContinueOnError)
	fs.SetOutput(progress)
	games := fs.Int("games", 500, "number of baseline games")
	workers := fs.Int("workers", 5, "independent game workers")
	seed := fs.Uint64("seed", 10000, "first actual game seed")
	sampleSeed := fs.Uint64("sample-seed", 54321, "independent fixed sampler seed")
	attempts := fs.Int("attempts", 64, "fixed proposal attempts per root")
	worlds := fs.Int("worlds", 4, "output worlds (this protocol requires four)")
	maxSubmits := fs.Int("max-submits", 5000, "per-attempt and per-rollout submit cap")
	corpus := fs.String("cards", ".cards", "pinned compiled corpus directory")
	outPath := fs.String("out", "", "new JSON output path (will not overwrite)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *games < 1 || *workers < 1 || *attempts < 1 || *worlds != 4 || *maxSubmits < 1 || *outPath == "" {
		return fmt.Errorf("require games/workers/attempts/max-submits > 0, worlds=4, and -out")
	}
	if uint64(*games-1) > ^uint64(0)-*seed {
		return fmt.Errorf("game seed range overflows")
	}
	// Resolve output before expensive work and refuse to overwrite evidence.
	f, err := os.OpenFile(*outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	start := time.Now()
	reg, err := cards.OpenCorpus(*corpus)
	if err != nil {
		return err
	}
	names := []string{"death-n-taxes", "dimir-tempo"}
	decks := make([][]*cards.Card, 2)
	for i, n := range names {
		decks[i], err = testutil.LoadRepoDeck(reg, n)
		if err != nil {
			return err
		}
	}
	setup := searchprobe.PublicGame{Names: names, Decks: decks, Tokens: reg.Tokens}
	loadSeconds := time.Since(start).Seconds()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	jobs := make(chan int)
	results := make(chan searchprobe.ExperimentResult, *workers)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results <- searchprobe.RunExperiment(setup, searchprobe.ExperimentOptions{
					Seed: *seed + uint64(i), SampleSeed: *sampleSeed,
					Attempts: *attempts, Worlds: *worlds, MaxSubmits: *maxSubmits,
					Clock: func() int64 { return time.Now().UnixNano() },
				})
			}
		}()
	}
	go func() {
		for i := 0; i < *games; i++ {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	all := make([]searchprobe.ExperimentResult, *games)
	completed, failed, covered := 0, 0, 0
	for r := range results {
		all[int(r.Seed-*seed)] = r
		completed++
		if r.Error != "" {
			failed++
			fmt.Fprintf(progress, "seed %d: %s\n", r.Seed, r.Error)
		}
		if r.FourWorld.Replays == 4 && r.FourWorld.Fallback == "" {
			covered++
		}
		if completed%10 == 0 || completed == *games {
			fmt.Fprintf(progress, "%d/%d games; sampled coverage %d; errors %d\n", completed, *games, covered, failed)
		}
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	payload := struct {
		Kind                                                                      string
		Games, Workers, GOMAXPROCS, Attempts, Worlds, MaxSubmits, Covered, Errors int
		Seed, SampleSeed                                                          uint64
		LoadSeconds, TotalSeconds                                                 float64
		AllocatedBytes, HeapAllocBytes, HeapSysBytes                              uint64
		Results                                                                   []searchprobe.ExperimentResult
	}{"history-conditioned calibration; not promotion evidence", *games, *workers, runtime.GOMAXPROCS(0), *attempts, *worlds, *maxSubmits, covered, failed, *seed, *sampleSeed, loadSeconds, time.Since(start).Seconds(), after.TotalAlloc - before.TotalAlloc, after.HeapAlloc, after.HeapSys, all}
	if err := json.NewEncoder(f).Encode(payload); err != nil {
		return err
	}
	if failed > 0 {
		return fmt.Errorf("%d experiment invariant errors; see %s", failed, *outPath)
	}
	return nil
}
