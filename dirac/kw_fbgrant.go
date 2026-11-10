package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A battlefield static granting flashback to cards in its controller's
// graveyard (kop.StatFbGrant; AffectedZone$ Graveyard AddKeyword$ Flashback;
// Iroh, Grand Lotus; CR 702.34a). gorge derives the keyword on the card while
// the static applies (legal_walk_alt.go's flashback walk, cast_altcost.go
// flashbackCost), so the kernel reads the static live: no mark is stamped on
// the card (the mech arena's flashback planes hold an Effect's or a Pump's
// grant, arenaobj.go fbGranted), and no State byte records it. A grant
// without a cost of its own costs the card's mana cost, as a stamped grant
// does; StFbOwnCost costs Static.GrantCost (Iroh's Lessons' {1}).

// fbStatic is the first StatFbGrant static in force that p controls whose
// filter matches card c owned by owner (read with p as "you"), or nil.
func (s *State) fbStatic(k *prep, p, owner uint8, c CardID) *kop.Static {
	if k.gen == nil || !k.gen.anyFbGrant {
		return nil
	}
	var got *kop.Static
	s.eachStaticC(k, sclsFbGrant, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatFbGrant || st.ctrl != p || st.sl == NoSlot {
			return true
		}
		// the card is read as a card, wherever it lies (no zone, no context:
		// kgen lowers a type and YouOwn filter only)
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, c, owner, p, NoObj, nil) {
			return true
		}
		got = &a.Static
		return false
	})
	return got
}

// fbLive reports whether a StatFbGrant static gives o, a card in its owner's
// graveyard, flashback now.
func (s *State) fbLive(k *prep, o Obj) bool {
	if k.gen == nil || !k.gen.anyFbGrant {
		return false
	}
	ob := &s.objs[o]
	return ob.zone == kop.ZoneGraveyard && s.fbStatic(k, ob.owner(), ob.owner(), ob.card) != nil
}

// fbGrantCost is the mana cost of o's granted flashback instance cast by p:
// the card's mana cost, or a live grant's own cost (StFbOwnCost) when no
// stamped grant (marked) is beside it. It reads the card, not its zone, so the
// cost builder asks it with the spell on the stack.
func (s *State) fbGrantCost(k *prep, p uint8, o Obj, marked bool) kop.ManaCost {
	c := s.objs[o].card
	if !marked && k.gen != nil && k.gen.anyFbGrant {
		if st := s.fbStatic(k, p, s.objs[o].owner(), c); st != nil && st.StFlags&kop.StFbOwnCost != 0 {
			return st.GrantCost
		}
	}
	return k.t.Cards[c].Cost
}
