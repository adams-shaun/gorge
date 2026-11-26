package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// PeekAndReveal with RevealOptional$ True,
// RevealValid$ <spec> and RememberRevealed$ True (Gathering Stone, Sidequest:
// Catch a Fish, Bucolic Ranch, Traveling Botanist). gorge effReveal narrows
// the top PeekAmount$ (here 1) cards of the target player's library to
// RevealValid$; with none left there is no ask, no reveal and nothing
// remembered. Otherwise the library's owner gets the reveal_optional yes/no
// (the yes option carries the card); a yes reveals the card (a public Note,
// which the kernel does not record) and appends it to the resolution's
// remembered set, a no remembers nothing (CR 701.20a, 701.20e, 608.2c).
// Cold and out of line.

// raGenPeekReveal is the may-reveal yes/no (OpPeek C&PeekMayReveal).
const raGenPeekReveal = raGenManifestDread + 61

// ChGenPeekReveal is that ask's election: mode 1 = yes (Source = the top
// card, gorge's yes option Obj), 0 = no.
const ChGenPeekReveal ChoiceKind = 241

// DebugPeekRevealAlwaysAsk is the mutation hook: the ask is posed even when
// the top card fails RevealValid$.
var DebugPeekRevealAlwaysAsk bool

// peekMayReveal runs OpPeek's may-reveal arm for player p; false: the ask is
// posed.
//
//go:noinline
func (s *State) peekMayReveal(k *prep, in *kop.Instr, p uint8) bool {
	pl := &s.pl[p]
	st := DebugPeekRevealStats
	if pl.libLen() == 0 {
		if st != nil {
			st.Empty++
		}
		return true
	}
	o := pl.lib[int(pl.libLo)]
	self, srcSl := s.resolutionTypeSource()
	if !DebugPeekRevealAlwaysAsk && !s.matchCardSrc(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o, srcSl, self, NoObj) {
		if st != nil {
			st.Skips++
		}
		return true
	}
	if st != nil {
		st.Asks++
	}
	s.res.ask, s.res.askP = raGenPeekReveal, p
	return false
}

// askOptionsPeekReveal poses the yes (naming the top card) and no options.
func (s *State) askOptionsPeekReveal(d *Decision) {
	pl := &s.pl[SeatIx(s.res.askP)]
	d.Sub = ChGenPeekReveal
	d.add(MakeAction(AkChoose, TCode(pl.lib[int(pl.libLo)]), 0, 1))
	d.add(MakeAction(AkChoose, 0, 0, 0))
}

// applyAskPeekReveal applies the answer: a yes remembers the top card.
func (s *State) applyAskPeekReveal(k *prep, a Action) {
	if a.Mode() == 1 {
		pl := &s.pl[SeatIx(s.res.askP)]
		if pl.libLen() > 0 {
			s.remember(pl.lib[int(pl.libLo)])
		}
	}
	s.continueRes(k, 0)
}

// PeekRevealStats counts the may-reveal arm's outcomes (the lockstep test's
// coverage floors).
type PeekRevealStats struct {
	Asks  int // the top card matched RevealValid$: the yes/no was posed
	Skips int // a card on top failed RevealValid$: no ask
	Empty int // the library was empty
}

// DebugPeekRevealStats is the test hook; nil in normal play.
var DebugPeekRevealStats *PeekRevealStats

// DebugPeekWindowOne is the lockstep mutation of the public reveal's
// remembered window (Dig RememberRevealed$): only the top card is remembered.
var DebugPeekWindowOne bool

// peekPublic is OpPeek with PeekPublic (gorge effReveal's public
// PeekAndReveal): player p reveals the top A cards of their library with no
// ask; with PeekRemember they join the resolution's remembered set,
// appended (gorge appends to Ctx.Remembered).
//
//go:noinline
func (s *State) peekPublic(k *prep, in *kop.Instr, p uint8) {
	if in.C&kop.PeekRemember == 0 {
		return
	}
	pl := &s.pl[p]
	n := s.value(k, in.A)
	if DebugPeekWindowOne && n > 1 {
		n = 1
	}
	for i := 0; i < n && i < pl.libLen(); i++ {
		s.remember(pl.lib[int(pl.libLo)+i])
	}
}

// DebugPeekHandNextSeat is the lockstep mutation of the pod hand reveal: the
// kernel remembers the next live seat's hand whoever the reveal names, the
// two-seat reading.
var DebugPeekHandNextSeat bool

// peekHand is OpPeek with PeekHand (gorge effReveal's RevealHand with
// RememberRevealed$, no Look$-less count or filter): each player the ref
// names, in gorge's definedPlayers order, reveals the whole hand, and its
// cards join the resolution's remembered set in hand order, appended (gorge
// appends to Ctx.Remembered; an empty hand reveals and remembers nothing). A
// full set stalls (remember's StallRemember).
//
//go:noinline
func (s *State) peekHand(k *prep, in *kop.Instr) {
	q := s.players(k, in.Obj)
	if DebugPeekHandNextSeat {
		q = seatOne(s.oppsFrom(s.res.e.ctrl).s[0])
	}
	for i := uint8(0); i < q.n; i++ {
		if q.s[i] == 255 {
			continue
		}
		pl := &s.pl[SeatIx(q.s[i])]
		for j := 0; j < int(pl.handN); j++ {
			s.remember(pl.hand[j])
		}
	}
}

// execPeek runs OpPeek's whole dispatch (interp_gen.go's case): the hand
// reveal, the may-reveal arm, the public reveal of one player or each player,
// and the bare private look with its pacing ack. It lives here rather than in
// the dispatch switch to keep execGen under its length ceiling.
//
//go:noinline
func (s *State) execPeek(k *prep, in *kop.Instr) (ok, done bool) {
	if in.C&kop.PeekHand != 0 {
		s.peekHand(k, in)
		return true, true
	}
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	if in.C != 0 {
		if in.C&kop.PeekMayReveal != 0 {
			return s.peekMayReveal(k, in, p), true
		}
		if in.Obj == kop.RefEachPlayer {
			// Defined$ Player: every player reveals, the resolving
			// controller first, as gorge's definedPlayers walks AliveFrom.
			q := s.eachPlayers(k, in.Obj)
			for i := uint8(0); i < q.n; i++ {
				s.peekPublic(k, in, q.s[i])
			}
			return true, true
		}
		s.peekPublic(k, in, p)
		return true, true
	}
	pl := &s.pl[p]
	n := s.value(k, in.A)
	s.g.nrem = 0
	for i := 0; i < n && i < pl.libLen() && i < maxRem; i++ {
		s.remember(pl.lib[int(pl.libLo)+i])
	}
	if s.g.nrem > 0 {
		// gorge poses the bare look's pacing ack (NoReveal$ True) first,
		// to the effect's controller (effReveal: asker = c.Controller)
		s.res.ask, s.res.askP = raGenLookAck, s.res.e.ctrl
		return false, true
	}
	return true, true
}
