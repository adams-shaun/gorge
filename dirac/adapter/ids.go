package adapter

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// idTable hands out the object ids a client sees. A deck card keeps its instance
// plus one, as gorge's genesis does. A token, a copy or a stack ability takes the
// next id when it first appears, and keeps it while it exists.
type idTable struct {
	next  state.ObjID
	objs  [dirac.MaxObj]state.ObjID
	gone  [dirac.MaxObj]bool
	stack [dirac.MaxStack]state.ObjID
	seen  [dirac.MaxStack]stackKey
	n     uint8
}

// stackKey tells whether a stack position still holds the same entry.
type stackKey struct {
	kind, ability uint8
	source        dirac.Obj
	card          dirac.CardID
}

// sync assigns ids to whatever appeared since the last call.
func (t *idTable) sync(kp *dirac.Projection) {
	if t.next == 0 {
		t.next = 1
	}
	for o := 0; o < int(kp.NObj); o++ {
		z := kp.Objs[o].Zone
		if t.objs[o] == 0 || t.gone[o] && z != kop.ZoneNone {
			t.objs[o] = t.next
			t.next++
		}
		t.gone[o] = z == kop.ZoneNone
	}
	for i := 0; i < int(kp.NStack); i++ {
		e := &kp.Stack[i]
		k := stackKey{e.Kind, e.Ability, e.Source, e.Card}
		if i >= int(t.n) || t.seen[i] != k {
			t.seen[i] = k
			t.stack[i] = 0
			if e.Kind != 0 {
				t.stack[i] = t.next
				t.next++
			}
		}
	}
	t.n = kp.NStack
}

// obj is kernel object o's id.
func (t *idTable) obj(o canon.Ref) state.ObjID {
	if int(o) >= len(t.objs) {
		return 0
	}
	return t.objs[o]
}

// ref is a canonical reference's id: 0 for a player or nothing.
func (t *idTable) ref(r canon.Ref, kp *dirac.Projection) state.ObjID {
	switch {
	case r >= canon.RefPlayer:
		return 0
	case r >= canon.RefStack:
		i := int(r - canon.RefStack)
		if i < int(kp.NStack) && kp.Stack[i].Kind == 0 {
			return t.obj(canon.Ref(kp.Stack[i].Source))
		}
		if i < len(t.stack) {
			return t.stack[i]
		}
		return 0
	}
	return t.obj(r)
}

// RefOf is the canonical reference id stands for, for a check comparing two engines'
// ids.
func (g *Game) RefOf(id state.ObjID) canon.Ref {
	if id == 0 {
		return canon.RefNone
	}
	for o := 0; o < int(g.kp.NObj); o++ {
		if g.ids.objs[o] == id {
			return canon.Ref(o)
		}
	}
	for i := 0; i < int(g.kp.NStack) && i < int(canon.RefPlayer-canon.RefStack); i++ {
		if g.ids.stack[i] == id {
			return canon.RefStack + canon.Ref(i)
		}
	}
	return canon.RefNone
}
