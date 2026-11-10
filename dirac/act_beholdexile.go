package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// BeholdExile<1/Type> as a spell's own mandatory additional cost
// (beholdexile; the Lorwyn Champion cycle's S:Mode$ RaiseCost | Cost$
// BeholdExile<1/Type>; CR 701.4a, 601.2b/f-h, 406.6, 607.2a). gorge folds the
// cost into the cast (foldRaiseExtra) and asks it as the either-or behold is
// asked (beholdCostAsk: fewer candidates than N abort the cast, exactly N are
// taken unasked, more pose "beholdcost"). At payment EmitChoiceCosts (after
// the discards and the life, damage, draw and return parts, before the blight
// counters, the taps and the sacrifices) exiles each beheld object with the
// cast spell as the MoveZone's source, setting Object.ExiledWith: the link the
// Champion's leaves trigger reads (Defined$ ExiledWith). Here the pick is
// recorded (pkBeholdEx), paid in payExilePicks' slot, and the reverse exw
// record written through exwPut. CardDef.BeholdExile is table data; the pool
// is the existing genState.exw (a full pool is StallExiledWith).

// pkBeholdEx marks a recorded BeholdExile pick (pickKs); the ask itself runs
// under pkBehold, whose pkExile bit routes the answer.
const pkBeholdEx = pickKind(kop.CostSacSelf) | pkExile

// DebugBeholdExileNoLink (mutation test only): the cost exile writes no
// exiled-with link, so the leaves trigger never returns the card.
var DebugBeholdExileNoLink bool

// beholdExRecord records a BeholdExile candidate as the spell's pick. It
// leaves pickLeft as it found it: the lone candidate is recorded outside any
// posed pick, and a pickLeft wrapped below zero would hold a later counter
// pick (pickSubCtr, Dawnhand Dissident's "removing three counters") open in
// costPickOptions, which then poses it with no candidate.
func (s *State) beholdExRecord(o Obj) {
	b := s.bld()
	left := b.pickLeft
	b.pickK = pkBeholdEx
	s.recordPick(o)
	b.pickK, b.pickLeft = 0, left
}

// answerBeholdPick consumes the behold answer a: the either-or's records
// nothing, a BeholdExile pick is the chosen candidate.
func (s *State) answerBeholdPick(a Action) {
	if s.k().t.Cards[s.bld().e.card].BeholdExile {
		s.beholdExRecord(Obj(a.Source()))
	}
	s.answerBehold(a)
}

// payBeholdExilePicks exiles each recorded BeholdExile pick and links it to
// the cast spell (payExilePicks' slot: after the discards, before the taps and
// sacrifices).
func (s *State) payBeholdExilePicks(k *prep) {
	e := &s.bld().e
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkBeholdEx {
			continue
		}
		o := s.bld().picks[i]
		ob := &s.objs[o]
		fl := exwRev
		if ob.of&ofToken != 0 {
			fl |= exwTok
		}
		switch ob.zone {
		case kop.ZoneBattlefield:
			s.leave(k, ob.slot, kop.ZoneExile, false)
		case kop.ZoneHand:
			from := s.detach(o)
			s.placeFrom(o, kop.ZoneExile, false, from, false)
		default:
			continue
		}
		if k.gen != nil && k.gen.anyExW && k.gen.exwRec[e.card] != 0 && !DebugBeholdExileNoLink {
			// the stamp is the ver the spell's permanent will have (gorge
			// stamps Incarnation(stack)+1, the permanent's): entering the
			// battlefield advances the ver by one
			s.exwPut(o, e.obj, s.objs[e.obj].ver+1, fl, int(e.card))
		}
	}
}
