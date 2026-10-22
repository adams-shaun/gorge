package policynet

import (
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// entTarget is one object target of one stack entry: the targeted object
// and whether the entry's controller is the encoding seat.
type entTarget struct {
	obj  state.ObjID
	mine bool
}

// entPair is one attacker/blocker relation: the attacking battlefield card
// and one id of its BlockedBy.
type entPair struct {
	att, blk state.ObjID
}

// stackHas reports whether id is a stack object.
func stackHas(v *view.View, id state.ObjID) bool {
	for i := range v.Stack {
		if v.Stack[i].ID == id {
			return true
		}
	}
	return false
}

// graveyardHas reports whether id is in some player's graveyard.
func graveyardHas(v *view.View, id state.ObjID) bool {
	for i := range v.Players {
		for j := range v.Players[i].Graveyard {
			if v.Players[i].Graveyard[j].ID == id {
				return true
			}
		}
	}
	return false
}

// card returns a battlefield card by id, the LAST battlefield entry with
// that id (the same walk controller uses), or nil when no battlefield has
// it.
func (b *entityBoard) card(id state.ObjID) *view.CardView {
	for i := len(b.v.Players) - 1; i >= 0; i-- {
		bf := b.v.Players[i].Battlefield
		for j := len(bf) - 1; j >= 0; j-- {
			if bf[j].ID == id {
				return &bf[j]
			}
		}
	}
	return nil
}

// combatFoes collects cv's combat foes into foeBuf and returns the slice:
// for an attacker, each battlefield card id in its BlockedBy; for a
// blocker, every battlefield card whose BlockedBy contains its id. A foe id
// found on no battlefield is skipped.
func (b *entityBoard) combatFoes(cv *view.CardView, foeBuf []state.ObjID) []state.ObjID {
	foes := foeBuf[:0]
	if cv.Attacking {
		for _, id := range cv.BlockedBy {
			if b.card(id) != nil {
				foes = append(foes, id)
			}
		}
		return foes
	}
	for _, p := range b.pairs {
		if p.blk != cv.ID {
			continue
		}
		dup := false
		for _, f := range foes {
			if f == p.att {
				dup = true
				break
			}
		}
		if !dup {
			foes = append(foes, p.att)
		}
	}
	return foes
}

// combatSlots writes slots entCombatFoes..entCombatKills for one
// battlefield card. Deliberate approximations (no state read beyond the
// view): first strike, protection, prevention, trample and an attacker
// splitting damage among several blockers are ignored, so on each blocker
// of a multi-blocked attacker entCombatDies is the could-die bound.
func combatSlots(raw []float32, cv *view.CardView, foes []state.ObjID, b *entityBoard) {
	if len(foes) == 0 {
		return
	}
	raw[entCombatFoes] = float32(len(foes)) / 2
	var pow, tgh int32
	deathFoe := false
	for _, id := range foes {
		foe := b.card(id)
		p := foe.Power
		if p < 0 {
			p = 0
		}
		pow += p
		rem := foe.Toughness - foe.Damage
		if rem < 0 {
			rem = 0
		}
		tgh += rem
		if p > 0 && hasKeyword(foe, kwDeathtouch) {
			deathFoe = true
		}
	}
	raw[entCombatFoePow] = float32(pow) / 4
	raw[entCombatFoeTgh] = float32(tgh) / 4
	selfRem := cv.Toughness - cv.Damage
	if !hasKeyword(cv, kwIndestructible) &&
		((pow > 0 && pow >= selfRem) || deathFoe) {
		raw[entCombatDies] = 1
	}
	if cv.Power > 0 {
		for _, id := range foes {
			foe := b.card(id)
			if hasKeyword(cv, kwDeathtouch) || cv.Power >= foe.Toughness-foe.Damage {
				raw[entCombatKills] = 1
				break
			}
		}
	}
}

// entityRelations writes the relational slots (EntityRawWidthV1..) of one
// listed card: how many stack entries target it, and for a listed stack card
// what its own entry targets, its X and whether it was kicked.
func entityRelations(raw []float32, cv *view.CardView, zone int, sv *view.StackView, seat state.PlayerID, b *entityBoard, foeBuf []state.ObjID) {
	if len(b.tgts) == 0 && sv == nil && len(b.pairs) == 0 {
		return
	}
	for _, t := range b.tgts {
		if t.obj != cv.ID {
			continue
		}
		if t.mine {
			raw[entTgtByMe]++
		} else {
			raw[entTgtByOpp]++
		}
	}
	if zone == entZoneBf && len(b.pairs) > 0 {
		combatSlots(raw, cv, b.combatFoes(cv, foeBuf), b)
	}
	if zone != entZoneStack || sv == nil {
		return
	}
	for _, t := range sv.Targets {
		if t.IsPlayer {
			if t.Player == seat {
				raw[entTgtPlayerMe]++
			} else {
				raw[entTgtPlayerOpp]++
			}
			continue
		}
		if t.Obj == 0 {
			continue
		}
		if c, ok := b.controller(t.Obj); ok {
			if c == seat {
				raw[entTgtPermMe]++
			} else {
				raw[entTgtPermOpp]++
			}
			continue
		}
		if stackHas(b.v, t.Obj) {
			raw[entTgtStack]++
			continue
		}
		if graveyardHas(b.v, t.Obj) {
			raw[entTgtGrave]++
		}
	}
	raw[entStackX] = float32(sv.X) / 5
	if sv.Kicks > 0 {
		raw[entStackKicked] = 1
	}
}
