package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The referent filter words (T13b-2): kop.FwSharesCr and kop.FwSharesName,
// gorge's sharesCreatureTypeWith/sharesNameWith over the two-token referent
// argument (engine/effects/filter_attachment.go sharesTypeArg's vocabulary,
// resolved by sharesTypeReferents against the evaluating SpecContext). The
// referents gorge reads whose object the kernel keeps live are the entry's
// object targets, its remembered object, its triggering object, the reading
// source, and the source's imprint; the referents with no kernel record stay
// unlowered (kgen refuses them: Convoked rides stackEntry.prov as a count,
// and Sacrificed/Enchanted are not in gorge's sharesTypeArg vocabulary at
// all). Both words read each side's characteristics where it now is, like
// sharesCrTrigObj does for the triggering object.

// fwSharesCr is kop.FwSharesCr: the candidate shares a creature type with an
// object the referent names. Unbound referent: false (gorge's fail-closed
// sharesTypeReferents empty set).
//
//kmanifest:fword FwSharesCr
func (s *State) fwSharesCr(c *fctx, l *kop.FLit) bool {
	cand := s.candObj(c)
	if cand == NoObj {
		return false
	}
	om, oa := s.crTypesOf(c.k, cand)
	for _, r := range s.fwReferentObjs(c, l.R) {
		rm, ra := s.crTypesOf(c.k, r)
		if s.sharesCr(om, oa, rm, ra) {
			return true
		}
	}
	return false
}

// fwSharesName is kop.FwSharesName: the candidate shares a printed name with
// an object the referent names. The name identity is sharesNameSelf's: a
// battlefield permanent reads its layer-3 name (nameCardAt), anything else
// its card (nameKey), and a face-down candidate has no name (CR 708.2).
//
//kmanifest:fword FwSharesName
func (s *State) fwSharesName(c *fctx, l *kop.FLit) bool {
	if c.fd {
		return false
	}
	cn := NoCard
	if c.perm() {
		cn = s.nameCardAt(c.sl)
	} else if c.c != NoCard {
		cn = s.nameKey(c.c)
	}
	if cn == NoCard {
		return false
	}
	for _, r := range s.fwReferentObjs(c, l.R) {
		if rn := s.objName(r); rn != NoCard && rn == cn {
			return true
		}
	}
	return false
}

// candObj is the candidate's object for the fctx: the permanent's, or the
// card matcher's object. NoObj when the fctx has neither (fail closed).
func (s *State) candObj(c *fctx) Obj {
	if c.perm() {
		return s.perms[c.sl].obj
	}
	return c.o
}

// fwReferentObjs resolves the word's Ref to the live objects it names, read
// in the fctx's evaluation context (its own when the matcher bound one, else
// the resolving entry's). Empty when the referent is unbound — the fail
// closed both gorge callers share (an unbound referent matches nothing).
func (s *State) fwReferentObjs(c *fctx, r kop.Ref) []Obj {
	cx := c.ctx
	if !c.hasCtx {
		cx = s.ctxCur()
	}
	if r == kop.RefTargets0123 {
		// the entry's object targets, every slot (gorge Targeted's
		// TargetBinding, the object half): a slot the resolution did not
		// bind, a player target, or an id a new object has since taken is
		// no referent (CR 400.7)
		e := cx.e
		if e == nil {
			return nil
		}
		var out []Obj
		for i := 0; i < kop.MaxTargets; i++ {
			t := e.tgt[i]
			if e == &s.res.e {
				t = s.tgtCode(i)
			}
			if t == NoTarget || isPlayerOrNone(t) {
				continue
			}
			if ob := &s.objs[Obj(t)]; ob.ver != e.tver[i] {
				continue
			}
			out = append(out, Obj(t))
		}
		return out
	}
	if o := s.refObjCtx(r, cx); o != NoObj {
		return []Obj{o}
	}
	return nil
}
