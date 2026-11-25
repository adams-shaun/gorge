// Command dpod plays 4-seat Commander pods of the five official Foundations
// Commander (FDC) precons in dirac, with no other engine in the loop, for the
// cross-engine bench (scripts/xengine-bench.sh, PAIRS=fdc-sub).
//
// dirac does not lower every FDC card, so each card the kernel refuses is
// replaced, copy for copy, by the deck's most common basic land (the edh
// pool's rule for kernel-refused copies). -write DIR writes the substituted
// decks as deck files and substitutions.tsv, so gorge plays the same pods
// (cmd/enginebench -pair fdc -pod-dir DIR). Game g leaves out deck (g/4)%5 and
// rotates the other four seats by g%4, as enginebench's fdc row does; every
// seat plays dirac.RandomPolicy under dirac.LockstepConfig's auto-pay
// surface, with no mulligan round.
//
//	dpod -cards .cards -decks testkit/testutil/decks -secs 20 -summary out.json
//
// The summary is one JSON object with dplay's keys (games, turns, decisions,
// cpu_s, stalls, errors), so the bench summarizer records it as a dplay run.
// A run is bounded by process CPU seconds; no game reads a clock.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// fdcDecks are cmd/enginebench's fdcDecks, in the same order.
var fdcDecks = [...]string{
	"foundations-calling-all-angels",
	"foundations-keen-engineering",
	"foundations-reign-of-dragons",
	"foundations-tramplesaurus-rex",
	"foundations-wretched-ranks",
}

const (
	podSeats  = 4
	maxTurns  = 300
	maxIntent = 200000
	// randomSalt is dplay's: a game's random stream from its seed.
	randomSalt = 0x72616e64
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

type summary struct {
	Games      int            `json:"games"`
	Turns      int64          `json:"turns"`
	Decisions  int64          `json:"decisions"`
	CPUSeconds float64        `json:"cpu_s"`
	Stalls     int            `json:"stalls"`
	StallKinds map[string]int `json:"stall_kinds,omitempty"`
	Errors     int            `json:"errors"`
	Wins       [podSeats]int  `json:"wins"`
	Substitute map[string]int `json:"substituted"` // card -> copies replaced, over the five decks
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dpod", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cardsDir := fs.String("cards", ".cards", "corpus directory")
	deckDir := fs.String("decks", "testkit/testutil/decks", "directory holding the foundations-* deck files")
	secs := fs.Float64("secs", 20, "process CPU seconds of timed play (0: write and exit)")
	seed := fs.Uint64("seed", 1, "base seed; game g uses seed+g")
	write := fs.String("write", "", "write the substituted deck files and substitutions.tsv here")
	sumPath := fs.String("summary", "", "summary file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := play(*cardsDir, *deckDir, *secs, *seed, *write, *sumPath, stdout); err != nil {
		fmt.Fprintln(stderr, "dpod:", err)
		return 1
	}
	return 0
}

// pod is the five substituted decks and the kernel tables of the five 4-deck pods.
type pod struct {
	files [len(fdcDecks)]deck.File
	tbls  [len(fdcDecks)]*kop.Table                  // by left-out deck
	ids   [len(fdcDecks)][len(fdcDecks)][]kop.CardID // [left out][deck]
	cmd   [len(fdcDecks)][len(fdcDecks)]int16        // [left out][deck]: the commander's index in ids
	subst map[string]int
	rows  []string // substitutions.tsv lines
}

func readDeck(dir, name string) (deck.File, error) {
	raw, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return deck.File{}, err
	}
	f, err := deck.Parse(raw)
	if err != nil {
		return deck.File{}, fmt.Errorf("%s: %w", name, err)
	}
	if len(f.CommanderNames()) != 1 {
		return deck.File{}, fmt.Errorf("%s: want one commander, have %v", name, f.CommanderNames())
	}
	return f, nil
}

func isBasic(n string) bool {
	switch n {
	case "Plains", "Island", "Swamp", "Mountain", "Forest", "Wastes":
		return true
	}
	return false
}

// basicOf is the deck's most common basic land, the replacement for its refused cards.
func basicOf(f deck.File) string {
	best, n := "", 0
	for _, e := range f.Cards {
		if isBasic(e.Name) && e.Count > n {
			best, n = e.Name, e.Count
		}
	}
	return best
}

// buildPod substitutes until every 4-deck pod's table builds: a card the kernel
// does not lower, and every card but a commander or a basic land named in an
// interaction the kernel refuses, is replaced in every deck that holds it.
func buildPod(reg *cards.Registry, deckDir string) (*pod, error) {
	p := &pod{subst: map[string]int{}}
	orig := [len(fdcDecks)]deck.File{}
	var pool []*cards.Card
	seen := map[string]bool{}
	commanders := map[string]bool{}
	for i, name := range fdcDecks {
		f, err := readDeck(deckDir, name)
		if err != nil {
			return nil, err
		}
		orig[i] = f
		commanders[cards.NormalizeName(f.CommanderNames()[0])] = true
		for _, e := range f.Cards {
			if seen[e.Name] {
				continue
			}
			seen[e.Name] = true
			c, ok := reg.Lookup(e.Name)
			if !ok {
				return nil, fmt.Errorf("%s: %q is not in the corpus", name, e.Name)
			}
			pool = append(pool, c)
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, "fdc-pod")
	if err != nil {
		return nil, err
	}
	// why[card]: the reason it is replaced. "not lowered" cards first; an
	// unverified card (no evidence yet) lowers and stays.
	why := map[string]string{}
	for i, st := range pd.Status {
		if i < len(pd.Names) && st != cardset.StatusSupported && st != cardset.StatusUnverified {
			why[pd.Names[i]] = strings.ReplaceAll(pd.Reason[i], "\t", " ")
		}
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	for round := 0; ; round++ {
		if round > 64 {
			return nil, fmt.Errorf("substitution did not converge")
		}
		for i := range p.files {
			p.files[i] = orig[i]
			basic := basicOf(orig[i])
			var out []deck.Entry
			for _, e := range orig[i].Cards {
				if _, bad := why[e.Name]; bad {
					out = append(out, deck.Entry{Name: basic, Count: e.Count})
				} else {
					out = append(out, e)
				}
			}
			p.files[i].Cards = out
		}
		grew := false
		for skip := range fdcDecks {
			var decks [][]cardset.Entry
			var which []int
			for i, f := range p.files {
				if i == skip {
					continue
				}
				which = append(which, i)
				var es []cardset.Entry
				for _, e := range f.Cards {
					es = append(es, cardset.Entry{Name: e.Name, Count: e.Count})
				}
				decks = append(decks, es)
			}
			tbl, ids, err := set.MatchupN(decks)
			var re *cardset.RefusedError
			var ir *cardset.InteractionRefusal
			switch {
			case errors.As(err, &re):
				if commanders[cards.NormalizeName(re.Card)] || isBasic(re.Card) {
					return nil, fmt.Errorf("pod without %s: %q refused: %v", fdcDecks[skip], re.Card, err)
				}
				why[re.Card] = err.Error()
				grew = true
				continue
			case errors.As(err, &ir):
				n := 0
				for _, c := range ir.Cards {
					if !commanders[cards.NormalizeName(c)] && !isBasic(c) {
						why[c] = "interaction " + ir.Reason + ": " + strings.Join(ir.Cards, ", ")
						n++
					}
				}
				if n == 0 {
					return nil, fmt.Errorf("pod without %s: %v", fdcDecks[skip], err)
				}
				grew = true
				continue
			case err != nil:
				return nil, fmt.Errorf("pod without %s: %v", fdcDecks[skip], err)
			}
			if dirac.TwoSeatOnly(tbl) {
				return nil, fmt.Errorf("pod without %s: the table is two-seat only: %v", fdcDecks[skip], dirac.TwoSeatCauses(tbl))
			}
			p.tbls[skip] = tbl
			for k, i := range which {
				p.ids[skip][i] = ids[k]
				p.cmd[skip][i] = -1
				want := cards.NormalizeName(p.files[i].CommanderNames()[0])
				for j, id := range ids[k] {
					if cards.NormalizeName(tbl.Cards[id].Name) == want {
						p.cmd[skip][i] = int16(j)
						break
					}
				}
				if p.cmd[skip][i] < 0 {
					return nil, fmt.Errorf("%s: commander %q not found in its kernel deck", fdcDecks[i], want)
				}
			}
		}
		if !grew {
			break
		}
	}
	for i, f := range orig {
		basic := basicOf(f)
		for _, e := range f.Cards {
			if w, bad := why[e.Name]; bad {
				p.subst[e.Name] += e.Count
				p.rows = append(p.rows, fmt.Sprintf("%s\t%s\t%d\t%s\t%s", fdcDecks[i], e.Name, e.Count, basic, w))
			}
		}
	}
	return p, nil
}

func (p *pod) writeTo(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, f := range p.files {
		b, err := json.MarshalIndent(f, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, fdcDecks[i]+".json"), append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	rows := append([]string{"deck\tcard\tcopies\treplacement\twhy"}, p.rows...)
	sort.Strings(rows[1:])
	return os.WriteFile(filepath.Join(dir, "substitutions.tsv"), []byte(strings.Join(rows, "\n")+"\n"), 0o644)
}

// seating is game g's decks in seat order: (g/4)%5 left out, the rest rotated by g%4.
func seating(g int) [podSeats]int {
	skip, rot := (g/podSeats)%len(fdcDecks), g%podSeats
	var order []int
	for i := range fdcDecks {
		if i != skip {
			order = append(order, i)
		}
	}
	var out [podSeats]int
	for s := range out {
		out[s] = order[(s+rot)%podSeats]
	}
	return out
}

func cpuNow() float64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return float64(ru.Utime.Sec+ru.Stime.Sec) + float64(ru.Utime.Usec+ru.Stime.Usec)/1e6
}

type game struct {
	s   dirac.State
	d   dirac.Decision
	cfg dirac.Config
}

// playOne plays game g and adds it to sum.
func (gm *game) playOne(p *pod, seed uint64, g int, sum *summary) {
	defer func() {
		if e := recover(); e != nil {
			sum.Errors++
		}
	}()
	seats := seating(g)
	skip := (g / podSeats) % len(fdcDecks)
	var decks [dirac.MaxSeats][]kop.CardID
	cfg := gm.cfg
	for i := range cfg.Commanders {
		cfg.Commanders[i] = [2]int16{-1, -1}
	}
	for s, i := range seats {
		decks[s] = p.ids[skip][i]
		cfg.Commanders[s][0] = p.cmd[skip][i]
	}
	gseed := seed + uint64(g)
	gm.s.InitN(p.tbls[skip], decks, podSeats, gseed, cfg)
	rnd := dirac.NewRandomPolicy(gseed ^ randomSalt)
	n := int64(0)
	stall := ""
	for {
		gm.s.StepInto(&gm.d)
		if gm.d.Kind == dirac.DkNone {
			break
		}
		if n >= maxIntent {
			stall = "intents"
			break
		}
		if gm.s.Turn() >= maxTurns {
			stall = "turns"
			break
		}
		gm.s.Apply(rnd.Pick(&gm.d))
		n++
	}
	if why := gm.s.StallReason(); why != dirac.StallNone && stall == "" {
		stall = dirac.StallName(why)
	}
	over, win := gm.s.Terminal()
	if !over && stall == "" {
		stall = "unfinished"
	}
	sum.Games++
	sum.Turns += int64(gm.s.Turn())
	sum.Decisions += n
	if stall != "" {
		sum.Stalls++
		if sum.StallKinds == nil {
			sum.StallKinds = map[string]int{}
		}
		sum.StallKinds[stall]++
	} else if win >= 0 && int(win) < podSeats {
		sum.Wins[win]++
	}
}

func play(cardsDir, deckDir string, secs float64, seed uint64, write, sumPath string, stdout io.Writer) error {
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		return err
	}
	p, err := buildPod(reg, deckDir)
	if err != nil {
		return err
	}
	if write != "" {
		if err := p.writeTo(write); err != nil {
			return err
		}
	}
	sum := summary{Substitute: p.subst}
	if secs > 0 {
		for _, t := range p.tbls {
			dirac.Register(t)
		}
		gm := &game{cfg: dirac.LockstepConfig(dirac.SurfaceAutopay)}
		gm.cfg.Commander = true
		var warm summary
		for g := 0; g < 2; g++ { // untimed, as enginebench's warm games
			gm.playOne(p, seed, g, &warm)
		}
		c0 := cpuNow()
		for g := 0; cpuNow()-c0 < secs; g++ {
			gm.playOne(p, seed, g, &sum)
		}
		sum.CPUSeconds = cpuNow() - c0
	}
	b, err := json.Marshal(sum)
	if err != nil {
		return err
	}
	if sumPath == "" {
		_, err = fmt.Fprintln(stdout, string(b))
		return err
	}
	return os.WriteFile(sumPath, append(b, '\n'), 0o644)
}
