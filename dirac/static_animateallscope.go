package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// overrideAll is OpOverrideAll: gorge effAnimateAll registers the animate
// grant on each battlefield permanent matching ValidCards$ at resolution
// (AliveFrom(0): seat 0's battlefield in entry order, then seat 1's).
//
//go:noinline
func (s *State) overrideAll(k *prep, in *kop.Instr) {
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	src := s.res.e.card
	p, t := s.overridePT(k, int(in.C))
	fl := s.ovDurFl(in.A.N)
	for _, sl := range ord[:cnt] {
		if s.occ.has(sl) {
			s.animAllTally(k, int(in.C), sl)
			s.applyOverridePT(k, sl, int(in.C), src, fl, p, t)
		}
	}
}

// overrideAllScoped is OpOverrideAll over the permanents of the player Obj
// names ("creatures target player controls": gorge targetPlayerKindScope in
// effAnimateAll); an unresolvable player sweeps nothing (gorge fails
// closed). The walk and the override are overrideAll's, restricted to the
// scoped player's permanents.
//
//go:noinline
func (s *State) overrideAllScoped(k *prep, in *kop.Instr) {
	only := s.playerRef(k, in.Obj)
	if only == 255 {
		return
	}
	AnimAllScoped.Add(1)
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	src := s.res.e.card
	p, t := s.overridePT(k, int(in.C))
	fl := s.ovDurFl(in.A.N)
	for _, sl := range ord[:cnt] {
		if (DebugAnimAllNoScope || s.ctrlM[only].has(sl)) && s.occ.has(sl) {
			s.animAllTally(k, int(in.C), sl)
			s.applyOverridePT(k, sl, int(in.C), src, fl, p, t)
		}
	}
}

// DebugAnimAllNoScope sweeps both seats (TestAnimallMutation proves the pool
// resolves a scoped AnimateAll).
var DebugAnimAllNoScope bool

// AnimAllScoped counts the scoped OverrideAll resolutions (read by
// TestAnimallLockstep; nothing in the engine reads it).
var AnimAllScoped atomic.Int64

// AnimAllRemoveKw counts the OverrideAll hits where a RemoveKeywords$ grant
// met a permanent that had the keyword; AnimAllLoseAb the hits where a
// RemoveAllAbilities$ grant met a permanent holding a keyword (read by
// TestAnimallLockstep; nothing in the engine reads them).
var AnimAllRemoveKw, AnimAllLoseAb atomic.Int64

// animAllTally counts the effect an OverrideAll record is about to have on sl.
//
//go:noinline
func (s *State) animAllTally(k *prep, idx int, sl Slot) {
	ov := &k.t.Overrides[idx]
	if ov.RemoveKw == 0 && !ov.LoseAbilities {
		return
	}
	have := s.kw(k, sl)
	if have&ov.RemoveKw != 0 {
		AnimAllRemoveKw.Add(1)
	}
	if ov.LoseAbilities && have != 0 {
		AnimAllLoseAb.Add(1)
	}
}
