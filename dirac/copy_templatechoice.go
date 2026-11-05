package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters choice of the template an as-enters copy is made of (CR
// 614.12, 707.9). A permanent spell entering as a copy of a permanent whose
// own enters replacement opens with a choice (Dominion Saboteur copying
// Nyota Uhura, Communications: "choose a number") is asked that choice by
// gorge with the spell still on the stack, right after the copy election
// (rules/cast_etbchoice.go faceETBChoice, fromTemplate); the kernel poses
// the same second pre-entry ask. The clone answer stays in s.g.pre.val, the
// template's answer is held in s.g.pre.val2 until becomeCopyEnter hands it
// to the copied card's enters program (the answered-on-the-stack path of
// runReplProgAs).
//
// Not modelled: a template whose choice is a shock land's pay-life election
// (that one is still queued once the copy is on the battlefield).

// cloneTemplateAsk poses the template's pre-entry choice after the clone
// election in s.g.pre.val, and reports whether it did (the answer then waits
// for it).
//
//go:noinline
func (s *State) cloneTemplateAsk(k *prep) bool {
	p := &s.g.pre
	t := Obj(p.val)
	if k.gen == nil || t == NoObj || s.objs[t].zone != kop.ZoneBattlefield {
		return false
	}
	kind := s.enterChoiceKind(k, s.cardAt(s.objs[t].slot))
	switch kind {
	case akColor, akType, akNumber, akName, akNameAny, akNameLook:
	default:
		return false
	}
	p.ask2, p.kind, p.state = kind, kind, 1
	return true
}

// preAskCard is the card whose enters program the pending pre-entry ask is
// about: the spell's, or the copy template's for the second ask.
func (s *State) preAskCard() CardID {
	if p := &s.g.pre; p.ask2 != 0 {
		return s.cardAt(s.objs[Obj(p.val)].slot)
	}
	return s.objs[s.g.pre.obj].card
}
