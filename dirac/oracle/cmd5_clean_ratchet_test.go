package oracle

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// cmd5CleanPin is the clean pairs of the cmd5 slice, one pair index a line.
var cmd5CleanPin = filepath.Join("testdata", "cmd5-clean.tsv")

// cmd5Window is how many pairs of the committed cmd5 slice the ratchet plays:
// all of them. Measured 2026-11-12 with
//
//	kfuzz -kernel cmd5 -first-pair 0 -pairs 1500 -seed 1 -keep-going -allow-unverified \
//	  -surface autopay -mulligans 1 -policy heuristic -workers 4 -shrink 0
//
// under GOMAXPROCS=4: 24 s wall and 350 MB RSS for the 1500 games, inside the
// test budget's 45 s.
const cmd5Window = SlicePairs

// cmd5Outcome is one slice game's verdict, the way kfuzz's pool report counts it.
type cmd5Outcome struct {
	clean bool
	class string // why the game is not clean
}

// TestCmd5SliceCleanOnlyGrows plays the first cmd5Window pairs of the committed
// cmd5 slice (1v1 Commander, the Foundations Commander decks) dirac against
// gorge, as kfuzz -kernel cmd5 plays them under the heuristic policy on the
// autopay surface with one mulligan, unverified cards allowed. The pairs that
// play clean are pinned in testdata/cmd5-clean.tsv. The test fails when a
// pinned pair is no longer clean (a divergence, an unmodelled stall, a capacity
// stall or a refusal) and when an unpinned pair becomes clean without the pin
// being raised: the clean set only grows. CMD5_CLEAN_REPIN=1 rewrites the pin
// to the measurement; never edit it by hand.
func TestCmd5SliceCleanOnlyGrows(t *testing.T) {
	set, err := cardset.ByName("cmd5")
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no cmd5 table or corpus (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatalf("set cmd5: %v", err)
	}
	set = cardset.AllowUnverified(set) // kfuzz -allow-unverified: no cmd5 card has evidence yet
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no gorge corpus: %v", err)
	}
	dir, remove, err := ExtractSlice("cmd5")
	if err != nil {
		t.Fatal(err)
	}
	defer remove()
	lines, err := ExplicitPairs(filepath.Join(dir, "pairs.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) < cmd5Window {
		t.Fatalf("%d pairs in the cmd5 slice, want at least %d", len(lines), cmd5Window)
	}
	c, err := OpenCorpus(cardsDir, filepath.Join(dir, "decks"))
	if err != nil {
		t.Fatal(err)
	}
	ps := make([]Pairing, cmd5Window)
	for i := range ps {
		s, err := PairSeed(lines[i], i, SliceSeed)
		if err != nil {
			t.Fatalf("pair %d: %v", i, err)
		}
		ps[i] = Pairing{Index: i, A: lines[i].A, B: lines[i].B, Seed: s}
	}
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}, MaxDecisions: 20000, MaxTurns: 100}
	out := make([]cmd5Outcome, len(ps))
	t0 := time.Now()
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(4, runtime.GOMAXPROCS(0)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				out[i] = playCmd5Pair(set, c, ps[i], rc)
			}
		}()
	}
	for i := range ps {
		next <- i
	}
	close(next)
	wg.Wait()

	var clean []int
	byClass := map[string]int{}
	for i, o := range out {
		if o.clean {
			clean = append(clean, i)
		} else {
			byClass[o.class]++
		}
	}
	t.Logf("cmd5 slice: N=%d pairs in %.1fs: %d clean, %d not clean %v", len(ps), time.Since(t0).Seconds(), len(clean), len(ps)-len(clean), byClass)
	if os.Getenv("CMD5_CLEAN_REPIN") == "1" {
		var b strings.Builder
		fmt.Fprintf(&b, "# cmd5 slice pairs 0..%d that play clean (TestCmd5SliceCleanOnlyGrows); grows only, CMD5_CLEAN_REPIN=1 rewrites it\n", len(ps)-1)
		for _, i := range clean {
			fmt.Fprintf(&b, "%d\n", i)
		}
		if err := os.WriteFile(cmd5CleanPin, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("re-pinned %s: %d clean pairs", cmd5CleanPin, len(clean))
		return
	}
	pin := readCmd5Pin(t, len(ps))
	const repro = "kfuzz -kernel cmd5 -first-pair %d -pairs 1 -seed 1 -allow-unverified -surface autopay -mulligans 1 -policy heuristic -shrink 0 -keep-going -v"
	var raised []string
	for i, o := range out {
		switch {
		case pin[i] && !o.clean:
			t.Errorf("pair %d (%s vs %s, seed %d) was clean and is not: %s; repro: "+repro, i, ps[i].A, ps[i].B, ps[i].Seed, o.class, i)
		case !pin[i] && o.clean:
			raised = append(raised, strconv.Itoa(i))
		}
	}
	if len(raised) > 0 {
		t.Errorf("%d pairs play clean that the pin does not list (%s): raise the pin, from dirac/oracle: CMD5_CLEAN_REPIN=1 go test -run '^TestCmd5SliceCleanOnlyGrows$' .", len(raised), strings.Join(raised, " "))
	}
}

// playCmd5Pair plays one slice pair the way kfuzz's pool path does (runFDN:
// SetMatchupN, SetSide, a gorge-led Lockstep) and classifies it as its report
// does: a capacity or unmodelled stall, a gorge trigger-cap stall, a skipped
// pair and a divergence are all not clean.
func playCmd5Pair(set cardset.Set, c *Corpus, p Pairing, rc RunConfig) cmd5Outcome {
	m, tbl, err := SetMatchupN(set, c, p.Decks(), 1)
	if err != nil {
		return cmd5Outcome{class: "skipped: " + err.Error()}
	}
	k, err := SetSide(set, tbl, rc.Config)(m, p.Seed)
	if err != nil {
		return cmd5Outcome{class: "skipped: " + err.Error()}
	}
	res := Lockstep(m, p.Seed, k, HeuristicPolicy(p.Seed), rc)
	CloseSide(k)
	switch {
	case res.Div != nil:
		if res.Div.What == "kernel" {
			return cmd5Outcome{class: fmt.Sprintf("divergent at %d: kernel %s", res.Div.At, firstLine(res.Div.Detail))}
		}
		return cmd5Outcome{class: fmt.Sprintf("divergent at %d: %s %s", res.Div.At, res.Div.What, res.Div.Field)}
	case res.Stall == GorgeTriggerCapStall:
		return cmd5Outcome{class: "capacity " + res.Stall}
	case strings.HasPrefix(res.Stall, KernelStallPrefix) && res.StallUnmodelled:
		return cmd5Outcome{class: "unmodelled " + res.Stall}
	case strings.HasPrefix(res.Stall, KernelStallPrefix):
		return cmd5Outcome{class: "capacity " + res.Stall}
	}
	return cmd5Outcome{clean: true}
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

// readCmd5Pin reads the pinned clean pair indexes, each below n.
func readCmd5Pin(t *testing.T, n int) map[int]bool {
	t.Helper()
	fh, err := os.Open(cmd5CleanPin)
	if err != nil {
		t.Fatalf("%v (CMD5_CLEAN_REPIN=1 writes it)", err)
	}
	defer fh.Close()
	pin := map[int]bool{}
	var order []int
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		l := sc.Text()
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		i, err := strconv.Atoi(l)
		if err != nil || i < 0 || i >= n {
			t.Fatalf("%s: bad line %q (want a pair index below %d)", cmd5CleanPin, l, n)
		}
		pin[i] = true
		order = append(order, i)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.IsSorted(order) || len(order) != len(pin) {
		t.Fatalf("%s: pair indexes must be sorted and distinct", cmd5CleanPin)
	}
	return pin
}
