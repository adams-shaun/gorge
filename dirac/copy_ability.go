package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's CopySpellAbility with TargetType$
// Activated.YouCtrl,Triggered.YouCtrl (or Triggered.YouCtrl): "copy target
// activated or triggered ability you control; you may choose new targets for
// the copy" (Echo, Perceptive Prodigy; Scientist Supreme of A.I.M.; Kirol,
// Attentive First-Year). The target is a stack ability named as a
// TgtStackAb names one (StackTarget0 + its index, found again by its stack
// identity); the copy is an ordinary stack entry that is not activated or
// cast (CR 707.10), so no cost is paid and no activation tally moves.

// DebugCopyAbNoRetarget makes the copy ignore MayChooseTarget$: it keeps the
// original's targets (the copyab mutation test).
var DebugCopyAbNoRetarget bool

// DebugCopyAbOneCopy makes the copy count one whatever Amount$ says, and
// DebugCopyAbIgnoreCantCopy copies an ability that can't be copied (the
// copyab amount mutation test).
var DebugCopyAbOneCopy, DebugCopyAbIgnoreCantCopy bool

// copyAbKind reports whether target kind kd is a TgtCopyAb or TgtCopyTrig.
func copyAbKind(kd kop.TargetKind) bool { return kd == kop.TgtCopyAb || kd == kop.TgtCopyTrig }

// copyAbOK is stackTgtOK's verdict for a TgtCopyAb or TgtCopyTrig slot on the
// stack entry at index i: an ability the chooser p controls (a triggered one
// for TgtCopyTrig) whose source's card types match the slot's Filter (gorge
// stackKindAdmits, then matchesTargetSpec binding the wrapper's types to its
// source's, CR 113.7a). The resolving entry is never a candidate: the caller
// has excluded stackTgtSelf (CR 115.5).
func (s *State) copyAbOK(k *prep, t *kop.TargetSpec, p uint8, i int) bool {
	e := &s.stack[i]
	if e.kind == seSpell || e.ctrl != p || t.Kind == kop.TgtCopyTrig && e.kind != seTrigger {
		return false
	}
	ty := k.t.Filters[t.Filter].Types
	return ty == 0 || s.abSrcTypes(k, e)&ty != 0
}

// abSrcTypes is the card types of ability entry e's source as gorge reads
// them: the permanent's current types while it is on the battlefield, else its
// card's printed types, a token's included: a Clue sacrificed to pay for its
// ability was an artifact when it left, and the ability on the stack is still
// from an artifact source (CR 113.7a).
func (s *State) abSrcTypes(k *prep, e *stackEntry) kop.TypeMask {
	ob := &s.objs[e.obj]
	if ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer {
		return s.typesAt(k, ob.slot)
	}
	return k.cards[e.card].types
}

// copyDeclSlotAb is copyDeclSlot for an ability copy: gorge's declaration is the
// ability itself (copyRootDeclarations: o.Ability), or each chosen mode of a charm
// (copyCharmModes), so the first is the ability's first cast-time slot of a mode it
// chose. copyNextDecl walks the chain links after it.
func (s *State) copyDeclSlotAb(k *prep, e *stackEntry) int {
	if !k.gen.anyCopyAb {
		return -1
	}
	if e.ab >= abCascade {
		return -1 // a kernel-native ability has no CardDef declaration
	}
	a := &k.t.Cards[e.card].Abilities[e.ab]
	for i := 0; i < int(a.NTargets); i++ {
		if a.Targets[i].TFlags&kop.TgfAtResolution != 0 {
			continue
		}
		if tm := a.TgtModes[i]; tm != 0 && tm&a.ModeSet(e.mode) == 0 {
			continue // a mode the copied ability did not choose
		}
		return i
	}
	return -1
}

// copyAbility is ExtCopyAbility: in.B copies (Amount$, read once) of the stack
// ability that target slot in.C>>1 names, controlled by the resolving
// controller, each minted below the resolving entry (which leaves the top when
// it finishes) with the original's source, ability, X, modes, targets and
// trigger context. C bit 0 (MayChooseTarget$) poses the new-target election
// after each copy (false: posed). A target no longer on the stack, or an
// ability that can't be copied, copies nothing.
func (s *State) copyAbility(k *prep, in *kop.Instr) bool {
	n := s.value(k, in.B)
	if DebugCopyAbOneCopy && n > 1 {
		n = 1
	}
	return s.copyAbilityN(k, in, min(n, 255))
}

// copyAbilityN makes n copies, one at a time as gorge does: mint, ask, mint,
// ask. Each pass finds the target again by its stack identity, because the
// copies already made shift the indices; copyResume re-enters it with the
// copies still owed once an election closes.
func (s *State) copyAbilityN(k *prep, in *kop.Instr, n int) bool {
	slot := int(in.C >> 1)
	e := &s.res.e
	for c := n - 1; c >= 0; c-- {
		tc := s.tgtCode(slot)
		if !isStackTarget(tc) {
			return true
		}
		si := s.stackTgtIdx(tc, e.tver[slot])
		if si < 0 || si >= int(s.sp)-1 {
			return true
		}
		if !DebugCopyAbIgnoreCantCopy && s.stack[si].ab < abCascade && k.t.Cards[s.stack[si].card].Abilities[s.stack[si].ab].CantCopy {
			return true // gorge SpellCopyAllowed: the target stays legal, nothing is copied
		}
		if int(s.sp) >= MaxStack {
			s.capFull(StallStack)
			return true
		}
		ce := s.stack[si]
		s.stackSid(&ce)
		ce.ctrl = e.ctrl
		ce.flags &= sfKicked                 // a copy is not cast: no cast provenance (CR 707.10)
		ce.prov, ce.spc, ce.etbSrc = 0, 0, 0 // a copy is not the remembered card: no rider count, no rider source
		if ce.xi != 0 {
			from := s.g.xtp[ce.xi-1]
			ce.xi = 0
			xt := s.xtMut(&ce)
			if xt == nil {
				return true
			}
			*xt = from
		}
		top := s.sp - 1
		s.stack[top+1] = s.stack[top]
		s.stack[top] = ce
		s.sp++
		if in.C&1 != 0 && !DebugCopyAbNoRetarget && s.copyDeclSlot(k, &ce) >= 0 && s.copyElect(k, uint8(c)) {
			return false
		}
	}
	return true
}
