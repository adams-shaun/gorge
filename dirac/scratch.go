package dirac

import "unsafe"

// the decision scratch that is never live together
// shares one word array, genState.scr. Go has no unions, so each arm is a
// plain struct read through a typed accessor over the same words. An arm's
// bytes mean something only inside its live window, and every window starts
// by zeroing, fully writing or bounding the arm it takes:
//
//   - armCombat: the combat damage pass: the paused pass's frozen body
//     (replBody) and the damage division (divDone, divAmt). Live while
//     ro.on, or in ctlDivide. Zeroed when the step begins (damageStep
//     fresh), the body again by replOrderFreeze and replOrderEnd.
//   - build: the cast or activation being put on the stack. Live in
//     ctlBuild (buildLive). Every build init assigns a whole build{}.
//   - armRes: a resolution's scratch (look, pileOrd, dmgSeqOrd). Live while
//     the resolution is suspended on a decision. Bounded by scalars that stay
//     outside scr (nlook, lookSel, dmgSeq*); resolveTop resets them.
//   - armEvidence: an evidence ask's picks in answer order, past arms B and C
//     because the ask runs inside either. Live while evSel is nonzero, its
//     length evSel's bit count. Only arm A overlaps it, and arm A is not live
//     beside a build or a resolution.
//
// The discriminants (ctl, ro.on / n, dmgSeqOn / N / I / Pl, divA / divI /
// divRem / divFirst) are outside scr; DecisionHash reads an arm only under
// its live condition. TestScratchArmsExclusive asserts at most one holds.

// armCombat is arm A: the combat damage pass.
type armCombat struct {
	replBody
	divDone slotMask       // attackers whose division is chosen this damage step
	divAmt  [MaxPerm]uint8 // per blocker slot: damage its attacker assigns to it
}

// armRes is arm C: a resolution's decision scratch.
type armRes struct {
	look      [24]Obj       // cards looked at (dig / peek / reveal); 24 since kcaps2x (was 12: Ulamog digs twenty)
	lookOrig  [24]uint8     // each look slot's window index: digPick reorders look to pick order, rest keeps window order
	pileOrd   [12]uint8     // OpTwoPiles: the face-down pile's rem indices in pick order
	dmgSeqOrd [MaxPerm]Slot // a multi-event damage op's recipients, frozen (dmgSeqOn / N / I / Pl, outside scr)
	plc       libTally      // kop.ExtLibPlace's tally of the cards put into libraries (act_libplace.go); live from its arm op to its ask op, zeroed by scrResEnter
	jf        int16         // the resolution's join-forces total (ExtJoinForces / ExJoinForces); zeroed by scrResEnter
}

// armEvidence is the evidence ask's scratch beside a build or a resolution.
type armEvidence struct {
	_     [max(unsafe.Sizeof(build{}), unsafe.Sizeof(armRes{}))]byte
	picks [MaxDeck]Obj // the picks in answer order; evSel counts them
}

const (
	scrBytes = max(unsafe.Sizeof(armCombat{}), unsafe.Sizeof(build{}), unsafe.Sizeof(armRes{}), unsafe.Sizeof(armEvidence{}))
	scrWords = (scrBytes + 7) / 8
)

// each arm fits the region, and the region is a whole number of words
var (
	_ = [scrWords*8 - unsafe.Sizeof(armCombat{})]struct{}{}
	_ = [scrWords*8 - unsafe.Sizeof(build{})]struct{}{}
	_ = [scrWords*8 - unsafe.Sizeof(armRes{})]struct{}{}
	_ = [scrWords*8 - unsafe.Sizeof(armEvidence{})]struct{}{}
)

// the words are 8-aligned: no arm needs more
var (
	_ = [8 - unsafe.Alignof(armCombat{})]struct{}{}
	_ = [8 - unsafe.Alignof(build{})]struct{}{}
	_ = [8 - unsafe.Alignof(armRes{})]struct{}{}
)

func (s *State) combatArm() *armCombat { return (*armCombat)(unsafe.Pointer(&s.g.scr)) }
func (s *State) resArm() *armRes       { return (*armRes)(unsafe.Pointer(&s.g.scr)) }

// evPicks is the evidence ask's picks in answer order, bounded by evSel.
func (s *State) evPicks() *[MaxDeck]Obj { return &(*armEvidence)(unsafe.Pointer(&s.g.scr)).picks }

// roBody is the paused combat pass's frozen body (arm A; live while ro.on).
func (s *State) roBody() *replBody { return &s.combatArm().replBody }

// bld is the build in progress (arm B; live in ctlBuild, see buildLive).
func (s *State) bld() *build { return (*build)(unsafe.Pointer(&s.g.scr)) }

// scrCombatLive: arm A holds a combat damage pass.
func (s *State) scrCombatLive() bool { return s.g.ro.on || s.ctl == ctlDivide }

// scrBuildLive: arm B holds the build.
func (s *State) scrBuildLive() bool { return s.buildLive() }

// scrResLive: arm C holds a resolution suspended on a decision.
func (s *State) scrResLive() bool {
	return s.ctl == ctlResolve || s.ctl == ctlGenAsk && s.g.ctlRet == ctlResolve
}

// scrResEnter starts a resolution's scratch afresh (resolveTop): the
// scalars that bound every read of armRes (nlook, lookSel, dmgSeq*) are
// reset; the arrays are read only below them (look[:nlook], pileOrd below
// the picks made, dmgSeqOrd[:dmgSeqN]), each written before it is read, so
// the stale bytes of another arm are never seen and need no clearing.
func (s *State) scrResEnter() {
	s.g.nlook, s.g.lookSel = 0, 0
	s.g.dmgSeqOn, s.g.dmgSeqN, s.g.dmgSeqI, s.g.dmgSeqPl = false, 0, 0, 0
	s.g.plusBatchAsked = false // a new resolution cannot inherit a parked batch
	s.resArm().plc = libTally{}
	s.resArm().jf = 0 // the join-forces total is per resolution
}

// joinForcesAdd runs ExtJoinForces: it adds b to the resolution's join-forces
// total (saturating at 32767), or zeroes it when reset (the chain's trailing
// StoreSVar$ 0). Each ask is at most numberOfferCeiling and there are at most
// MaxSeats askers, so the total stays far below the saturation.
func (s *State) joinForcesAdd(b int, reset bool) {
	a := s.resArm()
	if reset {
		a.jf = 0
		return
	}
	a.jf = int16(min(int(a.jf)+max(b, 0), 32767))
}

// scrCombatEnter starts a combat damage step's scratch afresh (damageStep,
// fresh): the whole region, whichever arm used it last.
func (s *State) scrCombatEnter() { s.g.scr = [scrWords]uint64{} }
