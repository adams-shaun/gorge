package oracle

import (
	"context"
	"fmt"
	"slices"
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
// shares with the kernel. Each difference is recorded, in game order, under a
// class: the candidate lists (mapped to canonical actions), the hidden zones
// of the two root worlds, the chosen candidate and the visits per candidate.
// The harness stops at none of them: the caller decides which classes it
// tolerates.
//
// The gorge search is a flat Search, not Seat.DecideSearch: DecideSearch
// returns only the played intent, while the harness needs the whole
// candidate list (Result.Candidates). The options are the ones DecideSearch
// builds for this config: DecisionSeed on Decision.Ordinal, no root noise,
// no move sampling, the heuristic leaf (net nil).

// searchMirrorSims is the simulation budget the mirror is measured at.
const searchMirrorSims = 8

// searchMirrorWideLimit is the candidate limit two full, differing lists are
// enumerated again at: the most candidates a kernel point holds.
const searchMirrorWideLimit = search.MaxCands

// The kinds of difference a SearchMirrorMismatch names in What.
const (
	// MirrorOrdinal: gorge's Decision.Ordinal and the kernel's SeqNo have
	// parted, so the seed inputs differ and the decision is not searched.
	MirrorOrdinal = "ordinal"
	// MirrorCandidates: the two candidate lists differ; Shape says how.
	MirrorCandidates = "candidates"
	// MirrorTruth: the true hidden zones of the two engines read differently,
	// so the root-world comparison cannot be trusted at this decision.
	MirrorTruth = "truth"
	// MirrorRootPool: gorge's honest root and the kernel's first redealt
	// world disagree on something no random stream decides: a hidden zone's
	// size, the cards a player's hidden zones hold between them, or the
	// searching player's own hand.
	MirrorRootPool = "root-pool"
	// MirrorRootWorld: the two root worlds hold the same cards, dealt to
	// different places.
	MirrorRootWorld = "root-world"
	// MirrorChoice and MirrorVisits: equal candidate lists, a different chosen
	// candidate or different visits on a candidate.
	MirrorChoice = "choice"
	MirrorVisits = "visits"
)

// SearchMirrorMismatch is one difference between the two searches.
type SearchMirrorMismatch struct {
	At     int   // canonical decision number
	Turn   int32 // gorge turn
	Kind   string
	Player uint8
	What   string // one of the Mirror constants
	// Shape qualifies MirrorCandidates: "gorge-more" or "kernel-more" (one
	// list holds the other's), "differ" (each has a candidate the other has
	// not), "limit-cut" (both lists were full at the candidate limit and are
	// equal when enumerated again without that cut), "multi-select" (gorge
	// offers sets of options where the kernel, which poses a multi-select
	// pick by pick, offers single ones), "gorge-only" or "kernel-only" (only
	// that engine searched the decision), "unmapped" (a candidate with no
	// canonical action).
	Shape  string
	Gorge  []string
	Kernel []string
	Detail string
}

// Class keys a difference by what differs, never by the game it showed in:
// "candidates/<shape>" for the two enumerator shapes that are the same at any
// decision kind, "candidates/<decision kind>/<shape>" for any other
// candidate difference, else What.
func (m *SearchMirrorMismatch) Class() string {
	switch {
	case m.What != MirrorCandidates:
		return m.What
	case m.Shape == "limit-cut" || m.Shape == "multi-select":
		return m.What + "/" + m.Shape
	}
	return m.What + "/" + m.Kind + "/" + m.Shape
}

func (m *SearchMirrorMismatch) String() string {
	return fmt.Sprintf("decision %d turn %d %s p%d: %s: %s\n  gorge:  %v\n  kernel: %v",
		m.At, m.Turn, m.Kind, m.Player, m.Class(), m.Detail, m.Gorge, m.Kernel)
}

// SearchMirrorResult is one mirrored pod game.
type SearchMirrorResult struct {
	Res Result
	// Mismatches are every difference, in game order.
	Mismatches []SearchMirrorMismatch
	// Compared counts decisions both searches searched; Worlds the root worlds
	// compared among them; Refused honest roots or redeals the gorge source
	// refused.
	Compared, Worlds, Refused int
	// Kinds names the kind of every compared decision, in order.
	Kinds []string
}

// SearchMirrorPod plays pod m under seed with every seat searched on both
// engines, stopping at maxTurns. seatSeeds is the per-seat seed each search
// folds into its per-decision seed (11, 22, 33 and 44 in the test). The kernel
// search chooses the game's canonical actions; the gorge search is asked in
// shadow at the same decision. The run holds the two decision counters to
// OrdinalCheck, so a parting no answer beneath the canonical surface explains
// is a lockstep divergence.
func SearchMirrorPod(m *Matchup, tbl *kop.Table, set cardset.Set, seed uint64, seatSeeds [MaxSeats]uint64, maxTurns int32) (SearchMirrorResult, error) {
	rc := RunConfig{MaxDecisions: 20000, MaxTurns: maxTurns, Check: OrdinalCheck(nil)}
	k, err := SetSide(set, tbl, rc.Config)(m, seed)
	if err != nil {
		return SearchMirrorResult{}, err
	}
	defer CloseSide(k)
	kern, ok := k.(*realSide)
	if !ok {
		return SearchMirrorResult{}, fmt.Errorf("search mirror needs the kernel side")
	}
	pol, err := newSearchMirrorPol(kern, set, seatSeeds, m.Config(seed))
	if err != nil {
		return SearchMirrorResult{}, err
	}
	res := Lockstep(m, seed, k, pol, rc)
	return SearchMirrorResult{
		Res: res, Mismatches: pol.mismatches,
		Compared: pol.compared, Worlds: pol.worlds, Refused: pol.refused, Kinds: pol.kinds,
	}, nil
}

type searchMirrorPol struct {
	k        *realSide
	set      cardset.Set
	seatSeed [MaxSeats]uint64
	setup    searchprobe.PublicGame
	seatCfg  azmcts.SeatConfig

	gseats [MaxSeats]*azmcts.Seat
	feeds  [MaxSeats]*searchseat.Feed
	ksrch  [MaxSeats]*search.Searcher
	st     [MaxSeats]search.SeatState
	facts  *hbot.Facts
	brd    botpolicy.Board
	base   Policy
	world  dirac.State // the kernel's first redealt world

	seq        uint64
	queue      []Canon
	compared   int
	worlds     int
	kinds      []string
	refused    int
	mismatches []SearchMirrorMismatch
}

func newSearchMirrorPol(k *realSide, set cardset.Set, seatSeeds [MaxSeats]uint64, rc rules.Config) (*searchMirrorPol, error) {
	p := &searchMirrorPol{
		k: k, set: set, seatSeed: seatSeeds,
		setup: searchprobe.PublicGame{
			Names: rc.Names, Decks: rc.Decks, Tokens: rc.Tokens, StartingLife: rc.StartingLife,
		},
		base: HeuristicPolicy(1),
	}
	p.seatCfg = azmcts.DefaultSeatConfig()
	p.seatCfg.World = azmcts.WorldRedeal
	p.seatCfg.Search.Sims = searchMirrorSims
	p.seatCfg.Search.Kinds = azmcts.FullKinds()
	// The kernel's kinds search the command-zone choice; azmcts needs it
	// named on top of FullKinds.
	p.seatCfg.Search.Kinds.CommanderZone = true
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
// kernel's answer. A decision whose ordinals have parted is recorded and not
// searched on gorge: the seed inputs differ, so the comparison would measure
// the counter gap, not the searches. Later decisions are still compared where
// the counters agree.
func (p *searchMirrorPol) ask(c *PickCtx, gd *decision.Decision) {
	r := p.k
	if int(gd.Player) >= int(r.n) || uint8(gd.Player) != r.d.Player {
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
	if gd.Ordinal != uint64(r.s.SeqNo()) {
		p.record(c, gd, MirrorOrdinal, "", nil, nil,
			fmt.Sprintf("gorge Ordinal %d, kernel SeqNo %d", gd.Ordinal, r.s.SeqNo()))
		return
	}
	bot, err := p.gorgeBot(c, gd)
	if err != nil {
		p.refused++
		return
	}
	gres, root, err := p.gorgeSearch(c, gd, bot, p.seatCfg.Search.Limit)
	if err != nil {
		p.refused++
		return
	}
	gs, ks := gres.Stats.Searched != 0, kres.Stats.Searched != 0
	if !gs && !ks {
		return
	}
	if gs != ks {
		// The counters agree here, so both engines pose this decision: one
		// of them found a choice to search and the other did not.
		shape := "kernel-only"
		var gc, kc [][]Canon
		if gs {
			shape = "gorge-only"
			gc, _, _, _ = p.gorgeCands(c, gd, gres)
		} else {
			kc, _, _, _ = p.kernelCands(p.ksrch[int(gd.Player)], kres)
		}
		p.record(c, gd, MirrorCandidates, shape, gc, kc, "one engine searched the decision")
		return
	}
	p.compared++
	p.kinds = append(p.kinds, string(gd.Kind))
	p.compareWorlds(c, gd, root)
	p.compareResults(c, gd, bot, gres, kres)
}

// compareWorlds holds the hidden zones gorge's honest root was dealt (root)
// against the first world the kernel's search deals for the same decision.
// The true zones of the two engines are compared first: if they read
// differently, the extraction or the lockstep is wrong and a world difference
// would say nothing.
func (p *searchMirrorPol) compareWorlds(c *PickCtx, gd *decision.Decision, root mirrorZones) {
	n := int(p.k.n)
	truth, ktruth := gorgeZones(c.G.E, n), p.kernelZones(&p.k.s)
	if d := truth.diff(&ktruth, n); d != "" {
		p.record(c, gd, MirrorTruth, "", nil, nil, d)
		return
	}
	p.ksrch[int(gd.Player)].RootWorld(&p.world, 0)
	kworld := p.kernelZones(&p.world)
	p.worlds++
	if d := root.poolDiff(&kworld, n, int(gd.Player)); d != "" {
		p.record(c, gd, MirrorRootPool, "", nil, nil, d)
	} else if d := root.diff(&kworld, n); d != "" {
		p.record(c, gd, MirrorRootWorld, "", nil, nil, d)
	}
}

// compareResults records how the two searches' candidates differ, or, over
// equal candidate lists, how their choice and visits do.
func (p *searchMirrorPol) compareResults(c *PickCtx, gd *decision.Decision, bot decision.Intent, gres azmcts.Result, kres *search.Result) {
	gc, gv, gchoice, gok := p.gorgeCands(c, gd, gres)
	kc, kv, kchoice, kok := p.kernelCands(p.ksrch[int(gd.Player)], kres)
	if !gok || !kok {
		p.record(c, gd, MirrorCandidates, "unmapped", gc, kc, "a candidate has no canonical action")
		return
	}
	if shape := candShape(gc, kc); shape != "" {
		detail := fmt.Sprintf("%d candidates vs %d", len(gc), len(kc))
		limit := p.seatCfg.Search.Limit
		switch {
		case multiSelect(gd, gc, kc):
			shape = "multi-select"
		case shape == "differ" && len(gc) == limit && len(kc) == limit:
			shape, detail = p.uncut(c, gd, bot)
		}
		p.record(c, gd, MirrorCandidates, shape, gc, kc, detail)
		return
	}
	if gchoice < 0 || gchoice >= len(gc) || kchoice < 0 || kchoice >= len(kc) ||
		candKey(gc[gchoice]) != candKey(kc[kchoice]) {
		p.record(c, gd, MirrorChoice, "", gc, kc,
			fmt.Sprintf("choice %s (visits %v) vs %s (visits %v)", candName(gc, gchoice), gv, candName(kc, kchoice), kv))
	}
	gm, km := candMap(gc, gv), candMap(kc, kv)
	for _, k := range sortedCandKeys(gm) {
		if km[k] != gm[k] {
			p.record(c, gd, MirrorVisits, "", gc, kc,
				fmt.Sprintf("candidate %s has %d visits, kernel %d", k, gm[k], km[k]))
			break
		}
	}
}

// multiSelect reports a multi-select pick the two engines pose differently:
// some gorge candidate is a set of options and no kernel candidate is.
// Declarations and priority are whole answers on both sides.
func multiSelect(gd *decision.Decision, gc, kc [][]Canon) bool {
	switch gd.Kind {
	case decision.KAttackers, decision.KBlockers, decision.KPriority:
		return false
	}
	set := func(c []Canon) bool { return len(dropDone(c)) > 1 }
	return gd.Max > 1 && slices.ContainsFunc(gc, set) && !slices.ContainsFunc(kc, set)
}

// uncut enumerates both engines' candidates of the decision again at
// searchMirrorWideLimit, for two full lists that differ: "limit-cut" when the
// wider lists are equal (only the order under the cut parted them), else the
// wider lists' own shape, which no cut explains.
func (p *searchMirrorPol) uncut(c *PickCtx, gd *decision.Decision, bot decision.Intent) (shape, detail string) {
	seat := int(gd.Player)
	gres, _, err := p.gorgeSearch(c, gd, bot, searchMirrorWideLimit)
	if err != nil {
		return "differ", "gorge could not enumerate again: " + err.Error()
	}
	wide := p.newKSearcher(searchMirrorWideLimit)
	kres := wide.Search(&p.k.s, p.seatSeed[seat])
	gc, _, _, gok := p.gorgeCands(c, gd, gres)
	kc, _, _, kok := p.kernelCands(wide, kres)
	if !gok || !kok {
		return "unmapped", "a candidate has no canonical action at the wide limit"
	}
	detail = fmt.Sprintf("%d candidates vs %d at limit %d: %v vs %v",
		len(gc), len(kc), searchMirrorWideLimit, canonLists(gc), canonLists(kc))
	if shape = candShape(gc, kc); shape == "" {
		shape = "limit-cut"
	}
	return shape, detail
}

func (p *searchMirrorPol) record(c *PickCtx, gd *decision.Decision, what, shape string, gc, kc [][]Canon, detail string) {
	m := SearchMirrorMismatch{
		At: c.Seq, Turn: c.G.E.G.Turn, Kind: string(gd.Kind), Player: uint8(gd.Player),
		What: what, Shape: shape, Detail: detail,
	}
	if gc != nil {
		m.Gorge = canonLists(gc)
	}
	if kc != nil {
		m.Kernel = canonLists(kc)
	}
	p.mismatches = append(p.mismatches, m)
}

// mirrorZones is each player's hidden cards by name: the hand sorted (its
// order means nothing), the library in order.
type mirrorZones struct {
	hand, lib [MaxSeats][]string
}

// diff names the first zone of the first n players that differs, or "".
func (z *mirrorZones) diff(o *mirrorZones, n int) string {
	for s := 0; s < n; s++ {
		if !slices.Equal(z.hand[s], o.hand[s]) {
			return fmt.Sprintf("p%d hand: gorge %v, kernel %v", s, z.hand[s], o.hand[s])
		}
		if !slices.Equal(z.lib[s], o.lib[s]) {
			i := 0
			for i < len(z.lib[s]) && i < len(o.lib[s]) && z.lib[s][i] == o.lib[s][i] {
				i++
			}
			return fmt.Sprintf("p%d library (%d vs %d cards) first differs at %d", s, len(z.lib[s]), len(o.lib[s]), i)
		}
	}
	return ""
}

// poolDiff names the first difference no deal can make between the root
// worlds of searcher's decision, or "".
func (z *mirrorZones) poolDiff(o *mirrorZones, n, searcher int) string {
	for s := 0; s < n; s++ {
		if len(z.hand[s]) != len(o.hand[s]) || len(z.lib[s]) != len(o.lib[s]) {
			return fmt.Sprintf("p%d zone sizes: gorge hand %d library %d, kernel hand %d library %d",
				s, len(z.hand[s]), len(z.lib[s]), len(o.hand[s]), len(o.lib[s]))
		}
		g := slices.Concat(z.hand[s], z.lib[s])
		k := slices.Concat(o.hand[s], o.lib[s])
		sort.Strings(g)
		sort.Strings(k)
		if !slices.Equal(g, k) {
			return fmt.Sprintf("p%d hidden cards: gorge %v, kernel %v", s, g, k)
		}
	}
	if !slices.Equal(z.hand[searcher], o.hand[searcher]) {
		return fmt.Sprintf("p%d (searching) hand: gorge %v, kernel %v", searcher, z.hand[searcher], o.hand[searcher])
	}
	return ""
}

func gorgeZones(e *rules.Engine, n int) mirrorZones {
	var z mirrorZones
	names := func(zone state.Zone, p state.PlayerID) []string {
		ids := e.G.Zone(zone, p)
		out := make([]string, len(ids))
		for i, id := range ids {
			out[i] = e.G.Obj(id).Face().Name
		}
		return out
	}
	for s := 0; s < n; s++ {
		z.hand[s] = names(state.ZHand, state.PlayerID(s))
		sort.Strings(z.hand[s])
		z.lib[s] = names(state.ZLibrary, state.PlayerID(s))
	}
	return z
}

func (p *searchMirrorPol) kernelZones(s *dirac.State) mirrorZones {
	var z mirrorZones
	var cards [2 * (dirac.MaxDeck + 8)]dirac.CardID
	for q := 0; q < int(p.k.n); q++ {
		n := s.HiddenCards(uint8(q), cards[:])
		names := make([]string, n)
		for i, id := range cards[:n] {
			names[i] = p.k.gname[id]
		}
		h := s.HandSize(uint8(q))
		z.hand[q], z.lib[q] = names[:h], names[h:]
		sort.Strings(z.hand[q])
	}
	return z
}

// kernelSearch runs the seat's kernel search at the kernel's pending
// decision.
func (p *searchMirrorPol) kernelSearch(seat int) *search.Result {
	return p.ksearcher(seat).SeatSearch(&p.k.s, &p.k.d, p.seatSeed[seat], &p.st[seat])
}

func (p *searchMirrorPol) ksearcher(seat int) *search.Searcher {
	if p.ksrch[seat] == nil {
		p.ksrch[seat] = p.newKSearcher(p.seatCfg.Search.Limit)
	}
	return p.ksrch[seat]
}

func (p *searchMirrorPol) newKSearcher(limit int) *search.Searcher {
	o := search.DefaultOptions()
	o.Sims = searchMirrorSims
	o.Limit = limit
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
	return search.NewSearcher(o, search.HeuristicLeaf{}, hbot.New(p.facts))
}

// gorgeBot is the seated bot's answer to gd, the search's candidate 0.
func (p *searchMirrorPol) gorgeBot(c *PickCtx, gd *decision.Decision) (decision.Intent, error) {
	e := c.G.E
	b := botpolicy.BoardFromGameInto(e.G, e, gd.Player, &p.brd)
	return p.gseats[int(gd.Player)].DecideBoard(context.Background(), b, *gd)
}

// gorgeSearch runs azmcts at gd on the honest root built from the seat's
// feed, with the options Seat.DecideSearch builds for this config at the
// candidate limit. It also returns the hidden zones that root was dealt, read
// before the search.
func (p *searchMirrorPol) gorgeSearch(c *PickCtx, gd *decision.Decision, bot decision.Intent, limit int) (azmcts.Result, mirrorZones, error) {
	var none mirrorZones
	seat := int(gd.Player)
	e := c.G.E
	f := p.feeds[seat]
	setup := searchseat.SeatSetup(p.setup, p.gseats[seat], gd.Player)
	root, reason := searchseat.HonestRoot(setup, f, e, bots.RootSeed(p.seatSeed[seat], gd.Ordinal))
	if root == nil {
		return azmcts.Result{}, none, fmt.Errorf("honest root: %s", reason)
	}
	zones := gorgeZones(root, int(p.k.n))
	opts := p.seatCfg.Search
	opts.Seed = azmcts.DecisionSeed(p.seatSeed[seat], gd.Ordinal)
	opts.Noise, opts.Sample = false, false
	opts.Limit = limit
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
		return azmcts.Result{}, none, err
	}
	rootArg := azmcts.Root{Engine: root, Decision: gd, Bot: bot, Observer: obs}
	res, err := azmcts.Search(context.Background(), rootArg, src, nil, opts)
	return res, zones, err
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

// candShape compares the two candidate lists as multisets of canonical action
// sets: "" when equal, else "differ", "gorge-more" or "kernel-more".
func candShape(gc, kc [][]Canon) string {
	n := map[string]int{}
	for _, c := range gc {
		n[candKey(c)]++
	}
	for _, c := range kc {
		n[candKey(c)]--
	}
	gorge, kernel := false, false
	for _, k := range sortedCandKeys(n) {
		gorge = gorge || n[k] > 0
		kernel = kernel || n[k] < 0
	}
	switch {
	case gorge && kernel:
		return "differ"
	case gorge:
		return "gorge-more"
	case kernel:
		return "kernel-more"
	}
	return ""
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
	x := dropDone(acts)
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
	x := dropDone(acts)
	sortCanon(x)
	return actionStrings(x)
}
