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

// prepPainSource reports whether c's mana production pains its controller: a
// self-Taps line (prepSelfTapsPain, mana_tapsdefer.go), a pain-only or a
// sole-life source (below), or a printed reflected ability whose cost pays
// life (reflLifePain, Horizon of Progress). prepare's painMana flag reads it.
func prepPainSource(c *kop.CardDef, pc *pcard) bool {
	return prepSelfTapsPain(c, pc) || prepPainOnlyLife(c, pc) || prepSoleLifeMana(c, pc) || reflLifePain(c)
}

// reflLifePain reports whether c has a printed reflected mana ability whose
// cost pays life (Horizon of Progress): its lone imprint view is the kernel's
// pain-only unit (mana_imprint.go prepManaImprint's view fields), so the
// board prices pain like any other pain source's.
func reflLifePain(c *kop.CardDef) bool {
	for j := range c.Abilities {
		if a := &c.Abilities[j]; reflLifeManaAb(a) {
			return true
		}
	}
	return false
}

// abPainOf is a mana ability's own activation pain: the life its cost pays
// (lifeCostOwed) PLUS the damage its rider deals (manaPain) — gorge's
// lethal guard (pay/search.go) sums both when it cuts a plan, never one or
// the other. The census's lethal read (availPotentialFor's cenPainOf) prices
// an activation whose own pain reaches the payer's life at no production
// (censusPainDrop's cenMaskAdd), while the fold keeps it.
func abPainOf(a *kop.Ability) int {
	if a == nil {
		return 0
	}
	pain := 0
	if a.Cost.Flags&kop.CostPayLife != 0 {
		pain += int(lifeCostOwed(&a.Cost))
	}
	if dmg, ok := manaPain(a); ok {
		pain += int(dmg)
	}
	return pain
}

// primaryTapAb reports whether a is a bare {T} ability for one mana of any
// colour, {C}, or a colour set (a plain mana ability with no rider, cost
// part, restriction or flag): the shape a self Pain-class Taps line prices
// (prepSelfTapsPain; kgen selfPainPlainMana).
func primaryTapAb(a *kop.Ability) bool {
	return a.Kind == kop.AbMana && a.AFlags == 0 && a.ManaRestrict == 0 && a.ManaRestrictAb == 0 && len(a.Prog) == 0 &&
		a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && max(a.ManaN, 1) == 1 && a.ManaColors != 0
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
// choice (pay.PlanLastResortCostOK), not a census gap. A lone reflected one
// whose colours are the reflected read is the same last-resort tier (Horizon
// of Progress, kop.ReflLifeManaAb; the views' pain, mana_reflected.go
// prepManaRefl).
func lifePainLandAb(c *kop.CardDef, a *kop.Ability) bool {
	if _, ok := manaLifePain(a); !ok {
		return false
	}
	if reflLifeManaAb(a) {
		return true
	}
	n, self := 0, -1
	first := true
	for j := range c.Abilities {
		b := &c.Abilities[j]
		if b == a {
			self = j
		}
		if b.Kind != kop.AbMana || b.AFlags&kop.AfLatent != 0 {
			continue
		}
		n++
		if n == 1 {
			first = primaryNormalAb(b)
		}
	}
	// (n == 1: Mana Confluence's sole paid-life ability, no normal alternative)
	return n == 2 && first || n == 1 && self >= 0 && kop.SoleLifeManaAb(c, self)
}

// reflLifeManaAb reports whether a is a printed reflected mana ability whose
// cost is {T} plus paying N life (Horizon of Progress, kop.ReflLifeManaAb).
func reflLifeManaAb(a *kop.Ability) bool {
	return kop.ReflLifeManaAb(a)
}

// prepPainOnlyLife prices a card whose only mana ability is "{T}, Pay N life:
// Add one mana" (Mana Confluence, kop.SoleLifeManaAb) as a pain-only source
// paying life: gorge's last-resort Consequence{Life: N} with no normal
// alternative, a pain-only source's tier (Elves of Deep Shadow) with
// Starting Town's life cost (painLife). It reports whether it did.
func prepPainOnlyLife(c *kop.CardDef, pc *pcard) bool {
	if pc.manaAb < 0 || pc.painCol != 0 || !kop.SoleLifeManaAb(c, int(pc.manaAb)) {
		return false
	}
	a := &c.Abilities[pc.manaAb]
	pc.painCol, pc.painN, pc.painOnly, pc.painLife = a.ManaColors, a.Cost.N, true, true
	pc.manaTap = true // its {T} is an ordinary one: the life is the painful alternative's cost
	return true
}

// prepSoleLifeMana prices a card whose only mana ability is a life-only one
// ("Pay N life: Add <mana>", no {T}: Treasonous Ogre, kop.SoleGapLifeManaAb)
// as the same pain-only source: gorge's last-resort Consequence{Life: N}
// (pay.PaymentPlanShapeTierOf prices a life cost with no {T} the same tier),
// so its unit joins the plain census and the planner's phase 2. Its cost taps
// nothing, so manaTap stays false: the shape test lifeOnlyPain (painLife with
// no tap) identifies it, and the source prices usable tapped and summoning
// sick alike (CR 605.1a; the census extends it tapped or not, mana_sacsource.go).
// It reports whether it did.
func prepSoleLifeMana(c *kop.CardDef, pc *pcard) bool {
	if pc.manaAb < 0 || pc.painCol != 0 || !kop.SoleGapLifeManaAb(c, int(pc.manaAb)) {
		return false
	}
	a := &c.Abilities[pc.manaAb]
	pc.painCol, pc.painN, pc.painOnly, pc.painLife = a.ManaColors, a.Cost.N, true, true
	return true
}

// lifeOnlyPain reports whether pc is prepSoleLifeMana's pain unit: a painLife
// one with no tap, the one pain shape whose activation taps nothing. Every
// other painLife class (Mana Confluence, a painland, a grant view, a
// self-Taps rider) carries manaTap.
func lifeOnlyPain(pc *pcard) bool {
	return pc.painLife && pc.painCol != 0 && !pc.manaTap && !pc.manaSac && !pc.manaSacOnly
}

// windowNoTapPain: the manual window's view pc is a life-only pain ability
// (Treasonous Ogre, prepSoleLifeMana): the walk pays its life where a tap is
// skipped. windowDry's probe restores pools but not life, so its plain census
// cannot model the cost and stops unknown at the source — the treatment a
// sacrifice-only source gets (windowSacOnly), with the real walk's pain arm
// paying the life (mana_window.go).
func windowNoTapPain(k *prep, pc *pcard) bool {
	return k.painMana && lifeOnlyPain(pc)
}

// windowColPain is windowColOwed's pain arm: a painful source (k.painMana,
// pc.painCol, not pain-only) on an ask that does owe a colour (an any-colour
// ask is answered above it). The colours an owed pip reaches through either
// the painful ability or a normal one beside it have answered above
// (owedColoursFirst); what is left is the lowest colour the ask still lists.
func (s *State) windowColPain(k *prep, p uint8, cost *kop.ManaCost, pc *pcard, mask uint8, noOwed bool) int {
	if prim := uint8(pc.manaCol) & 0x1f; prim != 0 {
		// A two-colour normal ability beside the painful one (Haunted
		// Screen: {W} or {B}, then {G}, {U} or {R} for 1 life): the ask
		// lists both abilities' colours, so an owed colour either
		// ability makes has answered above (owedColoursFirst) and
		// otherwise the answer is the lowest listed colour, mask's (W),
		// not the painful ability's lowest for its own sake
		// (payexec.WindowAnswer, TestPayWindowTapsLowestColour). A
		// noOwed ask (a ward/unless window) owes nothing at all, so it
		// takes the same lowest colour and pays no life. A {C} normal
		// ability (a painland, Starting Town) keeps the measured answer
		// below.
		return bits.TrailingZeros8(mask)
	}
	// a painland: gorge's "Choose a mana ability" ask carries no
	// payment, so the answer is its lowest colour option whatever
	// the window owes (mana_pain.go)
	col := bits.TrailingZeros8(uint8(pc.painCol))
	if pc.painLife && bits.OnesCount8(uint8(pc.painCol)&0x1f) == 1 && uint8(pc.painCol)&^0x1f == 0 {
		// a life-cost ability of one fixed colour (Ramunap Ruins' {R})
		// lists no mana symbol, like Starting Town's, so the window's
		// answer takes the first listed ability, the plain {C} (CR 605.3b)
		col = kop.C
	}
	if pc.painCol == kop.ColAny {
		// Starting Town: the any-colour ability lists no symbol. A fixed
		// pip still owed makes the ability ask take it (owedAbilitiesFirst);
		// the colour ask then answers the owed pip when the demand is
		// decided, and otherwise offers every colour, where the payment
		// window's bot takes the first listed one (W), not an owed pip.
		demand := s.windowDemand(k, p, cost)
		col = kop.C
		if noOwed && !pc.painLife {
			// an unless / ward window's ability ask is in printed order
			// (owedAbilitiesFirst reorders a cast's only): the plain {C}
			// ability answers it, Grand Coliseum beside an owed {G}. A
			// pay-life any-colour ability (Starting Town) is not this class:
			// its 1 life is a real cost (CR 118.3) gorge's window does pay,
			// so its ask keeps the measured answer below.
			demand = 0
		}
		if m := uint8(pc.painCol) & 0x1f & demand; m != 0 {
			col = bits.TrailingZeros8(uint8(pc.painCol))
			if _, decided := s.windowUseful(k, p, cost); decided {
				col = bits.TrailingZeros8(m)
			}
		}
	}
	return col
}

// canLifeOnlyMana is the census and planner admission of sl's life-only pain
// unit: the ability's cost taps nothing and needs no untapped, unsick source
// (the census extends it tapped or not, mana_sacsource.go), only an ability
// the board still gives p and a life total that pays the fee -- gorge's
// planner withholds a last-resort alternative whose pain reaches the payer's
// life (pay.PlanLastResortChoices, CR 119.4).
func (s *State) canLifeOnlyMana(k *prep, sl Slot, p uint8) bool {
	pc := s.manaPCAt(k, sl)
	if !lifeOnlyPain(pc) || s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		return false
	}
	if k.gen != nil && (s.genLost(k, sl) || k.gen.anyCantAct && s.actForbidden(k, sl, true)) {
		return false
	}
	return int(s.pl[p].life) > int(pc.painN)
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

// tapPain deals the pain rider of source sl's mana ability that made col,
// tapped by a filter prefix step (mana_filter.go filtDo): a pain-only
// source's (Cabal Pit) whatever the colour, a painland's for a colour its
// normal alternative cannot make, as the manual window does
// (payWindowAnswer).
func (s *State) tapPain(k *prep, p uint8, sl Slot, col int) {
	if pc := s.manaPCGen(k, sl); k.painMana && pc.painCol != 0 && (pc.painOnly || uint8(pc.manaCol)&(1<<col) == 0) {
		s.painDamage(k, sl, p, int(pc.painN))
		if k.gen != nil && k.gen.uses.has(useStatRdr) {
			// A prefix step is a decision gorge answers one activation at a
			// time, and its boundary runs the state-based check (CR 117.3c,
			// 704.3): damage a static redirect (Pariah's Shield) landed on a
			// creature kills it before the next step, so a filter that took
			// the redirect's damage is gone when the search reaches for it.
			s.sba(k)
		}
	}
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
	if k.gen != nil && k.gen.anyTapsPain && k.gen.flags[s.cardAt(sl)]&gfTapsPain != 0 {
		if _, ok := selfTapsPainN(&k.t.Cards[s.cardAt(sl)]); ok {
			// the pain is the source's own Taps line (City of Brass,
			// prepSelfTapsPain), a real trigger the tap queues and the stack
			// resolves: dealing it here too would count it twice
			return
		}
	}
	if s.manaPCAt(k, sl).painLife {
		s.loseLife(k, p, n) // a paid life cost, not damage (CR 119.4)
		return
	}
	code := PlayerCode(p)
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
	if k.gen != nil && (k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbProtAll|rbProtCol|rbPlusDmg|rbShield|rbPrevCol|rbPrevMill) != 0 ||
		k.gen.anyRepl2&(rb2DmgMinus|rb2DmgSet) != 0) {
		if n = s.replDamage(k, p, sl, s.cardAt(sl), code, false, n); n <= 0 {
			return
		}
	}
	s.noteDmgTaken(p, n)       // kw_bloodthirst.go
	s.notePlDamagedNC(k, code) // value_oppsdamaged.go
	// the damage life floors apply (loseLifeDmg, IsDamage$ True): a ReplLifeReduced
	// line (Fortune Thief, Ali from Cairo, Worship) clamps the rider's loss like any
	// damage's — gorge's foldDamage folds it into the mana ability's damage too, so
	// a payer at the floor survives its own painland tap. The paid-life arm above
	// stays an unfloored life cost (CR 119.4).
	s.loseLifeDmg(k, p, n)
	s.plDmgMark(p, n) // the rider is damage: wasDealtDamageThisTurn's seat arm (gen.go)
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
		// and a pain-only source by its colour, with no lethal guard (a
		// paid-life one, Mana Confluence, is painAvailMaskAt's)
		if pc.painOnly {
			return uint8(pc.painCol) & 0x3f
		}
		if pc.painCol == kop.ColAny && !pc.painLife {
			// a painland whose painful ability is a free "Add one mana of any
			// color" (Grand Coliseum, CR 605.1a): gorge's unless census
			// (unlessWindowUnits) pins that ability to every colour beside
			// the normal one, the damage rider unpriced
			// (A two-colour Combo painland stays its normal ability alone.)
			return prim | uint8(pc.painCol)&0x3f
		}
		return prim
	}
	m := uint8(0)
	if !pc.painOnly {
		m = prim
	}
	if s.g.availX&availWinCol != 0 && !pc.painLife || int(pc.painN) < int(s.pl[p].life) {
		m |= uint8(pc.painCol) & 0x3f
	}
	return m
}

// painAvailMaskAt is painAvailMask for the source at sl. An unless / ward
// window counts every paid-life activation through gorge's paid-cost layer
// (appendPaidManaUnits, paidCostShape): in battlefield order (zone order,
// battlefieldOrder: a reused slot is not entry order), every tappable
// paid-life alternative counts only while life - committed > its cost, and
// commits its life when it counts. A pain-only source (Mana Confluence) is
// its paid colour alone; a painland's "{T}, Pay 1 life" alternative (Starting
// Town, Hashep Oasis: painLife, not painOnly) joins its normal ability, so a
// later source sees less.
func (s *State) painAvailMaskAt(k *prep, sl Slot, pc *pcard, p uint8) uint8 {
	// The offer walk (neither window's availX) sums a life-only unit's cost
	// across the census, as gorge's planner phase 2 does (a plan whose summed
	// life reaches the payer's life is never a plan, CR 119.4): a life-only
	// source's colour is offered only while life - the earlier life-only
	// units' committed costs > its own. The tap-pain units' summed guard is
	// painTight's (this loop cannot read which ability a plan would tap a
	// painland for); the windows run their own loop below, whose
	// paid-life membership is the window census's.
	if s.g.availX&(availWin|availWinCol) == 0 && lifeOnlyPain(pc) {
		var ord [MaxPerm]Slot
		n := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
		committed := 0
		for _, x := range ord[:n] {
			if x == sl {
				break
			}
			xp := s.manaPCAt(k, x)
			if !lifeOnlyPain(xp) || !s.canLifeOnlyMana(k, x, p) {
				continue
			}
			committed += int(xp.painN)
		}
		if int(s.pl[p].life)-committed <= int(pc.painN) {
			return 0
		}
		return uint8(pc.painCol) & 0x3f
	}
	// The cast window (availWinCol) keeps painAvailMask's own life guard; only
	// the unless / ward window counts a painland's paid-life colour.
	if s.g.availX&(availWin|availWinCol) == 0 || !pc.painLife ||
		(s.g.availX&availWin == 0 && !pc.painOnly) {
		return s.painAvailMask(pc, p)
	}
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.ctrlM[p], ord[:], 0)
	committed := 0
	paid := false
	for _, x := range ord[:n] {
		xp := s.manaPCAt(k, x)
		if !xp.painLife || xp.painCol == 0 || !(s.canTapForManaPC(k, x, xp) || s.canLifeOnlyMana(k, x, p)) {
			continue
		}
		ok := int(s.pl[p].life)-committed > int(xp.painN)
		if x == sl {
			paid = ok
			break
		}
		if ok {
			committed += int(xp.painN)
		}
	}
	m := s.painAvailMask(pc, p)
	if !paid {
		if pc.painOnly {
			return 0
		}
		return m
	}
	if !pc.painOnly {
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
// painCensusPass is the census pass (0 shared, 1 appended) a paired pain unit
// sits on. gorge's census lists a source with a deterministic, free-cost
// production in the shared census, at its battlefield position, and appends
// the rest after: a choice-shaped ability, a tap-and-sacrifice one, an ability
// that pays life (payment_plan.go paymentPlanManaUnitsOnlyCompute). A
// pain-only source's shape is its own ability's, so a cost-of-life one
// (Blightsoil Druid's {T}, Pay 1 life: pay.ManaFreeCost rejects PayLife) joins
// the appended group, while a damage rider (Elves of Deep Shadow's Add {B}.
// deals 1) is a free-cost unit in place.
func painCensusPass(choice bool, pc *pcard) int {
	if choice || pc.painOnly && pc.painLife {
		return 1
	}
	return 0
}

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
	// (srcExtraPain); r is the painful pick's reflected extra beside a
	// several-type production, which rides that unit's amount
	xc, xp, r, ok := s.srcExtraPain(k, sl, pc)
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
		if pain != 0 && r > 0 {
			// a several-type production's reflected extra is one mana of the
			// pick (srcExtraPainPick): the activation makes n of the one
			// colour the plan picks (gorge addTapExtra per alternative)
			n += r
		}
		pk.types[t] = utype{mask: mask, dmask: mask, n: n, creat: cr, flex: fl, pain: pain, alt: alt, irr: 3 * pain, nocol: pain != 0, sacOnly: lifeOnlyPain(pc)}
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
	// a painful type: its colours, damage, the source kind and creature-ness
	// name it (a painful any-colour source of 3 damage, Tarnished Citadel, and
	// one of 1, Grand Coliseum, are two types: key 1 prices each its own); a
	// life-only unit (lifeOnlyPain) shares no type with a {T} one — the type's
	// activation taps or not (utype.sacOnly), so the shape is part of the key
	findP := func() int {
		for t := 0; t < pk.nt; t++ {
			ty := &pk.types[t]
			if ty.pain != 0 && ty.pain == pc.painN && ty.mask == pm && ty.creat == cr && ty.alt == !pc.painOnly && ty.xv == xp &&
				ty.sacOnly == lifeOnlyPain(pc) {
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
			if ty.pair != 0 && !ty.alt && ty.mask == prim && ty.creat == cr && pk.types[ty.pair-1].mask == pm && pk.types[ty.pair-1].pain == pc.painN && ty.xv == xc && pk.types[ty.pair-1].xv == xp {
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
	pk.rAb[pk.nu] = -1
	pk.nu++
	return true
}

// DebugNoPainDamage drops the painful alternatives' damage. The pain
// lockstep's mutation test flips it to prove the pool exercises them.
var DebugNoPainDamage bool

// painAvail is the offer gate's painful units when their summed damage
// could reach the payer's life: the planner's lethal guard then bounds how
// many may make a colour, which a per-unit Hall count cannot express.
// Groups are the distinct (colours, pain-only, pain) shapes of sources (most
// painlands and Elves of Deep Shadow deal 1; Staff of Compleation's mana
// ability pays 2 life); a painland's units sit in the base avail as {C}
// sources, a pain-only source's not at all, and affordsPlain tries each split
// of the budget over the groups, each unit of group g spending w[g] of it.
type painAvail struct {
	on     bool
	n      uint8
	only   uint8    // bit g: group g is pain-only (no normal alternative)
	budget uint8    // pain a plan may take: life - 1
	w      [6]uint8 // the pain one unit of the group takes (its painN)
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
	s.painTightAt(k, p, a, int(s.pl[p].life))
}

// contGrantLive reports whether a granted mana ability can be in force on
// this board: the table has one (anyContGrant) and either no battlefield
// card marks it (an override record's grant, a GainsAbilitiesOf$ one:
// contGrantAlways) or a permanent on either battlefield carries a static that
// grants one. A table-wide grant with none of its sources in play leaves
// every unit's pain as its own ability's, so the summed lethal guard
// (painTightAt) still binds: a Talisman's trigger grant, or an Aura's mana
// grant in a library, never lifts it (gorge's planner prices the plan
// whatever the table holds).
func (s *State) contGrantLive(k *prep) bool {
	g := k.gen
	if g == nil || !g.anyContGrant {
		return false
	}
	if k.contGrantAlways {
		return true
	}
	for seat := range s.ctrlM {
		for m := s.ctrlM[seat]; m.any(); m = m.rest() {
			if k.grantSrcCard[s.cardAt(m.first())] {
				return true
			}
		}
	}
	return false
}

// painTightAt is painTight with the life the summed guard reads cut to eff
// (the payer's life less what a restricted life unit of the payment pays,
// restrLifeCharge): a painful unit's colours stay in a on the payer's own
// life, so the groups carry whichever of them eff no longer admits.
func (s *State) painTightAt(k *prep, p uint8, a *avail, eff int) {
	life := int(s.pl[p].life)
	if life <= 1 || s.g.availX&(availAny|availWin|availWinCol) != 0 || s.contGrantLive(k) {
		return // at 1 life availFor already withheld every painful colour
	}
	modal := a.nModal != 0 // a source with a {C}-only extra: its groups split even when the budget does not bind
	var pa painAvail
	rpAny := false // some painful source's tap makes one mana of the pick more (srcExtraPainPick)
	sum := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCAt(k, sl)
		if pc.painCol == 0 || !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		if int(pc.painN) >= life {
			continue // its pain alone is lethal: painAvailMask already withheld its colours
		}
		if k.gen != nil && k.gen.anyTapExtra {
			r, _, stall := s.srcExtraPainPick(k, sl, pc)
			if stall {
				s.capFull(StallTapExtra)
			}
			rpAny = rpAny || r > 0
		}
		sum += int(pc.painN)
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
			if pa.mask[g] == pm && pa.prim[g] == prim && pa.only>>g&1 == ob && pa.ec[g] == ec && pa.w[g] == pc.painN {
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
			pa.w[g] = pc.painN
			pa.n++
		}
		pa.cnt[g]++
	}
	if rpAny && (sum >= eff || modal) {
		// the groups' budget counts activations, not mana: a unit whose tap
		// makes one mana of the pick more has no per-activation price here
		s.capFull(StallTapExtra)
		return
	}
	if sum < eff && !modal {
		return
	}
	pa.budget = uint8(min(eff-1, 255))
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
			used += int(x[g]) * int(pa.w[g])
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
const availWin uint16 = 32

// availWindowFor is availFor for gorge's unless / ward payability: the
// shared census's units (painAvailMask), not the planner's.
func (s *State) availWindowFor(k *prep, p uint8, a *avail) {
	x := s.g.availX
	if k.painMana {
		s.g.availX |= availWin
	}
	s.availFor(k, p, a)
	s.g.availX = x
	if k.gen != nil && k.gen.anyComboAny && x&3 == 0 {
		s.availCA(k, p, a) // the window taps a Combo Any source for its amount (mana_comboany.go; Sage of the Maze pays Urza's Chalice's {1})
	}
}

// availWindowForCapped is availWindowFor with the planner-deferred sources
// counted (availForCapped): the unless-cost gate of a mode offer must see
// every source the payment window can tap, including a mana rider whose tap
// the planner census leaves out.
func (s *State) availWindowForCapped(k *prep, p uint8, a *avail) {
	x := s.g.availX
	if k.painMana {
		s.g.availX |= availWin
	}
	if k.gen == nil {
		// no generated table: there is no planner census to defer, and the
		// script walk has no table to read
		s.availFor(k, p, a)
		s.g.availX = x
		return
	}
	s.g.availX |= availGaps
	s.availForMode(k, p, a, slotMask{}, true)
	s.g.availX = x
	if k.gen.anyComboAny && x&3 == 0 {
		s.availCA(k, p, a)
	}
}

// availWinCol is the availFor mode of a manual cast window's affordability
// (a hybrid pip's announcement: gorge's planner refuses the cost, the
// window pays it): every painful colour counts, with no lethal guard (the
// window's answer taps a painland for a colour whatever the life). A
// paid-life source (Mana Confluence, Starting Town) keeps its guard: gorge's
// source-window probe skips a paid-life alternative that would leave the payer
// at 0 or below (pay.windowManaAlt LifeKeep), so a hybrid face only that
// source could make is not offered at 1 life (painAvailMaskAt).
const availWinCol uint16 = 64

// availCastWindowFor is availFor under availWinCol.
func (s *State) availCastWindowFor(k *prep, p uint8, a *avail) {
	x := s.g.availX
	if k.painMana {
		s.g.availX |= availWinCol
	}
	s.availFor(k, p, a)
	s.g.availX = x
	if k.gen != nil && k.gen.anyComboAny && x&3 == 0 {
		s.availCA(k, p, a) // the window taps a Combo Any source (mana_comboany.go)
	}
}

// loweredWindowOK pays a lowered cast through its window: the window's answers
// can spend a source on a colour the cost does not owe (a painland or a gain
// land on its lowest), leaving the cost stranded with no useful source left;
// gorge then aborts the cast (CR 733.1), keeping the taps, the floated mana and
// the damage. It reports false after reversing. On true the cost is paid and
// left as it was, for the cast's recorded spend, and the restricted mana held
// out of it floats in the pool. xAnnounced: the cost folds an announced X, whose
// window may tap a planner-deferred source (windowTapWalk).
//
//go:noinline
func (s *State) loweredWindowOK(k *prep, p uint8, cost *kop.ManaCost, anyType, anyColor, xAnnounced bool) bool {
	ok, held := s.windowTapWalk(k, p, cost, cost, !anyType && !anyColor, anyColor, s.bld().pipN == 0, false, xAnnounced, false, false)
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

// printedAbPain is the activation pain (abPainOf) of the source's printed mana
// ability pc names; 0 when it has none.
func (s *State) printedAbPain(k *prep, sl Slot, pc *pcard) int {
	if pc.manaIntrinsic {
		return 0 // the view is a granted basic land type's intrinsic ability (blgView): its activation costs no life; the card's rider's pain prices its own fold (availPotentialFor's fixpoint, abPainOf)
	}
	if pc.manaAb < 0 {
		return 0
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	if int(pc.manaAb) >= len(cd.Abilities) {
		return 0
	}
	return abPainOf(&cd.Abilities[pc.manaAb])
}

// painGate is the census's lethal-guard read for a production of colours whose
// activation costs pain: when pain reaches life the planner never takes the
// activation, so drop is colours and the returned add records no census unit
// for them (recDrop) while the fold keeps the production whole; otherwise drop
// is 0 and add is returned as it came.
func (c *potCensus) painGate(pain, life int, colors uint8, add, fold func(Slot, uint8, uint8)) (drop uint8, out func(Slot, uint8, uint8)) {
	if pain <= 0 || pain < life || colors == 0 {
		return 0, add
	}
	return colors, func(sl Slot, mask, n uint8) {
		c.recDrop(sl, mask, n, colors)
		fold(sl, mask, n)
	}
}
