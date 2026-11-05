package search

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// landsearch.go: the land keys, each off by default and the search exactly
// as without it when off.
//
//   - LandSearch (:landsearch): a priority decision whose bot answer is a land play is a
//     searched point, the bot's land candidate 0 beside pass, casts and
//     activations. Off, such a decision is played unsearched, the land
//     first, so the search never weighs the drop against an instant cast
//     before it or holding the land, and that window never reaches the
//     visit corpus. Candidate 0 is never cut by topk= and wins visit ties,
//     so a drop is deferred only for a line with strictly more root visits;
//     at such a root neither Noise (the clean prior is kept) nor Sample (a
//     draw stands only with more visits than the land, landDraw) moves the
//     drop: a review measured :explore:landsearch otherwise
//     deferring 53% of the searched drops and missing 1.5 a game. The
//     searching seat's in-walk land decisions are points too (LandWalk), and
//     a mirrored opponent's (OpponentMirror; LandOpp); an unmirrored
//     opponent's stays its bot's answer, whose uniform prior would skip
//     drops the bot never skips.
//   - LandPick (:landpick): with LandSearch, every other distinct land in hand (one per
//     card) is a candidate too, after pass and before the casts, so the
//     search, not hbot's basic-first rule, picks which land.
//   - DedupSearch (:dedupsearch): a library search
//     (DkChoose ChGenSearch) offers one candidate per distinct card (the
//     dirac lists one option per object: five Forests are five options),
//     keyed by card name so every world's re-dealt library shares them; the
//     decision is a choose point when the distinct count is within Limit.
//     Dirac's options are untouched (dirac/gorge lockstep holds).
//   - LandAbNow (:landabnow): a land's own non-mana
//     activated ability with a {T} or sacrifice-self cost (Evolving Wilds'
//     crack) is played the moment it is offered, unsearched, at the root and
//     at the in-walk decisions of the seat (and of a mirrored opponent).

// landOn reports whether LandSearch makes a land-play decision of d's
// player a point (the root's and the walk's seat, a mirrored opponent).
func (sr *Searcher) landOn(d *dirac.Decision) bool {
	return sr.Opts.LandSearch && (d.Player == sr.seat || sr.Opts.OpponentMirror)
}

// optCard names option a by its card for the land keys' "distinct" rules:
// a land play or a choose pick by its object's card, anything else by
// itself (so Done is its own).
func optCard(s *dirac.State, a dirac.Action) uint64 {
	switch a.Kind() {
	case dirac.AkPlayLand, dirac.AkChoose:
		return uint64(a.Kind())<<32 | uint64(s.ObjCard(dirac.Obj(a.Source())))
	}
	return 1<<40 | uint64(a)
}

// dupBefore reports whether option i of d repeats the card of the bot's
// option or of an earlier option (the first object of each card is kept,
// the bot's own first of all).
func dupBefore(s *dirac.State, d *dirac.Decision, bot, i int) bool {
	c := optCard(s, d.Options[i])
	if i != bot && optCard(s, d.Options[bot]) == c {
		return true
	}
	for j := 0; j < i; j++ {
		if j != bot && optCard(s, d.Options[j]) == c {
			return true
		}
	}
	return false
}

// addLands is LandPick: every land play of d other than the bot's, one per
// card, appended to pt up to limit.
func (sr *Searcher) addLands(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point) {
	var one [1]uint32
	for i := 0; i < int(d.N) && pt.n < limit; i++ {
		if i == bot || d.Options[i].Kind() != dirac.AkPlayLand || dupBefore(s, d, bot, i) {
			continue
		}
		one[0] = uint32(d.Options[i])
		pt.addSeq(one[:])
	}
}

// libSearch reports whether d is a library search's pick.
func libSearch(d *dirac.Decision) bool {
	return d.Kind == dirac.DkChoose && d.Sub == dirac.ChGenSearch
}

// kindAt is kindOf with DedupSearch: a library search with more options
// than Limit but at most Limit distinct cards is a choose point.
func (sr *Searcher) kindAt(s *dirac.State, d *dirac.Decision) uint8 {
	k := kindOf(d, &sr.Opts.Kinds, sr.Opts.Limit)
	if k == kindNone && sr.Opts.DedupSearch && sr.Opts.Kinds.Choose && libSearch(d) {
		n := 0
		for i := 0; i < int(d.N); i++ {
			if !dupBefore(s, d, 0, i) {
				n++
			}
		}
		if n <= sr.Opts.Limit {
			k = kindChoose
		}
	}
	return k
}

// singleDedup is single over distinct cards (DedupSearch): the bot's
// option first, then each option whose card no earlier one has.
func (sr *Searcher) singleDedup(s *dirac.State, d *dirac.Decision, bot, limit int, pt *point) {
	var one [1]uint32
	one[0] = uint32(d.Options[bot])
	pt.addSeq(one[:])
	for i := 0; i < int(d.N) && pt.n < limit; i++ {
		if i == bot || dupBefore(s, d, bot, i) {
			continue
		}
		one[0] = uint32(d.Options[i])
		pt.addSeq(one[:])
	}
}

// landAbility is the index of the first option of priority decision d
// that activates a land's own non-mana ability costing {T} or sacrificing
// the land (LandAbNow), else -1.
func landAbility(s *dirac.State, d *dirac.Decision) int {
	if d.Kind != dirac.DkPriority {
		return -1
	}
	t := s.Table()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != dirac.AkActivate || a.Target() != 0 || a.Mode()&0x80 != 0 {
			continue
		}
		cd := &t.Cards[s.ObjCard(dirac.Obj(a.Source()))]
		if cd.Types&kop.TLand == 0 || int(a.Mode()) >= len(cd.Abilities) {
			continue
		}
		if ab := &cd.Abilities[a.Mode()]; ab.Kind == kop.AbActivated && ab.Cost.Flags&(kop.CostTap|kop.CostSacSelf) != 0 {
			return i
		}
	}
	return -1
}

// landAbNow is LandAbNow at decision d: when d's player is the seat (or a
// mirrored opponent) and a land ability is offered, *bot becomes it and
// the decision is not searched.
func (sr *Searcher) landAbNow(s *dirac.State, d *dirac.Decision, bot *int) bool {
	if d.Player != sr.seat && !sr.Opts.OpponentMirror {
		return false
	}
	i := landAbility(s, d)
	if i < 0 {
		return false
	}
	*bot = i
	return true
}

// landDraw is Sample's pick c at a LandSearch land root (the bot's land
// candidate 0; played the visits it was drawn from): c when it plays a
// land (candidate 0 or a LandPick land) or has strictly more visits than
// the land, as only the argmax would defer the drop; else the land. A tie
// plays the land, as the argmax's does.
func (sr *Searcher) landDraw(c int, played *[MaxCands]int32) int {
	if c != 0 && played[c] <= played[0] && dirac.Action(sr.rootPt.acts[sr.rootPt.off[c]]).Kind() != dirac.AkPlayLand {
		return 0
	}
	return c
}

// landStats counts a LandSearch land root: searched, and whether the
// choice deferred the drop (not a land play) or picked another land.
func (sr *Searcher) landStats(best int) {
	sr.St.LandSearched = 1
	if k := dirac.Action(sr.rootPt.acts[sr.rootPt.off[best]]).Kind(); k != dirac.AkPlayLand {
		sr.St.LandDeferred = 1
	} else if best != 0 {
		sr.St.LandPicked = 1
	}
}

// landWalk counts an in-walk LandSearch land point (the seat's, or a
// mirrored opponent's).
func (sr *Searcher) landWalk(d *dirac.Decision) {
	if d.Player == sr.seat {
		sr.St.LandWalk++
	} else {
		sr.St.LandOpp++
	}
}
