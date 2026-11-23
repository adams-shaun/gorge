package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// zone_stagestack.go: the stack a World may hold. The stack is public, so an
// observation shows every entry; what it admits is the entries whose whole
// state dirac can rebuild from a View:
//
//   - a spell with no target, no mode, no X and no kicker: a token-making
//     instant or sorcery, a creature or an Equipment, whose card is an
//     object of Objs in ZoneStack;
//   - an Aura spell stageableAttachment admits, with its one target, a
//     creature on a battlefield;
//   - an activated ability of the shape selfCounterAbility admits, whose
//     source is a battlefield creature of Objs (its cost, a mana payment and
//     a sacrificed creature, shows in the permanents and the graveyard);
//   - the Equip ability of an Equipment stageableAttachment admits, whose
//     source is a battlefield permanent its controller controls, with its one
//     target, a creature that controller controls.
//
// A triggered ability, any other target, a mode, an X or a copy is not
// rebuildable (no card admitted by stageableCard has a trigger) and stays
// refused. A table with the crime mechanic refuses a targeted entry: the
// crime ledger is not in a view (initFromWorld).

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
	// NT is the number of targets and Tgt their objects (indices of Objs),
	// the first NT entries.
	NT  uint8
	Tgt [kop.MaxTargets]TCode
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

// EquipAbility is the index of def's one Equip ability (kop.AfEquip), or -1
// when it has none or several (the view does not say which is on the stack).
func EquipAbility(def *kop.CardDef) int {
	at := -1
	for i := range def.Abilities {
		if ab := &def.Abilities[i]; ab.Kind == kop.AbActivated && ab.AFlags&kop.AfEquip != 0 {
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
	if def.Types&kop.TLand != 0 {
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
			if err := w.validateAbilityEntry(i, e, o, def); err != nil {
				return err
			}
			continue
		}
		if !onStack[e.Obj] || e.Ctrl != o.Owner || o.Token || stageableSpell(def) != "" {
			return unstageable(ReasonStackEntry, "entry %d: not a spell the stack may hold: %s", i, def.Name)
		}
		if err := w.validateTargets(i, e, def.Types&kop.TAura != 0, false); err != nil {
			return err
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

// validateAbilityEntry checks the activated ability entry e, whose source is o
// (card def): a self-counter ability of a battlefield creature, or the Equip
// ability of a battlefield Equipment its controller controls.
func (w *World) validateAbilityEntry(i int, e *WorldStack, o *WorldObj, def *kop.CardDef) error {
	if o.Zone != kop.ZoneBattlefield || e.Ctrl != o.Ctrl {
		return unstageable(ReasonStackEntry, "entry %d: the source is not the controller's battlefield permanent", i)
	}
	if def.Types&kop.TCreature != 0 && int(e.Ab) == SelfCounterAbility(def) {
		return w.validateTargets(i, e, false, false)
	}
	if def.Types&kop.TEquipment != 0 && !o.Token && stageableAttachment(def) == "" && int(e.Ab) == EquipAbility(def) {
		return w.validateTargets(i, e, true, true)
	}
	return unstageable(ReasonStackEntry, "entry %d: not a self-counter ability of a battlefield creature or an Equip ability", i)
}

// validateTargets checks e's targets: none, unless one is wanted (an Aura spell
// or an Equip ability), then at most one, a creature on a battlefield, which for
// an Equip ability e's controller controls. A view never shows such an entry
// with none (the cast or activation chose it); a World that does stages it as
// given, so a verifier's mutation that drops the target is judged by the replay.
func (w *World) validateTargets(i int, e *WorldStack, wantOne, own bool) error {
	if !wantOne {
		if e.NT != 0 {
			return unstageable(ReasonStackEntry, "entry %d: a target on an entry that takes none", i)
		}
		return nil
	}
	if e.NT > 1 {
		return unstageable(ReasonStackEntry, "entry %d: %d targets", i, e.NT)
	}
	if e.NT == 0 {
		return nil
	}
	if int(e.Tgt[0]) >= len(w.Objs) {
		return unstageable(ReasonStackEntry, "entry %d: target %d", i, e.Tgt[0])
	}
	t := &w.Objs[e.Tgt[0]]
	if t.Zone != kop.ZoneBattlefield || w.Tbl.Cards[t.Card].Types&kop.TCreature == 0 || own && t.Ctrl != e.Ctrl {
		return unstageable(ReasonStackEntry, "entry %d: the target is not a creature on a battlefield it may name", i)
	}
	return nil
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
		// a target is legal at resolution while its ver is the one it has now
		// (resolve.go); the entry's crime bits stay zero (initFromWorld)
		for j := 0; j < int(we.NT); j++ {
			e.tgt[j], e.tver[j] = we.Tgt[j], s.objs[we.Tgt[j]].ver
		}
		e.nt = we.NT
		s.push(e)
	}
}
