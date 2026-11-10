// Command dplay plays dirac games with no other engine in the loop and reports them: one
// JSON line per game and a summary. Every game runs under dirac.LockstepConfig on the table
// a lockstep run of the same pair plays, so what it measures is what dirac/oracle verifies
// against gorge.
//
//	go run -trimpath ./dirac/cmd/dplay -pairs 20 -a hbot -b random
//
// dirac/docs/running.md documents the flags and the output.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/deck"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

type options struct {
	cards, set, decks, pool, split string
	deckA, deckB, pairsFile        string
	a, b, mullPolicy, config       string
	out, summary                   string
	pairs, firstPair, mulligans    int
	maxTurns, maxIntents, workers  int
	seed                           uint64
	skipRefused                    bool
	sampleReplay                   float64
	wall                           time.Duration
	explicit                       map[string]bool
}

func (o *options) flags(errs io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("dplay", flag.ContinueOnError)
	fs.SetOutput(errs)
	fs.StringVar(&o.cards, "cards", "", "gorge corpus directory (default: $GORGE_CARDS, else .cards in or above the working directory)")
	fs.StringVar(&o.set, "set", "fdn", "the card set to play: a deck pool by name, or a synthetic pool (std-trk, tail-NN)")
	fs.StringVar(&o.decks, "decks", "", "deck directory (default: the set's pool, under $GORGE_POOLS or the module's .pools, else its committed slice)")
	fs.StringVar(&o.pool, "pool", "", "pool.tsv (default: <decks>/../pool.tsv)")
	fs.StringVar(&o.split, "split", "all", "pool split the decks come from: train, eval or all")
	fs.StringVar(&o.deckA, "deck-a", "", "deck JSON file for seat A, with -deck-b, instead of a pool")
	fs.StringVar(&o.deckB, "deck-b", "", "deck JSON file for seat B")
	fs.IntVar(&o.pairs, "pairs", 100, "deck pairs, one game each")
	fs.IntVar(&o.firstPair, "first-pair", 0, "index of the first pair")
	fs.Uint64Var(&o.seed, "seed", 1, "base seed: pairs and game seeds derive from it, as in kfuzz")
	fs.StringVar(&o.pairsFile, "pairs-file", "", "play these pairs: deckA<TAB>deckB[<TAB>seed] per line; -first-pair skips lines and -pairs, when given, caps them")
	fs.StringVar(&o.a, "a", "hbot", "seat A's player, who plays the pair's first deck: random, first, hbot or sbt")
	fs.StringVar(&o.b, "b", "random", "seat B's player")
	fs.IntVar(&o.mulligans, "mulligans", 0, "London mulligans each player may take (0 skips the round)")
	fs.StringVar(&o.mullPolicy, "mulligan-policy", "coin", "how every seat mulligans: coin, lands, never, constructed or forge")
	fs.IntVar(&o.maxTurns, "max-turns", 100, "turn cap; a capped game is a stall")
	fs.IntVar(&o.maxIntents, "max-intents", 20000, "decision cap per game; a capped game is a stall")
	fs.IntVar(&o.workers, "workers", 1, "games in parallel")
	fs.DurationVar(&o.wall, "wall", 0, "start no game after this much wall time (0: play every pair)")
	fs.StringVar(&o.out, "out", "", "games file, one JSON line per game (default: stdout)")
	fs.StringVar(&o.summary, "summary", "", "summary file, one JSON object (default: stderr)")
	fs.BoolVar(&o.skipRefused, "skip-refused", false, "skip a pair the set refuses and count it, instead of failing")
	fs.Float64Var(&o.sampleReplay, "sample-replay", 0, "fraction of games also written as koracle-game-2 records under <out>.replays, which kfuzz -replay re-executes in gorge; needs -out")
	fs.StringVar(&o.config, "config", configAutopay, "the verified surface to play: "+configAutopay+" or "+configFull)
	return fs
}

const (
	configAutopay = "lockstep-autopay"
	configFull    = "lockstep-full"
)

// check rejects a flag value before anything is opened.
func (o *options) check() error {
	for _, k := range []string{o.a, o.b} {
		if !slices.Contains([]string{seatRandom, seatFirst, seatHBot, seatSBT}, k) {
			return fmt.Errorf("seat %q: want random, first, hbot or sbt", k)
		}
	}
	if !slices.Contains(mullPolicies, o.mullPolicy) {
		return fmt.Errorf("-mulligan-policy %q: want one of %s", o.mullPolicy, strings.Join(mullPolicies, ", "))
	}
	if o.config != configAutopay && o.config != configFull {
		return fmt.Errorf("-config %q: want %s or %s", o.config, configAutopay, configFull)
	}
	if (o.deckA == "") != (o.deckB == "") {
		return errors.New("-deck-a and -deck-b go together")
	}
	if o.deckA != "" && (o.pairsFile != "" || o.explicit["decks"] || o.explicit["pool"]) {
		return errors.New("-deck-a and -deck-b replace -decks, -pool and -pairs-file")
	}
	if o.sampleReplay > 0 && o.out == "" {
		return errors.New("-sample-replay writes beside -out: pass -out")
	}
	if o.workers < 1 {
		return errors.New("-workers: want at least 1")
	}
	return nil
}

// dealtPairs resolves the run's pairs and the directory their decks are in. remove frees
// an extracted slice.
func (o *options) dealtPairs() (ps []oracle.Pairing, decks string, remove func(), err error) {
	remove = func() {}
	if o.deckA != "" {
		for p := o.firstPair; p < o.firstPair+o.pairs; p++ {
			seed, err := oracle.PairSeed(oracle.PairSpec{}, p, o.seed)
			if err != nil {
				return nil, "", remove, err
			}
			ps = append(ps, oracle.Pairing{Index: p, A: o.deckA, B: o.deckB, Seed: seed})
		}
		return ps, "", remove, nil
	}
	decks, pool, pairsFile, limit := o.decks, o.pool, o.pairsFile, o.explicit["pairs"]
	if decks == "" {
		decks = oracle.PoolDecks(o.set)
	}
	if !o.explicit["decks"] && pool == "" && pairsFile == "" && oracle.NeedDecks(decks) != nil {
		if o.seed != oracle.SliceSeed || o.split != "all" || o.firstPair+o.pairs > oracle.SlicePairs {
			return nil, "", remove, fmt.Errorf("no %s pool on disk, and its committed slice holds only -split all -seed %d pairs 0 to %d: set GORGE_POOLS for others", o.set, oracle.SliceSeed, oracle.SlicePairs-1)
		}
		dir, rm, err := oracle.ExtractSlice(o.set)
		if err != nil {
			return nil, "", remove, err
		}
		remove = rm
		decks, pairsFile, limit = filepath.Join(dir, "decks"), filepath.Join(dir, "pairs.tsv"), true
	}
	if err := oracle.NeedDecks(decks); err != nil {
		return nil, "", remove, err
	}
	if pairsFile == "" {
		if pool == "" {
			pool = filepath.Join(filepath.Dir(decks), "pool.tsv")
		}
		rows, err := oracle.ReadPool(pool, o.split)
		if err != nil {
			return nil, "", remove, err
		}
		return oracle.PairingsFrom(rows, o.firstPair, o.pairs, o.seed), decks, remove, nil
	}
	lines, err := oracle.ExplicitPairs(pairsFile)
	if err != nil {
		return nil, "", remove, err
	}
	for i, l := range lines {
		if i < o.firstPair {
			continue
		}
		if limit && len(ps) == o.pairs {
			break
		}
		seed, err := oracle.PairSeed(l, i, o.seed)
		if err != nil {
			return nil, "", remove, fmt.Errorf("%s pair %d: %w", pairsFile, i, err)
		}
		ps = append(ps, oracle.Pairing{Index: i, A: l.A, B: l.B, Seed: seed})
	}
	return ps, decks, remove, nil
}

// deckEntry is one deck, read once however many pairs name it.
type deckEntry struct {
	file    deck.File
	lines   []cardset.Entry
	refused *cardset.RefusedError // the first card the set will not play
}

// readDecks reads every deck of ps. A card outside the set is an error unless
// skipRefused is set, when it is the deck's first refusal: a drafted deck can hold a
// card outside its pool, while an unflagged run still fails loudly on a wrong set.
func readDecks(set cardset.Set, ps []oracle.Pairing, dir string, skipRefused bool) (map[string]*deckEntry, error) {
	out := map[string]*deckEntry{}
	supported := set.Supported()
	read := func(name string) error {
		if out[name] != nil {
			return nil
		}
		path := name // -deck-a and -deck-b name a file
		if dir != "" {
			path = filepath.Join(dir, name+".json")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := deck.Parse(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		e := &deckEntry{file: f}
		for _, c := range f.Cards {
			e.lines = append(e.lines, cardset.Entry{Name: c.Name, Count: c.Count})
			i, ok := set.Lookup(c.Name)
			if !ok {
				if !skipRefused {
					return fmt.Errorf("deck %s: card %q is not in set %s", name, c.Name, set.Name())
				}
				if e.refused == nil {
					e.refused = &cardset.RefusedError{Card: c.Name, Status: cardset.StatusNotLowered, Reason: "not in set " + set.Name()}
				}
				continue
			}
			if !supported[i] && e.refused == nil {
				e.refused = &cardset.RefusedError{Card: c.Name, Status: set.Status(i), Reason: set.Reason(i)}
			}
		}
		out[name] = e
		return nil
	}
	for _, p := range ps {
		if err := read(p.A); err != nil {
			return nil, err
		}
		if err := read(p.B); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// summary is the run's result. Games = AWins + BWins + Draws + Stalls + Errors.
type summary struct {
	Set                string                    `json:"set"`
	Config             string                    `json:"config"`
	Seats              [2]string                 `json:"seats"`
	Games              int                       `json:"games"`
	AWins              int                       `json:"a_wins"`
	BWins              int                       `json:"b_wins"`
	Draws              int                       `json:"draws"`
	Stalls             int                       `json:"stalls"`
	Errors             int                       `json:"errors"`
	StallsByClass      map[string]map[string]int `json:"stalls_by_class,omitempty"`
	RefusedPairs       int                       `json:"refused_pairs,omitempty"`
	RefusedCards       map[string]int            `json:"refused_cards,omitempty"`
	RefusedPairReasons map[string]int            `json:"refused_pair_reasons,omitempty"`
	Turns              int64                     `json:"turns"`
	Decisions          int64                     `json:"decisions"`
	CPUSeconds         float64                   `json:"cpu_s"`
	WallSeconds        float64                   `json:"wall_s"`
}

// sink takes finished games, tallies them and writes their rows in pair order: the games
// file is a function of the flags, whatever -workers says.
type sink struct {
	mu   sync.Mutex
	w    *bufio.Writer
	buf  []byte
	next int
	held map[int]*row
	sum  summary
	err  error
}

func (k *sink) add(i int, r *row) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if i != k.next {
		c := *r
		k.held[i] = &c
		return
	}
	k.emit(r)
	for k.next++; ; k.next++ {
		h := k.held[k.next]
		if h == nil {
			return
		}
		delete(k.held, k.next)
		k.emit(h)
	}
}

func (k *sink) emit(r *row) {
	s := &k.sum
	switch {
	case r.refused != "":
		s.RefusedPairs++
		s.RefusedPairReasons = addCount(s.RefusedPairReasons, r.refused)
		return
	case r.err != "":
		s.Errors++
	case r.stall != "":
		s.Stalls++
		if s.StallsByClass[r.class] == nil {
			s.StallsByClass[r.class] = map[string]int{}
		}
		s.StallsByClass[r.class][r.stall]++
	case r.winner == 0:
		s.AWins++
	case r.winner == 1:
		s.BWins++
	default:
		s.Draws++
	}
	s.Games++
	s.Turns += int64(r.turns)
	s.Decisions += int64(r.intents)
	k.buf = r.append(k.buf[:0])
	if _, err := k.w.Write(k.buf); err != nil && k.err == nil {
		k.err = err
	}
}

func addCount(m map[string]int, key string) map[string]int {
	if m == nil {
		m = map[string]int{}
	}
	m[key]++
	return m
}

func (r *row) append(b []byte) []byte {
	b = append(b, `{"pair":`...)
	b = strconv.AppendInt(b, int64(r.p.Index), 10)
	b = append(b, `,"decks":[`...)
	b = strconv.AppendQuote(b, r.p.A)
	b = append(b, ',')
	b = strconv.AppendQuote(b, r.p.B)
	b = append(b, `],"seed":`...)
	b = strconv.AppendUint(b, r.p.Seed, 10)
	b = append(b, `,"winner":`...)
	b = strconv.AppendInt(b, int64(r.winner), 10)
	b = append(b, `,"turns":`...)
	b = strconv.AppendInt(b, int64(r.turns), 10)
	b = append(b, `,"intents":`...)
	b = strconv.AppendInt(b, int64(r.intents), 10)
	if r.stall != "" {
		b = append(b, `,"stall":`...)
		b = strconv.AppendQuote(b, r.stall)
		b = append(b, `,"class":`...)
		b = strconv.AppendQuote(b, r.class)
	}
	if r.err != "" {
		q, _ := json.Marshal(r.err)
		b = append(append(b, `,"error":`...), q...)
	}
	return append(b, "}\n"...)
}

func run(args []string, stdout, stderr io.Writer) int {
	o := &options{explicit: map[string]bool{}}
	fs := o.flags(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fs.Visit(func(f *flag.Flag) { o.explicit[f.Name] = true })
	if err := o.check(); err != nil {
		fmt.Fprintln(stderr, "dplay:", err)
		return 2
	}
	if err := o.play(stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "dplay:", err)
		return 1
	}
	return 0
}

func (o *options) play(stdout, stderr io.Writer) error {
	if o.cards != "" {
		os.Setenv("GORGE_CARDS", o.cards) // the tables come from the corpus this run plays
	}
	set, err := oracle.PlaySet(o.set)
	if err != nil {
		return err
	}
	all, dir, remove, err := o.dealtPairs()
	defer remove()
	if err != nil {
		return err
	}
	decks, err := readDecks(set, all, dir, o.skipRefused)
	if err != nil {
		return err
	}
	sum := summary{Set: o.set, Config: o.config, Seats: [2]string{o.a, o.b}, StallsByClass: map[string]map[string]int{}}
	ps := all[:0]
	for _, p := range all {
		kept := true
		for _, name := range []string{p.A, p.B} {
			r := decks[name].refused
			if r == nil {
				continue
			}
			if !o.skipRefused {
				return fmt.Errorf("deck %s: %w (-skip-refused skips such pairs and counts them)", name, r)
			}
			sum.RefusedCards = addCount(sum.RefusedCards, r.Card)
			kept = false
		}
		if kept {
			ps = append(ps, p)
		} else {
			sum.RefusedPairs++
		}
	}
	r, err := newRunner(o, set, ps, decks, dir, stderr)
	if err != nil {
		return err
	}
	out, closeOut, err := output(o.out, stdout)
	if err != nil {
		return err
	}
	defer closeOut()
	k := &sink{w: bufio.NewWriterSize(out, 1<<16), held: map[int]*row{}, sum: sum}
	if err := r.playAll(k); err != nil {
		k.w.Flush()
		return err
	}
	if err := k.w.Flush(); err != nil {
		return err
	}
	if k.err != nil {
		return k.err
	}
	js, err := json.MarshalIndent(&k.sum, "", "  ")
	if err != nil {
		return err
	}
	dst, closeSum, err := output(o.summary, stderr)
	if err != nil {
		return err
	}
	defer closeSum()
	if _, err := fmt.Fprintf(dst, "%s\n", js); err != nil {
		return err
	}
	if k.sum.Games == 0 {
		return fmt.Errorf("no game played: %d refused pairs", k.sum.RefusedPairs)
	}
	return nil
}

// output is the named file, or fallback for "".
func output(path string, fallback io.Writer) (io.Writer, func(), error) {
	if path == "" {
		return fallback, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// playAll plays the pairs on -workers goroutines. The CPU and wall counts start after the
// first pair's table is built, so a run's setup is outside them.
func (r *runner) playAll(k *sink) error {
	workers := make([]*worker, r.o.workers)
	for i := range workers {
		workers[i] = r.newWorker()
	}
	if len(r.pairs) > 0 {
		workers[0].table(&r.pairs[0]) // a refusal here is met again when the pair is played
	}
	t0, c0 := time.Now(), cpuSeconds()
	var next atomic.Int64
	var fatal atomic.Pointer[error]
	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var rw row
			for fatal.Load() == nil && (r.o.wall == 0 || time.Since(t0) < r.o.wall) {
				i := int(next.Add(1) - 1)
				if i >= len(r.pairs) {
					return
				}
				w.play(&r.pairs[i], &rw)
				if rw.refused != "" && !r.o.skipRefused {
					err := fmt.Errorf("pair %d (%s vs %s): %s (-skip-refused skips such pairs and counts them)", rw.p.Index, rw.p.A, rw.p.B, rw.refused)
					fatal.CompareAndSwap(nil, &err)
				}
				k.add(i, &rw)
			}
		}()
	}
	wg.Wait()
	k.sum.CPUSeconds, k.sum.WallSeconds = cpuSeconds()-c0, time.Since(t0).Seconds()
	if e := fatal.Load(); e != nil {
		return *e
	}
	return nil
}

// cpuSeconds is the process's user and system CPU time.
func cpuSeconds() float64 {
	var ru syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &ru) != nil {
		return 0
	}
	return float64(ru.Utime.Nano()+ru.Stime.Nano()) / 1e9
}
