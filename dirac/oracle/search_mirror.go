package oracle

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// search_mirror.go plays one lockstep game with dirac/search choosing every
// seat (WithSearchSeats' policy shape), and at the first canonical pick of
// every gorge decision asks bots/azmcts the same position too: an honest
// redeal built from the seat's feed (searchseat.HonestRoot) rooted on the
// live gorge decision, then azmcts.Search with the per-decision seed it
// shares with the kernel. The two candidate lists are mapped to canonical
// actions and compared; the first difference stops the run and names the
// decision. The chosen candidate and the visits per candidate are recorded
// too: the two engines' world streams differ (dirac/search/doc.go), so at a
// small simulation budget they can part without a candidate difference.
//
// The gorge search is a flat Search, not Seat.DecideSearch: DecideSearch
// returns only the played intent, while the harness needs the whole
// candidate list (Result.Candidates). The options are the ones DecideSearch
// builds for this config: DecisionSeed on Decision.Ordinal, no root noise,
// no move sampling, the heuristic leaf (net nil).

// searchMirrorSims is the brief's simulation budget.
const searchMirrorSims = 8

// SearchMirrorMismatch is the first decision whose searches did not mirror.
type SearchMirrorMismatch struct {
	At     int   // canonical decision number
	Turn   int32 // gorge turn
	Kind   string
	Player uint8
	What   string // "candidates", "visits" or "choice"
	Gorge  []string
	Kernel []string
	Detail string
}

func (m *SearchMirrorMismatch) String() string {
	return fmt.Sprintf("decision %d turn %d %s p%d: %s: %s", m.At, m.Turn, m.Kind, m.Player, m.What, m.Detail)
}

// SearchMirrorResult is one mirrored pod game.
type SearchMirrorResult struct {
	Res      Result
	Mismatch *SearchMirrorMismatch
	// Choice and Visits are the first chosen-candidate and visit differences.
	// They are not failures by default: the two engines' world streams differ
	// (dirac/search/doc.go, "Where this does not match azmcts exactly"), so a
	// small simulation budget picks differently. compareVisits turns the
	// visit difference into Mismatch.
	Choice, Visits *SearchMirrorMismatch
	// Compared counts decisions both searches searched; Beneath counts
	// decisions whose gorge ordinal and kernel SeqNo had parted (the seed
	// input differs, so they are skipped); Refused counts honest roots or
	// redeals the gorge source refused.
	Compared, Beneath, Refused int
	// Kinds names the kind of every compared decision, in order.
	Kinds []string
}

// SearchMirrorPod plays pod m under seed with every seat searched on both
// engines, stopping at maxTurns. seatSeeds is the per-seat seed each search
// folds into its per-decision seed (the brief's 11, 22, 33, 44). The kernel
// search chooses the game's canonical actions; the gorge search is asked in
// shadow at the same decision. compareVisits also requires the visits per
// candidate to agree.
func SearchMirrorPod(m *Matchup, tbl *kop.Table, set cardset.Set, seed uint64, seatSeeds [MaxSeats]uint64, maxTurns int32, compareVisits bool) (SearchMirrorResult, error) {
	k, err := SetSide(set, tbl, Config{})(m, seed)
	if err != nil {
		return SearchMirrorResult{}, err
	}
	kern, ok := k.(*realSide)
	if !ok {
		CloseSide(k)
		return SearchMirrorResult{}, fmt.Errorf("search mirror needs the kernel side")
	}
	pol, err := newSearchMirrorPol(kern, set, seatSeeds, m.Config(seed), compareVisits)
	if err != nil {
		CloseSide(k)
		return SearchMirrorResult{}, err
	}
	rc := RunConfig{MaxDecisions: 20000, MaxTurns: maxTurns}
	res := Lockstep(m, seed, k, pol, rc)
	CloseSide(k)
	return SearchMirrorResult{
		Res: res, Mismatch: pol.mismatch, Choice: pol.choice, Visits: pol.visits,
		Compared: pol.compared, Beneath: pol.beneath, Refused: pol.refused, Kinds: pol.kinds,
	}, nil
}

type searchMirrorPol struct {
	k        *realSide
	set      cardset.Set
	seatSeed [MaxSeats]uint64
	setup    searchprobe.PublicGame
	seatCfg  azmcts.SeatConfig
	compareV bool

	gseats [MaxSeats]*azmcts.Seat
	feeds  [MaxSeats]*searchseat.Feed
	ksrch  [MaxSeats]*search.Searcher
	st     [MaxSeats]search.SeatState
	facts  *hbot.Facts
	brd    botpolicy.Board
	base   Policy

	seq      uint64
	queue    []Canon
	compared int
	beneath  int
	kinds    []string
	refused  int
	mismatch *SearchMirrorMismatch
	choice   *SearchMirrorMismatch
	visits   *SearchMirrorMismatch
}

func newSearchMirrorPol(k *realSide, set cardset.Set, seatSeeds [MaxSeats]uint64, rc rules.Config, compareVisits bool) (*searchMirrorPol, error) {
	p := &searchMirrorPol{
		k: k, set: set, seatSeed: seatSeeds, compareV: compareVisits,
		setup: searchprobe.PublicGame{
			Names: rc.Names, Decks: rc.Decks, Tokens: rc.Tokens, StartingLife: rc.StartingLife,
		},
		base: HeuristicPolicy(1),
	}
	p.seatCfg = azmcts.DefaultSeatConfig()
	p.seatCfg.World = azmcts.WorldRedeal
	p.seatCfg.Search.Sims = searchMirrorSims
	p.seatCfg.Search.Kinds = azmcts.FullKinds()
	p.seatCfg.Search.AutoPayment = true
	p.seatCfg.Search.OpponentNodes = true
	p.seatCfg.Search.OpponentMirror = true
	for i := 0; i < int(k.n); i++ {
		s, err := azmcts.NewSeat(seatSeeds[i], nil, p.seatCfg)
		if err != nil {
			return nil, fmt.Errorf("azmcts seat %d: %w", i, err)
		}
		p.gseats[i] = s
		p.feeds[i] = searchseat.NewFeed(state.PlayerID(i))
		p.feeds[i].Unchain()
	}
	p.facts = HBotFactsSet(k.m.reg, k.tbl, nil, set)
	return p, nil
}

func (p *searchMirrorPol) Pick(c *PickCtx) Canon {
	if c.G == nil {
		return p.base.Pick(c)
	}
	gd := c.G.Pending()
	if gd == nil {
		return p.base.Pick(c)
	}
	if gd.Seq != p.seq {
		p.seq = gd.Seq
		p.queue = p.queue[:0]
		// The feed's last frame must be this boundary: capture through every
		// live feed at every canonical decision, as the bench driver does.
		for _, f := range p.feeds {
			if f != nil && f.Live() {
				f.Observe(c.G.E)
			}
		}
		p.ask(c, gd)
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
	return p.base.Pick(c)
}

// ask runs both searches at the first canonical pick of gd and queues the
// kernel's answer. A decision whose ordinals have parted is skipped: the
// seed inputs differ, so the comparison would measure the counter gap, not
// the searches.
func (p *searchMirrorPol) ask(c *PickCtx, gd *decision.Decision) {
	r := p.k
	if int(gd.Player) >= int(r.n) || uint8(gd.Player) != r.d.Player {
		return
	}
	if gd.Ordinal != uint64(r.s.SeqNo()) {
		p.beneath++
		return
	}
	if gd.Kind == decision.KPriority {
		c.G.E.EnsurePaymentActions()
		if ed := c.G.E.Pending(); ed != nil && ed.Seq == gd.Seq {
			gd = ed
		}
	}
	// The kernel search drives the game; its result is the queue even when
	// only it searched.
	kres := p.kernelSearch(int(gd.Player))
	if kres.NActs > 0 {
		for _, a := range kres.Acts[:kres.NActs] {
			p.queue = append(p.queue, r.canon(a))
		}
	}
	gres, err := p.gorgeSearch(c, gd)
	if err != nil {
		p.refused++
		return
	}
	if gres.Stats.Searched == 0 || kres.Stats.Searched == 0 {
		// One engine posed a searched point the other did not: the candidate
		// lists cannot mirror.
		if gres.Stats.Searched != kres.Stats.Searched {
			// The counter gap: one engine poses a searched point the other
			// does not (a lowered X cast paid at priority, a manual payment
			// window). The seeds cannot mirror across it; count it and stop
			// comparing this game's remaining decisions.
			p.beneath++
			return
		}
		return
	}
	p.compared++
	p.kinds = append(p.kinds, string(gd.Kind))
	gc, gv, gchoice, ok := p.gorgeCands(c, gd, gres)
	if !ok {
		p.fail(c, gd, "candidates", nil, nil, "a gorge candidate has no canonical action")
		return
	}
	kc, kv, kchoice, ok := p.kernelCands(p.ksrch[int(gd.Player)], kres)
	if !ok {
		p.fail(c, gd, "candidates", nil, nil, "a kernel candidate has no canonical action")
		return
	}
	if len(gc) != len(kc) {
		p.fail(c, gd, "candidates", canonLists(gc), canonLists(kc),
			fmt.Sprintf("%d candidates vs %d", len(gc), len(kc)))
		return
	}
	gm, km := candMap(gc, gv), candMap(kc, kv)
	if len(gm) != len(km) {
		p.fail(c, gd, "candidates", canonLists(gc), canonLists(kc),
			fmt.Sprintf("%d distinct candidates vs %d", len(gm), len(km)))
		return
	}
	for _, k := range sortedCandKeys(gm) {
		if _, ok := km[k]; !ok {
			p.fail(c, gd, "candidates", canonLists(gc), canonLists(kc), "candidate sets differ")
			return
		}
	}
	if gchoice < 0 || gchoice >= len(gc) || kchoice < 0 || kchoice >= len(kc) ||
		candKey(gc[gchoice]) != candKey(kc[kchoice]) {
		if p.choice == nil {
			p.choice = p.note(c, gd, "choice", gc, kc,
				fmt.Sprintf("choice %s (visits %v) vs %s (visits %v)", candName(gc, gchoice), gv, candName(kc, kchoice), kv))
		}
	}
	for _, k := range sortedCandKeys(gm) {
		if km[k] != gm[k] {
			if p.visits == nil {
				p.visits = p.note(c, gd, "visits", gc, kc,
					fmt.Sprintf("candidate %s has %d visits, kernel %d", k, gm[k], km[k]))
			}
			if p.compareV {
				p.fail(c, gd, "visits", canonLists(gc), canonLists(kc),
					fmt.Sprintf("candidate %s has %d visits, kernel %d", k, gm[k], km[k]))
			}
			break
		}
	}
}

func (p *searchMirrorPol) fail(c *PickCtx, gd *decision.Decision, what string, gorge, kernel []string, detail string) {
	p.mismatch = p.note(c, gd, what, nil, nil, detail)
	p.mismatch.Gorge, p.mismatch.Kernel = gorge, kernel
	panic(p.mismatch)
}

func (p *searchMirrorPol) note(c *PickCtx, gd *decision.Decision, what string, gc, kc [][]Canon, detail string) *SearchMirrorMismatch {
	m := &SearchMirrorMismatch{
		At: c.Seq, Turn: c.G.E.G.Turn, Kind: string(gd.Kind), Player: uint8(gd.Player),
		What: what, Detail: detail,
	}
	if gc != nil {
		m.Gorge = canonLists(gc)
	}
	if kc != nil {
		m.Kernel = canonLists(kc)
	}
	return m
}

// kernelSearch runs the seat's kernel search at the kernel's pending
// decision.
func (p *searchMirrorPol) kernelSearch(seat int) *search.Result {
	return p.ksearcher(seat).SeatSearch(&p.k.s, &p.k.d, p.seatSeed[seat], &p.st[seat])
}

func (p *searchMirrorPol) ksearcher(seat int) *search.Searcher {
	if sr := p.ksrch[seat]; sr != nil {
		return sr
	}
	o := search.DefaultOptions()
	o.Sims = searchMirrorSims
	o.Kinds = search.FullKinds()
	o.OpponentNodes, o.OpponentMirror = true, true
	o.ClosedList = false
	o.PriorTopK = 0
	o.LazyPriors = false
	if err := o.Validate(); err != nil {
		panic(fmt.Sprintf("oracle: search mirror: %v", err))
	}
	if err := o.ValidateEval(search.HeuristicLeaf{}); err != nil {
		panic(fmt.Sprintf("oracle: search mirror: %v", err))
	}
	sr := search.NewSearcher(o, search.HeuristicLeaf{}, hbot.New(p.facts))
	p.ksrch[seat] = sr
	return sr
}

// gorgeSearch runs azmcts at gd on the honest root built from the seat's
// feed, with the options Seat.DecideSearch builds for this config.
func (p *searchMirrorPol) gorgeSearch(c *PickCtx, gd *decision.Decision) (azmcts.Result, error) {
	seat := int(gd.Player)
	e := c.G.E
	if gd.Kind == decision.KPriority {
		e.EnsurePaymentActions()
		if ed := e.Pending(); ed != nil && ed.Seq == gd.Seq {
			gd = ed
		}
	}
	b := botpolicy.BoardFromGameInto(e.G, e, gd.Player, &p.brd)
	botIn, err := p.gseats[seat].DecideBoard(context.Background(), b, *gd)
	if err != nil {
		return azmcts.Result{}, err
	}
	f := p.feeds[seat]
	setup := searchseat.SeatSetup(p.setup, p.gseats[seat], gd.Player)
	root, reason := searchseat.HonestRoot(setup, f, e, bots.RootSeed(p.seatSeed[seat], gd.Ordinal))
	if root == nil {
		return azmcts.Result{}, fmt.Errorf("honest root: %s", reason)
	}
	opts := p.seatCfg.Search
	opts.Seed = azmcts.DecisionSeed(p.seatSeed[seat], gd.Ordinal)
	opts.Noise, opts.Sample = false, false
	obs := searchprobe.NewCollector(gd.Player)
	in := azmcts.RedealInput{Setup: setup}
	if f != nil && f.Live() && f.Frames() > 0 {
		in.History = f.History()
		in.Base = searchprobe.RedealBase{Engine: root, Observer: f.Collector()}
		if known, kerr := f.Known(); kerr == nil {
			in.Known = known
		} else {
			in.Base = searchprobe.RedealBase{}
		}
	}
	src, err := azmcts.NewRedeal(in, obs, opts.Seed, p.seatCfg.Worlds)
	if err != nil {
		return azmcts.Result{}, err
	}
	rootArg := azmcts.Root{Engine: root, Decision: gd, Bot: botIn, Observer: obs}
	return azmcts.Search(context.Background(), rootArg, src, nil, opts)
}

// gorgeCands maps res.Candidates to canonical action sets, in candidate
// order. The native option index of a choice goes through the current
// canonical frame (fr.acts) or the priority vocabulary (fr.prio).
func (p *searchMirrorPol) gorgeCands(c *PickCtx, gd *decision.Decision, res azmcts.Result) ([][]Canon, []int, int, bool) {
	fr := c.G.fr
	if fr == nil || fr.seq != gd.Seq {
		return nil, nil, -1, false
	}
	out := make([][]Canon, len(res.Candidates))
	for ci, in := range res.Candidates {
		acts, ok := p.intentCands(c, gd, fr, in)
		if !ok {
			return nil, nil, -1, false
		}
		out[ci] = acts
	}
	visits := append([]int(nil), res.Visits...)
	return out, visits, res.Choice, true
}

func (p *searchMirrorPol) intentCands(c *PickCtx, gd *decision.Decision, fr *frame, in decision.Intent) ([]Canon, bool) {
	if in.Payment != nil || in.Announce != nil {
		id := ""
		if in.Payment != nil {
			id = in.Payment.ActionID
		} else {
			id = in.Announce.ActionID
		}
		for _, pa := range gd.PaymentActions {
			if pa.ID != id {
				continue
			}
			a, skip, _ := c.G.plannedCastAction(pa, c.G.InstRef)
			if skip || a == 0 {
				return nil, false
			}
			return []Canon{a}, true
		}
		return nil, false
	}
	out := make([]Canon, 0, len(in.Choices))
	for _, ch := range in.Choices {
		a, ok := p.nativeCanon(fr, gd, ch)
		if !ok {
			return nil, false
		}
		out = append(out, a)
	}
	return out, true
}

// nativeCanon is the canonical action of native option index idx.
func (p *searchMirrorPol) nativeCanon(fr *frame, gd *decision.Decision, idx int) (Canon, bool) {
	if gd.Kind == decision.KPriority {
		for _, e := range fr.prio {
			if e.opt == idx {
				return e.a, true
			}
		}
		return 0, false
	}
	pos := -1
	for i := range fr.d.Options {
		if fr.d.Options[i].Index == idx {
			pos = i
			break
		}
	}
	if pos >= 0 && pos < len(fr.acts) && fr.valid[pos] {
		return fr.acts[pos], true
	}
	return 0, false
}

func (p *searchMirrorPol) kernelCands(sr *search.Searcher, res *search.Result) ([][]Canon, []int, int, bool) {
	out := make([][]Canon, res.N)
	for i := 0; i < res.N; i++ {
		acts := sr.RootCand(i)
		cs := make([]Canon, 0, len(acts))
		for _, a := range acts {
			cs = append(cs, p.k.canon(dirac.Action(a)))
		}
		out[i] = cs
	}
	visits := make([]int, res.N)
	for i := 0; i < res.N; i++ {
		visits[i] = int(res.Visits[i])
	}
	return out, visits, res.Choice, true
}

// candMap keys a candidate by its canonical action set (sorted; a
// declaration's Done is not an action) and sums the visits of duplicate
// keys.
func candMap(cs [][]Canon, visits []int) map[string]int {
	out := make(map[string]int, len(cs))
	for i, acts := range cs {
		v := 1
		if i < len(visits) {
			v = visits[i]
		}
		out[candKey(acts)] += v
	}
	return out
}

func candKey(acts []Canon) string {
	x := append([]Canon(nil), acts...)
	dropDone(x)
	sortCanon(x)
	var b strings.Builder
	for _, a := range x {
		b.WriteString(strconv.FormatUint(uint64(a), 16))
		b.WriteByte(',')
	}
	return b.String()
}

func sortedCandKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func candName(cs [][]Canon, i int) string {
	if i < 0 || i >= len(cs) {
		return "?"
	}
	return strings.Join(showCanon(cs[i]), "+")
}

func canonLists(cs [][]Canon) []string {
	out := make([]string, len(cs))
	for i := range cs {
		out[i] = strings.Join(showCanon(cs[i]), "+")
	}
	return out
}

func showCanon(acts []Canon) []string {
	x := append([]Canon(nil), acts...)
	dropDone(x)
	sortCanon(x)
	return actionStrings(x)
}
