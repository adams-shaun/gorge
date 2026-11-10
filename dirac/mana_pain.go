package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Pain mana (CR 605.1a, 605.3b): a mana ability whose rider is
// "CARDNAME deals N damage to you" -- the painlands' second ability ({T}:
// Add {W} or {U}. Adarkar Wastes deals 1 damage to you) and a pain-only
// source's sole one (Elves of Deep Shadow). gorge's payment planner prices
// such an alternative as last resort with Consequence{Damage: N}
// (pay.PaymentPlanAbilityTier via PaymentPlanDamageRider): phase 1 plans
// from the normal alternatives alone (a painland's {C}); only when no such
// plan exists does phase 2 add the painful ones, at an irreversible cost of
// 3 per point of damage (pay.ConsequenceCost), never a plan whose summed
// damage reaches the payer's life (the lethal guard, pay/search.go walk),
// each source's flexibility read over all its alternatives (Alt.FlexAll)
// and its colours over its normal ones only (Alt.Colours: none for a
// painland or a pain-only source). The kernel's planner models a painland
// in phase 2 as two unit types sharing the source (utype.pair): the {C}
// alternative and the painful colour choice; the witness hands a class's
// chosen units {C} first, then the colours (pay.CompareSteps: ability index,
// then Produces), and an activation of the painful alternative deals its
// damage at once (painDamage). The manual window's ability ask answers
// ability 0 (the painland's {C}; payWindowAnswer), and a pain-only source
// tapped there or in a failed ward payment's float deals its damage too.

// manaPain reads a mana ability's painland rider: its program is exactly
// "deals N damage to you" (kgen painRider).
func manaPain(a *kop.Ability) (uint8, bool) {
	if len(a.Prog) != 1 {
		return 0, false
	}
	in := &a.Prog[0]
	if in.Op != kop.OpDealDamage || in.Obj != kop.RefController || in.A.K != kop.VConst || in.A.N < 1 || in.A.N > 255 {
		return 0, false
	}
	return uint8(in.A.N), true
}

// manaLifePain reads a mana ability whose only extra cost is paying N
// life ({T}, Pay 1 life: Add one mana of any colour; Starting Town's second
// ability, kgen lifeTapMana): no program, the life its painN.
func manaLifePain(a *kop.Ability) (uint8, bool) {
	if len(a.Prog) != 0 || a.Cost.Flags != kop.CostTap|kop.CostPayLife || a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 || a.Cost.N < 1 {
		return 0, false
	}
	return a.Cost.N, true
}

// primaryNormalAb reports whether b is the normal alternative the kernel
// prices for a pain source: a bare {T} for one mana of {C}, one WUBRG
// colour, or a two-colour choice (kgen primaryMana). prepPainMana and
// lifePainLandAb share it, so a shape the planner prices is never a census
// gap.
func primaryNormalAb(b *kop.Ability) bool {
	if len(b.Prog) != 0 || b.Cost.Flags != kop.CostTap || b.Cost.Mana.MV() != 0 || b.Cost.Mana.X != 0 || max(b.ManaN, 1) != 1 {
		return false
	}
	c := uint8(b.ManaColors)
	return c == uint8(kop.ColC) || c != 0 && c&^0x1f == 0 && bits.OnesCount8(c) <= 2
}

// lifePainLandAb: a is the paid-life ability of a life-pain land of c: its
// two mana abilities are a normal alternative ({C}, one colour, or a
// two-colour choice: Starting Town, Haunted Screen) and a, {T} and N life
// for one or more colours. gorge's planner prices that cost as a last-resort
// choice (pay.PlanLastResortCostOK), not a census gap.
func lifePainLandAb(c *kop.CardDef, a *kop.Ability) bool {
	if _, ok := manaLifePain(a); !ok {
		return false
	}
	n := 0
	first := true
	for j := range c.Abilities {
		b := &c.Abilities[j]
		if b.Kind != kop.AbMana || b.AFlags&kop.AfLatent != 0 {
			continue
		}
		n++
		if n == 1 {
			first = primaryNormalAb(b)
		}
	}
	return n == 2 && first
}

// prepPainMana sets pc's pain alternative: the card's mana abilities are a
// plain {T} normal alternative ({C}, one WUBRG colour, or a two-colour
// choice; kgen primaryMana) and a painful {T} ability of one or more colours
// (kgen refuseMultiMana's painland and life shapes). It reports whether it
// did.
//
//go:noinline
func prepPainMana(c *kop.CardDef, pc *pcard) bool {
	n := 0
	var col kop.ColorMask
	var dmg uint8
	life := false
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		n++
		if n == 2 {
			d, ok := manaPain(a)
			if !ok {
				if d, ok = manaLifePain(a); !ok {
					return false
				}
				life = true
			}
			col, dmg = a.ManaColors, d
		}
	}
	// The normal alternative is the source's first ability (primaryNormalAb:
	// {C}, one colour, or a plain two-colour {T} choice beside its life
	// ability, Haunted Screen's {W} or {B}).
	if n != 2 || pc.manaAb < 0 || !primaryNormalAb(&c.Abilities[pc.manaAb]) {
		return false
	}
	pc.painCol, pc.painN, pc.painLife = col, dmg, life
	return true
}

// painDamage is source sl's painland rider: it deals n damage to its
// controller p as the mana ability resolves (CR 605.3b), outside any stack
// resolution: the damage replacements (prevention, doubling) apply without
// an order ask, the life loss and the DamageDone triggers as any damage's.
//
//go:noinline
func (s *State) painDamage(k *prep, sl Slot, p uint8, n int) {
	if DebugNoPainDamage {
		return
	}
	if s.manaPCAt(k, sl).painLife {
		s.loseLife(k, p, n) // a paid life cost, not damage (CR 119.4)
		return
	}
	code := PlayerTarget0 + p
	// No CR 616.1 order ask here, unlike the dealHits batch and the other
	// direct replDamage sites (repl_plus.go): the mana step is not a
	// program step, so a parked event has no op whose re-execution would deal
	// it — the only park in this flow is the sac mana-hold (mana.go), which
	// re-runs the whole mana step after the production joined the pool, and
	// the out-of-program park route would ask only before priority, later
	// than gorge's ask point (inside the payment, engine/rules/
	// replacement.go's context-free collection), with damageTo's tail instead
	// of this rider's (infect, markDealt, fireDamage with the land). gorge
	// does ask there, so two or more pure plus-N replacements matching the
	// rider cap StallReplPlus loudly (replPlus): safe-side; the pool cannot
	// reach two candidates for a land's self-damage today.
	if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbProtAll|rbProtCol|rbPlusDmg|rbShield|rbPrevCol|rbPrevMill) != 0 {
		if n = s.replDamage(k, p, sl, s.cardAt(sl), code, false, n); n <= 0 {
			return
		}
	}
	s.loseLife(k, p, n)
	if k.gen != nil && k.gen.anyPermMark {
		s.markDealt(k, sl, false) // filter_permmarks.go
	}
	if s.subs[kop.TrigDamageDone].any() || k.gen != nil && (s.subs[kop.TrigDamageDealtOnce].any() || k.gen.graveDmg) {
		s.fireDamage(k, sl, int16(n), true, false, code)
	}
}

// painPlanLife is the payer's life for the planner's lethal guard: a plan
// whose summed pain reaches it is never a plan.
func (s *State) painPlanLife(p uint8) int { return int(s.pl[p].life) }

// painFlexAll is a pain source's flexibility over every alternative: gorge's
// phase-2 key 5, Alt.FlexAll, counts each mana type any alternative makes,
// the painful colours included whether or not the lethal guard admits them.
// A painland whose painful alternative painUnit withheld (its damage alone
// would reach the payer's life) is then its normal unit, and that unit must
// still read the whole source: if it read only its normal alternative's
// colours, it would fold into an otherwise identical plain source's type and
// take that source's flex (plan's type match). A pain-only source has no
// normal unit, so it keeps its painful colours alone.
func painFlexAll(pc *pcard, xc, xp [kop.NumManaKinds]uint8) uint8 {
	all := uint8(pc.painCol) & 0x3f
	if !pc.painOnly {
		all |= uint8(pc.manaCol) & 0x3f
	}
	for c := range xc {
		if xc[c] > 0 {
			all |= 1 << c
		}
		if xp[c] > 0 {
			all |= 1 << c
		}
	}
	return uint8(bits.OnesCount8(all))
}

// painAvailMask is a pain source's production mask for the offer gate
// (availFor): a painland makes its normal ability's mana and, while one
// point of its damage would not be lethal, its colours; a pain-only source
// only the latter. The summed guard over several painful units is the
// planner's (the option walk counts each unit alone).
func (s *State) painAvailMask(pc *pcard, p uint8) uint8 {
	prim := uint8(pc.manaCol)
	if s.g.availX&availWin != 0 {
		// an unless / ward window's affordability: gorge's shared census
		// (windowManaUnitsWith) lists a painland by its normal ability alone
		// and a pain-only source by its colour, with no lethal guard
		if pc.painOnly {
			return uint8(pc.painCol) & 0x3f
		}
		return prim
	}
	m := uint8(0)
	if !pc.painOnly {
		m = prim
	}
	if s.g.availX&availWinCol != 0 || int(pc.painN) < int(s.pl[p].life) {
		m |= uint8(pc.painCol) & 0x3f
	}
	return m
}

// painUnit registers pain source sl (pcs index i) in a last-resort phase:
// a painland as its normal-ability type and its painful colour type, paired
// (they share the source); a pain-only source as its painful type alone. An
// alternative whose damage alone reaches the payer's life is withheld
// (pay.PlanLastResortChoices' one-step guard): a painland is then its normal
// unit (reported false: the caller adds it as an ordinary source) and a
// pain-only source no unit. The unit sits where gorge's census lists the
// source: in the shared census (pass 0) when the production is
// deterministic, appended after the fixed-production ones (pass 1) when it
// is choice-shaped (paymentPlanManaUnitsOnlyCompute); want is that pass,
// the caller's census-shape choice.
// It reports whether the caller has nothing left to do for sl.
//
//go:noinline
func (s *State) painUnit(k *prep, sl Slot, pc *pcard, pass int, pk *planner, i int, want int) bool {
	if int(pc.painN) >= pk.life {
		return pc.painOnly
	}
	if pass != want {
		return true
	}
	if pk.nu >= maxUnits {
		return true
	}
	cr := s.isCreature(k, sl)
	prim := uint8(pc.manaCol) & 0x3f
	pm := uint8(pc.painCol) & 0x3f // a pain-only {C}{C} source (Ancient Tomb) makes colourless
	// the tap's extra: the normal ability's carriers' on its type, the
	// painful one's non-restricted carriers' on the painful type
	// (srcExtraPain)
	xc, xp, ok := s.srcExtraPain(k, sl, pc)
	if !ok {
		return true
	}
	var xmc, xmp, xtc, xtp uint8
	for c := range xc {
		if xc[c] > 0 {
			xmc |= 1 << c
			xtc += xc[c]
		}
		if xp[c] > 0 {
			xmp |= 1 << c
			xtp += xp[c]
		}
	}
	fl := painFlexAll(pc, xc, xp) // FlexAll: the normal mana and the colours, and the extra's
	add := func(mask, pain uint8, alt bool) int {
		if pk.nt == len(pk.types) {
			return -1
		}
		t := pk.nt
		pk.nt++
		n := uint8(1)
		if pain != 0 && pc.painOnly {
			n = max(pc.manaN, 1) // Ancient Tomb's {C}{C} in one activation
		}
		pk.types[t] = utype{mask: mask, dmask: mask, n: n, creat: cr, flex: fl, pain: pain, alt: alt, irr: 3 * pain, nocol: pain != 0}
		if pain != 0 {
			pk.types[t].xv, pk.types[t].xmask, pk.types[t].xt = xp, xmp, xtp
		} else {
			pk.types[t].xv, pk.types[t].xmask, pk.types[t].xt = xc, xmc, xtc
		}
		if pk.types[t].xt != 0 {
			pk.maxXt = true
		}
		if m := int(n) + int(pk.types[t].xt); pk.maxN < m {
			pk.maxN = m
		}
		return t
	}
	// a painful type: its colours, the source kind and creature-ness name it
	// (every pool card with painful colours has its own pair of colours)
	findP := func() int {
		for t := 0; t < pk.nt; t++ {
			ty := &pk.types[t]
			if ty.pain != 0 && ty.mask == pm && ty.creat == cr && ty.alt == !pc.painOnly && ty.xv == xp {
				return t
			}
		}
		return add(pm, pc.painN, !pc.painOnly)
	}
	// the normal-ability type first: the search counts it before its pair
	// (dfs's shared-avail cap reads the primary's count)
	tc := -1
	if !pc.painOnly {
		for t := 0; t < pk.nt && tc < 0; t++ {
			ty := &pk.types[t]
			if ty.pair != 0 && !ty.alt && ty.mask == prim && ty.creat == cr && pk.types[ty.pair-1].mask == pm && ty.xv == xc && pk.types[ty.pair-1].xv == xp {
				tc = t
			}
		}
		if tc < 0 {
			if tc = add(prim, 0, false); tc < 0 {
				return true
			}
		}
	}
	tp := findP()
	if tp < 0 {
		return true
	}
	ti := tp
	if tc >= 0 {
		pk.types[tc].pair, pk.types[tp].pair = uint8(tp)+1, uint8(tc)+1
		pk.types[tc].avail++
		pk.pairs = true
		ti = tc
	}
	pk.types[tp].avail++
	pk.units[pk.nu] = sl
	if pk.bfl {
		pk.uobj[pk.nu] = Obj(i)
	} else {
		pk.uobj[pk.nu] = s.perms[sl].obj
	}
	pk.utyp[pk.nu] = uint8(ti)
	pk.nu++
	return true
}

// DebugNoPainDamage drops the painful alternatives' damage. The pain
// lockstep's mutation test flips it to prove the pool exercises them.
var DebugNoPainDamage bool

// painAvail is the offer gate's painful units when their summed damage
// could reach the payer's life: the planner's lethal guard then bounds how
// many may make a colour, which a per-unit Hall count cannot express.
// Groups are the distinct (colours, pain-only) shapes of one-damage sources
// (every pool painland and Elves of Deep Shadow deal 1); a painland's units
// sit in the base avail as {C} sources, a pain-only source's not at all,
// and affordsPlain tries each split of the budget over the groups.
type painAvail struct {
	on     bool
	n      uint8
	only   uint8    // bit g: group g is pain-only (no normal alternative)
	budget uint8    // painful activations a plan may take: life - 1
	mask   [6]uint8 // the group's painful colours
	prim   [6]uint8 // the group's normal alternative's colours (a painland's {C}, Haunted Screen's {W}{B})
	cnt    [6]uint8
	ec     [6]uint8 // the extra {C} the group's normal ability brings (a modal source)
}

// painTight rebuilds a's painful units as a painAvail when every painful
// colour used at once would deal at least the payer's life (the summed
// lethal guard binds); otherwise a stands as availFor built it (each unit
// with its colours: the per-unit guard is painAvailMask's). The gaps walk
// (availGaps: the scripted prefix searches' relaxed proof) keeps the guard
// too: the planner that prices the play afterwards enforces it, and a gaps
// walk counting every Elf's {B} alone offered a play (Balustrade Spy at 2
// life beside three Elves of Deep Shadow and a Wall of Roots) the cast's
// payment then could not make: "autopay found no plan".
//
//go:noinline
func (s *State) painTight(k *prep, p uint8, a *avail) {
	life := int(s.pl[p].life)
	if life <= 1 || s.g.availX&(availAny|availWin|availWinCol) != 0 || k.gen != nil && k.gen.anyContGrant {
		return // at 1 life availFor already withheld every painful colour
	}
	modal := a.nModal != 0 // a source with a {C}-only extra: its groups split even when the budget does not bind
	var pa painAvail
	sum := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCAt(k, sl)
		if pc.painCol == 0 || !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		if pc.painN != 1 {
			s.painTightBail(modal)
			return // a heavier rider: keep the per-unit count
		}
		sum++
		pm := uint8(pc.painCol) & 0x1f
		prim := uint8(pc.manaCol) & 0x3f
		var ob uint8
		if pc.painOnly {
			ob = 1
			prim = 0
		}
		var ec uint8
		if modal && s.painAvailMask(pc, p)&^(1<<kop.C) != 0 {
			ec = s.painExtraC(k, sl, pc)
		}
		g := uint8(0)
		for ; g < pa.n; g++ {
			if pa.mask[g] == pm && pa.prim[g] == prim && pa.only>>g&1 == ob && pa.ec[g] == ec {
				break
			}
		}
		if g == pa.n {
			if int(pa.n) == len(pa.mask) {
				s.painTightBail(modal)
				return // an exotic board: keep the per-unit count
			}
			pa.mask[g] = pm
			pa.prim[g] = prim
			pa.only |= ob << g
			pa.ec[g] = ec
			pa.n++
		}
		pa.cnt[g]++
	}
	if sum < life && !modal {
		return
	}
	pa.budget = uint8(min(life-1, 255))
	// take the painful colours back out of a: each painland unit is its
	// normal ability's, each pain-only unit none
	for g := uint8(0); g < pa.n; g++ {
		only := pa.only>>g&1 != 0
		prim := pa.prim[g]
		full := pa.mask[g]
		if !only {
			full |= prim
		}
		a.remove(full, pa.cnt[g])
		if !only {
			a.addUnits(prim, pa.cnt[g])
			if pa.ec[g] != 0 {
				a.addUnits(1<<kop.C, pa.ec[g]*pa.cnt[g]) // the normal ability's own extra
			}
		}
	}
	pa.on = true
	a.pain = pa
}

// painTightBail: painTight leaves the units as availFor built them; a modal
// source cannot be left that way.
func (s *State) painTightBail(modal bool) {
	if modal {
		s.capFull(StallTapExtra)
	}
}

// painAffords is affordsPlain under a painful budget: some split of the
// budget over the groups (x[g] units of group g making a colour, at most
// the budget in all) leaves a Hall-feasible avail. A board with too many
// splits answers no past the cap.
//
//go:noinline
func (a *avail) painAffords(cost *kop.ManaCost, pipMask uint8, extra int) bool {
	pa := &a.pain
	var x [6]uint8
	tries := 0
	for {
		used := 0
		for g := uint8(0); g < pa.n; g++ {
			used += int(x[g])
		}
		if used <= int(pa.budget) {
			t := *a
			t.pain.on = false
			for g := uint8(0); g < pa.n; g++ {
				if x[g] == 0 {
					continue
				}
				if pa.only>>g&1 != 0 {
					t.addUnits(pa.mask[g], x[g])
				} else {
					// painTight holds the group's normal units under prim
					// and its modal extra's under {C}, not both under prim.
					t.remove(pa.prim[g], x[g])
					if pa.ec[g] != 0 {
						t.remove(1<<kop.C, x[g]*pa.ec[g])
					}
					t.addUnits(pa.mask[g]|pa.prim[g], x[g])
				}
			}
			if t.hallPlain(cost, pipMask, extra) {
				return true
			}
		}
		if tries++; tries > 4096 {
			return false
		}
		g := uint8(0)
		for ; g < pa.n; g++ {
			if x[g] < pa.cnt[g] {
				x[g]++
				break
			}
			x[g] = 0
		}
		if g == pa.n {
			return false
		}
	}
}

// availWin is the availFor mode of an unless / ward window's affordability
// (availWindowFor).
const availWin uint8 = 32

// availWindowFor is availFor for gorge's unless / ward payability: the
// shared census's units (painAvailMask), not the planner's.
func (s *State) availWindowFor(k *prep, p uint8, a *avail) {
	if !k.painMana {
		s.availFor(k, p, a)
		return
	}
	x := s.g.availX
	s.g.availX |= availWin
	s.availFor(k, p, a)
	s.g.availX = x
}

// availWinCol is the availFor mode of a manual cast window's affordability
// (a hybrid pip's announcement: gorge's planner refuses the cost, the
// window pays it): every painful colour counts, with no lethal guard (the
// window's answer taps a painland for a colour whatever the life).
const availWinCol uint8 = 64

// availCastWindowFor is availFor under availWinCol.
func (s *State) availCastWindowFor(k *prep, p uint8, a *avail) {
	if !k.painMana {
		s.availFor(k, p, a)
		return
	}
	x := s.g.availX
	s.g.availX |= availWinCol
	s.availFor(k, p, a)
	s.g.availX = x
}

// loweredWindowOK pays a lowered cast through its window: the window's answers
// can spend a source on a colour the cost does not owe (a painland or a gain
// land on its lowest), leaving the cost stranded with no useful source left;
// gorge then aborts the cast (CR 733.1), keeping the taps, the floated mana and
// the damage. It reports false after reversing. On true the cost is paid and
// left as it was, for the cast's recorded spend, and the restricted mana held
// out of it floats in the pool.
//
//go:noinline
func (s *State) loweredWindowOK(k *prep, p uint8, cost *kop.ManaCost, anyType, anyColor bool) bool {
	ok, held := s.windowTapUseful(k, p, cost, cost, !anyType && !anyColor, anyColor, s.bld().pipN == 0, false)
	if ok && s.winPaused() {
		for c := range held {
			s.pl[p].pool[c] += held[c]
		}
		return true // the pick is posed; the cast pays when the window ends (mana_window.go)
	}
	if !ok {
		for c := range held {
			s.pl[p].pool[c] += held[c]
		}
		s.reverseCast(k)
		return false
	}
	if s.rbAny(p) {
		s.payRestrRider(k, p, cost, 0, s.bld().e.obj, anyColor)
	} else {
		s.payFromPoolRider(k, p, cost, 0, anyColor)
	}
	for c := range held {
		s.pl[p].pool[c] += held[c]
	}
	return true
}

// availExt is availFor's cold extension: a Phyrexian pip's life budget and
// the painful units' (painTight).
//
//go:noinline
func (s *State) availExt(k *prep, p uint8, a *avail) {
	a.life = s.pl[p].life
	if k.painMana {
		s.painTight(k, p, a)
	}
}

// painAnyOwed is a cast window's answer for a {C} source whose other ability makes
// any colour for a life or damage rider (Starting Town): gorge's ability ask lists
// an ability that makes a colour a fixed pip still owes first (owedAbilitiesFirst),
// so while the pool is short of a {W}..{G} pip the any-colour ability is activated
// and its colour ask answers the lowest owed colour (owedColoursFirst), even when
// the {C} could pay the generic part.
func (s *State) painAnyOwed(p uint8, useful *kop.ManaCost, pc *pcard) (int, bool) {
	if pc.painCol != kop.ColAny {
		return 0, false
	}
	for c := 0; c < 5; c++ {
		if useful.Pips[c] > s.pl[p].pool[c] {
			return c, true
		}
	}
	return 0, false
}
