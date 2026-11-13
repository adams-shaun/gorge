package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/deck"
)

const (
	seatRandom = "random"
	seatFirst  = "first"
	seatHBot   = "hbot"
	seatSBT    = "sbt"
)

// randomSalt derives a game's random seat stream from its seed.
const randomSalt = 0x72616e64

// runner is what the workers share.
type runner struct {
	o      *options
	set    cardset.Set
	cfg    dirac.Config
	pairs  []oracle.Pairing
	decks  map[string]*deckEntry
	corp   *oracle.Corpus // the gorge cards hbot and sbt read; nil for seats that read none
	baseMu sync.Mutex     // guards bases and built
	bases  map[[2]string]*pairBase
	built  int // real fills so far; a revisit is a cache hit, not a fill
	stderr io.Writer
}

func newRunner(o *options, set cardset.Set, ps []oracle.Pairing, decks map[string]*deckEntry, dir string, stderr io.Writer) (*runner, error) {
	surface := dirac.SurfaceAutopay
	if o.config == configFull {
		surface = dirac.SurfaceFull
	}
	cfg := dirac.LockstepConfig(surface)
	cfg.Mulligans = uint8(o.mulligans)
	r := &runner{o: o, set: set, cfg: cfg, pairs: ps, decks: decks, bases: map[[2]string]*pairBase{}, stderr: stderr}
	cards := o.cards
	if cards == "" && (o.sampleReplay > 0 || o.reads()) {
		d, err := cardset.CorpusDir()
		if err != nil {
			return nil, err
		}
		cards = d
	}
	var err error
	switch {
	case o.sampleReplay > 0: // a replay record's matchup needs the whole registry
		if r.corp, err = oracle.OpenCorpus(cards, dir); err != nil {
			return nil, err
		}
		files := map[string]deck.File{}
		for name, e := range decks {
			files[name] = e.file
		}
		r.corp.AddDecks(files)
	case o.reads():
		if r.corp, err = oracle.OpenKernelCorpus(cards, dir, set.Names()); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// reads reports whether a seat reads gorge's compiled cards.
func (o *options) reads() bool {
	return o.a == seatHBot || o.b == seatHBot || o.a == seatSBT || o.b == seatSBT
}

// row is one game's result, or a pair's refusal.
type row struct {
	p              *oracle.Pairing
	winner         int8
	turns, intents int32
	stall, class   string
	err, refused   string
}

// pairBase is a pair's shared build: the part that depends only on the two decks. Every
// worker that plays the pair wraps one in its own bots; it is filled at most once per run
// and its table stays registered until playAll releases it.
type pairBase struct {
	lo, hi   string
	tbl      *kop.Table
	decks    [2][]kop.CardID // lo's, hi's
	lands    [2]int
	facts    *hbot.Facts      // nil unless a seat reads it
	sbtCards *oracle.SBTCards // nil unless a seat reads it
}

// pairTable is the two decks' table with what the seats built over it. Both seat orders
// of a pair share it: a table does not depend on which deck is first.
type pairTable struct {
	lo, hi string
	tbl    *kop.Table
	decks  [2][]kop.CardID // lo's, hi's
	lands  [2]int
	hb     [2]*hbot.Bot
	sbt    *oracle.SBT
}

// worker plays games, one at a time, on its own State.
type worker struct {
	r      *runner
	s      dirac.State
	d      dirac.Decision
	tab    *pairTable
	seats  [2]string
	rnd    dirac.RandomPolicy
	mull   mulligans
	record bool // keep the game's actions in acts
	acts   []dirac.Action
}

func (r *runner) newWorker() *worker {
	w := &worker{r: r, seats: [2]string{r.o.a, r.o.b}}
	w.mull.init(r.o.mullPolicy)
	return w
}

// table is the worker's table for p's decks, built when the pair changes. It reports
// whether p seats the decks in the table's reverse order.
func (w *worker) table(p *oracle.Pairing) (t *pairTable, swapped bool, err error) {
	lo, hi := p.A, p.B
	if hi < lo {
		lo, hi, swapped = hi, lo, true
	}
	if t = w.tab; t != nil && t.lo == lo && t.hi == hi {
		return t, swapped, nil
	}
	b, err := w.r.base(lo, hi)
	if err != nil {
		return nil, false, err
	}
	t = &pairTable{lo: lo, hi: hi, tbl: b.tbl, decks: b.decks, lands: b.lands}
	for seat, kind := range w.seats {
		switch kind {
		case seatHBot:
			t.hb[seat] = hbot.New(b.facts)
		case seatSBT:
			t.sbt = oracle.NewSBT(b.sbtCards)
		}
	}
	w.tab = t
	return t, swapped, nil
}

// base returns the pair's shared build, filling it under baseMu when absent. Its table is
// registered for the whole run; playAll releases the bases.
func (r *runner) base(lo, hi string) (*pairBase, error) {
	key := [2]string{lo, hi}
	r.baseMu.Lock()
	defer r.baseMu.Unlock()
	if b := r.bases[key]; b != nil {
		return b, nil
	}
	tbl, dl, dh, err := r.set.Matchup(r.decks[lo].lines, r.decks[hi].lines)
	if err != nil {
		return nil, err // a refused pair is not cached: it is refused again next time
	}
	dirac.Register(tbl)
	b := &pairBase{lo: lo, hi: hi, tbl: tbl, decks: [2][]kop.CardID{dl, dh}}
	for i, ids := range b.decks {
		for _, id := range ids {
			if tbl.Cards[id].Types&kop.TLand != 0 {
				b.lands[i]++
			}
		}
	}
	for _, kind := range []string{r.o.a, r.o.b} { // the seats are run-constant
		switch kind {
		case seatHBot:
			if b.facts == nil {
				b.facts = oracle.HBotFactsSet(r.corp.Reg, tbl, nil, r.set)
			}
		case seatSBT:
			if b.sbtCards == nil {
				b.sbtCards = oracle.NewSBTCardsSet(r.corp.Reg, tbl, nil, r.set)
			}
		}
	}
	r.bases[key] = b
	r.built++
	return b, nil
}

// play plays p into rw. A panic is the game's error: one bad game does not end the run.
func (w *worker) play(p *oracle.Pairing, rw *row) {
	*rw = row{p: p, winner: -1}
	defer func() {
		if e := recover(); e != nil {
			rw.err = fmt.Sprint(e)
			rw.winner = -1
			fmt.Fprintf(w.r.stderr, "dplay: pair %d (%s vs %s, seed %d): panic: %v\n", p.Index, p.A, p.B, p.Seed, e)
		}
	}()
	t, swapped, err := w.table(p)
	if err != nil {
		rw.refused = err.Error()
		return
	}
	a, b := 0, 1
	if swapped {
		a, b = 1, 0
	}
	s, d := &w.s, &w.d
	s.Init(t.tbl, t.decks[a], t.decks[b], p.Seed, w.r.cfg)
	w.rnd = dirac.NewRandomPolicy(p.Seed ^ randomSalt)
	w.mull.newGame(p.Seed, [2]deckShape{{len(t.decks[a]), t.lands[a]}, {len(t.decks[b]), t.lands[b]}})
	if t.sbt != nil {
		t.sbt.NewGame(seatSeeds(p.Seed))
	}
	o := w.r.o
	sample := sampled(p, o.sampleReplay)
	record := w.record || sample
	w.acts = w.acts[:0]
	n := int32(0)
	for {
		s.StepInto(d)
		if d.Kind == dirac.DkNone {
			break
		}
		if int(n) >= o.maxIntents {
			rw.stall, rw.class = "intents", "cap"
			break
		}
		if s.Turn() >= o.maxTurns {
			rw.stall, rw.class = "turns", "cap"
			break
		}
		var act dirac.Action
		if d.Kind == dirac.DkMulligan || d.Kind == dirac.DkChoose && d.Sub == dirac.ChBottom {
			act = w.mull.answer(s, d)
		} else {
			switch w.seats[d.Player] {
			case seatRandom:
				act = w.rnd.Pick(d)
			case seatFirst:
				act = d.Options[0]
			case seatHBot:
				act = d.Options[t.hb[d.Player].Pick(s, d)]
			default:
				act = d.Options[t.sbt.Pick(s, d)]
			}
		}
		n++
		if record {
			w.acts = append(w.acts, act)
		}
		s.Apply(act)
	}
	rw.intents, rw.turns = n, int32(s.Turn())
	if why := s.StallReason(); why != dirac.StallNone {
		rw.stall, rw.class = dirac.StallName(why), "capacity"
		if dirac.StallIsUnmodelled(why) {
			rw.class = "unmodelled"
		}
	}
	over, win := s.Terminal()
	if over && rw.stall == "" {
		rw.winner = win
	}
	if !over && rw.stall == "" {
		rw.stall, rw.class = "unfinished", "unmodelled"
	}
	if sample {
		w.writeReplay(p, t, [2][]kop.CardID{t.decks[a], t.decks[b]})
	}
}

// splitmix is SplitMix64's output for state x: a well-mixed value from a seed.
func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// seatSeeds is the two seats' own seeds for a game.
func seatSeeds(seed uint64) (out [2]uint64) {
	for p := range out {
		out[p] = splitmix(seed ^ uint64(p+1)*0x9e3779b97f4a7c15)
	}
	return out
}

// sampled reports whether p's game is in the -sample-replay fraction, by its identity.
func sampled(p *oracle.Pairing, fraction float64) bool {
	return fraction > 0 && float64(splitmix(p.Seed^uint64(p.Index))%1000000)/1e6 < fraction
}

// writeReplay writes the game just played as a record kfuzz -replay re-executes in gorge.
func (w *worker) writeReplay(p *oracle.Pairing, t *pairTable, decks [2][]kop.CardID) {
	o := w.r.o
	m, _, err := oracle.SetMatchup(w.r.set, w.r.corp, p.A, p.B, o.mulligans)
	if err == nil {
		var g *oracle.KernelGame
		cfg := oracle.Config{PassElision: true, GorgeAutopaySurface: o.config == configAutopay}
		if g, err = oracle.RecordKernelActions(m, t.tbl, nil, decks, p.Seed, cfg, w.r.cfg, w.acts); err == nil {
			dir := o.out + ".replays"
			if err = os.MkdirAll(dir, 0o755); err == nil {
				err = g.Write(filepath.Join(dir, fmt.Sprintf("game-%06d.json", p.Index)))
			}
		}
	}
	if err != nil {
		fmt.Fprintf(w.r.stderr, "dplay: replay record of pair %d: %v\n", p.Index, err)
	}
}
