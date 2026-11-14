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
	if !DebugPeekRevealAlwaysAsk && !s.matchCardSrc(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o, srcSl, self) {
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
	for i := 0; i < n && i < pl.libLen(); i++ {
		s.remember(pl.lib[int(pl.libLo)+i])
	}
}
