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
// left out of every pool (the modal census's source). kicked judges the
// kicked cast: a kicked-range slot owes its kicked minimum (castSlotMin).
//
//go:noinline
func (s *State) distinctCastFeasible(k *prep, a *kop.Ability, p uint8, src Slot, modes uint8, excl Obj, kicked bool) bool {
	if !kicked && k.gen != nil && !k.gen.anyUniqLater {
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
			if castSlotMin(t, kicked) > 0 {
				slots[ns] = uint8(i)
				ns++
			}
			continue
		}
		if t.TFlags&kop.TgfDistinct != 0 && castSlotMin(t, kicked) > 0 {
			slots[ns] = uint8(i)
			ns++
			later = true
		}
	}
	if !later || ns < 2 {
		return true
	}
	var pools [kop.MaxTargets][MaxOptions]TCode
	var ln [kop.MaxTargets]int
	var need [kop.MaxTargets]uint8
	for j := 0; j < ns; j++ {
		t := &a.Targets[slots[j]]
		var all [MaxOptions]TCode
		c := s.targetCands(k, t, p, src, excl, &all)
		w := 0
		for i := 0; i < c; i++ {
			if x := all[i]; x != NoTarget && (excl == NoObj || x != TCode(excl)) {
				pools[j][w] = x
				w++
			}
		}
		m := castSlotMin(t, kicked)
		if w < int(m) {
			return false
		}
		ln[j], need[j] = w, m
	}
	// uniqueFeasible with no pick taken and no current-slot remainder:
	// every collected slot is a "later" pool of the shared matching
	return s.uniqueFeasible(TCode(NoTarget), nil, 0, &pools, &ln, &need, ns)
}

// castSlotMin is slot t's mandatory minimum for the offered cast: the kicked
// cast reads a kicked-range slot's kicked bound (Urborg Repossession's
// "another target permanent card", Count$Kicked.1.0), the plain cast the
// printed one (staticSlotMin).
func castSlotMin(t *kop.TargetSpec, kicked bool) uint8 {
	if kicked && t.TFlags&kop.TgfKickedRange != 0 {
		return t.MinKicked
	}
	return staticSlotMin(t)
}

// kickedChainFeasible is the kicked cast's half of the target census (gorge
// kickedChainTargetsAvailable, rules/target_kicked_chain.go): a spell whose
// kicked cast announces a TargetUnique$ link the plain cast does not owe
// must have the root and that link jointly answerable by distinct targets,
// or the kicked cast could only end in the CR 733.1 reversal. Any other
// spell is the plain census's alone.
func (s *State) kickedChainFeasible(k *prep, c CardID, p uint8) bool {
	ab := spellAb(k, c)
	if ab == 255 {
		return true
	}
	a := &k.t.Cards[c].Abilities[ab]
	owed := false
	for i := 1; i < int(a.NTargets); i++ {
		if t := &a.Targets[i]; t.TFlags&kop.TgfDistinct != 0 && t.TFlags&kop.TgfKickedRange != 0 && staticSlotMin(t) == 0 && t.MinKicked > 0 {
			owed = true
			break
		}
	}
	if !owed {
		return true
	}
	if k.gen != nil && s.g.probe.kind == 0 {
		s.g.probe = probe{kind: probeSpell, src: NoSlot, card: c, ctrl: p}
		defer func() { s.g.probe = probe{} }()
	}
	return s.distinctCastFeasible(k, a, p, NoSlot, 0xff, NoObj, true)
}
