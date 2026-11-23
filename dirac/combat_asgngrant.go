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

// assignsUnblocked reports whether the attacker at a routes its combat damage
// whole to the player: the mandatory grant (asUnblocked) or an accepted
// as-unblocked election (CR 510.1c).
func (s *State) assignsUnblocked(k *prep, a Slot) bool {
	return s.asUnblocked(k, a) || k.anyAsUnblk && s.electMarks(a)&electUnblkMark != 0
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
// election routes the whole power to the defending player (electUnblk) and
// the attacker never divides. The marks are the elect cold region below, not
// perm.flags: all sixteen perm-flag bits are assigned (pfLastBear 14,
// pfSkipUntap 15).
func (s *State) nextElection(k *prep, first bool) bool {
	if !k.anyAsUnblk {
		return false
	}
	var ord [MaxPerm]Slot
	no := s.attackersInZoneOrder(&ord)
	for oi := 0; oi < no; oi++ {
		a := ord[oi]
		p := &s.perms[a]
		if s.electMarks(a)&electMarksAny != 0 { // answered already this pass
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
		s.setElectMarks(a, electAskMark)
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
// (electUnblk routes the attacker's whole combat damage to the defending
// player, CR 510.1c) and drops it from the division walk, mode 0 declines it
// and the ordinary assignment switch runs. gorge handleAsUnblockedElection.
func (s *State) applyElection(a Action) {
	if a.Mode() == 1 {
		s.setElectMarks(s.divA, electUnblkMark)
		s.touchSl(s.divA)
	}
	s.divA = NoSlot
}

// The as-unblocked election's marks: useElectMark tells prepareGen a table
// with a printed StatAssignCombatDamageAsUnblocked static needs a genPrep,
// and coldElect is each battlefield slot's two marks — bit 0 electAskMark,
// the election was asked of its controller this damage pass (nextElection's
// dedup across the pending ask); bit 1 electUnblkMark, it was accepted and
// the attacker's whole combat damage goes to the defending player (CR
// 510.1c, combatDamage/nextDivision's readers). A perSlot region, zeroed
// when the slot takes a new permanent, so a mark never outlives the
// permanent it was asked over; the damage pass clears its own marks at each
// fresh damageStep (combat.go's !ans loop, gorge beginCombatPass rebuilding
// electQueue/doneElect per pass). perm.flags has no bit for them (pfLastBear
// 14, pfSkipUntap 15; face_manifest.go), and the scratch's arms are sized
// without them (statelayout_test.go: State must not grow).
const (
	electAskMark   uint8 = 1
	electUnblkMark uint8 = 2

	electMarksAny = electAskMark | electUnblkMark
)

var useElectMark = newGenUse(func(t *kop.Table) bool {
	return tableHasStatic(t, kop.StatAssignCombatDamageAsUnblocked)
})

var coldElect = newCold[uint8]("elect", func(g *genPrep) int {
	if g != nil && g.uses.has(useElectMark) {
		return MaxPerm
	}
	return 0
}).perSlot()

// electMarks is sl's election mark bits, 0 when the table has none.
func (s *State) electMarks(sl Slot) uint8 {
	if c := coldElect.of(s); int(sl) < len(c) {
		return c[sl]
	}
	return 0
}

// setElectMarks adds the given marks to sl's.
func (s *State) setElectMarks(sl Slot, add uint8) {
	if c := coldElect.of(s); int(sl) < len(c) {
		c[sl] |= add
	}
}

// clrElect takes sl's marks away (the damage pass clears its own attackers'
// at each fresh damageStep, combat.go's !ans loop).
func (s *State) clrElect(sl Slot) {
	if c := coldElect.of(s); int(sl) < len(c) {
		c[sl] = 0
	}
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
