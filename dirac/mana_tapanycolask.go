package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tapAnyColAsks is how many colour asks the any-colour StatTapExtra triggers
// of a Taps-deferred plain gap's tap pose in gorge's prefix search: one per
// reaching carrier (Fertile Ground's, Trace of Abundance's, Buried in the
// Garden's: Produced$ Any, one colour for the whole amount; Dawn's
// Reflection's Combo Any, one colour per unit), none when no carrier reaches
// it, the asks name more colours than the scratch keeps (anyColAskCap) or the
// tap's own ask is the source's (a rider, a restricted or colour-asking gap).
// units holds each ask's colour count.
// gorge's scriptSearch.visit branches on every answer of an ask the activation
// poses, the triggers' colour asks included, so the search finds the line that
// floats the colours the cost still lacks beside the tap's own mana (CR
// 605.3b): a {R/W}{G} cast through an enchanted Plains takes the Plains' {W}
// and the trigger's {G}, where the deterministic fill (the first pip the cost
// owes, {W}) floats {W} twice, and a {2}{G}{W} cast through a Mountain under
// two such Auras takes {G} and one more mana of any colour.
func (s *State) tapAnyColAsks(k *prep, sl Slot, units *[anyColAskCap]uint8) (asks int) {
	if k.gen == nil || !k.gen.anyTapExtra || DebugTapAnyColOff || s.pool != nil && s.pool.census ||
		s.askColDeferred(k, sl) || s.restrTapsGapShape(k, sl) {
		return 0
	}
	if _, rider := s.riderAbilityOf(k, sl); rider {
		return 0
	}
	ctl := s.perms[sl].ctrl
	slots := 0
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if st.a.Kind != kop.AbStatic || a.Kind != kop.StatTapExtra || a.StFlags&kop.StTapAnyCol == 0 || st.sl == NoSlot ||
			st.ctrl != ctl && a.StFlags&kop.StTapAnyPlayer == 0 || !s.matchPermGen(k, a.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		u := anyColUnits(a)
		if asks < anyColAskCap {
			units[asks] = u
		}
		asks++
		slots += int(u)
		return true
	})
	if asks > anyColAskCap || slots > anyColAskCap {
		return 0
	}
	return asks
}

// anyColUnits is how many colours a carrier's ask names: one for a Produced$
// Any amount (one colour for the whole amount), its Amount$ for a Combo Any
// (one colour per unit, a multi-unit ask of its own: Dawn's Reflection's two).
func anyColUnits(a *kop.Static) uint8 {
	if a.A == 1 {
		return uint8(a.B)
	}
	return 1
}

// anyColAskMax caps one ask's answers: gorge's scriptAnswers lists at most 64
// per-unit combinations.
const anyColAskMax = 64

// anyColAskOrder fills out with the answers of one colour ask of u units in
// gorge's search order: every per-unit combination in option order (the last
// unit varying fastest, scriptAnswers' unit-major walk, at most 64), the ones
// naming a colour the play's printed cost asks for first (scriptOrderAnswers).
// An answer is its units' colours, three bits each, unit 0 lowest; it returns
// how many answers there are. One unit is the colour order needFirst gives.
func anyColAskOrder(u int, need *[kop.NumManaKinds]bool, out *[anyColAskMax]uint16) (n int) {
	total := 1
	for i := 0; i < u; i++ {
		total *= 5
	}
	total = min(total, anyColAskMax)
	wanted := false
	for c := 0; c < 5; c++ {
		wanted = wanted || need[c]
	}
	for pass := 0; pass < 2; pass++ {
		for r := 0; r < total; r++ {
			var code uint16
			named := false
			for x, j := r, u-1; j >= 0; x, j = x/5, j-1 {
				c := x % 5
				code |= uint16(c) << (3 * uint(j))
				named = named || need[c]
			}
			if pass == 0 && (named || !wanted) || pass == 1 && wanted && !named {
				out[n] = code
				n++
			}
		}
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

// anyColAsks is the colour asks of one Taps-deferred gap's tap, the answers
// gorge's prefix search branches on (tapAnyColAsks), ready to walk: the asks'
// unit counts, each ask's answers in search order (anyColAskOrder) and how
// many answer combinations the asks make together.
type anyColAsks struct {
	n     int
	units [anyColAskCap]uint8
	list  [anyColAskCap][anyColAskMax]uint16
	ln    [anyColAskCap]int
	total int // the combinations (1 when the tap poses no ask)
}

// anyColAsksOf fills x with the asks of the tap of sl for play a, the play's
// needed colours first (scriptOrderAnswers).
func (s *State) anyColAsksOf(k *prep, a Action, sl Slot, x *anyColAsks) {
	x.total = 1
	x.n = s.tapAnyColAsks(k, sl, &x.units)
	if x.n == 0 {
		return
	}
	need := s.playNeed(k, a)
	for u := 0; u < x.n; u++ {
		x.ln[u] = anyColAskOrder(int(x.units[u]), &need, &x.list[u])
		x.total *= x.ln[u]
	}
}

// visits is the search nodes the asks cost: the first ask is visited once, the
// next under every answer of the earlier ones.
func (x *anyColAsks) visits() (v int) {
	m := 1
	for u := 0; u < x.n; u++ {
		v += m
		m *= x.ln[u]
	}
	return v
}

// encode is the answer word (tapAnyColAnswerBits) of the gap's answer ai under
// the xi-th combination of the asks' answers, the last ask varying fastest.
func (x *anyColAsks) encode(ai, xi int) uint16 {
	if x.n == 0 {
		return uint16(ai)
	}
	var asked [anyColAskCap]uint16
	for u, r := x.n-1, xi; u >= 0; u, r = u-1, r/x.ln[u] {
		asked[u] = x.list[u][r%x.ln[u]]
	}
	var cols [anyColAskCap]uint8
	ns := 0
	for u := 0; u < x.n; u++ {
		for j := 0; j < int(x.units[u]); j++ {
			cols[ns] = uint8(asked[u] >> (3 * uint(j)) & 7)
			ns++
		}
	}
	return tapAnyColAnswerBits(ai, cols[:ns])
}

// dfsAskTotal is the answer combinations the ordered-prefix search
// (scriptDFS) walks under each of gap sl's own answers: the asks' combinations
// while the whole answer index (own answers times combinations) fits a path
// entry, else none (the deterministic fill answers).
func (s *State) dfsAskTotal(k *prep, a Action, sl Slot, na uint8) int {
	var x anyColAsks
	s.anyColAsksOf(k, a, sl, &x)
	if x.total <= 1 || int(na)*x.total >= scriptSkip {
		return 1
	}
	return x.total
}

// dfsGapAnswer is the answer word of the ordered-prefix search's path entry c
// for gap sl: the gap's own answer c / total under the c % total-th
// combination of the tap's colour asks (dfsAskTotal), else c itself.
func (s *State) dfsGapAnswer(k *prep, a Action, sl Slot, na uint8, c uint8) int {
	total := s.dfsAskTotal(k, a, sl, na)
	if total <= 1 {
		return int(c)
	}
	var x anyColAsks
	s.anyColAsksOf(k, a, sl, &x)
	return int(x.encode(int(c)/total, int(c)%total))
}

// dfsPathAnswer is the answer scriptGapTap reads for the path entry c of gap
// sl: the entry itself, except a Taps-deferred gap whose tap poses colour asks
// (dfsGapAnswer).
func (s *State) dfsPathAnswer(k *prep, a Action, sl Slot, tapsDef slotMask, c uint8) int {
	if c == scriptSkip || !tapsDef.has(sl) {
		return int(c)
	}
	return s.dfsGapAnswer(k, a, sl, s.deferGapAnswers(k, sl), c)
}
