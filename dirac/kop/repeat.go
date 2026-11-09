package kop

import "math/bits"

// A charm that may choose a mode more than once (CanRepeatModes$: Cosmium
// Confluence, Moment of Reckoning, the Seasons) keeps its chosen modes as a
// COUNT per mode in stackEntry.mode, in place of the bitmask a plain charm
// uses. Ability.RepMax[m] > 0 marks the ability and bounds mode m's count;
// mode m's count takes bits.Len8(RepMax[m]) bits, the modes in order from bit
// 0 (RepeatLayout), so the whole choice fits the byte when the widths sum to
// eight at most.
//
// Each mode's block is lowered once per instance: OpCharm with C carrying
// CharmRepeat runs instance r of mode m only when m was chosen more than r
// times, so the instances of a mode run in a row, in printed mode order (CR
// 608.2c).
//
// Target slots: a targeted mode owns one TEMPLATE slot (Ability.Targets, its
// TgtModes bit set); the mode's first instance stores its target there. A
// later instance stores its target in an overflow slot at NTargets or above,
// handed out in printed order to the extra instances of the targeted modes
// (SlotOwner). The block of such an instance runs with its target swapped into
// the template slot (the interpreter's charmRepeatEnter), so its program
// reads the one static slot.

// CharmRepeat flags an OpCharm C as the repeat form: bits 0-2 the count
// field's shift, 3-4 its width, 5-7 the instance, 9-10 the mode.
const CharmRepeat = 1 << 8

// RepeatCharmC packs the C of instance r of mode m for a repeat charm.
func RepeatCharmC(shift, width, r, m int) int16 {
	return int16(CharmRepeat | shift | width<<3 | r<<5 | m<<9)
}

// RepeatLayout is each mode's shift and width in the count byte, and the
// bits the layout needs.
func RepeatLayout(max [4]uint8) (shift, width [4]uint8, n int) {
	for m, v := range max {
		shift[m], width[m] = uint8(n), uint8(bits.Len8(v))
		n += int(width[m])
	}
	return
}

// Repeats reports whether the ability may choose a mode more than once.
func (a *Ability) Repeats() bool { return a.RepMax != [4]uint8{} }

// ModeCount is how many times mode m is chosen in mode.
func (a *Ability) ModeCount(mode uint8, m int) int {
	if !a.Repeats() {
		return int(mode>>uint(m)) & 1
	}
	shift, width, _ := RepeatLayout(a.RepMax)
	return int(mode>>shift[m]) & (1<<width[m] - 1)
}

// ModeAdd is mode with one more choice of mode m.
func (a *Ability) ModeAdd(mode uint8, m int) uint8 {
	if !a.Repeats() {
		return mode | 1<<uint(m)
	}
	shift, _, _ := RepeatLayout(a.RepMax)
	return mode + 1<<shift[m]
}

// ModeSet is the set of modes chosen at least once, a bit per mode.
func (a *Ability) ModeSet(mode uint8) uint8 {
	if !a.Repeats() {
		return mode
	}
	var set uint8
	for m := 0; m < int(a.NModes); m++ {
		if a.ModeCount(mode, m) > 0 {
			set |= 1 << uint(m)
		}
	}
	return set
}

// ModesPicked is how many modes mode holds, a repeat counted each time.
func (a *Ability) ModesPicked(mode uint8) int {
	n := 0
	for m := 0; m < int(a.NModes); m++ {
		n += a.ModeCount(mode, m)
	}
	return n
}

// PawSpent is the pawprints the chosen modes cost.
func (a *Ability) PawSpent(mode uint8) int {
	n := 0
	for m := 0; m < int(a.NModes); m++ {
		n += a.ModeCount(mode, m) * int(a.RepCost[m])
	}
	return n
}

// Template is the template slot of mode m, or -1 when the mode has no target.
func (a *Ability) Template(m int) int {
	for j := 0; j < int(a.NTargets); j++ {
		if a.TgtModes[j] == 1<<uint(m) {
			return j
		}
	}
	return -1
}

// extras is the number of overflow slots the modes below m take.
func (a *Ability) extras(mode uint8, m int) int {
	n := 0
	for i := 0; i < m; i++ {
		if a.Template(i) >= 0 && a.ModeCount(mode, i) > 1 {
			n += a.ModeCount(mode, i) - 1
		}
	}
	return n
}

// InstanceSlot is the slot holding instance r of mode m's target, or -1 for
// a mode without a target.
func (a *Ability) InstanceSlot(mode uint8, m, r int) int {
	t := a.Template(m)
	if t < 0 {
		return -1
	}
	if r == 0 || !a.Repeats() {
		return t
	}
	return int(a.NTargets) + a.extras(mode, m) + r - 1
}

// SlotOwner is the template slot whose spec target slot j of a repeat charm
// takes under mode, or ok false when no chosen instance owns j. A plain
// ability's slot is its own, chosen when TgtModes says so.
func (a *Ability) SlotOwner(mode uint8, j int) (tmpl int, ok bool) {
	if j < int(a.NTargets) {
		tm := a.TgtModes[j]
		return j, tm == 0 || tm&a.ModeSet(mode) != 0
	}
	if !a.Repeats() {
		return 0, false
	}
	e := j - int(a.NTargets)
	for m := 0; m < int(a.NModes); m++ {
		t := a.Template(m)
		if t < 0 || a.ModeCount(mode, m) < 2 {
			continue
		}
		if n := a.ModeCount(mode, m) - 1; e < n {
			return t, true
		} else {
			e -= n
		}
	}
	return 0, false
}

// AskOrder is the slots of the chosen targeted instances in printed order:
// the order gorge lists the per-instance target groups in (CR 608.2c).
func (a *Ability) AskOrder(mode uint8) (slots [MaxTargets]uint8, n int) {
	for m := 0; m < int(a.NModes); m++ {
		if a.Template(m) < 0 {
			continue
		}
		for r := 0; r < a.ModeCount(mode, m); r++ {
			if n < MaxTargets {
				slots[n] = uint8(a.InstanceSlot(mode, m, r))
				n++
			}
		}
	}
	return
}
