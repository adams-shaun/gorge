package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The mana value of the spell that triggered a SpellCast ability, as the spell
// is on the stack: the face it was cast as, with its announced {X} (CR 202.3e).
//
// The value is snapshotted from the triggering spell's stack entry when the
// trigger is queued (enqueueLKI): the spell can leave the stack before the
// trigger resolves, and stackExitFace turns an Adventure or split card back
// to its front face, which would otherwise lose the cast face.

// snapshotTrigSpellMV records the value on the pending trigger i just
// enqueued, when the ability is a SpellCast trigger and tobj is the spell on
// the stack (the cast path, a spell copy and an emblem's latent copy all
// queue through enqueueLKI). The stack entry itself may leave the stack, and
// its card be turned back to a front face, before the trigger resolves.
func (s *State) snapshotTrigSpellMV(i uint8, c CardID, ab uint8, tobj Obj) {
	k := s.k()
	if k == nil || k.gen == nil || tobj == NoObj || s.objs[tobj].zone != kop.ZoneStack {
		return
	}
	cd := &k.t.Cards[c]
	if int(ab) >= len(cd.Abilities) || cd.Abilities[ab].Trig.Event != kop.TrigSpellCast {
		return
	}
	s.pend[i].trigMV = objWord(s.triggerSpellMVLKI(k, &stackEntry{kind: seSpell, obj: tobj, card: s.objs[tobj].card}))
}

func (s *State) triggerSpellMVLKI(k *prep, e *stackEntry) uint8 {
	c := e.card
	mv := int(k.cards[c].total)
	if k.gen.anySplit {
		mv = s.splitMV(k, c, e.obj, mv)
	}
	return uint8(mv + s.stackXMV(e.obj, k.cards[c].cost.X))
}
