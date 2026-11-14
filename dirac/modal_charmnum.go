package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Multi-pick charms ("choose two", "choose one or both", "choose one or
// more"): kop.Ability.ModesPick is the most modes (CharmNum$), ModesFew how
// many fewer may be chosen (CharmNum$ - MinCharmNum$). Cold and out of line;
// reached behind ModesPick > 1 or genPrep.anyCharmNum.
//
// The chosen modes ride stackEntry.mode; the modes resolve in printed order
// (OpCharm skips each unchosen block; gorge CharmResolveOrder, CR 608.2c).
// Two or more chosen modes that each declare a target slot are asked as
// gorge's ONE combined KTarget (target_modes.go askCharmModeTargets), the
// filled-slot mask in genState.mt.picked while mt.slot == mtCharm; when a
// chosen slot is 1..N they are asked one mode at a time instead
// (modal_charmseq.go).

// mtCharm marks genState.mt as the combined charm target step; no
// multiTargetStep slot (0..MaxTargets-1) ever takes this value.
const mtCharm = 0xFE

// DebugCharmNoDone withholds the Done a multi-pick charm offers once its
// minimum is met (mutation test only).
var DebugCharmNoDone bool

// DebugCharmAnyFeasible gives a multi-pick charm the single-mode offer gate
// (feasible when any one mode is; mutation test only).
var DebugCharmAnyFeasible bool

// DebugCharmSlotOrder asks a multi-pick charm's targets slot by slot
// instead of as one combined decision (mutation test only).
var DebugCharmSlotOrder bool

const (
	cmNext  = 0 // the stage is done: continue the build
	cmPosed = 1 // a decision is posed
	cmGone  = 2 // a trigger with too few eligible modes was removed
	cmAbort = 3 // a cast below its minimum was reversed
)

// prepCharmNum sets anyCharmNum: some ability picks several modes.
func prepCharmNum(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			g.anyCharmNum = g.anyCharmNum || t.Cards[i].Abilities[j].ModesPick > 1 || t.Cards[i].ModesPickTW > 1 || t.Cards[i].Entwine || t.Cards[i].Abilities[j].Repeats()
		}
	}
}

// charmMin is the fewest modes a is chosen with.
func charmMin(a *kop.Ability) int { return int(a.ModesPick) - int(a.ModesFew) }

// charmFeasible is the offer gate (gorge charmTargetsAvailable): at least
// the minimum number of modes can be chosen. A mode with no target is
// feasible.
//
//go:noinline
func (s *State) charmFeasible(k *prep, a *kop.Ability, p uint8, src Slot) bool {
	if a.Repeats() {
		return true // gorge: repeat || min <= the legal modes (a mode can fill every pick)
	}
	need := charmMin(a)
	if DebugCharmAnyFeasible {
		need = 1
	}
	n := 0
	for m := 0; m < int(a.NModes); m++ {
		if s.abilityTargetsFeasible(k, a, p, src, 1<<m) {
			n++
		}
	}
	return n >= need
}

// charmModesStep poses the next mode pick of a multi-pick charm: the
// unpicked feasible modes, plus Done once the minimum is met. With no mode
// left it asks nothing (gorge's max clamp and OnlyEmptyAnswer).
//
//go:noinline
func (s *State) charmModesStep(k *prep, a *kop.Ability, d *Decision) int {
	if a.Repeats() {
		return s.charmRepeatModes(k, a, d)
	}
	e := &s.bld().e
	var feas [4]bool
	n := 0
	for m := 0; m < int(a.NModes); m++ {
		if e.mode&(1<<m) == 0 && s.modeFeasible(k, a, m) {
			feas[m] = true
			n++
		}
	}
	need := charmMin(a)
	if k.t.Cards[e.card].ModesMinCount {
		if c, dyn := s.charmCap(k); dyn {
			need = int(c) // no MinCharmNum$: gorge's minimum is the count itself
		}
	}
	if tw := s.twModes(k); tw != 0 {
		need = int(tw) // a teamworked Teamwork charm (kw_teamwork.go)
	}
	if s.bld().modesN == 0 && (n < need || n == 0 && e.kind == seTrigger) {
		if e.kind == seTrigger {
			// CR 603.3c / gorge removeModelessTrigger: a "choose two" trigger
			// with one eligible mode is removed, not resolved with one, and
			// a trigger with no eligible mode is removed even at minimum 0
			s.sp--
			s.ctl = ctlPlaceTriggers
			return cmGone
		}
		s.reverseCast(k)
		return cmAbort
	}
	if n == 0 {
		s.bld().modesN = 255
		return cmNext
	}
	d.Kind, d.Player = DkModes, e.ctrl
	for m := 0; m < int(a.NModes); m++ {
		if feas[m] {
			d.add(MakeAction(AkMode, 0, 0, uint8(m)))
		}
	}
	if int(s.bld().modesN) >= need && !DebugCharmNoDone {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return cmPosed
}

// charmSlots is the target slots owned by a chosen mode.
func charmSlots(a *kop.Ability, mode uint8) (mask uint8, n int) {
	if a.Repeats() {
		return charmRootSlots(a, mode)
	}
	for j := 0; j < int(a.NTargets); j++ {
		if a.TgtModes[j]&mode != 0 {
			mask |= 1 << j
			n++
		}
	}
	return
}

// charmCombined reports whether the build's targets are the combined step:
// a multi-pick charm with two or more chosen targeted modes.
//
//go:noinline
func (s *State) charmCombined(k *prep) bool {
	if s.g.mt.active && s.g.mt.slot == mtCharm {
		return true
	}
	a := s.buildAbility(k)
	if a == nil || DebugCharmSlotOrder || max8(a.ModesPick, s.twModes(k)) < 2 {
		return false
	}
	for j := 0; j < int(a.NTargets); j++ {
		if a.TgtModes[j] == 0 {
			panic("kernel: multi-pick charm with a non-mode target")
		}
	}
	_, n := charmRootSlots(a, s.bld().e.mode)
	return n >= 2 && !charmSeqSlots(a, s.bld().e.mode)
}

// charmCands lists each chosen slot's candidates (the ordinary per-slot
// census: targetCands, then affordableTargets). A slot of a repeated mode's
// extra instance takes its mode's spec (kop.Ability.SlotOwner).
func (s *State) charmCands(k *prep, a *kop.Ability, mask uint8, bufs *[kop.MaxTargets][MaxOptions]TCode, ns *[kop.MaxTargets]int) {
	e := &s.bld().e
	for j := 0; j < kop.MaxTargets; j++ {
		if mask&(1<<j) == 0 {
			continue
		}
		t, _ := a.SlotOwner(e.mode, j)
		n := s.targetCands(k, &a.Targets[t], e.ctrl, s.srcSlot(), e.obj, &bufs[j])
		if k.gen != nil {
			n = s.affordableTargets(k, &bufs[j], n)
		}
		if n == 0 {
			panic("kernel: multi-pick charm mode without a legal target")
		}
		ns[j] = n
	}
}

// charmOcc is the occurrence byte of candidate c in slot j: how many chosen
// earlier slots (mask, ascending) list it too (the oracle's disambiguate over
// gorge's group-major listing).
func charmOcc(mask uint8, j int, c TCode, bufs *[kop.MaxTargets][MaxOptions]TCode, ns *[kop.MaxTargets]int) uint8 {
	var order [kop.MaxTargets]uint8
	n := 0
	for i := 0; i < j; i++ {
		if mask&(1<<i) != 0 {
			order[n] = uint8(i)
			n++
		}
	}
	return charmOccOrder(&order, n, c, bufs, ns)
}

// charmOccOrder is charmOcc over the first p slots of order.
func charmOccOrder(order *[kop.MaxTargets]uint8, p int, c TCode, bufs *[kop.MaxTargets][MaxOptions]TCode, ns *[kop.MaxTargets]int) uint8 {
	occ := uint8(0)
	for q := 0; q < p; q++ {
		i := order[q]
		for x := 0; x < ns[i]; x++ {
			if bufs[i][x] == c {
				occ++
				break
			}
		}
	}
	return occ
}

// charmTargetsStep poses the combined target decision: for every unfilled
// chosen slot (gorge's group order: printed order), each candidate, tagged
// with its occurrence. It returns false when the ask would overflow
// MaxOptions; the caller stalls.
//
//go:noinline
func (s *State) charmTargetsStep(k *prep, d *Decision) bool {
	if s.charmTargetAskDead(k) {
		// gorge's pre-target gate (cast_targets.go) runs ahead of the
		// combined ask: the pool cannot pay and no untapped source is left
		// to tap, so the cast is reversed before any target
		// (kw_spree.go)
		s.reverseCast(k)
		return false
	}
	a := s.buildAbility(k)
	mt := &s.g.mt
	if !mt.active {
		*mt = multiTgt{active: true, slot: mtCharm}
	}
	order, no := charmAskOrder(a, s.bld().e.mode)
	mask, _ := charmRootSlots(a, s.bld().e.mode)
	var bufs [kop.MaxTargets][MaxOptions]TCode
	var ns [kop.MaxTargets]int
	s.charmCands(k, a, mask, &bufs, &ns)
	total := 0
	for p := 0; p < no; p++ {
		if j := order[p]; mt.picked&(1<<j) == 0 {
			total += ns[j]
		}
	}
	if int(d.N)+total > MaxOptions {
		s.capFull(StallCharmOpts)
		return false
	}
	d.Kind, d.Player = DkTarget, s.bld().e.ctrl
	for p := 0; p < no; p++ {
		j := order[p]
		if mt.picked&(1<<j) != 0 {
			continue
		}
		for x := 0; x < ns[j]; x++ {
			c := bufs[j][x]
			d.add(MakeAction(AkTarget, 0, c, charmOccOrder(&order, p, c, &bufs, &ns)))
		}
	}
	return true
}

// applyCharmTarget records one pick of the combined step into the slot whose
// occurrence of the candidate is the action's mode byte.
//
//go:noinline
func (s *State) applyCharmTarget(k *prep, a Action) {
	e := &s.bld().e
	mt := &s.g.mt
	ab := s.buildAbility(k)
	order, no := charmAskOrder(ab, e.mode)
	mask, _ := charmRootSlots(ab, e.mode)
	var bufs [kop.MaxTargets][MaxOptions]TCode
	var ns [kop.MaxTargets]int
	s.charmCands(k, ab, mask, &bufs, &ns)
	c := a.Target()
	for p := 0; p < no; p++ {
		j := int(order[p])
		if mt.picked&(1<<j) != 0 {
			continue
		}
		in := false
		for x := 0; x < ns[j]; x++ {
			in = in || bufs[j][x] == c
		}
		if !in || charmOccOrder(&order, p, c, &bufs, &ns) != a.Mode() {
			continue
		}
		e.tgt[j] = c
		if isObjTarget(c) {
			e.tver[j] = s.objs[c].ver
		}
		if j+1 > int(e.nt) {
			e.nt = uint8(j + 1)
		}
		if k.gen != nil && k.gen.crimeOn {
			// the combined ask is judged when its mask completes, below
			s.crimePick(k, c, false)
		}
		mt.picked |= 1 << j
		if mt.picked == mask {
			if k.gen != nil && k.gen.crimeOn && e.kind != seSpell {
				// the committed window's TargetsChosen (act_commitcrime.go):
				// every mask slot's pick, the earlier ones included
				for jj := 0; jj < kop.MaxTargets; jj++ {
					if mask&(1<<jj) != 0 {
						s.noteCrime(k, e.ctrl, e.tgt[jj])
					}
				}
			}
			*mt = multiTgt{}
			s.bld().ti = uint8(charmChainNext(ab, e.mode, 0)) // the chain links follow the combined ask
		}
		return
	}
	panic("kernel: multi-pick charm target without a slot")
}
