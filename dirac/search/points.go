package search

import (
	"math"

	"github.com/adams-shaun/gorge/dirac"
)

// Searched kinds (azmcts kindOf).
const (
	kindNone uint8 = iota
	kindPriority
	kindAttackers
	kindBlockers
	kindTarget
	kindModes
	kindOptional
	kindChoose
)

// kindOf is azmcts kindOf over dirac's decisions. Dirac poses
// gorge's multi-selects as single picks, so a choose / discard pick is
// searched pick by pick (the fuller kinds' sets become sequences). Arrange
// picks (scry/surveil order, a dig's rest) are bot-answered like mulligan
// bottoms: azmcts has no arrange searched kind.
func kindOf(d *dirac.Decision, k *Kinds, limit int) uint8 {
	switch d.Kind {
	case dirac.DkAttackers:
		if k.Attackers {
			return kindAttackers
		}
	case dirac.DkBlockers:
		if k.Blockers {
			return kindBlockers
		}
	case dirac.DkTarget:
		if k.Target {
			return kindTarget
		}
	case dirac.DkPriority:
		if k.Priority {
			return kindPriority
		}
	case dirac.DkModes:
		if k.Modes {
			return kindModes
		}
	case dirac.DkChoose:
		switch d.Sub {
		case dirac.ChOptional, dirac.ChOptDiscMay:
			if k.Optional {
				return kindOptional
			}
		case dirac.ChBottom, dirac.ChArrange:
			// bot-answered: mulligan bottoms and arrange order have no
			// azmcts searched kind
		default:
			if k.Choose && int(d.N) <= limit {
				return kindChoose
			}
		}
	}
	return kindNone
}

// key building -------------------------------------------------------------

func mix(h, v uint64) uint64 {
	h ^= v + 0x9e3779b97f4a7c15 + (h << 6) + (h >> 2)
	h *= 0xbf58476d1ce4e5b9
	return h ^ (h >> 31)
}

// objCode names an object in a key: by its object index when the searching
// seat saw it at the root (azmcts observer-local references), else by its
// card (azmcts NameKeys / the opponent's keys: re-dealt objects are named by
// card, two same-named objects share a key).
func (sr *Searcher) objCode(s *dirac.State, code uint8, byName bool) uint64 {
	if code >= dirac.PlayerTarget0 {
		return uint64(code)
	}
	o := dirac.Obj(code)
	if !byName && int(o) < len(sr.seen) && sr.seen[o] {
		return uint64(o)
	}
	return 0x1000 | uint64(s.ObjCard(o))
}

// actKey is one dirac action's semantic key.
func (sr *Searcher) actKey(s *dirac.State, a dirac.Action, byName bool) uint64 {
	k := uint64(a.Kind())
	src, tgt := uint64(a.Source()), uint64(a.Target())
	switch a.Kind() {
	case dirac.AkPlayLand, dirac.AkCast, dirac.AkActivate, dirac.AkAttack, dirac.AkBlock, dirac.AkChoose:
		src = sr.objCode(s, a.Source(), byName)
	}
	switch a.Kind() {
	case dirac.AkAttack, dirac.AkBlock, dirac.AkTarget:
		tgt = sr.objCode(s, a.Target(), byName)
	}
	if a.Kind() == dirac.AkChoose && (a.Source() == 0) {
		src = 0
	}
	return k<<56 ^ src<<32 ^ tgt<<12 ^ uint64(a.Mode())
}

// candidate building --------------------------------------------------------

// buildPoint fills pt with the candidates of decision d at s for its
// player, the bot's answer bot (an index into d.Options) as candidate 0;
// it reports false when d is not a searched point: not a searched kind,
// the bot's answer outside the candidate vocabulary (a land play), or
// fewer than two candidates (azmcts enumerateCutInto + botFirst +
// rankedPrior; the opponent's point under OpponentNodes names every object
// by its card and carries the opponent prefix).
func (sr *Searcher) buildPoint(s *dirac.State, d *dirac.Decision, bot int, pt *point) bool {
	o := &sr.Opts
	kind := sr.kindAt(s, d) // landsearch.go: kindOf, or DedupSearch's
	if kind == kindNone || d.N < 2 {
		return false
	}
	opp := d.Player != sr.seat
	if opp && !o.OpponentNodes {
		return false
	}
	pt.n, pt.opp, pt.seat, pt.off[0] = 0, opp, relSeat(d.Player, sr.seat, s.NumSeats()), 0
	byName := opp // a DedupSearch library search names its picks by card
	dedup := o.DedupSearch && libSearch(d)
	if dedup {
		byName = true
	}
	topK := 0
	if sr.netPrior && (!opp || o.OpponentMirror) {
		topK = o.PriorTopK
	}
	limit := o.Limit
	if topK > 0 {
		limit = MaxCands
	}
	rank := sr.netPrior && (o.PriorRankMaxOptions <= 0 || sr.ranks(d, topK)) // seat.go
	switch kind {
	case kindAttackers:
		if o.PerCreature&PerCreatureAttack == 0 || !sr.chainCands(s, d, bot, limit, pt) {
			sr.attackCands(s, d, bot, limit, pt)
		}
	case kindBlockers:
		if o.PerCreature&PerCreatureBlock == 0 || !sr.chainCands(s, d, bot, limit, pt) {
			sr.blockCands(s, d, bot, limit, pt)
		}
	case kindPriority:
		if !sr.priorityCands(s, d, bot, limit, pt) {
			return false
		}
	default:
		if dedup {
			sr.singleDedup(s, d, bot, limit, pt)
		} else if o.ReactAim && kind == kindTarget && !opp && sr.aimNow(s, d) {
			sr.singleAim(s, d, bot, limit, pt) // instkeys.go
		} else {
			sr.single(d, bot, limit, pt)
		}
	}
	if pt.n < 2 {
		return false
	}
	for i := 0; i < pt.n; i++ {
		k := uint64(kind)
		for j := pt.off[i]; j < pt.off[i+1]; j++ {
			k = mix(k, sr.actKey(s, dirac.Action(pt.acts[j]), byName))
		}
		if opp {
			k = mix(k, 0x6f70703a) // "opp:"
		}
		pt.keys[i] = Key(k)
	}
	if rank && sr.hint >= 0 && sr.priorsUnused(pt, topK) {
		if opp {
			// both prior-independent, exactly as below
			sr.dedupKeys(pt)
			if pt.n < 2 {
				return false
			}
		}
		for i := 0; i < pt.n; i++ {
			pt.prior[i] = 1 / float64(pt.n) // placeholder: nothing reads it
		}
		sr.St.PriorSkips++
		return true
	}
	if rank {
		for i := 0; i < int(d.N); i++ {
			sr.botPick[i] = false
		}
		for j := pt.off[0]; j < pt.off[1]; j++ {
			if x := sr.optIndex(d, dirac.Action(pt.acts[j])); x >= 0 {
				sr.botPick[x] = true
			}
		}
		if sr.raidMark > 0 {
			sr.raidBotMark(d) // timing.go: the bot's own held raid cast
		}
		ev := sr.Eval
		if sr.atRoot {
			ev, sr.St.RootPrior = sr.RootEval, 1
		}
		ev.Priors(s, d, sr.botPick[:d.N], sr.logits[:d.N])
		for i := 0; i < pt.n; i++ {
			pt.prior[i] = sr.candLogit(d, kind, pt, i)
		}
	}
	if opp {
		// same-named candidates share a key: keep the first (azmcts nameKeys)
		sr.dedupKeys(pt)
		if pt.n < 2 {
			return false
		}
	}
	if rank {
		if o.PriorTemp > 0 {
			sr.temper(pt, opp) // timing.go
		}
		softmax(pt.prior[:pt.n])
	} else {
		for i := 0; i < pt.n; i++ {
			pt.prior[i] = 1 / float64(pt.n)
		}
	}
	if o.Defer > 0 && kind == kindPriority && !opp {
		sr.deferScale(s, pt) // timing.go
	}
	if o.CastPrior > 0 && kind == kindPriority && !opp {
		sr.castPrior(s, pt) // instkeys.go
	}
	if topK > 0 {
		cutTopK(pt, topK)
	} else if pt.n > o.Limit {
		pt.n = o.Limit
		renorm(pt)
	}
	if sr.rootBuild && o.CardSimsForce && kind == kindPriority {
		sr.forceCards(s, d, pt, rank) // cardsims.go
	}
	return true
}

func renorm(pt *point) {
	sum := 0.0
	for i := 0; i < pt.n; i++ {
		sum += pt.prior[i]
	}
	for i := 0; i < pt.n; i++ {
		if sum > 0 {
			pt.prior[i] /= sum
		} else {
			pt.prior[i] = 1 / float64(pt.n)
		}
	}
}

func (sr *Searcher) dedupKeys(pt *point) {
	var tmp point
	for i := 0; i < pt.n; i++ {
		dup := false
		for j := 0; j < tmp.n; j++ {
			if tmp.keys[j] == pt.keys[i] {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		tmp.addSeq(pt.acts[pt.off[i]:pt.off[i+1]])
		tmp.keys[tmp.n-1], tmp.prior[tmp.n-1] = pt.keys[i], pt.prior[i]
	}
	tmp.opp, tmp.seat = pt.opp, pt.seat
	*pt = tmp
}

// priorityCands is azmcts paymentPriorityCands over dirac's autopay
// vocabulary: pass first, then casts (gorge's payment casts) and
// activations in option order; land plays are never candidates, and a bot
// answer outside the vocabulary (a land play) makes the decision unsearched.
// The bot's candidate moves to the front (botFirst). Options.LandSearch
// and LandPick (landsearch.go) make a land-play bot answer a point;
// AltCands and CastDedup (mzkeys.go) widen and merge the casts.
func (sr *Searcher) priorityCands(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point) bool {
	if d.Options[bot].Kind() == dirac.AkPlayLand && !sr.landOn(d) {
		return false
	}
	var hold, raid bool
	if sr.tmWalk {
		var ok bool
		if hold, raid, ok = sr.timingCands(s, d, bot); !ok { // timing.go
			return false
		}
	}
	var one [1]uint32
	one[0] = uint32(d.Options[bot])
	pt.addSeq(one[:])
	own := (sr.Opts.InstWin != InstWinOff || sr.Opts.Defer > 0) && d.Player == sr.seat
	ef := sr.Opts.EndFire && sr.endFireNow(s, d)
	add := func(i int) {
		if ef && d.Options[i].Kind() == dirac.AkPass {
			return // instwin.go: EndFire never passes with removal castable
		}
		if own && sr.instBlocked(s, d.Options[i]) {
			if sr.inRoot && i != bot && sr.nHeld < MaxCands {
				sr.held[sr.nHeld] = uint8(i) // InstWinReact: the root's pruned casts
				sr.nHeld++
			}
			return // instwin.go: outside the seat's own-turn window
		}
		if sr.Opts.CastDedup && castDup(s, pt, d.Options[i]) {
			return // mzkeys.go
		}
		if i != bot && pt.n < limit && !((hold || raid) && sr.timingDrop(s, d.Options[i], hold, raid)) {
			one[0] = uint32(d.Options[i])
			pt.addSeq(one[:])
		}
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPass {
			add(i)
		}
	}
	if sr.Opts.LandPick && sr.landOn(d) {
		sr.addLands(s, d, bot, limit, pt)
	}
	for i := 0; i < int(d.N); i++ {
		switch d.Options[i].Kind() {
		case dirac.AkCast, dirac.AkCastCasualty, dirac.AkCastConspire, dirac.AkActivate:
			add(i)
		default:
			if sr.Opts.AltCands && altKind(d.Options[i].Kind()) {
				add(i) // mzkeys.go
			}
		}
	}
	return true
}

func (sr *Searcher) optIndex(d *dirac.Decision, a dirac.Action) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == a {
			return i
		}
	}
	return -1
}

func softmax(x []float64) {
	m := math.Inf(-1)
	for _, v := range x {
		if v > m {
			m = v
		}
	}
	sum := 0.0
	for i, v := range x {
		x[i] = math.Exp(v - m)
		sum += x[i]
	}
	for i := range x {
		x[i] /= sum
	}
}

// addSeq appends a candidate playing acts.
func (pt *point) addSeq(acts []uint32) bool {
	if pt.n >= MaxCands || int(pt.off[pt.n])+len(acts) > maxActs {
		return false
	}
	base := pt.off[pt.n]
	copy(pt.acts[base:], acts)
	pt.n++
	pt.off[pt.n] = base + uint16(len(acts))
	return true
}

// single: the bot's option first, then every other option in decision
// order (azmcts TargetCandidates / the priority vocabulary), capped at
// limit.
func (sr *Searcher) single(d *dirac.Decision, bot, limit int, pt *point) {
	var one [1]uint32
	one[0] = uint32(d.Options[bot])
	pt.addSeq(one[:])
	for i := 0; i < int(d.N) && pt.n < limit; i++ {
		if i == bot {
			continue
		}
		one[0] = uint32(d.Options[i])
		pt.addSeq(one[:])
	}
}

// cutTopK is azmcts priorTopK: candidate 0 (the bot's) stays first; the
// others are stably sorted by prior descending (ties to enumeration order)
// and the first k-1 kept; only a cut renormalises.
func cutTopK(pt *point, k int) {
	var order [MaxCands]int
	m := 0
	for i := 1; i < pt.n; i++ {
		j := m
		for j > 0 && pt.prior[order[j-1]] < pt.prior[i] {
			order[j] = order[j-1]
			j--
		}
		order[j] = i
		m++
	}
	cut := m > k-1
	if cut {
		m = k - 1
	}
	var tmp point
	tmp.opp, tmp.seat = pt.opp, pt.seat
	add := func(i int) {
		tmp.addSeq(pt.acts[pt.off[i]:pt.off[i+1]])
		tmp.keys[tmp.n-1], tmp.prior[tmp.n-1] = pt.keys[i], pt.prior[i]
	}
	add(0)
	for x := 0; x < m; x++ {
		add(order[x])
	}
	if cut {
		renorm(&tmp)
	}
	*pt = tmp
}

// candLogit is candidate i's logit from the option logits (azmcts
// priorsWith): its option's for a single pick, policynet.CandidateScore's
// subset log-likelihood over the declaration options (dirac's Done is
// no gorge option) for attackers and blockers, the sum of its picks' for
// the fuller kinds.
func (sr *Searcher) candLogit(d *dirac.Decision, kind uint8, pt *point, i int) float64 {
	switch kind {
	case kindAttackers, kindBlockers:
		ll := 0.0
		for x := 0; x < int(d.N); x++ {
			a := d.Options[x]
			if a.Kind() == dirac.AkDone {
				continue
			}
			z := float64(sr.logits[x])
			in := false
			for j := pt.off[i]; j < pt.off[i+1]; j++ {
				if pt.acts[j] == uint32(a) {
					in = true
					break
				}
			}
			if in {
				ll -= softplus(-z)
			} else {
				ll -= softplus(z)
			}
		}
		return ll
	}
	l := 0.0
	for j := pt.off[i]; j < pt.off[i+1]; j++ {
		if x := sr.optIndex(d, dirac.Action(pt.acts[j])); x >= 0 {
			l += float64(sr.logits[x])
		}
	}
	return l
}

// softplus is log(1+e^z) without overflow (policynet candidateSoftplus).
func softplus(z float64) float64 {
	if z > 0 {
		return z + math.Log1p(math.Exp(-z))
	}
	return math.Log1p(math.Exp(z))
}

// priorsUnused reports whether a point of the existing node sr.hint cannot
// read its network prior: no cut or truncation can fire (the candidate
// count is within topK under a top-k cut, else Limit) and every candidate
// key already has an edge at the node (so the walk adds none and newNode is
// not called). Out of line so the hot file keeps its code layout.
func (sr *Searcher) priorsUnused(pt *point, topK int) bool {
	c := sr.Opts.Limit
	if topK > 0 {
		c = topK
	}
	if pt.n > c {
		return false
	}
	for i := 0; i < pt.n; i++ {
		if sr.t.child(sr.hint, pt.keys[i]) < 0 {
			return false
		}
	}
	return true
}
