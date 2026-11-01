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

// prepPainMana sets pc's pain alternative for a painland: the card's mana
// abilities are a plain {T}: Add {C} and a painful {T} ability of one or two
// colours (kgen refuseMultiMana's painland shape). It reports whether it did.
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
	if n != 2 || pc.manaCol != kop.ColC || len(c.Abilities[pc.manaAb].Prog) != 0 {
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
	if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbProtAll|rbProtCol|rbPlusDmg|rbShield|rbPrevCol) != 0 {
		if n = s.replDamage(k, p, sl, s.cardAt(sl), code, false, n); n <= 0 {
			return
		}
	}
	s.loseLife(k, p, n)
	if k.gen != nil && k.gen.anyPermMark {
		s.markDealt(k, sl, false) // filter_permmarks.go
	}
	if s.subs[kop.TrigDamageDone].any() || k.gen != nil && s.subs[kop.TrigDamageDealtOnce].any() {
		s.fireDamage(k, sl, int16(n), true, false, code)
	}
}

// painPlanLife is the payer's life for the planner's lethal guard: a plan
// whose summed pain reaches it is never a plan.
func (s *State) painPlanLife(p uint8) int { return int(s.pl[p].life) }

// painAvailMask is a pain source's production mask for the offer gate
// (availFor): a painland makes {C} and, while one point of its damage would
// not be lethal, its colours; a pain-only source only the latter. The
// summed guard over several painful units is the planner's (the option walk
// counts each unit alone).
func (s *State) painAvailMask(pc *pcard, p uint8) uint8 {
	if s.g.availX&availWin != 0 {
		// an unless / ward window's affordability: gorge's shared census
		// (windowManaUnitsWith) lists a painland by its {C} ability alone and
		// a pain-only source by its colour, with no lethal guard
		if pc.painOnly {
			return uint8(pc.painCol) & 0x1f
		}
		return 1 << kop.C
	}
	m := uint8(0)
	if !pc.painOnly {
		m = 1 << kop.C
	}
	if s.g.availX&availWinCol != 0 || int(pc.painN) < int(s.pl[p].life) {
		m |= uint8(pc.painCol) & 0x1f
	}
	return m
}

// painUnit registers pain source sl (pcs index i) in a last-resort phase:
// a painland as its {C} type and its painful colour type, paired (they
// share the source); a pain-only source as its painful type alone. An
// alternative whose damage alone reaches the payer's life is withheld
// (pay.PlanLastResortChoices' one-step guard): a painland is then its {C}
// unit (reported false: the caller adds it as an ordinary source) and a
// pain-only source no unit. The units sit with the fixed-production ones
// (pass 0: gorge's shared census lists the source by its fixed ability).
// It reports whether the caller has nothing left to do for sl.
//
//go:noinline
func (s *State) painUnit(k *prep, sl Slot, pc *pcard, pass int, pk *planner, i int) bool {
	if int(pc.painN) >= pk.life {
		return pc.painOnly
	}
	if pass != 0 {
		return true
	}
	if pk.nu >= maxUnits {
		return true
	}
	cr := s.isCreature(k, sl)
	pm := uint8(pc.painCol) & 0x1f
	fl := uint8(bits.OnesCount8(pm))
	if !pc.painOnly {
		fl++ // FlexAll: {C} and the colours
	}
	add := func(mask, pain uint8, alt bool) int {
		if pk.nt == len(pk.types) {
			return -1
		}
		t := pk.nt
		pk.nt++
		pk.types[t] = utype{mask: mask, dmask: mask, n: 1, creat: cr, flex: fl, pain: pain, alt: alt, irr: 3 * pain, nocol: pain != 0}
		if pk.maxN < 1 {
			pk.maxN = 1
		}
		return t
	}
	// a painful type: its colours, the source kind and creature-ness name it
	// (every pool card with painful colours has its own pair of colours)
	findP := func() int {
		for t := 0; t < pk.nt; t++ {
			ty := &pk.types[t]
			if ty.pain != 0 && ty.mask == pm && ty.creat == cr && ty.alt == !pc.painOnly {
				return t
			}
		}
		return add(pm, pc.painN, !pc.painOnly)
	}
	// the primary {C} type first: the search counts it before its pair
	// (dfs's shared-avail cap reads the primary's count)
	tc := -1
	if !pc.painOnly {
		for t := 0; t < pk.nt && tc < 0; t++ {
			ty := &pk.types[t]
			if ty.pair != 0 && !ty.alt && ty.creat == cr && pk.types[ty.pair-1].mask == pm {
				tc = t
			}
		}
		if tc < 0 {
			if tc = add(1<<kop.C, 0, false); tc < 0 {
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
	only   uint8    // bit g: group g is pain-only (no {C} alternative)
	budget uint8    // painful activations a plan may take: life - 1
	mask   [6]uint8 // the group's painful colours
	cnt    [6]uint8
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
	var pa painAvail
	sum := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCAt(k, sl)
		if pc.painCol == 0 || !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		if pc.painN != 1 {
			return // a heavier rider: keep the per-unit count
		}
		sum++
		pm := uint8(pc.painCol) & 0x1f
		var ob uint8
		if pc.painOnly {
			ob = 1
		}
		g := uint8(0)
		for ; g < pa.n; g++ {
			if pa.mask[g] == pm && pa.only>>g&1 == ob {
				break
			}
		}
		if g == pa.n {
			if int(pa.n) == len(pa.mask) {
				return // an exotic board: keep the per-unit count
			}
			pa.mask[g] = pm
			pa.only |= ob << g
			pa.n++
		}
		pa.cnt[g]++
	}
	if sum < life {
		return
	}
	pa.budget = uint8(min(life-1, 255))
	// take the painful colours back out of a: each painland unit is a {C}
	// unit, each pain-only unit none
	for g := uint8(0); g < pa.n; g++ {
		only := pa.only>>g&1 != 0
		full := pa.mask[g]
		if !only {
			full |= 1 << kop.C
		}
		a.remove(full, pa.cnt[g])
		if !only {
			a.addUnits(1<<kop.C, pa.cnt[g])
		}
	}
	pa.on = true
	a.pain = pa
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
					t.remove(1<<kop.C, x[g])
					t.addUnits(pa.mask[g]|1<<kop.C, x[g])
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
	if !ok {
		for c := range held {
			s.pl[p].pool[c] += held[c]
		}
		s.reverseCast(k)
		return false
	}
	if s.pl[p].poolR != 0 {
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
