package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Mana: an affordability check for the option walk (Hall's condition over
// the player's untapped sources, exact for one-colour-per-mana sources) and
// gorge's autopay planner (engine/rules/pay) ported for tap-only sources.

// manaMask returns a source's production mask over bits W U B R G C.
func srcMask(pc *pcard) uint8 {
	m := uint8(pc.manaCol)
	return m & 0x3f
}

// srcMaskAt is the production mask of the source at slot sl with mana
// facts pc: its chosen colour for a chosen-colour source (none while it
// has chosen nothing), else pc's.
func (s *State) srcMaskAt(pc *pcard, sl Slot) uint8 {
	if pc.manaChosen {
		// a printed colour beside the chosen one (Combo W Chosen: Citadel
		// Gate) is in manaCol; a plain chosen-colour source has none
		if ch := s.perms[sl].chosen; ch < 5 {
			return 1<<ch | srcMask(pc)
		}
		return srcMask(pc)
	}
	return srcMask(pc)
}

// canTapForMana reports whether the permanent at sl is an untapped tap-only
// mana source its controller can use now.
func (s *State) canTapForMana(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	if p.flags&pfTapped != 0 {
		return false
	}
	var pc *pcard
	if k.gen != nil {
		pc = s.manaPCGen(k, sl) // the generated profile, not the printed card
	} else {
		pc = &k.cards[s.cardAt(sl)]
	}
	return s.canTapForManaPC(k, sl, pc)
}

// canTapForManaPC is canTapForMana for a caller that already holds the
// slot's mana facts: the walk computes manaPCGen once per source and reuses
// it for the availability check, the mask and the unit count.
func (s *State) canTapForManaPC(k *prep, sl Slot, pc *pcard) bool {
	p := &s.perms[sl]
	if p.flags&pfTapped != 0 {
		return false
	}
	if pc.manaAb < 0 || !(pc.manaTap || pc.manaSac) || p.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		return false
	}
	if s.activationSick(k, sl) {
		return false
	}
	return true
}

// avail is a player's mana availability summary for the option walk:
// mana units grouped by production mask (W U B R G C bits).
type avail struct {
	total int
	n     int
	mask  [16]uint8  // distinct production masks (nine on a five-gain-land board: pair 5901)
	cnt   [16]uint16 // wide enough that eight 255-mana sources never saturate
	// pain: the painful units' budget when their summed damage could be
	// lethal (mana_pain.go painTight): their colours are
	// then held out of mask/cnt and affordsPlain enumerates how many of
	// each group may make one
	pain painAvail
	// life: the payer's life, read only by a Phyrexian pip's two-life face
	// (affordsTb; set when the table has one: prep.phyMana)
	life int16
	// one: the amount of each source making all its mana in one colour of
	// its choosing. Those units are also in mask/cnt, where they read as
	// independent; hallPlain then tries each source's colours.
	one  [maxOneColour]uint8
	nOne uint8
	// nModal: the painful sources that make either {C} and a {C}-only extra
	// or a colour (mana_tapextra.go addPainAvail); painTight holds them in pain
	nModal uint8
	// xd: the payment an {X} ask is bounded by, when xdOn; a granted ability
	// whose spend restriction refuses it adds nothing (availGrant). A value, not
	// a pointer, so an avail on the stack keeps no reference to a local.
	xd   payDesc
	xdOn bool
	// delve: the graveyard credit a granted-delve cast prices its twobrid
	// faces against (gorge FeasibleAny's delve argument): the payer's
	// graveyard size while the cast carries the granted keyword, else 0. Set
	// by the offer gates that price a cast (legal.go, legal_gen.go offerPlay,
	// featview.go); the twobrid leaf checks (mana_twobrid.go) take
	// min(generic, delve) per face where gorge's composed check takes the
	// credit once at the leaf (CR 702.66a). Zero everywhere else, so every
	// other caller prices exactly as before.
	delve int
}

// maxOneColour bounds a player's one-colour sources; one more stalls
// StallDynMana. oneColourFits tries at most C(n+4,4) colour multisets, 495 at 8.
const maxOneColour = 8

// manaOneColour reports whether a makes n mana all in one colour it chooses
// (Kami of Whispered Hopes, "X mana of any one color"), as opposed to a Combo's
// colour per unit.
func manaOneColour(a *kop.Ability, n uint8) bool {
	return n > 1 && a.ManaColors == kop.ColAny && a.AFlags&kop.AfManaChoice == 0 && !a.ManaEach
}

//go:noinline
func (s *State) addOneColour(a *avail, n uint8) {
	if int(a.nOne) == len(a.one) {
		s.capFull(StallDynMana)
		return
	}
	a.one[a.nOne] = n
	a.nOne++
}

// oneColourFits: a, with each of ns a unit of that many mana of one colour,
// meets cost's Hall bound. ns is sorted ascending: sources of equal amount
// are interchangeable, so the search tries each multiset of colours per
// distinct amount rather than each colour per source.
func (a avail) oneColourFits(cost *kop.ManaCost, pipMask uint8, extra int, ns []uint8) bool {
	if len(ns) == 0 {
		return a.hallUnits(cost, pipMask, extra)
	}
	g := 1
	for g < len(ns) && ns[g] == ns[0] {
		g++
	}
	return a.oneColourSplit(cost, pipMask, extra, ns[0], g, 0, ns[g:])
}

// oneColourSplit gives g sources of amount n to the colours from c on, then
// fits the sources rest.
func (a avail) oneColourSplit(cost *kop.ManaCost, pipMask uint8, extra int, n uint8, g, c int, rest []uint8) bool {
	if c == 4 {
		for ; g > 0; g-- {
			a.addUnits(1<<c, n)
		}
		return a.oneColourFits(cost, pipMask, extra, rest)
	}
	b := a
	for k := 0; ; k++ {
		if b.oneColourSplit(cost, pipMask, extra, n, g-k, c+1, rest) {
			return true
		}
		if k == g {
			return false
		}
		b.addUnits(1<<c, n)
	}
}

//go:noinline
func (a *avail) oneColourPays(cost *kop.ManaCost, pipMask uint8, extra int) bool {
	b := *a
	ns := a.one
	sorted := ns[:a.nOne]
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	for _, n := range sorted {
		b.remove(uint8(kop.ColAny), n)
	}
	return b.oneColourFits(cost, pipMask, extra, sorted)
}

// satAdd is a+n held at the top of a count: a wrapped count would read as a
// short board. The top is out of reach of any board (255 mana a source).
func satAdd(a uint16, n uint8) uint16 {
	if s := int(a) + int(n); s < 65535 {
		return uint16(s)
	}
	return 65535
}

func (a *avail) addUnits(mask, n uint8) {
	a.total += int(n)
	for i := 0; i < a.n; i++ {
		if a.mask[i] == mask {
			a.cnt[i] = satAdd(a.cnt[i], n)
			return
		}
	}
	if a.n == len(a.mask) {
		// fold into the last group: an overcount only for exotic boards
		// with more than 16 distinct production masks
		a.mask[a.n-1] |= mask
		a.cnt[a.n-1] = satAdd(a.cnt[a.n-1], n)
		return
	}
	a.mask[a.n], a.cnt[a.n] = mask, uint16(n)
	a.n++
}

// remove takes n units of mask out (a source paying its own {T} cost).
func (a *avail) remove(mask, n uint8) { a.removeWide(mask, uint16(n)) }

// removeWide is remove for a group's whole count, which can pass a byte.
func (a *avail) removeWide(mask uint8, n uint16) {
	for i := 0; i < a.n; i++ {
		if a.mask[i] == mask && a.cnt[i] >= n {
			a.cnt[i] -= n
			a.total -= int(n)
			if a.nOne != 0 && mask == uint8(kop.ColAny) && n <= 255 {
				a.dropOneColour(uint8(n))
			}
			return
		}
	}
}

// dropOneColour forgets a removed source's one-colour entry.
func (a *avail) dropOneColour(n uint8) {
	for i := uint8(0); i < a.nOne; i++ {
		if a.one[i] == n {
			a.nOne--
			a.one[i] = a.one[a.nOne]
			return
		}
	}
}

func (s *State) availFor(k *prep, p uint8, a *avail) {
	a.total, a.n, a.nOne, a.nModal, a.pain.on = 0, 0, 0, 0, false
	pl := &s.pl[p]
	for c := 0; c < kop.NumManaKinds; c++ {
		if pl.pool[c] > 0 {
			a.addUnits(1<<c, pl.pool[c])
		}
	}
	m := s.ctrlM[p]
	// A granted colour beside the own ability (mana_grantedoffers.go): the
	// planner's census and the offer read it, and grantCap's potential walk
	// (availWalk) counts the relaxed alternatives. An activated ability's
	// prefix search (noGrant) does not: its granted mana is the grant
	// search's (grantCap/grantPrefix), and counting it here would pay the
	// activation with mana its spend restriction cannot spend.
	grantOK := !(s.g.noGrant && s.g.availX&availPlain != 0)
	dual := k.gen != nil && k.gen.anyContGrant && grantOK
	// A Taps-deferred source's classification (mana_tapsdefer.go) is a
	// battlefield fact, so its gate lifts out of the loop: without a live
	// TrigTaps listener every source is an ordinary unit, and the potential
	// walks count a deferred source's tap regardless.
	deferSkip := s.deferLive(k) && s.g.availX&(availWalk|availHyp) == 0
	var deferGap slotMask
	if deferSkip {
		// Only the production the script route models is a census gap the
		// prefix search pays: any other deferred source keeps its ordinary
		// unit here, or no walk would count it and a play it funds would
		// vanish from the offer (mana_tapsdefer.go).
		deferGap = s.scriptTapsDefer(k, p)
	}
	for m.any() {
		sl := m.first()
		m = m.rest()
		pm := &s.perms[sl]
		var pc *pcard
		if k.gen != nil {
			pc = s.manaPCGen(k, sl)
		} else {
			pc = &k.cards[s.cardAt(sl)]
		}
		if pc.manaAb < 0 && pm.ovr.flags&ovCopy == 0 {
			continue
		}
		if !grantOK && k.gen != nil && s.manaGranted(k, sl) {
			continue // a source whose only mana ability is granted: grantCap/grantPrefix's search, not another gap's
		}
		if !s.canTapForManaPC(k, sl, pc) && !(k.sacOnlyMana && s.canSacOnlyForMana(k, sl, pc)) && !(dual && s.grantSacLive(sl, pc)) {
			continue
		}
		if deferSkip && deferGap.has(sl) {
			continue // gorge's census never lists a source whose tap fires an unvetted Taps watcher (mana_tapsdefer.go); the script route offers and prices the modelled gap, and the potential walks (availWalk, its hyp bound) still count it, its census gate downstream
		}
		if dual {
			s.availGrant(k, sl, pc, a)
			continue
		}
		if k.painMana && pc.painCol != 0 {
			s.addPainAvail(k, sl, pc, p, a, nil, true)
			continue
		}
		a.addUnits(s.srcMaskAt(pc, sl), s.srcYieldAt(k, sl, pc))
		if pc.manaOne {
			s.addOneColour(a, pc.manaN)
		}
		if k.gen != nil && k.gen.anyTapExtra {
			xv := s.srcExtra(k, sl, pc)
			for c := range xv {
				if xv[c] > 0 {
					a.addUnits(1<<c, xv[c])
				}
			}
		}
	}
	if k.availExt {
		s.availExt(k, p, a) // pain budget, Phyrexian life (mana_pain.go)
	}
	if s.g.availX&3 != 0 {
		s.availScript(k, p, a)
	}
}

// affords reports whether a can pay cost plus extra generic.
func (a *avail) affords(cost *kop.ManaCost, pipMask uint8, extra int) bool {
	if cost.Hyb[0] != 0 {
		return a.affordsHyb(cost, extra)
	}
	return a.affordsPlain(cost, pipMask, extra)
}

// affordsPlain is affords for a cost without hybrid or twobrid pips. The
// hybrid paths resolve their pips and call it, never affords: a call cycle
// would move their resolved costs to the heap.
func (a *avail) affordsPlain(cost *kop.ManaCost, pipMask uint8, extra int) bool {
	if a.pain.on {
		return a.painAffords(cost, pipMask, extra)
	}
	return a.hallPlain(cost, pipMask, extra)
}

// hallPlain is affordsPlain over a's units alone.
func (a *avail) hallPlain(cost *kop.ManaCost, pipMask uint8, extra int) bool {
	return a.hallUnits(cost, pipMask, extra) && (a.nOne == 0 || a.oneColourPays(cost, pipMask, extra))
}

// hallUnits is hallPlain with every unit independent.
func (a *avail) hallUnits(cost *kop.ManaCost, pipMask uint8, extra int) bool {
	tot := int(cost.Generic) + extra
	for _, n := range cost.Pips {
		tot += int(n)
	}
	if tot > a.total {
		return false
	}
	if pipMask == 0 {
		return true
	}
	// Hall: every subset Q of the pip colours needs at least sum(pips in Q)
	// units whose mask meets Q.
	for q := uint8(pipMask); q != 0; q = (q - 1) & pipMask {
		need := 0
		for c := 0; c < kop.NumManaKinds; c++ {
			if q&(1<<c) != 0 {
				need += int(cost.Pips[c])
			}
		}
		have := 0
		for i := 0; i < a.n; i++ {
			if a.mask[i]&q != 0 {
				have += int(a.cnt[i])
			}
		}
		if need > have {
			return false
		}
	}
	return true
}

// affordsAnyColor reports whether a can pay cost under the may-play
// any-colour rider: a {C} pip needs a colourless-producing unit, every
// other pip needs a coloured-producing unit, and generic takes anything
// (gorge pay/solve.go expandCostPips anyColorAlts; CR 107.4c: "any color"
// never includes colourless).
func (a *avail) affordsAnyColor(cost *kop.ManaCost, extra int) bool {
	if cost.Hyb[0] != 0 {
		if cost.Hyb[0] == kop.HybTwobrid {
			return a.affordsTb(cost, extra, true)
		}
		n, f := hybFacesOf(cost)
		for m := 0; m < 1<<n; m++ {
			r := f.resolve(*cost, n, m)
			if a.affordsAnyColorPlain(&r, extra) {
				return true
			}
		}
		return false
	}
	return a.affordsAnyColorPlain(cost, extra)
}

// affordsAnyColorPlain is affordsAnyColor for a cost without hybrid or
// twobrid pips (no call cycle: see affordsPlain).
func (a *avail) affordsAnyColorPlain(cost *kop.ManaCost, extra int) bool {
	tot := int(cost.Generic) + extra
	for _, n := range cost.Pips {
		tot += int(n)
	}
	if tot > a.total {
		return false
	}
	needC := int(cost.Pips[kop.C])
	needCol := 0
	for c := 0; c < 5; c++ {
		needCol += int(cost.Pips[c])
	}
	haveC, haveCol, both := 0, 0, 0
	for i := 0; i < a.n; i++ {
		m := a.mask[i]
		n := int(a.cnt[i])
		hasC := m&(1<<kop.C) != 0
		hasCol := m&0x1f != 0
		if hasC {
			haveC += n
		}
		if hasCol {
			haveCol += n
		}
		if hasC && hasCol {
			both += n
		}
	}
	return needC <= haveC && needCol <= haveCol && needC+needCol <= haveC+haveCol-both
}

// affordsHyb: some assignment of cost's hybrid pips is affordable (gorge's
// whole-cost feasibility, costMods.feasibleAny).
func (a *avail) affordsHyb(cost *kop.ManaCost, extra int) bool {
	if cost.Hyb[0] == kop.HybTwobrid {
		return a.affordsTb(cost, extra, false)
	}
	n, f := hybFacesOf(cost)
	for m := 0; m < 1<<n; m++ {
		r := f.resolve(*cost, n, m)
		if a.affordsPlain(&r, pipMaskOf(&r), extra) {
			return true
		}
	}
	return false
}

// hybWitness resolves cost's hybrid pips the way gorge's potential planner
// does (rules planHybridCost): the first resolution, pip 0 the fastest
// varying, each pip's lower colour first, that a affords; the cost
// unchanged when it has none or none is affordable.
func (a *avail) hybWitness(cost kop.ManaCost, x int) kop.ManaCost {
	return a.hybWitnessRider(cost, x, false)
}

// hybWitnessRider is hybWitness with the may-play any-colour rider.
func (a *avail) hybWitnessRider(cost kop.ManaCost, x int, anyColor bool) kop.ManaCost {
	if cost.Hyb[0] == 0 {
		return cost
	}
	if cost.Hyb[0] == kop.HybTwobrid {
		panic("kernel: a twobrid cost reached the hybrid witness (every twobrid payment is announced first)")
	}
	n, f := hybFacesOf(&cost)
	for m := 0; m < 1<<n; m++ {
		r := f.resolve(cost, n, hybWitnessMask(m, n))
		var ok bool
		if anyColor {
			ok = a.affordsAnyColor(&r, r.XGeneric(x))
		} else {
			ok = a.affords(&r, pipMaskOf(&r), r.XGeneric(x))
		}
		if ok {
			return r
		}
	}
	return cost
}

// maxXAsk bounds the X the kernel searches and asks: gorge's largest-X search is
// bounded by the sources' total alone (rules potentialModeCastPlan), and a board of
// Elves under two Priests of Titania pays 22.
const maxXAsk = 60

// maxX is the largest X a can pay for cost (bounded by maxXAsk).
func (a *avail) maxX(cost *kop.ManaCost, pipMask uint8) int {
	if !a.affords(cost, pipMask, 0) {
		return -1
	}
	x := 0
	per := int(cost.X)
	if per == 0 {
		return 0
	}
	for x < maxXAsk && a.affords(cost, pipMask, cost.XGeneric(x+1)) {
		x++
	}
	return x
}

// maxXAnyColor is maxX under the any-colour rider.
func (a *avail) maxXAnyColor(cost *kop.ManaCost) int {
	if !a.affordsAnyColor(cost, 0) {
		return -1
	}
	x := 0
	per := int(cost.X)
	if per == 0 {
		return 0
	}
	for x < maxXAsk && a.affordsAnyColor(cost, cost.XGeneric(x+1)) {
		x++
	}
	return x
}

// --- the planner (gorge engine/rules/pay PlanPaymentCost, tap-only units) ---

// maxUnits bounds the planner's census. It is MaxPerm, not a smaller budget: a
// player controls at most MaxPerm battlefield slots and each source is one
// unit, so the census never truncates. gorge's census
// (paymentPlanManaUnitsOnlyCompute) is uncapped, and the key-8 tie-break must
// see every source: cutting the list at 48 dropped the least-ObjID source of a
// board with more than 48 lands (Generated Horizons' Forest tokens), so the
// kernel tapped a different Forest than gorge did.
const maxUnits = MaxPerm

// maxTypes bounds the planner's unit types: sixteen ordinary shapes plus a
// painland pair per painful card (mana_pain.go)
const maxTypes = 24

// unit kinds (types of interchangeable units).
type utype struct {
	mask    uint8 // production mask (W U B R G C)
	dmask   uint8 // the colours it ranks over: the production mask but for a widened source its restricted ability cannot pay (restrViewW)
	n       uint8 // mana per tap
	creat   bool
	sac     bool // a tap-and-sacrifice source (last resort)
	sacOnly bool // a sacrifice-only source (last resort; no tap)
	flex    uint8
	avail   uint8
	k       uint8 // chosen count (search scratch)
	// pain: the damage one activation deals its controller (a painful
	// last-resort alternative, mana_pain.go); pair: 1 + the
	// index of the other type of the same sources (a painland's {C} and
	// coloured alternatives share avail; the {C} type is the primary, alt
	// marks the painful one)
	pain uint8
	pair uint8
	alt  bool
	// rb: a restricted source's larger alternative (restrPairUnit): the
	// mana a tap makes floats restricted
	rb bool
	// irr: key 1's irreversible cost of one unit (pay.ConsequenceCost: 10
	// a sacrificed source, 20 a creature, 3 a point of damage); nocol: a
	// last-resort unit, whose colours take nothing from the remainder
	irr   uint8
	nocol bool
	// first: 1 + the colour of a gated source's ability 0. gorge's witness
	// compares a step's ability before what it produces, so such a source
	// makes that colour whenever the rest stays payable (CompareSteps).
	first uint8
	// printed: under a granted basic land type (static_landtype.go), the
	// colours of a nonbasic's own abilities. CompareSteps reads Kind first and
	// "intrinsic" sorts before "printed", so its granted colours come first.
	printed uint8
	// xv: the fixed mana each tap also makes beyond its colour choice (a
	// StatTapExtra static, mana_tapextra.go); xmask its colours, xt its total.
	// gorge's Alt.Mana holds it, so the rank keys read it as production
	xv    [kop.NumManaKinds]uint8
	xmask uint8
	xt    uint8
}

type planner struct {
	units    [maxUnits]Slot
	uobj     [maxUnits]Obj
	utyp     [maxUnits]uint8
	nu       int
	types    [maxTypes]utype
	nt       int
	need     [kop.NumManaKinds]int // pips the units must cover
	needT    int                   // total mana the units must produce
	needAny  int                   // any-colour rider: coloured pips the units must cover
	rawT     int                   // the cost's total less the pool, before needT's clamps: gorge's deficit and surplus read it (limitPlan)
	anyColor bool                  // the may-play MayPlayIgnoreColor rider is active
	bfl      bool                  // rank units in battlefield order, not ObjID (hybrid)
	colSrc   [5]int                // eligible units per colour (WUBRG)
	demand   [5]int
	dOrder   [5]uint8
	dBase    int
	// best
	found   bool
	bestKey [7]int
	bestSeq [maxUnits]uint8 // key 8's least unit sequence among the best keys
	bestCol [maxUnits]uint8 // the colour each of bestSeq's units makes (seqCols)
	bestTy  [maxUnits]uint8 // the type each of bestSeq's units takes (seqTypes: a painland's alternative)
	bestN   int
	maxN    int
	// first: tap only the plan's first unit (gorge's resolution payment
	// re-plans after every activation: respay.go resPayFund)
	first  bool
	lastSl Slot // the last source tapped (first: the one)
	// kept: tap-and-sacrifice sources an ability's sacrifice part reserves
	// (gorge PlanPaymentCostWithout's kept: their consuming alternative is
	// withheld); dry: select the plan without tapping it, and report no
	// plan as found == false (cost_disjointmana.go)
	kept slotMask
	dry  bool
	try  bool
	// pairs: some type has a pair (a painland in the last-resort phase);
	// life: the payer's life, the lethal guard on summed pain
	pairs bool
	life  int
	// tapsIn: the plan also taps the sources a Taps watcher defers (the
	// second pass of plan, mana_tapsdefer.go)
	tapsIn bool
	// rdyn: the restricted evaluated-amount source (Helga) the plan taps and
	// its restriction: its mana floats restricted (State.rb), spent first by
	// the payment (mana_restrictedsource.go payRiderRestr)
	rdyn   Slot
	rdynOn bool
	rF     kop.FilterID
	rV     uint8 // rF's activated-half variant (genPrep.restrVarOf)
	// rbOn: the plan tapped a restricted pair unit's larger alternative,
	// whose mana floats restricted (restrPairUnit)
	rbOn bool
	// rkind: the restricted record kind (rkWiden, rkExt) of each unit's
	// source, 0 for a plain one; rAb: the restricted ability each unit taps
	// (a view's manaAb; -1 none); rtap: the restricted-kind units a tap made,
	// with the colour each made and the ability it activated
	// (witnessManaRestricted)
	rkind [maxUnits]uint8
	rAb   [maxUnits]int8
	rtapN int
	rtap  [maxUnits]struct {
		sl  Slot
		col uint8
		ext bool
		ab  int8
	}
	// keepCols: bestCol is the search's witness (limitPlan), not the least
	// colours the taps would choose
	keepCols bool
	// noGrant: a hybrid resolution's trial leaves the granted sources out
	// of the plan (phase 2): gorge's planHybridCost resolution has a
	// witness only from printed sources
	noGrant bool
	// maxXt: some unit type carries a fixed extra (mana_tapextra.go)
	maxXt bool
	// noChosenPair: the census-only trial of a payment whose chosen-colour
	// widened pair gorge's census omits (plan)
	noChosenPair bool
}

// pay pays cost (+x generic) for p: floating pool first, then gorge's
// planned taps. exclude is a hand object not counted for HandReserve (the
// card being cast). It panics if the cost cannot be paid (the option walk
// only offers affordable actions).
func (s *State) pay(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj) {
	s.payRider(k, p, cost, x, exclude, false)
}

// payRider is pay with the may-play any-colour rider.
func (s *State) payRider(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, anyColor bool) {
	if cost.Hyb[0] != 0 {
		// an unannounced hybrid pip (a cost no pip ask resolved): gorge's
		// witness resolution
		r := s.hybResolveRider(k, p, *cost, x, anyColor)
		cost = &r
	}
	r0, ds := s.deferredPool(k, p) // the {R} a script's deferred source floats (mana_deferred.go)
	if k.restrSrc {
		s.payRiderRestr(k, p, cost, x, exclude, anyColor)
	} else {
		s.tapForRider(k, p, cost, x, exclude, anyColor)
		s.payFromPoolRider(k, p, cost, x, anyColor)
	}
	if r0 != 0 {
		s.deferredSpent(k, p, r0, ds)
	}
}

// tapFor taps gorge's planned sources for what the floating pool lacks of
// cost (+X), adding their mana to the pool without spending it.
func (s *State) tapFor(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj) {
	s.tapForN(k, p, cost, x, exclude, false)
}

// tapForN is tapFor; first taps only the plan's first source. It returns
// the last source tapped (NoSlot: none).
func (s *State) tapForN(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, first bool) Slot {
	var pk planner
	return s.tapForPK(k, p, cost, x, exclude, first, false, &pk)
}

// tapForRider is tapFor with the may-play any-colour rider.
func (s *State) tapForRider(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, anyColor bool) {
	var pk planner
	s.tapForPK(k, p, cost, x, exclude, false, anyColor, &pk)
}

// tapForPK is tapForN with the caller's planner (its kept and dry set).
func (s *State) tapForPK(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, first, anyColor bool, pk *planner) Slot {
	if cost.Hyb[0] != 0 {
		if _, ok := s.hybResolveTappable(k, p, cost, x, exclude, first, anyColor, pk); ok {
			return pk.lastSl // the winning resolution planned and tapped
		}
		// no resolution plans: keep the availability witness's choice, so
		// the plan below reports the play the planner cannot price
		r := s.hybResolveRider(k, p, *cost, x, anyColor)
		cost = &r
	}
	s.tapPlanNeed(k, p, cost, x, exclude, first, anyColor, pk)
	return pk.lastSl
}

// tapNeed sets pk's deficit for cost (+X) after p's pool (gorge
// pay/search.go deficit).
func (s *State) tapNeed(pk *planner, p uint8, cost *kop.ManaCost, x int, anyColor bool) {
	pl := &s.pl[p]
	mv := cost.MV() + cost.XGeneric(x)
	pk.anyColor = anyColor
	pk.need = [kop.NumManaKinds]int{}
	pk.needAny = 0
	poolTot := 0
	for c := 0; c < kop.NumManaKinds; c++ {
		poolTot += int(pl.pool[c])
	}
	if anyColor {
		// coloured pips are one fungible category; {C} stays strict
		if d := int(cost.Pips[kop.C]) - int(pl.pool[kop.C]); d > 0 {
			pk.need[kop.C] = d
		}
		colPips, colPool := 0, 0
		for c := 0; c < 5; c++ {
			colPips += int(cost.Pips[c])
			colPool += int(pl.pool[c])
		}
		if d := colPips - colPool; d > 0 {
			pk.needAny = d
		}
	} else {
		for c := 0; c < kop.NumManaKinds; c++ {
			if d := int(cost.Pips[c]) - int(pl.pool[c]); d > 0 {
				pk.need[c] = d
			}
		}
	}
	pk.needT = mv - poolTot
	pk.rawT = pk.needT
	if pk.needT < 0 {
		pk.needT = 0
	}
	needPips := pk.needAny
	for c := range pk.need {
		needPips += pk.need[c]
	}
	if pk.needT < needPips {
		pk.needT = needPips
	}
}

// tapPlanNeed sets the deficit and taps the plan for it.
func (s *State) tapPlanNeed(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, first, anyColor bool, pk *planner) {
	s.tapNeed(pk, p, cost, x, anyColor)
	pk.lastSl = NoSlot
	if pk.needT > 0 {
		pk.first = first
		s.planTiered(k, p, pk, exclude)
	}
}

// hybResolveTappable resolves cost's hybrid pips to the first resolution
// the planner can pay, gorge's planHybridCost order (pip 0 the fastest
// varying, each pip's printed first colour first; mana_hybridorder.go).
// hybResolveRider's availability counts a source's granted any-colour
// ability beside its printed one, but plan's census taps the printed
// ability only (a granted ability is a prefix gap, mana_grantedoffers.go):
// a resolution the offer gate's availability pays can be one the planner
// cannot (Chromatic Lantern beside Forests: {R/G}{R/G} witnessed {R}{R}),
// so the resolution is chosen by an actual plan. A failed trial taps
// nothing (pk.try); the winning one plans and taps, exactly as
// tapPlanNeed does. The resolved cost is returned by value (a pointer
// return moves it to the heap).
func (s *State) hybResolveTappable(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, first, anyColor bool, pk *planner) (kop.ManaCost, bool) {
	if cost.Hyb[0] == 0 || cost.Hyb[0] == kop.HybTwobrid {
		return kop.ManaCost{}, false // twobrid is announced before the payment (a pip ask)
	}
	n, f := hybFacesOf(cost)
	for m := 0; m < 1<<n; m++ {
		r := f.resolve(*cost, n, hybWitnessMask(m, n))
		tp := *pk
		tp.found, tp.try, tp.noGrant = false, true, true
		s.tapPlanNeed(k, p, &r, x, exclude, first, anyColor, &tp)
		if tp.needT > 0 && !tp.found {
			continue // this resolution has no source plan; try the next
		}
		tp.try, tp.noGrant = pk.try, pk.noGrant
		*pk = tp
		return r, true
	}
	return kop.ManaCost{}, false
}

// payWindow pays an unless or ward mana cost (kernel ChGenPay / ChWardPay)
// the way gorge's mana window is answered (rules/ward.go
// wardManaDecision, the oracle's payWindowAnswer, the spellbench payWindow
// builtin): while the pool cannot pay it, the first untapped mana source in
// the payer's battlefield list is activated, a choice of colours taking
// the first one (W U B R G C), whatever the cost still owes: the window's
// decision carries no ManaPayment, so the answer has no owed colour to
// prefer (windowTapUseful's noOwed); then the pool pays, or the payment fails short
// (short) when those colours left it unpaid. The window lists every
// mana ability, so a Bloom Tender (its colours) and a restricted source
// (Giada: its {W} floats, unspendable on the cost) are activated in their
// turn too (windowTap, script_gen.go).
func (s *State) payWindow(k *prep, p uint8, cost *kop.ManaCost) (paused, short bool) {
	_, held := s.windowTapWalk(k, p, cost, cost, false, false, true, false, true) // an unless / ward window takes a deferred source once the plan cannot cover the cost
	if s.g.winMode != winNone {
		for c := range held {
			held[c] += s.g.winHeld[c] // what an earlier stretch of the walk kept out of the pool
		}
		s.g.winHeld = held
		if s.res.ask == raGenWinTap {
			return true, false // a tap-another source's pick: the answer runs the walk on (mana_windowtapx.go)
		}
		s.g.winHeld = [kop.NumManaKinds]uint8{}
	}
	if m := s.restrPoolNonSpell(k, p); m != 0 {
		// the restricted mana its restriction admits here pays first (CR 106.6)
		if c := s.restrShortAdm(p, m, *cost); s.poolPays(p, &c) {
			s.payRestrSpend(k, p, m, cost, 0, NoObj, false)
			for c := range held {
				s.pl[p].pool[c] += held[c]
			}
			return false, false
		}
	}
	if !s.poolPays(p, cost) {
		// the colours the window took leave the cost unpaid (a two-colour
		// source answered its lowest colour): the taps and the floating
		// mana stay, as gorge's Done leaves them, and the payment fails
		for c := range held {
			s.pl[p].pool[c] += held[c]
		}
		return false, true
	}
	s.payFromPool(p, cost, 0)
	for c := range held {
		s.pl[p].pool[c] += held[c]
	}
	return false, false
}

// hybResolve is cost with its hybrid pips resolved as gorge's potential
// planner's witness (avail.hybWitness) over p's pool and untapped sources.
func (s *State) hybResolve(k *prep, p uint8, cost kop.ManaCost, x int) kop.ManaCost {
	return s.hybResolveRider(k, p, cost, x, false)
}

// hybResolveRider is hybResolve with the may-play any-colour rider.
func (s *State) hybResolveRider(k *prep, p uint8, cost kop.ManaCost, x int, anyColor bool) kop.ManaCost {
	var av avail
	s.availFor(k, p, &av)
	return av.hybWitnessRider(cost, x, anyColor)
}

// payFromPool spends the pool: coloured pips first, then generic in order
// C W U B R G (gorge pay/solve.go ResolveManaWith).
func (s *State) payFromPool(p uint8, cost *kop.ManaCost, x int) {
	pl := &s.pl[p]
	for c := 0; c < kop.NumManaKinds; c++ {
		if pl.pool[c] < cost.Pips[c] {
			panic("kernel: payment failed (pips)")
		}
		pl.pool[c] -= cost.Pips[c]
	}
	g := int(cost.Generic) + cost.XGeneric(x)
	order := [6]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G}
	for _, c := range order {
		for g > 0 && pl.pool[c] > 0 {
			pl.pool[c]--
			g--
		}
	}
	if g > 0 {
		panic("kernel: payment failed (generic)")
	}
	if s.g.poolArt[SeatIx(p)]|s.fbR[SeatIx(p)]|(s.pmOn>>SeatIx(p))&1 != 0 {
		s.artSync(p)
	}
}

// payFromPoolRider is payFromPool with the may-play any-colour rider.
func (s *State) payFromPoolRider(k *prep, p uint8, cost *kop.ManaCost, x int, anyColor bool) {
	if k.gen != nil && s.ctl == ctlBuild && s.bld().e.flags&sfAnyMana != 0 && s.bld().e.kind == seSpell {
		var c kop.ManaCost
		if o := s.bld().e.obj; s.convAnyType(k, p, o) && (s.bld().e.flags&sfMayPlay == 0 || !s.mayPlayAnyType(k, p, o)) {
			c = s.payConvPips(k, p, *cost) // the conversion alone: exact colours first
		} else {
			c = s.payAnyTypePips(k, p, *cost)
		}
		cost = &c
	}
	if k.gen != nil && k.gen.anyColSpent && s.ctl == ctlBuild {
		s.payFromPoolSpc(p, cost, x, anyColor) // a cast's colours spent (mana_converge.go)
		return
	}
	if anyColor {
		s.payFromPoolAnyColor(p, cost, x)
		return
	}
	s.payFromPool(p, cost, x)
}

// payAnyTypePips spends the pips of a cast under an any-type rider before its
// generic, each from the first of W U B R G C the pool holds (gorge
// expandCostPips' anyTypeAlts), and returns what is left of cost, which holds
// them as generic (anyType). Generic alone would take {C} first.
//
//go:noinline
func (s *State) payAnyTypePips(k *prep, p uint8, cost kop.ManaCost) kop.ManaCost {
	b := s.bld()
	fl, bf, fg := b.e.flags, b.bfl, b.fixedGen
	b.e.flags &^= sfAnyMana
	b.bfl &^= bflCostFixed
	pc := s.buildCost(k)
	b.e.flags, b.bfl, b.fixedGen = fl, bf, fg
	n := uint8(0)
	for _, v := range pc.Pips {
		n += v
	}
	n = min(n, cost.Generic)
	cost.Generic -= n
	pl := &s.pl[p]
	for _, c := range [6]int{kop.W, kop.U, kop.B, kop.R, kop.G, kop.C} {
		d := min(n, pl.pool[c])
		pl.pool[c] -= d
		n -= d
		if d > 0 && k.gen.anyColSpent && c != kop.C {
			b.e.spc = spcAdd(b.e.spc, c, int(d))
		}
	}
	cost.Generic += n // (no pool unit left: the plain spend reports it)
	return cost
}

// payFromPoolAnyColor spends the pool under the may-play any-colour rider:
// {C} pips from the colourless pool, coloured pips from any of W U B R G
// (gorge ResolveMana's anyColorAlts, W first), then generic in order
// C W U B R G.
func (s *State) payFromPoolAnyColor(p uint8, cost *kop.ManaCost, x int) {
	pl := &s.pl[p]
	if pl.pool[kop.C] < cost.Pips[kop.C] {
		panic("kernel: payment failed (pips)")
	}
	pl.pool[kop.C] -= cost.Pips[kop.C]
	n := 0
	for c := 0; c < 5; c++ {
		n += int(cost.Pips[c])
	}
	colOrder := [5]int{kop.W, kop.U, kop.B, kop.R, kop.G}
	for _, c := range colOrder {
		for n > 0 && pl.pool[c] > 0 {
			pl.pool[c]--
			n--
		}
	}
	if n > 0 {
		panic("kernel: payment failed (coloured pips)")
	}
	g := int(cost.Generic) + cost.XGeneric(x)
	order := [6]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G}
	for _, c := range order {
		for g > 0 && pl.pool[c] > 0 {
			pl.pool[c]--
			g--
		}
	}
	if g > 0 {
		panic("kernel: payment failed (generic)")
	}
	if s.g.poolArt[SeatIx(p)]|s.fbR[SeatIx(p)]|(s.pmOn>>SeatIx(p))&1 != 0 {
		s.artSync(p)
	}
}

func (s *State) plan(k *prep, p uint8, pk *planner, exclude Obj) {
	// units: fixed-production sources in battlefield (entry) order, then
	// choice-shaped sources (any colour, a choice of colours) in battlefield
	// order (gorge payment_plan.go paymentPlanManaUnitsOnlyCompute).
	var order [MaxPerm]Slot
	no := 0
	m := s.ctrlM[p]
	var rd payDesc
	if k.restrSrc {
		rd = s.payDescNow(exclude) // the restricted sources this payment may spend
	}
	// A Taps-deferred source's classification (mana_tapsdefer.go) is a
	// battlefield fact, so its gate lifts out of the loop: without a live
	// TrigTaps listener every source is an ordinary census unit.
	deferSkip := !pk.tapsIn && s.deferLive(k)
	for m.any() {
		sl := m.first()
		m = m.rest()
		if s.canTapForMana(k, sl) || k.sacOnlyMana && s.canSacOnlyAt(k, sl) || k.restrSrc && s.restrOnlyUsable(k, p, sl, &rd) {
			if deferSkip && s.planDeferred(k, p, sl) {
				continue // gorge's census never lists a source whose tap fires an unvetted Taps watcher (mana_tapsdefer.go)
			}
			order[no] = sl
			no++
		}
	}
	// insertion sort by timestamp (entry order)
	for i := 1; i < no; i++ {
		for j := i; j > 0 && s.perms[order[j]].ts < s.perms[order[j-1]].ts; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	// HandReserve demand: the most pips of each colour on any single nonland
	// card in hand other than the card being cast (gorge pay/planner.go:234).
	pl := &s.pl[p]
	maxD := 0
	for i := 0; i < int(pl.handN); i++ {
		o := pl.hand[i]
		if o == exclude {
			continue
		}
		pc := &k.cards[s.objs[o].card]
		if pc.land {
			continue
		}
		for c := 0; c < 5; c++ {
			d := int(pc.cost.Pips[c])
			for _, h := range pc.cost.Hyb {
				d += int(h>>c) & 1 // a hybrid pip demands both its colours (pay.CostPips)
			}
			if pc.cost.Tb != 0 {
				d += tbDemand(&pc.cost, c)
			}
			if d > pk.demand[c] {
				pk.demand[c] = d
				if d > maxD {
					maxD = d
				}
			}
		}
	}
	pk.dBase = maxD + 1
	for i := range pk.dOrder {
		pk.dOrder[i] = uint8(i)
	}
	for i := 1; i < 5; i++ { // stable sort by demand desc
		for j := i; j > 0 && pk.demand[pk.dOrder[j]] > pk.demand[pk.dOrder[j-1]]; j-- {
			pk.dOrder[j], pk.dOrder[j-1] = pk.dOrder[j-1], pk.dOrder[j]
		}
	}
	if pk.maxN == 0 {
		pk.maxN = 1
	}
	// phase 1: normal sources only; phase 2 (only when phase 1 proves no
	// plan) adds the last-resort tap-and-sacrifice sources (Treasure),
	// ranked by irreversible cost (gorge plancost.go:160-179, plan.go:104),
	// and the granted ones.
	// A granted source comes after the Treasures (phase 3): gorge's planner
	// never sees it (its census holds printed abilities only), so a plan
	// with a Treasure is gorge's; only a play no printed source pays reaches
	// it, and that play's prefix (grantPrefix) has floated its mana already.
	hasSac, hasGrant, hasPain := false, false, false
	pk.life = 1 << 30
	if k.painMana {
		pk.life = s.painPlanLife(p)
	}
	// each source's mana facts, read once for the three passes of every phase
	var pcs [MaxPerm]*pcard
	var granted [MaxPerm]bool
	var rv [MaxPerm]uint8
	var ra [MaxPerm]int8      // the ability a restricted view is (restrViewW)
	var rk, dm [MaxPerm]uint8 // a restricted source's record kind and unadmitted ranking colours (restrViewW)
	var intr [MaxPerm]uint8   // a widened source's intrinsic colours: its granted basic land types' (static_landtype.go)
	var tapsPain [4]pcard     // pain-only views of the sources a Pain-class Taps watcher prices (mana_tapsdefer.go)
	ntp := 0
	for i := 0; i < no; i++ {
		sl := order[i]
		if k.gen != nil {
			pcs[i] = s.manaPCGen(k, sl)
			if k.gen.anyTapsPain && ntp < len(tapsPain) {
				if pn, _ := s.tapsWatchClass(k, p, sl); pn > 0 && pcs[i].painCol == 0 {
					if col := kop.ColorMask(s.srcMaskAt(pcs[i], sl) & 0x1f); col != 0 {
						tapsPain[ntp] = *pcs[i]
						tapsPain[ntp].painCol, tapsPain[ntp].painN, tapsPain[ntp].painOnly = col, uint8(pn), true
						pcs[i] = &tapsPain[ntp]
						ntp++
					}
				}
			}
			granted[i] = s.manaGranted(k, sl)
			if k.restrSrc && k.gen.flags[s.cardAt(sl)]&gfRestrSrc != 0 {
				if v, kd, ab, dmask, abi := s.restrViewW(k, p, sl, pcs[i], &rd); v != nil {
					if pk.noChosenPair && v.wideChosen {
						// The census-only trial: gorge's census omits a choice-shaped
						// larger alternative whose amount is not 1
						// (payment_plan.go paymentPlanManaUnitsOnlyCompute), so for
						// this search the source is only its ordinary ability.
					} else {
						ra[i] = abi
						if kd == rkWiden && pcs[i].blgFirst != 0 {
							// the restricted view is the card's: the intrinsic
							// abilities a granted basic land type gives (Jasmine
							// Dragon Tea Shop as an Island) rank first, gorge's
							// key 8 comparing an ability's kind before its mana
							intr[i] = uint8(pcs[i].manaCol&0x1f) &^ s.blgPrintedMask(k, sl)
						}
						pcs[i], rk[i], rv[i] = v, kd, kd
						if kd == rkDyn {
							pk.rdyn, pk.rdynOn, pk.rF, pk.rV = sl, true, ab.ManaRestrict, k.gen.restrVarOf(ab)
						}
					}
				} else if dmask != 0 {
					// the restricted alternative may not spend this payment:
					// the source is still a unit of its census ability, whose
					// search shape (deterministic or choice) stays its own, but
					// it ranks over every alternative's colours (gorge's
					// Alt.Flex/Alt.Colours). rk stays 0: the tap made no
					// restricted mana.
					dm[i] = dmask
				}
			}
		} else {
			pcs[i] = &k.cards[s.cardAt(sl)]
		}
	}
	// A chosen-colour widened pair is no unit of gorge's census while the
	// plain census pays (see the scan above): search once without it, and
	// fall back to the pair only when that plan proves no payer.
	if !pk.noChosenPair && k.restrSrc {
		chosen := false
		for i := 0; i < no && !chosen; i++ {
			chosen = rv[i] == rkWiden && pcs[i].wideChosen
		}
		if chosen {
			tp := *pk
			tp.noChosenPair = true
			tp.try = true // a census plan the pair must widen is not a panic
			s.plan(k, p, &tp, exclude)
			if tp.found {
				tp.noChosenPair, tp.try = false, pk.try
				*pk = tp
				return
			}
		}
	}
	for phase := 0; phase < 3; phase++ {
		if phase == 1 && !hasSac && !hasPain || phase == 2 && (!hasGrant || pk.noGrant) {
			continue
		}
		if phase > 0 {
			pk.nu, pk.nt, pk.maxN, pk.colSrc = 0, 0, 0, [5]int{}
		}
		for pass := 0; pass < 3; pass++ {
			for i := 0; i < no && pk.nu < maxUnits; i++ {
				sl := order[i]
				pc := pcs[i]
				if phase < 2 && granted[i] && rk[i] == 0 { // (an admitted restricted printed ability stays a census unit)
					// one whose mana ability an effect grants (gorge's census
					// holds printed abilities only: Imprisoned in the Moon's
					// land)
					hasGrant = true
					continue
				}
				if k.painMana && pc.painCol != 0 && phase == 0 {
					hasPain = true
					if pc.painOnly {
						continue // no normal alternative
					}
				}
				sacOnly := s.canSacOnlyForMana(k, sl, pc)
				sac := pc.manaSac || sacOnly
				if phase == 0 && sac {
					hasSac = true // last resort: a sacrificing source
					continue
				}
				if sac && pk.kept.any() && pk.kept.has(sl) {
					continue // reserved for the cost's sacrifice
				}
				// gorge's census lists a source with a deterministic
				// production first and appends the choice-shaped ones
				// (Produced$ Any, a Combo of two or more colours: the gain
				// lands) after, then the evaluated-amount layer (Elvish
				// Archdruid's {G} for each Elf) last, each in battlefield
				// order (payment_plan.go paymentPlanManaUnitsOnlyCompute)
				mask := s.srcMaskAt(pc, sl)
				choice := (pc.manaCol == kop.ColAny || bits.OnesCount8(mask) > 1 || pc.manaChosen) && !pc.manaMulti
				if choice && k.gen != nil && k.gen.blgOn && pc.manaCol == kop.ColAny && s.blgMask(k, sl) != 0 {
					// a Produced$ Any land with a granted basic land type
					// also has the intrinsic abilities, which are
					// deterministic: gorge's census lists it with the
					// fixed-production sources, the Any ability appended to
					// that unit
					choice = false
				}
				// a tap-and-sacrifice source of fixed production (Basal
				// Thrull's {B}{B}) is outside gorge's shared census (free-cost
				// abilities only): it is appended with the choice-shaped ones,
				// in battlefield order (paymentPlanManaUnitsOnlyCompute)
				choice = choice || sac
				if choice && !sac && k.gen != nil && len(k.gen.grantPC) > 0 && s.grantMay(k, sl) && s.grantsFixedMana(k, sl) {
					// a grant of fixed production (Rok-Tahk's {G}{G} to a creature) makes the
					// source's census unit in the shared census, at its battlefield position;
					// its own choice-shaped ability joins that unit
					choice = false
				}
				if rk[i] != 0 {
					choice = rk[i] == rkExt || DebugRestrictedSource == 3 && rk[i] == rkWiden // a widened unit keeps its census position
				}
				if k.painMana && pc.painCol != 0 && phase > 0 {
					// gorge's census lists a source with a deterministic,
					// free-cost production in the shared census, at its
					// battlefield position, and appends the rest after: a
					// choice-shaped ability, a tap-and-sacrifice one, an
					// ability that pays life (payment_plan.go
					// paymentPlanManaUnitsOnlyCompute). The paired pain unit
					// sits on the pass the source's census group is; a
					// pain-only source's shape is its own ability's (Elves
					// of Deep Shadow's {B} is deterministic, in the shared
					// census)
					want := 0
					if choice {
						want = 1
					}
					if s.painUnit(k, sl, pc, pass, pk, i, want) {
						continue
					}
				}
				if pc.manaDyn {
					if pass != 2 {
						continue
					}
				} else if pass == 2 || (pass == 0) == choice {
					continue
				}
				if pc.wideN != 0 && rv[i] == rkWiden && s.restrPairUnit(k, sl, pc, pk, i, true, ra[i]) {
					continue
				}
				if pc.wideN != 0 && rv[i] == 0 && k.gen != nil && k.gen.anyGatePair && k.gen.flags[s.cardAt(sl)]&(gfManaGate|gfManaDynPair) != 0 && debugGatedPair != 2 && s.restrPairUnit(k, sl, pc, pk, i, false, -1) {
					continue // a gated same-colour larger ability: the pair, its larger alternative ordinary mana (mana_gate.go)
				}
				cr := s.isCreature(k, sl)
				dmk := mask
				if dm[i] != 0 {
					dmk = dm[i]
				}
				var first, printed uint8
				if pc.manaMulti && k.gen != nil {
					if c0, ok := s.vergeAbility0Col(k, sl); ok {
						first = uint8(c0) + 1
					}
					if pc.blgFirst != 0 {
						printed = s.blgPrintedMask(k, sl)
					}
				}
				if intr[i] != 0 && rv[i] == rkWiden {
					printed = mask &^ intr[i] // the restricted ability's colours and the own printed ones follow
				}
				var xv [kop.NumManaKinds]uint8
				var xm, xt, tn uint8 = 0, 0, pc.manaN
				if k.gen != nil && k.gen.anyTapExtra && !sacOnly {
					tn += s.srcReflectPick(k, sl, pc) // a several-colour source under a reflect carrier: one more of its pick
					if pc.painCol != 0 {
						xv, _, _ = s.srcExtraPain(k, sl, pc) // phase 0: the painland's normal {C} ability
					} else {
						xv = s.srcExtra(k, sl, pc)
					}
					for c, v := range xv {
						if v > 0 {
							xm |= 1 << c
							xt += v
						}
					}
				}
				fl := uint8(bits.OnesCount8(dmk | xm))
				if phase > 0 && k.painMana && pc.painCol != 0 {
					xc, xp, _ := s.srcExtraPain(k, sl, pc)
					fl = painFlexAll(pc, xc, xp) // the withheld painful alternative's colours still count
				}
				rb := pc.restrFloat
				ti := -1
				for t := 0; t < pk.nt; t++ {
					ty := &pk.types[t]
					if ty.mask == mask && ty.dmask == dmk && ty.n == tn && ty.creat == cr && ty.sac == sac && ty.sacOnly == sacOnly && ty.pair == 0 && ty.pain == 0 && ty.first == first && ty.printed == printed && ty.flex == fl && ty.xv == xv && ty.rb == rb {
						ti = t
						break
					}
				}
				if ti < 0 {
					if pk.nt == maxTypes {
						continue // no room for another shape (no pool board reaches it)
					}
					ti = pk.nt
					pk.nt++
					pk.types[ti] = utype{mask: mask, dmask: dmk, n: tn, creat: cr, flex: fl, sac: sac, sacOnly: sacOnly, nocol: sac, first: first, printed: printed, xv: xv, xmask: xm, xt: xt, rb: rb}
					if sac {
						pk.types[ti].irr = 10
						if cr {
							pk.types[ti].irr = 20
						}
					}
					pk.maxXt = pk.maxXt || xt != 0
					if int(tn)+int(xt) > pk.maxN {
						pk.maxN = int(tn) + int(xt)
					}
				}
				pk.types[ti].avail++
				pk.units[pk.nu] = sl
				if pk.bfl {
					// a hybrid cast is paid through gorge's manual CR 601.2g
					// window (pay.PlanCostDetail "cost:hybrid"), whose source
					// order is pay.ManaSourceIDs: battlefield (entry) order,
					// not the planner witness's ObjID order (pay.CompareSteps)
					pk.uobj[pk.nu] = Obj(i)
				} else {
					pk.uobj[pk.nu] = s.perms[sl].obj
				}
				pk.utyp[pk.nu] = uint8(ti)
				pk.rkind[pk.nu] = rv[i]
				pk.rAb[pk.nu] = pc.manaAb
				if rv[i] == rkWiden && k.gen != nil && k.gen.rs != nil {
					pk.rAb[pk.nu] = ra[i] // the widened view's restricted ability
				}
				pk.nu++
				if sac {
					// a last-resort source has no normal alternative, so
					// its colours are 0 in gorge's census: keys 6 and 7
					// read the untapped NORMAL remainder in both phases
					// (pay/plan.go Alt.Colours, plancost.go rankCtx)
					continue
				}
				for c := 0; c < 5; c++ {
					if (dmk|xm)&(1<<c) != 0 {
						pk.colSrc[c]++
					}
				}
			}
		}
		if pk.maxN == 0 {
			pk.maxN = 1
		}
		pk.dfs(0, 0, 0)
		if pk.found {
			break
		}
	}
	if !pk.found {
		if pk.dry || pk.try {
			return
		}
		panic("kernel: autopay found no plan")
	}
	if pk.dry {
		return
	}
	if k.gen != nil {
		s.limitPlan(k, pk)
	}
	// key 8 (consider): among the tied count vectors, the lexicographically
	// least ObjID sequence in unit order.
	best, bn := &pk.bestSeq, pk.bestN
	if pk.first && bn > 1 {
		bn = 1 // pickColour below still reads the whole plan
	}
	// tap in unit order, choosing each unit's colour: the smallest Produces
	// vector first (C, then G R B U W) that keeps the rest payable.
	var rem [kop.NumManaKinds]int
	rem = pk.need
	remAny := pk.needAny
	pk.fixedOffSeq(&rem, &remAny, &pk.bestTy, pk.bestN)
	for i := 0; i < bn; i++ {
		u := best[i]
		sl := pk.units[u]
		if k.gen != nil && k.gen.castTapOn && s.buildLive() && s.bld().e.kind == seSpell && k.cards[s.cardAt(sl)].castSrc != 0 {
			s.noteTapSrc(k, &k.cards[s.cardAt(sl)], sl)
		}
		ty := &pk.types[pk.bestTy[i]]
		col := int(pk.bestCol[i])
		if !pk.keepCols {
			col = pk.pickUnitColour(ty, &rem, remAny, &pk.bestTy, i+1, pk.bestN)
		}
		if !ty.sacOnly {
			s.perms[sl].flags |= pfTapped
			s.touchSl(sl)
			s.manaTapped(k, sl)
		}
		pk.lastSl = sl
		if r := pk.rkind[u]; (r == rkExt || r == rkWiden) && pk.rtapN < maxUnits {
			pk.rtap[pk.rtapN] = struct {
				sl  Slot
				col uint8
				ext bool
				ab  int8
			}{sl, uint8(col), r == rkExt, pk.rAb[u]}
			pk.rtapN++
		}
		s.pl[p].pool[col] += ty.n
		if ty.rb && DebugRestrictedSource != 5 {
			// a restricted source's larger alternative (or its sole ability of
			// more than one mana): its mana floats restricted (restrPairUnit,
			// prepRestrSrc's rkExt over)
			s.pl[p].pool[col] -= ty.n
			abi := pk.rAb[u]
			if abi < 0 {
				abi = k.gen.restrRecOf(s.cardAt(sl)).ab
			}
			if !s.floatRestrOnly(k, p, sl, &k.t.Cards[s.cardAt(sl)].Abilities[abi], col, ty.n) {
				s.capFull(StallRestrSrc)
			}
			pk.rbOn = true
		}
		if ty.xt != 0 {
			s.addXv(p, &ty.xv)
		}
		s.artTag(k, p, sl, col)
		if pk.rdynOn && sl == pk.rdyn {
			// a restricted evaluated amount (Helga) floats restricted beside
			// the pool: the payment spends it first and what it leaves stays
			// (payRestrRider); a State.rb that cannot hold it stalls loud
			s.pl[p].pool[col] -= ty.n
			if !s.addRestrSrcV(k, p, sl, pk.rF, pk.rV, col, ty.n) {
				s.capFull(StallRestrSrc)
			}
		}
		if ty.pain != 0 {
			s.painDamage(k, sl, p, int(ty.pain))
		}
		if ty.sac {
			o := s.perms[sl].obj
			s.fireSac(k, sl)
			if s.manaSacParks() && s.park(k, o, parkLeave, parkManaHold(col, ty.n)) {
				if s.buildLive() && s.bld().bfl&bflManaStep != 0 {
					s.bld().bfl |= bflPlanCut
				}
				s.pl[p].pool[col] -= ty.n // it floats once the move has its answer (CR 605.3b)
				return                    // and the plan's other sources wait for it too: the mana step runs again after the answer
			} else {
				s.leave(k, sl, kop.ZoneGraveyard, false)
				if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard && s.buildLive() && (s.ctl != ctlBuild || s.bld().bfl&bflManaStep == 0) {
					s.ceaseManaToken(o) // a token sacrificed to pay a cast ceases at once, as gorge pays (a Treasure); one sacrificed to pay mid-resolution waits for the state-based check
				}
				s.sbaAfterManaSacrifice(k)
			}
		}
		if pk.anyColor && col != kop.C {
			remAny = max(remAny-int(ty.n), 0)
		} else if rem[col] > 0 {
			rem[col] = max(rem[col]-int(ty.n), 0)
		}
	}
}

// sbaAfterManaSacrifice runs the state-based check gorge reaches after a mana
// ability's own sacrifice while a cast is being paid. Gorge poses the
// window's activation as a decision (a Treasure or Lotus also asks its
// colour), and that answer's boundary runs the check, so by the cast's next
// choice an Aura whose host it sacrificed has already gone to the graveyard
// (CR 704.3, 704.5m). The kernel pays the window in one step, so it runs the
// same check at each sacrifice or the Aura would linger on the battlefield
// until the cast settles, moving the graveyard order.
func (s *State) sbaAfterManaSacrifice(k *prep) {
	// Only a manually paid spell cast. A planned activation (bflManaStep) is
	// one internal step of the payment plan, and gorge leaves its sacrificed
	// token and its attachments to the next state-based check (state.go
	// bflManaStep); an activated ability's own resolution follows its payment
	// in the same action, so its check also waits for the next priority
	// boundary, where a graveyard count in the effect still sees the token.
	if s.buildLive() && s.bld().e.kind == seSpell && s.bld().bfl&bflManaStep == 0 {
		s.sba(k)
	}
}

// pickUnitColour is pickColour for a unit of type ty: a nonbasic with a granted
// basic land type makes a granted colour before one of its own (utype.printed).
func (pk *planner) pickUnitColour(ty *utype, rem *[kop.NumManaKinds]int, remAny int, tys *[maxUnits]uint8, from, n int) int {
	if ty.printed != 0 {
		if m := ty.mask &^ ty.printed; m != 0 {
			for _, c := range [5]int{kop.G, kop.R, kop.B, kop.U, kop.W} {
				if m&(1<<c) != 0 && pk.colourKeepsPayable(c, int(ty.n), rem, remAny, tys, from, n) {
					return c
				}
			}
		}
		return pk.pickColour(ty.printed, 0, int(ty.n), rem, remAny, tys, from, n)
	}
	return pk.pickColour(ty.mask, ty.first, int(ty.n), rem, remAny, tys, from, n)
}

// pickColour picks the colour a unit of mask produces, given the pips still
// to cover and the units after it: first's colour (utype.first) if that keeps
// the rest payable, then the smallest Produces vector that does.
// tys holds the type of each plan position (seqTypes).
func (pk *planner) pickColour(mask, first uint8, amt int, rem *[kop.NumManaKinds]int, remAny int, tys *[maxUnits]uint8, from, n int) int {
	if first != 0 && pk.colourKeepsPayable(int(first-1), amt, rem, remAny, tys, from, n) {
		return int(first - 1)
	}
	pref := [6]int{kop.C, kop.G, kop.R, kop.B, kop.U, kop.W}
	for _, c := range pref {
		if mask&(1<<c) != 0 && pk.colourKeepsPayable(c, amt, rem, remAny, tys, from, n) {
			return c
		}
	}
	panic("kernel: autopay colour assignment failed")
}

// colourKeepsPayable reports whether the units from position from on still
// cover the pips once a unit makes amt mana of colour c (Elvish Archdruid's
// amt is above one).
func (pk *planner) colourKeepsPayable(c, amt int, rem *[kop.NumManaKinds]int, remAny int, tys *[maxUnits]uint8, from, n int) bool {
	if pk.anyColor {
		rc, ra := *rem, remAny
		if c == kop.C {
			rc[c] = max(rc[c]-amt, 0)
		} else {
			ra = max(ra-amt, 0)
		}
		return pk.coverableAny(rc[kop.C], ra, tys, from, n)
	}
	r := *rem
	r[c] = max(r[c]-amt, 0)
	return pk.coverable(&r, tys, from, n)
}

// coverableAny is coverable under the any-colour rider: {C} pips need a
// colourless-producing unit, coloured pips a coloured-producing one.
func (pk *planner) coverableAny(rC, rAny int, tys *[maxUnits]uint8, from, n int) bool {
	haveC, haveCol, both := 0, 0, 0
	for i := from; i < n; i++ {
		ty := &pk.types[tys[i]]
		v := int(ty.n)
		hasC := ty.mask&(1<<kop.C) != 0
		hasCol := ty.mask&0x1f != 0
		if hasC {
			haveC += v
		}
		if hasCol {
			haveCol += v
		}
		if hasC && hasCol {
			both += v
		}
	}
	return rC <= haveC && rAny <= haveCol && rC+rAny <= haveC+haveCol-both
}

func (pk *planner) coverable(r *[kop.NumManaKinds]int, tys *[maxUnits]uint8, from, n int) bool {
	var pm uint8
	for c := range r {
		if r[c] > 0 {
			pm |= 1 << c
		}
	}
	for q := pm; q != 0; q = (q - 1) & pm {
		need := 0
		for c := range r {
			if q&(1<<c) != 0 {
				need += r[c]
			}
		}
		mx := 0
		for c := range r {
			if q&(1<<c) != 0 && r[c] > mx {
				mx = r[c]
			}
		}
		have := 0
		for i := from; i < n; i++ {
			ty := &pk.types[tys[i]]
			if ty.mask&q != 0 {
				// a unit producing n mana of one colour covers at most
				// one colour's demand, not the whole subset's
				cap := int(ty.n)
				if bits.OnesCount8(q) > 1 && cap > mx {
					cap = mx
				}
				have += cap
			}
		}
		if need > have {
			return false
		}
	}
	if pk.maxN > 1 {
		return pk.assignable(*r, tys, from, n)
	}
	return true
}

// assignable reports whether units tys[from:n] can each pick one colour and
// together cover r. The capacity checks are a relaxation once a unit makes
// several mana: seven of any one colour pays one colour's demand, however
// many colours are short. A unit that can pay a short colour is never better
// spent elsewhere, so only those colours are tried.
func (pk *planner) assignable(r [kop.NumManaKinds]int, tys *[maxUnits]uint8, from, n int) bool {
	short := false
	for _, v := range r {
		short = short || v > 0
	}
	if !short {
		return true
	}
	if from == n {
		return false
	}
	ty := &pk.types[tys[from]]
	pays := false
	for c, v := range r {
		if v > 0 && ty.mask&(1<<c) != 0 {
			pays = true
			next := r
			next[c] = max(v-int(ty.n), 0)
			if pk.assignable(next, tys, from+1, n) {
				return true
			}
		}
	}
	return !pays && pk.assignable(r, tys, from+1, n)
}

func (pk *planner) dfs(t int, prod int, creat int) {
	if t == pk.nt {
		if prod < pk.needT || !pk.valid() {
			return
		}
		pk.consider()
		return
	}
	ty := &pk.types[t]
	lim := int(ty.avail)
	if pk.pairs && ty.alt {
		lim -= int(pk.types[ty.pair-1].k) // the primary {C} type is counted first (painUnit)
	}
	for c := 0; c <= lim; c++ {
		p := prod + c*(int(ty.n)+int(ty.xt))
		if c > 0 && p >= pk.needT+pk.maxN && !pk.unitNeeded(ty, c) {
			break
		}
		ty.k = uint8(c)
		cr := creat
		if ty.creat {
			cr += c
		}
		pk.dfs(t+1, p, cr)
	}
	ty.k = 0
}

// unitNeeded reports whether the c-th unit of ty can still cover a pip that
// c-1 of its kind leave open. The walk's total-mana cut (the cost plus the
// largest unit) is a minimal-plan argument that holds only while every unit
// is replaceable: when each of several sources covers a pip no other does
// (Swamp, a Tinder Wall's {R}{R}, a Battlement's three {G} for {B}{R}{G}),
// their total passes the cut with none of them spare. Gorge's own walk caps a
// level by the colour it still owes (pay/search.go walk), so a unit that owes
// a colour is never cut. A looser answer only adds candidates that valid
// and the ranking keys already reject as supersets.
func (pk *planner) unitNeeded(ty *utype, c int) bool {
	if pk.anyColor {
		return true
	}
	// A tap makes one colour, so c-1 units of an any-colour kind cover at
	// most c-1 of the colours still owed: the units each colour needs add up
	// (a Lotus token's three {R} or three {G}, for {R}{G}, takes two).
	// A unit also covers the colours of its fixed extra (a bounce land's
	// {W}{U} takes a {U}{U} cost from two lands though its unit is {W}).
	owed := 0
	for col := range pk.need {
		if pk.need[col] <= 0 {
			continue
		}
		per := int(ty.xv[col])
		if ty.mask&(1<<col) != 0 {
			per += int(ty.n)
		}
		if per > 0 {
			owed += (pk.need[col] + per - 1) / per
		}
	}
	return owed > c-1
}

// valid: the chosen units can cover the pips (Hall over colour subsets).
func (pk *planner) valid() bool {
	nd, ndAny := pk.need, pk.needAny
	if pk.maxXt {
		pk.fixedOff(&nd, &ndAny)
	}
	if pk.anyColor {
		haveC, haveCol, both := 0, 0, 0
		for t := 0; t < pk.nt; t++ {
			ty := &pk.types[t]
			v := int(ty.k) * int(ty.n)
			hasC := ty.mask&(1<<kop.C) != 0
			hasCol := ty.mask&0x1f != 0
			if hasC {
				haveC += v
			}
			if hasCol {
				haveCol += v
			}
			if hasC && hasCol {
				both += v
			}
		}
		return nd[kop.C] <= haveC && ndAny <= haveCol &&
			nd[kop.C]+ndAny <= haveC+haveCol-both
	}
	var pm uint8
	for c := range nd {
		if nd[c] > 0 {
			pm |= 1 << c
		}
	}
	for q := pm; q != 0; q = (q - 1) & pm {
		need := 0
		for c := range nd {
			if q&(1<<c) != 0 {
				need += nd[c]
			}
		}
		mx := 0
		for c := range nd {
			if q&(1<<c) != 0 && nd[c] > mx {
				mx = nd[c]
			}
		}
		have := 0
		for t := 0; t < pk.nt; t++ {
			ty := &pk.types[t]
			if ty.mask&q != 0 {
				// a tap producing n mana of one colour covers at most one
				// colour's demand, not the whole subset's
				cap := int(ty.n)
				if bits.OnesCount8(q) > 1 && cap > mx {
					cap = mx
				}
				have += int(ty.k) * cap
			}
		}
		if need > have {
			return false
		}
	}
	if pk.maxN > 1 {
		var seq [maxUnits]uint8
		n := 0
		for t := 0; t < pk.nt; t++ {
			for c := 0; c < int(pk.types[t].k); c++ {
				seq[n] = uint8(t)
				n++
			}
		}
		return pk.assignable(nd, &seq, 0, n)
	}
	return true
}

// consider ranks the current count vector (gorge pay/plan.go:195-334 keys
// 1..7; key 1, irreversible cost, is 10 per sacrificed source, 20 for a
// creature, 0 for tap-only sources).
func (pk *planner) consider() {
	var key [7]int
	size, prod, flex, pain := 0, 0, 0, 0
	var used [5]int
	for t := 0; t < pk.nt; t++ {
		ty := &pk.types[t]
		k := int(ty.k)
		key[0] += k * int(ty.irr)
		pain += k * int(ty.pain)
		if ty.creat {
			key[1] += k
		}
		size += k
		prod += k * (int(ty.n) + int(ty.xt))
		flex += k * int(ty.flex)
		if ty.nocol {
			continue // colours 0 (above): it takes nothing from the remainder
		}
		cm := ty.dmask // the colours colSrc counted it under (a withheld widened source: its own)
		if cm == 0 {
			cm = ty.mask // a painland type (painUnit sets no dmask)
		}
		cm |= ty.xmask
		for c := 0; c < 5; c++ {
			if cm&(1<<c) != 0 {
				used[c] += k
			}
		}
	}
	if pain > 0 && pain >= pk.life { // pay/search.go walk: a painless plan is never cut by the guard (life may be <= 0)
		return // the lethal guard (pay/search.go walk): never a plan
	}
	key[2] = size
	key[3] = prod - pk.needT // surplus (pool part is constant)
	key[4] = flex
	packed, remc := 0, 0
	for _, c := range pk.dOrder {
		r := pk.colSrc[c] - used[c]
		d := pk.demand[c]
		if r < d {
			d = r
		}
		packed = packed*pk.dBase + d
	}
	for c := 0; c < 5; c++ {
		if pk.colSrc[c]-used[c] > 0 {
			remc++
		}
	}
	key[5] = -packed
	key[6] = -remc
	if pk.found {
		cmp := 0
		for i := range key {
			if key[i] != pk.bestKey[i] {
				if key[i] < pk.bestKey[i] {
					cmp = -1
				} else {
					cmp = 1
				}
				break
			}
		}
		if cmp > 0 {
			return
		}
		if cmp == 0 {
			// a tie on keys 1-7: key 8 over every tied vector (a capped
			// list of them dropped the least sequence: pair 517)
			var kv [maxTypes]uint8
			for t := 0; t < pk.nt; t++ {
				kv[t] = pk.types[t].k
			}
			var cur, col, ty [maxUnits]uint8
			n := pk.lexLeast(&kv, &cur)
			pk.seqTypes(&kv, &cur, n, &ty)
			pk.seqCols(&ty, n, &col)
			if pk.seqLess(&cur, &col, &ty, n, &pk.bestSeq, &pk.bestCol, &pk.bestTy, pk.bestN) {
				pk.bestSeq, pk.bestCol, pk.bestTy, pk.bestN = cur, col, ty, n
			}
			return
		}
	}
	pk.found = true
	pk.bestKey = key
	var kv [maxTypes]uint8
	for t := 0; t < pk.nt; t++ {
		kv[t] = pk.types[t].k
	}
	pk.bestN = pk.lexLeast(&kv, &pk.bestSeq)
	pk.seqTypes(&kv, &pk.bestSeq, pk.bestN, &pk.bestTy)
	pk.seqCols(&pk.bestTy, pk.bestN, &pk.bestCol)
}

// seqCols is the colour each unit of seq makes when tapped in order: the
// smallest Produces vector that keeps the rest payable (pickColour, as
// the plan's taps below choose them).
func (pk *planner) seqCols(tys *[maxUnits]uint8, n int, col *[maxUnits]uint8) {
	rem := pk.need
	remAny := pk.needAny
	pk.fixedOffSeq(&rem, &remAny, tys, n)
	for i := 0; i < n; i++ {
		ty := &pk.types[tys[i]]
		c := pk.pickUnitColour(ty, &rem, remAny, tys, i+1, n)
		col[i] = uint8(c)
		if pk.anyColor && c != kop.C {
			remAny = max(remAny-int(ty.n), 0)
		} else if rem[c] > 0 {
			rem[c] = max(rem[c]-int(ty.n), 0)
		}
	}
}

// lexLeast builds, for count vector kv, the unit choice whose ObjID
// sequence in unit order is lexicographically least. Returns its length.
func (pk *planner) lexLeast(kv *[maxTypes]uint8, out *[maxUnits]uint8) int {
	var need [maxTypes]int
	total := 0
	for t := 0; t < pk.nt; t++ {
		need[t] += int(kv[t])
		total += int(kv[t])
		if pk.pairs && pk.types[t].alt {
			// a painland's coloured units are its {C} type's units
			// (painUnit registers each unit once, under the primary)
			p := int(pk.types[t].pair - 1)
			need[p] += need[t]
			need[t] = 0
		}
	}
	// rem: units of each type at positions >= pos (a running suffix count:
	// no per-position table to zero)
	var rem [maxTypes]uint8
	for i := 0; i < pk.nu; i++ {
		rem[pk.utyp[i]]++
	}
	n := 0
	pos := 0
	for n < total {
		bestU := -1
		cur := rem // the suffix after u, as u advances
		for u := pos; u < pk.nu; u++ {
			t := pk.utyp[u]
			cur[t]--
			if need[t] == 0 {
				continue
			}
			// feasible: after u, the remaining needs fit
			ok := true
			for tt := 0; tt < pk.nt; tt++ {
				r := need[tt]
				if tt == int(t) {
					r--
				}
				if r > int(cur[tt]) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			if bestU < 0 || pk.uobj[u] < pk.uobj[bestU] {
				bestU = u
			}
		}
		out[n] = uint8(bestU)
		n++
		need[pk.utyp[bestU]]--
		for u := pos; u <= bestU; u++ {
			rem[pk.utyp[u]]--
		}
		pos = bestU + 1
	}
	return n
}

// seqLess is gorge's key 8 (pay.CompareSteps) over two witnesses: step by
// step the source's ObjID, then its ability (a gated source's first ability
// is the smaller, whichever colour it makes), then the mana it produces as a
// WUBRGC vector (so a later colour in that order is the smaller: {U} before
// {W}, pair 8715), then the length.
func (pk *planner) seqLess(a, ac, at *[maxUnits]uint8, an int, b, bc, bt *[maxUnits]uint8, bn int) bool {
	for i := 0; i < an && i < bn; i++ {
		x, y := pk.uobj[a[i]], pk.uobj[b[i]]
		if x != y {
			return x < y
		}
		if aa, ba := pk.types[at[i]].alt, pk.types[bt[i]].alt; aa != ba {
			return ba // the source's census ability has the smaller index
		}
		if ac[i] != bc[i] {
			if f := pk.types[pk.utyp[a[i]]].first; f != 0 {
				if ac[i] == f-1 {
					return true
				}
				if bc[i] == f-1 {
					return false
				}
			}
			return ac[i] > bc[i]
		}
	}
	return an < bn
}

// seqTypes is the type each position of seq (lexLeast's choice for kv)
// takes: a unit's own type, or for a painland (a paired primary) its {C}
// type while kv still owes {C} units of that class, then the painful one
// (gorge materialize: each chosen unit the smallest alternative still owed,
// pay.CompareSteps' ability index order).
func (pk *planner) seqTypes(kv *[maxTypes]uint8, seq *[maxUnits]uint8, n int, out *[maxUnits]uint8) {
	left := *kv
	for i := 0; i < n; i++ {
		t := pk.utyp[seq[i]]
		if pk.pairs && pk.types[t].pair != 0 && !pk.types[t].alt && left[t] == 0 {
			t = pk.types[t].pair - 1
		}
		left[t]--
		out[i] = t
	}
}

// fixedOff takes the mana every chosen unit's fixed extra makes off the pips
// the choices must cover (a StatTapExtra's extra is the same whichever colour
// the unit picks, so it covers pips before any pick is made).
func (pk *planner) fixedOff(nd *[kop.NumManaKinds]int, ndAny *int) {
	for t := 0; t < pk.nt; t++ {
		ty := &pk.types[t]
		if ty.xt == 0 || ty.k == 0 {
			continue
		}
		pk.takeXv(nd, ndAny, ty, int(ty.k))
	}
}

// fixedOffSeq is fixedOff over the first n units of tys (a plan's sequence).
func (pk *planner) fixedOffSeq(nd *[kop.NumManaKinds]int, ndAny *int, tys *[maxUnits]uint8, n int) {
	if !pk.maxXt {
		return
	}
	for i := 0; i < n; i++ {
		if ty := &pk.types[tys[i]]; ty.xt != 0 {
			pk.takeXv(nd, ndAny, ty, 1)
		}
	}
}

func (pk *planner) takeXv(nd *[kop.NumManaKinds]int, ndAny *int, ty *utype, k int) {
	for c, x := range ty.xv {
		v := int(x) * k
		if v == 0 {
			continue
		}
		if pk.anyColor && c != kop.C {
			*ndAny = max(*ndAny-v, 0)
		} else {
			nd[c] = max(nd[c]-v, 0)
		}
	}
}
