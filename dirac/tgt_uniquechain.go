package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The TargetUnique$ chain look-ahead (gorge
// uniqueChainViableCandidates, rules/target_unique_chain.go): while a cast
// announces the root's targets or a TargetUnique$ link's (the kernel's slot
// 0, or a TgfDistinct slot), every candidate whose choice leaves a later
// mandatory TgfDistinct slot no distinct legal assignment is withheld --
// naming it could only end in the CR 733.1 reversal. Band Together with one
// creature on the table: "up to two target creatures you control" asks
// nothing, and "another target creature" takes it. Cold: reached only when
// genPrep.anyUniqLater.

// prepUniqueChain sets anyUniqLater: some ability has a mandatory
// TgfDistinct slot after its first.
func prepUniqueChain(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			for ti := 1; ti < int(a.NTargets); ti++ {
				if ts := &a.Targets[ti]; ts.TFlags&kop.TgfDistinct != 0 && staticSlotMin(ts) > 0 {
					g.anyUniqLater = true
					return
				}
			}
		}
	}
}

// staticSlotMin is a slot's printed minimum (the kicked and gift ranges
// read their unkicked bound; slotMinNow refines it).
func staticSlotMin(t *kop.TargetSpec) uint8 {
	if t.UpTo || slotNoneUnkicked(t) {
		return 0
	}
	if t.Min|t.Max != 0 {
		return t.Min
	}
	return 1
}

// slotMinNow is slot t's minimum for the entry being built
// (multiTargetStep's bounds).
func (s *State) slotMinNow(e *stackEntry, t *kop.TargetSpec) uint8 {
	if t.TFlags&kop.TgfKickedRange != 0 && e.flags&sfKicked != 0 {
		return t.MinKicked
	}
	return staticSlotMin(t)
}

// uniqueChainViable filters buf[:n], slot ti's candidates for the entry
// being built, to those that leave every later mandatory TgfDistinct slot a
// distinct assignment, and returns the new count (all of them when none is
// withheld, as gorge returns the candidates unchanged). The later slots'
// pools avoid the earlier slots' answers (gorge's chosen: nil for the
// root, the answers so far for a unique link's ask).
//
//go:noinline
func (s *State) uniqueChainViable(k *prep, ti uint8, buf *[MaxOptions]uint8, n int) int {
	e := &s.bld().e
	cur := tspecE(k, e, int(ti))
	if n == 0 || cur == nil || ti != 0 && cur.TFlags&kop.TgfDistinct == 0 {
		return n
	}
	a := s.buildAbility(k)
	// the units to assign: the current slot's remaining minimum after this
	// pick, then each later mandatory distinct slot's minimum
	var later [kop.MaxTargets][MaxOptions]uint8
	var ln [kop.MaxTargets]int
	var need [kop.MaxTargets]uint8
	nl := 0
	for j := int(ti) + 1; j < kop.MaxTargets; j++ {
		t := tspecE(k, e, j)
		if t == nil || t.Kind == kop.TgtNone || t.TFlags&kop.TgfDistinct == 0 || t.TFlags&kop.TgfAtResolution != 0 {
			continue
		}
		if a != nil {
			if tm := a.TgtModes[j]; tm != 0 && tm&a.ModeSet(e.mode) == 0 {
				continue
			}
		}
		m := s.slotMinNow(e, t)
		if m == 0 {
			continue
		}
		var all [MaxOptions]uint8
		c := s.targetCands(k, t, e.ctrl, s.srcSlot(), e.obj, &all)
		w := 0
		for i := 0; i < c; i++ {
			if x := all[i]; x != NoTarget && !s.pickedBefore(e, ti, x) {
				later[nl][w] = x
				w++
			}
		}
		ln[nl], need[nl] = w, m
		nl++
	}
	if nl == 0 {
		return n
	}
	// gorge judges a multi-object ask once, over its whole candidate list
	// with no pick made (the later slots avoid only the earlier slots'
	// answers): the slot's own picks so far rejoin its pool
	rest := int(s.slotMinNow(e, cur)) - 1
	var pool [MaxOptions + maxXTgt + 1]uint8
	np := copy(pool[:], buf[:n])
	if mt := &s.g.mt; mt.active && mt.slot == ti {
		rest = int(mt.min) - 1
		if mt.picked > 0 {
			pool[np] = e.tgt[ti]
			np++
			x := s.xtOf(e)
			for j := 0; j < int(x.n); j++ {
				if x.slot[j] == ti {
					pool[np] = x.code[j]
					np++
				}
			}
		}
	}
	rest = max(rest, 0)
	out := 0
	var keep [MaxOptions]uint8
	for i := 0; i < n; i++ {
		c := buf[i]
		if s.uniqueFeasible(c, pool[:np], rest, &later, &ln, &need, nl) {
			keep[out] = c
			out++
		}
	}
	if out == n {
		return n
	}
	copy(buf[:out], keep[:out])
	return out
}

// uniqueFeasible reports whether, with candidate c taken, the current
// slot's rest further picks (from cur less c) and each later slot's need
// (from its candidates less c) can all be distinct (gorge
// distinctTargetsFeasible): Kuhn's matching of the units to target codes.
func (s *State) uniqueFeasible(c uint8, cur []uint8, rest int, later *[kop.MaxTargets][MaxOptions]uint8, ln *[kop.MaxTargets]int, need *[kop.MaxTargets]uint8, nl int) bool {
	var m uniqMatch
	m.c, m.cur, m.later, m.ln = c, cur, later, ln
	for i := 0; i < rest; i++ {
		if m.nu == maxUniqUnits {
			return true // beyond any printed shape: never withhold
		}
		m.unit[m.nu] = -1
		m.nu++
	}
	for j := 0; j < nl; j++ {
		for i := 0; i < int(need[j]); i++ {
			if m.nu == maxUniqUnits {
				return true
			}
			m.unit[m.nu] = int8(j)
			m.nu++
		}
	}
	for u := 0; u < m.nu; u++ {
		m.seen = [256]bool{}
		if !m.try(u) {
			return false
		}
	}
	return true
}

const maxUniqUnits = 16

// uniqMatch is uniqueFeasible's matching scratch (stack only).
type uniqMatch struct {
	c     uint8
	cur   []uint8
	later *[kop.MaxTargets][MaxOptions]uint8
	ln    *[kop.MaxTargets]int
	unit  [maxUniqUnits]int8 // the slot a unit fills: -1 the current one, else a later index
	nu    int
	owner [256]int8 // target code -> unit + 1
	seen  [256]bool
}

func (m *uniqMatch) cands(u int) []uint8 {
	if j := m.unit[u]; j >= 0 {
		return m.later[j][:m.ln[j]]
	}
	return m.cur
}

// try finds unit u a code by an augmenting path.
func (m *uniqMatch) try(u int) bool {
	for _, x := range m.cands(u) {
		if x == m.c || x == NoTarget || m.seen[x] {
			continue
		}
		m.seen[x] = true
		if m.owner[x] == 0 || m.try(int(m.owner[x])-1) {
			m.owner[x] = int8(u + 1)
			return true
		}
	}
	return false
}
