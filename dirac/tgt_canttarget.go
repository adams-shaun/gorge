package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// cantTargetGen reports whether a printed S:Mode$ CantTarget
// (kop.StatCantTarget; CR 115.4; gorge cantTargetBlocks,
// engine/rules/canttarget_static.go) withholds the permanent at sl from the
// spell or ability player p is targeting with: the one being built or
// resolving (the target offer and the CR 608.2b recheck), else the one a
// priority-time feasibility probe is for. A spell is judged by its card, an
// ability by the permanent that has it (its card where it lies once it left
// the battlefield), both read with the static's controller as "you", as
// gorge matches ValidSource$ against protectionSource. The kind is the
// entry's: a spell, an activated ability or a triggered one (gorge
// cantTargetValidSA). With no spell or ability in view the static has
// nothing to judge, as targetableGen's other restrictions. eachStatic skips
// a source that lost its abilities, as gorge's activeStatics does.
//
//go:noinline
func (s *State) cantTargetGen(k *prep, sl Slot, p uint8) bool {
	var kind int16
	var card CardID
	srcSl, known := NoSlot, false
	if e := s.curEntry(); e != nil {
		switch e.kind {
		case seSpell:
			kind = kop.CantTgtSpell
		case seAbility:
			kind = kop.CantTgtActivated
		case seTrigger:
			kind = kop.CantTgtTriggered
		default:
			return false
		}
		card, known = e.card, true
		if e.kind != seSpell && e.obj != NoObj && s.objs[e.obj].zone == kop.ZoneBattlefield {
			srcSl = s.objs[e.obj].slot
		}
	} else {
		switch s.g.probe.kind {
		case probeSpell:
			kind, card, known = kop.CantTgtSpell, s.g.probe.card, true
		case probeAbility:
			kind = kop.CantTgtActivated
			if s.g.probe.src != NoSlot {
				srcSl, card, known = s.g.probe.src, s.cardAt(s.g.probe.src), true
			}
		default:
			return false
		}
	}
	hit := false
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantTarget {
			return true
		}
		st := &a.Static
		if st.A != 0 && st.A&kind == 0 {
			return true
		}
		if st.StFlags&kop.StRaiseYou != 0 && stc.ctrl != p || st.StFlags&kop.StRaiseOpp != 0 && stc.ctrl == p {
			return true
		}
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, s.ctxStatic(stc)) {
			return true
		}
		if st.Filter2 != 0 {
			if !known {
				return true
			}
			if srcSl != NoSlot && s.occ.has(srcSl) {
				if !s.matchPermGen(k, st.Filter2, srcSl, stc.ctrl, stc.sl, layerFull) {
					return true
				}
			} else if !s.matchCardGen(k, st.Filter2, card, p, stc.ctrl, NoObj) {
				return true
			}
		}
		if s.matchPermGen(k, st.Filter, sl, stc.ctrl, stc.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}
