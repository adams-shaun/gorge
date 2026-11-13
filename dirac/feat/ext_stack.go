package feat

import (
	"strconv"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Group S (ExtStackTarget), slots 112-399: what each stack spell does,
// crossed with what it aims at; for a creature target, the target's
// strength bucket crossed with whether resolution kills it (never a bare kill
// flag); whose best creature is hit, combatants, X and kicker; counterspells
// crossed with the countered spell's strength. Only Kind-0 (spell) entries are
// classified; an ability counts as class EffOther in S1 and sets nothing else.
// Approximations (the pool's EffectSpec): the first spell ability only, no
// SubAbility$ chain, no modes.

// S slot bases.
const (
	xS1          = 112 // 112 + (side*7 + eff)*8 + tgt
	xS2          = 224 // 224 + (((side*5+q)*2+tside)*4+b)*2 + kills
	xSKillsBest  = 384 // opp kills my best; 385 the mirror
	xSCombatant  = 386 // opp hits my combatant
	xSPumpCombat = 387 // my pump on my combatant
	xSKicked     = 388 // me, opp
	xSX          = 390 // me, opp
	xSCounters   = 392 // me 392..395, opp 396..399
	xSEnd        = 400
)

// Target classes of S1.
const (
	tcPlayerMe = iota
	tcPlayerOpp
	tcCreatureMe
	tcCreatureOpp
	tcPermMe
	tcPermOpp
	tcStack
	tcGrave // graveyard or exile
	tcNone  = -1
)

// s2Q maps an effect class to S2's q (kill, damage, bounce, pump, debuff), -1 for none.
var s2Q = [...]int8{EffKill: 0, EffDamage: 1, EffCounter: -1, EffBounce: 2, EffPump: 3, EffDebuff: 4, EffOther: -1}

// liveStrength is a battlefield creature's live strength:
// max(P,0) + max(T-damage,0) + 2*min(kw,3) + mv/2, keywords as hasKw reads them.
func liveStrength(c *vcard) int32 {
	var n int32
	for k := kop.KwMask(1); k != 0 && k <= strengthKw; k <<= 1 {
		if strengthKw&k != 0 && c.hasKw(k) {
			n++
		}
	}
	return max(c.power, 0) + max(c.tough-c.damage, 0) + 2*min(n, 3) + c.c.mv/2
}

// resolvesKills reports whether a spell of effect ef (its X x) kills creature t.
func resolvesKills(ef *EffectSpec, x int32, t *vcard) bool {
	rem := t.tough - t.damage
	indestr := t.hasKw(kop.KwIndestructible)
	switch ef.Class {
	case EffKill:
		return ef.Exile || !indestr
	case EffDamage:
		amt := int32(ef.Amount)
		if ef.Amount == -1 {
			amt = x
		} else if ef.Amount < 0 {
			return false
		}
		return amt >= rem && !indestr
	case EffDebuff:
		return -int32(ef.NumDef) >= rem
	}
	return false
}

// stackPstr is a stack spell's printed strength with its stack MV standing in for mv.
func stackPstr(c *vcard) int32 {
	smv, mv := stackMV(c), c.c.mv
	switch {
	case c.c.fold&tbCreature != 0:
		return c.c.pstr + smv/2 - mv/2
	case c.c.fold&tbLand != 0:
		return c.c.pstr
	}
	return c.c.pstr + 2*(smv-mv)
}

// stackEntryOf is the Kind-0 stack entry index whose spell object is obj, -1 for none.
func (e *Encoder) stackEntryOf(obj uint8) int {
	for i := 0; i < int(e.kp.NStack); i++ {
		if se := &e.kp.Stack[i]; se.Kind == 0 && uint8(se.Source) == obj {
			return i
		}
	}
	return -1
}

// targetClass classifies target byte t for seat as buildRel does; host is
// the battlefield card for classes 2-5.
func (e *Encoder) targetClass(seat, t uint8) (int, *vcard) {
	if dirac.TCode(t) >= dirac.PlayerTarget0 {
		if dirac.TCode(t) == dirac.PlayerTarget0+dirac.TCode(seat) {
			return tcPlayerMe, nil
		}
		return tcPlayerOpp, nil
	}
	if host := e.bfCard(t); host != nil {
		mine := host.ctrl == seat
		switch {
		case host.creature() && mine:
			return tcCreatureMe, host
		case host.creature():
			return tcCreatureOpp, host
		case mine:
			return tcPermMe, host
		}
		return tcPermOpp, host
	}
	if e.stackHas(t) {
		return tcStack, nil
	}
	if e.zoneHas(t, zGraveyard) || e.zoneHas(t, zExile) {
		return tcGrave, nil
	}
	return tcNone, nil
}

// extStack writes group S for seat.
func (e *Encoder) extStack(seat uint8, me, opp *vplayer) {
	v := &e.v
	// each side's max-str creature's strength (-1: none)
	best := [2]int32{-1, -1}
	for s, pv := range [2]*vplayer{me, opp} {
		for _, ci := range pv.bf {
			if c := &v.cards[ci]; c.creature() {
				best[s] = max(best[s], liveStrength(c))
			}
		}
	}
	var kicked, xsum [2]int32
	var killsBest [2]bool
	hitCombatant, pumpCombatant := false, false
	for i := 0; i < int(e.kp.NStack) && i < len(v.stack); i++ {
		se := &e.kp.Stack[i]
		side := 0
		if se.Ctrl != seat {
			side = 1
		}
		var c *vcard
		eff := uint8(EffOther)
		var ef *EffectSpec
		if se.Kind == 0 && v.stack[i].cv >= 0 {
			c = &v.cards[v.stack[i].cv]
			ef = &c.c.eff
			eff = ef.Class
			if c.rel.kicked {
				kicked[side]++
			}
			xsum[side] += int32(c.rel.x)
		}
		var seen [kop.MaxTargets + len(se.Extra)]uint8
		nseen := 0
		n := len(se.Targets) + int(se.NExtra)
		for j := 0; j < n; j++ {
			t := se.Extra[max(j-len(se.Targets), 0)]
			if j < len(se.Targets) {
				t = se.Targets[j]
			}
			if dirac.TCode(t) == dirac.NoTarget {
				continue
			}
			dup := false
			for k := 0; k < nseen; k++ {
				dup = dup || seen[k] == t
			}
			if dup {
				continue
			}
			seen[nseen] = t
			nseen++
			tc, host := e.targetClass(seat, t)
			if tc == tcNone {
				continue
			}
			e.extAdd(xS1+(side*7+int(eff))*8+tc, 1)
			if c == nil {
				continue // an ability: S1 only
			}
			if tc == tcStack && eff == EffCounter {
				if k := e.stackEntryOf(t); k >= 0 && e.kp.Stack[k].Ctrl != se.Ctrl && v.stack[k].cv >= 0 {
					b := StrengthBucket(stackPstr(&v.cards[v.stack[k].cv]))
					e.extAdd(xSCounters+4*side+int(b), 1)
				}
			}
			if host == nil || (tc != tcCreatureMe && tc != tcCreatureOpp) {
				continue
			}
			q := s2Q[eff]
			if q < 0 {
				continue
			}
			tside := 0
			if host.ctrl != seat {
				tside = 1
			}
			str := liveStrength(host)
			kills := resolvesKills(ef, int32(c.rel.x), host)
			ki := 0
			if kills {
				ki = 1
			}
			e.extAdd(xS2+((((side*5+int(q))*2+tside)*4+int(StrengthBucket(str)))*2+ki), 1)
			removal := eff == EffKill || eff == EffDamage || eff == EffDebuff
			if removal && kills && side != tside && str == best[tside] {
				killsBest[1-side] = true // by slot: side 1 hits my best (384), side 0 theirs (385)
			}
			combatant := host.attacking || host.blocks != noObj
			if combatant && tside == 0 {
				if side == 1 {
					hitCombatant = true
				} else if eff == EffPump {
					pumpCombatant = true
				}
			}
		}
	}
	for s := 0; s < 2; s++ {
		if killsBest[s] {
			e.extAdd(xSKillsBest+s, 1)
		}
		if kicked[s] > 0 {
			e.extAdd(xSKicked+s, float32(kicked[s]))
		}
		if xsum[s] > 0 {
			e.extAdd(xSX+s, float32(xsum[s])/5)
		}
	}
	if hitCombatant {
		e.extAdd(xSCombatant, 1)
	}
	if pumpCombatant {
		e.extAdd(xSPumpCombat, 1)
	}
}

var s1Eff = [...]string{"kill", "damage", "counter", "bounce", "pump", "debuff", "other"}
var s1Tgt = [...]string{"player-me", "player-opp", "creature-me", "creature-opp", "perm-me", "perm-opp", "stack", "grave-exile"}
var s2QName = [...]string{"kill", "damage", "bounce", "pump", "debuff"}
var sSide = [...]string{"me", "opp"}

// extStackSlotName names an S slot (112..399).
func extStackSlotName(n int) string {
	switch {
	case n < xS2:
		k := n - xS1
		return "S|" + sSide[k/56] + "|" + s1Eff[k/8%7] + "|" + s1Tgt[k%8]
	case n < xSKillsBest:
		k := n - xS2
		return "S|" + sSide[k/80] + "|" + s2QName[k/16%5] + "|" + sSide[k/8%2] + "|str=" + strconv.Itoa(k/2%4) + "|kills=" + strconv.Itoa(k%2)
	case n < xSCounters:
		return [...]string{"S|opp-kills-my-best", "S|me-kills-their-best", "S|opp-hits-my-combatant",
			"S|me-pumps-my-combatant", "S|kicked|me", "S|kicked|opp", "S|x|me", "S|x|opp"}[n-xSKillsBest]
	case n < xSCounters+4:
		return "S|me-counters|str=" + strconv.Itoa(n-xSCounters)
	}
	return "S|opp-counters|str=" + strconv.Itoa(n-xSCounters-4)
}
