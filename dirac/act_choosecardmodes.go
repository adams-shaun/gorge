package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Counted, random and each-player ChooseCard (kop.ChooseCardAmount ..
// ChooseCardActivatorBase). The plain and up-to-one forms stay in
// interp_gen.go/interp_ask.go; these are the shapes the chooser slice's
// remainder (AtRandom$, Amount$ X, Defined$ Player/TriggeredPlayer) needs.
//
// A counted pick is posed one card per decision (the kernel's existing
// multi-pick convention, as chooseLibUpTo), with the already picked cards
// removed from the pool. An AtRandom$ pick asks nobody: gorge's
// randomChoices draws from the engine rng over the choice pool in object ID
// order (engine/effects/choose_control.go), so the kernel walks g.objs the
// same way. The each-player forms walk AliveFrom(controller) (or that order
// rotated to the first opponent for StartingWith$ Opponent), asking each
// player one pick whose pool is matched from the activator, as gorge's
// cardChoicesIn does.

// chooseCardAsk is OpChooseCard's dispatch past the chooser: the object-side
// picks, the counted/random forms and the plain one-pick ask. Split out of
// execGen (its code-shape ceiling).
func (s *State) chooseCardAsk(k *prep, in *kop.Instr, p, base uint8) (ok, done bool) {
	if c := in.C; c == chooseAttachPick || c == chooseAttachPickOpt {
		return s.attachPick(k, in, p) // tgt_attachpick.go
	}
	if in.C == chooseAttachDest {
		return s.attachDest(k, p) // tgt_attachdest.go
	}
	if in.C == choosePutCounterPick {
		return s.putCounterPick(k, in, p) // ctr_putparams.go
	}
	if in.C == choosePopulatePick {
		return s.populatePick(k, in, p) // copy_picked.go
	}
	if in.C == kop.ChooseCardAtRandom {
		s.chooseCardAtRandom(k, in, base)
		return true, true
	}
	if in.C == kop.ChooseCardAmount || in.C == kop.ChooseCardAmountOpt {
		s.chooseCardAmountReset(k, in)
		if s.chooseCardAmountPose(k, in, p, base, true) {
			return false, true
		}
		return true, true
	}
	if s.cardCandsBase(k, in.F, p, base, nil) == 0 {
		return true, true
	}
	s.res.ask, s.res.askP = raGenChooseCard, p
	return false, true
}

// chooseCardAmountReset starts a counted pick.
func (s *State) chooseCardAmountReset(k *prep, in *kop.Instr) {
	s.g.nchosen = 0
	n := s.value(k, in.A)
	if n < 0 {
		n = 0
	}
	if n > len(s.g.chosen) {
		n = len(s.g.chosen)
	}
	s.res.left = uint8(n)
}

// chooseCardAmountPose poses the next counted pick; false when the pool is
// empty. excl removes this pick's earlier answers.
//
//go:noinline
func (s *State) chooseCardAmountPose(k *prep, in *kop.Instr, p, base uint8, excl bool) bool {
	if s.res.left == 0 || s.cardCandsBaseEx(k, in.F, p, base, nil, excl) == 0 {
		return false
	}
	s.res.ask, s.res.askP = raGenChooseCard, p
	return true
}

// chooseCardAtRandom is ChooseCardAtRandom: n A, drawn from the engine rng
// over the pool in object ID order, nobody asked. A pool no larger than n is
// taken whole without a draw (gorge randomChoices).
//
//go:noinline
func (s *State) chooseCardAtRandom(k *prep, in *kop.Instr, base uint8) {
	z := k.t.Filters[in.F].Zone
	if z == kop.ZoneNone {
		z = kop.ZoneBattlefield
	}
	var pool [MaxObj]Obj
	m := 0
	for o := Obj(0); o < s.nobj; o++ {
		ob := &s.objs[o]
		if ob.zone != z || s.pickOtherBarred(k, in.F, o) {
			continue
		}
		if s.matchCardGen(k, in.F, ob.card, ob.owner(), base, o) {
			pool[m] = o
			m++
		}
	}
	n := s.value(k, in.A)
	if n < 0 {
		n = 0
	}
	if n > m {
		n = m
	}
	// gorge randomChoices draws h.Rand(len(pool)) for every pick, a
	// one-candidate pool included (it consumes one IntN(1) draw); only an
	// empty pool draws nothing.
	for i := 0; i < n; i++ {
		j := s.rng.IntN(m)
		s.g.chosen[i] = pool[j]
		copy(pool[j:], pool[j+1:m])
		m--
	}
	s.g.nchosen = uint8(n)
}

// chooseCardEachOrder is the seat walk of an each-player ChooseCard:
// AliveFrom(controller). gorge chooseCardChoosers only rotates for
// StartingWith$ You (the default order anyway); StartingWith$ Opponent does
// not change the walk, as measured.
func (s *State) chooseCardEachOrder(k *prep, in *kop.Instr) seatSeq {
	return s.aliveFrom(s.res.e.ctrl)
}

// chooseCardEachPose poses the next player's pick; false when every player
// has answered (or has no candidate). first clears the chosen set.
//
//go:noinline
func (s *State) chooseCardEachPose(k *prep, in *kop.Instr, first bool) bool {
	if first {
		s.g.nchosen = 0
		s.g.pickLeft = 0
	}
	q := s.chooseCardEachOrder(k, in)
	for i := s.g.pickLeft; i < q.n; i++ {
		p := q.s[i]
		if s.out(p) {
			continue
		}
		if s.cardCandsBaseEx(k, in.F, p, s.res.e.ctrl, nil, false) == 0 {
			continue // an empty pool poses no decision (gorge Ask's guard)
		}
		s.g.pickP = p
		s.g.pickLeft = i + 1
		s.res.left = 1
		s.res.ask, s.res.askP = raGenChooseCard, p
		return true
	}
	s.g.pickLeft = 0
	return false
}

// applyChooseCard is raGenChooseCard's answer: a counted pick takes one card
// per decision and asks again, an each-player pick advances to the next
// player, a library-up-to pick has its own answer, and the plain forms store
// one card (or none).
func (s *State) applyChooseCard(k *prep, a Action) {
	in := &s.prog(k)[s.res.pc]
	if in.C == chooseLibUpTo {
		s.chooseLibAnswer(k, a) // zone_optionalmoves.go: asks again or continues
		return
	}
	if in.C == kop.ChooseCardAmount || in.C == kop.ChooseCardAmountOpt {
		if a.Kind() == AkDone {
			s.continueRes(k, 0)
			return
		}
		if int(s.g.nchosen) >= len(s.g.chosen) {
			s.capFull(StallRemember)
			return
		}
		s.g.chosen[s.g.nchosen] = Obj(a.Source())
		s.g.nchosen++
		if s.res.left > 0 {
			s.res.left--
		}
		if s.chooseCardAmountPose(k, in, s.res.askP, s.res.e.ctrl, true) {
			return // ask the next card
		}
		s.continueRes(k, 0)
		return
	}
	if in.C == kop.ChooseCardEach {
		if int(s.g.nchosen) >= len(s.g.chosen) {
			s.capFull(StallRemember)
			return
		}
		s.g.chosen[s.g.nchosen] = Obj(a.Source())
		s.g.nchosen++
		if s.chooseCardEachPose(k, in, false) {
			return // the next player's pick
		}
		s.continueRes(k, 0)
		return
	}
	if a.Kind() == AkDone {
		s.g.nchosen = 0 // chose none (an up-to-one ask)
	} else {
		s.g.chosen[0] = Obj(a.Source())
		s.g.nchosen = 1
		if in.C == 2 {
			s.remember(Obj(a.Source())) // RememberChosen$
		}
	}
	s.continueRes(k, 0)
}
