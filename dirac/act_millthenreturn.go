package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Breach the Multiverse: "Each player mills ten cards. For each
// player, choose a creature or planeswalker card in that player's graveyard.
// Put those cards onto the battlefield under your control. Then each
// creature you control becomes a Phyrexian in addition to its other types."
//
// Three instructions after the ordinary Mill:
//
//   - ExtBreachChoose: gorge's RepeatEach Player { ChooseCard Defined$ You
//     ... ImprintChosen$ True }. The resolving controller first, then the
//     other player (repeatPlayers lists from the controller); a player whose
//     graveyard holds no creature or planeswalker card is skipped without an
//     ask, one holding exactly one still poses the (mandatory) ask. The
//     picks append to the chosen set (genState.chosen, the imprint) in pick
//     order; the progress rides genState.pickLeft (players handled) and
//     pickP (the player being asked). 0 B.
//   - ExtBreachReturn: gorge's ChangeZone Defined$ Imprinted GainControl$
//     Origin$ Graveyard Destination$ Battlefield. Every chosen card still in
//     a graveyard enters under the controller's control, in pick order, as
//     one entering event (two objects open BeginZoneBatch: the enters checks
//     are held to the end, entBatch).
//   - The "becomes a Phyrexian" part is the ordinary OpOverrideAll (a
//     permanent AddSub override on each creature the controller controls as
//     it resolves, CR 611.2c); OpClearChosen then forgets the imprint.

// raBreach is the ask of one graveyard pick.
const raBreach uint8 = 236

// breachCand reports whether graveyard card o is a creature or planeswalker
// card (gorge Creature./Planeswalker.RememberedPlayerCtrl over the graveyard).
func (s *State) breachCand(k *prep, o Obj) bool {
	return k.cards[s.objs[o].card].types&(kop.TCreature|kop.TPlaneswalker) != 0
}

// breachNext poses the pick of the next player (in resolution order) whose
// graveyard holds a candidate; false: every player is done.
func (s *State) breachNext(k *prep) bool {
	e := &s.res.e
	for i := s.g.pickLeft; i < s.nSeats(); i++ {
		p := (e.ctrl + i) % s.nSeats() // each player in turn order from the controller
		if s.out(p) {
			continue
		}
		pl := &s.pl[p]
		for _, o := range pl.grave[:pl.graveN] {
			if s.breachCand(k, o) {
				s.g.pickP, s.g.pickLeft = p, i+1
				s.res.ask, s.res.askP = raBreach, e.ctrl
				return true
			}
		}
	}
	s.g.pickLeft = 0
	return false
}

// breachStart is ExtBreachChoose: clears the imprint and poses the first
// pick; true: nothing to ask (every graveyard is without a candidate).
//
//go:noinline
func (s *State) breachStart(k *prep) bool {
	s.g.nchosen = 0
	s.g.pickLeft = 0
	return !s.breachNext(k)
}

// askOptionsBreach lists the pick: the candidates of the player in pickP
// (the tail of askOptionsTrigX).
//
//go:noinline
func (s *State) askOptionsBreach(k *prep, d *Decision) bool {
	if s.res.ask == raExileLand {
		return s.askOptionsExileLand(k, d) // act_exilelandchoose.go
	}
	if s.res.ask != raBreach {
		return s.askOptionsUnlockDoor(k, d) // face_unlockdoor.go
	}
	d.Sub = ChGenChooseCard
	pl := &s.pl[s.g.pickP]
	for _, o := range pl.grave[:pl.graveN] {
		if s.breachCand(k, o) {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return true
}

// applyAskBreach imprints the pick and goes on to the next player (the tail
// of applyAskTrigX).
//
//go:noinline
func (s *State) applyAskBreach(k *prep, a Action) bool {
	if s.res.ask == raExileLand {
		return s.applyAskExileLand(k, a) // act_exilelandchoose.go
	}
	if s.res.ask != raBreach {
		return s.applyAskUnlockDoor(k, a) // face_unlockdoor.go
	}
	s.res.ask = raNone
	if int(s.g.nchosen) < len(s.g.chosen) {
		s.g.chosen[s.g.nchosen] = Obj(a.Source())
		s.g.nchosen++
	} else {
		s.capFull(StallBreach)
		return true
	}
	if !s.breachNext(k) {
		s.continueRes(k, 0)
	}
	return true
}

// breachReturn is ExtBreachReturn.
//
//go:noinline
func (s *State) breachReturn(k *prep) {
	e := &s.res.e
	n := int(s.g.nchosen)
	if n > 1 && k.gen != nil && k.gen.anyBatch {
		defer s.batchClose(s.batchOpen()) // one graveyard-leave batch
	}
	var eb entBatch
	for i := 0; i < n; i++ {
		o := s.g.chosen[i]
		if s.objs[o].zone != kop.ZoneGraveyard {
			continue // gorge's Origin$ precondition: it left the graveyard
		}
		if s.occ.full() {
			s.fireEntries(k, &eb)
			s.stall(StallBoard)
			return
		}
		from := s.detach(o)
		if k.gen != nil && k.gen.anyMayLimit {
			s.countMayPlayLand(k, o, from)
		}
		sl := s.enterHeld(k, &eb, o, e.ctrl, false, from)
		s.fireLeavesGraveyardSelf(k, sl, o)
	}
	s.fireEntries(k, &eb)
}

func init() { stallNames[StallBreach] = "breach" }
