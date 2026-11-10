package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ensign Pavel Chekov: "each opponent exiles the top X cards of their library
// ... For each opponent, choose a land card that player owns exiled this way.
// You may play the chosen cards for as long as they remain exiled."
//
// The Dig remembers the exiled cards (the remembered set); then
//
//   - ExtExileLandChoose: gorge's RepeatEach Opponent { ChooseCard Defined$
//     You Choices$ Land.IsRemembered+RememberedPlayerOwn ChoiceZone$ Exile
//     Mandatory$ True ImprintChosen$ True }. The dirac engine is two-seat, so
//     the one opponent is the whole loop: the controller picks one of the
//     remembered exiled land cards that opponent owns (a mandatory
//     ChGenChooseCard ask, also posed with one candidate), none and no ask
//     when there is none. The pick is the chosen set (the imprint). No new
//     State: the chosen set, ask code and pick progress are existing fields.
//   - OpMayPlay over RefChosen (cast_mayplayremembered.go): a per-card play
//     permission record (genState.may) for each chosen card still in exile,
//     lasting while it remains exiled (DurPermanent; a card that leaves
//     exile has a new object version, so the record is dead).
//   - OpClearRemembered / OpClearChosen end the chain.

// raExileLand is the ask of one remembered-land pick.
const raExileLand uint8 = 235

// DebugExileLandOwn makes the pick offer the resolving controller's own
// remembered lands instead of the opponent's (a mutation check of the
// lockstep: the wrong player's land chosen).
var DebugExileLandOwn bool

// exileLandCand reports whether o is a remembered card in exile that is a
// land owned by player p (gorge Land.IsRemembered+RememberedPlayerOwn).
func (s *State) exileLandCand(k *prep, o Obj, p uint8) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneExile && ob.owner() == p && k.cards[ob.card].types&kop.TLand != 0
}

// exileLandStart is ExtExileLandChoose: clears the imprint and poses the
// pick of the one opponent's land; true: nothing to ask.
//
//go:noinline
func (s *State) exileLandStart(k *prep) bool {
	e := &s.res.e
	s.g.nchosen = 0
	p := s.nextAlive(e.ctrl) // two live seats only (kop.TwoSeatInstr: ExtExileLandChoose)
	if DebugExileLandOwn {
		p = e.ctrl
	}
	for i := 0; i < int(s.g.nrem); i++ {
		if s.exileLandCand(k, s.g.rem[i], p) {
			s.g.pickP = p
			s.res.ask, s.res.askP = raExileLand, e.ctrl
			return false
		}
	}
	return true
}

// askOptionsExileLand lists the pick: the remembered lands the player in
// pickP owns in exile (a tail of askOptionsBreach).
//
//go:noinline
func (s *State) askOptionsExileLand(k *prep, d *Decision) bool {
	d.Sub = ChGenChooseCard
	for i := 0; i < int(s.g.nrem); i++ {
		if o := s.g.rem[i]; s.exileLandCand(k, o, s.g.pickP) {
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
	return true
}

// applyAskExileLand imprints the pick and resumes the resolution (a tail of
// applyAskBreach).
//
//go:noinline
func (s *State) applyAskExileLand(k *prep, a Action) bool {
	s.res.ask = raNone
	s.g.chosen[0], s.g.nchosen = Obj(a.Source()), 1
	s.continueRes(k, 0)
	return true
}
