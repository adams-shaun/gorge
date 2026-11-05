package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"
)

// bootstrapResamples is how many pod resamples the summary's interval takes.
const bootstrapResamples = 1000

// gameResult is one decided cross-label game: the pod it came from, the two
// decks' labels in seat order and the winning seat (0 or 1).
type gameResult struct {
	Pod    int
	A, B   string
	Winner int
}

// labelCI is one label's win rate over its games and the 95% interval of that
// rate when pods are resampled with replacement.
type labelCI struct {
	Label        string
	Rate, Lo, Hi float64
	Games        int
}

// bootstrap is each label's win rate with a 95% percentile interval from
// resamples pod bootstraps (a pod's games stay together: its decks share
// packs). The resampling stream is seeded from seed alone. Labels are ordered
// by rate, highest first, then by name.
func bootstrap(games []gameResult, seed uint64, resamples int) []labelCI {
	byPod := map[int][]gameResult{}
	for _, g := range games {
		byPod[g.Pod] = append(byPod[g.Pod], g)
	}
	pods := make([]int, 0, len(byPod))
	for p := range byPod {
		pods = append(pods, p)
	}
	slices.Sort(pods)
	labels := map[string]bool{}
	for _, g := range games {
		labels[g.A], labels[g.B] = true, true
	}
	names := make([]string, 0, len(labels))
	for l := range labels {
		names = append(names, l)
	}
	slices.Sort(names)

	tally := func(sample []int) (rate map[string]float64, n map[string]int) {
		won, n := map[string]int{}, map[string]int{}
		for _, p := range sample {
			for _, g := range byPod[p] {
				n[g.A]++
				n[g.B]++
				if g.Winner == 0 {
					won[g.A]++
				} else {
					won[g.B]++
				}
			}
		}
		rate = map[string]float64{}
		for l, k := range n {
			rate[l] = float64(won[l]) / float64(k)
		}
		return rate, n
	}
	base, n := tally(pods)
	draws := make(map[string][]float64, len(names))
	r := rand.New(rand.NewPCG(seed, 0xb0075742))
	sample := make([]int, len(pods))
	for range resamples {
		for i := range sample {
			sample[i] = pods[r.IntN(len(pods))]
		}
		rate, _ := tally(sample)
		for _, l := range names {
			if v, ok := rate[l]; ok {
				draws[l] = append(draws[l], v)
			}
		}
	}
	out := make([]labelCI, 0, len(names))
	for _, l := range names {
		d := draws[l]
		slices.Sort(d)
		ci := labelCI{Label: l, Rate: base[l], Games: n[l]}
		if len(d) > 0 {
			ci.Lo, ci.Hi = d[int(0.025*float64(len(d)))], d[min(len(d)-1, int(0.975*float64(len(d))))]
		}
		out = append(out, ci)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Rate > out[j].Rate })
	return out
}

// readResults reads every finished chunk under out: decks.jsonl maps a deck id
// to its pod and label, games.jsonl gives the games. Draws, stalls and errors
// (winner neither 0 nor 1) are left out.
func readResults(out string) ([]gameResult, error) {
	dirs, err := filepath.Glob(filepath.Join(out, "chunk-*"))
	if err != nil {
		return nil, err
	}
	slices.Sort(dirs)
	var gs []gameResult
	for _, dir := range dirs {
		if !chunkDone(dir) {
			continue
		}
		type deckRow struct {
			ID    string `json:"id"`
			Pod   int    `json:"pod"`
			Label string `json:"label"`
		}
		decks := map[string]deckRow{}
		if err := eachLine(filepath.Join(dir, "decks.jsonl"), func(b []byte) error {
			var d deckRow
			if err := json.Unmarshal(b, &d); err != nil {
				return err
			}
			decks[d.ID] = d
			return nil
		}); err != nil {
			return nil, err
		}
		if err := eachLine(filepath.Join(dir, "games.jsonl"), func(b []byte) error {
			var g struct {
				Decks  [2]string `json:"decks"`
				Winner int       `json:"winner"`
			}
			if err := json.Unmarshal(b, &g); err != nil {
				return err
			}
			if g.Winner != 0 && g.Winner != 1 {
				return nil
			}
			a, ok := decks[g.Decks[0]]
			b2, ok2 := decks[g.Decks[1]]
			if !ok || !ok2 {
				return fmt.Errorf("game between %s and %s: deck not in decks.jsonl", g.Decks[0], g.Decks[1])
			}
			gs = append(gs, gameResult{Pod: a.Pod, A: a.Label, B: b2.Label, Winner: g.Winner})
			return nil
		}); err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
	}
	return gs, nil
}

func eachLine(path string, f func([]byte) error) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		if err := f(sc.Bytes()); err != nil {
			return err
		}
	}
	return sc.Err()
}

func printSummary(w io.Writer, games []gameResult, cis []labelCI) {
	pods := map[int]bool{}
	for _, g := range games {
		pods[g.Pod] = true
	}
	fmt.Fprintf(w, "draftres: %d pods, %d decided games, 95%% CI by pod bootstrap (%d resamples)\n", len(pods), len(games), bootstrapResamples)
	for _, c := range cis {
		fmt.Fprintf(w, "  %-40s %5.1f%%  [%5.1f, %5.1f]  n=%d\n", c.Label, 100*c.Rate, 100*c.Lo, 100*c.Hi, c.Games)
	}
}
