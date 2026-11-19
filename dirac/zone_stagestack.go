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
		if int(e.Obj) >= len(w.Objs) || e.Ctrl >= 2 {
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

// validateStackPriority refuses the one priority an observation leaves
// ambiguous. The holder of a just-cast entry's controller has passed no one
// (passes 0); any other holder was passed to (1); but the active player also
// holds priority with no pass after a resolution, and the view does not tell
// the two apart when the top entry is the other seat's.
func (w *World) validateStackPriority() error {
	if n := len(w.Stack); n != 0 && w.Prio == w.Active && w.Stack[n-1].Ctrl != w.Active {
		return unstageable(ReasonPriority, "the active seat holds priority over the other seat's entry: a pass or a resolution")
	}
	return nil
}

// stackPasses is the pass count of the priority round at w's stack.
func (w *World) stackPasses() uint8 {
	if n := len(w.Stack); n != 0 && w.Stack[n-1].Ctrl == w.Prio {
		return 0
	}
	return 1
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
