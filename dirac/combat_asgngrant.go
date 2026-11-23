package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// AsgnGrantStats counts the combat damage assignments an ExtAsgnGrant record
// changed (the oracle tests assert their pools reach the mechanism).
var AsgnGrantStats struct{ Tough, Unblocked atomic.Int64 }

// ExtAsgnGrant: the Effect-delivered combat-damage-assignment grants gorge
// registers as continuous effects over a remembered object, until end of turn
// (CR 611.2): "assigns combat damage equal to its toughness rather than its
// power" (Plagon, Lord of the Beach; Mode$ CombatDamageToughness, CR 510.1)
// and "assign their combat damage as though they weren't blocked"
// (Outmaneuver; Mode$ AssignCombatDamageAsUnblocked with no Optional$, so
// mandatory, CR 510.1c-d). Each is a record in State.cold.exH beside the
// ExHaste ones (kinds exkTough / exkUnblocked; a record is neither a keyword
// nor an ability, so a loss of all abilities leaves it) tied to the object's
// (obj, ver): the permanent leaving the battlefield ends it (ForgetOnMoved$
// Battlefield), and exhClear ends all of them at the cleanup step. Behind
// genPrep.anyExHaste, so every other table's hot paths and hashes are
// unchanged.

// Kinds of an exhRec beyond ExHaste's 1 (haste) and 0x80|cmp<<4|N (power).
const (
	exkTough     uint8 = 2 // assigns combat damage equal to its toughness
	exkUnblocked uint8 = 3 // assigns combat damage as though it weren't blocked
)

// prepAsgnGrant sets anyExHaste (the record store and its cleanup) and, for a
// toughness grant, anyCdmgTough (the amount read in combat.go).
func prepAsgnGrant(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() != kop.ExtAsgnGrant {
					continue
				}
				g.anyExHaste = true
				if in.C == 0 {
					g.anyCdmgTough = true
				}
			}
		}
	}
}

// asgnGrant is ExtAsgnGrant: C 0 records the toughness grant, C 1 the
// as-unblocked grant, on the object in Obj.
func (s *State) asgnGrant(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	kind := exkTough
	if in.C == 1 {
		kind = exkUnblocked
	}
	s.exhAdd(sl, kind)
}

// exhHas reports whether the permanent at sl has a live record of kind.
func (s *State) exhHas(sl Slot, kind uint8) bool {
	o := s.perms[sl].obj
	for i := range s.cold.exH {
		if r := &s.cold.exH[i]; r.kind == kind && r.obj == o && r.ver == s.objs[o].ver {
			return true
		}
	}
	return false
}

// asUnblocked reports whether the attacker at a assigns its combat damage as
// though it weren't blocked (gorge asUnblockedStaticMatches' mandatory arm,
// which damageStep's chosenElection case routes whole to the defender).
func (s *State) asUnblocked(k *prep, a Slot) bool {
	if k.gen != nil && k.gen.anyExHaste && s.exhHas(a, exkUnblocked) {
		AsgnGrantStats.Unblocked.Add(1)
		return true
	}
	return false
}

// attackersInZoneOrder lists the active player's attacking creatures in
// battlefield (entry) order (gorge G.Zone(Battlefield, active), the order
// asUnblockedNeeding and divisionNeeding walk; the ts insertion restores it
// from the kernel's slot order, nextDivision's prologue).
func (s *State) attackersInZoneOrder(ord *[MaxPerm]Slot) int {
	no := 0
	for am := s.ctrlM[s.active]; am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking == 0 {
			continue
		}
		j := no
		for j > 0 && s.perms[ord[j-1]].ts > s.perms[a].ts {
			ord[j] = ord[j-1]
			j--
		}
		ord[j] = a
		no++
	}
	return no
}

// nextElection finds the next attacker (battlefield order) whose controller
// is offered the as-unblocked election (gorge askNextCombatAsk's electQueue,
// CR 510.1c's optional "you may assign this creature's combat damage as
// though it weren't blocked"): a blocked attacker that deals damage this
// pass, whose power is above zero, whose outcome the election can change (a
// trampler with no live blocker left routes the whole power to the player
// either way), whose printed StatAssignCombatDamageAsUnblocked static
// applies (gorge asUnblockedStaticMatches' Optional$ True arm; the kgen body
// lowers only that spelling, the mandatory one is gorge's comment-only
// auto-accept), and that has not been asked yet this pass. An ExtAsgnGrant
// record (Outmaneuver) is gorge's MANDATORY match: auto-accepted without an
// ask, so asUnblocked skips it too. Elections are asked BEFORE every damage
// division (gorge asks the two queues in that fixed order); an accepted
// election routes the whole power to the defending player (pfElectUnblk) and
// the attacker never divides.
func (s *State) nextElection(k *prep, first bool) bool {
	if !k.anyAsUnblk {
		return false
	}
	var ord [MaxPerm]Slot
	no := s.attackersInZoneOrder(&ord)
	for oi := 0; oi < no; oi++ {
		a := ord[oi]
		p := &s.perms[a]
		if p.flags&(pfElectAsk|pfElectUnblk) != 0 { // answered already this pass
			continue
		}
		if p.flags&pfBlocked == 0 || s.asUnblocked(k, a) {
			continue
		}
		if !s.dealsNow(k, a, first) || s.combatAmount(k, a) <= 0 {
			continue
		}
		if s.kw(k, a)&kop.KwTrample != 0 {
			var bl [MaxPerm]Slot
			if s.liveBlockers(a, &bl) == 0 {
				continue
			}
		}
		if !s.staticFlag(k, kop.StatAssignCombatDamageAsUnblocked, a) {
			continue
		}
		p.flags |= pfElectAsk
		s.touchSl(a)
		s.divA, s.divI, s.divRem = a, 0, 0 // divRem 0 marks the pending ask an election
		return true
	}
	return false
}

// electionOptions poses the as-unblocked election (gorge askNextCombatAsk's
// KChoose): two options over the attacker, 0 assign normally (blocked), 1
// assign as though not blocked.
func (s *State) electionOptions(k *prep, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChAsUnblocked, s.active
	o := s.perms[s.divA].obj
	d.add(MakeAction(AkChoose, TCode(o), 0, 0))
	d.add(MakeAction(AkChoose, TCode(o), 0, 1))
}

// applyElection answers the pending as-unblocked election: mode 1 accepts it
// (pfElectUnblk routes the attacker's whole combat damage to the defending
// player, CR 510.1c) and drops it from the division walk, mode 0 declines it
// and the ordinary assignment switch runs. gorge handleAsUnblockedElection.
func (s *State) applyElection(a Action) {
	if a.Mode() == 1 {
		s.perms[s.divA].flags |= pfElectUnblk
		s.touchSl(s.divA)
	}
	s.divA = NoSlot
}

// asgnTough reports whether the creature at sl assigns its combat damage
// equal to its toughness by an ExtAsgnGrant record (combatAmtTough).
func (s *State) asgnTough(sl Slot) bool {
	if s.exhHas(sl, exkTough) {
		AsgnGrantStats.Tough.Add(1)
		return true
	}
	return false
}
