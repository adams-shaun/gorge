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

// asgnTough reports whether the creature at sl assigns its combat damage
// equal to its toughness by an ExtAsgnGrant record (combatAmtTough).
func (s *State) asgnTough(sl Slot) bool {
	if s.exhHas(sl, exkTough) {
		AsgnGrantStats.Tough.Add(1)
		return true
	}
	return false
}
