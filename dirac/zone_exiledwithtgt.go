package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A card target that reads ExiledWithSource (Nyla, Shirshu Sleuth's end-step
// "target card exiled with this", Bronzebeak Foragers' and Gelatinous Cube's
// {X} abilities): the link is read at TARGET time, against the ability's own
// source, before any resolution (CR 406.6 / 607.2a). matchCardGen has no
// source then and exwFlag would fall to exwCard, which reads only a
// resolving entry. The same read runs again at the resolution recheck.

// chainFlag reports whether filter f's Or chain carries flag fl.
func chainFlag(t *kop.Table, f kop.FilterID, fl kop.FilterFlags) bool {
	for n := 0; f != 0 && int(f) < len(t.Filters) && n < len(t.Filters); n++ {
		if t.Filters[f].Flags&fl != 0 {
			return true
		}
		f = t.Filters[f].Or
	}
	return false
}

// matchCardExwTgt is matchCardGen with the target's source threaded in: the
// source object (self, else the slot's occupant) is the one the link is read
// against. While that source's own entry resolves, its entry rides along so
// a trigger's fire ver counts, as exwCard passes it.
func (s *State) matchCardExwTgt(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, src Slot, self Obj) bool {
	srcObj := self
	if srcObj == NoObj && src != NoSlot {
		srcObj = s.perms[src].obj
	}
	ctx := evalCtx{src: srcObj, srcSl: src, ctrl: ctrl}
	if s.res.active && srcObj != NoObj && s.res.e.obj == srcObj {
		ctx.e = &s.res.e
	}
	return s.matchCardGenAt(k, f, c, owner, ctrl, o, &ctx)
}
