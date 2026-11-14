package feat

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// entRel is one card's relational entity slots (policynet entity format 3):
// how many stack entries target it and, on a stack card, what its own entry
// targets, its X and whether it was kicked. The values are read off the
// build's view; stamp folds them into the cache key so a cached entity card
// is rebuilt when the relations move.
type entRel struct {
	tgtMe, tgtOpp       uint8
	permMe, permOpp     uint8
	playerMe, playerOpp uint8
	stack, grave        uint8
	x                   uint8
	kicked              bool
	combatFoes          uint8
	combatFoePow        int32
	combatFoeTgh        int32
	combatDies          bool
	combatKills         bool
}

// stamp is the relation's cache-key contribution.
func (r *entRel) stamp() uint64 {
	h := uint64(r.tgtMe) | uint64(r.tgtOpp)<<8 | uint64(r.permMe)<<16 | uint64(r.permOpp)<<24 |
		uint64(r.playerMe)<<32 | uint64(r.playerOpp)<<40 | uint64(r.stack)<<48 | uint64(r.grave)<<56
	var k uint64
	if r.kicked {
		k = 1
	}
	h = mix64(h, uint64(r.x)|k<<8)
	h = mix64(h, uint64(r.combatFoes))
	h = mix64(h, uint64(uint32(r.combatFoePow)))
	h = mix64(h, uint64(uint32(r.combatFoeTgh)))
	if r.combatDies {
		h = mix64(h, 1)
	}
	if r.combatKills {
		h = mix64(h, 2)
	}
	return h
}

// buildRel fills every listed card's relational slots from the build's view
// (policynet entityrel.go entityRelations, entity format 3). Target counts
// are per stack entry deduplicated; a stack card's own-entry slots count
// every target in its entry.
func (e *Encoder) buildRel(s *dirac.State) {
	v := &e.v
	// What the stack targets: every object target of every entry, split by
	// the entry's controller. Player targets and NoTarget do not count.
	for i := 0; i < int(e.kp.NStack); i++ {
		se := &e.kp.Stack[i]
		mine := se.Ctrl == v.seat
		var seen [16]dirac.TCode
		nseen := 0
		add := func(t dirac.TCode) {
			if t == dirac.NoTarget || t >= dirac.PlayerTarget0 {
				return
			}
			for j := 0; j < nseen; j++ {
				if seen[j] == t {
					return
				}
			}
			seen[nseen] = t
			nseen++
			for ci := range v.cards {
				if dirac.TCode(v.cards[ci].obj) != t {
					continue
				}
				if mine {
					v.cards[ci].rel.tgtMe++
				} else {
					v.cards[ci].rel.tgtOpp++
				}
			}
		}
		for _, t := range se.Targets {
			add(t)
		}
		for j := 0; j < int(se.NExtra); j++ {
			add(se.Extra[j])
		}
	}
	// A stack card's own entry: what its targets are, its X, its kicked bit.
	for i := 0; i < int(e.kp.NStack); i++ {
		ci := v.stack[i].cv
		if ci < 0 {
			continue
		}
		c := &v.cards[ci]
		se := &e.kp.Stack[i]
		classify := func(t dirac.TCode) {
			if t == dirac.NoTarget {
				return
			}
			if t >= dirac.PlayerTarget0 {
				if t == dirac.PlayerTarget0+dirac.TCode(v.seat) {
					c.rel.playerMe++
				} else {
					c.rel.playerOpp++
				}
				return
			}
			if host := e.bfCard(dirac.Obj(t)); host != nil {
				if host.ctrl == v.seat {
					c.rel.permMe++
				} else {
					c.rel.permOpp++
				}
				return
			}
			if e.stackHas(dirac.Obj(t)) {
				c.rel.stack++
				return
			}
			if e.zoneHas(dirac.Obj(t), zGraveyard) {
				c.rel.grave++
			}
		}
		for _, t := range se.Targets {
			classify(t)
		}
		for j := 0; j < int(se.NExtra); j++ {
			classify(se.Extra[j])
		}
		if _, _, building := s.BuildingCast(); !building || i < int(e.kp.NStack)-1 {
			c.rel.x = se.X // gorge writes Object.X as the cast commits, not as X is announced
		}
		c.rel.kicked = s.FeatStackKicked(i)
	}
	for ci := range v.cards {
		if c := &v.cards[ci]; c.zone == zBattlefield {
			e.combatRel(c)
		}
	}
}

// hasKw reports the effective keyword: a granted bit, or a printed one the
// layer walk has not removed.
func (c *vcard) hasKw(k kop.KwMask) bool {
	return c.kw&k != 0 || c.printedKw && c.c.kwMask&k != 0
}

// combatRel fills a battlefield card's combat slots (gorge entityrel.go
// combatSlots/combatFoes): an attacker's foes are the live blockers that
// block it, a blocker's foe is the attacker it blocks.
func (e *Encoder) combatRel(c *vcard) {
	var foes [dirac.MaxPerm]dirac.Obj
	n := 0
	if c.attacking {
		for bi := range e.v.cards {
			b := &e.v.cards[bi]
			if b.zone == zBattlefield && b.blocks == c.obj {
				foes[n] = b.obj
				n++
			}
		}
	} else if c.blocks != noObj {
		if a := e.bfCard(c.blocks); a != nil {
			foes[0] = a.obj
			n = 1
		}
	}
	if n == 0 {
		return
	}
	c.rel.combatFoes = uint8(n)
	var pow, tgh int32
	deathFoe := false
	for i := 0; i < n; i++ {
		foe := e.bfCard(foes[i])
		if foe == nil {
			continue
		}
		p := foe.power
		if p < 0 {
			p = 0
		}
		pow += p
		rem := foe.tough - foe.damage
		if rem < 0 {
			rem = 0
		}
		tgh += rem
		if p > 0 && foe.hasKw(kop.KwDeathtouch) {
			deathFoe = true
		}
	}
	c.rel.combatFoePow = pow
	c.rel.combatFoeTgh = tgh
	selfRem := c.tough - c.damage
	if !c.hasKw(kop.KwIndestructible) && ((pow > 0 && pow >= selfRem) || deathFoe) {
		c.rel.combatDies = true
	}
	if c.power > 0 {
		for i := 0; i < n; i++ {
			foe := e.bfCard(foes[i])
			if foe != nil && (c.hasKw(kop.KwDeathtouch) || c.power >= foe.tough-foe.damage) {
				c.rel.combatKills = true
				break
			}
		}
	}
}

// stackHas reports whether obj is a stack object (gorge entityrel.go
// stackHas over StackView.ID: a spell's stack object is its card object; the
// kernel has no separate ability objects, so an ability's source card is not
// a stack target).
func (e *Encoder) stackHas(obj dirac.Obj) bool {
	for i := 0; i < int(e.kp.NStack); i++ {
		if se := &e.kp.Stack[i]; se.Kind == 0 && se.Source == obj {
			return true
		}
	}
	return false
}

// zoneHas reports whether obj is listed in zone z.
func (e *Encoder) zoneHas(obj dirac.Obj, z uint8) bool {
	for i := range e.v.cards {
		if c := &e.v.cards[i]; c.obj == obj && c.zone == z {
			return true
		}
	}
	return false
}
