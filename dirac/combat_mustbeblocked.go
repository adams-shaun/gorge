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
//
// A blocker that may block two attackers (combat_extrablock.go) is one search
// entry with up to two pairs, and a declared one with room for a second is an
// entry with one more. Its unit still counts once (gorge's
// blockRequirementsSatisfied counts a required blocker at its first required
// pair), but each pair can be the first blocker of a must-be-blocked attacker.
// gorge's quota is the one-pair-per-blocker maximum over the whole declaration
// (blockRequiredCore), not what the declaration so far reaches: a second pair
// can pass it, so mbQuota searches the declaration afresh. The oracle's
// blockDutyFull also holds a blocker with a required pair to required pairs
// when the quota needs every such blocker (mbDutyFull).

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
		if in.C == kop.MustBeBlockedLure {
			u[mechLure], u[mechLurePost] = true, true // combat_lure.go
		} else {
			u[mechMustBlocked], u[mechMustBlockedPost] = true, true
		}
	}
}

// mustBlockNewTurn ends the grants as the turn begins.
func (s *State) mustBlockNewTurn(k *prep) {
	s.mSlZero(k, mechMustBlocked)
	s.mSlZero(k, mechMustBlockedPost)
	s.mSlZero(k, mechLure)
	s.mSlZero(k, mechLurePost)
	s.mSlZero(k, mechAttacksIfAble) // combat_attacksifable.go
	s.mSlZero(k, mechAttacksPlayer) // combat_attacksifable.go
	s.mSlZero(k, mechBlocksIfAble)  // combat_blocksifable.go
}

// mustBlockExt is ExtMustBeBlocked: Obj, or every battlefield permanent
// matching F with Obj RefNone (the suspectExt shape), is marked.
func (s *State) mustBlockExt(k *prep, in *kop.Instr) {
	if in.C == kop.MustBeBlockedLure {
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.lureMark(k, sl) // combat_lure.go
		}
		return
	}
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
	luSuf [MaxPerm + 1]int  // blockers bl[i:] whose unit is not yet scored and that may block some lure attacker, or have a block duty and may block any
	last  [MaxPerm]int      // by attacker slot: highest blocker index able to block it, -1 none
	want  int               // stop at the first valid completion scoring this much (-1: find the maximum)
	best  int               // best valid score so far, -1 none
	nodes int
	over  bool // past mbBudget
	// CR 506.5 (combat_alone.go): a completion with one blocker that can't
	// block alone is not valid.
	alone [MaxPerm]bool // by blocker index: bl[i] can't block alone
	// A blocker of two attackers (combat_extrablock.go) is one blocker with up
	// to two pairs, scoring its unit once; a declared blocker with room for a
	// second is a search entry too (decl: its first pair is in cnt and score).
	cap      [MaxPerm]uint8 // by blocker index: pairs the search may still give it
	decl     [MaxPerm]bool  // by blocker index: already declared against an attacker
	unit     [MaxPerm]bool  // by blocker index: a required pair of it is declared or assigned
	capSuf   [MaxPerm + 1]int
	dutyFull bool // gorge's blockDutyFull (dirac/oracle): a blocker with a required pair takes only required pairs
	nBlk     int  // blockers declared so far (xb included)
	nBlkR    int  // of them, those that can't block alone
	added    int  // free blockers the search has assigned
	addedR   int  // of them, those that can't block alone
}

// mbInit options.
const (
	mbFresh    = 1 << iota // the declaration from its start: every blocker that could block is free, none declared
	mbExtra                // a blocker of two attackers may take a second pair (combat_extrablock.go)
	mbDutyFull             // a blocker with a required pair takes only required pairs (mbQuota)
)

// mbInit loads the search from the declaration so far with the extra block
// (xb blocks xa; NoSlot for none): xa has one more blocker, and xb is
// declared, with room for a second pair when it may take two (mbExtra). It
// returns the score and deficit of that declaration.
func (m *mbSearch) mbInit(s *State, k *prep, lure, mb, duty slotMask, xb, xa Slot, opt int) (score, deficit int) {
	*m = mbSearch{s: s, k: k, lure: lure, mb: mb, duty: duty, best: -1, want: -1, dutyFull: opt&mbDutyFull != 0}
	fresh := opt&mbFresh != 0
	def := s.blockDef()
	for am := s.blockAtkM(k); am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking == 0 {
			continue
		}
		m.am = m.am.with(a)
		if !fresh {
			m.cnt[a] = s.perms[a].nblock
		}
		m.min[a] = uint8(s.minBlockers(k, a))
		m.one[a] = k.gen != nil && k.gen.anyMaxBlk && s.maxOneBlocker(k, a)
		m.last[a] = -1
	}
	for bm := s.ctrlM[def]; bm.any(); bm = bm.rest() {
		b := bm.first()
		p := &s.perms[b]
		declared := !fresh && (p.flags&pfBlocking != 0 || b == xb)
		var first, second Slot = NoSlot, NoSlot // its declared attackers
		unit := false
		if declared {
			first, second = p.blocking, s.xblkOf(b)
			if p.flags&pfBlocking == 0 {
				first = NoSlot
			}
			if b == xb {
				if first == NoSlot {
					first = xa
				} else {
					second = xa
				}
			}
			unit = duty.has(b) || m.lure.has(first) || second != NoSlot && m.lure.has(second)
			if unit {
				score++ // a lure duty blocker blocking a lure attacker, or a duty blocker blocking
			}
			m.nBlk++
			if s.aloneBlk(k, b) {
				m.nBlkR++
			}
		}
		var cp uint8
		switch {
		case !declared && (fresh && s.blockerFresh(k, b) || !fresh && s.blockerReady(k, b)):
			cp = 1
			if opt&mbExtra != 0 && s.blockCap(k, b) >= 2 {
				cp = 2
			}
		case declared && opt&mbExtra != 0 && second == NoSlot && s.blockCap(k, b) >= 2:
			cp = 1 // room for a second pair
		}
		if cp == 0 {
			continue
		}
		var able slotMask
		for am := m.am; am.any(); am = am.rest() {
			if a := am.first(); a != first && a != second && s.canBlock(k, b, a) && !s.menaceImpossible(k, a) {
				able = able.with(a)
			}
		}
		if m.dutyFull && !duty.has(b) && able.and(m.lure).any() {
			able = able.and(m.lure) // a blocker with a lure pair is held to it
		}
		for am := able; am.any(); am = am.rest() {
			m.last[am.first()] = m.nb
		}
		m.bl[m.nb], m.able[m.nb], m.alone[m.nb] = b, able, s.aloneBlk(k, b)
		m.cap[m.nb], m.decl[m.nb], m.unit[m.nb] = cp, declared, unit
		m.nb++
	}
	if xa != NoSlot {
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
		m.luSuf[i], m.capSuf[i] = m.luSuf[i+1], m.capSuf[i+1]+int(m.cap[i])
		if !m.unit[i] && (m.able[i].and(m.lure).any() || m.duty.has(m.bl[i]) && m.able[i].any()) {
			m.luSuf[i]++
		}
	}
	return
}

// blockerFresh is blockerReady without the declared-blocking test: b could
// block when the declaration began.
func (s *State) blockerFresh(k *prep, b Slot) bool {
	return s.perms[b].flags&pfTapped == 0 && s.isCreature(k, b) && s.kw(k, b)&kop.KwCantBlock == 0
}

// required: blocker bl[i] on attacker a obeys a requirement of the blocker (a
// lure duty blocker on a lure attacker, a blocker with a block duty).
func (m *mbSearch) required(i int, a Slot) bool {
	return m.lure.has(a) || m.duty.has(m.bl[i])
}

// gain is what blocker bl[i] on attacker a scores: 1 for a lure duty
// blocker or a blocker with a block duty (one unit per blocker, as gorge's
// blockRequiredCore counts a required blocker: its second pair adds none),
// 1 more for the first blocker of a must-be-blocked attacker.
func (m *mbSearch) gain(i int, a Slot) int {
	g := 0
	if !m.unit[i] && m.required(i, a) {
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
	return m.luSuf[i] + min(n, m.capSuf[i]) // each pair left is the first on one attacker at most
}

// dfs assigns bl[i:]; it reports whether the search is done (the target was
// reached, or the budget ran out).
func (m *mbSearch) dfs(i, score, deficit int) bool {
	if m.nodes++; m.nodes > mbBudget {
		m.over = true
		return true
	}
	if deficit > m.capSuf[i] {
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
			if (m.gain(i, a) > 0) != (pass == 0) || m.one[a] && m.cnt[a] >= 1 {
				continue
			}
			if m.place(i, a, score, deficit, false) {
				return true
			}
		}
	}
	return m.dfs(i+1, score, deficit)
}

// place gives blocker bl[i] attacker a (its second pair when second), then
// goes on: with a blocker that may take two, to each later attacker as its
// second pair and then without one.
func (m *mbSearch) place(i int, a Slot, score, deficit int, second bool) bool {
	g := m.gain(i, a)
	d := deficit - m.owed(a, int(m.cnt[a])) + m.owed(a, int(m.cnt[a])+1)
	wasUnit := m.unit[i]
	counted := !second && !m.decl[i]
	m.unit[i] = m.unit[i] || m.required(i, a)
	m.cnt[a]++
	if counted {
		m.assign(i, 1)
	}
	done := false
	if !second && m.cap[i] > 1 {
		for am := m.able[i].and(m.am); am.any() && !done; am = am.rest() {
			if a2 := am.first(); a2 > a && !(m.one[a2] && m.cnt[a2] >= 1) {
				done = m.place(i, a2, score+g, d, true)
			}
		}
	}
	if !done {
		done = m.dfs(i+1, score+g, d)
	}
	if counted {
		m.assign(i, -1)
	}
	m.cnt[a]--
	m.unit[i] = wasUnit
	return done
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
func (s *State) mbReach(k *prep, lure, mb, duty slotMask, xb, xa Slot, opt, want int, over *bool) int {
	if opt&mbDutyFull != 0 && xb != NoSlot && !duty.has(xb) && !lure.has(xa) && s.lureAble(k, xb, lure) {
		return -1 // the pair is not a required one of a blocker with a required pair (mbQuota's full)
	}
	var m mbSearch
	score, deficit := m.mbInit(s, k, lure, mb, duty, xb, xa, opt)
	m.want = want
	m.run(score, deficit)
	*over = *over || m.over
	return m.best
}

// lureAble: blocker b could block some lure attacker.
func (s *State) lureAble(k *prep, b Slot, lure slotMask) bool {
	for am := lure; am.any(); am = am.rest() {
		if a := am.first(); s.canBlock(k, b, a) && !s.menaceImpossible(k, a) {
			return true
		}
	}
	return false
}

// mbQuota is gorge's quota of the whole declaration (decision.RequiredQuota):
// the most requirements one pair a blocker meets, every blocker that could
// block free, as blockRequiredCore searches. With a blocker of two attackers
// the declaration so far can pass it (a second pair meets what no one-pair
// declaration does), so it is not read off the declaration so far. full
// reports gorge's blockDutyFull on the oracle's side (dirac/oracle
// blockDutyFull): the quota needs every blocker with a required pair, and no
// must-be-blocked attacker or team minimum is offered, so a blocker with a
// required pair takes no other.
func (s *State) mbQuota(k *prep, lure, mb, duty slotMask, over *bool) (q int, full bool) {
	var m mbSearch
	score, deficit := m.mbInit(s, k, lure, mb, duty, NoSlot, NoSlot, mbFresh)
	m.run(score, deficit)
	*over = *over || m.over
	nReq := 0
	for i := 0; i < m.nb; i++ {
		if m.duty.has(m.bl[i]) && m.able[i].any() || m.able[i].and(m.lure).any() {
			nReq++
		}
	}
	full = nReq > 0 && m.best >= nReq
	for am := m.am; am.any() && full; am = am.rest() {
		a := am.first()
		full = m.last[a] < 0 || !m.mb.has(a) && m.min[a] <= 1
	}
	return m.best, full
}

// extraBlockInPlay: a defending creature that could block two attackers is
// ready, or is declared against one or two.
func (s *State) extraBlockInPlay(k *prep) bool {
	for bm := s.ctrlM[s.blockDef()]; bm.any(); bm = bm.rest() {
		if b := bm.first(); s.blockCap(k, b) >= 2 && (s.blockerRoom(k, b) || s.perms[b].flags&pfBlocking != 0) {
			return true
		}
	}
	return false
}

// blockerOptionsReq is blockerOptions with must-be-blocked attackers in
// combat (mb), lure attackers included: a pair is offered when some
// completion of the declaration still meets the most requirements, Done when
// the declaration meets them. A blocker of two attackers is offered its second
// pair in the same way, against the quota of one pair a blocker (mbQuota).
//
//go:noinline
func (s *State) blockerOptionsReq(k *prep, d *Decision, mb slotMask) bool {
	var lure slotMask
	if k.gen.anyLure {
		lure = s.lureAttackers(k)
	}
	duty := s.blockDuty(k) // combat_blocksifable.go
	over := false
	opt, q := 0, 0
	if k.gen.anyExtraBlock && s.extraBlockInPlay(k) {
		var full bool
		q, full = s.mbQuota(k, lure, mb, duty, &over)
		if opt = mbExtra; full {
			opt |= mbDutyFull
		}
	} else {
		q = s.mbReach(k, lure, mb, duty, NoSlot, NoSlot, 0, -1, &over)
	}
	for bm := s.ctrlM[s.blockDef()]; bm.any() && !over; bm = bm.rest() {
		b := bm.first()
		if !s.blockerRoom(k, b) {
			continue
		}
		for am := s.blockAtkM(k); am.any(); am = am.rest() {
			a := am.first()
			if s.blocks(b, a) || !s.aloneBlockOfferable(k, b, a) || s.perms[a].flags&pfAttacking == 0 || !s.canBlock(k, b, a) || s.menaceImpossible(k, a) ||
				k.gen.anyMaxBlk && s.blockCapped(k, a) || s.mbReach(k, lure, mb, duty, b, a, opt, q, &over) < q {
				continue
			}
			if int(d.N) >= MaxOptions-1 {
				return s.optionsFull(d)
			}
			d.add(MakeAction(AkBlock, TCode(s.perms[b].obj), TCode(s.perms[a].obj), 0))
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
		cur, _ := m.mbInit(s, k, lure, mb, duty, NoSlot, NoSlot, opt)
		if cur >= q || DebugMustBlockGreedy && cur >= 1 {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	}
	return true
}
