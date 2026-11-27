package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Commandeer (api ControlSpell Mode$ Gain; gorge effects/choose_control.go
// effControlSpell): "Gain control of target noncreature spell." The transfer
// is stack-scoped exactly as gorge's single ControlChange is (the primitive
// only emits it for a target whose zone is the stack): the named stack
// entry's controller becomes the resolving controller, so the artifact,
// enchantment or planeswalker the spell becomes enters under them
// (resolve.go's enter(k, e.obj, e.ctrl)). There is no duration and no
// control record: a stack entry has none, and the permanent keeps the
// controller it entered under. An object that is not on the stack (resolved,
// or never named) transfers nothing, as gorge's ZStack gate skips it.

// controlSpellExt is OpExt ExtControlSpell over Obj (the cast's spell
// target slot, or any object ref naming a stack entry).
//
//kmanifest:ext ExtControlSpell
func (s *State) controlSpellExt(k *prep, in *kop.Instr) (ok, done bool) {
	// ksrSubject is the shared "the stack entry in.Obj names, or -1": the
	// target slot's stack object with its announced version, or an object
	// ref's entry still on the stack (act_stackrewrite.go).
	if i := s.ksrSubject(k, in); i >= 0 && s.stack[i].kind == seSpell {
		// a stack ability is no object: gorge's Object-on-the-stack gate
		// transfers nothing, and the lowering admits only spell slots
		s.stack[i].ctrl = s.res.e.ctrl
	}
	return true, true
}
