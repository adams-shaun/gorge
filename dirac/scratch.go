package dirac

import (
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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
//   - armRes: a resolution's scratch (look, pileOrd, dmgSeqOrd, dmgBRecs). Live while
//     the resolution is suspended on a decision. Bounded by scalars that stay
//     outside scr (nlook, lookSel, dmgSeq*); resolveTop resets them.
//   - armEvidence: an evidence ask's picks in answer order, past arms B and C
//     because the ask runs inside either. Live while evSel is nonzero, its
//     length evSel's bit count. Only arm A overlaps it, and arm A is not live
//     beside a build or a resolution.
//   - armAttackers: a declare-attackers declaration in progress (poseMask,
//     the attackers declared before the ask). Live in ctlAttackers; written
//     whole at the pose (turn.go).
//   - armBlockers: a declare-blockers declaration in progress (poseMask,
//     the blockers declared before the ask, earlier defenders' included).
//     Live in ctlBlockers; written whole at the pose (turn.go, combat.go
//     blockRound).
//
// The discriminants (ctl, ro.on / n, dmgSeqOn / N / I / Pl, dmgBOn / dmgBn, divA / divI /
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
	// voteCounts/voteCur/voteAll/voteAllN: a StoreVoteNum fixed-list vote's
	// per-option tallies and the option branch being resolved (act_vote.go).
	// The tallies are written once after the last voter; voteAll is live from
	// then through the last option branch, and voteCur is set by each flagged
	// branch before its body (ExVoteTally reads it).
	voteCounts [voteCMask + 1]uint8
	voteCur    uint8
	voteAll    bool
	voteAllN   uint8
	// dmgBRecs: a DamageAll sweep's latched DamageDealtOnce lines
	// (trig_dmgbatch.go): the sweep's per-recipient damage events are one
	// simultaneous batch (gorge effDamageAll's BeginDamageBatch, CR 510.4), so
	// each once line queues at its first matching event and dmgBatchClose
	// patches the queued entry's amount to the batch's matching-event total
	// (gorge trigger_match.go latchDamageBatch / closeDamageBatch). The open
	// flag and the line count (dmgBOn / dmgBn) stay outside scr: the records
	// are read only while dmgBOn holds, inside a resolution's sweep.
	dmgBRecs [maxDmgBatch]dmgBatchEnt
}

// armEvidence is the evidence ask's scratch beside a build or a resolution.
type armEvidence struct {
	_     [max(unsafe.Sizeof(build{}), unsafe.Sizeof(armRes{}))]byte
	picks [MaxDeck]Obj // the picks in answer order; evSel counts them
}

// armAttackers is the declare-attackers declaration in progress. poseMask is
// the pfAttacking mask as the declaration was posed: the must-attack
// attackers declareMustAttack declared before the ask (turn.go). CR 508.1c
// checks the declaration's restrictions while none of its chosen creatures
// is attacking yet (CR 508.2 makes the chosen creatures attacking after the
// checks), so attackerOptions lifts the picks served so far -- poseMask tells
// them apart from the pre-declared must-attack attackers -- and enumerates
// the pairs on the board as it stood at the pose. Live only in ctlAttackers;
// written whole at the pose.
type armAttackers struct {
	poseMask slotMask
}

// armBlockers is the declare-blockers declaration in progress. poseMask is
// the pfBlocking mask as the declaration was posed: an earlier defender's
// declaration (blockRound, CR 802.4) and nothing of the ask's own. CR 509.1b
// checks the declaration's restrictions while none of its chosen creatures
// is blocking yet (CR 509.2 makes the chosen creatures blocking after the
// checks), and gorge computes the whole option list at the pose, so the
// static blocking-state reads (filters_gen.go's FBlocking /
// FAttackingOrBlocking, filter_props.go's W7NotBlocking, matchCurOne,
// matchPerm) treat a pick served so far -- its mark lies beyond poseMask --
// as not blocking (blkLifted). Live only in ctlBlockers; written whole at
// the pose.
type armBlockers struct {
	poseMask slotMask
}

const (
	scrBytes = max(unsafe.Sizeof(armCombat{}), unsafe.Sizeof(build{}), unsafe.Sizeof(armRes{}), unsafe.Sizeof(armEvidence{}), unsafe.Sizeof(armAttackers{}), unsafe.Sizeof(armBlockers{}))
	scrWords = (scrBytes + 7) / 8
)

// each arm fits the region, and the region is a whole number of words
var (
	_ = [scrWords*8 - unsafe.Sizeof(armCombat{})]struct{}{}
	_ = [scrWords*8 - unsafe.Sizeof(build{})]struct{}{}
	_ = [scrWords*8 - unsafe.Sizeof(armRes{})]struct{}{}
	_ = [scrWords*8 - unsafe.Sizeof(armEvidence{})]struct{}{}
	_ = [scrWords*8 - unsafe.Sizeof(armAttackers{})]struct{}{}
	_ = [scrWords*8 - unsafe.Sizeof(armBlockers{})]struct{}{}
)

// the words are 8-aligned: no arm needs more
var (
	_ = [8 - unsafe.Alignof(armCombat{})]struct{}{}
	_ = [8 - unsafe.Alignof(build{})]struct{}{}
	_ = [8 - unsafe.Alignof(armRes{})]struct{}{}
	_ = [8 - unsafe.Alignof(armAttackers{})]struct{}{}
	_ = [8 - unsafe.Alignof(armBlockers{})]struct{}{}
)

func (s *State) combatArm() *armCombat { return (*armCombat)(unsafe.Pointer(&s.g.scr)) }
func (s *State) resArm() *armRes       { return (*armRes)(unsafe.Pointer(&s.g.scr)) }
func (s *State) atkArm() *armAttackers { return (*armAttackers)(unsafe.Pointer(&s.g.scr)) }
func (s *State) blkArm() *armBlockers  { return (*armBlockers)(unsafe.Pointer(&s.g.scr)) }

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

// scrAtkLive: the attackers arm holds a declaration in progress.
func (s *State) scrAtkLive() bool { return s.ctl == ctlAttackers }

// scrBlkLive: the blockers arm holds a declaration in progress.
func (s *State) scrBlkLive() bool { return s.ctl == ctlBlockers }

// DebugBlkLiftOff is a test-only mutation: it turns the blocker-pose lift
// off, so mid-declaration blocking marks reach the static reads again (the
// oracle's blockpose mutation must diverge).
var DebugBlkLiftOff bool

// blkLifted reports whether the pfBlocking mark of sl, if it is set, is one
// the pose board never carried: a pick served so far (its mark lies beyond
// the pose's mask, armBlockers), or a blocker an earlier defender declared
// past the pose captured for this defender's round (blockRound). The static
// blocking-state reads -- filters_gen.go matchPermOne's FBlocking and
// FAttackingOrBlocking, filter_props.go's W7NotBlocking, static_layerwalk.go
// matchCurOne's and chars.go matchPerm's -- treat such a permanent as not
// blocking, so every pick's option reads see the board as it stood at the
// pose (CR 509.1b: the declaration's restrictions are checked while none of
// its chosen creatures is blocking yet; CR 509.2 makes the chosen creatures
// blocking after the checks, and gorge computes the whole option list at the
// pose). The pick-progression reads that count the marks directly
// (blockerReady, blockDeclared, nblock) are not lifted: they are the kernel's
// emulation of the answer's own validation, not of the pose. Live only in
// ctlBlockers.
func (s *State) blkLifted(sl Slot) bool {
	if DebugBlkLiftOff {
		return false
	}
	return s.ctl == ctlBlockers && !s.blkArm().poseMask.has(sl)
}

// blkNotShown reports sl's blocking mark as the pose board carried it
// (CR 509.1b): unset, or one the pose never showed (blkLifted). The
// blocking-state reads' shared predicate.
func (s *State) blkNotShown(sl Slot, p *perm) bool {
	return p.flags&pfBlocking == 0 || s.blkLifted(sl)
}

// blkMarkFail is matchPermOne's and matchCurOne's blocking-state read: fl
// demands a blocking creature (FBlocking, or FAttackingOrBlocking of one
// that is not attacking) and sl's mark is not one the pose board carried
// (blkNotShown), so the read fails. Live only in ctlBlockers.
func (s *State) blkMarkFail(fl *kop.Filter, sl Slot, p *perm) bool {
	if fl.Flags&kop.FBlocking != 0 && s.blkNotShown(sl, p) {
		return true
	}
	return fl.Flags&kop.FAttackingOrBlocking != 0 && p.flags&pfAttacking == 0 && s.blkNotShown(sl, p)
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
	s.resArm().jf = 0          // the join-forces total is per resolution
	s.resArm().voteAll = false // a vote's all-options mode never outlives its resolution
	s.resArm().voteCur = 0
	s.g.dmgBOn, s.g.dmgBn = false, 0 // a new resolution's sweep batch is closed (trig_dmgbatch.go)
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
