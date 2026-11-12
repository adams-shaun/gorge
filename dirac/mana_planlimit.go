package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorge's payment planner stops at a node limit and takes the best plan it has
// seen. Leyline of the Guildpact reaches it on an ordinary board: ten lands
// with five alternatives each, split into classes by the order of those
// alternatives, and every plan tying on rank keys 1-7. The plan then depends
// on where the search stopped, so the kernel's exact planner picks a different
// one. limitPlan replays pay.Search (level order, bound, node count) and, when
// that search reaches the limit, replaces the exact plan with its best.
//
// It runs on a board where a land has a granted basic land type, and on one
// with planLimitUnits sources or more (an {X} spell paid from a dozen Treasures
// and a few lands reaches the limit in phase 2, where the sacrificed sources'
// irreversible cost is rank key 1). Painful, sacrifice-only, gated and
// restricted sources stay with the exact planner, as do the any-colour rider
// and a hybrid cast's source order.

const (
	planLimitNodes = 65536 // decision.MaxPaymentPlanSearchNodes
	planLimitActs  = 64    // decision.MaxPaymentActivations
	planLimitAlts  = 10    // five own colours and five granted ones: Public Thoroughfare's any-colour ability and the intrinsic ones
	planLimitAny   = 63    // the mask of all six mana kinds
	planLimitUnits = 16    // sources from which a board can reach the node limit without a granted type
)

// lsAlt is one alternative of a unit: the colour it makes and the ability that
// makes it, -1 for an intrinsic one (CR 305.6). pay.CompareSteps orders the
// intrinsic ones first, then by ability, then by the WUBRGC vector they make.
type lsAlt struct {
	col uint8
	ab  int8
}

func (a lsAlt) step() int { return (int(a.ab)+1)*8 + 5 - int(a.col) }

// lsClass is pay.Class: the units whose alternatives are interchangeable.
type lsClass struct {
	units [maxUnits]uint8
	n     int32
	ty    uint8
	amt   int32
	irr   int32 // pay.ConsequenceCost of one unit: a sacrificed source's 10 (20 a creature)
	alts  [planLimitAlts]lsAlt
	na    int
	pick  [planLimitAlts]uint8 // alternatives in witness-step order
}

type lsLevel struct{ class, alt uint8 }

// lsStats is pay.boundStats, whose irreversible cost is 0 in phase 1.
type lsStats struct {
	most     [planLimitAny + 1]int32
	least    int32
	irr      int32
	creature int32
	flex     int32
	any      bool
}

func (b *lsStats) add(col uint8, amt, irr, creature, flex int32) {
	for m := 1; m <= planLimitAny; m++ {
		if m>>col&1 != 0 {
			b.most[m] = max(b.most[m], amt)
		}
	}
	if !b.any {
		b.least, b.irr, b.creature, b.flex, b.any = amt, irr, creature, flex, true
		return
	}
	b.least, b.irr, b.creature, b.flex = min(b.least, amt), min(b.irr, irr), min(b.creature, creature), min(b.flex, flex)
}

func (b lsStats) merge(o *lsStats) lsStats {
	if !o.any {
		return b
	}
	if !b.any {
		return *o
	}
	for m := range b.most {
		b.most[m] = max(b.most[m], o.most[m])
	}
	b.least, b.irr, b.creature, b.flex = min(b.least, o.least), min(b.irr, o.irr), min(b.creature, o.creature), min(b.flex, o.flex)
	return b
}

// lsKeys is pay.Rank's keys 1-7.
type lsKeys struct{ cost, creatures, sources, surplus, flex, reserve, remainder int32 }

func (a *lsKeys) cmp(b *lsKeys) int {
	switch {
	case a.cost != b.cost:
		return int(a.cost - b.cost)
	case a.creatures != b.creatures:
		return int(a.creatures - b.creatures)
	case a.sources != b.sources:
		return int(a.sources - b.sources)
	case a.surplus != b.surplus:
		return int(a.surplus - b.surplus)
	case a.flex != b.flex:
		return int(a.flex - b.flex)
	case a.reserve != b.reserve:
		return int(b.reserve - a.reserve)
	}
	return int(b.remainder - a.remainder)
}

// limitSearch lives on limitPlan's stack (var ls), so it must stay under the
// compiler's 128 KiB limit for a declared variable: past it, the whole search
// moves to the heap on every call (TestLimitPlanAllocatesNothing). The bound
// tables that would be per level are not: later and laterCap are the same for
// every level of a class, so they are per class; here is a suffix of one
// class's alternatives, so it is a colour mask per level, rebuilt by hereStats.
// It holds a copy of the planner: a pointer stored here would tie the search
// to the caller's planner.
type limitSearch struct {
	pk       planner
	classes  [maxUnits]lsClass
	nc       int
	levels   [maxUnits * planLimitAlts]lsLevel
	nl       int
	hereCols [maxUnits * planLimitAlts]uint8 // level l: the colours of its class's alternatives from l on
	later    [maxUnits]lsStats               // class c: the stats of the classes after it
	laterCap [maxUnits][planLimitAny + 1]int32
	counts   [maxUnits][planLimitAlts]int32
	used     [maxUnits]int32
	produced [kop.NumManaKinds]int32
	sources  int32
	creats   int32
	flex     int32
	irr      int32
	nodes    int
	limited  bool
	found    bool
	bestKeys lsKeys
	bestU    [maxUnits]uint8
	bestA    [maxUnits]lsAlt
	wU       [maxUnits]uint8
	wA       [maxUnits]lsAlt
	granted  bool // some source has a granted basic land type (unitAltsChoice's gate)
}

// limitPlan replays the search over pk's phase-1 census and, when it reaches
// the node limit, makes its best the plan.
//
//go:noinline
func (s *State) limitPlan(k *prep, pk *planner) {
	if pk.bfl || pk.anyColor || pk.pairs || pk.rdynOn {
		return
	}
	for t := 0; t < pk.nt; t++ {
		if ty := &pk.types[t]; ty.sacOnly || ty.pain != 0 || ty.dmask != ty.mask {
			return
		}
	}
	granted := false
	for u := 0; u < pk.nu; u++ {
		sl := pk.units[u]
		granted = granted || s.blgMask(k, sl)&^k.cards[s.cardAt(sl)].manaCol != 0
	}
	if !granted && pk.nu < planLimitUnits {
		return
	}
	var ls limitSearch
	ls.pk = *pk
	ls.granted = granted
	ls.census(k, s)
	ls.order()
	ls.walk(0)
	if !ls.limited || !ls.found {
		return
	}
	pk.bestN = int(ls.bestKeys.sources)
	for i := 0; i < pk.bestN; i++ {
		u := ls.bestU[i]
		pk.bestSeq[i], pk.bestTy[i], pk.bestCol[i] = u, pk.utyp[u], ls.bestA[i].col
	}
	pk.keepCols = true
}

// census groups the units into classes in order of each class's first unit.
func (ls *limitSearch) census(k *prep, s *State) {
	pk := &ls.pk
	for u := 0; u < pk.nu; u++ {
		var alts [planLimitAlts]lsAlt
		na := s.unitAlts(k, pk, u, &alts, ls.granted)
		ci := -1
		for c := 0; c < ls.nc; c++ {
			if cl := &ls.classes[c]; cl.alts == alts && pk.sameClassType(cl.ty, pk.utyp[u]) {
				ci = c
				break
			}
		}
		if ci < 0 {
			ci = ls.nc
			ls.nc++
			cl := &ls.classes[ci]
			cl.ty, cl.alts, cl.na, cl.amt, cl.irr = pk.utyp[u], alts, na, int32(pk.types[pk.utyp[u]].n), int32(pk.types[pk.utyp[u]].irr)
			for a := 0; a < na; a++ {
				cl.pick[a] = uint8(a)
				for j := a; j > 0 && alts[cl.pick[j]].step() < alts[cl.pick[j-1]].step(); j-- {
					cl.pick[j], cl.pick[j-1] = cl.pick[j-1], cl.pick[j]
				}
			}
		}
		cl := &ls.classes[ci]
		cl.units[cl.n] = uint8(u)
		cl.n++
	}
}

// sameClassType reports whether unit types a and b read the same in pay.Class:
// what the rank and the witness see of a source is its alternatives, its
// creature flag, its amount and its flexibility, not the card's own shape (a
// Plains Island surveil land and a Plains with the grant list the same
// alternatives and are one class).
func (pk *planner) sameClassType(a, b uint8) bool {
	if a == b {
		return true
	}
	x, y := &pk.types[a], &pk.types[b]
	return x.mask == y.mask && x.dmask == y.dmask && x.n == y.n && x.creat == y.creat && x.flex == y.flex && x.xv == y.xv &&
		x.sac == y.sac && x.irr == y.irr
}

// unitAlts lists unit u's alternatives. Without a choice-shaped ability, or
// with no granted type on the board, that is its own abilities in card order
// and then the intrinsic ability of each granted type (WUBRG); with both,
// unitAltsChoice emits the census order instead.
func (s *State) unitAlts(k *prep, pk *planner, u int, out *[planLimitAlts]lsAlt, granted bool) int {
	if granted {
		if n, ok := s.unitAltsChoice(k, pk, u, out, true); ok {
			return n
		}
	}
	left := pk.types[pk.utyp[u]].mask
	cd := &k.t.Cards[s.cardAt(pk.units[u])]
	n := 0
	// a granted type's intrinsic ability is appended unless an ability of the
	// card produces exactly that colour (gorge ManaAbilitiesProduce keys on
	// the Produced$ text): an any-colour ability does not
	var printed kop.ColorMask
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && bits.OnesCount8(uint8(a.ManaColors)) == 1 && a.AFlags&kop.AfManaChoice == 0 {
			printed |= a.ManaColors
		}
	}
	for c, i := range k.gen.blgSub {
		if i != subOutside && cd.Subtypes.Has(int(i)) {
			printed |= 1 << c // the card's own basic land type: its ability is listed above
		}
	}
	more := s.blgMask(k, pk.units[u]) &^ printed & kop.ColorMask(pk.types[pk.utyp[u]].mask)
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		for m := uint8(a.ManaColors) & left; m != 0; m &= m - 1 {
			c := uint8(bits.TrailingZeros8(m))
			ab := int8(j)
			if c < 5 && k.gen.blgSub[c] != subOutside && cd.Subtypes.Has(int(k.gen.blgSub[c])) {
				ab = -1
			}
			out[n] = lsAlt{c, ab}
			n++
			left &^= 1 << c
		}
	}
	for left |= uint8(more); left != 0; left &= left - 1 {
		out[n] = lsAlt{uint8(bits.TrailingZeros8(left)), -1}
		n++
	}
	return n
}

// order is pay.Search.order: the cheapest irreversible cost, non-creatures
// before creatures, then classes that can pay a short colour, the least
// flexible, battlefield order; inside a class, the alternatives making a short
// colour first.
func (ls *limitSearch) order() {
	pk := &ls.pk
	var shortMask uint8
	for c, n := range pk.need {
		if n > 0 {
			shortMask |= 1 << c
		}
	}
	rank := func(cl *lsClass) (int, int, int, int, uint8) {
		ty := &pk.types[cl.ty]
		creature, pips := 0, 1
		if ty.creat {
			creature = 1
		}
		if ty.mask&shortMask != 0 {
			pips = 0
		}
		return int(cl.irr), creature, pips, int(ty.flex), cl.units[0]
	}
	less := func(a, b *lsClass) bool {
		a0, a1, a2, a3, a4 := rank(a)
		b0, b1, b2, b3, b4 := rank(b)
		if a0 != b0 {
			return a0 < b0
		}
		if a1 != b1 {
			return a1 < b1
		}
		if a2 != b2 {
			return a2 < b2
		}
		if a3 != b3 {
			return a3 < b3
		}
		return a4 < b4
	}
	for i := 1; i < ls.nc; i++ {
		for j := i; j > 0 && less(&ls.classes[j], &ls.classes[j-1]); j-- {
			ls.classes[j], ls.classes[j-1] = ls.classes[j-1], ls.classes[j]
		}
	}
	var classStart [maxUnits + 1]int
	for c := 0; c < ls.nc; c++ {
		cl := &ls.classes[c]
		classStart[c] = ls.nl
		for pass := 0; pass < 2; pass++ {
			for a := 0; a < cl.na; a++ {
				if (shortMask>>cl.alts[a].col&1 != 0) == (pass == 0) {
					ls.levels[ls.nl] = lsLevel{uint8(c), uint8(a)}
					ls.nl++
				}
			}
		}
	}
	classStart[ls.nc] = ls.nl
	var later lsStats
	var laterCap [planLimitAny + 1]int32
	for c := ls.nc - 1; c >= 0; c-- {
		cl := &ls.classes[c]
		ty := &pk.types[cl.ty]
		var creature int32
		if ty.creat {
			creature = 1
		}
		var inClass lsStats
		var cols uint8
		for l := classStart[c+1] - 1; l >= classStart[c]; l-- {
			col := cl.alts[ls.levels[l].alt].col
			inClass.add(col, cl.amt, cl.irr, creature, int32(ty.flex))
			cols |= 1 << col
			ls.hereCols[l] = cols
		}
		ls.later[c] = later
		ls.laterCap[c] = laterCap
		for m := range laterCap {
			laterCap[m] += cl.n * inClass.most[m]
		}
		later = later.merge(&inClass)
	}
}

// hereStats is what order's inClass held at level: every alternative of the
// level's class from that level on, each an lsStats.add of the class's
// constants, so most[m] is the class's amount when m holds one of their colours.
func (ls *limitSearch) hereStats(level int) lsStats {
	cl := &ls.classes[ls.levels[level].class]
	ty := &ls.pk.types[cl.ty]
	cols := uint(ls.hereCols[level])
	h := lsStats{least: cl.amt, irr: cl.irr, flex: int32(ty.flex), any: true}
	if ty.creat {
		h.creature = 1
	}
	for m := 1; m <= planLimitAny; m++ {
		if uint(m)&cols != 0 {
			h.most[m] = cl.amt
		}
	}
	return h
}

// deficit is pay.Search.deficit over the pips and the total the pool leaves.
func (ls *limitSearch) deficit() ([kop.NumManaKinds]int32, int32) {
	var short [kop.NumManaKinds]int32
	var made int32
	for c := range short {
		made += ls.produced[c]
		if d := int32(ls.pk.need[c]) - ls.produced[c]; d > 0 {
			short[c] = d
		}
	}
	return short, max(int32(ls.pk.rawT)-made, 0)
}

func (ls *limitSearch) walk(level int) {
	if ls.nodes >= planLimitNodes {
		ls.limited = true
		return
	}
	ls.nodes++
	short, total := ls.deficit()
	if total == 0 && short == [kop.NumManaKinds]int32{} {
		ls.complete()
		return
	}
	if level == ls.nl || ls.sources >= planLimitActs || !ls.promising(level, &short, total) {
		return
	}
	lv := ls.levels[level]
	cl := &ls.classes[lv.class]
	col := cl.alts[lv.alt].col
	useful := ceilDiv32(total, cl.amt)
	if short[col] > 0 {
		useful = max(useful, ceilDiv32(short[col], cl.amt))
	}
	hi := min(cl.n-ls.used[lv.class], planLimitActs-ls.sources, useful)
	for n := hi; n >= 0; n-- {
		ls.take(lv, n)
		ls.walk(level + 1)
		ls.take(lv, -n)
		if ls.limited {
			return
		}
	}
}

func ceilDiv32(a, b int32) int32 { return (a + b - 1) / b }

func (ls *limitSearch) take(lv lsLevel, n int32) {
	if n == 0 {
		return
	}
	cl := &ls.classes[lv.class]
	ty := &ls.pk.types[cl.ty]
	ls.counts[lv.class][lv.alt] += n
	ls.used[lv.class] += n
	ls.produced[cl.alts[lv.alt].col] += n * cl.amt
	ls.sources += n
	if ty.creat {
		ls.creats += n
	}
	ls.flex += n * int32(ty.flex)
	ls.irr += n * cl.irr
}

// promising is pay.Search.promising.
func (ls *limitSearch) promising(level int, short *[kop.NumManaKinds]int32, total int32) bool {
	lv := ls.levels[level]
	var here lsStats
	avail := ls.classes[lv.class].n - ls.used[lv.class]
	if avail != 0 {
		here = ls.hereStats(level)
	}
	rest := here.merge(&ls.later[lv.class])
	if !rest.any {
		return false
	}
	cap := &ls.laterCap[lv.class]
	if avail*here.most[planLimitAny]+cap[planLimitAny] < total || rest.most[planLimitAny] == 0 {
		return false
	}
	need := max(1, ceilDiv32(total, rest.most[planLimitAny]))
	shortMask := 0
	for c, n := range short {
		if n > 0 {
			shortMask |= 1 << c
		}
	}
	for sub := shortMask; sub > 0; sub = (sub - 1) & shortMask {
		var want int32
		for c := range short {
			if sub&(1<<c) != 0 {
				want += short[c]
			}
		}
		if avail*here.most[sub]+cap[sub] < want || rest.most[sub] == 0 {
			return false
		}
		need = max(need, ceilDiv32(want, rest.most[sub]))
	}
	if ls.sources+need > planLimitActs {
		return false
	}
	if !ls.found {
		return true
	}
	lb := lsKeys{
		cost:      ls.irr + need*rest.irr,
		creatures: ls.creats + need*rest.creature,
		sources:   ls.sources + need,
		surplus:   max(ls.madeTotal()+need*rest.least-int32(ls.pk.rawT), 0),
		flex:      ls.flex + need*rest.flex,
	}
	lb.reserve, lb.remainder = ls.reserve()
	return lb.cmp(&ls.bestKeys) <= 0
}

func (ls *limitSearch) madeTotal() int32 {
	var made int32
	for _, n := range ls.produced {
		made += n
	}
	return made
}

// reserve is pay.Search.reserve: keys 6 and 7 from the current counts.
func (ls *limitSearch) reserve() (int32, int32) {
	pk := &ls.pk
	remaining := pk.colSrc
	for c := 0; c < ls.nc; c++ {
		if ls.used[c] == 0 {
			continue
		}
		if pk.types[ls.classes[c].ty].nocol {
			continue // a last-resort unit has no colours in gorge's census
		}
		colours := pk.types[ls.classes[c].ty].dmask
		for i := range remaining {
			if colours&(1<<i) != 0 {
				remaining[i] -= int(ls.used[c])
			}
		}
	}
	var reserve, remainder int32
	for _, left := range remaining {
		if left > 0 {
			remainder++
		}
	}
	for _, c := range pk.dOrder {
		reserve = reserve*int32(pk.dBase) + int32(min(remaining[c], pk.demand[c]))
	}
	return reserve, remainder
}

// complete is pay.Search.complete: a count vector beats the best on keys 2-7,
// or ties them with a smaller witness (pay.CompareSteps). The deficit
// arithmetic is exact for these one-colour alternatives, so no solver refuses it.
func (ls *limitSearch) complete() {
	keys := lsKeys{cost: ls.irr, creatures: ls.creats, sources: ls.sources, surplus: ls.madeTotal() - int32(ls.pk.rawT), flex: ls.flex}
	keys.reserve, keys.remainder = ls.reserve()
	c := 0
	if ls.found {
		if c = keys.cmp(&ls.bestKeys); c > 0 {
			return
		}
	}
	ls.materialize()
	if ls.found && c == 0 && !ls.witnessLess() {
		return
	}
	ls.found = true
	ls.bestKeys = keys
	ls.bestU, ls.bestA = ls.wU, ls.wA
}

func (ls *limitSearch) witnessLess() bool {
	pk := &ls.pk
	for i := 0; i < int(ls.sources); i++ {
		x, y := pk.uobj[ls.wU[i]], pk.uobj[ls.bestU[i]]
		if x != y {
			return x < y
		}
		if a, b := ls.wA[i].step(), ls.bestA[i].step(); a != b {
			return a < b
		}
	}
	return false
}

// materialize is pay.Search.materialize: the current count vector's least
// witness, steps in unit order, each the smallest source that still leaves
// every class enough later units for what it owes.
func (ls *limitSearch) materialize() {
	pk := &ls.pk
	owed := ls.used
	owedAlt := ls.counts
	var next [maxUnits]int32
	limit := func(c int) int {
		cl := &ls.classes[c]
		return int(cl.units[cl.n-owed[c]])
	}
	const none = int(^uint(0) >> 1)
	for i := 0; i < int(ls.sources); i++ {
		low1, low2, lowC := none, none, -1
		for c := 0; c < ls.nc; c++ {
			if owed[c] == 0 {
				continue
			}
			if l := limit(c); l < low1 {
				low1, low2, lowC = l, low1, c
			} else if l < low2 {
				low2 = l
			}
		}
		bestC, bestPos := -1, -1
		var bestID Obj
		for c := 0; c < ls.nc; c++ {
			if owed[c] == 0 {
				continue
			}
			upper := limit(c)
			other := low1
			if c == lowC {
				other = low2
			}
			if other != none {
				upper = min(upper, other-1)
			}
			cl := &ls.classes[c]
			for j := next[c]; j < cl.n && int(cl.units[j]) <= upper; j++ {
				id := pk.uobj[cl.units[j]]
				if bestC < 0 || id < bestID {
					bestC, bestPos, bestID = c, int(cl.units[j]), id
				}
			}
		}
		cl := &ls.classes[bestC]
		alt := uint8(0)
		for a := 0; a < cl.na; a++ {
			if owedAlt[bestC][cl.pick[a]] > 0 {
				alt = cl.pick[a]
				break
			}
		}
		ls.wU[i], ls.wA[i] = uint8(bestPos), cl.alts[alt]
		owed[bestC]--
		owedAlt[bestC][alt]--
		for c := 0; c < ls.nc; c++ {
			m := &ls.classes[c]
			for next[c] < m.n && int(m.units[next[c]]) <= bestPos {
				next[c]++
			}
		}
	}
}
