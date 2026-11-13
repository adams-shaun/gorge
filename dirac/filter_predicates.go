package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Filter predicates and flags the tier-G sweep added (cold, out of line).

// predPermStats: PredToughnessGE, PredPowerLTToughness (gorge
// `powerLTtoughness`) (current toughness, gorge `toughnessGE<n>`)
// and PredHasXCost (gorge `hasXCost`: the printed mana cost has {X}, never
// for a face-down object) for a battlefield permanent.
func (s *State) predPermStats(k *prep, fl *kop.Filter, sl Slot) bool {
	if fl.Pred == kop.PredHasXCost {
		return s.hasXCostPerm(k, sl) // CR 708.2: false face down
	}
	pw, t := s.ptGen(k, sl)
	if fl.Pred == kop.PredPowerLTToughness {
		return ptRel(fl.PredN, pw, t)
	}
	return t >= int(fl.PredN)
}

// predCardStats is predPermStats for a card off the battlefield (printed
// toughness, or the CDA's).
func (s *State) predCardStats(k *prep, fl *kop.Filter, c CardID, owner uint8, o Obj) bool {
	pc := &k.cards[c]
	if fl.Pred == kop.PredHasXCost {
		return s.hasXCostCard(k, c, o) // CR 708.4: false for a face-down spell
	}
	pw, t := int(pc.power), int(pc.tough)
	if k.cda && k.gen.has(c, gfCDAPT) {
		pw, t = s.cardCDAPT(k, c, owner, o)
	}
	if fl.Pred == kop.PredPowerLTToughness {
		return ptRel(fl.PredN, pw, t)
	}
	return t >= int(fl.PredN)
}

// ptRel is PredPowerLTToughness's relation (kop.PTRel*) over power pw and
// toughness t.
func ptRel(rel int16, pw, t int) bool {
	switch rel {
	case kop.PTRelEQ:
		return pw == t
	case kop.PTRelNE:
		return pw != t
	}
	return pw < t
}

// notTriggerObj: F2NotTriggerObj (gorge `NotDefinedTriggeredCard`): o is
// not the triggering object of the ability being resolved or put on the
// stack. Outside both it fails closed.
func (s *State) notTriggerObj(o Obj) bool {
	e := s.curEntry()
	if e == nil || e.trigObj == NoObj {
		return false // an unbound referent never negates into a match (gorge notDefinedTriggerReferent)
	}
	return e.trigObj != o
}

// isTriggerObj: PredTriggerObj, the ChooseFromDefined$ TriggeredCards pool's
// membership test (gorge chooseFromDefinedPool over the trigger's capture):
// o IS the triggering object of the ability being resolved or put on the
// stack. An unbound referent never matches: the pool is empty (gorge
// trigger_match.go, the ManifestDreadAction arm's empty list stays empty).
func (s *State) isTriggerObj(o Obj) bool {
	e := s.curEntry()
	if e == nil || e.trigObj == NoObj {
		return false
	}
	return e.trigObj == o
}

// activateCondHolds: an activated ability's activation restriction (gorge
// CheckSVar$/SVarCompare$ on an AB$, CR 602.5b), carried in the otherwise
// unused Trig.Cond of an AbActivated ability, holds now.
func (s *State) activateCondHolds(k *prep, sl Slot, a *kop.Ability) bool {
	p := &s.perms[sl]
	return s.condEval(k, a.Trig.Cond, evalCtx{src: p.obj, srcSl: sl, ctrl: p.ctrl})
}

// digEnter puts a DigUntil's found card onto the battlefield under its
// owner's control (gorge effDigUntil: the found move before the revealed
// rest, Tapped$ applied, then the Attacking$ rider). kgen lowers it only for
// a Valid$ that cannot admit an Aura, so no bearer is ever chosen. A full
// battlefield stalls (the kernel's capacity, as ChangeZoneAll's mass return);
// it reports false then.
func (s *State) digEnter(k *prep, p uint8, o Obj, tapped, attacking bool) bool {
	if s.occ.full() {
		s.stall(StallBoard)
		return false
	}
	if s.objs[o].zone == kop.ZoneLibrary {
		s.pl[p].libRemove(o)
	}
	s.objs[o].zone = kop.ZoneNone
	sl := s.enter(k, o, p, tapped, kop.ZoneLibrary)
	if attacking {
		s.enterAttacking(k, sl, p)
	}
	return true
}

// Filter predicates (cold, out of line).

// predPermKind: PredMultiColor (gorge wordMultiColor: more than one current
// colour), PredHistoric (gorge historicMatches: artifact, legendary or Saga,
// current types) and PredEquipped (gorge equipped/hasAttachmentOfKind: a
// battlefield permanent with the Equipment type is attached to it) for a
// battlefield permanent.
func (s *State) predPermKind(k *prep, fl *kop.Filter, sl Slot) bool {
	switch fl.Pred {
	case kop.PredMultiColor:
		return bits.OnesCount8(uint8(s.colorsGen(k, sl)&kop.ColAny)) > 1
	case kop.PredHistoric:
		if s.typesGen(k, sl)&(kop.TArtifact|kop.TLegendary) != 0 {
			return true
		}
		return fl.PredN != 0 && s.hasSubGen(k, sl, uint8(fl.PredN-1))
	}
	kind := kop.TEquipment
	if fl.Pred == kop.PredEnchanted {
		kind = kop.TAura // gorge enchanted (hasAttachmentOfKind "Aura")
	}
	for m := s.occ; m.any(); m = m.rest() {
		q := m.first()
		if s.perms[q].attached == sl && s.typesGen(k, q)&kind != 0 {
			return true
		}
	}
	return false
}

// predCardKind is predPermKind for a card off the battlefield (printed
// characteristics); nothing is attached to a card, so PredEquipped fails.
func predCardKind(k *prep, fl *kop.Filter, c CardID) bool {
	cd := &k.t.Cards[c]
	switch fl.Pred {
	case kop.PredMultiColor:
		return bits.OnesCount8(uint8(cd.Colors&kop.ColAny)) > 1
	case kop.PredHistoric:
		if k.cards[c].types&(kop.TArtifact|kop.TLegendary) != 0 {
			return true
		}
		return fl.PredN != 0 && int(fl.PredN-1) < len(k.t.Subtypes) && cd.Subtypes.Has(int(fl.PredN-1))
	}
	return false
}

// attacksGate: an Attacks trigger's Alone$ (Tf2Alone: the whole
// declaration is one attacker, gorge declaredAttackerCount) and Attacked$
// (Tf2Attacked: attackedMatch on this event group's defender def).
//
//go:noinline
func (s *State) attacksGate(f kop.TrigFlags2, ctrl uint8, def uint8) bool {
	if f&kop.Tf2Alone != 0 && s.nAttackers != 1 {
		return false
	}
	return f&kop.Tf2Attacked == 0 || s.attackedMatch(f, ctrl, def)
}

// attackedMatch: gorge attackedDefenderMatches for the Attacked$ forms kgen
// lowers (Tf2Attacked): def (an attacker's perms.atk: PlayerTarget0+p, or a
// planeswalker Obj) is the trigger controller ctrl (Tf2AtkYou), a
// planeswalker ctrl controls (Tf2AtkYourPW), or an opponent player
// (Tf2AtkOpp).
func (s *State) attackedMatch(f kop.TrigFlags2, ctrl uint8, def uint8) bool {
	if isPlayerOrNone(def) {
		p := CodeSeat(def)
		return p == ctrl && f&kop.Tf2AtkYou != 0 || p != ctrl && f&kop.Tf2AtkOpp != 0
	}
	if f&kop.Tf2AtkYourPW == 0 {
		return false
	}
	o := Obj(def)
	return s.objs[o].zone == kop.ZoneBattlefield && s.perms[s.objs[o].slot].ctrl == ctrl
}

// enchCtrlIs: kop.WhoseEnchCtrl (gorge playerEnchantedController): the
// listener at sl is attached to a permanent controlled by p.
//
//go:noinline
func (s *State) enchCtrlIs(sl Slot, p uint8) bool {
	if sl == NoSlot {
		return false
	}
	at := s.perms[sl].attached
	return at != NoSlot && s.perms[at].ctrl == p
}

// attackersMatching: gorge AttackersDeclaredMatchedAttackers -- the
// declared attackers (atk, the whole declaration) matching the listener's
// ValidAttackers$ filter, counted for ValidAttackersAmount$ and the
// trigger amount; none matching means no trigger.
//
//go:noinline
func (s *State) attackersMatching(k *prep, l trigListener, a *kop.Ability, atk []Slot) int16 {
	n := int16(0)
	for _, as := range atk {
		if s.matchPermGen(k, a.Trig.Filter, as, l.ctrl, l.sl, layerFull) {
			n++
		}
	}
	return n
}

// exprCompare: kop.ExCompare (gorge evalCompare): the compared value Sub[0]
// against the literal threshold N with Op; the hit picks the then arm of the
// ExBranches at Sub[1], a miss its else arm.
//
//go:noinline
func (s *State) exprCompare(k *prep, x *kop.Expr, ctx evalCtx) int {
	v, th := s.evalV(k, x.Sub[0], ctx), int(x.N)
	var hit bool
	switch x.Op {
	case kop.CmpGE:
		hit = v >= th
	case kop.CmpGT:
		hit = v > th
	case kop.CmpEQ:
		hit = v == th
	case kop.CmpLE:
		hit = v <= th
	case kop.CmpLT:
		hit = v < th
	case kop.CmpPrime:
		hit = isPrimeN(v)
	}
	if DebugHasCardsGE && x.Op == kop.CmpGT { // lockstep mutation (value_hascards.go)
		hit = v >= th
	}
	br := &k.t.Exprs[x.Sub[1].N]
	if hit {
		return s.evalV(k, br.Sub[0], ctx)
	}
	return s.evalV(k, br.Sub[1], ctx)
}

// predAttached: PredAttached (gorge objectAttachedToSource) for a battlefield
// permanent: attached to the source's slot, or, with the resolving ability's source
// gone, attached to it as it left (noteLastBear; CR 608.2h).
func (s *State) predAttached(sl, src Slot) bool {
	p := &s.perms[sl]
	if src != NoSlot {
		return p.attached == src
	}
	if p.attached != NoSlot || p.flags&pfLastBear == 0 || !s.res.active {
		return false
	}
	o := s.res.e.obj
	return Obj(p.bord) == o && s.objs[o].zone != kop.ZoneBattlefield
}

// sweep 1 in Standard-weighted order. Cold, out of line; nothing
// here is reached by a table without the lifted shapes (an FDN/BLB filter
// never carries PredHasCounters).

// predHasCounters is PredHasCounters (gorge wordHasCounters: hasRealCounters
// over Object.Counters): the battlefield permanent at sl holds one or more
// counters of any kind. A regeneration shield (pfRegen) or a deathtouch mark
// (pfDeathtouched) is not a counter (CR 122.1), and neither is a Class level
// (CtrLevel; gorge's Object.ClassLevelValue).
//
//go:noinline
func (s *State) predHasCounters(sl Slot) bool {
	p := &s.perms[sl]
	if p.p1 != 0 {
		return true
	}
	for i := range p.ck {
		if p.cn[i] != 0 && kop.CounterKind(p.ck[i]) != kop.CtrLevel {
			return true
		}
	}
	return p.ck[ctrSlots-1] != 0 && s.ctrOverSum(p.obj, 0) != 0
}

// The kgen lifts unblocked (gorge pin 81dd3f940). Cold,
// out of line; nothing here is reached by a table without the lifted
// shapes (an FDN/BLB filter never carries PredModified, and the ops are
// appended past OpPutCounterAll on execCold1's chain).

// predModified is PredModified (CR 700.9; gorge modifiedPermanent):
// the battlefield permanent at sl has one or more counters, an Equipment
// attached, or an Aura attached that its own controller controls. A counter
// is any kind the kernel stores as one: +1/+1 (p1) or a general slot whose
// kind is not CtrLevel (a Class's level is not a counter, kw_class.go); the
// regeneration shield and the deathtouch mark are flags, matching gorge's
// InternalCounterMarker exclusion.
//
//go:noinline
func (s *State) predModified(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	if p.p1 != 0 {
		return true
	}
	for i := range p.ck {
		if p.cn[i] != 0 && kop.CounterKind(p.ck[i]) != kop.CtrLevel {
			return true
		}
	}
	if p.ck[ctrSlots-1] != 0 && s.ctrOverSum(p.obj, 0) != 0 {
		return true
	}
	for m := s.occ; m.any(); m = m.rest() {
		q := m.first()
		if s.perms[q].attached != sl {
			continue
		}
		ty := s.typesGen(k, q)
		if ty&kop.TEquipment != 0 || ty&kop.TAura != 0 && s.perms[q].ctrl == p.ctrl {
			return true
		}
	}
	return false
}
