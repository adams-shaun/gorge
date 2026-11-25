package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Learn (CR 701.48a; gorge effLearn, engine/effects/learn.go): "You may
// reveal a Lesson card you own from outside the game and put it into your
// hand, or discard a card to draw a card." kgen lowers DB$ Learn / AB$ Learn
// to one OpExt ExtLearn whose F is gorge's learnLessonSpec (a Lesson card the
// learner owns). The learner is the resolving controller.
//
// gorge poses one KChoose (LearnDecision): "nothing" first, then one "lesson"
// per Lesson card of the learner's sideboard in zone order, then "rummage"
// when the hand holds a card. The kernel poses the same list (ChLearn: mode 0
// nothing, mode 1 over each Lesson in object order, the order the sideboard
// was laid out in, mode 2 rummage), asked even when "nothing" is the only
// option. A lesson answer reveals the card (a public Note in gorge, no state)
// and moves it from the sideboard into the hand. A rummage poses the discard
// (gorge learnRummage's "discard" KChoose over the hand, hand order, asked
// even with one card; ChRecruitDiscard, the same canonical choose:discard),
// then draws one card, the dredge ask first when a candidate exists
// (gorge drawFor). Cold and out of line: a table with no Learn never reaches
// this file. The state grows by no bytes.

// ChLearn is gorge's learn election: mode 0 = do nothing, mode 1 (source =
// the sideboard card) = reveal that Lesson and put it into your hand, mode 2
// = discard a card, then draw a card. Explicit ordinal, the iota tails stay
// untouched.
const ChLearn ChoiceKind = 208

// Learn election modes (ChLearn).
const (
	learnNothing uint8 = 0
	learnLesson  uint8 = 1
	learnRummage uint8 = 2
)

// DebugLearnNoLesson is the mutation switch: no Lesson is ever offered, so a
// game where a Lesson is fetched diverges from gorge. Tests only.
var DebugLearnNoLesson bool

// LearnStats, when set (dirac/oracle/learn_lockstep_test.go), counts the
// learn answers by arm.
type LearnStats struct {
	Nothing, Lessons, Rummages atomic.Int64
}

// DebugLearnStats, when set, counts the answers.
var DebugLearnStats *LearnStats

// learnExt is OpExt ExtLearn: the learn election is posed of the resolving
// controller.
//
//kmanifest:ext ExtLearn
func (s *State) learnExt(k *prep, in *kop.Instr) (ok, done bool) {
	s.res.ask, s.res.askP = raLearn, s.res.e.ctrl
	return false, true
}

// learnOptions poses the election: nothing, each Lesson card the learner
// owns in their sideboard (object order), then the rummage with a card in
// hand.
//
//kmanifest:askopts raLearn
func (s *State) learnOptions(k *prep, d *Decision) {
	p := s.res.askP
	d.Sub = ChLearn
	d.add(MakeAction(AkChoose, 0, 0, learnNothing))
	if !DebugLearnNoLesson {
		f := s.prog(k)[s.res.pc].F
		for o := Obj(0); int(o) < int(s.nobj); o++ {
			if ob := &s.objs[o]; ob.zone == kop.ZoneSideboard && ob.owner() == p && s.matchCardGen(k, f, ob.card, p, p, o) {
				d.add(MakeAction(AkChoose, TCode(o), 0, learnLesson))
			}
		}
	}
	if s.pl[p].handN > 0 {
		d.add(MakeAction(AkChoose, 0, 0, learnRummage))
	}
}

// learnAnswer applies the election. A Lesson answer that no longer names a
// sideboard card the learner owns moves nothing (gorge learnTakeLesson).
//
//kmanifest:askapply raLearn
func (s *State) learnAnswer(k *prep, a Action) {
	p := s.res.askP
	if st := DebugLearnStats; st != nil {
		[3]*atomic.Int64{&st.Nothing, &st.Lessons, &st.Rummages}[min(a.Mode(), learnRummage)].Add(1)
	}
	switch a.Mode() {
	case learnLesson:
		s.res.ask = raNone
		if o := Obj(a.Source()); int(o) < int(s.nobj) && s.objs[o].zone == kop.ZoneSideboard && s.objs[o].owner() == p {
			from := s.detach(o)
			s.placeFromK(k, o, kop.ZoneHand, false, from, false, false)
		}
	case learnRummage:
		if s.pl[p].handN > 0 {
			s.res.ask = raLearnDiscard
			return // the discard is posed; its answer draws
		}
		s.res.ask = raNone
	default:
		s.res.ask = raNone
	}
	s.continueRes(k, 0)
}

// learnDiscardOptions poses the rummage's discard: one pick over the hand in
// hand order.
//
//kmanifest:askopts raLearnDiscard
func (s *State) learnDiscardOptions(k *prep, d *Decision) {
	d.Sub = ChRecruitDiscard // gorge's KChoose option kind "discard" (canon choose:discard)
	pl := &s.pl[s.res.askP]
	for i := 0; i < int(pl.handN); i++ {
		d.add(MakeAction(AkChoose, TCode(pl.hand[i]), 0, 0))
	}
}

// learnDiscardAnswer discards the pick and draws a card. A pick no longer in
// the learner's hand discards and draws nothing (gorge learnRummage).
//
//kmanifest:askapply raLearnDiscard
func (s *State) learnDiscardAnswer(k *prep, a Action) {
	p := s.res.askP
	s.res.ask = raNone
	o := Obj(a.Source())
	pl := &s.pl[p]
	inHand := false
	for i := 0; i < int(pl.handN); i++ {
		if pl.hand[i] == o {
			inHand = true
			break
		}
	}
	if inHand && s.objs[o].owner() == p {
		s.discardObj(k, o)
		if !s.dredgeResPose(k, p, dredgeResUpto, 1) {
			return // the dredge ask is posed (kw_dredge.go); its answer's tail goes on
		}
		s.draw(p)
	}
	s.continueRes(k, 0)
}

// LearnKey is a ChLearn option's place in gorge's option order: nothing, the
// Lessons in object order (the sideboard's zone order), then the rummage.
func LearnKey(src TCode, mode uint8) uint32 {
	if mode == learnLesson {
		return 1<<24 | uint32(src)
	}
	return uint32(mode) << 24
}
