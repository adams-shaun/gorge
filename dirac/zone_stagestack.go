package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// zone_stagestack.go: the stack a World may hold. The stack is public, so an
// observation shows every entry; what it admits is the entries whose whole
// state dirac can rebuild from a View:
//
//   - a spell with no target, no mode, no X and no kicker: a token-making
//     instant or sorcery, a creature or an Equipment, whose card is an
//     object of Objs in ZoneStack (an Aura targets, and stays refused);
//   - an activated ability of the shape selfCounterAbility admits, whose
//     source is a battlefield creature of Objs (its cost, a mana payment and
//     a sacrificed creature, shows in the permanents and the graveyard).
//
// A triggered ability, a target, a mode, an X or a copy is not rebuildable
// (no card admitted by stageableCard has a trigger) and stays refused.

// ReasonStackEntry is the reason of a stack entry NewFromWorld cannot rebuild.
const ReasonStackEntry = "stack-entry"

// WorldStack is one stack entry of a World. Entries run bottom to top.
type WorldStack struct {
	// Ability marks an activated ability; the entry is a spell otherwise.
	Ability bool
	// Obj is the spell's object (in ZoneStack in Objs) or the ability's source.
	Obj  Obj
	Ctrl uint8
	// Ab is an ability's index in its source's CardDef.
	Ab uint8
}

// SelfCounterAbility is the index of def's one self +1/+1 counter ability,
// or -1 when it has none or several (the view does not say which is on the
// stack).
func SelfCounterAbility(def *kop.CardDef) int {
	at := -1
	for i := range def.Abilities {
		if selfCounterAbility(&def.Abilities[i]) {
			if at >= 0 {
				return -1
			}
			at = i
		}
	}
	return at
}

// stageableSpell reports why def, a card of Objs, is not a spell the stack may
// hold ("" if it is).
func stageableSpell(def *kop.CardDef) string {
	if def.Types&(kop.TLand|kop.TAura) != 0 {
		return "type"
	}
	return stageableCard(def, false)
}

// validateStack checks w.Stack against w.Objs. It returns the ErrUnstageable
// or nil.
func (w *World) validateStack() error {
	if len(w.Stack) > MaxStack {
		return unstageable(ReasonStackEntry, "%d entries", len(w.Stack))
	}
	var onStack [MaxObj]bool
	for i := range w.Objs {
		if w.Objs[i].Zone == kop.ZoneStack {
			onStack[i] = true
		}
	}
	for i := range w.Stack {
		e := &w.Stack[i]
		if int(e.Obj) >= len(w.Objs) || e.Ctrl >= w.NSeat {
			return unstageable(ReasonStackEntry, "entry %d: object %d controller %d", i, e.Obj, e.Ctrl)
		}
		o := &w.Objs[e.Obj]
		def := &w.Tbl.Cards[o.Card]
		if e.Ability {
			if o.Zone != kop.ZoneBattlefield || def.Types&kop.TCreature == 0 || e.Ctrl != o.Ctrl ||
				int(e.Ab) != SelfCounterAbility(def) {
				return unstageable(ReasonStackEntry, "entry %d: not a self-counter ability of a battlefield creature", i)
			}
			continue
		}
		if !onStack[e.Obj] || e.Ctrl != o.Owner || o.Token || stageableSpell(def) != "" {
			return unstageable(ReasonStackEntry, "entry %d: not a spell the stack may hold: %s", i, def.Name)
		}
		onStack[e.Obj] = false
	}
	for i, on := range onStack {
		if on {
			return unstageable(ReasonStackEntry, "object %d is on the stack and in no entry", i)
		}
	}
	return w.validateStackPriority()
}

// validateStackPriority refuses every priority over a stack whose top entry is
// not the active seat's. Such a round may have started at the entry's
// controller after a cast or activation (no pass) or at the active seat after
// a resolution (also no pass); a view shows neither, and the two differ by one
// pass as soon as the active seat passes.
func (w *World) validateStackPriority() error {
	if n := len(w.Stack); n != 0 && w.Stack[n-1].Ctrl != w.Active {
		return unstageable(ReasonPriority, "the top entry is not the active seat's: the round may have started at it or after a resolution")
	}
	return nil
}

// roundPasses is the pass count of the priority round at w. Every round starts
// at the active seat with no pass (openPriority, afterResolve) and each pass
// moves to the next seat in the game (nextAlive), so the holder has been passed
// to by the seats not lost from Active up to it. A stack's top entry is the
// active seat's (validateStackPriority), so the same count holds over a stack.
func (w *World) roundPasses() uint8 {
	n := uint8(0)
	for p := w.Active; p != w.Prio && n < MaxSeats; n++ {
		p = w.nextAlive(p)
	}
	return n
}

// nextAlive is the first seat after p in turn order that has not left.
func (w *World) nextAlive(p uint8) uint8 {
	for i := uint8(1); i < w.NSeat; i++ {
		if q := (p + i) % w.NSeat; !w.Lost[q] {
			return q
		}
	}
	return p
}

// prevAlive is the last seat before p in turn order that has not left.
func (w *World) prevAlive(p uint8) uint8 {
	for i := uint8(1); i < w.NSeat; i++ {
		if q := (p + w.NSeat*2 - i) % w.NSeat; !w.Lost[q] {
			return q
		}
	}
	return p
}

// aliveN is the number of seats that have not left.
func (w *World) aliveN() uint8 {
	n := uint8(0)
	for p := uint8(0); p < w.NSeat; p++ {
		if !w.Lost[p] {
			n++
		}
	}
	return n
}

// stageStack puts w's stack entries on s, bottom to top.
func (s *State) stageStack(k *prep, w *World) {
	for i := range w.Stack {
		we := &w.Stack[i]
		ob := &s.objs[we.Obj]
		s.touch(we.Obj)
		e := stackEntry{kind: seSpell, obj: we.Obj, card: ob.card, ab: spellAb(k, ob.card), ctrl: we.Ctrl, trigObj: NoObj, tgt: noTargets}
		if we.Ability {
			e.kind, e.ab = seAbility, we.Ab
		} else {
			ob.zone = kop.ZoneStack
			ob.ver++
			if k.gen != nil && k.gen.anyCastProv {
				// every card staged is cast from the hand: nothing admitted grants another way
				e.prov = uint8(kop.ZoneHand)
			}
		}
		e.srcVer = ob.ver
		s.push(e)
	}
}
