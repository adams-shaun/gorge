package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The member set of a one-or-more zone trigger's batch (Kaya, Spirits'
// Justice: "choose a creature card from among them"). gorge keeps, per queued
// ChangesZoneAll line, every object that line matched in the batch,
// deduplicated (rules/trigger_match.go zoneBatchEntry.moved), and patches it
// into the queued trigger's Remembered at batch close; the filter's
// TriggeredCards head reads it. The kernel's queued instance carries one
// object (trigObj), so a body that picks among the members (a CloneCopy with
// kop.CloneBatch) gets a record in the attacker-group pool (trig_attackers.go):
// pendingTrig.grp / stackEntry.grp index + 1, a fresh record per queued
// instance (never shared, because later members of the open batch are added to
// it), filled as the batch's later moves reach the same pending instance
// (batchTrigOK). A full pool stalls by name (StallAtkGrp).

// ChCloneBatchPick is gorge's clone_choice KChoose over the cards of a
// non-battlefield ChoiceZone$: source = the card picked.
const ChCloneBatchPick ChoiceKind = 246

// readsBatchPool reports whether a's program picks among its batch's members.
func readsBatchPool(a *kop.Ability) bool {
	for i := range a.Prog {
		if in := &a.Prog[i]; in.Ext() == kop.ExtCloneCopy && in.B.N&kop.CloneBatch != 0 {
			return true
		}
	}
	return false
}

// prepBatchPool sets genPrep.anyBatchPool: some trigger picks among its
// batch's members.
func prepBatchPool(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if readsBatchPool(&t.Cards[i].Abilities[j]) {
				g.anyBatchPool = true
				return
			}
		}
	}
}

// batchMemberAdd adds o to the member set of pending instance i (a duplicate
// of the batch line that o's move also matched).
//
//go:noinline
func (s *State) batchMemberAdd(i int, a *kop.Ability, o Obj) {
	if o == NoObj || !readsBatchPool(a) {
		return
	}
	if g := s.pend[i].grp; g != 0 {
		s.cold.atkGrp[g-1].set(o)
	}
}

// batchMemberNew opens the member set {o} for the pending instances
// n..npend-1 just queued from a batch line that picks among its members.
//
//go:noinline
func (s *State) batchMemberNew(n uint8, o Obj) {
	if len(s.cold.atkGrp) == 0 {
		s.stall(StallTables)
		return
	}
	free := uint8(0)
	for i := uint8(1); i <= maxAtkGrp && free == 0; i++ {
		if !s.grpUsed(i) {
			free = i
		}
	}
	if free == 0 {
		s.capFull(StallAtkGrp)
		return
	}
	var b objBits
	b.set(o)
	s.cold.atkGrp[free-1] = b
	s.g.atkHi = max(s.g.atkHi, free)
	for i := n; i < s.npend; i++ {
		s.pend[i].grp = free
	}
}

// cloneBatchCands lists the Choices$ pick of a batch pool: the batch's
// members still in exile that match F, each player's exile in zone order,
// seat 0 first (gorge cloneChoiceCandidates sweeps AliveFrom(0), each
// player's zone in insertion order).
func (s *State) cloneBatchCands(k *prep, in *kop.Instr, d *Decision) int {
	g := s.grpOf(&s.res.e)
	n := 0
	for p := range s.pl {
		pl := &s.pl[p]
		for i := 0; i < int(pl.exileN); i++ {
			o := pl.exile[i]
			if !g.has(o) {
				continue
			}
			ob := &s.objs[o]
			if !s.matchCardGen(k, in.F, ob.card, ob.owner(), s.res.e.ctrl, o) {
				continue
			}
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	}
	return n
}
