package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The countered-spell mana-value look-back (CR 608.2h, 202.3e). gorge
// snapshots the mana value of every spell on the stack among a resolution's
// targets and trigger object as the resolution starts
// (captureStackTargetSnapshots, read by count_ref_property.go's
// SpellManaValue arm): a Counter later in the chain moves the spell off the
// stack, where CR 400.7 zeroes its announced X, so the read must be the value
// it had there (Sound the Trumpets' recruit gate, Illumination's life gain).

// snapTgtMV records, as the resolution starts, the mana value of every object
// target that is a spell on the stack, and of the resolving trigger's object
// (gorge's TriggerCard capture). The Counter that follows may remove the
// target's stack entry before a later op reads the target's mana value.
func (s *State) snapTgtMV(k *prep, e *stackEntry) {
	for i := range s.g.tgtMVCd {
		s.g.tgtMVCd[i], s.g.tgtMV[i] = 0, 0
	}
	s.g.trigMVCd, s.g.trigMV = 0, 0
	if e == nil {
		return
	}
	for i := 0; i < kop.MaxTargets; i++ {
		t := e.tgt[i]
		if !isObjTarget(t) || s.objs[Obj(t)].zone != kop.ZoneStack {
			continue
		}
		s.g.tgtMVCd[i], s.g.tgtMV[i] = s.stackSpellMVLKI(k, Obj(t))
	}
	if e.trigObj != NoObj && isObjTarget(TCode(e.trigObj)) && s.objs[e.trigObj].zone == kop.ZoneStack {
		s.g.trigMVCd, s.g.trigMV = s.stackSpellMVLKI(k, e.trigObj)
	} else if e.trigObj != NoObj && e.trigMV != 0 && s.isSpellCastTrigger(k, e) {
		// The spell left the stack (a counterspell) before its SpellCast
		// trigger resolved: the queue-time snapshot keeps the X it was cast
		// with (CR 608.2h, 202.3e).
		s.g.trigMVCd, s.g.trigMV = TCode(e.trigObj)+1, uint8(min(int(e.trigMV), 254))+1
	}
}

// isSpellCastTrigger reports whether e is a triggered ability whose event is
// a spell being cast, the only entries whose trigMV is a spell's mana value
// (value_spellmv.go; the other trigMV carriers are delayed programs).
func (s *State) isSpellCastTrigger(k *prep, e *stackEntry) bool {
	if e.kind != seTrigger || int(e.card) >= len(k.t.Cards) {
		return false
	}
	cd := &k.t.Cards[e.card]
	return int(e.ab) < len(cd.Abilities) && cd.Abilities[e.ab].Trig.Event == kop.TrigSpellCast
}

// stackSpellMVLKI is object o's mana value as a spell on the stack (CR
// 202.3e: the printed face and its announced X), the snapshot's cd + 1 and mv
// + 1 pair, the mv saturating at 254. A face-down spell's mana value is 0 (CR
// 708.2; gorge's state.ManaValue FaceDownHidden case).
func (s *State) stackSpellMVLKI(k *prep, o Obj) (TCode, uint8) {
	mv := 0
	if !s.faceDownAt(k, o) {
		mv = s.objMV(k, s.objs[o].card, o) // static_cda.go
		if mv > 254 {
			mv = 254 // saturate (tgtSpent's rule)
		}
	}
	return TCode(o) + 1, uint8(mv) + 1
}

// clearTgtMV drops the resolution's snapshots (finishResolve): a resolution
// that did not start with snapTgtMV must not read the last one's.
func (s *State) clearTgtMV() {
	for i := range s.g.tgtMVCd {
		s.g.tgtMVCd[i], s.g.tgtMV[i] = 0, 0
	}
	s.g.trigMVCd, s.g.trigMV = 0, 0
}

// tgtMVLKIMV is the CR 608.2h look-back of a Targeted$CardManaCost read: the
// mana value the named target had as a spell on the stack, as the resolution
// started (snapTgtMV), once the chain has moved it off the stack. ok is false
// while the object is still on the stack (the live read counts X), when the
// ref names another object (a rewritten target; a code is only ever handed out
// again to a ceased token or copy, zone_objreuse.go, never to a stack spell),
// or with no snapshot.
func (s *State) tgtMVLKIMV(r kop.Ref, o Obj) (int, bool) {
	sl, ok := tgtSlot(r)
	if !ok || o == NoObj || s.objs[o].zone == kop.ZoneStack {
		return 0, false
	}
	if s.g.tgtMVCd[sl] == 0 || int(s.g.tgtMVCd[sl])-1 != int(o) || s.g.tgtMV[sl] == 0 {
		return 0, false
	}
	return int(s.g.tgtMV[sl]) - 1, true
}

// trigMVLKIMV is tgtMVLKIMV's read for the resolving trigger's object
// (TriggeredCard$CardManaCost; gorge's TriggerCard capture), once the chain
// has moved it off the stack.
func (s *State) trigMVLKIMV(r kop.Ref, o Obj) (int, bool) {
	if r != kop.RefTriggerObj || o == NoObj || s.objs[o].zone == kop.ZoneStack {
		return 0, false
	}
	if s.g.trigMVCd == 0 || int(s.g.trigMVCd)-1 != int(o) || s.g.trigMV == 0 {
		return 0, false
	}
	return int(s.g.trigMV) - 1, true
}
