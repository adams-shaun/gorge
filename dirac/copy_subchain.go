package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A copied spell's SubAbility$ chain declarations
// (CR 707.10c, gorge rules/target_modes.go copyDeclarationSplit,
// copyChainDeclarations, AskCopyTargets, resolution.go finishCopyTargets).
//
// A copy whose controller may choose new targets is asked one declaration at
// a time: the root's first, then each announced SubAbility$ link's in chain
// order (the slots kgen marks TgfDeclSub; a mid-resolution link is not one).
// Each ask offers that declaration's own cast-time targets first (kept even
// when illegal), then the link's other legal targets; a link that offers
// nothing is passed over, as is one whose bound admits no target. The
// answers are recorded together after the last: the root's replaces the
// root's slots (the chain's slots keep theirs), a link's replaces its own
// slot alone, and the crime and "becomes the target" reads see them all.
// The cursor is the declaration slot (res.left), so the chain walk adds no
// State.

// copySubSlot reports whether target slot j of e's ability is a chain
// link's copy declaration.
func (s *State) copySubSlot(k *prep, e *stackEntry, j int) bool {
	if e.kind != seSpell || e.ab == 255 {
		return false
	}
	t := tspecE(k, e, j)
	return t != nil && t.TFlags&kop.TgfDeclSub != 0
}

// copyHasSub reports whether e's ability declares a chain link's copy target.
func (s *State) copyHasSub(k *prep, e *stackEntry) bool {
	if e.ab == 255 || e.ab >= abCascade {
		return false
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	for i := 0; i < int(a.NTargets); i++ {
		if copySlotChosen(a, i, e) && copyLinkSlot(a, i, e.kind == seSpell) {
			return true
		}
	}
	return false
}

// copySlotChosen reports whether slot i belongs to no mode or to one e chose.
func copySlotChosen(a *kop.Ability, i int, e *stackEntry) bool {
	tm := a.TgtModes[i]
	return tm == 0 || tm&a.ModeSet(e.mode) != 0
}

// copyLinkSlot reports whether slot i is a SubAbility$ link's announced target:
// a plain spell's (TgfDeclSub), or one after its root's in a charm mode or an
// ability (gorge copyChainDeclarations: the links the original has a record for,
// and a slot held for resolution has none).
func copyLinkSlot(a *kop.Ability, i int, spell bool) bool {
	t := &a.Targets[i]
	if t.TFlags&kop.TgfAtResolution != 0 {
		return false
	}
	if a.NModes != 0 {
		return !copyModeDecl(a, i, spell)
	}
	if spell {
		return t.TFlags&kop.TgfDeclSub != 0
	}
	for j := 0; j < i; j++ {
		if a.Targets[j].TFlags&kop.TgfAtResolution == 0 {
			return true
		}
	}
	return false
}

// copyNextDecl is the declaration slot that follows slot after (-1: the
// first) in gorge's order: the root's or each chosen mode's, then the chain
// links' in chain order; -1 when none is left. A chain-only spell (an untargeted
// root) has no root declaration.
func (s *State) copyNextDecl(k *prep, e *stackEntry, after int) int {
	if after < 0 {
		if i := s.copyDeclSlot(k, e); i >= 0 {
			return i
		}
	}
	if e.ab == 255 || e.ab >= abCascade {
		return -1
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	spell := e.kind == seSpell
	if after < 0 || !copyLinkSlot(a, after, spell) {
		for i := after + 1; i < int(a.NTargets); i++ {
			if copySlotChosen(a, i, e) && copyModeDecl(a, i, spell) {
				return i
			}
		}
		after = -1
	}
	for i := after + 1; i < int(a.NTargets); i++ {
		if copySlotChosen(a, i, e) && copyLinkSlot(a, i, spell) {
			return i
		}
	}
	return -1
}

// copyModeDecl reports whether slot i is a charm mode's own declaration
// (copyCharmModes: each chosen target-bearing mode is one, in order), not a link
// of a mode an earlier slot already declares.
func copyModeDecl(a *kop.Ability, i int, spell bool) bool {
	t := &a.Targets[i]
	if a.NModes == 0 || t.TFlags&kop.TgfAtResolution != 0 || spell && t.TFlags&kop.TgfDecl == 0 {
		return false
	}
	for j := 0; j < i; j++ {
		if a.TgtModes[j] == a.TgtModes[i] {
			return false
		}
	}
	return true
}

// copyManyDecls reports whether e's copy election runs over two or more
// declarations, so its answers land in a working entry until the last closes.
func (s *State) copyManyDecls(k *prep, e *stackEntry) bool {
	d := s.copyNextDecl(k, e, -1)
	return d >= 0 && s.copyNextDecl(k, e, d) >= 0
}

// copyOwnSlot reports whether slot i of e's ability is answered alone: a chain
// link's, a charm mode's, or an ability's, whose chain keeps its targets.
func (s *State) copyOwnSlot(k *prep, e *stackEntry, i int) bool {
	if e.ab == 255 || e.ab >= abCascade {
		return false
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	return e.kind != seSpell || a.NModes != 0 || a.Targets[i].TFlags&kop.TgfDeclSub != 0
}

// copyAskFrom poses the first declaration at or after slot i that has
// something to ask; false = none is left.
func (s *State) copyAskFrom(k *prep, w *stackEntry, i int) bool {
	for ; i >= 0; i = s.copyNextDecl(k, w, i) {
		if s.copyAskSlot(k, w, i) {
			return true
		}
	}
	return false
}

// copyAskSlot poses declaration slot i's election (gorge AskCopyTargets for
// one stage); false = nothing to ask.
func (s *State) copyAskSlot(k *prep, top *stackEntry, i int) bool {
	s.res = resCtx{e: *top, active: true, lastTok: NoObj, eachObj: NoObj}
	var d Decision
	if s.copyTargetOptions(k, &d, i); d.N == 0 {
		s.res = resCtx{}
		return false // nothing to offer: gorge asks no empty target
	}
	lo, hi, multi := s.copyBounds(k, top, i, &d)
	if hi == 0 {
		s.res = resCtx{}
		return false // gorge resolves a no-target declaration silently
	}
	if multi || lo == 0 { // an "up to" declaration is closed by Done
		s.res.ask = raGenCopyTargets
		s.res.chosen, s.res.peekI = lo, hi
	} else {
		s.res.ask = raGenCopyTarget
	}
	s.res.askP, s.res.left = top.ctrl, uint8(i)
	s.ctl = ctlResolve
	return true
}

// copyKeepSlot reports whether the answer for declaration slot i leaves
// slot j's cast-time target alone: a mid-resolution slot, every slot when i
// is a chain link or a charm mode (its answer replaces its own slot only) or
// of an ability, and a chain link's slot when the root is answered.
func copyKeepSlot(a *kop.Ability, i, j int) bool {
	return a.Kind != kop.AbSpell || a.NModes != 0 || a.Targets[j].TFlags&(kop.TgfAtResolution|kop.TgfDeclSub) != 0 || a.Targets[i].TFlags&kop.TgfDeclSub != 0
}

// copyAnswered closes a copy's election after declaration slot i was
// answered into the working entry res.e with picks: it poses the next
// declaration (true), or, when none is left, writes the answers to the stack
// entry, reads the crime and announces every declaration's targets (false;
// gorge finishCopyTargets records the root's picks, then each link's, only
// after the last answer, so the stack object keeps its inherited targets, the
// chain's among them, until then).
func (s *State) copyAnswered(k *prep, i int, picks []Obj) bool {
	w := s.res.e
	if nx := s.copyNextDecl(k, &w, i); nx >= 0 && s.copyAskFrom(k, &w, nx) {
		return true
	}
	e := &s.stack[s.sp-1]
	e.tgt, e.tver, e.nt, e.xi = w.tgt, w.tver, w.nt, w.xi
	if k.gen != nil && k.gen.crimeOn {
		s.crimeCopy(k, e) // gorge resolution.go: the copy's answer is a crime for its controller
	}
	if !s.copyManyDecls(k, e) && !s.copyHasSub(k, e) {
		s.copyRetargeted(k, picks)
		return false
	}
	var all [MaxOptions]Obj
	n := 0
	for d := s.copyNextDecl(k, e, -1); d >= 0; d = s.copyNextDecl(k, e, d) {
		var own [MaxOptions]uint8
		for _, c := range own[:s.copySlotTargets(e, d, &own)] {
			if n < len(all) {
				all[n] = Obj(c)
				n++
			}
		}
	}
	s.copyRetargeted(k, all[:n])
	return false
}

// copyOwnTargets gives the working entry w its own multi-object targets
// when the copy has a chain declaration, so an answer's picks stay out of
// the stack object's (shown) table until the last declaration closes. False
// = the pool is full (the game stalls).
func (s *State) copyOwnTargets(k *prep, w *stackEntry) bool {
	if w.xi == 0 || !s.copyManyDecls(k, w) && !s.copyHasSub(k, w) {
		return true
	}
	from := s.g.xtp[w.xi-1]
	w.xi = 0
	xt := s.xtMut(w)
	if xt == nil {
		return false
	}
	*xt = from
	return true
}

// copySlotTargets lists slot d's own targets: the first, then its extras.
func (s *State) copySlotTargets(e *stackEntry, d int, out *[MaxOptions]uint8) int {
	n := 0
	if d < int(e.nt) && TCode(e.tgt[d]) != NoTarget {
		out[n] = e.tgt[d]
		n++
	}
	if e.xi != 0 {
		x := s.xtOf(e)
		for j := 0; j < int(x.n); j++ {
			if x.slot[j] == uint8(d) && n < MaxOptions {
				out[n] = x.code[j]
				n++
			}
		}
	}
	return n
}

// copyUniqueLink drops from a TgfDistinct chain link's election ("up to one
// other target creature") every target an earlier declaration slot of the
// working entry e holds -- the root's or a mode's, and an earlier distinct
// link's -- inherited keep-current ones included (CR 707.10c, 115.3; gorge
// rules/target_modes.go copyUniqueLinkCandidates). It returns the new count.
func (s *State) copyUniqueLink(k *prep, e *stackEntry, slot int, all *[MaxOptions]uint8, n int) int {
	if e.ab == 255 || e.ab >= abCascade {
		return n
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	if a.Targets[slot].TFlags&kop.TgfDistinct == 0 || !copyLinkSlot(a, slot, e.kind == seSpell) {
		return n
	}
	var held [MaxOptions]uint8
	nh := 0
	for j := 0; j < slot; j++ {
		t := &a.Targets[j]
		if t.TFlags&kop.TgfAtResolution != 0 || !copySlotChosen(a, j, e) {
			continue
		}
		if copyLinkSlot(a, j, e.kind == seSpell) && t.TFlags&kop.TgfDistinct == 0 {
			continue
		}
		var own [MaxOptions]uint8
		for _, c := range own[:s.copySlotTargets(e, j, &own)] {
			if nh < len(held) {
				held[nh] = c
				nh++
			}
		}
	}
	m := 0
next:
	for _, c := range all[:n] {
		for _, h := range held[:nh] {
			if c == h {
				continue next
			}
		}
		all[m] = c
		m++
	}
	return m
}
