package oracle

import (
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

// searchseat.go plays chosen seats of a lockstep game with dirac/search. A
// search seat's decision is searched on the kernel's current state, and the
// result's action list (search.Result.Acts[:NActs]) is played as canonical
// actions through the side's translator, queued across the picks of one
// declaration as hbotParityPol queues them. The other seats keep the base
// policy. Lockstep still compares every decision, so one run is both a
// strong opponent and a lockstep check.

// SearchSeatConfig selects the search seats and the search they run.
type SearchSeatConfig struct {
	Seats uint8 // a bit per seat; 0 leaves the game as it is
	MaxN  bool  // search.Options.MaxN (else paranoid)
	Sims  int   // simulations per searched decision
}

// SearchSeatDefaultSims is the simulation budget kfuzz -search-sims defaults to.
const SearchSeatDefaultSims = 100

// WithSearchSeats returns mk and base wrapped so the seats of cfg.Seats are
// answered by dirac/search on the kernel's state, and every other seat by
// base. With no seat it returns mk and base unchanged. The factory and the
// policy share one game: build a pair for each game. The wrapped factory
// refuses a side that is not the dirac kernel side.
func WithSearchSeats(mk SideFactory, base Policy, cfg SearchSeatConfig, reg *cards.Registry, set cardset.Set) (SideFactory, Policy) {
	if cfg.Seats == 0 {
		return mk, base
	}
	p := &searchSeatPol{base: base, cfg: cfg, reg: reg, set: set, seq: ^uint64(0)}
	wrapped := func(m *Matchup, seed uint64) (Side, error) {
		side, err := mk(m, seed)
		if err != nil {
			return nil, err
		}
		r, ok := side.(*realSide)
		if !ok {
			CloseSide(side)
			return nil, errors.New("search seats need the dirac kernel side")
		}
		p.r, p.seed = r, seed
		p.srch, p.facts = [MaxSeats]*search.Searcher{}, nil
		p.st = [MaxSeats]search.SeatState{}
		p.queue = p.queue[:0]
		p.seq = ^uint64(0)
		return side, nil
	}
	return wrapped, p
}

// SearchedBy is the number of decisions each seat's search searched in the
// game p played (zero for any policy WithSearchSeats did not wrap).
func SearchedBy(p Policy) [MaxSeats]int {
	if sp, ok := p.(*searchSeatPol); ok {
		return sp.searched
	}
	return [MaxSeats]int{}
}

type searchSeatPol struct {
	base Policy
	cfg  SearchSeatConfig
	reg  *cards.Registry
	set  cardset.Set

	r     *realSide
	seed  uint64
	srch  [MaxSeats]*search.Searcher
	st    [MaxSeats]search.SeatState
	facts *hbot.Facts

	seq   uint64 // the gorge decision the queue answers
	queue []Canon

	searched  [MaxSeats]int
	fallbacks int
}

// PolicyFallbacks is the base policy's fallbacks and the search seats' picks
// the gorge-led decision did not offer.
func (p *searchSeatPol) PolicyFallbacks() int { return PolicyFallbacks(p.base) + p.fallbacks }

func (p *searchSeatPol) Pick(c *PickCtx) Canon {
	r := p.r
	if c.G == nil {
		panic("oracle: search seats need gorge to lead (PickCtx.G is nil in a kernel-led lockstep)")
	}
	if r == nil || r.d.N == 0 || int(r.d.Player) >= MaxSeats || p.cfg.Seats>>r.d.Player&1 == 0 {
		return p.base.Pick(c)
	}
	if gd := c.G.Pending(); gd != nil && gd.Seq != p.seq {
		p.seq, p.queue = gd.Seq, p.queue[:0]
	}
	// a per-target-priced cast ask closes once the affordable picks are spent
	// (AffordableTargetPolicy)
	if done, ok := affordableDone(c.D); ok {
		p.queue = p.queue[:0]
		return done
	}
	if len(p.queue) == 0 {
		p.search(int(r.d.Player))
	}
	for len(p.queue) > 0 {
		x := p.queue[0]
		p.queue = p.queue[1:]
		for _, o := range c.D.Options {
			if o == x {
				return x
			}
		}
	}
	p.fallbacks++
	return c.D.Options[0]
}

// search runs the seat's search at the kernel's pending decision and queues
// its answer as canonical actions.
func (p *searchSeatPol) search(seat int) {
	r := p.r
	sr := p.searcher(seat)
	res := sr.SeatSearch(&r.s, &r.d, SeatSeed(p.seed, seat), &p.st[seat])
	p.searched[seat] += res.Stats.Searched
	p.queue = p.queue[:0]
	for _, a := range res.Acts[:res.NActs] {
		p.queue = append(p.queue, r.canon(a))
	}
}

// searcher is seat's searcher, built on its first decision: the kernel
// search bot over the side's table, the heuristic leaf, and the options
// kfuzz -search and -search-sims name.
func (p *searchSeatPol) searcher(seat int) *search.Searcher {
	if sr := p.srch[seat]; sr != nil {
		return sr
	}
	o := search.DefaultOptions()
	o.Sims, o.MaxN = p.cfg.Sims, p.cfg.MaxN
	if err := o.Validate(); err != nil {
		panic(fmt.Sprintf("oracle: search seats: %v", err))
	}
	if p.facts == nil {
		p.facts = HBotFactsSet(p.reg, p.r.tbl, nil, p.set)
	}
	bot := hbot.New(p.facts)
	sr := search.NewSearcher(o, search.HeuristicLeaf{}, bot)
	p.srch[seat] = sr
	return sr
}
