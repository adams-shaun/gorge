package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ManaCostWordStats counts how often a ManaCost<N> literal (FwManaCostEQ)
// read a candidate and how often it matched, so a lockstep over Urza's Saga
// can prove the Chapter III search ran this read.
var ManaCostWordStats struct {
	Reads, Hits atomic.Int64
}

// DebugNoManaCostWord, when set, makes FwManaCostEQ match nothing
// (TestDemoUrzaSagaMutation).
var DebugNoManaCostWord bool

// fwManaCostEQ: FwManaCostEQ, the candidate prints a mana cost whose mana
// value is the literal's Arg (gorge's ManaCost<N>, engine/effects/
// filter_manacost.go). A printed "no cost" or empty cost never matches (a
// land has mana value 0 but no cost {0}, CR 202.1b), nor does a face-down
// object (CR 708.2) or a copy that lost its cost (embalm, a back face).
// A permanent reads cmcPerm, a card cardMVIn as FwCMCCmp does.
//
//kmanifest:fword FwManaCostEQ
func (s *State) fwManaCostEQ(c *fctx, l *kop.FLit) bool {
	var ans bool
	switch {
	case c.walk:
	case c.perm():
		ans = s.permPrintsCost(c.k, c.sl) && s.cmcPerm(c.k, c.sl) == int(l.Arg)
	case c.card():
		ans = c.c != NoCard && !c.fd && cardPrintsCost(c.k, c.c) && s.cardMVIn(c.k, c.c, c.o, c.fd, nil) == int(l.Arg)
	}
	if DebugNoManaCostWord {
		return false
	}
	ManaCostWordStats.Reads.Add(1)
	if ans {
		ManaCostWordStats.Hits.Add(1)
	}
	return ans
}

// cardPrintsCost reports whether card c's printed face has a mana cost: not
// a "no cost" card and not an empty cost text.
func cardPrintsCost(k *prep, c CardID) bool {
	d := &k.t.Cards[c]
	return !d.NoManaCost && !d.CostEmpty
}

// permPrintsCost is cardPrintsCost for the permanent at sl, also false while
// face down or under a copy exception that removes the cost.
func (s *State) permPrintsCost(k *prep, sl Slot) bool {
	return !s.faceDown(k, sl) && !s.cpxNoCost(k, sl) && !s.copiesBackFace(k, sl) && cardPrintsCost(k, s.cardAt(sl))
}
