package lab

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// Policy answers a seat's decisions. Pick returns one of d.Options[:d.N]; s is
// the position d is posed in, which a heuristic or a search reads. A dirac.Picker
// (Pick(d) only) joins through FromPicker.
type Policy interface {
	Pick(s *dirac.State, d *dirac.Decision) dirac.Action
}

// Starter is a Policy that wants to know a game is starting: Play calls Start
// before each game, once per seat it plays, so a policy resets what it carries
// between games and rebinds to the game's table.
type Starter interface {
	Start(g *Game, seat int)
}

// Cloner is a Policy that can be copied. Batch gives each worker its own copy,
// since a policy holds per-game state; a policy that is not a Cloner runs in a
// Batch of one worker only.
type Cloner interface {
	Clone() Policy
}

// FromPicker adapts a dirac.Picker, such as a *dirac.RandomPolicy.
func FromPicker(p dirac.Picker) Policy { return pickerPolicy{p} }

type pickerPolicy struct{ p dirac.Picker }

func (pp pickerPolicy) Pick(_ *dirac.State, d *dirac.Decision) dirac.Action { return pp.p.Pick(d) }

// splitmix is SplitMix64's output for state x.
func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// seatSeed is the seed a seat's own stream starts from in a game.
func seatSeed(policy, game uint64, seat int) uint64 {
	return splitmix(policy ^ splitmix(game^uint64(seat+1)*0x9e3779b97f4a7c15))
}

// Random picks uniformly. Each game it restarts its stream from seed, the
// game's seed and its seat, so a game's result depends on those alone.
func Random(seed uint64) Policy { return &randomPolicy{seed: seed} }

type randomPolicy struct {
	seed uint64
	rng  dirac.RandomPolicy
}

func (r *randomPolicy) Start(g *Game, seat int) {
	r.rng = dirac.NewRandomPolicy(seatSeed(r.seed, g.Seed(), seat))
}

func (r *randomPolicy) Pick(_ *dirac.State, d *dirac.Decision) dirac.Action { return r.rng.Pick(d) }
func (r *randomPolicy) Clone() Policy                                       { return &randomPolicy{seed: r.seed} }

// HBot is dirac/search/hbot, the port of gorge's auto-pay heuristic bot. Its
// facts come from the card table alone (hbot.NewFacts), which is coarser than
// the facts dplay's hbot reads from gorge's compiled cards: games can differ
// from dplay's. HBotWith takes another source of facts.
func HBot() Policy { return HBotWith(hbot.NewFacts) }

// HBotWith is HBot over facts built by mk for each game's table, once per table.
func HBotWith(mk func(*kop.Table) *hbot.Facts) Policy { return &hbotPolicy{mk: mk} }

type hbotPolicy struct {
	mk  func(*kop.Table) *hbot.Facts
	tbl *kop.Table
	bot *hbot.Bot
}

func (h *hbotPolicy) Start(g *Game, _ int) {
	if h.tbl != g.su.tbl {
		h.tbl, h.bot = g.su.tbl, hbot.New(h.mk(g.su.tbl))
	}
}

func (h *hbotPolicy) Pick(s *dirac.State, d *dirac.Decision) dirac.Action {
	return d.Options[h.bot.Pick(s, d)]
}

func (h *hbotPolicy) Clone() Policy { return &hbotPolicy{mk: h.mk} }

// SearchOptions is dirac/search's DefaultOptions with sims simulations per
// searched decision: the option set oracle's search seats run.
func SearchOptions(sims int) search.Options {
	o := search.DefaultOptions()
	o.Sims = sims
	return o
}

// Search is dirac/search's PUCT (a Searcher over the heuristic leaf and hbot's
// priors, as dirac/oracle's search seats build it) answering every decision of
// its seat. It panics on an invalid opts.
func Search(opts search.Options) Policy {
	if err := opts.Validate(); err != nil {
		panic(fmt.Sprintf("lab: Search: %v", err))
	}
	if err := opts.ValidateEval(search.HeuristicLeaf{}); err != nil {
		panic(fmt.Sprintf("lab: Search: %v", err))
	}
	return &searchPolicy{opts: opts}
}

type searchPolicy struct {
	opts  search.Options
	tbl   *kop.Table
	sr    *search.Searcher
	st    search.SeatState
	seed  uint64
	queue []dirac.Action // the rest of the answer to a declaration
}

func (p *searchPolicy) Start(g *Game, seat int) {
	if p.tbl != g.su.tbl {
		bot := hbot.New(hbot.NewFacts(g.su.tbl))
		p.tbl, p.sr = g.su.tbl, search.NewSearcher(p.opts, search.HeuristicLeaf{}, bot)
	}
	p.st, p.queue = search.SeatState{}, p.queue[:0]
	p.seed = seatSeed(0, g.Seed(), seat)
}

func (p *searchPolicy) Pick(s *dirac.State, d *dirac.Decision) dirac.Action {
	if a, ok := p.next(d); ok {
		return a
	}
	res := p.sr.SeatSearch(s, d, p.seed, &p.st)
	p.queue = append(p.queue[:0], res.Acts[:res.NActs]...)
	if a, ok := p.next(d); ok {
		return a
	}
	return d.Options[0]
}

// next pops the queued answers until one is offered at d.
func (p *searchPolicy) next(d *dirac.Decision) (dirac.Action, bool) {
	for len(p.queue) > 0 {
		a := p.queue[0]
		p.queue = p.queue[1:]
		if offered(d, a) {
			return a, true
		}
	}
	return 0, false
}

func offered(d *dirac.Decision, a dirac.Action) bool {
	for _, o := range d.Options[:d.N] {
		if o == a {
			return true
		}
	}
	return false
}

func (p *searchPolicy) Clone() Policy { return &searchPolicy{opts: p.opts} }
