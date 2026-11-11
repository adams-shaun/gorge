package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "CARDNAME must be blocked if able." (CR 509.1c; gorge
// hidden keyword MustBlock, read layer-derived per attacker by
// combat.DerivedHiddenFlags in askBlockers). Printed (CardDef.MustBeBlocked)
// or granted until end of turn (ExtMustBeBlocked: Pump / PumpAll KW$ HIDDEN,
// an Animate's HiddenKeywords$), the grant being the arena slot bit
// mechMustBlocked (0 B): set by the instruction, cleared as the next turn
// begins (the mark is read only while blockers are declared) and when a new
// permanent takes the slot (arenaClrSlot, CR 400.7).
//
// gorge publishes each offered pair of a must-be-blocked attacker as an
// attacker requirement (Option.AttackMust) and accepts only a declaration
// that obeys the MAXIMUM number of requirements (decision.blockRequiredQuota):
// one unit per must-be-blocked attacker with at least one blocker (any one
// satisfies it) plus one per lure duty blocker blocking a lure attacker
// (Option.BlockMust, Lure). The lockstep emulates the multi-pick ask as
// completability: a pair is offered only when some completion of the
// declaration still reaches that maximum, and Done only once it is met.
// blockerOptionsReq computes the same: the maximum, then each pair's
// reachability, by a bounded search over the defender's ready blockers
// (fixed arrays; StallMustBlock past mbBudget nodes). Cold: reached only with
// prep.anyMustBlock and a must-be-blocked attacker in combat.

// StallMustBlock: the block-requirement search ran past mbBudget nodes.
const StallMustBlock = 113

func init() { stallNames[StallMustBlock] = "mustblock" }

// DebugMustBlockGreedy offers Done as soon as any one requirement is met
// instead of only at the maximum (mutation testing only: the lockstep must
// catch it).
var DebugMustBlockGreedy bool

// mbBudget bounds one search's nodes.
const mbBudget = 1 << 20

// prepMustBlock sets anyMustBlock: some card is printed must-be-blocked, or
// the table gives mechMustBlocked an arena mask.
func prepMustBlock(t *kop.Table, g *genPrep) {
	g.anyMustBlock = g.ix[mechMustBlocked] != arenaNone || g.ix[mechBlocksIfAble] != arenaNone
	for i := 0; i < len(t.Cards) && !g.anyMustBlock; i++ {
		g.anyMustBlock = t.Cards[i].MustBeBlocked
	}
}

// mustBlockUses reports the ExtMustBeBlocked mask need (arenaUses).
func mustBlockUses(in *kop.Instr, u *[numMech]bool) {
	if in.Op == kop.OpExt && in.Ext() == kop.ExtMustBeBlocked {
		u[mechMustBlocked], u[mechMustBlockedPost] = true, true
	}
}

// mustBlockNewTurn ends the grants as the turn begins.
func (s *State) mustBlockNewTurn(k *prep) {
	s.mSlZero(k, mechMustBlocked)
	s.mSlZero(k, mechMustBlockedPost)
	s.mSlZero(k, mechAttacksIfAble) // combat_attacksifable.go
	s.mSlZero(k, mechBlocksIfAble)  // combat_blocksifable.go
}

// mustBlockExt is ExtMustBeBlocked: Obj, or every battlefield permanent
// matching F with Obj RefNone (the suspectExt shape), is marked.
func (s *State) mustBlockExt(k *prep, in *kop.Instr) {
	if in.Obj == kop.RefNone {
		var ord [MaxPerm]Slot
		n := s.sweepHits(k, in.F, &ord)
		for _, sl := range ord[:n] {
			s.mustBlockMark(k, sl)
		}
		return
	}
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		s.mustBlockMark(k, sl)
	}
}

func (s *State) mustBlockMark(k *prep, sl Slot) {
	if !s.mSl(k, mechMustBlocked, sl) {
		s.mSlSet(k, mechMustBlocked, sl)
		s.touchSl(sl)
	}
	// a grant made after the permanent lost its abilities is a later layer-6
	// effect than the loss and survives it (CR 613.7; gorge's layer-6 walk)
	if !s.mSl(k, mechMustBlockedPost, sl) && s.genLost(k, sl) {
		s.mSlSet(k, mechMustBlockedPost, sl)
		s.touchSl(sl)
	}
}

// mustBeBlocked reports whether the attacker at a must be blocked if able:
// printed, or marked, and its abilities not lost; a mark made after the loss
// holds through it.
func (s *State) mustBeBlocked(k *prep, a Slot) bool {
	return (k.t.Cards[s.cardAt(a)].MustBeBlocked || s.mSl(k, mechMustBlocked, a)) && !s.genLost(k, a) ||
		s.mSl(k, mechMustBlockedPost, a)
}

// mustBlockAttackers is the attacking creatures that must be blocked if able.
func (s *State) mustBlockAttackers(k *prep) (m slotMask) {
	for am := s.blockAtkM(k); am.any(); am = am.rest() {
		if a := am.first(); s.perms[a].flags&pfAttacking != 0 && s.mustBeBlocked(k, a) {
			m = m.with(a)
		}
	}
	return
}

// mbSearch is one maximum-requirements search over the defender's free
// blockers (ready, not yet declared). The declaration so far is in cnt (the
// blockers each attacker has), score (requirements it meets) and deficit
// (blockers still missing from attackers below their minimum).
type mbSearch struct {
	s     *State
	k     *prep
	nb    int
	bl    [MaxPerm]Slot     // free blockers
	able  [MaxPerm]slotMask // attackers bl[i] may be added to
	cnt   [MaxPerm]uint8    // by attacker slot: blockers so far
	min   [MaxPerm]uint8    // by attacker slot: blockers it needs once blocked
	one   [MaxPerm]bool     // by attacker slot: takes at most one blocker
	am    slotMask          // attackers
	lure  slotMask          // lure attackers
	mb    slotMask          // must-be-blocked attackers
	duty  slotMask          // blockers that block this turn if able (combat_blocksifable.go)
	luSuf [MaxPerm + 1]int  // blockers bl[i:] that may block some lure attacker, or have a block duty and may block any
	last  [MaxPerm]int      // by attacker slot: highest blocker index able to block it, -1 none
	want  int               // stop at the first valid completion scoring this much (-1: find the maximum)
	best  int               // best valid score so far, -1 none
	nodes int
	over  bool // past mbBudget
	// CR 506.5 (combat_alone.go): a completion with one blocker that can't
	// block alone is not valid.
	alone  [MaxPerm]bool // by blocker index: bl[i] can't block alone
	nBlk   int           // blockers declared so far (xb included)
	nBlkR  int           // of them, those that can't block alone
	added  int           // free blockers the search has assigned
	addedR int           // of them, those that can't block alone
}

// mbInit loads the search from the declaration so far with the extra block
// (xb blocks xa; NoSlot for none): xb is not free, xa has one more blocker.
// It returns the score and deficit of that declaration.
func (m *mbSearch) mbInit(s *State, k *prep, lure, mb, duty slotMask, xb, xa Slot) (score, deficit int) {
	*m = mbSearch{s: s, k: k, lure: lure, mb: mb, duty: duty, best: -1, want: -1}
	def := s.blockDef()
	for am := s.blockAtkM(k); am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking == 0 {
			continue
		}
		m.am = m.am.with(a)
		m.cnt[a] = s.perms[a].nblock
		m.min[a] = uint8(s.minBlockers(k, a))
		m.one[a] = k.gen != nil && k.gen.anyMaxBlk && s.maxOneBlocker(k, a)
		m.last[a] = -1
	}
	for bm := s.ctrlM[def]; bm.any(); bm = bm.rest() {
		b := bm.first()
		if p := &s.perms[b]; p.flags&pfBlocking != 0 && (m.lure.has(p.blocking) || m.duty.has(b)) {
			score++ // a lure duty blocker blocking a lure attacker, or a duty blocker blocking
		}
		if p := &s.perms[b]; p.flags&pfBlocking != 0 || b == xb {
			m.nBlk++
			if s.aloneBlk(k, b) {
				m.nBlkR++
			}
		}
		if b == xb || !s.blockerReady(k, b) {
			continue
		}
		var able slotMask
		for am := m.am; am.any(); am = am.rest() {
			if a := am.first(); s.canBlock(k, b, a) && !s.menaceImpossible(k, a) {
				able = able.with(a)
				m.last[a] = m.nb
			}
		}
		m.bl[m.nb], m.able[m.nb], m.alone[m.nb] = b, able, s.aloneBlk(k, b)
		m.nb++
	}
	if xa != NoSlot {
		if m.lure.has(xa) || m.duty.has(xb) {
			score++
		}
		m.cnt[xa]++ // the first blocker of a must-be-blocked xa scores in the sweep below
	}
	for am := m.am; am.any(); am = am.rest() {
		a := am.first()
		if m.mb.has(a) && m.cnt[a] > 0 {
			score++
		}
		deficit += m.owed(a, int(m.cnt[a]))
	}
	for i := m.nb - 1; i >= 0; i-- {
		m.luSuf[i] = m.luSuf[i+1]
		if m.able[i].and(m.lure).any() || m.duty.has(m.bl[i]) && m.able[i].any() {
			m.luSuf[i]++
		}
	}
	return
}

// gain is what blocker bl[i] on attacker a scores: 1 for a lure duty
// blocker or a blocker with a block duty (one unit per blocker, as gorge's
// blockRequiredCore counts a required blocker), 1 more for the first blocker
// of a must-be-blocked attacker.
func (m *mbSearch) gain(i int, a Slot) int {
	g := 0
	if m.lure.has(a) || m.duty.has(m.bl[i]) {
		g++
	}
	if m.mb.has(a) && m.cnt[a] == 0 {
		g++
	}
	return g
}

// owed is how many more blockers attacker a with c blockers needs.
func (m *mbSearch) owed(a Slot, c int) int {
	if c > 0 && c < int(m.min[a]) {
		return int(m.min[a]) - c
	}
	return 0
}

// run searches from blocker index 0 with the given score and deficit.
func (m *mbSearch) run(score, deficit int) {
	m.dfs(0, score, deficit)
}

// bound is an over-estimate of the score blockers bl[i:] can still add.
func (m *mbSearch) bound(i int) int {
	n := 0
	for um := m.mb; um.any(); um = um.rest() {
		if a := um.first(); m.cnt[a] == 0 && m.last[a] >= i {
			n++
		}
	}
	return m.luSuf[i] + min(n, m.nb-i) // each blocker left is the first on one attacker at most
}

// dfs assigns bl[i:]; it reports whether the search is done (the target was
// reached, or the budget ran out).
func (m *mbSearch) dfs(i, score, deficit int) bool {
	if m.nodes++; m.nodes > mbBudget {
		m.over = true
		return true
	}
	if deficit > m.nb-i {
		return false
	}
	thr := m.best + 1
	if m.want > thr {
		thr = m.want
	}
	if score+m.bound(i) < thr {
		return false
	}
	if i == m.nb {
		if deficit == 0 && score > m.best && !m.loneAlone() {
			m.best = score
			return m.want >= 0 && score >= m.want
		}
		return false
	}
	// attackers that score first, then the rest, then no block
	for pass := 0; pass < 2; pass++ {
		for am := m.able[i].and(m.am); am.any(); am = am.rest() {
			a := am.first()
			g := m.gain(i, a)
			if (g > 0) != (pass == 0) || m.one[a] && m.cnt[a] >= 1 {
				continue
			}
			d := deficit - m.owed(a, int(m.cnt[a])) + m.owed(a, int(m.cnt[a])+1)
			m.cnt[a]++
			m.assign(i, 1)
			done := m.dfs(i+1, score+g, d)
			m.assign(i, -1)
			m.cnt[a]--
			if done {
				return true
			}
		}
	}
	return m.dfs(i+1, score, deficit)
}

// assign counts bl[i] in (by +1) or out of (by -1) the completion.
func (m *mbSearch) assign(i, by int) {
	m.added += by
	if m.alone[i] {
		m.addedR += by
	}
}

// loneAlone: the completion has one blocker and it can't block alone.
func (m *mbSearch) loneAlone() bool {
	return m.nBlk+m.added == 1 && m.nBlkR+m.addedR == 1
}

// mbReach is the most requirements a valid completion of the declaration
// plus the extra block (xb blocks xa) meets, stopping early at want (-1: the
// maximum); -1 when none completes it.
func (s *State) mbReach(k *prep, lure, mb, duty slotMask, xb, xa Slot, want int, over *bool) int {
	var m mbSearch
	score, deficit := m.mbInit(s, k, lure, mb, duty, xb, xa)
	m.want = want
	m.run(score, deficit)
	*over = *over || m.over
	return m.best
}

// blockerOptionsReq is blockerOptions with must-be-blocked attackers in
// combat (mb), lure attackers included: a pair is offered when some
// completion of the declaration still meets the most requirements, Done when
// the declaration meets them.
//
//go:noinline
func (s *State) blockerOptionsReq(k *prep, d *Decision, mb slotMask) bool {
	var lure slotMask
	if k.gen.anyLure {
		lure = s.lureAttackers(k)
	}
	duty := s.blockDuty(k) // combat_blocksifable.go
	over := false
	q := s.mbReach(k, lure, mb, duty, NoSlot, NoSlot, -1, &over)
	for bm := s.ctrlM[s.blockDef()]; bm.any() && !over; bm = bm.rest() {
		b := bm.first()
		if !s.blockerReady(k, b) {
			continue
		}
		for am := s.blockAtkM(k); am.any(); am = am.rest() {
			a := am.first()
			if !s.aloneBlockOfferable(k, b, a) || s.perms[a].flags&pfAttacking == 0 || !s.canBlock(k, b, a) || s.menaceImpossible(k, a) ||
				k.gen.anyMaxBlk && s.blockCapped(k, a) || s.mbReach(k, lure, mb, duty, b, a, q, &over) < q {
				continue
			}
			if int(d.N) >= MaxOptions-1 {
				return s.optionsFull(d)
			}
			d.add(MakeAction(AkBlock, uint8(s.perms[b].obj), uint8(s.perms[a].obj), 0))
		}
	}
	if over {
		s.capFull(StallMustBlock)
		d.N = 0
		d.Kind = DkNone
		return true
	}
	if s.menaceDeficit(k, NoSlot, NoSlot, nil) == 0 && s.blocksCompletable(k, NoSlot, NoSlot) && !s.loneAloneBlocker(k) {
		var m mbSearch
		cur, _ := m.mbInit(s, k, lure, mb, duty, NoSlot, NoSlot)
		if cur >= q || DebugMustBlockGreedy && cur >= 1 {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	}
	return true
}
