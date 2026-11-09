package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A charm that may choose a mode more than once (CanRepeatModes$; kop/repeat.go
// has the encoding): stackEntry.mode holds a count per mode. gorge poses one
// modes decision (Repeatable, Max = CharmNum$, MaxSum = the Pawprint$ budget
// over the modes' own prices); the kernel poses it as single picks, each
// offering every feasible mode the answer so far can still take (the oracle's
// multi-pick grammar), plus Done once the minimum is met.

// charmRepeatModes poses the next mode pick of a repeatable charm. It asks
// nothing when the maximum is reached or no pick is left with nothing owed.
//
//go:noinline
func (s *State) charmRepeatModes(k *prep, a *kop.Ability, d *Decision) int {
	e := &s.bld().e
	picked := a.ModesPicked(e.mode)
	spent := a.PawSpent(e.mode)
	n := 0
	var feas [4]bool
	if picked < int(a.ModesPick) {
		for m := 0; m < int(a.NModes); m++ {
			if a.RepMax[m] == 0 || a.RepBudget != 0 && spent+int(a.RepCost[m]) > int(a.RepBudget) {
				continue
			}
			if s.modeFeasible(k, a, m) {
				feas[m] = true
				n++
			}
		}
	}
	need := charmMin(a)
	if picked >= int(a.ModesPick) || n == 0 && picked == 0 && need == 0 {
		s.bld().modesN = 255
		return cmNext
	}
	if n == 0 && picked < need {
		s.reverseCast(k)
		return cmAbort
	}
	d.Kind, d.Player = DkModes, e.ctrl
	for m := 0; m < int(a.NModes); m++ {
		if feas[m] {
			d.add(MakeAction(AkMode, 0, 0, uint8(m)))
		}
	}
	if picked >= need && !DebugCharmNoDone {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return cmPosed
}

// charmRepeatEnter is OpCharm's repeat form: it reports whether instance r of
// mode m runs (m chosen more than r times). A later instance of a targeted
// mode runs with its target swapped into the mode's template slot, the slot
// the mode's block reads, so no earlier-or-later instance's target is lost.
//
//go:noinline
func (s *State) charmRepeatEnter(k *prep, c int16) bool {
	m, r := int(c>>9)&3, int(c>>5)&7
	e := &s.res.e
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.ModeCount(e.mode, m) <= r {
		return false
	}
	if r == 0 {
		return true
	}
	t, j := a.Template(m), a.InstanceSlot(e.mode, m, r)
	if t < 0 || j < 0 || t == j {
		return true
	}
	e.tgt[t], e.tgt[j] = e.tgt[j], e.tgt[t]
	e.tver[t], e.tver[j] = e.tver[j], e.tver[t]
	bt, bj := s.res.bad>>uint(t)&1, s.res.bad>>uint(j)&1
	s.res.bad = s.res.bad&^(1<<uint(t)|1<<uint(j)) | bj<<uint(t) | bt<<uint(j)
	return true
}

// charmAskOrder is the slots the combined target ask groups, in the order
// gorge lists the groups: a plain charm's chosen root slots ascending, a
// repeat charm's instances in printed order.
func charmAskOrder(a *kop.Ability, mode uint8) (order [kop.MaxTargets]uint8, n int) {
	if a.Repeats() {
		return a.AskOrder(mode)
	}
	mask, _ := charmRootSlots(a, mode)
	for j := 0; j < int(a.NTargets); j++ {
		if mask&(1<<j) != 0 {
			order[n] = uint8(j)
			n++
		}
	}
	return
}
