package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kop.StatColorWords (Swirl the Mists, CR 612.5): while the source is on the
// battlefield with a recorded colour, every colour word in the text of the
// spells on the stack and the permanents on the battlefield is that colour
// word. gorge rewrites the faces (rules colorwords_sync.go); the kernel holds
// no per-object text, so it models the change only where it is a no-op and
// stalls (StallTextChange, unmodelled) where it is not, exactly as
// ExtChangeText does (act_changetext.go): a card whose face reads
// (CardDef.TextWords bits 0..4) a colour word other than the chosen one would
// be read unchanged. The words a static rewrites are all five but its own, so
// the chosen word itself is a no-op; a source with no recorded colour changes
// nothing (gorge's colourWordEffects fails closed on an unanswered ask).
//
// Two checkpoints cover every read of a changed face without a hook in each
// reader: beginCast (a spell takes the change as it reaches the stack, before
// its targets are asked) and the priority grant (every permanent and spell
// already in play, before a player may act or a trigger's targets are asked).
// Both sit behind genPrep.anyColorWords, so a pool with no such static pays
// one branch.

// ColorWordsStats counts the mechanism for the lockstep tests: Live is each
// priority grant that found a StatColorWords source with a recorded colour,
// Stalls those that stalled on a card reading another colour word, Casts the
// spells whose cast stalled.
var ColorWordsStats struct{ Live, Stalls, Casts atomic.Int64 }

// colorWordsBites is the colour words (CardDef.TextWords bits 0..4) the live
// StatColorWords sources rewrite: every word but a source's chosen one, over
// all the sources (two Swirls on different words leave the union, a superset
// of what gorge's chained substitutions change, which stalls where it need
// not and never answers from a changed face). 0 with no live source.
//
//go:noinline
func (s *State) colorWordsBites(k *prep) uint16 {
	var bites uint16
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatColorWords || stc.sl == NoSlot {
			return true
		}
		if ch := uint8(s.perms[stc.sl].chosen); ch < 5 {
			bites |= 0x1f &^ (1 << ch)
		}
		return true
	})
	return bites
}

// colorWordsStall stalls the game when a permanent or a spell on the stack
// reads a rewritten colour word (the priority checkpoint). It reports whether
// it stalled.
//
//go:noinline
func (s *State) colorWordsStall(k *prep) bool {
	bites := s.colorWordsBites(k)
	if bites == 0 {
		return false
	}
	ColorWordsStats.Live.Add(1)
	hit := false
	for m := s.occ; m.any() && !hit; m = m.rest() {
		hit = k.t.Cards[s.cardAt(m.first())].TextWords&bites != 0
	}
	for i := 0; i < int(s.sp) && !hit; i++ {
		if e := &s.stack[i]; e.kind == seSpell {
			hit = k.t.Cards[e.card].TextWords&bites != 0
		}
	}
	if hit {
		ColorWordsStats.Stalls.Add(1)
		s.capFull(StallTextChange)
	}
	return hit
}

// colorWordsCastStall stalls the game when the card about to be cast reads a
// rewritten colour word (the cast checkpoint). It reports whether it stalled.
//
//go:noinline
func (s *State) colorWordsCastStall(k *prep, c CardID) bool {
	bites := s.colorWordsBites(k)
	if bites == 0 || k.t.Cards[c].TextWords&bites == 0 {
		return false
	}
	ColorWordsStats.Casts.Add(1)
	s.capFull(StallTextChange)
	return true
}
