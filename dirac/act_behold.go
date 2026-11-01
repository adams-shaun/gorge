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
			if o == src || !s.matchCardGen(k, f, s.objs[o].card, p, p, o) {
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
	if cd.BeholdN == 0 || !(cd.BeholdExile || cd.HasAltAdd && s.bld().altAdd == 0 || optCostBehold(k, e)) {
		return false
	}
	d.N = 0
	n := s.beholdCands(k, e.ctrl, e.card, e.obj, d)
	if n < int(cd.BeholdN) {
		panic("kernel: behold cost no longer payable") // gorge aborts the cast; unreachable after the offer gate
	}
	if n == int(cd.BeholdN) {
		if cd.BeholdExile {
			s.beholdExRecord(Obj(d.Options[0].Source())) // the lone candidate is exiled too (act_beholdexile.go)
		}
		s.bld().donePicks |= pkBehold
		d.N = 0
		return false
	}
	s.bld().pickK, s.bld().pickLeft = pkBehold, cd.BeholdN
	d.Kind, d.Sub, d.Player = DkChoose, ChCostBehold, e.ctrl
	return true
}

// answerBehold consumes the behold answer: nothing moves or is marked.
func (s *State) answerBehold() {
	s.bld().donePicks |= pkBehold
	s.bld().pickK, s.bld().pickLeft = 0, 0
}
