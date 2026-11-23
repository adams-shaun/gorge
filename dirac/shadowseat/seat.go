// Package shadowseat is a gorge seat.Seat that plays by searching on dirac.
//
// At each priority, declare-attackers and declare-blockers decision the seat
// stages the view it was handed (shadow.Layout.Deal, shadow.StageFromView)
// into a dirac.State, searches that State with dirac/search (the heuristic
// leaf and hbot priors, PIMC over Config.Worlds cached deals), and maps the
// dirac actions the search chose back onto the gorge decision's options. A
// priority answer that casts or activates keeps the staged State with the
// answer applied as a plan, so the target decision that follows is answered
// from the same position. Every other decision, and every position the seat
// cannot stage or map, goes to Config.Fallback, and the reason is counted.
//
// The open-list assumption: the seat knows every seat's deck list
// (Config.Lists). The staged root holds only the Layout's deal of the unseen
// cards, never a hidden zone of the game the view came from, and the search
// redeals what the seat cannot see at each simulation (a world is a function
// of the view and the lists). A closed list, an unknown opponent deck, would
// need a search.Belief and is not supported. search.Options.Clairvoyant is
// never set.
//
// The fairness rule: the seat reads gorge only through the view.View and
// decision.Decision a host hands any seat. gorge itself is the host, so
// every answer is checked by gorge's own Submit.
package shadowseat

import (
	"context"
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// Defaults of Config.
const (
	DefaultSims   = 64
	DefaultWorlds = 4
	// maxCardRefusals bounds Stats.CardRefusals.
	maxCardRefusals = 200
)

// Config is one Seat's configuration.
type Config struct {
	// Tbl is the dirac card table the game is played over.
	Tbl *kop.Table
	// Set is the card set Tbl was generated from; nil for a hand-written
	// table such as dirac/vanilla. Reg is then required.
	Set cardset.Set
	Reg *cards.Registry
	// Lists is every seat's deck, one card name per copy, by seat.
	Lists [][]string
	// Sims is the simulations per search (default 64); Worlds is
	// search.Options.CachedWorlds (default 4). Sims must be a multiple of
	// Worlds.
	Sims, Worlds int
	// Seed derives every search's seed.
	Seed uint64
	// Fallback answers the decisions the seat does not search. Required.
	Fallback seat.Seat
}

// Stats counts what a Seat did. Decisions is every Decide call; Searched the
// answers a search produced (Continued of them from a plan); Fallbacks the
// calls handed to Config.Fallback, counted by reason in Reasons:
//
//	stage/<reason>        the view is not stageable (dirac.ErrUnstageable.Reason)
//	kind/<kind>           a gorge decision kind the seat does not stage, no plan held
//	continuation/<kind>   a plan was held but this kind cannot continue it
//	decision              the staged decision is not the gorge one's kind or player
//	unmapped/<action>     a dirac action with no gorge option
//
// CardRefusals counts the dirac.ErrUnstageable details of reason "card", at
// most 200 distinct keys.
type Stats struct {
	Decisions, Searched, Continued, Fallbacks int
	Reasons                                   map[string]int
	CardRefusals                              map[string]int
}

// Seat is one player of one game. It is not safe for concurrent use.
type Seat struct {
	cfg    Config
	layout *shadow.Layout
	dcfg   dirac.Config
	sr     *search.Searcher
	ss     search.SeatState
	tr     canon.Translator // the table binding; S, D and KP are set per position
	plan   plan
	stats  Stats
}

// New builds the Seat of cfg.
func New(cfg Config) (*Seat, error) {
	if cfg.Tbl == nil {
		return nil, errors.New("shadowseat: no table")
	}
	if cfg.Fallback == nil {
		return nil, errors.New("shadowseat: no fallback seat")
	}
	if cfg.Set != nil && cfg.Reg == nil {
		return nil, errors.New("shadowseat: a Set needs its Reg")
	}
	if cfg.Sims == 0 {
		cfg.Sims = DefaultSims
	}
	if cfg.Worlds == 0 {
		cfg.Worlds = DefaultWorlds
	}
	if cfg.Sims < 0 || cfg.Worlds < 0 || cfg.Sims%cfg.Worlds != 0 {
		return nil, fmt.Errorf("shadowseat: Sims %d must be a positive multiple of Worlds %d", cfg.Sims, cfg.Worlds)
	}
	layout, err := shadow.NewLayout(cfg.Tbl, cfg.Lists)
	if err != nil {
		return nil, err
	}
	opts := search.DefaultOptions()
	opts.Sims, opts.CachedWorlds, opts.Seed = cfg.Sims, cfg.Worlds, cfg.Seed
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("shadowseat: %w", err)
	}
	if err := opts.ValidateEval(search.HeuristicLeaf{}); err != nil {
		return nil, fmt.Errorf("shadowseat: %w", err)
	}
	s := &Seat{
		cfg:    cfg,
		layout: layout,
		dcfg:   dirac.LockstepConfig(dirac.SurfaceFull),
		sr:     search.NewSearcher(opts, search.HeuristicLeaf{}, hbot.New(hbot.NewFacts(cfg.Tbl))),
		plan:   newPlan(),
		stats:  Stats{Reasons: map[string]int{}, CardRefusals: map[string]int{}},
	}
	s.dcfg.NoElide = true // dirac poses gorge's decisions one for one
	s.tr.Tbl = cfg.Tbl
	if cfg.Set != nil {
		s.tr.BindSet(cfg.Set, cfg.Tbl, cfg.Reg)
	}
	return s, nil
}

// WantsPaymentActions opts into seat.PaymentPlanConsumer: casts map onto
// Decision.PaymentActions.
func (s *Seat) WantsPaymentActions() bool { return true }

// Stats returns a copy of the seat's counters.
func (s *Seat) Stats() Stats {
	out := s.stats
	out.Reasons = make(map[string]int, len(s.stats.Reasons))
	for k, v := range s.stats.Reasons {
		out.Reasons[k] = v
	}
	out.CardRefusals = make(map[string]int, len(s.stats.CardRefusals))
	for k, v := range s.stats.CardRefusals {
		out.CardRefusals[k] = v
	}
	return out
}

// Decide answers d from a search over the staged view, or from the fallback.
func (s *Seat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	s.stats.Decisions++
	in, reason := s.decide(&v, &d)
	if reason == "" {
		s.stats.Searched++
		return in, nil
	}
	s.plan.clear()
	s.stats.Fallbacks++
	s.stats.Reasons[reason]++
	return s.cfg.Fallback.Decide(ctx, v, d)
}

// decide returns the searched answer, or "" and the reason it has none.
func (s *Seat) decide(v *view.View, d *decision.Decision) (decision.Intent, string) {
	switch d.Kind {
	case decision.KPriority, decision.KAttackers, decision.KBlockers:
		return s.decideStaged(v, d)
	case decision.KTarget:
		if s.plan.held {
			return s.decideContinued(v, d)
		}
	default:
		if s.plan.held {
			return decision.Intent{}, "continuation/" + string(d.Kind)
		}
	}
	return decision.Intent{}, "kind/" + string(d.Kind)
}

// seatSeed is the search seed of gorge decision d.
func (s *Seat) seatSeed(d *decision.Decision) uint64 { return search.DecisionSeed(s.cfg.Seed, d.Seq) }

// staged is a position the seat staged from a view, stepped to its decision.
type staged struct {
	st  *dirac.State
	num *shadow.Numbering
	dd  dirac.Decision
	kp  dirac.Projection
	tr  canon.Translator
}

// stage stages v and steps it to its decision. reason is "" on success.
func (s *Seat) stage(v *view.View, d *decision.Decision) (*staged, string) {
	seed := s.seatSeed(d)
	in, num, err := s.layout.Deal(v, s.dcfg, seed)
	var st *dirac.State
	if err == nil {
		st, err = shadow.StageFromView(in)
	}
	if err != nil {
		return nil, s.stageReason(err)
	}
	x := &staged{st: st, num: num}
	st.StepInto(&x.dd)
	if want := diracKind(d.Kind); x.dd.Kind != want || int(x.dd.Player) != int(v.Viewer) || int(d.Player) != int(v.Viewer) {
		return nil, "decision"
	}
	x.bind(s)
	return x, ""
}

// bind points x's translator at its State and decision.
func (x *staged) bind(s *Seat) {
	x.tr = s.tr
	x.st.Project(&x.kp)
	x.tr.S, x.tr.D, x.tr.KP = x.st, &x.dd, &x.kp
}

// stageReason is the reason key of a Deal or StageFromView error, counting a
// card refusal's detail.
func (s *Seat) stageReason(err error) string {
	var u dirac.ErrUnstageable
	if !errors.As(err, &u) {
		return "stage/error"
	}
	if u.Reason == dirac.ReasonCard {
		if _, ok := s.stats.CardRefusals[u.Detail]; ok || len(s.stats.CardRefusals) < maxCardRefusals {
			s.stats.CardRefusals[u.Detail]++
		}
	}
	return "stage/" + u.Reason
}

// diracKind is the dirac decision kind posing gorge kind k.
func diracKind(k decision.Kind) dirac.DecisionKind {
	switch k {
	case decision.KPriority:
		return dirac.DkPriority
	case decision.KAttackers:
		return dirac.DkAttackers
	case decision.KBlockers:
		return dirac.DkBlockers
	case decision.KTarget:
		return dirac.DkTarget
	}
	return dirac.DkNone
}

// decideStaged is a priority, attackers or blockers decision.
func (s *Seat) decideStaged(v *view.View, d *decision.Decision) (decision.Intent, string) {
	x, reason := s.stage(v, d)
	if reason != "" {
		return decision.Intent{}, reason
	}
	res := s.sr.SeatSearch(x.st, &x.dd, s.seatSeed(d), &s.ss)
	if res.NActs == 0 {
		return decision.Intent{}, "unmapped/none"
	}
	acts := res.Acts[:res.NActs]
	ans, _ := gorgeAnswers(v, d, x.num)
	var picks []answer
	if d.Kind == decision.KPriority {
		a, reason := mapPick(&x.tr, ans, acts[0])
		if reason != "" {
			return decision.Intent{}, reason
		}
		picks = append(picks, a)
	} else if picks, reason = mapPicks(&x.tr, ans, acts); reason != "" {
		return decision.Intent{}, reason
	}
	if d.Kind == decision.KPriority && continuesPlan(acts[0]) {
		s.plan.start(x.st, x.num, acts[0])
	} else {
		s.plan.clear()
	}
	return intentOf(d, picks), ""
}

// decideContinued is a target decision answered from the held plan. A
// decision that wants more picks than one search returns is searched again
// at the plan the first picks left.
func (s *Seat) decideContinued(v *view.View, d *decision.Decision) (decision.Intent, string) {
	p := &s.plan
	ans, _ := gorgeAnswers(v, d, p.num)
	var picks []answer
	for round := 0; round < maxRounds; round++ {
		var x staged
		p.st.StepInto(&x.dd)
		if x.dd.Kind != dirac.DkTarget || int(x.dd.Player) != int(v.Viewer) || int(d.Player) != int(v.Viewer) {
			return decision.Intent{}, "decision"
		}
		x.st = p.st
		x.bind(s)
		res := s.sr.SeatSearch(p.st, &x.dd, s.seatSeed(d), &s.ss)
		if res.NActs == 0 {
			return decision.Intent{}, "unmapped/none"
		}
		acts := res.Acts[:res.NActs]
		got, reason := mapPicks(&x.tr, ans, acts)
		if reason != "" {
			return decision.Intent{}, reason
		}
		picks = append(picks, got...)
		p.apply(acts)
		if len(picks) >= d.Min {
			break
		}
	}
	if len(picks) < d.Min {
		return decision.Intent{}, "unmapped/short"
	}
	s.stats.Continued++
	return intentOf(d, picks), ""
}

// maxRounds bounds the searches one continued target decision takes.
const maxRounds = 8
