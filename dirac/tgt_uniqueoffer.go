package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The TargetUnique$ chain offer census (gorge
// chainTargetsAvailable -> uniqueChainTargetsFeasible,
// rules/target_unique_chain.go): a cast or activation whose root target and
// mandatory TgfDistinct links cannot all be answered by pairwise-distinct
// legal targets is not offered (CR 601.2c, 115.3; it could only end in the
// CR 733.1 reversal). Incremental Growth with two creatures on the table:
// each of its three slots has a candidate, but no three distinct ones. The
// per-slot loop in abilityTargetsFeasible cannot see that; this joint
// matching can. The ask-time pruning stays uniqueChainViable's.

// distinctCastFeasible reports whether the root (the first mandatory
// target slot of the mode set) and every later mandatory TgfDistinct slot
// have a distinct assignment of their printed minimums. excl, when set, is
// left out of every pool (the modal census's source).
//
//go:noinline
func (s *State) distinctCastFeasible(k *prep, a *kop.Ability, p uint8, src Slot, modes uint8, excl Obj) bool {
	if k.gen != nil && !k.gen.anyUniqLater {
		return true
	}
	var slots [kop.MaxTargets]uint8
	ns, root, later := 0, false, false
	for i := 0; i < int(a.NTargets); i++ {
		t := &a.Targets[i]
		if t.Kind == kop.TgtNone || t.UpTo || t.TFlags&(kop.TgfAtResolution|kop.TgfGiftRange) != 0 {
			continue
		}
		if tm := a.TgtModes[i]; tm != 0 && tm&modes == 0 {
			continue
		}
		if !root {
			root = true
			if staticSlotMin(t) > 0 {
				slots[ns] = uint8(i)
				ns++
			}
			continue
		}
		if t.TFlags&kop.TgfDistinct != 0 && staticSlotMin(t) > 0 {
			slots[ns] = uint8(i)
			ns++
			later = true
		}
	}
	if !later || ns < 2 {
		return true
	}
	var pools [kop.MaxTargets][MaxOptions]uint8
	var ln [kop.MaxTargets]int
	var need [kop.MaxTargets]uint8
	for j := 0; j < ns; j++ {
		t := &a.Targets[slots[j]]
		var all [MaxOptions]uint8
		c := s.targetCands(k, t, p, src, excl, &all)
		w := 0
		for i := 0; i < c; i++ {
			if x := all[i]; TCode(x) != NoTarget && (excl == NoObj || x != uint8(excl)) {
				pools[j][w] = x
				w++
			}
		}
		m := staticSlotMin(t)
		if w < int(m) {
			return false
		}
		ln[j], need[j] = w, m
	}
	// uniqueFeasible with no pick taken and no current-slot remainder:
	// every collected slot is a "later" pool of the shared matching
	return s.uniqueFeasible(uint8(NoTarget), nil, 0, &pools, &ln, &need, ns)
}
