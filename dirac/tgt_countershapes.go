package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Counter's target shapes (Spider-Sense's TgtStackObjTrig,
// Fugitive Droid's TgtSpellTgting) and its RememberCounteredCMC$ /
// RememberCountered$ riders (Hurl into History, Thranduil's Decree). Reached
// only from a slot of those kinds or an OpCounter with B set.

// DebugCtrShapesOff (mutation, TestCtrshapesLockstepMutation):
// TgtStackObjTrig admits an activated ability too, and TgtSpellTgting ignores
// the targeting clause (any spell).
var DebugCtrShapesOff bool

// ctrShapeKind reports whether target kind kd is TgtStackObjTrig or
// TgtSpellTgting.
func ctrShapeKind(kd kop.TargetKind) bool {
	return kd == kop.TgtStackObjTrig || kd == kop.TgtSpellTgting
}

// trigOnlyOK is TgtStackObjTrig's verdict for a stack ability entry of the
// given kind: a triggered ability only (gorge StackKindTokenOf Triggered).
func trigOnlyOK(kind uint8) bool {
	return kind == seTrigger || DebugCtrShapesOff && kind != seSpell
}

// spellTgtingOK is targetOK for a TgtSpellTgting slot: a spell other than the
// source whose own object targets include a battlefield object matching the
// slot's Filter for the chooser p (gorge filterTargetValidTargeting, the offer
// only; an ability on the stack is not a candidate).
//
//go:noinline
func (s *State) spellTgtingOK(k *prep, t *kop.TargetSpec, p uint8, self Obj, code uint8) bool {
	if !isObjTarget(code) {
		return false
	}
	o := Obj(code)
	if s.objs[o].zone != kop.ZoneStack || o == self {
		return false
	}
	i := s.stackIndexOf(o)
	if i < 0 || s.stack[i].kind != seSpell {
		return false
	}
	if DebugCtrShapesOff {
		return true
	}
	if t.SpellF != 0 && !s.matchCardAt(k, t.SpellF, s.objs[o].card, s.stackCtrl(o), p, o) {
		return false // the spell side's own filter (Avoid Fate: Instant,Aura)
	}
	e := &s.stack[i]
	for _, c := range e.tgt {
		if s.tgtClauseMatch(k, t, p, c) {
			return true
		}
	}
	x := s.xtOf(e)
	for j := 0; j < int(x.n); j++ {
		if s.tgtClauseMatch(k, t, p, x.code[j]) {
			return true
		}
	}
	return false
}

// tgtClauseMatch: target byte c satisfies slot t's targeting clause for
// chooser p: the chooser's own player (VYou, gorge MatchesPlayerSpec "You"),
// else a battlefield permanent matching the slot's Filter.
func (s *State) tgtClauseMatch(k *prep, t *kop.TargetSpec, p uint8, c uint8) bool {
	if t.VYou {
		return isPlayerTarget(c) && CodeSeat(c) == p
	}
	return s.tgtBattlefieldMatch(k, t.Filter, p, c)
}

// tgtBattlefieldMatch: target byte c names a permanent on the battlefield now
// that matches f for chooser p (a player, a stack object or an empty slot
// matches nothing).
func (s *State) tgtBattlefieldMatch(k *prep, f kop.FilterID, p uint8, c uint8) bool {
	if !isObjTarget(c) {
		return false
	}
	ob := &s.objs[c]
	return ob.zone == kop.ZoneBattlefield && s.matchPerm(k, f, ob.slot, p, NoSlot)
}

// spellTgtingStill is targetStillLegal for a TgtSpellTgting slot: gorge does
// not reread the targeting clause at resolution, so the target is a plain
// spell target (still a spell on the stack, never the entry itself).
func (s *State) spellTgtingStill(e *stackEntry, code uint8) bool {
	if !isObjTarget(code) || Obj(code) == e.obj || s.objs[code].zone != kop.ZoneStack {
		return false
	}
	i := s.stackIndexOf(Obj(code))
	return i >= 0 && s.stack[i].kind == seSpell
}

// counterRiders runs OpCounter's B riders for the spell at stack index i,
// just before it leaves the stack (gorge effCounter adds it only for a spell
// that actually moves): CtrRemMV adds its mana value (announced X included) to
// the resolution's sum.
//
//go:noinline
func (s *State) counterRiders(k *prep, i int, bits int) {
	e := &s.stack[i]
	if e.kind != seSpell {
		return
	}
	if bits&kop.CtrRemember != 0 && !DebugBluffNoRemember {
		s.remember(e.obj)
	}
	if bits&kop.CtrRemMV != 0 && !s.faceDownAt(k, e.obj) {
		v := int(s.res.remLK) + s.objMV(k, s.objs[e.obj].card, e.obj)
		if v > 32767 {
			v = 32767
		}
		s.res.remLK = int16(v)
	}
}
