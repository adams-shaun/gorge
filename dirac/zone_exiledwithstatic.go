package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The static readers of the exiled-with link: Keen-Eyed
// Curator's card-types count over its own exile, Dawnhand Dissident's and
// Maralen's "cards exiled with this" permission filters. A filter reads the
// link as kop.FExiledWith against the matcher's source: a static's own object
// (the evalCtx it passes), else the resolving entry's (what exwCard read, kept
// for every resolution-time mover). CR 406.6 / 607.2a: the second ability
// refers only to the cards the first one exiled.

// DebugExwStaticNoSrc makes a static's exiled-with flag read nothing (its
// source dropped), which the lockstep must catch. Never set outside that test.
var DebugExwStaticNoSrc bool

// exwFlag is the FExiledWith test for card object o: ctx's source when the
// matcher is a static's, else the resolving entry's (outside a resolution
// nothing matches, as exwCard).
func (s *State) exwFlag(o Obj, ctx *evalCtx) bool {
	if o == NoObj {
		return false
	}
	if ctx == nil {
		return s.exwCard(o)
	}
	if DebugExwStaticNoSrc || ctx.src == NoObj {
		return false
	}
	trig, fire := false, uint8(0)
	if ctx.e != nil {
		trig, fire = ctx.e.kind == seTrigger, ctx.e.srcVer
	}
	ok := s.exwHas(o, ctx.src, trig, fire)
	s.exwNote(o, ctx.src, ok)
	return ok
}

// exwPermFlag is the FExiledWith test for the permanent object o against the
// source slot src: a resolving entry's own source reads as exwCard did (its
// fire ver counts), any other source slot (a static's) by its object.
func (s *State) exwPermFlag(o Obj, src Slot) bool {
	if src == NoSlot {
		return s.exwCard(o)
	}
	so := s.perms[src].obj
	if s.res.active && s.res.e.obj == so {
		return s.exwCard(o)
	}
	ok := s.exwHas(o, so, false, 0)
	s.exwNote(o, so, ok)
	return ok
}

// exwNote counts a static-source read in ExwStats (exwCard counts the
// resolution-time ones).
func (s *State) exwNote(o, src Obj, ok bool) {
	st := DebugExwStats
	if st == nil {
		return
	}
	st.Reads++
	if ok {
		st.Matched++
		if c := int(s.objs[src].card); c >= 0 && c < len(st.MatchBy) {
			st.MatchBy[c]++
		}
		if s.objs[src].zone != kop.ZoneBattlefield {
			st.AfterLeft++
		}
	} else if s.objs[o].zone == kop.ZoneExile {
		st.Rejected++
	}
}

// exileTypeAgg is ExCountExile with N 1: the number of distinct card types
// (CR 205.2a) among the printed faces of the exiled cards matching F, read in
// ctx (gorge zoneCountFold's CardTypes over the exile zones, graveTypeAgg's
// twin; Keen-Eyed Curator's "four or more card types among cards exiled with
// it"). Both seats' exile zones, in zone order.
//
//go:noinline
func (s *State) exileTypeAgg(k *prep, x *kop.Expr, ctx evalCtx) int {
	var types kop.TypeMask
	for pp := uint8(0); pp < MaxSeats; pp++ {
		pl := &s.pl[pp]
		for j := 0; j < int(pl.exileN); j++ {
			o := pl.exile[j]
			c := s.objs[o].card
			if s.matchCardGenAt(k, x.F, c, pp, ctx.ctrl, o, &ctx) {
				types |= k.t.Cards[c].Types
			}
		}
	}
	return bits.OnesCount16(uint16(types & cardTypeBits))
}
