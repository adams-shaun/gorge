package search

import (
	"errors"
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The remaining instant-timing search keys, written from a design note's
// description rather than ported from code. Each is off by default and the
// search exactly as without it when off.
//
//   - HoldEps (:holdeps=E): at a searched priority root
//     with an empty stack whose chosen cast or activation the timing rules
//     offer again (laterWindow, the WaitTieUntap horizon), pass is played
//     instead with probability E; visits and the record are the search's.
//   - CastPrior (:castprior=B): at the seat's priority
//     points with an empty stack (root and walk), a cast or activation the
//     timing rules offer again has its prior times B, renormalised.
//   - DeferQ / DeferQMin (:deferq=EPS, :deferqmin=N): at a searched
//     priority root whose move A is a re-offered cast or activation, pass
//     is played where the tree's seat node after pass (passNext) values the
//     same move within EPS of now: Q(A) - Q_next(A) < EPS, Q_next read from
//     that node's A edge when it has at least N visits.
//   - Replan (:replan): after the seat passes
//     with an empty stack, a later priority window of the same turn is
//     passed unsearched while the seat's view of the game (its information,
//     the board, the stack) is unchanged, every option was offered at that
//     pass, and every option is offered again later (laterWindow: no option
//     about to expire, so the last window before the seat's untap and a
//     sorcery-speed move's main 2 are searched).
//   - RootSH (:rootsh): sequential halving at a priority root with pass
//     and a cast among its candidates: the budget split into ceil(log2 m)
//     phases, the survivors visited in turn, the better half by Q kept after
//     each phase; the move is the best survivor by Q.
//   - Reuse (:reuse=F): after a priority search that passed with an
//     empty stack, the statistics of the seat's next node under pass
//     (through the opponent's pass) are kept; when the seat's next search
//     is that point (two decisions on, the same turn, an empty stack), its
//     matching root candidates start with F times those visits.
//   - InfoSeed (:infoseed): the decision's worlds are dealt from a seed
//     of the seat's information state (its hand, its library size, the
//     opponent's seen cards, hand and library sizes, a revealed hand), so
//     decisions with the same information see the same worlds.
//   - ReactAim (:reactaim): under the horizon form of :react, at a
//     quiet point in the seat's own precombat main a cast the rule would
//     hold stays a candidate when a probe of its target decisions (in a
//     scratch copy) finds an opponent's object; once such a cast is being
//     built, its target decisions keep only the opponent's objects where
//     any is offered.

// instKeyState is the Searcher's state for these keys (Searcher.ik).
type instKeyState struct {
	// RootSH's schedule at the current root
	shOn                   bool
	shK, shPhase, shPhases int
	shPer, shLeft, shPos   int
	shSims                 int
	shSurv                 [MaxCands]int32
	// Reuse: the SeatState SeatSearch passes (nil from a bare Search) and
	// the warm start matched at this root
	seatSt *SeatState
	warmOn bool
	warmN  [MaxCands]int32
	warmW  [MaxCands]float64
	// InfoSeed: the worlds' seed (Search sets it to the decision seed when
	// off)
	wseed uint64
	// ReactAim's probe scratch
	aimS dirac.State
	aimD dirac.Decision
}

// ---- shared ------------------------------------------------------------------

// ReadsHorizon reports whether an option reads the untap-step horizon
// WaitTieUntap (:waittiehorizon=untap): a wait margin, HoldEps, CastPrior,
// DeferQ or Replan. Options.Validate and a host's spec check both use it,
// so the list is kept once.
func (o *Options) ReadsHorizon() bool {
	return o.WaitTie > 0 || o.WaitSorcOn && o.WaitSorc > 0 || o.HoldEps > 0 || o.CastPrior > 0 || o.DeferQ > 0 || o.Replan
}

// reoffered is laterWindow for the searching seat under the options'
// horizon: a re-offered move.
func (sr *Searcher) reoffered(s *dirac.State, a dirac.Action) bool {
	return laterWindow(s, a, sr.seat, sr.Opts.WaitTieUntap)
}

func castOrActivate(a dirac.Action) bool {
	k := a.Kind()
	return k == dirac.AkCast || k == dirac.AkCastCasualty || k == dirac.AkCastConspire || k == dirac.AkActivate
}

// rootPassCand is the root's one-action pass candidate, -1 none.
func rootPassCand(res *Result) int {
	for i := 0; i < res.N; i++ {
		if res.CandOne[i] && res.CandAct[i].Kind() == dirac.AkPass {
			return i
		}
	}
	return -1
}

// rootMove reports whether candidate best of the searched priority root is
// a one-action cast or activation the timing rules offer again, with a pass
// candidate pass (the HoldEps / DeferQ precondition).
func (sr *Searcher) rootMove(res *Result, best int) (pass int, ok bool) {
	if sr.rootD.Kind != dirac.DkPriority || best < 0 || best >= res.N || !res.CandOne[best] || !castOrActivate(res.CandAct[best]) {
		return -1, false
	}
	if pass = rootPassCand(res); pass < 0 || !sr.reoffered(&sr.root, res.CandAct[best]) {
		return -1, false
	}
	return pass, true
}

// passNext is the seat's next node on the root pass candidate pass's line:
// from pass's child through every opponent node by its most-visited edge
// (passLine's walk), -1 when the line ends before a seat node.
func (sr *Searcher) passNext(pass int) int32 {
	t := &sr.t
	e := t.nodes[0].first
	for i := 0; i < pass && e >= 0; i++ {
		e = t.edges[e].sib
	}
	if e < 0 {
		return -1
	}
	ni := t.edges[e].next
	for depth := 0; ni >= 0 && t.nodes[ni].opp && depth < maxDepth; depth++ {
		best := int32(-1)
		for c := t.nodes[ni].first; c >= 0; c = t.edges[c].sib {
			if t.edges[c].n > 0 && (best < 0 || t.edges[c].n > t.edges[best].n) {
				best = c
			}
		}
		if best < 0 {
			return -1
		}
		ni = t.edges[best].next
	}
	if ni < 0 || t.nodes[ni].opp {
		return -1
	}
	return ni
}

// ---- HoldEps (C4) ------------------------------------------------------------

const holdEpsSalt = 0x686f6c642d657073 // "hold-eps"

// holdEps is HoldEps at the searched root after the move best is chosen
// (Search; the sampled pick included): pass with probability HoldEps,
// drawn from a hash of the decision seed (no stream consumed).
// Stats.HoldEpsEligible is 1 where the rule could act, HoldEps 1 where it
// played pass. Result.Waited is untouched: the record is the search's.
func (sr *Searcher) holdEps(res *Result, best int) int {
	pass, ok := sr.rootMove(res, best)
	if !ok {
		return best
	}
	sr.St.HoldEpsEligible = 1
	u := float64(splitmix(sr.seed^holdEpsSalt)>>11) / (1 << 53)
	if !(u < sr.Opts.HoldEps) {
		return best
	}
	sr.St.HoldEps = 1
	return pass
}

// ---- CastPrior (C10) ---------------------------------------------------------

// castPrior is CastPrior at the seat's priority point pt of s (buildPoint,
// after the softmax and before the top-k cut, where Defer scales): every
// one-action cast or activation candidate the timing rules offer again has
// its prior times CastPrior, then the point renormalises.
func (sr *Searcher) castPrior(s *dirac.State, pt *point) {
	if s.StackLen() != 0 {
		return
	}
	hit := false
	for i := 0; i < pt.n; i++ {
		if pt.off[i+1]-pt.off[i] != 1 {
			continue
		}
		if a := dirac.Action(pt.acts[pt.off[i]]); castOrActivate(a) && sr.reoffered(s, a) {
			pt.prior[i] *= sr.Opts.CastPrior
			hit = true
		}
	}
	if hit {
		renorm(pt)
		sr.St.CastPriorPoints++
	}
}

// ---- DeferQ (C16) ------------------------------------------------------------

// deferQ is DeferQ at the searched root's move best (finalMove): pass when
// the seat's next node on pass's line values the same move (its key) within
// DeferQ of the root's Q, the next edge having at least DeferQMin visits
// (0 means 2). It sets Result.Waited as the wait does (a host's :waittarget
// reads it). Stats.DeferQEligible is 1 where the comparison was made.
func (sr *Searcher) deferQ(res *Result, best int) int {
	pass, ok := sr.rootMove(res, best)
	if !ok || pass == best {
		return best
	}
	ni := sr.passNext(pass)
	if ni < 0 {
		return best
	}
	e := sr.t.child(ni, res.Keys[best])
	if e < 0 {
		return best
	}
	ed := &sr.t.edges[e]
	minN := int32(sr.Opts.DeferQMin)
	if minN <= 0 {
		minN = 2
	}
	if ed.n < minN {
		return best
	}
	sr.St.DeferQEligible = 1
	if !(res.Q[best]-ed.w/float64(ed.n) < sr.Opts.DeferQ) {
		return best
	}
	sr.St.DeferQ = 1
	res.Waited = best + 1
	return pass
}

// ---- RootSH (C3) -------------------------------------------------------------

// shRoot reports whether the built root is RootSH's: a priority root whose
// candidates hold a pass and a cast.
func shRoot(res *Result) bool {
	pass, cast := false, false
	for i := 0; i < res.N; i++ {
		if !res.CandOne[i] {
			continue
		}
		switch res.CandAct[i].Kind() {
		case dirac.AkPass:
			pass = true
		case dirac.AkCast, dirac.AkCastCasualty, dirac.AkCastConspire:
			cast = true
		}
	}
	return pass && cast
}

// shInit starts RootSH's schedule for a tree of sims simulations over the
// root's m candidates: ceil(log2 m) phases, each of floor(sims / (phases x
// survivors)) visits per survivor (at least 1).
func (sr *Searcher) shInit(sims int) {
	x := &sr.ik
	m := sr.rootPt.n
	x.shK, x.shPhase, x.shPos, x.shSims = m, 0, 0, sims
	x.shPhases = max(1, int(math.Ceil(math.Log2(float64(m)))))
	for i := 0; i < m; i++ {
		x.shSurv[i] = int32(i)
	}
	x.shPer = max(1, sims/(x.shPhases*m))
	x.shLeft = x.shPer * m
}

// shSelect is the root's selection under RootSH (selectAt at node 0): the
// next survivor in turn, after halving the survivors by Q (ties to the
// earlier) where a phase's visits are spent; the last phase continues in
// turn to the end of the budget. A survivor this world does not offer (or
// a dead edge) falls back to PUCT.
func (sr *Searcher) shSelect(ni int32, pt *point) int32 {
	x := &sr.ik
	t := &sr.t
	if x.shLeft == 0 && x.shPhase < x.shPhases-1 && x.shK > 1 {
		sr.shHalve()
	}
	c := x.shSurv[x.shPos]
	x.shPos++
	if x.shPos >= x.shK {
		x.shPos = 0
	}
	if x.shLeft > 0 {
		x.shLeft--
	}
	e := t.nodes[ni].first
	for i := int32(0); i < c && e >= 0; i++ {
		e = t.edges[e].sib
	}
	if e < 0 || t.edges[e].dead || !pt.has(t.edges[e].key) {
		return t.selectEdge(ni, pt, &sr.Opts)
	}
	return e
}

// shHalve keeps the better half of the survivors by their root edges' Q
// (an unvisited edge last), in a stable order, and starts the next phase.
func (sr *Searcher) shHalve() {
	x := &sr.ik
	var q [MaxCands]float64
	for i := 0; i < x.shK; i++ {
		q[i] = sr.shQ(x.shSurv[i])
	}
	for i := 1; i < x.shK; i++ { // stable insertion sort, Q descending
		c, qc := x.shSurv[i], q[i]
		j := i
		for j > 0 && q[j-1] < qc {
			x.shSurv[j], q[j] = x.shSurv[j-1], q[j-1]
			j--
		}
		x.shSurv[j], q[j] = c, qc
	}
	x.shK = (x.shK + 1) / 2
	x.shPhase++
	x.shPos = 0
	x.shPer = max(1, x.shSims/(x.shPhases*x.shK))
	x.shLeft = x.shPer * x.shK
}

// shQ is root candidate c's edge Q (-Inf unvisited).
func (sr *Searcher) shQ(c int32) float64 {
	t := &sr.t
	e := t.nodes[0].first
	for i := int32(0); i < c && e >= 0; i++ {
		e = t.edges[e].sib
	}
	if e < 0 || t.edges[e].n == 0 {
		return math.Inf(-1)
	}
	return t.edges[e].w / float64(t.edges[e].n)
}

// shBest is RootSH's move: the visited survivor of best Q (the earliest of
// equals in survivor order), else best.
func (sr *Searcher) shBest(res *Result, best int) int {
	x := &sr.ik
	pick := -1
	for i := 0; i < x.shK; i++ {
		c := int(x.shSurv[i])
		if c < res.N && res.Visits[c] > 0 && (pick < 0 || res.Q[c] > res.Q[pick]) {
			pick = c
		}
	}
	if pick < 0 {
		return best
	}
	sr.St.RootSH = 1
	return pick
}

// ---- Reuse (D5) --------------------------------------------------------------

// reuseState is a SeatState's kept pass line (Reuse): the root's turn and
// sequence number, and the seat's next node's edges under pass.
type reuseState struct {
	on   bool
	turn int32
	seq  uint32
	n    int
	keys [MaxCands]Key
	vis  [MaxCands]int32
	w    [MaxCands]float64
}

// reuseMatch matches the built root against the seat's kept line (Search,
// before the tree runs): the root is two decisions after the kept one, in
// the same turn, a priority decision with an empty stack; each candidate
// whose key the line has starts with F times its visits and the same Q.
func (sr *Searcher) reuseMatch() {
	x := &sr.ik
	x.warmOn = false
	st := x.seatSt
	if st == nil || !st.reuse.on || sr.rootD.Kind != dirac.DkPriority || sr.root.StackLen() != 0 ||
		int32(sr.root.Turn()) != st.reuse.turn || sr.root.SeqNo() != st.reuse.seq+2 {
		return
	}
	rs := &st.reuse
	pt := &sr.rootPt
	for i := 0; i < pt.n; i++ {
		x.warmN[i], x.warmW[i] = 0, 0
		for j := 0; j < rs.n; j++ {
			if rs.keys[j] != pt.keys[i] {
				continue
			}
			if n := int32(math.Round(sr.Opts.Reuse * float64(rs.vis[j]))); n > 0 {
				x.warmN[i], x.warmW[i] = n, rs.w[j]/float64(rs.vis[j])*float64(n)
				x.warmOn = true
			}
			break
		}
	}
}

// reuseWarm gives the root's edges (runTree, the root node top just built,
// one edge per candidate in order) their warm visits; every root edge's
// availability counts them too, as if those simulations had run here.
func (sr *Searcher) reuseWarm(top int32) {
	x := &sr.ik
	t := &sr.t
	var total int32
	i := 0
	for e := t.nodes[top].first; e >= 0 && i < sr.rootPt.n; e = t.edges[e].sib {
		t.edges[e].n += x.warmN[i]
		t.edges[e].w += x.warmW[i]
		total += x.warmN[i]
		i++
	}
	for e := t.nodes[top].first; e >= 0; e = t.edges[e].sib {
		t.edges[e].avail += total
	}
	sr.St.Reused, sr.St.ReusedVisits = 1, int(total)
}

// reuseKeep keeps the pass line of the search just run (SeatSearch): where
// the searched root was a priority decision with an empty stack and its
// move a pass, the seat's next node under pass -- through the opponent's
// pass where the opponent's decision is a node -- and its visited edges.
func (sr *Searcher) reuseKeep(root *dirac.State, d *dirac.Decision, res *Result, st *SeatState) {
	rs := &st.reuse
	rs.on = false
	if res.Stats.Searched == 0 || d.Kind != dirac.DkPriority || root.StackLen() != 0 || sr.Opts.CachedWorlds > 0 {
		return
	}
	c := res.Choice
	if c < 0 || c >= res.N || !res.CandOne[c] || res.CandAct[c].Kind() != dirac.AkPass {
		return
	}
	t := &sr.t
	e := t.child(0, res.Keys[c])
	if e < 0 {
		return
	}
	ni := t.edges[e].next
	if ni >= 0 && t.nodes[ni].opp {
		pass := dirac.MakeAction(dirac.AkPass, 0, 0, 0)
		k := mix(mix(uint64(kindPriority), sr.actKey(root, pass, true)), 0x6f70703a) // buildPoint's opponent key
		if e = t.child(ni, Key(k)); e < 0 {
			return
		}
		ni = t.edges[e].next
	}
	if ni < 0 || t.nodes[ni].opp {
		return
	}
	n := 0
	for c := t.nodes[ni].first; c >= 0 && n < MaxCands; c = t.edges[c].sib {
		if ed := &t.edges[c]; ed.n > 0 {
			rs.keys[n], rs.vis[n], rs.w[n] = ed.key, ed.n, ed.w
			n++
		}
	}
	if n == 0 {
		return
	}
	rs.on, rs.n, rs.turn, rs.seq = true, n, int32(root.Turn()), root.SeqNo()
}

// ---- Replan (D4) -------------------------------------------------------------

// replanState is a SeatState's last pass (Replan): its turn, the seat's
// view of the game then (boardSig) and the options it was offered.
type replanState struct {
	on   bool
	turn int32
	sig  uint64
	n    int
	opts [dirac.MaxOptions]dirac.Action
}

// boardSig is a hash of the game as seat sees it, the step aside: every
// object's zone and owner, and for a visible one its card, controller, and
// on the battlefield its tap, damage, power and toughness and combat marks;
// the stack; both life totals and mana pools. Hidden cards are counted,
// never read.
func boardSig(s *dirac.State, seat uint8) uint64 {
	h := uint64(0x7265706c616e)
	for i, n := 0, s.NumObjs(); i < n; i++ {
		o := dirac.Obj(i)
		z := s.ObjZone(o)
		h = mix(h, uint64(z)<<8|uint64(s.ObjOwner(o)))
		if z == kop.ZoneNone || s.Hidden(o, seat) {
			continue
		}
		x := uint64(s.ObjCard(o)) | uint64(s.Controller(o))<<32
		if z == kop.ZoneBattlefield {
			p, tg := s.PowerToughness(o)
			x = mix(x, uint64(uint16(p))|uint64(uint16(tg))<<16|uint64(s.Damage(o))<<32)
			if s.Tapped(o) {
				x ^= 1 << 40
			}
			if _, atk := s.HBotAttackTarget(o); atk {
				x ^= 1 << 41
			}
			if b := s.HBotBlocking(o); b != dirac.NoObj {
				x = mix(x, uint64(b)+1)
			}
		}
		h = mix(h, x)
	}
	for i, n := 0, s.StackLen(); i < n; i++ {
		k, src, card, ctrl := s.StackEntry(i)
		h = mix(h, uint64(k)|uint64(src)<<8|uint64(card)<<24|uint64(ctrl)<<56)
	}
	for p := uint8(0); p < s.NumSeats(); p++ {
		h = mix(h, uint64(uint32(s.LifeOf(p))))
		pool, restr := s.PoolOf(p)
		for i, m := range pool {
			h = mix(h, uint64(m)|uint64(restr[i])<<8)
		}
	}
	return h
}

// replanSkip is Replan at seat decision d of root (SeatSearch, before any
// search): the unsearched pass when the seat's last answer this turn was
// a pass into an empty stack, the view is unchanged (boardSig), every
// option now was offered then, and every non-pass option is offered again
// later (laterWindow); nil otherwise.
func (sr *Searcher) replanSkip(root *dirac.State, d *dirac.Decision, st *SeatState) *Result {
	rp := &st.rp
	if !rp.on || d.Kind != dirac.DkPriority || root.StackLen() != 0 || int32(root.Turn()) != rp.turn {
		return nil
	}
	pass := -1
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == dirac.AkPass {
			pass = i
			continue
		}
		if !rp.offered(a) || !laterWindow(root, a, d.Player, sr.Opts.WaitTieUntap) {
			return nil
		}
	}
	if pass < 0 || boardSig(root, d.Player) != rp.sig {
		return nil
	}
	res := &sr.merged
	*res = Result{}
	sr.St = Stats{Skipped: 1, Replanned: 1}
	res.NActs, res.Acts[0], res.Weight = 1, d.Options[pass], 1
	res.Stats = sr.St
	return res
}

func (rp *replanState) offered(a dirac.Action) bool {
	for i := 0; i < rp.n; i++ {
		if rp.opts[i] == a {
			return true
		}
	}
	return false
}

// replanNote records the seat's answer res at decision d of root for
// Replan: a pass into an empty stack at a priority decision is kept (its
// turn, view and options), anything else clears the record.
func replanNote(root *dirac.State, d *dirac.Decision, res *Result, st *SeatState) {
	rp := &st.rp
	if d.Kind != dirac.DkPriority || root.StackLen() != 0 || res.NActs != 1 || res.Acts[0].Kind() != dirac.AkPass {
		rp.on = false
		return
	}
	if res.Stats.Replanned != 0 {
		return // still passing: the record stands
	}
	rp.on, rp.turn, rp.sig, rp.n = true, int32(root.Turn()), boardSig(root, d.Player), int(d.N)
	copy(rp.opts[:], d.Options[:d.N])
}

// seatSearchKeys is SeatSearch with Replan or Reuse on: Replan's skip
// before the search, the search with the SeatState in reach (Reuse), and
// both records after it.
func (sr *Searcher) seatSearchKeys(root *dirac.State, d *dirac.Decision, seatSeed uint64, st *SeatState) *Result {
	if sr.Opts.Replan {
		if res := sr.replanSkip(root, d, st); res != nil {
			st.reuse.on = false
			return res
		}
	}
	sr.ik.seatSt = st
	res := sr.seatSearch(root, d, seatSeed, st)
	sr.ik.seatSt = nil
	if sr.Opts.Reuse > 0 {
		sr.reuseKeep(root, d, res, st)
	}
	if sr.Opts.Replan {
		replanNote(root, d, res, st)
	}
	return res
}

// ---- InfoSeed (C2) -----------------------------------------------------------

// infoSeed is InfoSeed's world seed at the root for seatSeed: a hash of
// the seat's information -- its hand's cards (as a multiset), its library
// size, each opponent's seen cards (a multiset), hand and library sizes, and
// each opponent's hand where it is revealed to the seat (opponents in seat
// order).
func (sr *Searcher) infoSeed(seatSeed uint64) uint64 {
	s := &sr.root
	p := sr.seat
	n := s.NumSeats()
	var hand uint64
	var rev [dirac.MaxSeats]uint64
	for i, no := 0, s.NumObjs(); i < no; i++ {
		o := dirac.Obj(i)
		if s.ObjZone(o) != kop.ZoneHand {
			continue
		}
		switch q := s.ObjOwner(o); {
		case q == p:
			hand += splitmix(uint64(s.ObjCard(o)) + 0x68616e64)
		case s.HandRevealed(q, p):
			rev[q] += splitmix(uint64(s.ObjCard(o)) + 0x72657665)
		}
	}
	h := splitmix(seatSeed ^ 0x696e666f73656564) // "infoseed"
	h = mix(h, hand)
	h = mix(h, uint64(s.LibrarySize(p)))
	for q := uint8(0); q < n; q++ {
		if q == p {
			continue
		}
		var seen uint64
		ns := s.SeenCards(q, p, sr.seenC[:])
		for _, c := range sr.seenC[:ns] {
			seen += splitmix(uint64(c) + 0x7365656e)
		}
		h = mix(h, seen)
		h = mix(h, uint64(s.HandSize(q))<<16|uint64(s.LibrarySize(q)))
	}
	for q := uint8(0); q < n; q++ {
		if q != p {
			h = mix(h, rev[q])
		}
	}
	return h
}

// ---- ReactAim ------------------------------------------------------------------

// oppObjOption is d's first target option naming an opponent's object (not
// a player), -1 none.
func (sr *Searcher) oppObjOption(s *dirac.State, d *dirac.Decision) int {
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkTarget || a.Target() >= dirac.PlayerTarget0 {
			continue
		}
		if o := dirac.Obj(a.Target()); int(o) < s.NumObjs() && s.Controller(o) != sr.seat {
			return i
		}
	}
	return -1
}

// aimKeeps is ReactAim at a cast a the react rule would hold at s in step
// st: in the seat's own precombat main, a probe of a's target decisions
// in a scratch copy (cast it, then answer each of its target decisions
// with its first option, at most four) finds an opponent's object offered.
func (sr *Searcher) aimKeeps(s *dirac.State, a dirac.Action, st kop.Step) bool {
	if st != kop.StepMain1 || s.ActivePlayer() != sr.seat {
		return false
	}
	x := &sr.ik
	x.aimS.CopyFrom(s)
	x.aimS.StepInto(&x.aimD)
	x.aimS.Apply(a)
	for i := 0; i < 4; i++ {
		x.aimS.StepInto(&x.aimD)
		if x.aimD.Kind != dirac.DkTarget || x.aimD.Player != sr.seat || x.aimD.N == 0 {
			return false
		}
		if sr.oppObjOption(&x.aimS, &x.aimD) >= 0 {
			sr.St.ReactAimKept++
			return true
		}
		x.aimS.Apply(x.aimD.Options[0])
	}
	return false
}

// aimNow reports whether d is a target decision ReactAim narrows: the
// seat's, for an instant-speed spell it is casting into an empty stack in
// its own quiet precombat main (a cast the react rule would have held: the
// spell being built is the stack's only entry),
// with an opponent's object among the options.
func (sr *Searcher) aimNow(s *dirac.State, d *dirac.Decision) bool {
	if d.Kind != dirac.DkTarget || d.Player != sr.seat || s.ActivePlayer() != sr.seat || s.StepNow() != kop.StepMain1 || s.StackLen() > 1 {
		return false
	}
	if s.StackLen() == 1 { // the spell being built is the stack's only entry
		if _, _, _, c := s.StackEntry(0); c != sr.seat {
			return false
		}
	}
	card, _, ctrl, ok := s.Building()
	if !ok || ctrl != sr.seat {
		return false
	}
	def := &s.Table().Cards[card]
	if def.Types&kop.TLand != 0 || def.Types&kop.TInstant == 0 && def.Keywords&kop.KwFlash == 0 {
		return false
	}
	if sr.curReact == nil {
		sr.curReact = &sr.rootReact
	}
	if sr.curReact.firedNow(s, sr.seat) {
		return false
	}
	return sr.oppObjOption(s, d) >= 0
}

// aimBot replaces the bot's answer at a ReactAim target decision by the
// first opponent's object when it names none (instBot's target case).
func (sr *Searcher) aimBot(s *dirac.State, d *dirac.Decision, bot int) int {
	if !sr.aimNow(s, d) {
		return bot
	}
	if a := d.Options[bot]; a.Kind() == dirac.AkTarget && dirac.IsObjTarget(a.Target()) && s.Controller(dirac.Obj(a.Target())) != sr.seat {
		return bot
	}
	return sr.oppObjOption(s, d)
}

// singleAim is single at a ReactAim target decision: the bot's answer (an
// opponent's object, aimBot) first, then every other opponent's object.
func (sr *Searcher) singleAim(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point) {
	sr.St.ReactAimPoints++
	var one [1]uint32
	one[0] = uint32(d.Options[bot])
	pt.addSeq(one[:])
	for i := 0; i < int(d.N) && pt.n < limit; i++ {
		a := d.Options[i]
		if i == bot || a.Kind() != dirac.AkTarget || a.Target() >= dirac.PlayerTarget0 || s.Controller(dirac.Obj(a.Target())) == sr.seat {
			continue
		}
		one[0] = uint32(a)
		pt.addSeq(one[:])
	}
}

// ---- validation --------------------------------------------------------------

// validateInstKeys refuses these keys' bad values (Options.Validate).
func (o *Options) validateInstKeys() error {
	fin := func(x float64) bool { return x >= 0 && !math.IsInf(x, 1) }
	switch {
	case !(o.HoldEps >= 0 && o.HoldEps <= 1):
		return fmt.Errorf("search: HoldEps %g must be in [0, 1] (0 is off)", o.HoldEps)
	case !fin(o.CastPrior):
		return fmt.Errorf("search: CastPrior %g must be finite and >= 0 (0 is off)", o.CastPrior)
	case !(o.DeferQ >= 0 && o.DeferQ <= 1):
		return fmt.Errorf("search: DeferQ %g must be in [0, 1] (0 is off)", o.DeferQ)
	case o.DeferQMin < 0 || o.DeferQMin != 0 && o.DeferQ == 0:
		return errors.New("search: DeferQMin must be >= 0 and needs DeferQ")
	case o.DeferQ > 0 && o.CachedWorlds > 0:
		return errors.New("search: DeferQ reads one tree; it is incompatible with CachedWorlds")
	case !fin(o.Reuse):
		return fmt.Errorf("search: Reuse %g must be finite and >= 0 (0 is off)", o.Reuse)
	case o.Reuse > 0 && (o.CachedWorlds > 0 || o.Batch > 0):
		return errors.New("search: Reuse warms one serial tree; it is incompatible with CachedWorlds and Batch")
	case o.RootSH && (o.CachedWorlds > 0 || o.Batch > 0 || o.NodeCache > 0 || o.ForcedK > 0 || o.EarlyCutGap || o.EarlyCutShare > 0):
		return errors.New("search: RootSH runs on one serial tree; it is incompatible with CachedWorlds, Batch, NodeCache, ForcedK and the early cut")
	case o.ReactAim && (o.InstWin != InstWinReact || !o.ReactHorizon):
		return errors.New("search: ReactAim needs the horizon react (InstWin == InstWinReact with ReactHorizon)")
	}
	return nil
}
