package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Axebane Guardian: "{T}: Add X mana in any combination of colors, where X is
// the number of creatures with defender you control" (Produced$ Combo Any,
// kop.AfManaChoice with ManaColors == ColAny; comboAnyMana). gorge's payment
// planner cannot price it (pay.PaymentPlanChoiceColours has no colour list for
// an any-combination production), so the source is a census gap: a plain cast
// or activation only its mana pays is insufficient in the planner and gets a
// prefix search (rules potentialPrefixScript) that activates the source and
// answers its colour ask, one colour per unit. The kernel mirrors it as a gap
// kind of its own (gfGapOther; never a planner unit, no gfManaVar view), modelled
// like the Lion's Eye Diamond's (mana_discardhand.go): when these sources are
// their controller's only census gaps (caBoard), availScript counts each one's
// amount of any-colour units, and scriptOffers searches the plays the sources'
// mana would pay. A board with a gap of another kind keeps its old behaviour (the
// Combo Any source is then outside the model).
//
// The ask is gorge's scriptAnswers: the per-unit colours of an N-unit allocation
// in odometer order (unit 0 the slowest digit, W U B R G), at most 64 answers; one
// unit lists its five. scriptOrderAnswers then moves the answers that name a
// colour the play's cost announces to the front, order kept inside each half.

// maxCAAnswers is gorge's cap on an N-unit allocation's answers (scriptAnswers).
const maxCAAnswers = 64

// comboAnyMana reports whether a is an unrestricted "{T}: Add N mana in any
// combination of colors" ability (N dynamic or at least two): the Combo Any
// census gap. A restricted one keeps the wide any-colour model
// (restrWideAnyMana), one with a cost beyond {T} its own lowering or refusal.
func comboAnyMana(a *kop.Ability) bool {
	return a.Kind == kop.AbMana &&
		a.AFlags&(kop.AfManaChoice|kop.AfLatent|kop.AfManaEachColor|kop.AfManaChosen|kop.AfManaReflect) == kop.AfManaChoice &&
		a.ManaColors == kop.ColAny && a.ManaRestrict == 0 && !a.ManaDeferred && !a.ManaEach && a.ManaRider == 0 &&
		a.SplitGate == 0 && a.SplitCols == 0 &&
		a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		(a.AFlags&kop.AfManaDynamic != 0 || a.ManaN >= 2)
}

// isComboAny reports whether card c's census gap (gfGapOther) is its Combo Any
// ability.
func (k *prep) isComboAny(c CardID) bool {
	if !k.gen.anyComboAny || !k.gen.has(c, gfGapOther) {
		return false
	}
	j := k.gen.gapOther(c)
	return j >= 0 && comboAnyMana(&k.t.Cards[c].Abilities[j])
}

// comboAnyAb is the Combo Any ability of card c (isComboAny).
func (k *prep) comboAnyAb(c CardID) *kop.Ability {
	return &k.t.Cards[c].Abilities[k.gen.gapOther(c)]
}

// caAmount is the mana the Combo Any source at sl adds now.
func (s *State) caAmount(k *prep, sl Slot) int {
	a := k.comboAnyAb(s.cardAt(sl))
	if a.AFlags&kop.AfManaDynamic != 0 {
		return min(max(s.dynManaN(k, a, sl), 0), maxDynMana)
	}
	return max(int(a.ManaN), 1)
}

// caGaps: p's Combo Any sources that can be activated now and add mana.
func (s *State) caGaps(k *prep, p uint8) slotMask {
	if !k.gen.anyComboAny {
		return slotMask{}
	}
	m := s.gapMask(k, p, gfGapOther)
	for b := m; b.any(); b = b.rest() {
		sl := b.first()
		if !k.isComboAny(s.cardAt(sl)) || s.caAmount(k, sl) == 0 {
			m = m.andNot(slotBit(sl))
		}
	}
	return m
}

// caBoard reports whether p controls a live Combo Any source and no census gap
// of another kind (the Diamond's own dhBoard test, with the Diamond itself one
// of the other kinds).
func (s *State) caBoard(k *prep, p uint8) bool {
	if k.gen == nil || !k.gen.anyComboAny || !s.caGaps(k, p).any() {
		return false
	}
	if k.gen.anyDiscHand && s.dhGaps(k, p).any() {
		return false
	}
	if s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptTapsDefer(k, p)).or(s.scriptCtr(k, p)).or(s.scriptFilt(k, p)).any() {
		return false
	}
	if k.tapMana && s.scriptTapX(k, p).any() || s.liveGrantGap(k, p) || s.grantGapLive(k, p) {
		return false
	}
	return !s.gapOthers(k, p).andNot(s.caGaps(k, p)).any()
}

// availCA adds each live Combo Any source's units: its amount of any colour.
func (s *State) availCA(k *prep, p uint8, a *avail) {
	for b := s.caGaps(k, p); b.any(); b = b.rest() {
		a.addUnits(0x1f, uint8(s.caAmount(k, b.first())))
	}
}

// caAnswerCount is the answers the colour ask of n units poses (scriptAnswers):
// a single unit's five, else the first 64 of the 5^n per-unit combinations.
func caAnswerCount(n int) int {
	switch {
	case n <= 1:
		return 5
	case n == 2:
		return 25
	}
	return maxCAAnswers
}

// caDigit is the colour (W U B R G = 0..4) answer i gives unit u of n: i read in
// base five, unit 0 the most significant digit. A unit above the last three
// reads a zero (the answers stop at 64).
func caDigit(i, n, u int) int {
	for q := n - 1 - u; q > 0 && i > 0; q-- {
		i /= 5
	}
	return i % 5
}

// caNames reports whether odometer answer i of n units picks a colour need names.
func caNames(i, n int, need *[kop.NumManaKinds]bool) bool {
	if n > 3 && need[0] {
		return true // the leading units all read W
	}
	for u := max(n-3, 0); u < n; u++ {
		if need[caDigit(i, n, u)] {
			return true
		}
	}
	return false
}

// caRank is the odometer index of the r'th answer gorge's search tries
// (scriptOrderAnswers): the answers naming a colour the play announces first,
// then the rest, each half in odometer order; odometer order when the play names
// no colour.
func caRank(n int, need *[kop.NumManaKinds]bool, r int) int {
	if !(need[0] || need[1] || need[2] || need[3] || need[4]) {
		return r
	}
	total := caAnswerCount(n)
	for _, first := range [2]bool{true, false} {
		for i := 0; i < total; i++ {
			if caNames(i, n, need) == first {
				if r == 0 {
					return i
				}
				r--
			}
		}
	}
	return r
}

// caActivate activates the Combo Any source at sl with odometer answer ans:
// the tap, then one unit of each answered colour into p's pool.
func (s *State) caActivate(k *prep, p uint8, sl Slot, ans int) {
	n := s.caAmount(k, sl)
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	var got [5]int
	for u := max(n-3, 0); u < n; u++ {
		got[caDigit(ans, n, u)]++
	}
	got[0] += max(n-3, 0) // the leading units read W
	for col, c := range got {
		if c > 0 {
			s.poolAdd(p, col, c)
			s.artTag(k, p, sl, col)
		}
	}
	s.tapExtraStall(k, sl)
}

type caStep struct {
	sl  Slot
	ans uint8 // the odometer answer (below maxCAAnswers)
}

// caSearch is gorge's prefix search over p's Combo Any sources
// (scriptSearch.visit, gapsOnly with no funding): at each payer priority node it
// asks whether the play is offered or the planner pays it from the pool, else
// activates each remaining source in battlefield order, branching on its colour
// answers.
type caSearch struct {
	a     Action
	p     uint8
	need  [kop.NumManaKinds]bool
	nodes int
	path  [MaxPerm]caStep
	np    int
}

// visit is scriptSearch.visit at payer priority node t (root: no source used yet,
// so the play is not priced there).
func (cs *caSearch) visit(k *prep, t *State, d *Decision, root bool) bool {
	if cs.nodes >= potentialScriptNodes {
		return false
	}
	cs.nodes++
	if !root {
		t.restrOptions(k, d, cs.p)
		if hasOption(d, cs.a) {
			return true
		}
	}
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(cs.p, t.caGaps(k, cs.p), order[:], 0)
	for i := 0; i < n; i++ {
		if cs.nodes >= potentialScriptNodes {
			return false
		}
		cs.nodes++ // the colour ask's node
		units := t.caAmount(k, order[i])
		for r, na := 0, caAnswerCount(units); r < na; r++ {
			ans := caRank(units, &cs.need, r)
			c := t.fork()
			c.caActivate(k, cs.p, order[i], ans)
			cs.path[cs.np] = caStep{order[i], uint8(ans)}
			cs.np++
			found := cs.visit(k, c, d, false)
			t.unfork()
			if found {
				return true
			}
			cs.np--
		}
	}
	return false
}

// caScript reports whether gorge's prefix search over p's Combo Any sources
// finds a script for play a; with apply it plays the script on s (the play's
// prefix).
func (s *State) caScript(k *prep, a Action, p uint8, apply bool) bool {
	cs := caSearch{a: a, p: p, need: s.playNeed(k, a)}
	var d Decision
	if !cs.visit(k, s, &d, true) {
		return false
	}
	if apply {
		for _, st := range cs.path[:cs.np] {
			s.caActivate(k, p, st.sl, int(st.ans))
		}
	}
	return true
}
