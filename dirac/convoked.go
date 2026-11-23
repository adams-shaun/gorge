package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The convoked creatures (T13b-2 remnants): CR 702.66's "each creature that
// convoked it", the picks a convoke cast paid its cost with. gorge folds the
// pay-time CastInfo's FlagConvoked IDs onto Object.Convoked, and
// sharesTypeReferents' Convoked referent reads them there
// (engine/effects/filter_attachment.go). The kernel kept only their COUNT
// (stackEntry.prov bits 3-7, Convoked$Amount, trig_gravedrawn.go); the set
// now rides a small pool (State's cold convoked) the cast's stack entry
// indexes (stackEntry.cvok, index+1; 0 = none), the attacker groups'
// pattern (trig_attackers.go). A record is free when no building, stacked
// or resolving spell holds its index. The body reads the set at resolution:
// a creature that has since left the battlefield is read where it now is,
// exactly as gorge's referent read does (an object id survives the zone
// change, CR 400.7).

const maxConvoked = 16

// StallConvokedSet: a seventeenth convoked set held at once.
const StallConvokedSet = 194

func init() { stallNames[StallConvokedSet] = "convoked" }

// coldConvoked is the convoked sets, for a table whose filter words read
// them (useConvokedReads).
var coldConvoked = newCold[objBits]("convoked", func(g *genPrep) int {
	if g.uses.has(useConvokedReads) {
		return maxConvoked
	}
	return 0
}).view(func(c *coldArrays) *[]objBits { return &c.convoked })

// useConvokedReads is the genPrep.uses bit for a table whose filter programs
// read the convoked set (FwSharesCr over RefConvoked); the cold pool's need
// holds it (the useFilterProg pattern, filter_prog.go).
var useConvokedReads = newGenUse(func(t *kop.Table) bool {
	for i := range t.Filters {
		f := &t.Filters[i]
		if f.Pred != kop.PredProg {
			continue
		}
		for _, l := range t.FilterProgs[f.PredN].Lits {
			if l.R == kop.RefConvoked {
				return true
			}
		}
	}
	return false
})

// cvokUsed reports whether record i (index+1) is held.
func (s *State) cvokUsed(i uint8) bool {
	if s.ctl == ctlBuild && s.bld().e.cvok == i || s.res.active && s.res.e.cvok == i {
		return true
	}
	for j := 0; j < int(s.sp); j++ {
		if s.stack[j].cvok == i {
			return true
		}
	}
	return false
}

// convokedNew captures the convoked creatures as a pool record and returns
// its index + 1: a held record with the same objects is shared; 0 when the
// pool is full (the game stalls, StallConvokedSet).
//
//go:noinline
func (s *State) convokedNew(k *prep, set []Obj) uint8 {
	var b objBits
	for _, o := range set {
		b.set(o)
	}
	free := uint8(0)
	for i := uint8(1); i <= maxConvoked; i++ {
		if !s.cvokUsed(i) {
			if free == 0 {
				free = i
			}
		} else if s.cold.convoked[i-1] == b {
			return i
		}
	}
	if free == 0 {
		s.capFull(StallConvokedSet)
		return 0
	}
	s.cold.convoked[free-1] = b
	s.g.cvokHi = max(s.g.cvokHi, free)
	return free
}

var noConvoked objBits

// stampConvoked records the cast being paid's convoked creatures on its
// stack entry (cast.go's convoke payment arm, beside the Convoked$Amount
// count): the cast's own picks, a colour pip or a generic, on the
// battlefield to tap (payConvoke's own gate); improvise (mode 6) and
// harmonize (mode 7) picks are not convoked.
func (s *State) stampConvoked(k *prep) {
	if !k.gen.uses.has(useConvokedReads) {
		return
	}
	var cv [maxConvoked]Obj
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		kd := s.bld().pickKs[i]
		if !isConvokePick(kd) || kd&7 > 5 || s.objs[s.bld().picks[i]].zone != kop.ZoneBattlefield {
			continue
		}
		cv[n] = s.bld().picks[i]
		n++
	}
	s.bld().e.cvok = s.convokedNew(k, cv[:n])
}

// convokedOf is entry e's convoked set, as objects (nil when it has none).
// The objects are read where they now are; a pick whose creature is no
// longer on the battlefield still names it (gorge's g.Obj check passes for
// an object that moved, CR 400.7).
func (s *State) convokedOf(e *stackEntry) []Obj {
	if e.cvok == 0 {
		return nil
	}
	b := &s.cold.convoked[e.cvok-1]
	var out []Obj
	for w := 0; w < len(b); w++ {
		for v := b[w]; v != 0; v &= v - 1 {
			out = append(out, Obj(w*64+bits.TrailingZeros64(v)))
		}
	}
	return out
}

// convokedHash mixes the held convoked sets into h: the index in a hashed
// entry names a record whose objects differ between states (atkGrpHash's
// reason, trig_attackers.go).
func (s *State) convokedHash(h uint64) uint64 {
	for i := uint8(1); i <= s.g.cvokHi; i++ {
		if s.cvokUsed(i) {
			h = hmix(h, uint64(i))
			for _, w := range s.cold.convoked[i-1] {
				h = hmix(h, w)
			}
		}
	}
	return h
}
