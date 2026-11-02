package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// a reflected mana ability (kop.AfManaReflect; AB$ ManaReflected,
// ReflectProperty$ Is over the controller's own permanents: Plaza of
// Heroes' "{T}: Add one mana of any color among legendary permanents you
// control"). kgen admits it only as the third of {T}: Add {C}; {T}: Add one
// mana of any color (restricted, the Courtyard gap genPrep.anyR); {T}:
// reflected (dirac/kgen lower_reflectedmana.go).
//
// gorge's planner census (rules paymentPlanManaUnitsOnlyCompute) lists the
// source as one unit: the shared census's {C}, the reflected colours (the
// resolution's object walk, {C} dropped: pay.PaymentPlanReflectedCounts),
// and the restricted any colour, which the planner defers
// (paymentPlanAbilityShapeTier: RestrictValid$). The kernel's planner view
// is therefore the kmana2 union {C} | the reflected colours, a function of
// the board: varPC holds one view per reflected colour set (32), and
// manaPCGen picks the live one (manaReflView). The payment window asks
// "Choose a mana ability" and the oracle answers ability 0 ({C}):
// vergeAbility0Col reads view 0 ({C} alone) through anyVergeOrder.

// debugReflMutate is the test-only mutation switch (0 none; 1 the reflected
// colours are never read: the view is {C} alone; 2 the relaxed proof counts
// a Plaza once, its census unit replaced (mana_window.go availAnyR); 3 the
// prefix search answers the ability ask's {C} after the restricted colours).
// TestPlazaLockstepMutation requires each to diverge.
var debugReflMutate uint8

// SetDebugReflMutate sets the mutation switch; tests only.
func SetDebugReflMutate(m uint8) { debugReflMutate = m }

// prepManaRefl builds card c's 32 reflected views (prepareGen, once per
// card): view m is the first ability's {C} with the colours m beside it.
//
//go:noinline
func prepManaRefl(g *genPrep, c *kop.CardDef, id CardID, pc *pcard) {
	ra := -1
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfManaReflect != 0 && a.AFlags&kop.AfLatent == 0 {
			ra = j
			break
		}
	}
	if ra < 0 || g.has(id, gfManaVar) || pc.manaAb < 0 {
		return
	}
	g.setFlag(id, gfManaVar|gfManaRefl)
	g.smallFor(id).manaVar = int16(len(g.varPC))
	for m := 0; m < 32; m++ {
		v := *pc
		v.manaCol = pc.manaCol | kop.ColorMask(m)
		v.manaMulti = m != 0
		g.varPC = append(g.varPC, v)
	}
	g.anyVergeOrder, g.anyRefl = true, true
}

// reflColours is the colours the reflected ability of the source at sl
// reads: those of its controller's permanents matching ManaAmong (gorge
// effects.ManaReflectedObjects with ReflectProperty$ Is), {C} dropped.
func (s *State) reflColours(k *prep, sl Slot, c CardID) kop.ColorMask {
	cd := &k.t.Cards[c]
	var f kop.FilterID
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfManaReflect != 0 {
			f = a.ManaAmong
			break
		}
	}
	p := s.perms[sl].ctrl
	var m kop.ColorMask
	for b := s.ctrlM[p&1]; b.any(); b = b.rest() {
		o := b.first()
		if s.matchPerm(k, f, o, p, sl) {
			m |= s.colorsAt(k, o)
		}
	}
	return m & kop.ColAny
}

// manaReflView is manaPCGen's arm for a reflected source: the view of the
// colours its reflected ability reads now.
//
//go:noinline
func (s *State) manaReflView(k *prep, sl Slot, c CardID, vi int16) *pcard {
	m := s.reflColours(k, sl, c)
	if debugReflMutate == 1 {
		m = 0
	}
	return &k.gen.varPC[int(vi)+int(m)]
}

// Restricted mana in several colours (State.poolRx). gorge's
// Players[p].RestrictedMana holds one batch per activation: Plaza's
// restricted any colour tapped twice by a prefix script for {W} and {R}
// (a legendary RW spell) floats two batches of one restriction. poolR keeps
// the first colour; poolRx the others, 3 bits each.

// addRestrX floats n restricted mana of colour col beside poolR's other
// colour (same restriction); past 7 it floats unrestricted, as addRestr's
// spill (the prefix search refuses that loudly first: reflDo).
func (pl *player) addRestrX(rx *uint16, col int, n uint8) bool {
	cur := uint8(*rx>>(3*col)) & 7
	if col >= 5 || int(cur)+int(n) > 7 {
		pl.pool[col] += n
		return false
	}
	*rx += uint16(n) << (3 * col)
	return true
}

// restrXInto adds poolRx's mana rx to m; restrXOut takes it out again.
func restrXInto(rx uint16, m *[kop.NumManaKinds]uint8) {
	for c := 0; rx != 0 && c < 5; c++ {
		m[c] += uint8(rx>>(3*c)) & 7
	}
}

func restrXOut(rx uint16, m *[kop.NumManaKinds]uint8) {
	for c := 0; rx != 0 && c < 5; c++ {
		m[c] -= uint8(rx>>(3*c)) & 7
	}
}

// payRestrX is payRestrRider with restricted mana of several colours: all
// of it joins the pool, the cast taps for the rest and pays, and per colour
// the restricted mana is spent first (gorge emitRestrictedManaSpend carves
// the matching batches before ordinary mana of their colour); what is left
// of it floats restricted again.
func (s *State) payRestrX(k *prep, p uint8, cost *kop.ManaCost, x int, o Obj, anyColor bool) {
	pl := &s.pl[p]
	kind := pl.poolR & poolRKind
	var rn [kop.NumManaKinds]uint8
	rn[pl.poolR>>poolRShift] += pl.poolR & poolRCnt
	restrXInto(s.poolRx[p&1], &rn)
	for c := range rn {
		pl.pool[c] += rn[c]
	}
	pl.poolR, s.poolRx[p&1] = 0, 0
	if cost.Hyb[0] != 0 {
		h := s.hybResolveRider(k, p, *cost, x, anyColor)
		cost = &h
	}
	s.tapForRider(k, p, cost, x, o, anyColor)
	pre := pl.pool
	s.payFromPoolRider(k, p, cost, x, anyColor)
	for c := range rn {
		left := rn[c] - min(pre[c]-pl.pool[c], rn[c])
		if left == 0 {
			continue
		}
		pl.pool[c] -= left
		if pl.poolR == 0 {
			pl.poolR = uint8(c)<<poolRShift | kind | left
		} else {
			s.poolRx[p&1] += uint16(left) << (3 * c)
		}
	}
}

// The exact prefix search for an anyR-only board on a table with a
// reflected source (gorge rules/potential_script.go scriptSearch.visit
// under potentialPrefixScript: gapsOnly, funding 0, potentialScriptNodes).
// Each Plaza (or Courtyard) gap is activated in battlefield order; its
// "Choose a mana ability" ask (one node) is answered {C}, then "Add any
// color" (the colour ask, one node, W U B R G: restricted mana), then,
// when the reflected ability reads a colour, the reflected one (a colour
// ask, one node, only for two colours or more). A priority node is one
// node; a payer state already visited is not searched again (scriptStateKey:
// the pool with its restricted part and the tapped set); after a gap was
// activated the play is looked up in the walk with the restricted mana
// counted as ordinary (restrOptions: gorge's offer ignores the restriction
// there, its planner declines a pool holding it).

// reflStep is one activation: source sl, answer ans (0 {C}; 1+c the
// restricted colour c; 6+c the reflected colour c).
type reflStep struct {
	sl  Slot
	ans uint8
}

type reflKey struct {
	pool   [kop.NumManaKinds]uint8
	r      uint8
	rx     uint16
	tapped slotMask
}

const maxReflPath = 24

type reflSearch struct {
	a     Action
	p     uint8
	gaps  slotMask
	nodes int
	nseen int
	seen  [potentialScriptNodes]reflKey
	n     int
	path  [maxReflPath]reflStep
	dead  bool
}

// reflBoard reports whether p's census gaps are anyR sources alone (the
// boards reflScript models).
func (s *State) reflBoard(k *prep, p uint8, restr, anyR slotMask) bool {
	return k.gen.anyRefl && anyR.any() && !restr.any() && !s.scriptGaps(k, p).any() && !s.scriptSac(k, p).any() &&
		!s.scriptCtr(k, p).any() && !s.scriptFilt(k, p).any()
}

// reflScript runs the search for play a of p: found, and dead when the
// found script leaves restricted mana the play's pool cannot spend (the
// oracle's completion fails). With apply it plays the script on s.
//
//go:noinline
func (s *State) reflScript(k *prep, a Action, p uint8, apply bool) (found, dead bool) {
	var rs reflSearch
	rs.a, rs.p, rs.gaps = a, p, s.scriptAnyR(k, p)
	t := s.fork()
	defer s.unfork()
	if !rs.visit(k, t, false) || rs.n == 0 {
		return false, false
	}
	if apply {
		for i := 0; i < rs.n; i++ {
			s.reflDo(k, p, rs.path[i])
		}
	}
	return true, rs.dead
}

func (rs *reflSearch) tick() bool {
	if rs.nodes >= potentialScriptNodes {
		return false
	}
	rs.nodes++
	return true
}

// reflDo plays step st for p.
func (s *State) reflDo(k *prep, p uint8, st reflStep) {
	pl := &s.pl[p&1]
	s.perms[st.sl].flags |= pfTapped
	s.touchSl(st.sl)
	cd := &k.t.Cards[s.cardAt(st.sl)]
	switch {
	case st.ans == 0:
		f := &cd.Abilities[firstManaAb(cd)]
		pl.pool[bits.TrailingZeros8(uint8(f.ManaColors))] += max(f.ManaN, 1)
	case st.ans <= 5:
		c := int(st.ans) - 1
		if pl.poolR&poolRCnt != 0 && int(pl.poolR>>poolRShift) != c && uint8(s.poolRx[p&1]>>(3*c))&7 == 7 {
			s.capFull(StallRestrX) // loud: never float it unrestricted
			return
		}
		if !s.floatRestrFrom(k, p, st.sl, cd.Abilities[k.gen.anyR(s.cardAt(st.sl))].ManaRestrict, c, 1) {
			s.capFull(StallRestrX) // another restriction or past 15: never a silent spill
		}
	default:
		pl.pool[st.ans-6]++
	}
}

func (rs *reflSearch) try(k *prep, t *State, st reflStep) bool {
	if rs.n == maxReflPath {
		return false
	}
	c := t.fork()
	defer t.unfork()
	c.reflDo(k, rs.p, st)
	rs.path[rs.n] = st
	rs.n++
	if rs.visit(k, c, true) {
		return true
	}
	rs.n--
	return false
}

func (rs *reflSearch) visit(k *prep, t *State, gapUsed bool) bool {
	if !rs.tick() {
		return false
	}
	p := rs.p
	pl := &t.pl[p&1]
	key := reflKey{pool: pl.pool, r: pl.poolR, rx: t.poolRx[p&1]}
	for b := t.ctrlM[p&1].and(t.occ); b.any(); b = b.rest() {
		if sl := b.first(); t.perms[sl].flags&pfTapped != 0 {
			key.tapped = key.tapped.with(sl)
		}
	}
	for i := 0; i < rs.nseen; i++ {
		if rs.seen[i] == key {
			return false
		}
	}
	if rs.nseen < len(rs.seen) {
		rs.seen[rs.nseen] = key
		rs.nseen++
	}
	if gapUsed {
		var d Decision
		t.restrOptions(k, &d, p)
		if hasOption(&d, rs.a) {
			rs.dead = false
			if pl.poolR != 0 {
				rs.dead = !t.scriptCompletes(k, p, rs.a)
			}
			return true
		}
	}
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(p, t.ctrlM[p&1].and(t.occ), order[:], 0)
	for i := 0; i < n; i++ {
		sl := order[i]
		if !rs.gaps.has(sl) || t.perms[sl].flags&pfTapped != 0 {
			continue
		}
		if !rs.tick() { // the ability ask
			return false
		}
		if debugReflMutate != 3 && rs.try(k, t, reflStep{sl: sl}) {
			return true
		}
		if !rs.tick() { // "Add any color": the colour ask
			return false
		}
		for c := 0; c < 5; c++ {
			if rs.try(k, t, reflStep{sl: sl, ans: uint8(1 + c)}) {
				return true
			}
		}
		if debugReflMutate == 3 && rs.try(k, t, reflStep{sl: sl}) {
			return true
		}
		rc := uint8(0)
		if c := t.cardAt(sl); k.gen.has(c, gfManaRefl) {
			rc = uint8(t.reflColours(k, sl, c))
		}
		if bits.OnesCount8(rc) > 1 && !rs.tick() { // the reflected colour ask
			return false
		}
		for m := rc; m != 0; m &= m - 1 {
			if rs.try(k, t, reflStep{sl: sl, ans: uint8(6 + bits.TrailingZeros8(m))}) {
				return true
			}
		}
	}
	return false
}
