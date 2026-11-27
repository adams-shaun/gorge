package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the per-permanent attacked / dealt-damage / activated / face-up marks
// (kop.PredPermMark), slot masks on the mechanic-bit arena (0 B).
//   - mechAttacked: set for each declared attacker (finishAttacks; gorge
//     stamps the attack-declaration event's IDs, so a creature put onto the
//     battlefield attacking is not stamped), cleared as the turn begins.
//   - mechDealtTurn: set when the permanent deals damage (after prevention,
//     to anything: gorge's DamageProvenance fold, amount > 0), cleared as the
//     turn begins.
//   - mechDealtAny / mechDealtCombat: the same event, game-long: never reset
//     by the turn; a new permanent takes the slot clear (arenaClrSlot, CR 400.7).
//   - mechActTurn: set when the permanent activates a non-mana ability
//     (cast.go's activation tally, gorge's Object.ActivatedThisTurn), cleared
//     as the turn begins.
//   - mechFaceUpTurn: set when the permanent is turned face up (turnUpFlip,
//     gorge's TurnFaceUp fold and Object.TurnedFaceUpThisTurn), cleared as
//     the turn begins.
// Every write sits behind genPrep.anyPermMark and touches the slot, so the
// layer caches that read the marks (a hexproof grant on !dealtDamagetoAny)
// see the change.

// DebugPermMarkNoReset keeps mechAttacked and mechDealtTurn across turns
// (mutation testing only: the lockstep must catch it).
var DebugPermMarkNoReset bool

// permMarkMech maps kop.PermMark* to the arena mechanic.
var permMarkMech = [6]mech{mechAttacked, mechDealtTurn, mechDealtAny, mechDealtCombat,
	mechActTurn, mechFaceUpTurn}

// prepPermMark sets anyPermMark: the table gives some mark an arena mask.
func prepPermMark(g *genPrep) {
	g.anyPermMark = g.ix[mechAttacked] != arenaNone || g.ix[mechDealtTurn] != arenaNone ||
		g.ix[mechDealtAny] != arenaNone || g.ix[mechDealtCombat] != arenaNone ||
		g.ix[mechActTurn] != arenaNone || g.ix[mechFaceUpTurn] != arenaNone
	g.anyTappedTurn = g.ix[mechTappedTurn] != arenaNone
	g.anyBtTurn = g.ix[mechBtTurn] != arenaNone
}

// permMarkUses reports which marks some table filter reads (arenaUses).
func permMarkUses(t *kop.Table, u *[numMech]bool) {
	for i := range t.Filters {
		if f := &t.Filters[i]; f.Pred == kop.PredPermMark {
			u[permMarkMech[f.PredN&7]] = true
		}
	}
}

// permMarkRead is PredPermMark on a battlefield permanent.
func (s *State) permMarkRead(k *prep, fl *kop.Filter, sl Slot) bool {
	return s.mSl(k, permMarkMech[fl.PredN&7], sl) != (fl.PredN&kop.PermMarkNot != 0)
}

// markAttackers stamps every declared attacker (finishAttacks, after the
// declaration; a permanent entering attacking later is not declared).
//
//go:noinline
func (s *State) markAttackers(k *prep) {
	if k.gen.ix[mechAttacked] == arenaNone {
		return
	}
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfAttacking != 0 {
			s.mSlSet(k, mechAttacked, sl)
			s.touchSl(sl)
		}
	}
}

// markDealt records that the permanent at src dealt damage (amount > 0,
// after prevention). NoSlot (a spell, or an ability whose source left) is
// not marked: only a battlefield permanent is filtered.
//
//go:noinline
func (s *State) markDealt(k *prep, src Slot, combat bool) {
	if src == NoSlot || !s.occ.has(src) {
		return
	}
	s.mSlSet(k, mechDealtTurn, src)
	s.mSlSet(k, mechDealtAny, src)
	if combat {
		s.mSlSet(k, mechDealtCombat, src)
	}
	s.touchSl(src)
}

// markDealtRec marks every source that dealt combat damage in r.
//
//go:noinline
func (s *State) markDealtRec(k *prep, r *dmgRec) {
	for m := s.occ; m.any(); m = m.rest() {
		if sl := m.first(); r.dealt[sl] > 0 {
			s.markDealt(k, sl, true)
		}
	}
}

// markActivated records that the permanent at sl activated a non-mana
// ability this turn (the same ledger the object's activation tally keeps,
// gorge's Object.ActivatedThisTurn; Cut Short's Planeswalker.activated). The
// write sits beside the tally's own increment, so the filter and the network
// feature never disagree.
//
//go:noinline
func (s *State) markActivated(k *prep, sl Slot) {
	if sl == NoSlot || !s.occ.has(sl) {
		return
	}
	s.mSlSet(k, mechActTurn, sl)
	s.touchSl(sl)
}

// markFaceUp records that the face-down permanent at sl was turned face up
// this turn (gorge's TurnFaceUp fold sets Object.TurnedFaceUpThisTurn on the
// same event; Kaust, Eyes of the Glade). Call it only on a real flip.
//
//go:noinline
func (s *State) markFaceUp(k *prep, sl Slot) {
	if sl == NoSlot || !s.occ.has(sl) {
		return
	}
	s.mSlSet(k, mechFaceUpTurn, sl)
	s.touchSl(sl)
}

// permMarkNewTurn ends the per-turn marks as the turn begins.
func (s *State) permMarkNewTurn(k *prep) {
	if DebugPermMarkNoReset {
		return
	}
	s.mSlZero(k, mechAttacked)
	s.mSlZero(k, mechDealtTurn)
	s.mSlZero(k, mechTappedTurn)
	s.mSlZero(k, mechBtTurn)
	s.mSlZero(k, mechActTurn)
	s.mSlZero(k, mechFaceUpTurn)
}

// markTapped records that the permanent at sl has become tapped this turn
// (gorge Facts.TappedTurn; an entry-tapped state is not a tap, CR 603.2e, and
// its callers do not mark; a tap of an already-tapped permanent is a no-op
// and does not mark either, since gorge emits no Tap event for it). It is
// written after the tap's Taps triggers are queued, so a FirstTime$ watcher
// reads whether an EARLIER tap happened this turn. Every real tap marks, even
// with no watcher on the battlefield: the watcher may enter later and still
// needs to know (Captain America, Living Legend). A table without a Taps
// FirstTime$ line gives the mechanic no mask, so the write is one failed
// bounds test.
//
//go:noinline
func (s *State) markTapped(k *prep, sl Slot) {
	s.mSlSet(k, mechTappedTurn, sl)
}
