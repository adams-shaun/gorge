package dirac

import "math/bits"

// slotLast is the valid-bit mask of a slot set's high word.
const slotLast = ^uint64(0) >> ((64 - MaxPerm%64) % 64)

// slotMask is a set of battlefield slots: slots 0..63 in lo, 64..127 in mid,
// 128.. in hi. Every battlefield slot set (occupancy, controller, trigger
// subscribers, the per-turn slot marks, local scratch sets) is one, so
// MaxPerm alone sets the battlefield's size.
//
// It is a three-field struct, not a [3]uint64: Go keeps no array of more
// than one element in registers, so the array form lived on the stack and
// the wide-board build measured about twice as slow per transition. SSA
// decomposes a struct of up to four fields into registers, so mask loops
// stay in registers and rest() no longer round-trips through memory. MaxPerm
// is bounded to 129..192 by the assertion below (two words held 65..128
// before 2026-11-19, when four-seat Commander pods filled 128); a wider
// board needs a fourth word, not a re-parameterised array.
type slotMask struct{ lo, mid, hi uint64 }

// The struct form is exactly three words, so 128 < MaxPerm <= 192. Indexing
// a [64]struct{} by MaxPerm-129 is a compile-time range assertion: a MaxPerm
// outside 129..192 is an out-of-bounds (or negative) constant index and
// fails the build rather than silently truncating slots. slotLast masks the
// high word to the MaxPerm%64 valid bits.
var _ = [64]struct{}{}[MaxPerm-129]

// Out-of-range slots (NoSlot) behave as the bare uint64 shift did: has is
// false, set / clr / slotBit touch nothing (Go shifts past the width give 0;
// the checks below reject slot MaxPerm and above).

// slotBit is the set holding only slot sl.
func slotBit(sl Slot) slotMask {
	switch {
	case sl < 64:
		return slotMask{lo: 1 << sl} // low word first: the normal board never fills it
	case sl < 128:
		return slotMask{mid: 1 << (sl & 63)}
	case sl < MaxPerm:
		return slotMask{hi: 1 << (sl & 63)}
	}
	return slotMask{}
}

// has reports whether sl is in m.
func (m slotMask) has(sl Slot) bool {
	switch {
	case sl < 64:
		return m.lo&(1<<sl) != 0
	case sl < 128:
		return m.mid&(1<<(sl&63)) != 0
	}
	return sl < MaxPerm && m.hi&(1<<(sl&63)) != 0
}

// set adds sl to m.
func (m *slotMask) set(sl Slot) {
	switch {
	case sl < 64:
		m.lo |= 1 << sl
	case sl < 128:
		m.mid |= 1 << (sl & 63)
	case sl < MaxPerm:
		m.hi |= 1 << (sl & 63)
	}
}

// clr removes sl from m.
func (m *slotMask) clr(sl Slot) {
	switch {
	case sl < 64:
		m.lo &^= 1 << sl
	case sl < 128:
		m.mid &^= 1 << (sl & 63)
	case sl < MaxPerm:
		m.hi &^= 1 << (sl & 63)
	}
}

// with is m ∪ {sl}; without is m \ {sl}. These value forms are for
// locals and parameters: calling set / clr (pointer receivers) on a local
// takes its address and keeps it in memory instead of a register. set / clr
// are for masks already in memory (State fields).
func (m slotMask) with(sl Slot) slotMask {
	switch {
	case sl < 64:
		m.lo |= 1 << sl
	case sl < 128:
		m.mid |= 1 << (sl & 63)
	case sl < MaxPerm:
		m.hi |= 1 << (sl & 63)
	}
	return m
}

func (m slotMask) without(sl Slot) slotMask {
	switch {
	case sl < 64:
		m.lo &^= 1 << sl
	case sl < 128:
		m.mid &^= 1 << (sl & 63)
	case sl < MaxPerm:
		m.hi &^= 1 << (sl & 63)
	}
	return m
}

// or is m ∪ o.
func (m slotMask) or(o slotMask) slotMask {
	return slotMask{m.lo | o.lo, m.mid | o.mid, m.hi | o.hi}
}

// and is m ∩ o.
func (m slotMask) and(o slotMask) slotMask {
	return slotMask{m.lo & o.lo, m.mid & o.mid, m.hi & o.hi}
}

// andNot is m \ o.
func (m slotMask) andNot(o slotMask) slotMask {
	return slotMask{m.lo &^ o.lo, m.mid &^ o.mid, m.hi &^ o.hi}
}

// not is the set of the MaxPerm slots not in m.
func (m slotMask) not() slotMask {
	return slotMask{^m.lo, ^m.mid, ^m.hi & slotLast}
}

// any reports whether m is non-empty.
func (m slotMask) any() bool {
	return m.lo|m.mid|m.hi != 0
}

// count is |m|.
func (m slotMask) count() int {
	return bits.OnesCount64(m.lo) + bits.OnesCount64(m.mid) + bits.OnesCount64(m.hi)
}

// many reports whether m holds two or more slots.
func (m slotMask) many() bool {
	if m.lo&(m.lo-1) != 0 || m.mid&(m.mid-1) != 0 || m.hi&(m.hi-1) != 0 {
		return true // two or more in one word
	}
	// each word holds at most one: two only if two words do
	n := 0
	if m.lo != 0 {
		n++
	}
	if m.mid != 0 {
		n++
	}
	if m.hi != 0 {
		n++
	}
	return n >= 2
}

// full reports whether every one of the MaxPerm slots is in m.
func (m slotMask) full() bool {
	return m.lo == ^uint64(0) && m.mid == ^uint64(0) && m.hi == slotLast
}

// first is m's lowest slot; m must be non-empty.
func (m slotMask) first() Slot {
	switch {
	case m.lo != 0:
		return Slot(bits.TrailingZeros64(m.lo))
	case m.mid != 0:
		return Slot(64 + bits.TrailingZeros64(m.mid))
	case m.hi != 0:
		return Slot(128 + bits.TrailingZeros64(m.hi))
	}
	return NoSlot
}

// rest is m without its lowest slot (m itself when empty). With first it is
// the iteration idiom, ascending slot order:
//
//	for m := x; m.any(); m = m.rest() {
//		sl := m.first()
//		...
//	}
func (m slotMask) rest() slotMask {
	switch {
	case m.lo != 0:
		m.lo &= m.lo - 1
	case m.mid != 0:
		m.mid &= m.mid - 1
	default:
		m.hi &= m.hi - 1
	}
	return m
}
