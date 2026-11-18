package pay

import (
	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// ColourCap is the cast window walk's per-colour reach bound: a strict
// coloured (or {C}) pip is paid only by mana of that type (CR 107.4, CR
// 601.2h), so a walk whose pool plus the most of that colour its unused
// sources could still add falls short of the cost's pips of that colour is
// dead. The walk's total-mana bound misses this whenever the sources hold
// enough mana of other colours: Crimson Hellkite's red-only {X} priced past
// the payer's red sources walked to the node cap for every such X.
//
// It is an upper bound like the total one: fees are ignored (they only
// spend) and every unused source counts at its best production of the
// colour. A colour another type of mana may pay -- a may-play rider ("mana of
// any color/type"), a spend-as-though conversion, or life for {B} -- is not
// bounded.
type ColourCap struct {
	need, held state.Mana
	best       []state.Mana
	// extra is each unit's best no-tap second activation (WindowUnit.Extras):
	// a unit already spent on its main activation can still take it.
	extra []state.Mana
	on    bool
}

// NewColourCap builds the bound for c over units (in the walk's order, the
// order its used flags index). held is the mana outside the walk's pool the
// cost may also spend (the snow and restricted pools).
func NewColourCap(c cost.Cost, held state.Mana, rider PipRider, conv *Conv, bLifeOK bool, units []WindowUnit) ColourCap {
	if rider.AnyColor || rider.AnyType || conv != nil {
		return ColourCap{}
	}
	k := ColourCap{need: c.Colored, held: held}
	if bLifeOK {
		k.need[state.ManaIndex('B')] = 0
	}
	if k.need.Total() == 0 {
		return ColourCap{}
	}
	k.on = true
	k.best = make([]state.Mana, len(units))
	k.extra = make([]state.Mana, len(units))
	mana := func(a WindowAlt) state.Mana {
		m := a.Mana()
		if a.Any {
			// An open choice may come out as any type of mana.
			t := m.Total()
			for j := range m {
				m[j] = t
			}
		}
		return m
	}
	for i, u := range units {
		for _, a := range u.Alts {
			m := mana(a)
			for j := range m {
				k.best[i][j] = max(k.best[i][j], m[j])
			}
		}
		// A no-tap self-sacrifice beside the unit's tap is a second
		// activation, so its colours are reachable too (cast_window.go),
		// even after the unit's main activation has been spent.
		for _, a := range u.Extras {
			m := mana(a)
			for j := range m {
				k.best[i][j] = max(k.best[i][j], m[j])
				k.extra[i][j] = max(k.extra[i][j], m[j])
			}
		}
	}
	return k
}

// Reaches reports whether pool, the held mana and the unused units' best
// productions could still cover every coloured pip.
func (k *ColourCap) Reaches(pool state.Mana, used []bool) bool {
	if !k.on {
		return true
	}
	for j, n := range k.need {
		if n == 0 {
			continue
		}
		have := int64(pool[j]) + int64(k.held[j])
		for i := range k.best {
			if have >= int64(n) {
				break
			}
			if !used[i] {
				have += int64(k.best[i][j])
			} else {
				// The main activation is spent; the unit's no-tap extra
				// (its self-sacrifice) is still activatable.
				have += int64(k.extra[i][j])
			}
		}
		if have < int64(n) {
			return false
		}
	}
	return true
}
