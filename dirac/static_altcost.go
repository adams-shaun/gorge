package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The StatAltEquipCost static (Kíli the Resourceful's "As long as you have
// an enduring story, you may pay {0} rather than pay the equip cost of the
// first equip ability you activate each turn"): the battlefield grant of an
// activated ability's alternative cost (gorge activate_static_altcost.go's
// abilityAlternateCostFor / staticAltCostScopesAbility; CR 118.9, the
// alternative replaces the printed one). The offer lists the alternative as
// a second AkActivate action beside the printed cost, payable independently;
// the charge prices the alternative, which the stack entry's seAltEquip bit
// carries from the offered action, because the push has already recorded the
// activation the count gate reads (the gate is 0 only before the turn's
// first equip push).

// DebugNoAltEquip drops the alternative-cost offer (TestAltEquipMutation
// proves the pool notices).
var DebugNoAltEquip bool

// altEquipTgt is the AkActivate target field of the alternative-cost
// activation: every plain battlefield activation's target field is 0 (the
// granted forms, which carry a target, ride mode bit 0x80), so a marker in
// the unused code band cannot collide with one (dirac/seats.go). The
// oracle's canonical form reads it (canon.AltAbilityMode beside the face
// index).
const altEquipTgt TCode = 508

// AltEquipTgt is altEquipTgt for the oracle's canonical form.
const AltEquipTgt = altEquipTgt

// seAltEquip is a stackEntry.mode bit of an ability activation paying the
// StatAltEquipCost static's alternative instead of the printed cost. The
// modal-spell bits (0..3) and seFaceDown/seAtkPlayer/seAtkCtrl (spells and
// triggers) never sit on an ability entry, and an Equip ability has no charm
// modes, so the free spell bits are free here too.
const seAltEquip uint8 = 1 << 6

// altEquipCost is the alternative cost the StatAltEquipCost statics grant
// the activator p's ability a of the battlefield source sl, priced at the
// offer: the first applying static's Cost (eachStaticTS's ts order, gorge's
// activeStatics scan; gorge staticAltCostScopesAbility). The gates are the
// activator's, the Cond2 composite (which holds the enduring story beside
// the zero count of the turn's first equip) and the source filter. false
// when none applies or the printed cost is outside the shape the kernel
// offers an alternative for (mana plus kop.CostSorcery; gorge replaces the
// whole cost, so a non-mana part is a cardset refusal, not an offer).
func (s *State) altEquipCost(k *prep, a *kop.Ability, sl Slot, p uint8) (kop.ManaCost, bool) {
	if DebugNoAltEquip || !altEquipCostShape(a) {
		return kop.ManaCost{}, false
	}
	// no genPrep gate: a new fixed prep field fails TestNoNewFixedStateFields;
	// the sclsAltEq class bit (static_classgates.go) skips every card without
	// a StatAltEquipCost static inside the walk, so the walk is its own gate.
	if k.gen == nil {
		return kop.ManaCost{}, false
	}
	c, ok := kop.ManaCost{}, false
	s.eachStaticTS(k, sclsAltEq, func(st statSrc) bool {
		if !s.altEquipStaticApplies(k, st, a, sl, p) {
			return true
		}
		c, ok = st.a.Cost.Mana, true
		return false // the first applying static's cost is the one alternative
	})
	return c, ok
}

// altEquipCostShape reports whether the printed cost is mana plus
// kop.CostSorcery and nothing else, the shape the kernel offers an
// alternative for.
func altEquipCostShape(a *kop.Ability) bool {
	return a.Kind == kop.AbActivated && a.AFlags&kop.AfEquip != 0 &&
		a.Cost.Flags == kop.CostSorcery && a.Cost.Mana.X == 0 && a.Cost.XMin == 0 &&
		a.Cost.Mana.Snow == 0 && a.Cost.Host == 0 && a.CostProg == 0 && a.Evidence == 0
}

// altEquipStaticApplies is the per-static gate of a StatAltEquipCost static
// st on ability a of the battlefield source sl activated by ctrl (the shape
// of cost_reduceability.go abReduceStaticApplies): the activator, the
// condition and the filter over the ability's source.
func (s *State) altEquipStaticApplies(k *prep, st statSrc, a *kop.Ability, sl Slot, ctrl uint8) bool {
	sa := &st.a.Static
	if st.a.Kind != kop.AbStatic || sa.Kind != kop.StatAltEquipCost || st.ctrl != ctrl && sa.StFlags&kop.StReduceAny == 0 {
		return false
	}
	return s.statCond(k, st) && s.matchPermGen(k, sa.Filter, sl, st.ctrl, st.sl, layerFull)
}

// altEquipCharge is the alternative cost the charge of the seAltEquip
// activation prices: the first StatAltEquipCost static of the same gates the
// offer read, WITHOUT the Cond2 read -- the push has already recorded the
// activation (static_turnbits.go noteTurnEquip), so the count gate reads 1
// by now, while the offer's answer carried the choice here whole (nothing
// can move between the offered decision and its answer). false: the static
// is gone (cannot happen within one answer); the caller keeps the printed
// cost, as gorge's stale-option fallback does.
func (s *State) altEquipCharge(k *prep, a *kop.Ability, o Obj, ctrl uint8) (kop.ManaCost, bool) {
	if !altEquipCostShape(a) {
		return kop.ManaCost{}, false
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return kop.ManaCost{}, false // the offer's source left: the printed cost (cannot happen within one answer)
	}
	sl := ob.slot
	c, ok := kop.ManaCost{}, false
	s.eachStaticTS(k, sclsAltEq, func(st statSrc) bool {
		sa := &st.a.Static
		if st.a.Kind != kop.AbStatic || sa.Kind != kop.StatAltEquipCost || st.ctrl != ctrl && sa.StFlags&kop.StReduceAny == 0 {
			return true
		}
		if !s.matchPermGen(k, sa.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
		}
		c, ok = st.a.Cost.Mana, true
		return false
	})
	return c, ok
}

// altEquipPicked reports the plain AkActivate action that targets
// altEquipTgt: the StatAltEquipCost static's alternative was picked, and the
// pushed entry must carry that choice (seAltEquip), because the push has
// already recorded the activation the count gate reads — the charge cannot
// re-read the gate (altEquipChargeEntry).
func (s *State) altEquipPicked(k *prep, a Action) bool {
	// no genPrep gate (TestNoNewFixedStateFields): the target comparison is
	// the gate — altEquipTgt is only ever offered when a StatAltEquipCost
	// static applied (activateOffers), so a plain activation never carries it.
	return a.Mode()&0x80 == 0 && a.Mode() < 255 && a.Target() == altEquipTgt
}

// altEquipChargeEntry prices a stack entry marked seAltEquip from
// altEquipCharge: the charge prices the choice the offer carried on the
// entry (the mode bit applyPriority set), never re-reading the count gate
// (the push has already recorded this activation, so it reads 1). ok is
// false when the entry is not an equip alternative or the static no longer
// applies, and the caller keeps the printed cost.
func (s *State) altEquipChargeEntry(k *prep, e *stackEntry, a *kop.Ability) (kop.ManaCost, bool) {
	// no genPrep gate (TestNoNewFixedStateFields): the mode bit is the gate —
	// seAltEquip is set only on an entry pushed from an altEquipTgt action,
	// which the offer made only under a StatAltEquipCost static.
	if e.mode&seAltEquip == 0 || k.gen == nil {
		return kop.ManaCost{}, false
	}
	return s.altEquipCharge(k, a, e.obj, e.ctrl)
}
