package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplPayBeforeETB (Mox Diamond): "If this would enter, you may discard a land
// card instead. If you do, put it onto the battlefield. If you don't, put it
// into its owner's graveyard." gorge runs the discard, then the entry or the
// graveyard move, inside the replaced move while the spell is still on the
// stack, so no enters watcher ever sees a card that did not enter. The kernel
// runs the replacement's Prog (the optional, remembering land discard) as the
// permanent spell starts resolving (payETBStart); when the program ends,
// payETBGate re-enters the resolution with the discard paid and the card
// enters as any permanent spell does, or the spell leaves the stack for its
// owner's graveyard through the ordinary finish when nothing was discarded.
// The answered marker rides s.g.pre (akPayETB, state 2) until the entry
// completes, so no State byte is added. Entering by any other route (put onto
// the battlefield by an effect, a token copy) stalls: gorge asks there with
// the move still pending, and the kernel's entry is already made.

// StallPayETB: a card whose entry is replaced by ReplPayBeforeETB entered the
// battlefield other than by its cast spell resolving.
const StallPayETB = 170

func init() { stallNames[StallPayETB] = "paybeforeetb" }

// akPayETB marks s.g.pre as the gate's answered record (never posed).
const akPayETB uint8 = 30

// PayETBStats counts the gate's outcomes for the lockstep test's reach check
// (dirac/oracle/moxdiamond_test.go): spells that discarded a land and entered,
// and spells that discarded nothing and went to the graveyard.
type PayETBStats struct{ Entered, Graveyard int }

// DebugPayETBStats, when set, is filled as pay-before-entering programs end.
var DebugPayETBStats *PayETBStats

// payETBAb is the index of card c's ReplPayBeforeETB ability, or -1.
func payETBAb(k *prep, c CardID) int {
	if k.gen.replBits[c]&rbPayETB == 0 {
		return -1
	}
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplPayBeforeETB {
			return i
		}
	}
	return -1
}

// payETBPaid reports whether the gate has answered for object o.
func (s *State) payETBPaid(o Obj) bool {
	return s.g.pre.state == 2 && s.g.pre.kind == akPayETB && s.g.pre.obj == o
}

// payETBStart begins the permanent spell e's pay-before-entering program;
// false when its card has none or the gate already answered.
func (s *State) payETBStart(k *prep, e *stackEntry) bool {
	if k.gen.anyRepl&rbPayETB == 0 || s.payETBPaid(e.obj) {
		return false
	}
	ab := payETBAb(k, e.card)
	if ab < 0 {
		return false
	}
	s.res.e.ab = uint8(ab) // prog(k) reads the replacement's Prog
	s.runProg(k)
	return true
}

// payETBGate runs when a resolution's program ends: the pay-before-entering
// program of a spell that remembered a discard re-enters the resolution to
// enter the card; false (the ordinary finish) when it discarded nothing.
func (s *State) payETBGate(k *prep) bool {
	if k.gen.anyRepl&rbPayETB == 0 {
		return false
	}
	e := &s.res.e
	if e.kind != seSpell || payETBAb(k, e.card) != int(e.ab) {
		return false
	}
	if st := DebugPayETBStats; st != nil {
		if s.g.nrem == 0 {
			st.Graveyard++
		} else {
			st.Entered++
		}
	}
	if s.g.nrem == 0 {
		return false
	}
	s.g.pre = preAsk{obj: e.obj, kind: akPayETB, state: 2}
	s.resolveTop(k)
	return true
}

// payETBStray stalls the game when o, a card carrying the replacement, enters
// without the gate's answer (enterGen).
func (s *State) payETBStray(k *prep, o Obj) {
	if k.gen.anyRepl&rbPayETB != 0 && k.gen.replBits[s.objs[o].card]&rbPayETB != 0 && !s.payETBPaid(o) {
		s.capFull(StallPayETB)
	}
}

// The effect path: a pick-and-move effect (OpChooseMove / OpSearchEx's move
// phase, moveFoundAns) puts the card onto the battlefield. Gorge parks the
// move on the replacement and asks its controller -- the entry player -- the
// may-election of the replacement's own body (the same Optional$ discard
// TgtChoose asks the cast path poses), then a Min 1 land pick when more than
// one is eligible (the strict-supersets single card discards with no pick).
// The answer is carried in s.g.pre (akPayETB) until the re-run's gate
// consumes it: paid, the card enters as the ordinary move does; declined (or
// no eligible land, where gorge's discard resolves to nothing), the
// replacement chain's ChangeZone-to-graveyard branch moves it there instead.
// The chain's Cleanup ClearRemembered runs before the entrant's own
// RememberChanged$ joins the resolution (Arms Race reads Remembered after the
// move, and must not see the discarded land). Entry routes the move phase
// does not reach (a ChangeZoneAll sweep, an Aura's return, a face-down entry)
// keep the StallPayETB stall: their move is already made where gorge asks.

// the effect path's pre states. prePayETBEnter is the spell path's answered
// marker (payETBPaid reads state 2).
const (
	prePayETBMay   uint8 = 3 // the may-election is posed, the move parked
	prePayETBPick  uint8 = 4 // the may-election was yes and more than one land is eligible: the pick is posed
	prePayETBEnter uint8 = 2 // the answer consumed: the entry goes through
	prePayETBGrave uint8 = 5 // declined, or no eligible land: the card goes to its owner's graveyard
)

// raGenPayETB (dirac/manifest/ask/raGenPayETB, manifest_gen.go) is a parked
// battlefield move's pay-before-entering election (the may-election first,
// then the land pick).

// payETBEntry is the move phase's gate for a battlefield destination: it runs
// the replacement of the card o about to move (the entry player ep would
// control it) and reports what the caller does with the ordinary move.
//
//go:noinline
func (s *State) payETBEntry(k *prep, o Obj, ep uint8) int {
	if k.gen.anyRepl&rbPayETB == 0 || k.gen.replBits[s.objs[o].card]&rbPayETB == 0 {
		return payETBNone
	}
	p := &s.g.pre
	if p.kind == akPayETB && p.obj == o {
		switch p.state {
		case prePayETBEnter:
			// the chain's Cleanup ClearRemembered: the discarded land leaves
			// the resolution's remembered set before the entrant's own
			// RememberChanged$ adds it
			s.payETBCleanup()
			return payETBNone
		case prePayETBGrave:
			s.payETBToGrave(k, o)
			return payETBGrave
		}
	}
	if p.state != 0 || s.g.nrem != 0 {
		// another pre-entry park is live, or the resolution already
		// remembers something: the replacement's X (Remembered$Amount) is
		// not the discard alone, so the entry stays unmodelled (the
		// enterGen stall)
		return payETBNone
	}
	// eligible = the discarder's hand cards the replacement's Discard takes;
	// none: gorge's discard resolves to nothing, X stays 0, the card goes to
	// its owner's graveyard without a question
	m, ab := s.payETBEligible(k, s.objs[o].card, uint8(payETBAb(k, s.objs[o].card)), ep)
	if m == 0 {
		p.obj, p.kind, p.state = o, akPayETB, prePayETBGrave
		s.payETBToGrave(k, o)
		return payETBGrave
	}
	p.obj, p.kind, p.state, p.val = o, akPayETB, prePayETBMay, objWord(ab)
	s.res.ask, s.res.askP = raGenPayETB, ep
	return payETBPark
}

// payETBEntry's report.
const (
	payETBNone  = 0 // no modelled gate: the ordinary move (or the enterGen stall)
	payETBPark  = 1 // the election is posed: the move is parked
	payETBGrave = 2 // the decline branch moved the card to its owner's graveyard
)

// payETBEligible counts hand card o of player p matching the pay-before-
// entering replacement ab's Discard filter, and returns the ability index
// with it (the caller re-reads the instruction off it).
func (s *State) payETBEligible(k *prep, c CardID, ab uint8, p uint8) (int, uint8) {
	in := &k.t.Cards[c].Abilities[ab].Prog[0]
	pl := &s.pl[p]
	m := 0
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if s.matchCardGen(k, in.F, s.objs[o].card, p, p, o) {
			m++
		}
	}
	return m, ab
}

// payETBDiscardOK reports whether hand card o is one the parked replacement's
// Discard takes (the pick ask's and the strict-supersets rule's filter).
func (s *State) payETBDiscardOK(k *prep, c CardID, ab uint8, p uint8, o Obj) bool {
	in := &k.t.Cards[c].Abilities[ab].Prog[0]
	return s.matchCardGen(k, in.F, s.objs[o].card, p, p, o)
}

// payETBToGrave is the replacement chain's ChangeZone-to-graveyard branch:
// the card leaves its zone for its owner's graveyard, and the move's own
// RememberChanged$ saw it move as any moved card does.
func (s *State) payETBToGrave(k *prep, o Obj) {
	from := s.detach(o)
	s.placeFromK(k, o, kop.ZoneGraveyard, false, from, false)
	if s.g.pickF&kop.SrRemember != 0 {
		s.remember(o)
	}
	s.g.pre = preAsk{}
}

// payETBCleanup is the replacement chain's Cleanup ClearRemembered node
// (OpClearRemembered's body).
func (s *State) payETBCleanup() {
	s.g.nrem = 0
	s.g.remCnt = 0
	s.rememberMarkClr()
	s.res.cpl &^= cplRemMask
}

// payETBOptions builds the parked move's ask: the may-election (ChOptDiscMay,
// yes then no, gorge's cardflow.go order) while it is pending, then the land
// pick (ChDiscard, gorge's hand order, Done once the Min 1 pick is recorded).
//
//kmanifest:askopts raGenPayETB
func (s *State) payETBOptions(k *prep, d *Decision) {
	p := &s.g.pre
	c := s.objs[p.obj].card
	switch p.state {
	case prePayETBMay:
		d.Sub = ChOptDiscMay
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case prePayETBPick:
		d.Sub = ChDiscard
		pl := &s.pl[s.res.askP]
		for i := 0; i < int(pl.handN); i++ {
			o := pl.hand[i]
			if !s.optaskPicked(o) && s.payETBDiscardOK(k, c, uint8(p.val), s.res.askP, o) {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
		if s.res.peekI >= 1 {
			d.add(MakeAction(AkDone, 0, 0, 0)) // Min 1: Done from the first pick on
		}
	}
}

// payETBAnswer applies the parked move's ask and re-runs the move phase with
// the answer consumed (the paid gate enters the card, the decline gate
// graveyards it).
//
//kmanifest:askapply raGenPayETB
func (s *State) payETBAnswer(k *prep, a Action) {
	p := &s.g.pre
	c := s.objs[p.obj].card
	switch p.state {
	case prePayETBMay:
		if a.Mode() == 0 {
			p.state = prePayETBGrave
			s.payETBResume(k)
			return
		}
		m, _ := s.payETBEligible(k, c, uint8(p.val), s.res.askP)
		if m <= 1 { // gorge's strict-supersets rule: the single eligible card discards with no pick
			if m == 1 {
				pl := &s.pl[s.res.askP]
				for i := 0; i < int(pl.handN); i++ {
					o := pl.hand[i]
					if s.payETBDiscardOK(k, c, uint8(p.val), s.res.askP, o) {
						s.discardObj(k, o)
						s.remember(o) // RememberDiscarded$ (discardRemember reads res.pc, not the discard)
						break
					}
				}
			}
			p.state = prePayETBEnter
			s.payETBResume(k)
			return
		}
		p.state = prePayETBPick
		s.res.peekI, s.res.peekN, s.res.left = 0, 0, 1 // Min 1, Max min(NumCards 1, eligible)
	case prePayETBPick:
		if a.Kind() != AkDone && s.optaskPick(k, a, s.discardObj) {
			return // re-pose: the Min 1 pick is not recorded yet
		}
		r := &s.res
		for j := 0; j < int(r.peekI); j++ {
			s.remember(r.arr[j])
		}
		for j := 0; j < int(r.peekN); j++ {
			s.remember(r.peek[j])
		}
		s.optaskFireDiscard(k) // the picks leave in pick order, CR 404.3
		p.state = prePayETBEnter
		s.payETBResume(k)
	}
}

// payETBResume re-runs the parked move phase: the paid gate enters the card,
// the decline gate graveyards it, and the resolution continues after the
// moves.
func (s *State) payETBResume(k *prep) {
	s.res.ask = raNone
	s.finishSearchNow(k)
	if s.res.ask != raNone {
		return // parked again (a later card's ask)
	}
	s.continueRes(k, 0)
}
