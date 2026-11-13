// Command khbot measures a kernel search bot against the bot it ports,
// gorge's heuristic autopay bot: FDN pool pairings played in lockstep
// (gorge and the kernel), gorge's bot choosing every action on gorge, the
// kernel bot asked the same decisions on the kernel's state
// (oracle.HBotParityGame). The kernel bot is the native port
// (dirac/search/hbot). It prints the agreement rate per decision kind and
// the disagreement classes, each with one example. With -seats 3 or 4
// (the dirac_mp build) each game is a pod of consecutive pairings' decks
// (oracle.HBotParityPod).
//
//	go run ./dirac/oracle/cmd/khbot -pairs 100 -seed 78 -workers 4
//	go run -tags dirac_mp ./dirac/oracle/cmd/khbot -seats 4 -pairs 50
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/deck"
)

func main() { os.Exit(run()) }

func run() int {
	var (
		cardsDir  = flag.String("cards", "", "gorge corpus directory (default: $GORGE_CARDS, else .cards in or above the working directory)")
		synth     = flag.String("synth", "", "play a synthetic evidence pool (make dirac-tables writes them) instead of -decks and -pool")
		dealt     = flag.String("dealt", "", "play decks dealt from a pool's table (fdn), as a clone without the deck pool does, instead of -decks and -pool")
		decksDir  = flag.String("decks", "", "pool deck directory (default: the fdn pool's, under $GORGE_POOLS or the module's .pools, else its committed slice)")
		poolPath  = flag.String("pool", "", "pool.tsv (default: <decks>/../pool.tsv)")
		split     = flag.String("split", "eval", "pool split: train, eval or all")
		pairs     = flag.Int("pairs", 100, "deck pairs (one game each)")
		firstPair = flag.Int("first-pair", 0, "index of the first pair")
		seed      = flag.Uint64("seed", 78, "base seed (the pairing rule)")
		workers   = flag.Int("workers", 4, "games in parallel")
		maxDec    = flag.Int("max-decisions", 20000, "canonical decision cap per game")
		surface   = flag.Bool("autopay-surface", true, "oracle.Config.GorgeAutopaySurface (gorge's own autopay surface)")
		elide     = flag.Bool("pass-elision", true, "elide single-option decisions (the kernel's default configuration does)")
		verbose   = flag.Bool("v", false, "print every disagreement")
		hold      = flag.Bool("hold", false, "arm gorge TimingRules.HoldInstant on both sides (hbot must hold too)")
		seats     = flag.Int("seats", 2, "seats per game; above 2 (the dirac_mp build) a game is a pod of consecutive pairings' decks, -pairs counting pods")
	)
	flag.Parse()
	if *cardsDir == "" {
		d, err := cardset.CorpusDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		*cardsDir = d
	} else {
		os.Setenv("GORGE_CARDS", *cardsDir) // the tables come from the corpus this run plays, not the one the cwd happens to sit under
	}
	if *decksDir == "" && *synth == "" && *dealt == "" {
		*decksDir = oracle.PoolDecks("fdn")
		if oracle.NeedDecks(*decksDir) != nil {
			dir, remove, err := oracle.ExtractSlice("fdn")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			defer remove()
			*decksDir = filepath.Join(dir, "decks")
		}
	}
	if *synth == "" && *dealt == "" {
		if err := oracle.NeedDecks(*decksDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	c, err := oracle.OpenCorpus(*cardsDir, *decksDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var (
		set  cardset.Set
		pool []oracle.PoolDeck
	)
	if *synth != "" || *dealt != "" {
		if *synth != "" {
			set, err = cardset.SynthByName(*synth)
		} else {
			set, err = cardset.RawByName(*dealt)
		}
		if err == nil {
			set = cardset.AllowUnverified(set) // decks are dealt without the refused cards
			var files map[string]deck.File
			if pool, files, err = oracle.SynthPool(c.Reg, set, 1, oracle.SynthDecks); err == nil {
				c.AddDecks(files)
			}
		}
	} else {
		pp := *poolPath
		if pp == "" {
			pp = filepath.Join(filepath.Dir(*decksDir), "pool.tsv")
		}
		pool, err = oracle.ReadPool(pp, *split)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	per := (*seats + 1) / 2 // pairings per game
	all := oracle.PairingsFrom(pool, *firstPair, *pairs*per, *seed)
	ps := all
	if per > 1 {
		ps = make([]oracle.Pairing, 0, *pairs)
		for i := 0; i+per <= len(all); i += per {
			ps = append(ps, all[i])
		}
	}
	podOf := func(i int) []string {
		var names []string
		for j := i * per; j < (i+1)*per; j++ {
			names = append(names, all[j].A, all[j].B)
		}
		return names[:*seats]
	}
	cfg := oracle.Config{GorgeAutopaySurface: *surface, PassElision: *elide}
	mk := botFactory(c, set, *hold)
	type res struct {
		p   oracle.Pairing
		r   oracle.HBotParityResult
		err error
	}
	results := make([]res, len(ps))
	var wg sync.WaitGroup
	next := make(chan int)
	t0 := time.Now()
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				opt := oracle.HBotParityOptions{HoldInstant: *hold, Set: set}
				var (
					r   oracle.HBotParityResult
					err error
				)
				if *seats > 2 {
					r, err = oracle.HBotParityPod(c, podOf(i), ps[i].Seed, cfg, *maxDec, mk, opt)
				} else {
					r, err = oracle.HBotParityGameOpts(c, ps[i].A, ps[i].B, ps[i].Seed, cfg, *maxDec, mk, opt)
				}
				results[i] = res{ps[i], r, err}
			}
		}()
	}
	for i := range ps {
		next <- i
	}
	close(next)
	wg.Wait()
	type tally struct{ n, agree, equiv int }
	kinds := map[string]*tally{}
	classes := map[string]int{}
	example := map[string]string{}
	games, divs, unoff, total, unoffRec, restr, stalls := 0, 0, 0, 0, 0, 0, 0
	for _, x := range results {
		if x.err != nil {
			fmt.Fprintf(os.Stderr, "pair %d: %v\n", x.p.Index, x.err)
			continue
		}
		games++
		if x.r.Res.Stall != "" {
			stalls++
		}
		if x.r.Res.Div != nil {
			divs++
			if *verbose {
				fmt.Printf("pair %d diverged: %s\n", x.p.Index, x.r.Res.Div)
			}
		}
		unoff += x.r.Unoffered
		restr += x.r.Manual
		for _, rc := range x.r.Recs {
			if rc.Unoffered {
				unoffRec++
				classes[rc.Class]++
				if _, ok := example[rc.Class]; !ok {
					example[rc.Class] = fmt.Sprintf("pair %d: %s", x.p.Index, rc.Where)
				}
				continue
			}
			t := kinds[rc.Kind]
			if t == nil {
				t = &tally{}
				kinds[rc.Kind] = t
			}
			t.n++
			total++
			if rc.Agree {
				t.agree++
			}
			if rc.Equiv {
				t.equiv++
			}
			if !rc.Agree {
				cl := rc.Class
				if rc.Equiv {
					cl += " (same card)"
				}
				classes[cl]++
				if _, ok := example[cl]; !ok {
					example[cl] = fmt.Sprintf("pair %d: %s", x.p.Index, rc.Where)
				}
				if *verbose {
					fmt.Printf("pair %d %s\n", x.p.Index, rc.Where)
				}
			}
		}
	}
	fmt.Printf("khbot hbot:")
	fmt.Printf(" %d games (%d diverged before the end), %d decisions compared, %d not comparable (gorge's answer not on the canonical surface; %d picks followed another option), wall %s\n",
		games, divs, total, unoffRec, unoff, time.Since(t0).Round(time.Millisecond))
	var ks []string
	for k := range kinds {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	fmt.Printf("(%d games stopped at a cap or a kernel stall)\n", stalls)
	fmt.Printf("(%d priority decisions where gorge's bot tapped mana by hand: compared as the cast it taps for)\n", restr)
	fmt.Printf("%-18s %7s %8s %8s\n", "kind", "n", "agree", "same-card")
	for _, k := range ks {
		t := kinds[k]
		fmt.Printf("%-18s %7d %7.2f%% %7.2f%%\n", k, t.n, 100*float64(t.agree)/float64(t.n), 100*float64(t.equiv)/float64(t.n))
	}
	type kv struct {
		k string
		n int
	}
	var cl []kv
	for k, n := range classes {
		cl = append(cl, kv{k, n})
	}
	sort.Slice(cl, func(i, j int) bool { return cl[i].n > cl[j].n || cl[i].n == cl[j].n && cl[i].k < cl[j].k })
	fmt.Println("disagreement classes:")
	for i, x := range cl {
		if i >= 25 {
			break
		}
		fmt.Printf("%6d  %s\n        e.g. %s\n", x.n, x.k, example[x.k])
	}
	return 0
}

// botFactory builds the kernel bot for a matchup's table.
func botFactory(c *oracle.Corpus, set cardset.Set, hold bool) func(tbl *kop.Table) oracle.KernelBot {
	return func(tbl *kop.Table) oracle.KernelBot {
		facts := oracle.HBotFacts(c.Reg, tbl, nil)
		if set != nil {
			facts = oracle.HBotFactsSet(c.Reg, tbl, nil, set)
		}
		b := hbot.New(facts)
		if hold {
			b.EnableHoldInstant()
		}
		return b
	}
}
