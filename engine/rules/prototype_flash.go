package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// protoTimingReader is the part of the engine prototypeTimingOK reads.
type protoTimingReader interface {
	spellTimingPre(p state.PlayerID, id state.ObjID, f *cards.Face, sorcery bool) (ok, needFlash bool)
	costPotentialTargets(p state.PlayerID, id state.ObjID, scope costScope) []state.Target
	castWithFlashTargets(p state.PlayerID, id state.ObjID, targets []state.Target, teamworkOffer, asPrototype bool) bool
}

// protoFlashHint is the walk's cheap "some flash grant could exist" test.
type protoFlashHint interface {
	flashBoard() bool
	faceFlash(f *cards.Face) bool
}

// prototypeTimingOK is the walk's spellTimingOK for the prototyped cast route:
// the shared pre-gate, then the flash grant read as the prototyped spell. The
// spell is the K:Prototype inset's colour and mana value while it is cast (CR
// 718.3a), so a grant keyed to them (Yeva, Nature's Herald's "green creature
// spells") reaches a colourless card cast prototyped. hint is the walk's
// cheap "some grant could exist" test (flashBoard or the face's own static);
// mayPlay is the may-play flash permission, read only when the grant fails.
func prototypeTimingOK(r protoTimingReader, p state.PlayerID, id state.ObjID, f *cards.Face, sorcery bool, hint protoFlashHint, mayPlay func() bool) bool {
	ok, needFlash := r.spellTimingPre(p, id, f, sorcery)
	if !ok {
		return false
	}
	if !needFlash {
		return true
	}
	if (hint.flashBoard() || hint.faceFlash(f)) && r.castWithFlashTargets(p, id, r.costPotentialTargets(p, id, spellScope("")), false, true) {
		return true
	}
	return mayPlay()
}
