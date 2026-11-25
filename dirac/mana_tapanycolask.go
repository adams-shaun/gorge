package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tapAnyColAsks is how many colour asks the any-colour StatTapExtra triggers
// of a Taps-deferred plain gap's tap pose in gorge's prefix search: one per
// reaching carrier (Fertile Ground's, Trace of Abundance's, Buried in the
// Garden's: Produced$ Any, one colour for the whole amount), none when no
// carrier reaches it or one of them is a Combo Any (a multi-unit ask) or the
// tap's own ask is the source's (a rider, a restricted or colour-asking gap).
// gorge's scriptSearch.visit branches on every answer of an ask the activation
// poses, the triggers' colour asks included, so the search finds the line that
// floats the colours the cost still lacks beside the tap's own mana (CR
// 605.3b): a {R/W}{G} cast through an enchanted Plains takes the Plains' {W}
// and the trigger's {G}, where the deterministic fill (the first pip the cost
// owes, {W}) floats {W} twice, and a {2}{G}{W} cast through a Mountain under
// two such Auras takes {G} and one more mana of any colour.
func (s *State) tapAnyColAsks(k *prep, sl Slot) int {
	if k.gen == nil || !k.gen.anyTapExtra || DebugTapAnyColOff || s.pool != nil && s.pool.census ||
		s.askColDeferred(k, sl) || s.restrTapsGapShape(k, sl) {
		return 0
	}
	if _, rider := s.riderAbilityOf(k, sl); rider {
		return 0
	}
	ctl := s.perms[sl].ctrl
	n, combo := 0, false
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || a.StFlags&kop.StTapAnyCol == 0 || st.sl == NoSlot ||
			st.ctrl != ctl && a.StFlags&kop.StTapAnyPlayer == 0 || !s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		n++
		combo = combo || a.A == 1
		return true
	})
	if combo || n > anyColAskCap {
		return 0
	}
	return n
}

// anyColAskCap is the most any-colour carriers one tap's answers are kept for
// (the scratch holds three units: anyColExtraColour).
const anyColAskCap = 3

// tapAnyColAnswerBits packs a gap's answer ai and the colour each trigger's
// ask answered (a deferGapWalk answer word: the gap's own answer in the low
// nibble, then three bits per ask holding the colour plus one; zero is the
// deterministic fill).
func tapAnyColAnswerBits(ai int, cols []uint8) uint16 {
	w := uint16(ai)
	for u, c := range cols {
		w |= uint16(c+1) << (4 + 3*uint(u))
	}
	return w
}

// tapAnyColAnswerSplit is tapAnyColAnswerBits' inverse: the gap's answer and
// the packed forced colours (0: none forced).
func tapAnyColAnswerSplit(c int) (ai, forced int) { return c & 15, c >> 4 }

// anyColForcedAt is the reserved byte that holds the count of carriers whose
// colour answer a search forced (zero: the deterministic fill, one answer for
// every carrier). rsv[3] is the second-pass flag (anyColOwedRun,
// mana_tapextra.go), so the count sits past rsv[1] and rsv[2], the per-unit
// answers anyColExtraColour reads.
const anyColForcedAt = 4

// forceAnyColAnswers records the packed answers of tapAnyColAnswerBits as the
// pending tap's any-colour extra answers, carrier by carrier (anyColExtraColour's
// scratch). The next fill (fillTapAnyColOwed) or the end of the tap clears it.
func (s *State) forceAnyColAnswers(forced int) {
	n := 0
	for u := 0; u < anyColAskCap; u++ {
		d := forced >> (3 * uint(u)) & 7
		if d == 0 {
			break
		}
		if u == 0 {
			s.g.winCol = uint8(d - 1)
		} else {
			s.g.rsv[u] = uint8(d - 1)
		}
		n = u + 1
	}
	s.g.rsv[anyColForcedAt] = uint8(n)
}
