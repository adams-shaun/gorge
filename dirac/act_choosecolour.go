package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// colorExclude is the Exclude$ colour mask (W U B R G = bits 0..4) of the
// as-enters colour ask being posed: the OpChooseColor C of the entering
// card's enters program (gorge chooseColorOptions; Citadel Gate's "a colour
// other than white"). The card is the pre-entry ask's object, else the
// queued ask's permanent. Read only while building a colour ask.
//
//go:noinline
func (s *State) colorExclude(k *prep) uint8 {
	c := NoCard
	switch {
	case s.preAskPosed() || s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice:
		c = s.preAskCard()
	case s.g.nask > 0:
		c = s.cardAt(s.g.ask[0].sl)
	}
	if c == NoCard {
		return 0
	}
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplEntersProgram {
			continue
		}
		for j := range a.Prog {
			if a.Prog[j].Op == kop.OpChooseColor {
				return uint8(a.Prog[j].C) & 31
			}
		}
	}
	return 0
}
