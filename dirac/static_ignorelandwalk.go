package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// S:Mode$ IgnoreLandWalk (kop.StatIgnoreLandWalk): a creature can block an
// attacker as though the landwalk line the static names were absent (gorge
// combat.landwalkIgnored; Crevasse, Great Wall, Lord Magnus, Street Savvy).
// Static.Landwalk is the land filter of the line it names (0 = every landwalk
// line), Static.Filter the would-be blocker (0 = any blocker), read against
// the static's controller and source. Each link of an attacker's landwalk
// filter is one landwalk line (CR 702.14c, kgen's landwalkFilter), so
// landwalkHitLinks asks per link whether a static ignores it for blocker b
// before it reads the defending player's lands.

// useIgnoreLandWalk arms the per-link read: a table with an IgnoreLandWalk
// static.
var useIgnoreLandWalk = newGenUse(func(t *kop.Table) bool {
	has := func(abs []kop.Ability) bool {
		for j := range abs {
			if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatIgnoreLandWalk {
				return true
			}
		}
		return false
	}
	for i := range t.Cards {
		if has(t.Cards[i].Abilities) {
			return true
		}
	}
	for i := range t.Emblems {
		if has(t.Emblems[i].Abilities) {
			return true
		}
	}
	return false
})

// LandwalkStats counts the landwalk and skulk evasion decisions a lockstep
// game reached (DebugLandwalkStats; TestLandwalkGrantsFourSeatLockstep reads
// it so a green game is one where the mechanic decided a block; nothing in the
// engine reads it).
type LandwalkStats struct {
	GrantedEvades  int64 // a granted landwalk (Aura, static, Pump) made the attacker unblockable by a blocker
	AttachedEvades int64 // of those, an attached host's grant (an Aura's or an Equipment's)
	IgnoredBlocks  int64 // an IgnoreLandWalk static let a blocker block an attacker whose landwalk line had a land to hit
	GrantedSkulk   int64 // a granted skulk forbade a blocker with greater power
	GrantedHorse   int64 // a granted horsemanship (a mark, kw_kwmark.go) forbade a blocker without it
}

// DebugLandwalkStats, when set, receives the counts.
var DebugLandwalkStats *LandwalkStats

// DebugIgnoreLandWalkOff switches every IgnoreLandWalk static off
// (TestIgnoreLandWalkMutation proves a pool where one decides a block diverges
// from gorge without it).
var DebugIgnoreLandWalkOff bool

// landwalkHitLinks is landwalkHit over a table holding an IgnoreLandWalk
// static: the defending player (blocker b's controller) controls a land
// matching a link of f that no static ignores for b.
func (s *State) landwalkHitLinks(k *prep, f kop.FilterID, b, a Slot) bool {
	for ; f != 0; f = k.t.Filters[f].Or {
		if s.landwalkIgnored(k, f, b) {
			if DebugLandwalkStats != nil && s.landwalkLinkHit(k, f, b, a) {
				DebugLandwalkStats.IgnoredBlocks++
			}
			continue
		}
		if s.landwalkLinkHit(k, f, b, a) {
			return true
		}
	}
	return false
}

// landwalkLinkHit: the defending player (blocker b's controller) controls a
// land matching the one landwalk line link of attacker a.
func (s *State) landwalkLinkHit(k *prep, link kop.FilterID, b, a Slot) bool {
	for m := s.ctrlM[s.perms[b].ctrl]; m.any(); m = m.rest() {
		sl := m.first()
		if s.typesAt(k, sl)&kop.TLand != 0 && s.matchPermOne(k, &k.t.Filters[link], sl, s.perms[a].ctrl, a, layerFull) {
			return true
		}
	}
	return false
}

// landwalkIgnored: an IgnoreLandWalk static in force names the landwalk line
// link (or every line) and lets blocker b block as though it were absent.
//
//go:noinline
func (s *State) landwalkIgnored(k *prep, link kop.FilterID, b Slot) bool {
	if DebugIgnoreLandWalkOff {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatIgnoreLandWalk {
			return true
		}
		if v := ab.Static.Landwalk; v != 0 && !landwalkSameLine(k.t, v, link) {
			return true
		}
		if !s.statCond(k, st) || ab.Static.Filter != 0 && !s.matchPerm(k, ab.Static.Filter, b, st.ctrl, st.sl) {
			return true
		}
		hit = true
		return false
	})
	return hit
}

// landwalkSameLine: the first link of the land filter named and the link
// are the same land clause (gorge compares the keyword's spec string).
func landwalkSameLine(t *kop.Table, named, link kop.FilterID) bool {
	a, b := t.Filters[named], t.Filters[link]
	a.Or, b.Or = 0, 0
	a.Zone, b.Zone = kop.ZoneNone, kop.ZoneNone
	return a == b
}
