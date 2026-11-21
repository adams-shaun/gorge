package dirac

import (
	"math/bits"
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// GrantManaStats counts the granted mana abilities a play's prefix
// activated: the taps replayed at the play (grantApply) and the
// sacrifice-only steps the search branches on (grantSacDo, its forks
// included). Pool tests read it so a deck that never funds a play through a
// granted source cannot pass vacuously.
var GrantManaStats struct {
	Taps atomic.Int64
	Sac  atomic.Int64
}

// Granted mana abilities in the cast offers (tokmana). A Continuous
// static's AddAbility$ grant of a mana ability reaches permanents its
// filter matches anywhere on the battlefield (Springleaf Parade's "Creature
// tokens you control have '{T}: Add one mana of any color'", Cryptolith
// Rite, Chromatic Lantern's lands, Gemhide Sliver's Slivers of either
// player), not only the permanent the static's source is attached to
// (grantedAbilitiesN's grantM walk: an Aura's or Equipment's host). The
// grant walk below adds those (contGrants), so manaPCGen / manaGranted see
// the granted source exactly as Imprisoned in the Moon's land.
//
// gorge (pin 81dd3f940) lists the granted ability in every mana walk
// (rules mana_activation.go appendAvailableManaAbilitiesGateCompute: the
// Continuous AddAbility$ scan, then grantedAbilities), but its payment
// planner never witnesses one (pay/board.go PaymentAbility excludes grants), so the granted source is a census gap
// (potential_plan.go paymentPlanCensusOf): a play the printed sources
// cannot pay gets a prefix search (potential_script.go
// potentialPrefixScript) that activates the gap sources first, branching
// over every answer (the colour of an "Any" ability, W U B R G in order)
// and visiting each distinct payer state once within potentialScriptNodes
// clones; the play is offered when the search finds a prefix after which
// the pool and the printed sources pay it, and the oracle plays that prefix
// before the play (prScript). grantSearch copies that search: the kernel's
// offer (grantCap) keeps only a play the search pays, and the play taps the
// found prefix into the pool (grantPrefix) before its own payment.
//
// A permanent with a printed mana ability AND a granted one (Llanowar Elves
// under Cryptolith Rite; every land under Chromatic Lantern) is a printed
// source for the planner (manaPCGen returns its own ability: gorge's
// census covers the printed ability, the granted one is the gap) and a
// prefix candidate whose activation first offers the printed ability, then
// the granted one (gorge's "choose a mana ability" ask, printed first).
//
// Shapes outside "{T}: Add <fixed colours or any> x N" (a spend
// restriction, a mana fee, a dynamic amount, a sacrifice or life part) are
// refused by kgen (dirac/kgen/lower_grantedmana.go grantedManaShape).

// (potentialScriptNodes, the node budget, is mana_filter.go's.)

// prepTokMana marks the cards with a Continuous grant of a mana ability
// whose filter is not limited to the source's host (contGrant).
func prepTokMana(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			a := &c.Abilities[j]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 {
				continue
			}
			if int(a.Static.Grant-1) >= len(c.Abilities) || c.Abilities[a.Static.Grant-1].Kind != kop.AbMana {
				continue
			}
			if filterAttachedOnly(t, a.Static.Filter) {
				g.anyContGrant = true // availFor counts the grant beside the host's own ability (New Horizons' Forest)
				continue              // the grantM walk's (an Aura's / Equipment's host)
			}
			g.setFlag(CardID(i), gfContGrant)
			g.anyContGrant = true
		}
	}
}

// prepOvrManaGrant marks beside prepTokMana the tables whose override
// records grant a mana ability: an AnimateAll's Abilities$ grant (Rain of
// Filth's "lands you control gain 'Sacrifice this land: Add {B}'") is a
// record grant (ovGrantsBetween), which grantIdxs lists beside the statics'
// as the walk's tail (a grant without a source in it keeps its place after
// them), so availFor's dual census must run for its sources too. The Grant
// indexes name abilities of the record's own card, so the sweep is
// per-card: only a record the card's own programs apply counts.
func prepOvrManaGrant(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		c := &t.Cards[i]
		for j := range c.Abilities {
			for _, in := range c.Abilities[j].Prog {
				if in.Op != kop.OpOverride && in.Op != kop.OpOverrideAll {
					continue
				}
				if int(in.C) >= len(t.Overrides) {
					continue
				}
				for _, gr := range t.Overrides[in.C].Grant {
					if gr != 0 && int(gr-1) < len(c.Abilities) && c.Abilities[gr-1].Kind == kop.AbMana {
						g.anyContGrant = true // the record grant is availFor's dual census (mana_grantedoffers.go)
						return
					}
				}
			}
		}
	}
}

// grantWalkKey is a slot's place in gorge's static walk as one number: the
// controller's seat, then the permanent's timestamp (listenerOrder's order,
// which is each seat's battlefield list in entry order). Equal keys keep
// slot order, as the walk's stable timestamp sort does.
func grantWalkKey(s *State, sl Slot) uint32 {
	p := &s.perms[sl]
	return uint32(p.ctrl)<<16 | uint32(p.ts)
}

// grantOrder writes the slots of statM whose card at cardAt can grant an
// ability from a static (pc.grantStat: a StatOverride static, or a
// Continuous static with a Grant -- prep.go's), in listenerOrder's order
// (gorge's static walk: seat order, then timestamp), and returns their
// count. It replaces listenerOrder's sort of EVERY static slot on each
// query (the sweep's 37% of std-big): one pass over statM inserts only the
// grant-capable slots in (seat, timestamp) order.
//
// grantStat is read off cardAt's card, so the list is the full walk
// restricted to the slots it could name: every source grantSrc names --
// grantsBetween's attached StatOverride and Continuous Grant statics, and
// contGrants' gfContGrant cards -- carries it, so an order-preserving
// restriction gives grantIdxs' place scan and contGrants' walk the same
// relative order listenerOrder's list does. Equal keys keep slot order, as
// the walk's stable sort does.
func (s *State) grantOrder(k *prep, ord *[MaxPerm]Slot) int {
	n := 0
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		if !k.cards[s.cardAt(sl)].grantStat {
			continue
		}
		key := grantWalkKey(s, sl)
		j := n
		for j > 0 && grantWalkKey(s, ord[j-1]) > key {
			ord[j] = ord[j-1]
			j--
		}
		ord[j] = sl
		n++
	}
	return n
}

// contGrants calls f for the mana abilities unattached Continuous grants
// give sl: each contGrant static in force whose condition holds and whose
// filter matches sl (a grant older than a "loses all abilities" effect on
// sl is removed with the rest). A static attached to sl is the grantM
// walk's.
//
//go:noinline
func (s *State) contGrants(k *prep, sl Slot, lost bool, lts uint16, f func(c CardID, ab uint8, n int)) {
	// gorge lists the grants in its static walk's order (collectActionStatics:
	// each seat's battlefield in entry order), which the activation ask
	// and the prefix search follow (the first grant's colours are tried
	// first): every seat's sources in seat order, each in entry order.
	// grantOrder's list is that walk's order restricted to the slots whose
	// card at cardAt can grant a static ability (pc.grantStat); the loop's
	// gfContGrant check keeps the ones f can be called for, in the same
	// relative order.
	var order [MaxPerm]Slot
	n := s.grantOrder(k, &order)
	for oi := 0; oi < n; oi++ {
		src := order[oi]
		cid := s.cardAt(src)
		if !k.gen.has(cid, gfContGrant) || s.perms[src].attached == sl || s.genLost(k, src) {
			continue
		}
		if lost && s.layerTs(src) < lts {
			continue
		}
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Grant == 0 || cd.Abilities[a.Static.Grant-1].Kind != kop.AbMana {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: s.perms[src].ctrl, obj: s.perms[src].obj}
			if !s.statCond(k, st) || !s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, src, layerKw) {
				continue
			}
			prev := s.g.grantSrc
			s.g.grantSrc = st.obj + 1 // grantIdxs orders by this source's place in the walk
			f(cid, a.Static.Grant-1, 1)
			s.g.grantSrc = prev
		}
	}
}

// maxGrants bounds the granted mana abilities read on one permanent (two
// Gemhide Slivers grant a Sliver two; the tokmana pool reaches 10 on a
// changeling token); one more stalls (StallGrants).
const maxGrants = 16

// grantIdxs lists the genPrep.grantPC indices of the mana abilities effects
// grant sl, in the grant walk's order, one per granting instance (two
// Gemhide Slivers give a Sliver two abilities, as gorge lists them: each
// is its own activation choice); n is their count.
//
//go:noinline
func (s *State) grantIdxs(k *prep, sl Slot, out *[maxGrants]int16) (n int) {
	g := k.gen
	// gorge lists every Continuous AddAbility$ grant in its static walk's
	// order (seat, then entry), an attached source's among the unattached
	// ones (an Honest Work's "{T}: Add {C}" on a land of the other seat
	// follows a Great Divide Guide's grant of the first); grantedAbilities
	// reports the attached statics first, so the list is stably reordered
	// by the granting source's place in that walk. A grant without a
	// source in it (an Animate record) keeps its place after them.
	var order [MaxPerm]Slot
	no := 0
	if g.anyContGrant {
		// the grant walk's order restricted to the slots whose card at
		// cardAt can grant a static ability (pc.grantStat): grantsBetween's
		// attached statics and contGrants' unattached ones, every source
		// grantSrc names. The place scan below reads the same relative
		// order off it as off listenerOrder's full walk.
		no = s.grantOrder(k, &order)
	}
	var key [maxGrants]uint8
	s.grantedAbilities(k, sl, func(c CardID, ab uint8) {
		pos := uint8(255)
		for oi := 0; oi < no; oi++ {
			if s.g.grantSrc != 0 && s.perms[order[oi]].obj+1 == s.g.grantSrc {
				pos = uint8(oi)
				break
			}
		}
		for i, r := range g.grantOf {
			if r.card != c || r.ab != ab {
				continue
			}
			if n == maxGrants {
				s.stall(StallGrants) // capacity: loud, never a dropped grant
				continue
			}
			if g.grantSelf[i] && s.selfColMask(sl) == 0 {
				continue // a colourless grantee reflects nothing: gorge lists no such ability
			}
			j := n
			for j > 0 && key[j-1] > pos {
				out[j], key[j] = out[j-1], key[j-1]
				j--
			}
			out[j], key[j] = int16(i), pos
			n++
		}
	})
	if g.uses.has(useGainedMana) {
		n = s.gainedIdxs(k, sl, out, n) // the mana abilities a GainsAbilitiesOf$ static gives sl (mana_gainedgrants.go)
	}
	return n
}

// grantIdx is the genPrep.grantPC index of the first mana ability an effect
// grants sl, -1 when none.
//
//go:noinline
func (s *State) grantIdx(k *prep, sl Slot) int {
	var gi [maxGrants]int16
	if s.grantIdxs(k, sl, &gi) == 0 {
		return -1
	}
	return int(gi[0])
}

// ownSacGrantTap is the planner's side of availGrant's ownSac shape: the
// granted {T} mana ability beside sl's printed sacrifice-only one, an
// Eldrazi Spawn under Path of Annihilation (CR 602.2b, 605.1a). nil when
// the grant is a sacrifice itself, restricted, or cannot be activated now
// (a tapped or summoning-sick source has only the sacrifice, CR 302.6): the
// census credits no tap for those either.
//
//go:noinline
func (s *State) ownSacGrantTap(k *prep, sl Slot) *pcard {
	if k.gen == nil || !k.sacOnlyMana {
		return nil
	}
	var gi [maxGrants]int16
	ng := s.grantIdxs(k, sl, &gi)
	for i := 0; i < ng; i++ {
		if k.gen.grantRestr[gi[i]] != 0 {
			continue
		}
		g := s.grantView(k.gen, gi[i], sl)
		if !g.grantSac && s.canTapForManaPC(k, sl, g) {
			return g
		}
	}
	return nil
}

// planGrants is one source's granted abilities the planner prices: a printed
// sacrifice-only source's granted {T} (ownSacGrantTap), and a printed tap
// source's granted sacrifice, its joined colours and amount (planGrantSacOf).
type planGrants struct {
	tap        *pcard
	sacM, sacN uint8
}

// planGrantsOf reads sl's planGrants into g and reports whether it has one.
func (s *State) planGrantsOf(k *prep, pc *pcard, sl Slot, g *planGrants) bool {
	g.tap = s.planOwnSacGrant(k, pc, sl)
	if s.planTapGrantSac(k, sl) {
		g.sacM, g.sacN, _ = s.planGrantSacOf(k, sl)
	}
	return g.tap != nil || g.sacN != 0
}

// planGrantSacOnly handles a printed tap ability the planner prices that
// cannot tap now (tapped, summoning sick: CR 302.6) beside a live granted
// sacrifice-only ability (planTapGrantSacNow, Rain of Filth's land): the grant
// is the shape's only unit, phase 2's (gorge's planner never sees a granted
// ability, so a plan with one is reached only when the printed units have no
// plan), its sacrifice planGrantSacUnit's. It reports whether the source is
// done for this pass; before phase 2 it only marks the shape as having a grant.
func (s *State) planGrantSacOnly(k *prep, p uint8, pk *planner, sl Slot, pc *pcard, phase int, g *planGrants, hasGrant *bool) bool {
	if g.sacN == 0 || s.canTapForManaPC(k, sl, pc) {
		return false
	}
	if phase < 2 {
		*hasGrant = true
		return true
	}
	s.planGrantSacUnit(k, p, pk, sl, g.sacM, g.sacN, s.isCreature(k, sl))
	return true
}

// planGrantsPhase2 adds sl's granted units in phase 2, after its printed unit:
// the granted {T} of a sacrifice-only source, and the granted sacrifice of an
// untapped tap source (its printed unit is phase 0's; planTapGrantSac).
func (s *State) planGrantsPhase2(k *prep, p uint8, pk *planner, sl Slot, pc *pcard, phase int, g *planGrants, cr bool) {
	if phase != 2 {
		return
	}
	if g.tap != nil {
		s.planGrantUnit(k, p, pk, sl, g.tap, cr)
	}
	if g.sacN != 0 && s.canTapForManaPC(k, sl, pc) {
		s.planGrantSacUnit(k, p, pk, sl, g.sacM, g.sacN, cr)
	}
}

// planOwnSacGrant is the facts loop's read of a printed sacrifice-only
// source's granted {T} (ownSacGrantTap), the mirror of a granted sacrifice
// beside a printed tap (availGrant's ownSac): two units, the grant's tap
// beside the printed {C} (CR 602.2b, 605.1a). The grant is phase 2's unit:
// phase 1 holds only the printed sacrifice, gorge's census, so a cast its
// printed plan pays is planned as gorge does (the grant is the prefix
// search's alternative), and the grant joins only when that plan fails.
func (s *State) planOwnSacGrant(k *prep, pc *pcard, sl Slot) *pcard {
	if !k.sacOnlyMana || !pc.manaSacOnly {
		return nil
	}
	return s.ownSacGrantTap(k, sl)
}

// planGrantUnit (called from plan's phase 2; gorge's planner never sees a
// granted ability, so a plan with one is the kernel's, reached only when the
// printed units have no plan) adds gt, a granted mana view of sl, as its own unit of the
// planner beside the printed unit the loop emitted: the tap unit of
// ownSacGrantTap, shaped like a grant-only source's unit (manaPCGen returns
// its granted view there).
func (s *State) planGrantUnit(k *prep, p uint8, pk *planner, sl Slot, gt *pcard, cr bool) {
	if pk.nu >= maxUnits || pk.nt >= maxTypes {
		return // the caller's loop holds the same guards
	}
	mask, n := s.srcMaskAt(gt, sl), gt.manaN
	dmk := mask
	fl := uint8(bits.OnesCount8(dmk))
	ti := -1
	for t := 0; t < pk.nt; t++ {
		ty := &pk.types[t]
		if ty.mask == mask && ty.dmask == dmk && ty.n == n && ty.creat == cr && !ty.sac && !ty.sacOnly &&
			ty.pair == 0 && ty.pain == 0 && ty.first == 0 && ty.printed == 0 && ty.flex == fl &&
			ty.xv == ([kop.NumManaKinds]uint8{}) && !ty.rb {
			ti = t
			break
		}
	}
	if ti < 0 {
		ti = pk.nt
		pk.nt++
		pk.types[ti] = utype{mask: mask, dmask: dmk, n: n, creat: cr, flex: fl}
		if int(n) > pk.maxN {
			pk.maxN = int(n)
		}
	}
	pk.types[ti].avail++
	pk.units[pk.nu] = sl
	pk.uobj[pk.nu] = s.perms[sl].obj
	pk.utyp[pk.nu] = uint8(ti)
	pk.rkind[pk.nu] = 0
	// the grant's ability number names nothing on sl's own card, and
	// fireViewAbilityCast's granted-source skip reads manaGranted, which an
	// own-sacrifice source is not: -1 fires nothing (trig_abilitycast.go)
	pk.rAb[pk.nu] = -1
	pk.nu++
	for c := 0; c < 5; c++ {
		if dmk&(1<<c) != 0 {
			pk.colSrc[c]++
		}
	}
}

// planInertGrant registers, in phases 0 and 1, the census unit of a source
// whose only mana ability an effect grants. gorge's shared census prices a
// fixed grant like a printed ability (windowManaUnitsWith appends the
// deterministic alternative, the source becomes a unit at its battlefield
// position) before pay.PlanAlts drops the granted alternative (PaymentAbility's
// V1 exclusion): the unit has no search class and is never selectable, but it
// keeps its position, and a class's materialize limit counts every unit before
// its members (search.go materialize's limit(k)). The kernel has no
// anchor-only unit, so the position is registered as a zero-production type
// that dfs never selects (utype.inert). Only a fixed unrestricted tap grant
// has a deterministic alternative in gorge's walk, and only a source that can
// tap now: a tapped or summoning-sick source leaves the walk without the
// unit. Phase 2 registers the grant as payable (planGrantUnit's ordinary loop
// pass), so this is the deterministic pass 0 of phases 0 and 1 only.
func (s *State) planInertGrant(k *prep, pk *planner, i int, sl Slot, pc *pcard, pass int) {
	if pass != 0 || pk.nu >= maxUnits || !s.grantsFixedMana(k, sl) || !s.canTapForManaPC(k, sl, pc) {
		return
	}
	ti := -1
	for t := 0; t < pk.nt; t++ {
		if pk.types[t].inert {
			ti = t
			break
		}
	}
	if ti < 0 {
		if pk.nt == maxTypes {
			return // no room for another shape (no pool board reaches it)
		}
		ti = pk.nt
		pk.nt++
		pk.types[ti] = utype{inert: true}
	}
	pk.types[ti].avail++
	pk.units[pk.nu] = sl
	if pk.bfl {
		pk.uobj[pk.nu] = Obj(i) // battlefield order (plan's hybrid census)
	} else {
		pk.uobj[pk.nu] = s.perms[sl].obj
	}
	pk.utyp[pk.nu] = uint8(ti)
	pk.rkind[pk.nu] = 0
	pk.rAb[pk.nu] = -1 // planGrantUnit's: the grant's ability number names nothing on sl's own card
	pk.nu++
}

// ownManaUsable reports whether sl's own card has a usable tap mana
// planGrantSacUnit (called from plan's phase 2, beside planGrantUnit; gorge's
// planner never sees a granted ability, so a plan with one is the kernel's,
// reached only when the printed units have no plan) adds sl's granted
// sacrifice-only abilities as one unit: the source is sacrificed once (CR
// 605.1a), the colours joined, the amount the largest. It is the printed
// last-resort unit's sacrifice (utype.sac) with no tap (sacOnly):
// grantSacLive costs no {T}, tapped or summoning sick alike (CR 302.6).
func (s *State) planGrantSacUnit(k *prep, p uint8, pk *planner, sl Slot, mask, n uint8, cr bool) {
	if pk.nu >= maxUnits || pk.nt >= maxTypes {
		return // the caller's loop holds the same guards
	}
	dmk := mask
	fl := uint8(bits.OnesCount8(dmk))
	ti := -1
	for t := 0; t < pk.nt; t++ {
		ty := &pk.types[t]
		if ty.mask == mask && ty.dmask == dmk && ty.n == n && ty.creat == cr && ty.sac && ty.sacOnly &&
			ty.pair == 0 && ty.pain == 0 && ty.first == 0 && ty.printed == 0 && ty.flex == fl &&
			ty.xv == ([kop.NumManaKinds]uint8{}) && !ty.rb {
			ti = t
			break
		}
	}
	if ti < 0 {
		ti = pk.nt
		pk.nt++
		pk.types[ti] = utype{mask: mask, dmask: dmk, n: n, creat: cr, flex: fl, sac: true, sacOnly: true, nocol: true, irr: unitIrr(true, cr, false)}
		if int(n) > pk.maxN {
			pk.maxN = int(n)
		}
	}
	pk.types[ti].avail++
	pk.units[pk.nu] = sl
	pk.uobj[pk.nu] = s.perms[sl].obj
	pk.utyp[pk.nu] = uint8(ti)
	pk.rkind[pk.nu] = 0
	pk.rAb[pk.nu] = -1 // planGrantUnit's: the grant's ability number names nothing on sl's own card
	pk.nu++
	for c := 0; c < 5; c++ {
		if dmk&(1<<c) != 0 {
			pk.colSrc[c]++
		}
	}
}

// planGrantSacOf is the payment plan's read of sl's live granted
// sacrifice-only abilities, availGrantSacOnly's census shape: the colours
// joined, the amount the largest (the source is sacrificed once). ok is
// false when none is live.
func (s *State) planGrantSacOf(k *prep, sl Slot) (mask, n uint8, ok bool) {
	if !s.grantSacOK(k, sl) {
		return 0, 0, false
	}
	var gi [maxGrants]int16
	for i, ng := 0, s.grantIdxs(k, sl, &gi); i < ng; i++ {
		gv := s.grantView(k.gen, gi[i], sl)
		if !s.grantSacLive(sl, gv) {
			continue
		}
		mask |= s.srcMaskAt(gv, sl)
		n = max(n, gv.manaN)
	}
	return mask, n, mask != 0
}

// planTapGrantSac: sl's printed mana ability is a plain tap the planner
// prices while an effect grants it a live sacrifice-only ability (Rain of
// Filth's land): plan's phase 2 prices the grant, the mirror of
// planOwnSacGrant's granted {T} beside a printed sacrifice
// (availGrantSacOnly counts both on the offer side).
func (s *State) planTapGrantSac(k *prep, sl Slot) bool {
	pc := &k.cards[s.cardAt(sl)]
	if pc.manaAb < 0 || !pc.manaTap || pc.manaSac || pc.manaSacOnly {
		return false // not a plain printed tap, or phase 1's own shape
	}
	return s.grantSacOK(k, sl)
}

// planTapGrantSacNow is the payment plan's order gate for it: the printed
// tap cannot be used now (tapped, summoning sick: CR 302.6), so the grant
// is the shape's only unit.
func (s *State) planTapGrantSacNow(k *prep, sl Slot) bool {
	return !s.canTapForMana(k, sl) && s.planTapGrantSac(k, sl)
}

// ability (one the planner prices: a plain or sacrificing tap, a chosen
// colour, a dynamic amount) that it has not lost. Beside a granted mana
// ability it is the planner's source (gorge's census covers it); the
// grant is the prefix search's alternative (grantAlt).
//
//go:noinline
func (s *State) ownManaUsable(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	if k.gen.flags[c]&gfManaGate != 0 {
		// a gated source's own ability is the gate's view, not the
		// variant flag: a gate-off single gated source has no mana ability
		// (the grant is then the source's only unit, as gorge offers it).
		pc := s.manaGateView(k, sl, c, k.gen.manaVar(c))
		return pc.manaAb >= 0 && (pc.manaTap || pc.manaSac || pc.manaSacOnly || lifeOnlyPain(pc)) && !s.genLost(k, sl)
	}
	if k.gen.flags[c]&gfManaRefl != 0 {
		// a reflected source's own ability is its view of the colours it
		// reads now: one that reads none (Hidden Hideout with no commander,
		// CR 903.4f) makes no mana, and gorge's census holds no unit for it,
		// so a granted ability (Weirding Wood's) is the source's only one
		pc := s.manaReflView(k, sl, c, k.gen.manaVar(c))
		return pc.manaAb >= 0 && (pc.manaTap || pc.manaSac || pc.manaSacOnly || lifeOnlyPain(pc)) && !s.genLost(k, sl)
	}
	pc := &k.cards[c]
	if !(pc.manaAb >= 0 && (pc.manaTap || pc.manaSac || pc.manaSacOnly || lifeOnlyPain(pc)) || k.gen.flags[c]&gfManaVar != 0) || k.gen.flags[c]&gfManaVar != 0 && s.varManaEmpty(k, sl, c) {
		return false
	}
	return !s.genLost(k, sl)
}

// varManaEmpty reports whether c's own variable mana ability makes no mana
// at sl now: a reflected source reading no colour (Hidden Hideout's "one mana
// of any color in your commander's color identity" with no commander; CR
// 903.4 reads nothing, not colourless) or a plain dynamic amount of 0. Such a
// source has no own ability to be the planner's, so a mana ability an Aura
// grants it is its only unit (CR 605.1a), as gorge lists it. A gate, pair,
// split or chosen-colour source keeps its own view.
func (s *State) varManaEmpty(k *prep, sl Slot, c CardID) bool {
	g := k.gen
	vi := g.manaVar(c)
	if vi < 0 || g.flags[c]&(gfManaGate|gfManaDynPair) != 0 || g.anyCondSplit && g.splitAb(c) >= 0 {
		return false
	}
	if g.flags[c]&gfManaRefl != 0 {
		return g.varPC[int(vi)+int(s.reflColours(k, sl, c))].manaAb < 0
	}
	cd := &k.t.Cards[c]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbMana {
			continue
		}
		if a.AFlags&kop.AfManaChosen != 0 {
			return false
		}
		return g.varPC[int(vi)+min(max(s.dynManaN(k, a, sl), 0), maxDynMana)].manaAb < 0
	}
	return false
}

// windowGrantMask is the colours of the mana abilities effects grant sl,
// joined: what the cast window's clamp reads beside the source's own ability.
//
//go:noinline
func (s *State) windowGrantMask(k *prep, sl Slot) (mask uint8) {
	var gi [maxGrants]int16
	for i, n := 0, s.grantIdxs(k, sl, &gi); i < n; i++ {
		if g := s.grantView(k.gen, gi[i], sl); s.grantCostPayable(s.perms[sl].ctrl, g) {
			mask |= s.srcMaskAt(g, sl) // a life its controller cannot pay activates nothing (CR 119.4)
		}
	}
	return mask
}

// grantUnit is a source's merged granted mana: the largest amount one
// activation makes, its colour mask, and whether that amount is all one
// chosen colour (manaOneColour), which the availability must model as a
// one-colour unit rather than that many independent any-colour units. alt is
// the source's own fixed face when a larger one-colour grant governs the
// merged unit (a colourless host's {C} under Nature's Embrace): the payment
// takes that face or the grant, never both.
type grantUnit struct {
	mask, n       uint8
	one           bool
	altMask, altN uint8
}

// add merges one grant into u: a larger amount governs, and an equal amount
// is one-colour only when every candidate of that amount is.
func (u *grantUnit) add(mask, n uint8, one bool) {
	u.mask |= mask
	if n > u.n {
		u.n, u.one = n, one
	} else if n == u.n {
		u.one = u.one && one
	}
}

// mergeGrantFace joins a source's own tap face with its granted tap face: the
// permanent taps once, so the two are alternatives. A larger grant, or one of
// the same amount that is one-colour, governs the merged amount. The own face
// is kept as the fixed alternative only when it makes {C}, the one colour the
// grant's WUBRG block cannot choose -- a printed colour is already covered by
// that block -- so the printed {C} is neither lost nor counted beside the
// grant (CR 106.4, 106.12a). Otherwise the own face governs, as before.
func mergeGrantFace(own, grant grantUnit) grantUnit {
	if grant.mask == 0 {
		return own
	}
	if own.mask == 0 {
		return grant
	}
	if grant.n > own.n || grant.n == own.n && grant.one {
		if grant.one && own.mask&(1<<kop.C) != 0 {
			grant.altMask, grant.altN = own.mask, own.n
		}
		grant.mask |= own.mask
		return grant
	}
	if grant.n == own.n {
		grant.one = grant.one && own.one
	}
	own.mask |= grant.mask
	return own
}

// addGrantUnit adds u's unit to a: its colours and count, and its one-colour
// constraint when the amount is all one chosen colour. The one-colour entry
// carries u's group mask so oneColourPays takes this unit's units out even
// when the source's printed mask joined them (0x3f), and u's fixed
// alternative face when the printed ability is the other choice.
func (s *State) addGrantUnit(a *avail, u grantUnit) {
	if u.mask == 0 {
		return
	}
	s.addGrantUnits(a, u.mask, u.n)
	if u.one {
		s.addOneColourAlt(a, u.mask, u.n, u.altMask, u.altN)
	}
}

// grantTapNow reports whether sl can pay a {T} ability now: untapped and not
// summoning sick (CR 302.6).
func (s *State) grantTapNow(k *prep, sl Slot) bool {
	return s.perms[sl].flags&pfTapped == 0 && !s.activationSick(k, sl)
}

// availGrant adds slot sl's unit(s) to a (availFor) on a table with
// Continuous mana grants: its own ability's colours (pc) joined with
// every granted ability's, one unit (the permanent taps once; gorge's
// potential walk lists a play any prefix pays). Under availWalk -- the
// walk grantCap counts gorge's prefix searches over -- the relaxed census
// instead: gorge's unpayability proof (potential_plan.go
// paymentPlanRelaxedAlternatives) relaxes EACH granted ability of a source
// to its own free unit beside the printed one, so a play those units pay
// takes one of the four searches even when no real sequence pays it.
//
//go:noinline
func (s *State) availGrant(k *prep, sl Slot, pc *pcard, a *avail) {
	mask, n := s.srcMaskAt(pc, sl), srcYield(pc) // a pair view's larger alternative, as availFor (mana_gate.go)
	var gi [maxGrants]int16
	ng := s.grantIdxs(k, sl, &gi)
	own := s.ownManaUsable(k, sl)
	// The tap's reflected extra (Mana Flare over a dual: one more mana of the
	// type the tap produced, gorge addTapExtra) rides the own face as the plain
	// census's does (availForMode's srcYieldAt): availGrantExtra prices only a
	// fixed-vector extra, and a dual (or any-colour) source's pick-priced one
	// dropped out of the census, withholding a hand cast gorge priced
	// (tail-41 pair 2, Genestealer Patriarch over Mana Flare).
	if own {
		n += s.srcReflectPick(k, sl, pc)
	}
	if a.xdOn {
		if n := s.grantsAdmitted(k, sl, &gi, ng, &a.xd); n < ng {
			if ng = n; ng == 0 && !own {
				return // the source made mana only by a grant the payment cannot spend
			}
		}
	}
	// CR 605.1a, 601.2g-h: the plain payment taps the printed ability of a dual
	// source (manaPCGen), so the plain X ceiling must not credit its granted one.
	if a.xPlain && own && ng > 0 {
		ng = 0
	}
	if ng == 0 && k.painMana && pc.painCol != 0 {
		s.addPainAvail(k, sl, pc, s.perms[sl].ctrl, a, nil, false)
		return
	}
	if ng == 0 || (!own && ng == 1) {
		if ng == 1 {
			// a grant-only source whose single ability costs a {T} prices
			// nothing while it cannot tap (CR 302.6): its early return bypasses
			// the census gate availFor applies
			if gv := s.grantView(k.gen, gi[0], sl); gv.manaTap && !s.grantTapNow(k, sl) {
				return
			}
		}
		s.addGrantUnits(a, mask, n)
		if ng == 0 && pc.manaOne {
			s.addOneColour(a, n)
		} else if ng == 1 && s.g.availX&availHyp == 0 {
			// a source whose only mana is one grant: its "N mana of any one color" is one colour
			if gv := s.grantView(k.gen, gi[0], sl); gv.manaOne {
				s.addOneColour(a, gv.manaN)
			}
		}
		s.availGrantExtra(k, sl, pc, &gi, ng, own, a)
		return
	}
	if s.g.availX&availWalk != 0 {
		if own {
			s.addGrantUnits(a, mask, n)
			if pc.manaOne && s.g.availX&availHyp == 0 {
				s.addOneColour(a, n) // the printed amount is one colour, whichever is chosen (availFor's census)
			}
		}
		for i := 0; i < ng; i++ {
			gv := s.grantView(k.gen, gi[i], sl)
			s.addGrantUnits(a, s.srcMaskAt(gv, sl), gv.manaN)
			if gv.manaOne && s.g.availX&availHyp == 0 {
				s.addOneColour(a, gv.manaN) // a grant's "two mana of any one color" is one colour, whichever is chosen
			}
		}
		s.availGrantExtra(k, sl, pc, &gi, ng, own, a)
		return
	}
	s.availGrantJoin(k, sl, pc, a, &gi, ng, own, mask, n)
}

// availGrantJoin is availGrant's census of a dual source's grants beside its
// own ability: the tap is one ability of the join's colours and the larger
// amount, so the units join (one unit of the larger amount, the colours
// unioned), a printed or granted sacrifice-only part is a unit of its own
// beside it, and a one-colour source (a printed manaOne ability or a grant's
// "N mana of any one color") reads as one colour, whichever is chosen
// (oneColourPays), as availFor's printed path does. gi holds the granted
// abilities' indices, ng how many are live.
func (s *State) availGrantJoin(k *prep, sl Slot, pc *pcard, a *avail, gi *[maxGrants]int16, ng int, own bool, mask, n uint8) {
	if !own {
		mask, n = 0, 0
	}
	ownFace := grantUnit{mask: mask, n: n, one: own && pc.manaOne}
	// A printed sacrifice-only ability (an Eldrazi Spawn's) costs no {T} either:
	// it is a unit of its own beside a tap grant's, and a tapped source still
	// pays it, though not the tap grant (CR 602.2b, 605.1a).
	var pMask, pN uint8
	pOne := false
	spawnNoTap := false
	if own && k.sacOnlyMana && pc.manaSacOnly {
		pMask, pN = mask, n
		pOne = pc.manaOne
		ownFace = grantUnit{}
		spawnNoTap = s.perms[sl].flags&pfTapped != 0 || s.activationSick(k, sl)
	}
	// A sacrifice-only grant (Lotus Ring's "Sacrifice this creature: Add three
	// mana of any one color") costs no {T}: the source taps for its other
	// ability, then is sacrificed (CR 602.2, 605.1a), so that grant is a unit
	// of its own beside the tap's, the sacrifice made once.
	var su, gu grantUnit
	ownSac := own && k.sacOnlyMana && pc.manaSacOnly
	tapsNot := false
	if ownSac {
		// the mirror shape: a printed sacrifice-only ability (Eldrazi Spawn's
		// {C}) beside a granted {T} ability (Path of Annihilation's any
		// colour) -- the tap makes its mana, then the sacrifice makes its own
		// (CR 602.2b, 605.1a); a tapped or summoning-sick source has only the
		// sacrifice (CR 302.6)
		su = ownFace
		tapsNot = s.perms[sl].flags&pfTapped != 0 || s.activationSick(k, sl)
	}
	for i := 0; i < ng; i++ {
		gv := s.grantView(k.gen, gi[i], sl)
		if tapsNot && !gv.grantSac {
			continue
		}
		if gv.grantSac && s.grantSacLive(sl, gv) {
			su.add(s.srcMaskAt(gv, sl), gv.manaN, gv.manaOne)
			continue
		}
		if spawnNoTap {
			continue // a tapped or summoning-sick Spawn cannot pay a granted {T} ability (CR 302.6)
		}
		gu.add(s.srcMaskAt(gv, sl), gv.manaN, gv.manaOne)
	}
	u := mergeGrantFace(ownFace, gu)
	if u.mask == 0 {
		u, su = su, grantUnit{}
	}
	s.addGrantUnit(a, u)
	s.addGrantUnit(a, su)
	if pMask != 0 {
		s.addGrantUnits(a, pMask, pN)
		if pOne && pN > 1 && pMask == uint8(kop.ColAny) {
			s.addOneColour(a, pN) // the printed sacrifice-only amount is one colour, whichever is chosen
		}
	}
	s.availGrantExtra(k, sl, pc, gi, ng, own, a)
}

// availGrantExtra adds the extra mana a tap of sl brings (mana_tapextra.go) to
// a: gorge prices it on the printed ability, the planner's alternative, and the
// ability a prefix search activates floats the same extra, so the unit carries it
// once. The extra counts only while every ability of the source agrees on it
// (a carrier limited to a {C} tap reads the pick), and only a source whose own
// ability is usable carries it: the relaxed alternatives of a granted ability
// bring none (potential_plan.go paymentPlanRelaxedAlternatives), and a source
// with nothing but a grant is not priced here, so it stalls.
// grantsAdmitted keeps the first ng granted mana abilities of the source at sl
// that d may spend (a restricted grant refused by d is no unit of the window),
// returning how many remain.
func (s *State) grantsAdmitted(k *prep, sl Slot, gi *[maxGrants]int16, ng int, d *payDesc) int {
	n := 0
	for i := 0; i < ng; i++ {
		if r := k.gen.grantOf[gi[i]]; k.gen.grantRestr[gi[i]] != 0 && !s.restrSrcAdmits(k, s.perms[sl].ctrl, sl, &k.t.Cards[r.card].Abilities[r.ab], d) {
			continue
		}
		gi[n] = gi[i]
		n++
	}
	return n
}

// walkHandAvail is the potential walk's census for a hand alternative-cost
// payment d: availFor with each granted mana ability whose spend restriction
// refuses d left out (CR 106.6: Mm'menon's granted blue spends only on a spell
// not cast from the hand; gorge potentialRestrictionAdmits).
func (s *State) walkHandAvail(k *prep, p uint8, d payDesc, a *avail) {
	a.xd, a.xdOn = d, true
	s.availFor(k, p, a)
	a.xdOn = false
}

// castGrantAvail is the census o's plain hand cast is priced against: av (the
// player's shared census) unless the table has both a restricted granted mana
// ability and a tap's extra, else the census filtered by the cast's own
// payment into fav, with a restricted grant the cast cannot spend left out and
// with it a tap's extra on its source (CR 106.6, 605.3a; gorge
// potentialManaFor(payment) and potentialRestrictionAdmits). Without a
// tap-extra carrier grantCap removes the over-counted play afterwards
// (script_gen.go); it skips every play when one exists, so only then does the
// gate itself have to.
func (s *State) castGrantAvail(k *prep, p uint8, o Obj, av, fav *avail) *avail {
	if k.gen == nil || !k.gen.anyTapExtra || !k.gen.anyGrantRestr() {
		return av
	}
	fav.dynSac = av.dynSac // the rebuilt census keeps a dynamic {T}, sacrifice source the offer counts
	s.walkHandAvail(k, p, s.grantPayDescOf(k, MakeAction(AkCast, TCode(o), 0, 0)), fav)
	fav.delve = av.delve
	return fav
}

func (s *State) availGrantExtra(k *prep, sl Slot, pc *pcard, gi *[maxGrants]int16, ng int, own bool, a *avail) {
	if !k.gen.anyTapExtra {
		return
	}
	if !own && ng > 0 && s.g.availX&availWalk != 0 {
		return // the relaxed census: a gap's alternative carries none
	}
	var xv [kop.NumManaKinds]uint8
	hit := false
	if pc.manaXv != xv {
		// an all-at-once source (mana_eachcolour.go): its rest comes with its
		// own ability only, so beside a granted one its units cannot say which
		if ng > 0 {
			s.capFull(StallTapExtra)
			return
		}
		for c, n := range pc.manaXv {
			if n > 0 {
				a.addUnits(1<<c, n)
			}
		}
	}
	if own || ng == 0 {
		xv, hit = s.srcExtraHit(k, sl, pc)
	}
	for i := 0; i < ng; i++ {
		if i == 0 && !(own || ng == 0) {
			xv, hit = s.srcExtraHit(k, sl, s.grantView(k.gen, gi[i], sl))
			continue
		}
		gx, ghit := s.srcExtraHit(k, sl, s.grantView(k.gen, gi[i], sl))
		if ghit != hit || gx != xv {
			hit, xv = true, [kop.NumManaKinds]uint8{}
		}
	}
	if !hit {
		return
	}
	if xv == [kop.NumManaKinds]uint8{} || !s.tapExtraPlain(pc) || pc.painCol != 0 {
		s.capFull(StallTapExtra)
		return
	}
	for c := range xv {
		if xv[c] > 0 {
			a.addUnits(1<<c, xv[c])
		}
	}
}

// availUngranted is p's availability over the pool and the printed mana
// sources only (affordsUngranted's walk), for an {X} witness: gorge's
// planner pays the largest X the PRINTED sources fund (a granted ability is
// no witness source; potentialModeCastPlan).
//
//go:noinline
func (s *State) availUngranted(k *prep, p uint8, a *avail) {
	a.total, a.n, a.nOne = 0, 0, 0
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < kop.NumManaKinds; i++ {
		if pl.pool[i] > 0 {
			a.addUnits(1<<i, pl.pool[i])
		}
	}
	var deferGap slotMask
	if s.deferLive(k) && s.g.availX&(availWalk|availHyp) == 0 {
		deferGap = s.scriptTapsDefer(k, SeatIx(p)) // as availFor: gorge's census never lists a planner-deferred source (a counter rider, a Taps watcher), so the witness never pays with it
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		pm := &s.perms[sl]
		pc := s.manaPCGen(k, sl)
		// A sacrifice-only mana source (an Eldrazi Spawn token, a Gold token) is
		// a census unit of the {X} witness too, tapped or untapped alike (CR
		// 605.1a, 602.2b; gorge paymentPlanManaUnitsOnlyCompute lists it and the
		// planner taps it as the plan's last resort, mana_sacsource.go):
		// availForMode's census admits it (mana.go), so this one must too, or
		// the witness prices an {X} one short and the payment taps one source
		// fewer than gorge floats (Lunar Frenzy's X beside the Gold token).
		sacOnly := !DebugSacManaAfterPay && s.canSacOnlyForMana(k, sl, pc)
		// A life-only mana source (Treasonous Ogre's "Pay 3 life: Add {R}") is
		// a census unit of the {X} witness too: its ability costs no {T}, so
		// the source pays it tapped or untapped alike (CR 605.1a), and
		// availForMode's census admits it beside a grant as this one must, or
		// the witness prices an {X} one short and the payment taps one source
		// fewer than gorge floats. canLifeOnlyMana keeps the lethal guard
		// (lifeGapSurvives' CR 119.4 read): a payment that would reach the
		// payer's life is no witness source.
		lifeOnly := !DebugXWindowSkipLifeOnly && k.painMana && s.canLifeOnlyMana(k, sl, SeatIx(p))
		// a granted basic land type's intrinsic unit is the deferred source's
		// exception (blgIntrinsicUnit): gorge's census lists it like any
		// printed source's (CR 305.6), so the witness reads it beside the
		// source's own abilities
		if pm.flags&pfTapped != 0 && !sacOnly && !lifeOnly || s.manaGranted(k, sl) || deferGap.has(sl) && s.planDeferred(k, SeatIx(p), sl) && !s.blgIntrinsicUnit(k, sl) {
			continue
		}
		if pc.manaDyn && pc.manaSac {
			continue // gorge's witness census (paymentPlanManaUnitsOnlyCompute) prices no evaluated-amount source whose cost sacrifices it: the {T} probe needs a TapOnly shape
		}
		if pc.manaAb < 0 && pm.ovr.flags&ovCopy == 0 || !s.canTapForManaPC(k, sl, pc) && !sacOnly && !lifeOnly {
			continue
		}
		if k.painMana && pc.painCol != 0 {
			s.addPainAvail(k, sl, pc, SeatIx(p), a, nil, false) // the planner's census has a pain ability (Spire of Industry's any colour for 1 life), as ungrAvail counts it
			continue
		}
		a.addUnits(s.srcMaskAt(pc, sl), s.srcYieldAt(k, sl, pc))
		if pc.manaOne {
			s.addOneColour(a, pc.manaN)
		}
		s.addReflectOne(k, a, sl, pc)
		if k.gen.anyTapExtra {
			xv := s.srcExtra(k, sl, pc) // the planner prices a tap's extra too
			for c := range xv {
				if xv[c] > 0 {
					a.addUnits(1<<c, xv[c])
				}
			}
		}
	}
}

// phasedScriptNodes is the phased run's payer-state bound: the phased order
// also branches each free source's colour asks and each fee-carrying gap's
// fee, so the 17-step script the plain order never reaches is the 187th
// distinct payer state. Like potentialScriptNodes it is a stated limit, not a
// proof; the ask branches between states are not charged, matching gorge's
// clone count (scriptSearch.try clones per answer, scriptSearch.visit charges
// the clone).
const phasedScriptNodes = potentialScriptNodes + potentialScriptNodes/4

// grantSrch is one prefix search (gorge scriptSearch under gapsOnly): the
// granted sources of p in battlefield order, the play's mana cost, the
// node budget and the visited payer states, and the found prefix.
type grantSrch struct {
	k    *prep
	p    uint8
	cost kop.ManaCost
	// full is the play's cost before a waterbend offer credit and dyn marks
	// that the credit depends on this node's pool (a paid filter's fee, a
	// once-a-turn ability already used): visit prices full again at every
	// node (gorge's potentialPlayVerdict -> withWaterbendOfferCredit).
	full kop.ManaCost
	dyn  bool
	// cheaper: a repriced leaf asked less mana than the root price (a trigger
	// the path answered moved a card the price reads, Gran-Gran's loot
	// discarding a Lesson into the graveyard, CR 601.2f): grantCap offers the
	// walk play the root price refused (grantCostCheaper)
	cheaper bool
	// offer is a Power-up activation's charge when CR 702.193b lowers it
	// under the planner's price cost (hasOffer): gorge's search succeeds at a
	// node whose own priority walk offers the play, and that walk prices the
	// reduced charge against the floating pool (powerUpOffer)
	offer    kop.ManaCost
	hasOffer bool
	ex       Slot // the play's own {T} source (an activation's), never tapped
	// d is the payment the play makes, read by a granted source's spend
	// restriction (Mm'menon's Spell.!wasCastFromYourHand): the payment read,
	// the cast's origin known and the spell on the stack when paid
	d        payDesc
	n        int
	cand     [MaxPerm]Slot
	idle     slotMask                  // holders whose granted mana ability no step could activate at the root (tapped or sick): gorge's census covers their printed sacrifice, so it is no gap (printedSacPass)
	cobj     [MaxPerm]Obj              // the candidate's object at the root: a sacrificed one's slot may hold another (mana_grantcost.go)
	gi       [MaxPerm][maxGrants]int16 // each candidate's granted abilities (grantPC indices)
	ng       [MaxPerm]uint8
	dual     [MaxPerm]bool // the candidate's own ability is tried first
	skipPool bool          // the next branch's mana is restricted past the play (a printed ability or a grant): the step is taken, its mana prices nothing
	// ghost: the units those steps floated, which price nothing but sit in gorge's
	// pool, so its payer-state key tells a path that made them from one that did not
	ghost [kop.NumManaKinds]uint8
	// filt: the candidate also has a filter ability ({1},{T}: Add one mana of any
	// color), a paid census gap of gorge's like the grants (mana_filter.go)
	filt [MaxPerm]bool
	// tapx: the candidate's own ability is a tap-another-permanent one, a census
	// gap of gorge's beside the grants (mana_granttapx.go)
	tapx [MaxPerm]bool
	// sacx: the candidate's own ability is a sacrifice-another mana shape
	// (kop.SacXMana), a census gap of gorge's beside the grants
	// (mana_grantsacx.go); a grant holder keeps the grant side
	sacx [MaxPerm]bool
	// def: the candidate is a deferred source (Pyromancer's Goggles), a census
	// gap of gorge's beside the grants (mana_grantdeferred.go)
	def [MaxPerm]bool
	// each: the candidate is an each-colour gap (Tarnation Vista), a census gap
	// of gorge's beside the grants (mana_granteach.go)
	each [MaxPerm]bool
	// tdef: the candidate is a Taps-deferred gap (Peat Bog's rider), a census
	// gap of gorge's beside the grants (mana_granttapsdefer.go)
	tdef [MaxPerm]bool
	// wide: the candidate is a wide second-ability gap (Hermitic Herbalist),
	// a census gap of gorge's beside the grants (mana_grantwide.go)
	wide [MaxPerm]bool
	// ctr: the candidate is a counter-cost gap (Wall of Roots), a census gap
	// of gorge's beside the grants (mana_countercost.go)
	ctr [MaxPerm]bool
	// store: the candidate is a storage battery (Mercadian Bazaar), a census
	// gap of gorge's beside the grants (mana_grantstore.go)
	store [MaxPerm]bool
	// gap: the candidate is a census gap. Non-gap sources join the search only
	// to fund a paid gap's fee, as many as funding allows (gorge
	// scriptSearch.funding: census.fees()).
	gap     [MaxPerm]bool
	gapN    int
	funding int
	funded  int
	nodes   int
	nseen   int
	seen    [potentialScriptNodes]grantKey
	np      int
	path    [2 * MaxPerm]grantStep
	// use is the play's printed mana cost, read by the fee-only repeatable
	// filter's doomed-colour prune (filtFeeColStalls) at a colour ask.
	use   kop.ManaCost
	useOK bool

	// stall: the first stall a node of the search latched on its fork
	// (stallOf), StallNone none; the caller declares it where it finally drops
	// the play
	stall uint8
	// sp0 is the stack at the root: a deeper one is a trigger a prefix step
	// caused, which gorge's search lets resolve (a pass) once its activations are
	// spent. tapsLive: a tap can fire such a trigger (Gran-Gran's "whenever this
	// becomes tapped, draw a card, then discard a card"), so those steps run on real
	// transitions (settle).
	sp0      uint8
	tapsLive bool
	// need is the colours the play's printed base announces (gorge's
	// scriptNeedColours, potential_script.go): a colour ask's answers branch
	// on those first (branch, filtBranch), the mirror of gorge's
	// scriptOrderAnswers, so a budget-capped search reaches a colour the
	// play needs before the colours it does not. It only reorders a colour
	// ask's answers, never the answer set.
	need [kop.NumManaKinds]bool
	// sel is the pass of a source's several-ability ask (visit, needPass):
	// 1 branches only the answers that name a needed colour, 2 the others
	// (selColours). 0: the ask is not split.
	sel uint8
	// pver is the play's own object's version at the root (Obj.ver): the play
	// stands only while its card has not changed zones (waitsForStack)
	pver uint8
	// limited: a node was refused for the spent budget (gorge's
	// scriptSearch.limited); subsume: this is the subsume run, whose colour
	// asks keep only the colours costCol still lacks (mana_grantsubsume.go)
	limited, subsume bool
	costCol          [kop.NumManaKinds]uint8
	// phased: this run walks gorge's phased order (potential_script.go
	// scriptSearch.phased): the gaps whose mana ability costs no mana, then
	// the covered funding sources, then the fee-carrying gaps. phasedScope
	// gates it to a play whose own cost sacrifices or returns a permanent
	// (or one whose gaps include a hybrid-fee mana ability).
	phased bool
	// feeOnly: the gaps include a repeatable fee-only filter (gorge's
	// scriptSearch.feeOnly). A plain run that exhausted its budget on one is
	// the case gorge's phased fallback takes outside phasedScope.
	feeOnly bool
	// feeOrder: this run walks gorge's fee order (mana_grantfeeorder.go): the
	// covered funding, then the fee gaps (feeGap), then the other gaps.
	// wantFeeOrder asks the next grantSearchRun for it (set by grantSearch).
	feeOrder, wantFeeOrder bool
	feeGap                 [MaxPerm]bool
	// visitNode marks the node charge of a visit entry: the phased run's
	// budget counts payer states there, not the ask branches between them.
	visitNode bool
}

// grantKey identifies a payer state (gorge scriptStateKey: the pool and
// each own permanent's tapped bit).
type grantKey struct {
	tapped slotMask
	pool   [kop.NumManaKinds]uint8
	// art: the pool slots holding an artifact's mana (genState.poolArt):
	// gorge's key reads the typed units apart (Player.ManaUnits), so a
	// filter's tagged {W} over a plain {W} is a new state
	art uint8
	// ghost: the units an unspendable step floated (grantSrch.ghost)
	ghost [kop.NumManaKinds]uint8
	// gorge's key counts the stack, hand and graveyard too (scriptStateKey)
	sp, hand, gy uint8
	// bf: p's battlefield objects (gorge's key lists each by id): a token a
	// sacrifice's dies trigger made is a new state (mana_grantcost.go)
	bf uint64
}

// folded is the key as gorge's scriptStateKey reads it: the unspendable
// units counted in the pool, whose restriction its key does not carry.
func (k grantKey) folded() grantKey {
	for c := range k.pool {
		k.pool[c] += k.ghost[c]
	}
	k.ghost = [kop.NumManaKinds]uint8{}
	return k
}

// grantStep is one prefix activation: candidate i tapped for n mana of
// colour col.
type grantStep struct {
	i   uint8
	col uint8
	n   uint8
	gi  int16 // the grantPC index tapped; -1 the candidate's own printed ability; stepFilt, stepFund
	sl  Slot
	// kind: stepAct (a tap), or the pass that lets the trigger a tap caused resolve
	// and the answer act gives to the ask that trigger poses
	kind  uint8
	act   Action
	cand  Slot // stepTapX: the permanent its cost taps; stepSacX: the one it sacrifices
	cand2 Slot // stepTapX: the second permanent of a two-permanent cost (NoSlot: none)
	pain  bool // the source's painful printed ability (mana_grantpain.go)
	sac   bool // a granted sacrifice-only ability: the source was sacrificed (mana_grantcost.go)
	// restr: a restricted grant's mana the play may spend, floated into the
	// pool; a filter's fee spends it only where the restriction admits the
	// filter's activation (filtBranch)
	restr bool
}

const (
	stepAct uint8 = iota
	stepPass
	stepAnswer
)

// exHolds reports whether the play's own {T} source (g.ex) holds a granted mana
// ability: the candidates loop skips it, so it is the one holder it never counts.
func (g *grantSrch) exHolds(s *State) bool {
	if g.ex == NoSlot {
		return false
	}
	var gi [maxGrants]int16
	return s.grantIdxs(g.k, g.ex, &gi) > 0
}

// stepFilt is the gi of a prefix step that is a candidate's filter ability (col the
// colour it makes, its fee paid from the pool).
const stepFilt int16 = -2

// filtFirstFreeFee reports a first-and-only filter whose fee gorge's
// unrelaxable census does not count: it has a sound relaxed alternative, so
// it is no `fund` entry, while an unrelaxable census has already cleared the
// `paid` list that would have carried the fee. A hybrid or repeatable fee is
// no free unit (its `fund` entry is counted), and a relaxable census counts
// the fee in `paid`. The helper is read only where the census is unrelaxable
// (filtFunding).
func (s *State) filtFirstFreeFee(k *prep, sl Slot) bool {
	if !k.gen.anyFiltFirst || !s.filtFirstOnly(k, sl) {
		return false
	}
	if _, _, hyb := s.filtHyb(k, sl); hyb {
		return false
	}
	return !s.filtRepeat(k, sl)
}

// candidates lists p's permanents with a granted mana ability (granted only, or
// beside their own) in battlefield order. On a board where a grant is in force the
// filters join them: gorge's census misses a filter's paid ability as it does a
// granted one, so one search activates the gaps of both in battlefield order (the
// funding gorge's search also allows, other sources tapped to pay a fee, is not
// modelled here).
func (g *grantSrch) candidates(s *State) {
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(g.p, s.ctrlM[SeatIx(g.p)].and(s.occ), order[:], 0)
	var fm slotMask
	if k := g.k; k.gen.anyFilt {
		fm = s.scriptFilt(k, g.p)
	}
	g.feeOnly = s.filtRepeatIn(g.k, fm)
	tm := s.grantTapXGaps(g.k, g.p)
	dm := s.scriptDeferred(g.k, g.p)
	em := s.scriptGaps(g.k, g.p) // (mana_granteach.go)
	wm := s.wideGaps(g.k, g.p)   // (mana_grantwide.go)

	xm := s.tapsDeferGrantGaps(g.k, g.p, &g.d) // (mana_granttapsdefer.go)
	cm := s.scriptCtr(g.k, g.p)                // (mana_countercost.go)
	sm := s.grantStoreGaps(g.k, g.p)           // (mana_grantstore.go)
	si := s.grantStoreIdleGaps(g.k, g.p)
	am := s.grantSacXGaps(g.k, g.p) // (mana_grantsacx.go)
	g.n = 0
	holders := 0
	plain := g.d.hasPlay && gapFilteredPlay(g.d.play)
	var dropped slotMask
	for i := 0; i < n; i++ {
		sl := order[i]
		if sl == g.ex {
			continue
		}
		ng := s.grantIdxs(g.k, sl, &g.gi[g.n])
		smi := sm
		if ng > 0 && si.has(sl) {
			smi = sm.with(sl) // an idle battery beside its grant: the printed ask answer first
		}
		if ng == 0 && !fm.has(sl) && !tm.has(sl) && !dm.has(sl) && !em.has(sl) && !wm.has(sl) && !xm.has(sl) && !cm.has(sl) && !smi.has(sl) && !am.has(sl) {
			continue
		}
		if ng > 0 {
			holders++ // (a dropped holder still makes this the grant search's play)
		}
		if plain && !s.gapSpendable(g.k, sl, &g.d) {
			// gorge's prefix search drops a gap whose mana the play
			// cannot spend (scriptSpendableGaps); its fee still counts
			if fm.has(sl) {
				g.funding += s.filtFee(g.k, sl)
			}
			dropped = dropped.with(sl) // (nor a funding source: it makes no mana the play spends)
			continue
		}
		g.gapFlags(s, g.n, sl, ng, fm, tm, dm, em, wm, xm, cm, smi, am)
		if ng > 0 && !s.grantGapUsable(g.k, sl) {
			g.idle = g.idle.with(sl)
		}
		g.gap[g.n] = true
		if fm.has(sl) {
			g.funding += s.filtFee(g.k, sl)
		}
		g.n++
	}
	if holders == 0 && (g.n == 0 || !g.exHolds(s)) {
		// No permanent but the excluded one holds a grant. When that is the
		// activation's own source (g.ex), its {T} cannot also make the mana,
		// so no grant funds it, but another gap kind (a Taps-deferred source,
		// a filter) still can, and gorge's one prefix search activates it:
		// keep those candidates. With no grant held at all this is not the
		// grant search's board (the filter search plays it), and with no gap
		// surviving the printed sources pay.
		g.n, g.gapN = 0, 0
		return
	}
	// A paid gap's fee is funded by other sources (gorge scriptSearch's
	// funding loop: the gaps first, then non-gaps while funding lasts). The
	// non-gaps sit after every gap in battlefield order, exactly the walk
	// order that loop reads.
	g.gapN = g.n
	if g.gapN == 0 {
		return // every gap dropped: gorge's search runs on none
	}
	if !s.filtFunded(g.k, g.p, fm) {
		g.funding = s.filtFunding(g.k, g.p, fm) // (gorge's unrelaxable census still funds what it cannot relax: a repeatable one unbounded, a hybrid at fee x 160)
	}
	if g.funding == 0 {
		return
	}
	seen := func(sl Slot) bool {
		for i := 0; i < g.n; i++ {
			if g.cand[i] == sl {
				return true
			}
		}
		return false
	}
	for i := 0; i < n; i++ {
		sl := order[i]
		if sl == g.ex || seen(sl) || dropped.has(sl) || !s.canTapForMana(g.k, sl) {
			continue
		}
		g.cand[g.n], g.ng[g.n], g.dual[g.n], g.filt[g.n], g.tapx[g.n], g.def[g.n] = sl, 0, true, false, false, false
		g.each[g.n], g.wide[g.n], g.tdef[g.n], g.ctr[g.n], g.store[g.n] = false, false, false, false, false
		g.gap[g.n] = false
		g.n++
	}
}

// gapFlags sets candidate n's gap kinds for slot sl, out of the masks one
// search's candidates lists: the kinds exclude the later ones, so a source
// with several gap kinds branches as the first that matches. A holder of a
// granted ability (ng > 0) branches on the grant side, except a rider-first
// source whose printed rider is a Taps-deferred gap: that rider is tried
// first, as gorge's "choose a mana ability" ask lists the printed ability
// before the grant.
func (g *grantSrch) gapFlags(s *State, n int, sl Slot, ng int, fm, tm, dm, em, wm, xm, cm, sm, am slotMask) {
	g.cand[n], g.ng[n], g.cobj[n], g.filt[n], g.tapx[n] = sl, uint8(ng), s.perms[sl].obj, fm.has(sl), tm.has(sl)
	// a sacrifice-another source (mana_grantsacx.go): a holder's is an answer
	// of its ability ask beside the grants (visit), another's its own gap step
	// (gapBranch)
	g.sacx[n] = am.has(sl)
	if ng > 0 {
		g.dual[n] = (!tm.has(sl) && !am.has(sl) || g.k.tapXSecond(s.cardAt(sl))) && (s.ownManaUsable(g.k, sl) || s.ownRestrAb(g.k, sl) != nil)
		// A rider gap beside its grants (Rain of Filth's sacrifice mana on the
		// rider land): its own production is no planner unit (availGrant counts
		// the grant alone), so the search taps the rider beside the grants --
		// first the rider step (gapBranch), then the grants' (visit), never
		// both taps of one source (CR 605.1a). Gorge's "choose a mana ability"
		// ask lists the printed rider first, so the search tries the rider's
		// step before the grant, and the grant side follows a failed rider
		// (gapBranch's tdef arm reports not-a-gap then), at every node
		// (Sandstone Needle beside Rain of Filth: the rider's {R}{R} pays the
		// play, not the grant's sacrifice — and a play the rider cannot pay
		// prices the Needle's own grant, not another land's).
		g.tdef[n] = xm.has(sl) && !fm.has(sl) && !tm.has(sl)
		g.each[n], g.wide[n], g.ctr[n] = false, false, false
		// a battery that is itself a grant holder keeps both sides: its
		// counters pay this slot (CR 107.3), its granted {T} the grants'
		g.store[n] = sm.has(sl) && !fm.has(sl) && !tm.has(sl)
		return
	}
	g.dual[n] = (!tm.has(sl) || g.k.tapXSecond(s.cardAt(sl))) && s.ownManaUsable(g.k, sl)
	g.each[n], g.wide[n], g.tdef[n], g.ctr[n], g.store[n] = false, false, false, false, false
	if g.sacx[n] && !fm.has(sl) {
		// a sacrifice-another source (Krark-Clan Ironworks): its step is the
		// sacrifice ask and its fixed production, no printed {T} beside it
		// (a filter beside it is the printed ability tried first)
		g.dual[n] = false
		return
	}
	if dm.has(sl) {
		g.def[n] = true // (mana_grantdeferred.go)
		return
	}
	if sm.has(sl) && !fm.has(sl) && !tm.has(sl) {
		g.store[n] = true // (mana_grantstore.go)
		return
	}
	switch {
	case em.has(sl) && !fm.has(sl) && !tm.has(sl):
		g.each[n] = true // (mana_granteach.go)
	case wm.has(sl) && !fm.has(sl) && !tm.has(sl) && !em.has(sl):
		g.wide[n] = true // (mana_grantwide.go)
	case xm.has(sl) && !fm.has(sl) && !tm.has(sl) && !em.has(sl) && !wm.has(sl):
		g.tdef[n] = true // (mana_granttapsdefer.go)
	case cm.has(sl) && !fm.has(sl) && !tm.has(sl) && !em.has(sl) && !wm.has(sl) && !xm.has(sl):
		// the dead and storage shapes stay on the script route (a counter-scaled sacrifice source is a step: mana_sacctr.go): a dead gap
		// makes no mana and a storage battery's X is announced, neither a
		// branch of this search
		a := g.k.ctrAbOf(s.cardAt(sl))
		g.ctr[n] = !deadCtrAb(a) && (!kop.StoreManaAb(a) || g.k.ctrSecond(s.cardAt(sl))) // (mana_countercost.go, mana_sacctr.go; a later storage battery beside its plain {T} is a chain, ctrBranch)
	}
}

// tappedOf is the tapped bits of p's permanents.
func (s *State) tappedOf(p uint8) slotMask {
	var m slotMask
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		if s.perms[b.first()].flags&pfTapped != 0 {
			m = m.with(b.first())
		}
	}
	return m
}

// bfObjKey hashes p's battlefield objects, each with its slot and counters
// (FNV-1a): gorge's scriptStateKey lists each permanent's id and counters, so
// an Army a resolved amass grew is a new payer state though its stack, hand
// and graveyard sizes are those of an earlier node (CR 701.47a).
func (s *State) bfObjKey(p uint8) uint64 {
	h := uint64(14695981039346656037)
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		pm := &s.perms[sl]
		h = (h ^ uint64(sl)<<16 ^ uint64(pm.obj)) * 1099511628211
		h = (h ^ uint64(pm.p1)) * 1099511628211
		for i := range pm.cn {
			h = (h ^ uint64(pm.ck[i])<<16 ^ uint64(pm.cn[i])) * 1099511628211
		}
	}
	return h
}

// node charges one search node; false when the budget is spent.
func (g *grantSrch) node() bool {
	if g.phased && !g.visitNode {
		// gorge's phased run spends its budget on cloned payer states: its
		// scriptSearch.try takes one clone per answer and scriptSearch.visit
		// charges the node at the clone. The kernel also charges each ask
		// branch separately (the choose-a-mana-ability and colour asks), so a
		// phased run that charged both would spend gorge's 160 clones on half
		// the states. The phased order charges payer states alone.
		return true
	}
	limit := potentialScriptNodes
	if g.phased {
		limit = phasedScriptNodes
	}
	if g.nodes >= limit {
		g.limited = true
		return false
	}
	g.nodes++
	return true
}

// gapBranch runs candidate i's own gap kind's branch of the search (slot sl),
// found when one found the play; is false when the candidate is none of the
// per-kind gaps (its grant side, a filter or a tap-another one branches
// elsewhere in visit).
func (g *grantSrch) gapBranch(t *State, i int, sl Slot) (found, is bool) {
	switch {
	case g.sacx[i] && g.ng[i] == 0:
		// the sacrifice ask, then the production (mana_grantsacx.go)
		if !t.scriptTapX(g.k, g.p).has(sl) {
			return false, true // tapped, spent or its candidates gone since the root
		}
		return g.sacXBranch(t, i, sl), true
	case g.ctr[i]:
		return g.ctrBranch(t, i, sl), true // its counter cost, its colour ask's answers (mana_countercost.go)
	case g.wide[i]:
		return g.wideBranch(t, i, sl), true // its two mana abilities' answers (mana_grantwide.go)
	case g.each[i]:
		return g.eachBranch(t, i, sl), true // its chosen colour, or its fee-paid colours (mana_granteach.go)
	case g.def[i]:
		return g.deferBranch(t, i, sl), true // its plain {R} (mana_grantdeferred.go)
	case g.tdef[i] && g.ng[i] > 0:
		// its rider step beside the grants (gapFlags): the step before the
		// grants' own; failing it leaves the grant side to visit
		if g.tapsDeferBranch(t, i, sl) {
			return true, true
		}
		return false, false
	case g.tdef[i]:
		// its rider or deferred tap (mana_granttapsdefer.go). A grant holder's
		// failed rider reports not-a-gap so visit runs its grant side: gorge's
		// "choose a mana ability" ask lists the printed rider first, the
		// granted ability after (Sandstone Needle beside Rain of Filth, whose
		// {R}{R} cannot pay the {B} play — the plan taps the Needle's rider
		// and sacrifices the NEEDLE over its grant, not another land's grant),
		// and a node the rider's tap reached runs the tapped source's
		// sacrifice-only grant alone (grantSacLive, mana_grantcost.go)
		if found := g.tapsDeferBranch(t, i, sl); found || g.ng[i] == 0 {
			return found, true
		}
		return false, false
	case g.store[i]:
		// every counter it holds (mana_grantstore.go); a failed storage branch
		// of a grant holder leaves its grant side to visit (CR 605.1a)
		if g.storeBranch(t, i, sl) {
			return true, true
		}
		if g.ng[i] > 0 {
			return false, false
		}
		return false, true
	}
	return false, false
}

// grantFirstGaps reports the gap kinds beside a grant whose one search runs
// before the script search (applyPriority): a deferred, each-colour, wide,
// Taps-deferred, counter-cost or sacrifice-another gap among the grants is one
// prefix search in gorge (mana_grantdeferred.go, mana_granteach.go,
// mana_grantwide.go, mana_granttapsdefer.go, mana_countercost.go,
// mana_grantsacx.go: a grant holder before Phyrexian Altar in battlefield
// order is activated first, which the tap-another search's noGrant walk
// skips).
func (s *State) grantFirstGaps(k *prep, p uint8) bool {
	return s.deferBesideGrant(k, p) || s.eachBesideGrant(k, p) || s.wideBesideGrant(k, p) ||
		s.tdBesideGrant(k, p) || s.ctrBesideGrant(k, p) || s.storeBesideGrant(k, p) ||
		s.sacXBesideGrant(k, p) // gorge's one search taps the granted lands after the sacrifice too (mana_grantsacx.go, mana_grantlive.go)
}

// phasedScope reports whether gorge's phased order may price this search's
// play (scriptSearch.phasedScope): a cast whose own cost sacrifices or
// returns a permanent (CR 601.2b: the activation comes before the sacrifice,
// so the funding must float first), or one whose gaps include a hybrid-fee
// mana ability (paid only from mana already floating, CR 601.2g). For any
// other play the plain order is the one the search mirrors.
func (g *grantSrch) phasedScope(s *State) bool {
	if !g.d.hasPlay || !isCastKind(g.d.play.Kind()) {
		return false
	}
	cd := &g.k.t.Cards[s.castFaceCard(g.k, g.d.play)]
	const sacOrReturn = kop.CostSac | kop.CostSacSelf | kop.CostReturn | kop.CostReturnSelf
	if cd.SpellCost.Flags&sacOrReturn != 0 || cd.AltAddCost.Flags&sacOrReturn != 0 {
		return true
	}
	for ai := range cd.Abilities {
		// An instant's or sorcery's additional cost rides on its Spell ability
		// (kgen build: cb.spellCost), not CardDef.SpellCost.
		if a := &cd.Abilities[ai]; a.Kind == kop.AbSpell && a.Cost.Flags&sacOrReturn != 0 {
			return true
		}
	}
	for i := 0; i < g.gapN; i++ {
		if g.filt[i] {
			if _, _, ok := s.filtHyb(g.k, g.cand[i]); ok {
				return true
			}
		}
	}
	return false
}

// gapFree reports whether candidate i has a mana ability that costs no mana:
// gorge's scriptSearch.payable "free" arm decides the phased order's first
// pass. The source, not the chosen option, is the phase: a source with a free
// grant beside a fee-carrying filter is free.
func (g *grantSrch) gapFree(t *State, i int) bool {
	sl := g.cand[i]
	if t.grantTapOK(g.k, sl) || t.grantSacOK(g.k, sl) {
		return true
	}
	return g.k.sacOnlyMana && t.canSacOnlyNow(g.k, sl, t.manaPCGen(g.k, sl))
}

// gapVisit is visit's per-gap body: candidate i's branches, found when one
// found the play, stop when the node budget is spent. The caller walks the
// candidates in the plain order or the phased one (visit).
func (g *grantSrch) gapVisit(t *State, i int, sacd bool, sacStepped *slotMask) (found, stop bool) {
	sl := g.cand[i]
	if g.ng[i] == 0 && g.filt[i] && g.k.gen.anyFiltFirst && t.filtFirstOnly(g.k, sl) {
		// a filter that is its source's only mana ability: no printed
		// ability to ask beside, one colour ask once its fee is in the pool
		if t.filtUsable(g.k, sl) && t.filtPaid(g.k, g.p, sl) && g.filtBranch(t, i, sl) {
			return true, false
		}
		return false, false
	}
	if found, is := g.gapBranch(t, i, sl); is {
		if found {
			return true, false
		}
		return false, false
	}
	tx := (g.tapx[i] || g.sacx[i]) && t.scriptTapX(g.k, g.p).has(sl) // (a holder's printed tap- or sacrifice-another ability)
	if sacd && (!t.occ.has(sl) || t.perms[sl].obj != g.cobj[i]) {
		return false, false // sacrificed by an earlier step (mana_grantcost.go)
	}
	grants := t.grantTapOK(g.k, sl)
	fok := g.filt[i] && t.filtPaid(g.k, g.p, sl)
	if !grants && !tx {
		if g.tappedHolderStep(t, i, sl, sacStepped) { // no {T} in its cost: tapped or sick alike (mana_grantcost.go)
			return true, false
		}
		if !(fok && t.filtUsable(g.k, sl)) {
			return false, false // (a summoning-sick Changeling's filter stays: only its granted {T} ability is out, CR 302.6)
		}
	}
	// several abilities (its own beside a grant, two grants): the
	// "choose a mana ability" ask, one node, the printed ability's
	// answers first (gorge flattens a Combo's colours into that ask;
	// an Any ability still asks its colour)
	alts := g.printedAlts(t, i, sl, grants, fok, tx)
	if alts > 1 && !g.node() {
		return false, true
	}
	if tx && g.printedXFirst(t, i, sl) { // the printed tap- or sacrifice-another ability first
		return true, false
	}
	if !grants {
		if fok && g.filtBranch(t, i, sl) {
			return true, false
		}
		return false, false
	}
	return g.gapAbilities(t, i, sl, grants, fok, tx, alts)
}

// gapAbilities is gapVisit's "choose a mana ability" walk: the source's own
// and granted abilities in gorge's answer order, found when one found the
// play, stop when the node budget is spent.
func (g *grantSrch) gapAbilities(t *State, i int, sl Slot, grants, fok, tx bool, alts int) (found, stop bool) {
	// gorge's "choose a mana ability" ask lists the answers of the source's
	// abilities flattened (a Combo's colours each, an Any ability one
	// option that asks its colour after) and tries first the ones that name
	// a colour the play needs (scriptOrderAnswers): a granted {U} before
	// the printed any-colour ability's answers when the play needs {U}
	lo, hi := uint8(0), uint8(0)
	if alts > 1 && !tx && !fok && g.needPass(t, i, sl) {
		lo, hi = 1, 2
	}
	for g.sel = lo; g.sel <= hi; g.sel++ {
		blgExtra, blgN := uint8(0), uint8(0)
		if ppc := t.manaPCGen(g.k, sl); g.dual[i] && !t.ownRestrPrinted(g.k, sl) && grantPainShape(g.k, ppc) {
			if g.painBranch(t, i, sl, ppc, alts == 1) { // its painful ability's colours too (mana_grantpain.go)
				return true, false
			}
		} else if g.dual[i] {
			mask, n := uint8(0), uint8(0)
			if ra := t.ownRestrAb(g.k, sl); ra != nil && !t.ownManaUsable(g.k, sl) {
				mask, n = uint8(ra.ManaColors)&0x3f, t.restrAbN(g.k, sl, ra) // the restricted printed ability
				// gorge's search taps it first whether or not the play may spend
				// it; mana the play cannot spend prices nothing
				g.skipPool = !t.restrSrcAdmits(g.k, g.p, sl, ra, &g.d)
			} else {
				ppc := t.manaPCGen(g.k, sl)
				mask, n = t.srcMaskAt(ppc, sl), ppc.manaN
				if base := &g.k.cards[t.cardAt(sl)]; g.k.gen.blgOn && ppc != base && base.manaAb >= 0 && !base.manaChosen && mask&^srcMask(base) != 0 {
					// a granted basic land type's intrinsic abilities (CR 305.7)
					// follow the printed ones in gorge's ask (its printed
					// restricted and filter abilities too): White Lotus
					// Hideout's {C} first, an Island's {U} before the grants
					blgExtra, blgN = mask&^srcMask(base), n
					mask, n = srcMask(base), base.manaN
				}
			}
			if ppc := t.manaPCGen(g.k, sl); g.k.sacOnlyMana && ppc.manaSacOnly {
				// a printed sacrifice-only ability (an Eldrazi Spawn's) beside a
				// tap grant: the step sacrifices the source (CR 605.1a)
				if g.sacBranch(t, i, sl, -1, ppc, mask, alts == 1) {
					return true, false
				}
			} else if g.branch(t, i, sl, -1, mask, n, alts == 1) {
				return true, false
			}
		}
		if g.dual[i] && t.ownManaUsable(g.k, sl) {
			// a restricted printed ability beside the usable one (White Lotus
			// Hideout's Lesson-only any colour): gorge's search taps it too, after
			// the usable one, whether or not the play may spend it
			if ra := t.ownRestrAb(g.k, sl); ra != nil {
				g.skipPool = !t.restrSrcAdmits(g.k, g.p, sl, ra, &g.d)
				if g.branch(t, i, sl, -1, uint8(ra.ManaColors)&0x3f, t.restrAbN(g.k, sl, ra), ra.ManaColors&kop.ColAny == kop.ColAny) {
					return true, false
				}
			} else if wn := t.gatedPairWide(g.k, sl); wn > 0 && g.sel != 1 && grants {
				// Whisperer of the Wilds' ferocious {T}: Add {G}{G} beside its
				// {G} and the grant: the ask's second option, one colour and
				// no colour ask (mana_gatedpairgrant.go)
				if g.branchSel(t, i, sl, -1, t.gatedPairMask(g.k, sl), wn, false, 0) {
					return true, false
				}
			} else if t.wideSecond(g.k, sl) && g.sel != 1 && g.wide2Branch(t, i, sl) {
				// the wide second ability beside the usable one (Hermitic
				// Herbalist's Lesson-only two mana): its option follows the
				// printed pair's first ability in gorge's ask, the grant's
				// colours after (CR 106.6, 601.2g)
				return true, false
			}
		}
		if fok && g.filtBranch(t, i, sl) { // (listed after the printed ability, before the grants)
			return true, false
		}
		if blgExtra != 0 && g.branch(t, i, sl, -1, blgExtra, blgN, false) {
			return true, false
		}
		for j := 0; j < int(g.ng[i]); j++ {
			gi := g.gi[i][j]
			pc := t.grantView(g.k.gen, gi, sl)
			mask := t.srcMaskAt(pc, sl)
			if !t.grantCostPayable(g.p, pc) {
				continue // its life cannot be paid (CR 119.4)
			}
			// gorge's search taps a granted census gap whether or not the play may
			// spend it (the mana floats, the tap fires its triggers); mana the
			// play's origin or class cannot spend prices nothing
			g.skipPool = !g.grantAdmits(t, sl, gi)
			ask := alts == 1 || mask&uint8(kop.ColAny) == uint8(kop.ColAny)
			if g.grantStepBranch(t, i, sl, gi, pc, mask, ask) {
				return true, false
			}
		}
	}
	g.sel = 0
	return false, false
}

// fundingPass is visit's funding loop: a paid gap's fee is funded by other
// sources (gorge scriptSearch's funding loop: the gaps first, then non-gaps
// while funding lasts).
func (g *grantSrch) fundingPass(t *State) (found, stop bool) {
	var tried [MaxPerm]Slot
	ntried := 0
	twinCut := g.d.hasPlay && isAbilityPlay(g.d.play)
	for i := g.gapN; i < g.n && g.funded < g.funding; i++ {
		sl := g.cand[i]
		if t.perms[sl].flags&pfTapped != 0 {
			continue
		}
		pc := t.manaPCGen(g.k, sl)
		if pc.manaAb < 0 || !t.canTapForManaPC(g.k, sl, pc) {
			continue
		}
		mask, n := t.srcMaskAt(pc, sl), pc.manaN
		if mask == 0 {
			continue
		}
		if twinCut && g.fundTwinTried(t, sl, tried[:ntried]) {
			continue // an indistinguishable earlier funder already walked this subtree (gorge's scriptTwin)
		}
		tried[ntried] = sl
		ntried++
		g.funded++
		// The filter's own card taps here through its plain ability, whose
		// one-bit mask poses no ask of its own; charge the zero-filter ask
		// now, as the filter search does before its one answer
		// (mana_filtzerofund.go).
		if zeroFiltTap(g.k, t, g.p, sl) {
			if !g.node() {
				g.funded--
				return false, true
			}
		}
		var ok bool
		if painFunds(g.k, pc) {
			ok = g.fundPainBranch(t, i, sl, pc) // a painland asks its painful colours too (mana_filtpain.go)
		} else {
			ok = g.branch(t, i, sl, -1, mask, n, mask&(mask-1) != 0)
		}
		g.funded--
		if ok {
			return true, false
		}
	}
	return false, false
}

// visit is one payer priority of the search on working copy t: the play
// is found when the pool and the printed sources pay it (gorge
// offered / priced); else each untapped candidate is activated in turn,
// its answers branched on in gorge's order, each payer state once.
func (g *grantSrch) visit(t *State) bool {
	g.stallOf(t) // (the stall the step into this node latched on its fork; the node's own probes are the root's too)
	g.visitNode = true
	ok := g.node()
	g.visitNode = false
	if !ok {
		return false
	}
	if g.playSourceGone(t) {
		return false
	}
	cost := g.cost
	cheap := false // this node's price asks less than the root's (set on success only)
	if g.dyn {
		cost = t.grantDynamicCost(g.k, g.p, g.d.play, g.full)
	}
	if g.d.hasPlay && isCastKind(g.d.play.Kind()) && g.answered() {
		// gorge's planner prices the play again at every node
		// (potentialPlayVerdict, scriptSearch.priced): a trigger the path
		// answered can have moved a card the price reads (Gran-Gran's loot
		// discarding a Lesson into the graveyard, CR 601.2f). Price again on
		// this copy; when the fresh price asks less mana than the root price,
		// the play is payable where the walk's root price refused it
		// (grantCostCheaper), and grantCap keeps the play on the offer. The
		// flag is set at the success returns below, never here: a node or a
		// branch that fails must not leave a later root-priced success looking
		// repriced
		if fc, _, _, ok := t.grantPlayCost(g.k, g.p, g.d.play, true); ok && grantCostCheaper(fc, g.cost) {
			cost = fc
			cheap = true
		}
	}
	pl := &t.pl[SeatIx(g.p)]
	key := grantKey{tapped: t.tappedOf(g.p), pool: pl.pool, ghost: g.ghost, sp: t.sp, hand: pl.handN, gy: pl.graveN, bf: t.bfObjKey(g.p)}
	if g.k.artMana {
		t.artSync(g.p)
		key.art = t.g.poolArt[SeatIx(g.p)]
	}
	// gorge's key reads the floated pool, whose restriction it does not
	// (scriptStateKey): a state that differs from a seen one only in which
	// pool units are the play's unspendable ones (ghost) is that state, and
	// gorge drops it before pricing. Its offered check comes before the seen
	// check, though: the priority options price a cast against the floating
	// pool alone (legalActions), so a unit that is spendable here where the
	// seen state's was not still offers the play when the pool pays it. A
	// state equal in the ghost too is the same payer state in gorge's key
	// (the key reads the same pool, tapped bits, counters, stack, hand and
	// graveyard sizes, and neither the cards a discard moved nor which card
	// went), so it is dropped the same way: Gran-Gran's loot discarding the
	// cast card first shadows the discard of any other card, and the printed
	// sources the planner would add at the pricing are not offered
	// (scriptSearch.visit: offered, then seen, then priced).
	fk := key.folded()
	for i := 0; i < g.nseen; i++ {
		if g.seen[i].folded() == fk {
			return g.seenPays(t, &cost, cheap)
		}
	}
	if g.leafPays(t, &cost, cheap) {
		return true
	}
	g.remember(key)
	sacd, sacStepped := g.sacrificed(), slotMask{}
	if g.feeOrder {
		if found, stop := g.feeOrderPasses(t, sacd, &sacStepped); found {
			return true
		} else if stop {
			return false
		}
	} else if g.phased {
		// gorge's phased order (potential_script.go scriptSearch.activations):
		// the gaps whose mana ability costs no mana first (their mana is the
		// play's for one step), then the covered funding sources the
		// fee-carrying gaps' fees come from, then the gaps that carry a fee.
		// A fee-carrying gap activated before the funding drains the pool the
		// prefix still needs (CR 601.2b).
		var tried [MaxPerm]Slot
		ntried := 0
		for i := 0; i < g.gapN; i++ {
			if !g.gapFree(t, i) {
				continue
			}
			if g.gapTwinSkip(t, i, &tried, &ntried) {
				continue // an indistinguishable earlier free gap already walked this subtree (gorge's scriptTwin)
			}
			if found, stop := g.gapVisit(t, i, sacd, &sacStepped); found {
				return true
			} else if stop {
				return false
			}
		}
		if g.printedSacPass(t, sacd, sacStepped) { // a tapped holder's printed sacrifice (mana_grantcost.go)
			return true
		}
		if found, stop := g.fundingPass(t); found {
			return true
		} else if stop {
			return false
		}
		var tried2 [MaxPerm]Slot
		ntried2 := 0
		for i := 0; i < g.gapN; i++ {
			if g.gapFree(t, i) {
				continue
			}
			if g.gapTwinSkip(t, i, &tried2, &ntried2) {
				continue // an indistinguishable earlier fee gap already walked this subtree (gorge's scriptTwin)
			}
			if found, stop := g.gapVisit(t, i, sacd, &sacStepped); found {
				return true
			} else if stop {
				return false
			}
		}
	} else {
		var tried3 [MaxPerm]Slot
		ntried3 := 0
		for i := 0; i < g.gapN; i++ {
			if g.gapTwinSkip(t, i, &tried3, &ntried3) {
				continue // an indistinguishable earlier gap already walked this subtree (gorge's scriptTwin)
			}
			if found, stop := g.gapVisit(t, i, sacd, &sacStepped); found {
				return true
			} else if stop {
				return false
			}
		}
		if g.printedSacPass(t, sacd, sacStepped) { // a tapped holder's printed sacrifice (mana_grantcost.go)
			return true
		}
		if found, stop := g.fundingPass(t); found {
			return true
		} else if stop {
			return false
		}
	}
	if t.sp > g.sp0 && g.np < len(g.path) {
		// a trigger a step caused: let it resolve, the pool floats (gorge
		// scriptSearch.visit, the last resort at a node)
		pass := MakeAction(AkPass, 0, 0, 0)
		c := t.fork()
		c.Apply(pass)
		g.path[g.np] = grantStep{kind: stepPass, act: pass, sl: NoSlot}
		g.np++
		ok := g.settle(c)
		t.unfork()
		if !ok {
			g.np--
		}
		return ok
	}
	return false
}

// twinCutOn is gorge's twin-cut scope for the search's branches (scriptTwin,
// run on the fee-only path and for the plays potentialTwinCut classifies as
// printed or gained -- isAbilityPlay here): the funding pass applies it too
// (fundingPass).
func (g *grantSrch) twinCutOn() bool {
	return g.feeOnly || g.d.hasPlay && isAbilityPlay(g.d.play)
}

// gapTwinSkip reports whether the search's twin cut skips gap candidate i at
// this node: gorge's scriptTwin runs over the activation options too, an
// indistinguishable earlier source's subtree covering sl's (the same mana for
// the same cost, tail-08 pair 78: twelve lands granted "{T}: Add one mana of
// any color" beside The World Tree's search -- the plain run's walk over the
// twelve as distinct subtrees spent the budget on their permutations before
// it reached the paying colour assignment, so the offer fell one activation
// short of gorge's). Only an untapped, tap-branchable candidate is tried
// (scriptTwin needs both sources untapped; a candidate with no tap to branch
// is no option of gorge's menu, so it neither cuts nor cuts for another --
// gapTapBranchable), and the cut reads only the cut's own scope (twinCutOn).
func (g *grantSrch) gapTwinSkip(t *State, i int, tried *[MaxPerm]Slot, ntried *int) bool {
	if !g.twinCutOn() {
		return false
	}
	sl := g.cand[i]
	if t.perms[sl].flags&pfTapped != 0 || !t.occ.has(sl) || !g.gapTapBranchable(t, i, sl) {
		return false
	}
	if g.fundTwinTried(t, sl, tried[:*ntried]) {
		return true
	}
	tried[*ntried] = sl
	*ntried++
	return false
}

// gapTapBranchable reports whether candidate i's branch at this node goes
// through a tap: the granted tap or the printed one (grantTapOK reads both),
// the tap- or sacrifice-another ability (scriptTapX) or the filter's
// (filtPaid with a usable fee). The no-tap census gaps (a counter-cost Wall
// of Roots, a storage battery, a deferred source) branch without a tap and
// keep their whole subtrees.
func (g *grantSrch) gapTapBranchable(t *State, i int, sl Slot) bool {
	if t.grantTapOK(g.k, sl) {
		return true
	}
	if (g.tapx[i] || g.sacx[i]) && t.scriptTapX(g.k, g.p).has(sl) {
		return true
	}
	return g.filt[i] && t.filtPaid(g.k, g.p, sl) && t.filtUsable(g.k, sl)
}

// fundTwinTried reports whether an earlier funder of this pass is
// indistinguishable from sl (twinSlotOf): the branch through sl reaches no
// payer state the earlier one's does not (gorge's scriptTwin, which an
// activated play's prefix search runs over its funding sources too).
func (g *grantSrch) fundTwinTried(t *State, sl Slot, tried []Slot) bool {
	for _, a := range tried {
		if twinSlotOf(t, a, sl) {
			return true
		}
	}
	return false
}

// printedAlts counts the answers a holder offers beside its grants in the
// "choose a mana ability" ask: its printed {T} ability (g.dual[i], one more
// when it is a gate-on pair's larger alternative or a restricted ability
// beside the usable one), its filter's colour ask (fok) and its tap- or
// sacrifice-another ability (tx); with no grants the grants alone.
func (g *grantSrch) printedAlts(t *State, i int, sl Slot, grants, fok, tx bool) int {
	alts := 0
	if grants {
		alts = t.grantAlts(g.k, g.p, sl, &g.gi[i], int(g.ng[i]))
	}
	if g.dual[i] && (!tx || t.canTapForMana(g.k, sl)) { // (a tapped Relic of Legends asks only its tap-another ability)
		alts++
		if grants && t.gatedPairWide(g.k, sl) > 0 {
			alts++ // the gate-on pair's larger {T} ability is an answer of the ask too (mana_gatedpairgrant.go)
		}
		if t.ownManaUsable(g.k, sl) && t.ownRestrAb(g.k, sl) != nil {
			alts++ // (the restricted printed ability beside the usable one is an answer of the ask)
		}
	}
	if fok {
		alts++
	}
	if tx {
		alts++
	}
	return alts
}

// remember records key as a seen node, while the table has room.
func (g *grantSrch) remember(key grantKey) {
	if g.nseen < len(g.seen) {
		g.seen[g.nseen] = key
		g.nseen++
	}
}

// playSourceGone: the searched activation's source left the battlefield on
// this path (a granted sacrifice ability paid with the enchanted land sends
// its Aura to the graveyard), so no continuation offers the play and gorge's
// search ends the line at the node (potential_script.go leftOwnZone, CR
// 602.2b). The node is charged, as there, but nothing under it is expanded.
func (g *grantSrch) playSourceGone(t *State) bool {
	if !g.d.hasPlay || g.d.play.Kind() != AkActivate {
		return false
	}
	o := Obj(g.d.play.Source())
	return o != NoObj && int(o) < len(t.objs) && t.objs[o].ver != g.pver
}

// waitsForStack: the play is sorcery-speed and the path left a trigger of its own
// on the stack, so the play is not offered until it resolves (CR 307.1, 602.5d;
// gorge scriptSearch.waitsForStack) and the search continues through a pass.
func (g *grantSrch) waitsForStack(t *State) bool {
	if !g.d.hasPlay {
		return false
	}
	if o := Obj(g.d.play.Source()); o != NoObj && int(o) < len(t.objs) && t.objs[o].ver != g.pver {
		// the play's own card left the zone it is played from (a Mechan
		// Navigator tap's draw-then-discard binning the library-top spell): a
		// new object (CR 400.7), so gorge's search finds no such play
		return true
	}
	// a stack the prefix grew, a sacrificed source (the play's only target,
	// or the equipped creature: mana_grantcost.go), or a trigger's answer that
	// may have moved the play's own card (Gran-Gran's loot discarding the
	// spell: gorge prices a cast from a card no longer in its origin zone as
	// unsupported)
	if t.sp <= g.sp0 && !g.answered() && !g.sacrificed() {
		return false
	}
	var d Decision
	t.plainOptions(g.k, &d, 0)
	return !hasOption(&d, g.d.play)
}

// answered: the path holds an answer to a trigger's ask.
func (g *grantSrch) answered() bool {
	for i := 0; i < g.np; i++ {
		if g.path[i].kind == stepAnswer {
			return true
		}
	}
	return false
}

// settle continues the search at t after a pass or a tap whose trigger is
// queued: t runs to p's next decision, a priority being a node (visit) and an
// ask the trigger poses (the discard of Gran-Gran's loot) a node branching on its
// answers (gorge scriptSearch.visit's scriptAnswers).
func (g *grantSrch) settle(t *State) bool {
	var d Decision
	if !t.tapXNext(g.p, &d) {
		g.stallOf(t) // a latched stall ends the fork's line here
		g.node()     // gorge's visit charges the clone before it finds no decision of the payer's: each answer of a loot's discard ask (Gran-Gran) is a node
		return false
	}
	if d.Kind == DkPriority {
		return g.visit(t)
	}
	if !g.node() {
		return false
	}
	if d.Kind == DkTarget {
		t.gorgeTargetOrder(&d)
	}
	if !gorgeAskOrder(&d) {
		return false // (Jett Reno's surveil: the card to the graveyard first)
	}
	for i := 0; i < int(d.N) && g.np < len(g.path); i++ {
		c := t.fork()
		c.Apply(d.Options[i])
		g.path[g.np] = grantStep{kind: stepAnswer, act: d.Options[i], sl: NoSlot}
		g.np++
		ok := g.settle(c)
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// next continues the search after sl's tap: a tap that fires a trigger runs on a
// copy whose trigger is stacked, as does a step whose own cost queued one (Haunted
// Screen's 1 life heard by Marina Vendrell's Grimoire, CR 119.4, 603.3).
func (g *grantSrch) next(t *State, sl Slot) bool {
	taps := g.tapTriggers(t, sl)
	if !taps && t.npend == 0 {
		return g.visit(t)
	}
	u := t.fork()
	if taps {
		u.manaTapped(g.k, sl)
	}
	ok := g.settle(u)
	t.unfork()
	return ok
}

// tapTriggers reports whether tapping sl may fire a trigger the search stacks
// before the next activation (a TapAll listener hears any permanent's tap:
// trig_tapall.go).
func (g *grantSrch) tapTriggers(t *State, sl Slot) bool {
	return g.tapsLive && (t.subs[kop.TrigTaps].has(sl) || g.k.gen.anyTapAll)
}

// branch tries tapping candidate i (slot sl) for each colour of mask in
// W U B R G C order: a choice of colours is an ask node when ask (gorge's
// colour ask, one node, every answer branched on).
//
// A step floats its tap's extra (mana_tapextra.go), whatever kind of effect
// granted the ability it taps (gorge adds the extra to every tapped source).
func (g *grantSrch) branch(t *State, i int, sl Slot, gi int16, mask, n uint8, ask bool) bool {
	sel := g.sel
	g.sel = 0 // the search below is whole again
	ok := g.branchSel(t, i, sl, gi, mask, n, ask, sel)
	g.sel = sel
	return ok
}

// needPass reports whether source i (slot sl) with several mana abilities
// asks its answers in two passes (sel): its alternatives all go through
// branch, none a pain, sacrifice or life step.
func (g *grantSrch) needPass(t *State, i int, sl Slot) bool {
	any := false
	for _, b := range g.need {
		any = any || b
	}
	if !any {
		return false
	}
	if g.dual[i] && !t.ownRestrPrinted(g.k, sl) && grantPainShape(g.k, t.manaPCGen(g.k, sl)) {
		return false
	}
	for j := 0; j < int(g.ng[i]); j++ {
		if pc := t.grantView(g.k.gen, g.gi[i][j], sl); pc.grantSac || pc.manaSac || grantCostLife(pc) > 0 {
			return false
		}
	}
	return true
}

// selColours keeps of cols the answers pass sel of the ask walks (gorge's
// scriptOrderAnswers: an option naming a needed colour first): pass 1 the
// colours the play needs, pass 2 the others. An any-colour ability's
// option names no colour, so it waits for pass 2 with all of its colours.
func (g *grantSrch) selColours(cols []uint8, mask, sel uint8, named bool) []uint8 {
	if !named {
		// no option names a colour (stepNamed): all wait for pass 2, in engine order
		if sel == 1 {
			return nil
		}
		return cols
	}
	anyAb := mask&uint8(kop.ColAny) == uint8(kop.ColAny)
	if anyAb {
		if sel == 1 {
			return nil
		}
		return cols
	}
	out := cols[:0]
	for _, c := range cols {
		if g.need[c] == (sel == 1) {
			out = append(out, c)
		}
	}
	return out
}

// branchSel is branch walking only pass sel's answers (0: all of them).
func (g *grantSrch) branchSel(t *State, i int, sl Slot, gi int16, mask, n uint8, ask bool, sel uint8) bool {
	if mask == 0 {
		mask = 1 << kop.C
	}
	skip := g.skipPool // the printed restricted ability's unspendable mana (mana_grantrestrictedown.go)
	g.skipPool = false
	if sel == 1 && mask&uint8(kop.ColAny) == uint8(kop.ColAny) {
		return false // an any-colour ability's option names no colour: its ask waits for pass 2
	}
	if ask && mask&(mask-1) != 0 && !g.node() {
		return false
	}
	pn := n
	if skip {
		pn = 0
	}
	var pc *pcard
	if g.k.gen.anyTapExtra {
		pc = t.stepView(g.k, sl, gi)
	}
	var order [kop.NumManaKinds]uint8
	no := t.stepColours(g.k, sl, gi, mask, &order)
	named := t.stepNamed(g.k, sl, gi, mask)
	if named {
		needFirst(order[:no], g.need)
	}
	cols := g.subsumeCols(t, order[:no], ask, named)
	switch {
	case sel != 0 && g.grantHoisted(t, sl, gi, mask):
		if sel == 2 {
			return false // walked in pass 1, ahead of the printed abilities (mana_grantnamed.go)
		}
	case sel != 0:
		cols = g.selColours(cols, mask, sel, named)
	}
	for _, c := range cols {
		t.perms[sl].flags |= pfTapped
		preBit := t.g.poolArt[SeatIx(g.p)] // the typed-mana tag before the step: the untag restores exactly it (the slot may hold plain mana too)
		t.pl[SeatIx(g.p)].pool[c] += pn
		t.artTag(g.k, g.p, sl, int(c))
		xv := t.srcExtraPick(g.k, sl, pc, int(c))
		var gx [kop.NumManaKinds]uint8 // the unspendable tap prices no extra either; its units sit in the ghost
		if skip {
			gx, xv = xv, [kop.NumManaKinds]uint8{}
			gx[c] += n
			for j := range gx {
				g.ghost[j] += gx[j]
			}
		}
		t.addXv(SeatIx(g.p), &xv)
		g.path[g.np] = grantStep{i: uint8(i), col: c, n: n, gi: gi, sl: sl, restr: gi >= 0 && !skip && g.k.gen.grantRestr[gi] != 0}
		g.np++
		if g.next(t, sl) {
			return true
		}
		g.np--
		for j := range gx {
			g.ghost[j] -= gx[j]
		}
		t.subXv(SeatIx(g.p), &xv)
		t.pl[SeatIx(g.p)].pool[c] -= pn
		t.g.poolArt[SeatIx(g.p)] = preBit
		t.perms[sl].flags &^= pfTapped
	}
	return false
}

// stepColours lists the colours of mask a prefix step on sl may answer, in the
// order gorge's ask offers them: a Combo ability's printed order (Sun-Blessed
// Peak's "R or W"; manaColourOrder), else W U B R G C. gi is the granted
// ability the step taps, -1 for the source's own.
func (s *State) stepColours(k *prep, sl Slot, gi int16, mask uint8, out *[kop.NumManaKinds]uint8) (n int) {
	cd := s.cardAt(sl)
	var pc *pcard
	if gi < 0 {
		pc = s.manaPCGen(k, sl)
	}
	ab := s.stepAbility(k, sl, gi, pc)
	var seen uint8
	if gi < 0 && pc.blgFirst != 0 {
		// a land with a granted basic type: gorge lists the face's own abilities
		// before the intrinsic ones the type adds (appendAvailableManaAbilities),
		// so a Forest made an Island asks G then U, not U then G; a printed
		// Combo asks its token order (Kyoshi Village's Combo G W asks G first)
		if own := &k.cards[cd]; own.manaAb >= 0 {
			if ownAb := manaAbView(&k.t.Cards[cd], own); ownAb != nil {
				var ord [5]uint8
				for _, c := range ord[:manaColourOrder(ownAb, &ord)] {
					if uint8(own.manaCol)&mask&(1<<c) != 0 {
						out[n] = c
						n++
						seen |= 1 << c
					}
				}
			}
		}
	}
	if ab != nil && ab.ManaColOrd != 0 && uint8(ab.ManaColors)&0x1f == mask&0x1f {
		var ord [5]uint8
		for _, c := range ord[:manaColourOrder(ab, &ord)] {
			out[n] = c
			n++
			seen |= 1 << c
		}
	}
	if gi < 0 && pc.manaMulti && k.gen.anyVergeOrder {
		// a gated Verge's ability 0 comes before its lower colour (mana_gate.go)
		if c0, ok := s.vergeAbility0Col(k, sl); ok && c0 < kop.NumManaKinds && mask&(1<<uint(c0)) != 0 && seen&(1<<uint(c0)) == 0 {
			out[n] = uint8(c0)
			n++
			seen |= 1 << uint(c0)
		}
	}
	for c := uint8(0); c < kop.NumManaKinds; c++ {
		if mask&(1<<c) != 0 && seen&(1<<c) == 0 {
			out[n] = c
			n++
		}
	}
	return n
}

// filtBranch is the filter's colour ask at candidate i (slot sl): its fee comes out
// of the pool, then the colours in turn (gorge's askManaColor, mana_filter.go).
func (g *grantSrch) filtBranch(t *State, i int, sl Slot) bool {
	each := t.filtAb(g.k, sl).ManaEach // a fixed production: no colour ask, one branch (mana_filtereach.go)
	if !each && !g.node() {
		return false
	}
	var order [5]uint8
	n := manaColourOrder(t.filtAb(g.k, sl), &order)
	if n == 0 && t.filtFixed(g.k, sl) {
		// a {C}-only fixed filter (Apprentice Wizard's {U}, {T}: Add {C}{C}{C}): the
		// order names WUBRG only, but the one answer still activates it
		order[0], n = uint8(bits.TrailingZeros8(uint8(t.filtAb(g.k, sl).ManaColors))), 1
	}
	needFirst(order[:n], g.need)
	if each {
		n = 1
	}
	var doomed uint8
	if n > 1 && g.useOK {
		// A fee-only repeatable filter's colour that fills no fixed pip the
		// play still owes after the fee is paid cannot bring the play closer
		// to paid: activating it only converts pool mana into a colour it
		// could make without the fee (CR 605.3a; gorge's
		// scriptSearch.feeOnlyAskPrune, potential_script.go). Without the
		// prune the search spends its node budget on those branches and
		// starves the grant line that pays the play (Darksteel Forge beside
		// Elven Chorus's granted any-colour mana and Prismite's {2} filter).
		for j := 0; j < n; j++ {
			if t.filtFeeColStalls(g.k, g.p, sl, &g.use, int(order[j]), g.ex) {
				doomed |= 1 << order[j]
			}
		}
		if bits.OnesCount8(doomed) == n {
			return false
		}
	}
	for j := 0; j < n; j++ {
		c := order[j]
		if doomed&(1<<c) != 0 {
			continue
		}
		u := t.fork()
		// the restricted mana the filter's activation may not spend stands
		// aside while its fee is paid (CR 106.6: gorge pays an activation's
		// cost from the mana its restriction admits, so Ambulatory Reactor's
		// artifact-only {C} pays no land's fee)
		barred := g.feeBarred(u, sl)
		for c, n := range barred {
			u.pl[SeatIx(g.p)].pool[c] -= n
		}
		if !u.filtPaid(g.k, g.p, sl) {
			t.unfork()
			return false
		}
		u.filtDo(g.k, g.p, filtStep{sl: sl, gap: true, ans: 1 + c})
		for c, n := range barred {
			u.pl[SeatIx(g.p)].pool[c] += n
		}
		g.path[g.np] = grantStep{i: uint8(i), col: c, gi: stepFilt, sl: sl}
		g.np++
		var ok bool
		if g.tapsLive {
			ok = g.settle(u) // (the filter's tap may have fired a trigger)
		} else {
			ok = g.visit(u)
		}
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// feeBarred is the restricted grant mana the prefix floated (grantStep.restr)
// that the activation of filter sl may not spend: it stays in the pool for
// the play, but its fee is paid without it.
func (g *grantSrch) feeBarred(t *State, sl Slot) (out [kop.NumManaKinds]uint8) {
	d := payDesc{class: pdAct, card: t.cardAt(sl), src: sl}
	for _, st := range g.path[:g.np] {
		if !st.restr {
			continue
		}
		r := g.k.gen.grantOf[st.gi]
		if !t.restrSrcAdmits(g.k, g.p, st.sl, &g.k.t.Cards[r.card].Abilities[r.ab], &d) {
			out[st.col] = min(out[st.col]+st.n, t.pl[SeatIx(g.p)].pool[st.col])
		}
	}
	return out
}

// grantAdmits: the play g.d makes may spend granted ability gi's mana (an
// unrestricted grant always; a restricted one as restrSrcAdmits reads the
// play). The search is feasibility-only, so mana the play cannot spend
// must not price it (the offer would stall the payment); the grant is still
// a gap step, tapped as gorge's search taps it (visit sets skipPool).
func (g *grantSrch) grantAdmits(t *State, sl Slot, gi int16) bool {
	if g.k.gen.grantRestr[gi] == 0 {
		return true
	}
	r := g.k.gen.grantOf[gi]
	ok := t.restrSrcAdmits(g.k, g.p, sl, &g.k.t.Cards[r.card].Abilities[r.ab], &g.d)
	if st := DebugRestrGrantStats; st != nil {
		st.Reads++
		if !ok {
			st.Denied++
		}
	}
	return ok
}

// grantPayDesc is the payment play a makes, for a granted source's spend
// restriction: a cast is read as paid (cast=true: the spell is on the
// stack when the prefix's mana is spent), an activation as an ability.
func (s *State) grantPayDesc(k *prep, a Action) payDesc {
	d := s.grantPayDescOf(k, a)
	d.play, d.hasPlay = a, true
	return d
}

func (s *State) grantPayDescOf(k *prep, a Action) payDesc {
	o := Obj(a.Source())
	switch a.Kind() {
	case AkCast, AkCastCasualty, AkCastConspire:
		og := restrOrigin{from: s.objs[o].zone}
		// a face cast's spend restriction reads the face being cast (CR
		// 715.3b, 709.3a): Storm Reading's mana value, not Galvanic Giant's
		return payDesc{class: pdSpell, card: s.castFaceCard(k, a), src: NoSlot, og: og}
	case AkActivate:
		d := payDesc{class: pdAct, card: s.objs[o].card, src: NoSlot}
		if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
			d.card, d.src = s.cardAt(ob.slot), ob.slot
		}
		return d
	}
	return payDesc{class: pdOther, src: NoSlot}
}

// grantSearch runs gorge's prefix search for p's play of cost (ex its own
// {T} source) on a copy of s: ok is false when no prefix within the node
// budget makes the pool and the printed sources pay it; np is 0 when they
// already do. On false, g.stall is the first rule-gap stall a failed line's
// fork latched (stallOf, StallNone none): the search declares nothing — the
// caller declares it only where it finally drops the play (mana_tapanother.go
// tapXScript). A plain search the budget cut short is followed by gorge's
// subsume run (grantSubsume, mana_grantsubsume.go).
//
// need is gorge's own announced set (playNeedPrinted: empty for a graveyard
// cast), so the search walks the colour asks in gorge's order and finds the
// plan gorge's does, the pool colours included (a flashback cast's Resonating
// Lute mana is W, not the cast's R). Only when that walk finds nothing, the
// budget-capped search is run again with the graveyard cast's announced pips
// (playNeed's hint, CR 601.2b), which steers it to a plan gorge's larger
// budget reaches (Eviscerator's Insight's {4}{B} flashback).
func (s *State) grantSearch(k *prep, p uint8, cost *kop.ManaCost, ex Slot, d payDesc, need [kop.NumManaKinds]bool, g *grantSrch) bool {
	if s.grantSearchRun(k, p, cost, ex, d, need, g, nil, false) {
		return true
	}
	// gorge's fallback runs: the phased order for a play whose own cost
	// sacrifices or returns a permanent or whose gaps include a hybrid fee
	// (phasedScope), or for a plain run the budget cut short on a repeatable
	// fee-only filter (potentialPrefixScript's plainLimited).
	feeOrder := g.feeOrderOf(s)
	if feeOrder || g.phasedScope(s) || g.limited && g.feeOnly {
		limited, funding, stall := g.limited, g.funding, g.stall
		g.wantFeeOrder = feeOrder
		if s.grantSearchRun(k, p, cost, ex, d, need, g, nil, true) {
			return true
		}
		g.limited, g.funding, g.stall = limited, funding, stall
	}
	if s.grantSubsume(k, p, cost, ex, d, need, g) {
		return true
	}
	if !d.hasPlay {
		return false
	}
	hint := s.playNeed(k, d.play)
	if hint == need {
		return false
	}
	return s.grantSearchRun(k, p, cost, ex, d, hint, g, nil, false) || s.grantSubsume(k, p, cost, ex, d, hint, g)
}

// grantSearchRun is one run of grantSearch: the plain one (sub nil) or the
// subsume one, whose colour asks keep only the colours *sub still lacks.
//
//go:noinline
func (s *State) grantSearchRun(k *prep, p uint8, cost *kop.ManaCost, ex Slot, d payDesc, need [kop.NumManaKinds]bool, g *grantSrch, sub *[kop.NumManaKinds]uint8, phased bool) bool {
	wantFeeOrder := g.wantFeeOrder
	*g = grantSrch{k: k, p: p, cost: *cost, ex: ex, d: d, sp0: s.sp, tapsLive: k.gen.manaTaps, need: need}
	if d.hasPlay {
		g.use, g.useOK = s.filtUsefulCost(k, d.play)
	}
	if sub != nil {
		g.subsume, g.costCol = true, *sub
	}
	if d.hasPlay {
		if full, ok := s.grantWaterbendFull(k, d.play); ok {
			g.full, g.dyn = full, true
		}
	}
	if d.hasPlay {
		if o := Obj(d.play.Source()); o != NoObj && int(o) < len(s.objs) {
			g.pver = s.objs[o].ver
		}
		if oc, ok := s.powerUpOffer(k, d.play); ok && oc != *cost {
			g.offer, g.hasOffer = oc, true
		}
	}
	g.candidates(s)
	g.phased = sub == nil && phased
	if g.phased && wantFeeOrder {
		g.feeGap, g.feeOrder = g.feeGapMarks(s)
	}
	if sub == nil && g.phasedScope(s) {
		// The graveyard-cast pip hint (playNeed's exception) is the r1
		// workaround for the missing phased run: gorge reads no moded cast's
		// pips (scriptPlayBaseCost), so its colour asks keep engine order and
		// its script floats the play's {B} through a fee-carrying filter rather
		// than the grant the hint would tap first. The phased fallback below is
		// the mirror; the play's own sacrifice is what puts it in scope.
		if _, _, ok := s.playBaseCost(g.k, g.d.play); !ok {
			g.need = [kop.NumManaKinds]bool{}
		}
	}
	if g.n == 0 {
		// no candidate to tap: the offered check still stands (the pool alone
		// pays a Power-up play's reduced charge, offerPays)
		if g.offerPays(s) {
			g.cheaper = true
			return true
		}
		return s.affordsUngrantedFor(k, p, cost, ex, d)
	}
	t := s.fork()
	defer s.unfork()
	ok := g.visit(t)
	return ok
}

// stallOf records the stall a node's fork latched (capFull) while the line ran
// to it, the first one, so the caller declares it on the real state where it
// finally drops the play instead of reporting "no payment" for one the fork
// could still have paid along: the fork cannot go on (its next stepGen ends
// the game), but the play could have been payable along it. The search
// declares nothing — a caller that keeps searching may still price the play
// (grantCap's tapXScript, grantPrefix's grantTapXPrefix, tapXScriptOffered's
// fall-through); the offer walk declares it only where the drop is final.
func (g *grantSrch) stallOf(t *State) {
	if g.stall == 0 {
		g.stall = t.g.stalled
	}
}

// grantApply taps the found prefix into p's pool on s (the play's
// announcement: the mana floats from here, as the oracle's prScript
// activations do).
func (s *State) grantApply(k *prep, g *grantSrch) {
	n, passed := s.Decisions, false
	var d Decision
	for i := 0; i < g.np; i++ {
		st := &g.path[i]
		if st.kind != stepAct {
			// the pass and the trigger's answers, each as the decision then pending
			// (the other player passes for the trigger to resolve)
			passed = true
			if s.tapXNext(g.p, &d) && hasOption(&d, st.act) {
				s.Apply(st.act)
			}
			continue
		}
		sl := st.sl
		switch st.gi {
		case stepTapX:
			s.tapXDo(k, g.p, tapXStep{sl: sl, cand: st.cand, cand2: st.cand2, col: st.col, kind: tapXAct})
			continue
		case stepSacX:
			// the sacrifice and its production, the asked colour of an
			// any-colour one (mana_grantsacx.go); the sacrifice's dies
			// triggers queue, and the search's settle stacked them: gorge
			// answers each as its own decision
			s.tapXDo(k, g.p, tapXStep{sl: sl, cand: st.cand, cand2: NoSlot, col: st.col, kind: tapXAct})
			if g.tapTriggers(s, sl) || s.npend != 0 {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepWide:
			s.wideApply(k, g.p, sl, st.col) // (mana_grantwide.go)
			if g.tapTriggers(s, sl) {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepEach:
			s.eachApply(k, g.p, sl, st.col) // (mana_granteach.go)
			if g.tapTriggers(s, sl) {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepDeferred:
			s.deferApply(k, g.p, sl, int(st.col)) // (mana_grantdeferred.go)
			if g.tapTriggers(s, sl) {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepStore:
			s.storeGap(k, g.p, sl, k.ctrAbOf(s.cardAt(sl))) // (mana_grantstore.go)
			if g.tapTriggers(s, sl) {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepTapsDefer:
			s.tapsDeferApply(k, g.p, g.d.play, sl, st.col) // (mana_granttapsdefer.go)
			if g.tapTriggers(s, sl) || s.npend != 0 {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepCtr:
			s.ctrStepDo(k, g.p, sl, g.need, int(st.col)-1, true) // the {T} tap or the counters, the use, the mana (mana_countercost.go)
			if g.tapTriggers(s, sl) || s.npend != 0 {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		case stepFilt:
			s.filtDo(k, g.p, filtStep{sl: sl, gap: true, ans: 1 + st.col})
			if !s.filtNoTap(k, sl) {
				s.manaTapped(k, sl) // (a once-each-turn filter taps nothing)
			}
			continue
		}
		if st.sac {
			s.grantSacStepApply(k, g.p, st) // the sacrifice step's replay, sac-only or after the tap (mana_grantcost.go)
			if s.npend != 0 {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		}
		if st.gi >= 0 && grantCostLife(s.grantView(k.gen, st.gi, sl)) > 0 {
			s.grantCostApply(k, g.p, st) // the tap and its life (mana_grantcost.go)
			if g.tapTriggers(s, sl) || s.npend != 0 {
				passed = true
				s.tapXNext(g.p, &d)
			}
			continue
		}
		sacrifice := false
		if st.gi < 0 {
			// the source's own ability sacrifices it (a Treasure beside a grant):
			// gorge pays that cost at priority, so the token has ceased before
			// the next activation, not at the play's state-based check
			pc := s.manaPCGen(k, sl)
			sacrifice = pc.manaSac || s.canSacOnlyForMana(k, sl, pc)
		}
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		if st.gi >= 0 {
			GrantManaStats.Taps.Add(1) // the granted {T} step's mana (a life-cost step took the arm above)
		}
		if ra := s.ownRestrAb(k, sl); st.gi < 0 && ra != nil && !s.ownManaUsable(k, sl) {
			// the printed restricted ability (mana_grantrestrictedown.go)
			if !s.floatRestrAb(k, SeatIx(g.p), sl, ra, int(st.col), st.n) {
				s.capFull(StallRestrSrc)
			}
		} else if ab := restrAbOfStep(k, st); ab != nil {
			// a restricted grant floats beside the pool (State.rb), spendable only
			// as its restriction allows (both halves, spells and activated
			// abilities); never a silent spill into the pool
			if !s.floatRestrAb(k, SeatIx(g.p), sl, ab, int(st.col), st.n) {
				s.capFull(StallRestrSrc)
			}
		} else {
			s.pl[SeatIx(g.p)].pool[st.col] += st.n
			s.artTag(k, g.p, sl, int(st.col)) // the search branch's tag: a snow source's unit is snow (mana_snow.go)
		}
		if st.pain {
			s.painDamage(k, sl, SeatIx(g.p), int(s.manaPCGen(k, sl).painN)) // its life or damage (mana_grantpain.go)
		}
		if k.gen.anyTapExtra {
			xv := s.srcExtraPick(k, sl, s.stepView(k, sl, st.gi), int(st.col))
			s.addXv(SeatIx(g.p), &xv)
		}
		if sacrifice {
			// at priority, outside the build's and a resolution's mana steps, so the
			// move cannot park for gorge's order ask: two competing exile-instead
			// replacements stall it by name in dieZone (StallReplLeave, CR 616.1)
			o := s.perms[sl].obj
			s.fireSac(k, sl)
			s.leave(k, sl, kop.ZoneGraveyard, false)
			if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard {
				s.ceaseManaToken(o)
			}
		}
		if g.tapTriggers(s, sl) || s.npend != 0 {
			// the tap's trigger (or the one its paid life queued) is stacked before the next activation, as the
			// search's settle stacked it: gorge answers each activation as its own
			// decision, so two Taps triggers of one prefix (Beckett Mariner and
			// Erica Ortegas tapped for Rok-Tahk's {G}{G}) go on the stack one at a
			// time and never wait together for an order ask (CR 603.3b)
			passed = true
			s.tapXNext(g.p, &d)
		}
	}
	if passed {
		s.tapXNext(g.p, &d) // the trigger the last step queued is stacked and asked now
		s.Decisions = n
	}
}

// grantTapXPrefix plays gorge's prefix script for play a when a spell-only
// grant is the only census pricing it: the granted sources tapped in
// battlefield order, their mana floating beside the pool (State.rb), and the
// tap-another gap's step where it sits in that order (tapXScript's path), the
// play re-priced after each (gorge potential_script.go: gaps first, then
// priced; it stops at the first that prices it). The granted taps are
// dead-end taps on gorge's script — their mana cannot pay the play (CR
// 605.1a) — and the play then prices from the printed sources and the
// tap-another gap's own mana (CR 601.2g-h).
func (s *State) grantTapXPrefix(k *prep, a Action, p uint8) {
	if !s.grantsLive(k) {
		return
	}
	if found, _ := s.tapXScript(k, a, p, false); !found {
		return // no tap-another script within gorge's node budget (mana_grantedoffers.go)
	}
	cost, ex, _, ok := s.grantPlayCost(k, p, a, true)
	if !ok {
		return
	}
	desc := s.grantPayDesc(k, a)
	var gaps, tx slotMask
	tx = s.scriptTapXModelled(k, p)
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		if s.canTapForMana(k, sl) && s.grantIdx(k, sl) >= 0 {
			gaps = gaps.with(sl)
		}
	}
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, gaps.or(tx), order[:], 0)
	for i := 0; i < n; i++ {
		sl := order[i]
		if tx.has(sl) {
			s.tapXScript(k, a, p, true) // the tap-another gap's step (mana_tapanother.go)
		} else {
			s.tapGrantedGap(k, p, sl)
		}
		if s.npend != 0 {
			s.flushPrefixTriggers(k) // the tap's trigger is asked as gorge's script answers it (CR 603.3b)
		}
		if s.affordsUngrantedPlay(k, p, a, &cost, ex, desc) {
			return // gorge's search stops: the play prices now
		}
	}
}

// tapGrantedGap taps one granted mana source of p in gorge's prefix script:
// the first granted ability's mana floats beside the pool (State.rb),
// spendable only as its restriction allows (grantApply's step; never a
// silent spill), in the colour the source's ask lists first (gorge's search
// answers in order).
func (s *State) tapGrantedGap(k *prep, p uint8, sl Slot) {
	var gi [maxGrants]int16
	if ng := s.grantIdxs(k, sl, &gi); ng == 0 {
		return
	}
	pc := s.grantView(k.gen, gi[0], sl)
	var order [kop.NumManaKinds]uint8
	no := s.stepColours(k, sl, gi[0], s.srcMaskAt(pc, sl), &order)
	if no == 0 {
		return
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	n := max(pc.manaN, 1)
	if k.gen.anyTapExtra {
		xv := s.srcExtraPick(k, sl, pc, int(order[0]))
		s.addXv(SeatIx(p), &xv)
	}
	if ab := restrAbOfGrant(k, gi[0]); ab != nil {
		if !s.floatRestrAb(k, SeatIx(p), sl, ab, int(order[0]), n) {
			s.capFull(StallRestrSrc)
		}
		return
	}
	s.pl[SeatIx(p)].pool[order[0]] += n
}

// stepView is the mana ability a prefix step taps on sl: the source's own
// (gi -1) or the granted one.
func (s *State) stepView(k *prep, sl Slot, gi int16) *pcard {
	if gi < 0 {
		return s.manaPCGen(k, sl)
	}
	return s.grantView(k.gen, gi, sl)
}

// restrAbOfStep is the granted mana ability a prefix step tapped when it
// carries a spend restriction (nil: none, or the candidate's own printed
// ability).
func restrAbOfStep(k *prep, st *grantStep) *kop.Ability {
	if st.gi < 0 {
		return nil
	}
	return restrAbOfGrant(k, st.gi)
}

// restrAbOfGrant is grant gi's mana ability when it carries a spend
// restriction (nil: none), so every float of its mana names both halves of
// the restriction (spells and activated abilities).
func restrAbOfGrant(k *prep, gi int16) *kop.Ability {
	if k.gen.grantRestr[gi] == 0 {
		return nil
	}
	r := k.gen.grantOf[gi]
	return &k.t.Cards[r.card].Abilities[r.ab]
}

// grantPlayCost is the mana cost gorge's prefix search prices for p's
// play a (grantPrefix / grantCap), ok false for a play it never searches.
// need is the colours the play's printed base announces (gorge's
// scriptNeedColours, potential_script.go): empty for an alternative, a
// kicked, face-down or MayPlay cast, as gorge's scriptNeedColours leaves
// it -- the search branches a colour ask on those first (grantSrch.branch).
func (s *State) grantPlayCost(k *prep, p uint8, a Action, kicked bool) (cost kop.ManaCost, ex Slot, need [kop.NumManaKinds]bool, ok bool) {
	if a.Kind() == AkPlot && a.Mode() == 0 {
		return s.grantPlotCost(k, p, a)
	}
	if s.g.raiseO == 0 {
		// the cast owes its permission's surcharge (Zero's {2}) once it is
		// built; the search prices what the cast will pay (CR 601.2f, 118.8)
		if r := s.mayRaiseOwed(k, p, a); r != 0 {
			s.g.raiseO = r
			defer func() { s.g.raiseO = 0 }()
		}
	}
	cost, ex, need, ok = s.grantPlayCostRaise(k, p, a, kicked)
	if ok && a.Kind() == AkCast && a.Mode()&castMayPlay != 0 && a.Mode()&castFree == 0 && (s.g.mayPlayAny || s.mayPlayAnyType(k, p, Obj(a.Source()))) {
		// an any-type may-play rider: gorge's planner funds the cast under
		// the rider (pay.PlanManaUnderRider), so the printed lands pay
		// Fuss's {R/W} from Outrageous Robbery's exile and no granted
		// source is tapped first (CR 601.2g-h, 609.4b)
		cost = planUnderRider(cost, true, false)
	} else if ok && a.Kind() == AkCast && a.Mode()&castMayPlay != 0 && a.Mode()&castFree == 0 && s.mayPlayIgnoreColor(k, p, Obj(a.Source())) {
		// an any-colour may-play rider prices the coloured pips generic too
		// (pay.PlanManaUnderRider, potentialPinnedColourPlan; CR 609.4b), so
		// the printed lands pay Brainstealer Dragon's {1}{R} Underworld
		// Breach from its exile (grantCap's walk keeps the offer)
		cost = planUnderRider(cost, false, true)
	}
	return cost, ex, need, ok
}

// grantCostCheaper reports whether cost c asks strictly less mana than the
// root price root, kind for kind: a pool that pays root pays c, so a leaf
// priced at c is payable where the walk priced the play with root (visit).
// A repricing never reshapes the flexible pips (hybrid, twobrid, X, a
// reduction's excess), so costs that differ there are not comparable
// (false: the root price stands).
func grantCostCheaper(c, root kop.ManaCost) bool {
	if c.Hyb != root.Hyb || c.Tb != root.Tb || c.HybOrd != root.HybOrd ||
		c.X != root.X || c.XPip != root.XPip || c.XRed != root.XRed {
		return false
	}
	if c.Generic > root.Generic {
		return false
	}
	for i := range c.Pips {
		if c.Pips[i] > root.Pips[i] {
			return false
		}
	}
	return c != root // every kind at most root's, one strictly under
}

// castFaceCard is the card cast action a puts on the stack: for a face cast
// (an Adventure or split half from the hand, a Room's other door) the face,
// else the object's card.
func (s *State) castFaceCard(k *prep, a Action) CardID {
	c := s.objs[a.Source()].card
	if a.Mode()&castFace == 0 || a.Mode()&3 != 0 {
		return c
	}
	if cd := &k.t.Cards[c]; roomDef(k, c) {
		return cd.Faces[a.Target()] // a room_alt cast: the other door, whichever door the card shows
	} else if fd, ok := fusedDef(k, c); ok && a.Target() == TCode(FeatFuseFace) {
		return fd // a fused cast: both halves (face_fuse.go)
	} else if len(cd.Faces) == 2 && a.Target() == 1 {
		return cd.Faces[1]
	}
	return c
}

// grantPlayCostRaise is grantPlayCost with raiseO set as the caller left it.
func (s *State) grantPlayCostRaise(k *prep, p uint8, a Action, kicked bool) (cost kop.ManaCost, ex Slot, need [kop.NumManaKinds]bool, ok bool) {
	ex = NoSlot
	need, _ = s.playNeedPrinted(k, a)
	switch a.Kind() {
	case AkActivate:
		if isGainedMode(a.Mode()) {
			return cost, ex, need, false // a gained ability (static_gainsof.go): unpriced, as a granted one
		}
		if a.Mode()&0x80 != 0 {
			return cost, ex, need, false
		}
		cost, ex, ok = s.activationMana(k, Obj(a.Source()), a.Mode())
		if ok {
			cost = s.waterbendPlanCost(k, p, Obj(a.Source()), a.Mode(), cost)
		}
		return cost, ex, need, ok
	case AkCast, AkCastCasualty, AkCastConspire:
		if alt, ok := s.altPlayCost(k, p, a); ok {
			return alt, ex, need, true
		}
		if a.Mode()&castFace != 0 && a.Mode()&3 == 0 {
			// a face cast (an Adventure from the hand) pays the face's own
			// cost, not the front's: the search priced it as the front and
			// removed a cast gorge offers
			c := s.castFaceCard(k, a)
			return s.adjustCost(k, p, c, Obj(a.Source()), k.cards[c].cost), ex, need, true
		}
		if a.Mode() == 0 && a.Kind() == AkCast {
			// an impended or evoked cast pays its alternative cost (CR 118.9)
			c := s.objs[a.Source()].card
			switch cd := &k.t.Cards[c]; {
			case a.Target() == castImpTgt && cd.Impending > 0:
				return s.adjustCost(k, p, c, Obj(a.Source()), cd.ImpendingCost), ex, need, true
			case a.Target() == castEvokeTgt && cd.HasEvoke:
				return s.adjustCost(k, p, c, Obj(a.Source()), cd.EvokeCost), ex, need, true
			case a.Target() == castDashTgt && cd.HasDash:
				return s.adjustCost(k, p, c, Obj(a.Source()), cd.DashCost), ex, need, true
			case a.Target() == castOverloadTgt && cd.HasOverload:
				return s.adjustCost(k, p, c, Obj(a.Source()), cd.OverloadCost), ex, need, true
			case a.Target() == castCleaveTgt && cd.CleaveAb != 0:
				return s.adjustCost(k, p, c, Obj(a.Source()), cd.CleaveCost), ex, need, true
			case a.Target() == castSurgeTgt && cd.HasSurge:
				return s.adjustCost(k, p, c, Obj(a.Source()), cd.SurgeCost), ex, need, true
			}
		}
		if a.Mode()&1 != 0 {
			if !kicked {
				return cost, ex, need, false // kicked: lowered by its witness (beginCast)
			}
			if cost, ok = s.plannedCastCostOf(k, p, Obj(a.Source()), a.Mode()&2, false, true); !ok {
				return cost, ex, need, false
			}
			// the composed cost (pair 8638)
			c := s.objs[a.Source()].card
			cost = s.featCost(k, SeatIx(p), c, Obj(a.Source()), addCost(k.cards[c].cost, k.t.Cards[c].Kicker))
			if s.convAnyType(k, SeatIx(p), Obj(a.Source())) {
				cost = anyTypePips(cost) // a creature's kicked total under Vizier's conversion, as plannedCastCostOf prices the plain one (featview.go)
			}
			return cost, ex, need, true
		}
		if xc, ok := s.xCastPlanCost(k, p, a); ok {
			return xc, ex, need, true
		}
		if c := s.objs[a.Source()].card; a.Kind() == AkCast && a.Mode() == 0 && a.Target() == 0 && k.t.Cards[c].HasAltAdd && k.t.Cards[c].Cost.X == 0 && !s.contribPays(k, p, c, Obj(a.Source()), s.objs[a.Source()].zone == kop.ZoneHand) {
			// an either-or additional cost: the planner prices the spell with the part
			// gorgeLowers prices, so the search funds that (Deadly Precision's {B})
			return s.altAddPlanCost(k, p, c, Obj(a.Source())), ex, need, true
		}
		if c := s.objs[a.Source()].card; a.Kind() == AkCast && a.Mode() == 0 && a.Target() == 0 && s.modalFloorFree(k, p, c) {
			// a Spree or Tiered face: the cast planner declines the shape
			// ("shape:modal_cost"), so gorge's potential planner prices the printed
			// cost from its census and the prefix search taps a granted source the
			// census misses (CR 601.2g); the mode costs are added at the cast
			// (CR 702.172a)
			cost := s.featCost(k, SeatIx(p), c, Obj(a.Source()), k.cards[c].cost)
			if s.g.mayPlayAny {
				cost = anyType(cost)
			}
			return cost, ex, need, true
		}
		if c := s.objs[a.Source()].card; s.fixedTapsCast(k, a) || s.altAddOneCast(k, a) {
			// a fixed tap of other permanents beside the printed cost: gorge's
			// potential planner prices the composed cost (cost_fixedtaps.go), so
			// the prefix search taps a granted source the census misses (CR 601.2g-h)
			cost := s.featCost(k, SeatIx(p), c, Obj(a.Source()), k.cards[c].cost)
			if s.g.mayPlayAny {
				cost = anyType(cost)
			}
			return cost, ex, need, true
		}
		cost, ok = s.plannedCastCostOf(k, p, Obj(a.Source()), a.Mode()&(3|castMayPlay), true, true)
		return cost, ex, need, ok
	}
	return cost, ex, need, false
}

// xCastPlanCost is the grant prefix search's price of an {X} cast: the fixed
// part of the cost, the witness (lowerWitness) then paying the largest X. ok is
// false for any other play.
func (s *State) xCastPlanCost(k *prep, p uint8, a Action) (cost kop.ManaCost, ok bool) {
	if a.Kind() != AkCast {
		return cost, false
	}
	c := s.objs[a.Source()].card
	if (a.Mode() == 0 || a.Mode() == castMayPlay) && k.t.Cards[c].Cost.X != 0 && !k.t.Cards[c].HasAltAdd && k.t.Cards[c].Cost.Hyb[0] == 0 {
		// an {X} cast: gorge's potentialModeCastPlan prices it at the
		// largest X the sources pay, insufficient when they cannot
		// pay X = 0, so the prefix search runs on the fixed part and
		// the witness (lowerWitness) then pays the largest X from the
		// pool and the printed sources. A may-play one too (Worldsoul's
		// Rage from Outrageous Robbery's exile), its pips generic under
		// an any-type rider (pay.PlanManaUnderRider, CR 609.4b): the
		// search's first colour then pays it (CR 601.2g-h)
		cost = s.featCost(k, SeatIx(p), c, Obj(a.Source()), k.cards[c].cost)
		if a.Mode() == castMayPlay && (s.g.mayPlayAny || s.mayPlayAnyType(k, p, Obj(a.Source()))) {
			cost = anyTypePips(cost)
		}
		return cost, true
	}
	return s.xFlashbackPlanCost(k, p, a)
}

// xFlashbackPlanCost is the grant prefix search's price of an {X} flashback cast
// (Devil's Play's {X}{R}{R}{R}): gorge's potentialModeCastPlan prices the
// flashback cost at the largest X the sources pay, insufficient when they cannot
// pay X = 0, so the prefix search runs on the fixed part as an {X} hand cast's
// does. ok is false for any other play.
func (s *State) xFlashbackPlanCost(k *prep, p uint8, a Action) (cost kop.ManaCost, ok bool) {
	c := s.objs[a.Source()].card
	cd := &k.t.Cards[c]
	if a.Kind() != AkCast || a.Mode() != 2 || cd.Keywords&kop.KwFlashback == 0 || cd.AltCost.X == 0 ||
		cd.AltCost.Hyb[0] != 0 || cd.HasAltAdd || cd.FbCost.Flags != 0 || cd.FbBehold {
		return cost, false
	}
	return s.featCost(k, SeatIx(p), c, Obj(a.Source()), cd.AltCost), true
}

// playNeed is the colours play a's printed base announces (gorge's
// scriptNeedColours): a plain cast's printed mana cost, a printed ability's
// compiled cost. An alternative, kicked, face-down or may-play cast, and the
// impended/evoked alternative costs, leave it empty, as gorge's
// scriptNeedColours does -- except a plain graveyard cast (a mode-2 cast of
// no special target), which announces the pips the cast pays
// (graveyardCastNeedCost): the kernel's branch-order hint alone, never a
// pricing read, so the budget-capped grant search reaches the announced
// colour before the colours the play does not need and prices the cast the
// census affords. The read is independent of grantPlayCost's ok: the
// filter search also prices plays the grant search declines (a waterbend or
// behold cast, a target-priced shape), and both searches must branch on the
// same set. The card's printed cost is read unreduced (CR 601.2b announces
// the printed pip colours).
func (s *State) playNeed(k *prep, a Action) (need [kop.NumManaKinds]bool) {
	if need, ok := s.playNeedPrinted(k, a); ok {
		return need
	}
	if m, ok := s.graveyardCastNeedCost(k, a); ok {
		need = grantNeedColours(m)
	}
	return need
}

// playNeedPrinted is gorge's scriptNeedColours exactly: the pips of the
// printed base cost playBaseCost reads, ok false for a play whose base gorge
// leaves unread (a mode, an alternative cost), whose need is then empty.
func (s *State) playNeedPrinted(k *prep, a Action) (need [kop.NumManaKinds]bool, ok bool) {
	m, _, ok := s.playBaseCost(k, a)
	if ok {
		need = grantNeedColours(m)
	}
	return need, ok
}

// graveyardCastNeedCost is the cost a plain graveyard cast (a mode-2 cast of
// no special target) pays, playNeed's exception read only: the alternative
// cost the cast prices -- the printed flashback cost, the printed mayhem,
// harmonize, beam-me-up or escape cost (kop/table.go) -- or the printed mana
// cost of a retrace, jump-start or aftermath cast (the printed cost's cast)
// and a granted flashback instance (fbGrantCost: the card's mana cost, or a
// live StFbOwnCost grant's own GrantCost, Iroh, Grand Lotus).
// An {X} cost leaves it empty: the grant search
// prices that cast through the fixed-part prefix search (xFlashbackPlanCost),
// whose witness pays the X, and the announced pips would steer the search to
// a plan gorge's census does not list (Devil's Play's {X}{R}{R}{R}
// flashback). gorge prices no other moded cast either (engine/rules
// potential_script.go scriptPlayBaseCost leaves a mode unread -- a may-play
// permission's printed base is the one exception it reads): the read is
// a hint for the colour-ask order, so nothing may price off it.
func (s *State) graveyardCastNeedCost(k *prep, a Action) (m kop.ManaCost, ok bool) {
	if a.Kind() != AkCast || a.Mode() != 2 || a.Target() != 0 {
		return m, false
	}
	o := Obj(a.Source())
	cd := &k.t.Cards[s.objs[o].card]
	switch {
	case cd.Mayhem || cd.Harmonize:
		m = cd.AltCost
	case cd.Keywords&kop.KwFlashback != 0 || cd.BeamMeUp || cd.Escape:
		if s.graveyardCastGranted(k, o) {
			m = s.fbGrantCost(k, s.objs[o].owner(), o, s.fbGranted(k, o))
		} else {
			m = cd.AltCost
		}
	default:
		m = cd.Cost
		if !s.g.fbProbe && (s.fbGranted(k, o) || s.fbLive(k, o)) {
			m = s.fbGrantCost(k, s.objs[o].owner(), o, s.fbGranted(k, o)) // a card with no flashback of its own, cast through a grant
		}
	}
	return m, m.X == 0
}

// graveyardCastGranted reports whether a mode-2 cast of no special target of
// o pays a granted flashback instance rather than the card's own graveyard
// alternative (flashbackBase's sfFbGranted). A printed Escape or Beam me up
// card's own offers carry marker targets (castPrintedEscapeTgt, castBeamTgt),
// so a target-0 cast of one is always the granted instance, priced at
// cd.Cost (flashbackBase skips the KwFlashback/BeamMeUp arm for
// sfFbGranted, and excludes BeamMeUp from the grant arm). A printed
// flashback card's granted instance is the one the cast takes when the
// printed instance is unaffordable (fbInstances' two && granted), and a
// filter script's probe node keeps the printed instance (fbProbe).
func (s *State) graveyardCastGranted(k *prep, o Obj) bool {
	cd := &k.t.Cards[s.objs[o].card]
	if cd.Escape || cd.BeamMeUp {
		return true
	}
	if s.g.fbProbe || cd.Keywords&kop.KwFlashback == 0 || !(s.fbGranted(k, o) || s.fbLive(k, o)) {
		return false
	}
	two, _, granted := s.fbInstances(k, s.objs[o].owner(), o)
	return two && granted
}

// playBaseCost is the cost play a's printed base announces (gorge's
// scriptPlayBaseCost), with the cost's flags (an ability's; none for a cast):
// ok is false for a play whose base gorge does not read (see playNeed).
func (s *State) playBaseCost(k *prep, a Action) (m kop.ManaCost, f kop.CostFlags, ok bool) {
	switch a.Kind() {
	case AkActivate:
		if !isGainedMode(a.Mode()) && a.Mode()&0x80 == 0 { // a gained or granted ability names no own index
			return s.abilityBaseCost(k, Obj(a.Source()), a.Mode())
		}
	case AkCast:
		// A may-play permission that only opens the zone reads the printed
		// base too (gorge's scriptPlayBaseCost castModeMayplay admission,
		// scriptSpendableGaps' same admission; gapFilteredPlay prices the
		// play's gap filter by it): a top-of-library cast behind a fee-only
		// filter beside the granted tap that pays it was priced unpruned
		// (usefulOK false) while gorge pruned the filter's colours, and the
		// two walks paid different mana. A mode that is not exactly the
		// may-play one (a kicked, bargained, flashback cast) stays unread.
		if (a.Mode() == 0 || a.Mode() == castMayPlay) && a.Target() != castImpTgt && a.Target() != castEvokeTgt && a.Target() != castDashTgt && a.Target() != castOverloadTgt && a.Target() != castSurgeTgt && a.Target() != castCleaveTgt {
			return k.cards[s.objs[a.Source()].card].cost, 0, true
		}
	}
	return m, 0, false
}

// grantNeedColours reports the colours m's symbols name, gorge's
// scriptNeedColours for a printed mana cost: a coloured pip, a hybrid pip's
// faces, a monocoloured hybrid or Phyrexian pip's colour (kop.HasColour) --
// the printed base's pips, never a reduction's or an additional cost's
// (CR 601.2b announces the printed pip colours before targets). A colourless
// pip is {C}: a granted "{T}: Add {C}" asks its colour too.
func grantNeedColours(m kop.ManaCost) (need [kop.NumManaKinds]bool) {
	for c := 0; c < 5; c++ {
		need[c] = m.HasColour(c)
	}
	need[kop.C] = m.Pips[kop.C] > 0
	return need
}

// abilityBaseCost is the printed ability ab of object o's cost for
// grantPlayCost's need set (gorge's scriptNeedColours prices a printed
// ability's compiled cost), read from the card's printed cost, unreduced.
func (s *State) abilityBaseCost(k *prep, o Obj, ab uint8) (kop.ManaCost, kop.CostFlags, bool) {
	ob := &s.objs[o]
	card := ob.card
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		card = s.cardAt(ob.slot)
	}
	abs := k.t.Cards[card].Abilities
	if int(ab) < len(abs) {
		return abs[ab].Cost.Mana, abs[ab].Cost.Flags, true
	}
	return kop.ManaCost{}, 0, false
}

// needFirst stably reorders the colours of order so the ones need names
// come first, engine order kept inside each half: the mirror of gorge's
// scriptOrderAnswers (potential_script.go), which reorders an ask's valid
// answers the same way so a budget-capped search reaches a colour the play
// announces before the colours it does not. The answer set is unchanged.
func needFirst(order []uint8, need [kop.NumManaKinds]bool) {
	for i := 1; i < len(order); i++ {
		c := order[i]
		if !need[c] {
			continue // a non-needed colour stays: the needed ones already sit before it
		}
		j := i
		for j > 0 && !need[order[j-1]] {
			order[j] = order[j-1]
			j--
		}
		order[j] = c
	}
}

// altPlayCost is what cast a pays when that isn't the card's printed cost: an
// airbend's {2}, a warp's cost, nothing for a free cast, a face-down cast's {3} or
// the card's own alternative cost. A witness priced on the printed cost has no plan.
func (s *State) altPlayCost(k *prep, p uint8, a Action) (kop.ManaCost, bool) {
	if a.Kind() != AkCast {
		return kop.ManaCost{}, false
	}
	o := Obj(a.Source())
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	switch m := a.Mode(); {
	case m == castAirbend:
		return s.airbendCost(k, p, c, o), true
	case m&castWarp != 0:
		if wi := s.warpInstances(k, p, o); a.Target() < TCode(wi.n) { // code-ok: a warp instance index
			return s.adjustCost(k, p, c, o, wi.cost[a.Target()]), true // the action's warp instance (kw_warpgrant.go)
		}
		return s.adjustCost(k, p, c, o, cd.WarpCost), true
	case m&castFree != 0:
		return kop.ManaCost{}, true
	case m&castFaceDown != 0:
		return s.adjustCost(k, p, c, o, kop.ManaCost{Generic: 3}), true
	case m == 0 && a.Target() == castAltTgt && k.gen != nil && k.gen.altFree != nil && k.gen.altFree[c] != 0:
		return s.adjustCostGen(k, p, c, o, cd.Abilities[k.gen.altFree[c]-1].Cost.Mana), true
	}
	return kop.ManaCost{}, false
}

// FeatGrantAvail is what battlefield object o adds to gorge's
// AvailableMana total through a granted mana ability (rules
// mana_available.go AvailableMana: one free ability counts every colour it
// can make times its amount; a permanent with several free abilities --
// its own beside a granted one -- is omitted): ok is false when o has no
// granted mana ability.
func (s *State) FeatGrantAvail(o Obj) (w int32, ok bool) {
	ob := &s.objs[o]
	k := s.k()
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot || k.gen == nil || len(k.gen.grantPC) == 0 {
		return 0, false
	}
	var gi [maxGrants]int16
	ng := s.grantIdxs(k, ob.slot, &gi)
	if ng == 0 {
		return 0, false
	}
	if ng > 1 || s.ownManaUsable(k, ob.slot) {
		return 0, true // several free abilities: omitted
	}
	pc := s.grantView(k.gen, gi[0], ob.slot)
	m := s.srcMaskAt(pc, ob.slot)
	if m == 0 {
		return int32(pc.manaN), true
	}
	for c := uint8(0); c < kop.NumManaKinds; c++ {
		if m&(1<<c) != 0 {
			w += int32(pc.manaN)
		}
	}
	return w, true
}

// grantsFixedMana reports whether an effect grants sl an unrestricted mana
// ability of fixed production (a plain tap for a set colour): gorge's shared
// census lists such a source at its battlefield position whatever shape its
// own ability has. A spend-restricted grant is never in that census, whether
// or not this payment's descriptor admits it (Mm'menon's "{U} to cast from
// anywhere but your hand" beside an artifact's own any-colour ability): the
// source keeps its own ability's census position, in the planner's
// restricted layer.
func (s *State) grantsFixedMana(k *prep, sl Slot) bool {
	var gi [maxGrants]int16
	n := s.grantIdxs(k, sl, &gi)
	for _, i := range gi[:n] {
		gp := &k.gen.grantPC[i]
		if gp.manaTap && !gp.manaSac && !gp.manaSacOnly && !gp.manaChosen && !gp.manaDyn && !gp.manaMulti && gp.manaCol != kop.ColAny && srcMask(gp) != 0 && k.gen.grantRestr[i] == 0 {
			return true
		}
	}
	return false
}

// filterSource: p controls a permanent with a second mana ability that costs
// mana (a filter: {1}, {T}: add one mana of any color). affordsUngrantedFor
// counts a source's first mana ability only, so its verdict on a play such a
// source funds is not gorge's. A filter that cannot be activated now (tapped,
// or a summoning-sick creature, CR 302.6) funds nothing, and a counters-gated
// filter whose gate fails is no mana ability (CR 702.184a): beside Mm'menon's
// hand-barred grant, either leaves a hand warp cast to the prune (CR 106.6,
// 702.185a).
func (s *State) filterSource(k *prep, p uint8) bool {
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		c := s.cardAt(sl)
		if k.cards[c].manaAb < 0 {
			continue
		}
		if pm := &s.perms[sl]; pm.flags&pfTapped != 0 || s.activationSick(k, sl) {
			continue // a filter that cannot be activated now funds nothing (gapMask)
		}
		cd := &k.t.Cards[c]
		seen := false
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
				continue
			}
			if pm := &s.perms[sl]; a.Trig.Cond != 0 && !s.condEval(k, a.Trig.Cond, evalCtx{src: pm.obj, srcSl: sl, ctrl: pm.ctrl}) {
				continue // a counters-gated ability while its gate fails (filtGateOn)
			}
			if seen && a.Cost.Mana.MV() > 0 {
				return true
			}
			seen = true
		}
	}
	return false
}

// restrAbN is the mana a tap of sl's printed restricted ability ra makes: an
// evaluated amount (Helga's X, her power) is read off the board, where kgen's
// ManaN is the placeholder 1 for every dynamic ability.
func (s *State) restrAbN(k *prep, sl Slot, ra *kop.Ability) uint8 {
	if ra.AFlags&kop.AfManaDynamic != 0 {
		return uint8(min(max(s.dynManaN(k, ra, sl), 1), maxDynMana))
	}
	return uint8(max(ra.ManaN, 1))
}

// floatPaysPlay reports whether p's floating pool alone pays cost for the
// play d describes, the restricted batches it may spend included (CR 106.6):
// gorge's priority options price a cast against that pool (legalActions).
func (s *State) floatPaysPlay(k *prep, p uint8, cost *kop.ManaCost, d payDesc) bool {
	if !s.playTargetsFeasible(k, p, d) {
		return false
	}
	var av avail
	for c := 0; c < kop.NumManaKinds; c++ {
		if n := s.pl[SeatIx(p)].pool[c]; n > 0 {
			av.addUnits(1<<c, n)
		}
	}
	if m := s.restrFloatAdmits(k, SeatIx(p), &d); m != 0 {
		s.restrAvailAdd(SeatIx(p), m, &av)
	}
	if !av.affords(cost, pipMaskOf(cost), 0) {
		return false
	}
	// a fixed-taps cast's part keeps its candidates apart from the pool too
	// (CR 601.2g-h): the walk's gate prices the mana from the pool and the
	// part against the real state, so a pool that pays only by tapping a
	// candidate the part needs is no offer -- after the prefix tapped the
	// granted mana, only the summoning-sick Kona is left for Fear of
	// Exposure's tap-two (gorge planReserving; cost_fixedtaps.go).
	if d.hasPlay && k.t != nil && s.fixedTapsCast(k, d.play) {
		return s.costPartDisjointD(k, p, NoSlot, spellAddCost(k, s.objs[d.play.Source()].card), cost, &av, pipMaskOf(cost), &d)
	}
	return true
}
