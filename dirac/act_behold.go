package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Behold<1/Type> as part one of a spell's either-or additional cost (CR
// 701.4a, 601.2b; gorge cast_asks.go beholdCostAsk, reached from
// costObjectAsks after forage and reveal, before blight): the caster
// reveals or chooses a permanent they control or a card in hand matching
// the type, or pays the other part. Beholding moves and marks nothing, so
// the answer records no pick. Cold and out of line: CardDef.BeholdN/BeholdF
// are table data and the ask reuses the build's pickK / pickLeft /
// donePicks.

// ChCostBehold is gorge's "beholdcost" KChoose: source = the candidate.
const ChCostBehold ChoiceKind = 211

// pkBehold marks the behold pick (CostTap's bit is no pick kind). It
// carries pkExile's bit so the answer routes to answerCostObj, as pkBlight
// and pkReturn do.
const pkBehold = pickKind(kop.CostTap) | pkExile

// DebugBeholdHandFirst (mutation test only): the hand candidates are
// offered before the battlefield ones, which then join only when the hand
// has none. (A bare reordering is invisible to the lockstep, whose
// canonical options are sorted; the dropped battlefield candidates are not.)
var DebugBeholdHandFirst bool

// beholdCands counts (and, with d != nil, lists as AkChoose{source: the
// object}) the candidates of card c's behold part for p: p's permanents
// matching CardDef.BeholdF in battlefield order, then p's hand cards
// matching it, the cast card src excepted, in hand order (gorge
// CostCandidates ZBattlefield then ZHand with excludeSource).
func (s *State) beholdCands(k *prep, p uint8, c CardID, src Obj, d *Decision) int {
	f := k.t.Cards[c].BeholdF
	n := 0
	for pass := 0; pass < 2; pass++ {
		if (pass == 0) != DebugBeholdHandFirst {
			if DebugBeholdHandFirst && n > 0 {
				continue
			}
			for m := s.ctrlM[p]; m.any(); m = m.rest() {
				sl := m.first()
				if d != nil && s.beholdPicked(s.perms[sl].obj) {
					continue
				}
				if s.matchPerm(k, f, sl, p, NoSlot) {
					n++
					if d != nil {
						d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
					}
				}
			}
			continue
		}
		pl := &s.pl[p]
		for i := 0; i < int(pl.handN); i++ {
			o := pl.hand[i]
			if o == src || d != nil && s.beholdPicked(o) || !s.matchCardGen(k, f, s.objs[o].card, p, p, o) {
				continue
			}
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	}
	return n
}

// beholdPicked: o is an earlier pick of the behold being asked (a
// Behold<N/Type> of N > 1 is posed one pick at a time, as the lockstep
// walks gorge's one N-pick KChoose).
func (s *State) beholdPicked(o Obj) bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().picks[i] == o && s.bld().pickKs[i] == pkBehold {
			return true
		}
	}
	return false
}

// beholdPicks counts the behold picks recorded so far.
func (s *State) beholdPicks() int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkBehold {
			n++
		}
	}
	return n
}

// fbBeholdEntry: e is card cd's printed flashback cast, which owes the
// flashback cost's behold part (a granted flashback pays the mana cost
// only).
func fbBeholdEntry(cd *kop.CardDef, e *stackEntry) bool {
	return cd.FbBehold && e.flags&sfFlashback != 0 && e.flags&sfFbGranted == 0
}

// fbBeholdUnplanned: p's flashback cast of o pays the printed flashback
// cost's behold part, a shape gorge's cast planner declines ("cost:behold",
// rules/pay classify.go), so the oracle lowers the cast by tapping a witness
// at priority (beginCast). The granted instance pays mana only.
func (s *State) fbBeholdUnplanned(k *prep, p uint8, o Obj, mode uint8) bool {
	if mode&2 == 0 || !k.t.Cards[s.objs[o].card].FbBehold {
		return false
	}
	_, _, granted := s.fbInstances(k, SeatIx(p), o)
	return !granted
}

// beholdPayable: card c has no behold part, or p has enough candidates to
// pay it (src: the cast card, not a hand candidate).
func (s *State) beholdPayable(k *prep, p uint8, c CardID, src Obj) bool {
	cd := &k.t.Cards[c]
	return cd.BeholdN == 0 || s.beholdCands(k, p, c, src, nil) >= int(cd.BeholdN)
}

// beholdOptions poses the behold ask of the spell being built (bsCosts,
// ahead of the other cost-object picks, none of which a behold carrier
// owes): false when none is owed, the choice made, or a lone candidate
// taken unasked.
func (s *State) beholdOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind != seSpell || s.bld().donePicks&pkBehold != 0 {
		return false
	}
	cd := &k.t.Cards[e.card]
	if cd.BeholdN == 0 || !(cd.BeholdExile || cd.HasAltAdd && s.bld().altAdd == 0 || optCostBehold(k, e) || cd.MayFlashBehold && mayFlashEntry(e) || fbBeholdEntry(cd, e)) {
		return false
	}
	d.N = 0
	n := s.beholdCands(k, e.ctrl, e.card, e.obj, d)
	left := int(cd.BeholdN)
	if left > 1 {
		left -= s.beholdPicks() // the picks so far are no candidates
	}
	if n < left {
		// gorge offers every either-or part, the unpayable ones last, and a
		// behold part chosen without enough candidates (another player's
		// card cast through a grant, its owner's Merfolk not the caster's)
		// aborts the cast here (beholdCostAsk; CR 601.2h, 733.1).
		d.N = 0
		s.reverseCast(k)
		return true
	}
	if n == left {
		if cd.BeholdExile {
			s.beholdExRecord(Obj(d.Options[0].Source())) // the lone candidate is exiled too (act_beholdexile.go)
		}
		s.bld().donePicks |= pkBehold
		d.N = 0
		return false
	}
	s.bld().pickK, s.bld().pickLeft = pkBehold, uint8(left)
	d.Kind, d.Sub, d.Player = DkChoose, ChCostBehold, e.ctrl
	return true
}

// answerBehold consumes the behold answer a: nothing moves or is marked.
// A Behold<N/Type> of N > 1 records each pick (pkBehold) so the next one
// excludes it, and stays open until N are made.
func (s *State) answerBehold(a Action) {
	if b := s.bld(); s.k().t.Cards[b.e.card].BeholdN > 1 {
		s.recordPick(Obj(a.Source())) // under pickK pkBehold
		if s.beholdPicks() < int(s.k().t.Cards[b.e.card].BeholdN) {
			b.pickK, b.pickLeft = 0, 0 // beholdOptions poses the next one
			return
		}
	}
	s.bld().donePicks |= pkBehold
	s.bld().pickK, s.bld().pickLeft = 0, 0
}
