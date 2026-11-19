package shadow

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/view"
)

// stack adds the entries the view's stack shows, bottom to top. The stack is
// public, so each entry's source and controller are as shown; an entry
// carrying a target, a mode, an X, a kick or a copy, or a trigger, is not
// rebuildable and returns ReasonStack. dirac.NewFromWorld decides whether the
// card of an entry is one it can rebuild.
func (b *builder) stack(w *dirac.World) error {
	for i := range b.in.View.Stack {
		sv := &b.in.View.Stack[i]
		if len(sv.Targets) != 0 || sv.X != 0 || sv.Kicks != 0 || len(sv.Modes) != 0 || sv.Optional || sv.Card == nil {
			return bad(ReasonStack, "entry %d (%s): %s", i, sv.Kind, sv.Name)
		}
		switch sv.Kind {
		case "spell":
			if err := b.spellEntry(w, i, sv); err != nil {
				return err
			}
		case "ability":
			if err := b.abilityEntry(w, sv); err != nil {
				return err
			}
		default:
			return bad(ReasonStack, "entry %d is a %q", i, sv.Kind)
		}
	}
	return nil
}

// spellEntry adds the spell sv: its card as an object in the stack zone,
// placed by its order on the stack.
func (b *builder) spellEntry(w *dirac.World, i int, sv *view.StackView) error {
	cv := sv.Card
	o, ok := b.in.ObjOf(int(cv.ID))
	if !ok {
		return bad(ReasonUnmapped, "%s (%d)", cv.Name, cv.ID)
	}
	if err := b.add(o, cv.Name, int(cv.Owner), i, kop.ZoneStack, cv); err != nil {
		return err
	}
	w.Stack = append(w.Stack, dirac.WorldStack{Obj: dirac.Obj(o), Ctrl: uint8(sv.Controller)})
	return nil
}

// abilityEntry adds the activated ability sv: its source is a permanent the
// view shows on a battlefield, whose one self-counter ability it is.
func (b *builder) abilityEntry(w *dirac.World, sv *view.StackView) error {
	o, ok := b.in.ObjOf(int(sv.Source))
	if !ok {
		return bad(ReasonUnmapped, "the source of %s (%d)", sv.Name, sv.Source)
	}
	id, ok := b.byName[sv.Card.Name]
	if !ok {
		return bad(ReasonUnknown, "%q", sv.Card.Name)
	}
	ab := dirac.SelfCounterAbility(&b.in.Tbl.Cards[id])
	if ab < 0 {
		return bad(ReasonStack, "%s has no one self-counter ability", sv.Card.Name)
	}
	w.Stack = append(w.Stack, dirac.WorldStack{Ability: true, Obj: dirac.Obj(o), Ctrl: uint8(sv.Controller), Ab: uint8(ab)})
	return nil
}
