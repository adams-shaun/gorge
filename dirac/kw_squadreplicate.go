package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Squad (CR 702.157a) and Replicate (CR 702.56a): "as an additional cost to
// cast this spell, you may pay [cost] any number of times", the multikicker
// shape with a possible non-mana pick part (Exterminate!'s tapXType,
// Ruthless Radrat's ExileFromGrave, Thrill-Kill Disciple's Discard,
// Reiterating Bolt's PayEnergy). The kernel reuses Multikicker's kick offer,
// count storage (cold.mkObj, and cold.mk only for a pure Multikicker) and
// count ask (ChMultikick):
//
//   - CardDef.Multikicker holds the payment's mana half, so the kicked
//     offer, buildCost, multikickMax and the offer gate price base plus one
//     payment exactly as for Multikicker;
//   - CardDef.Repeated holds the full payment, and CardDef.RepeatedKind
//     places the count ask where gorge asks it: Replicate before the
//     multikicker ask (cast_begin.go 1154), Squad after it (chorus_cost.go's
//     preCostAsks), Multikicker in the multikicker stage. A card prints at
//     most one of the three.
//   - the non-mana part is paid once per answered payment: the pick cost's
//     Cost.N is multiplied by the count (the sum of the same part paid N
//     times, which is gorge's composed Cost), and the count bound checks
//     that many picks are still payable.
//
// The count's provenance is the spell object's, so a replicate SpellCast
// trigger's Count$ReplicatePaid reads cold.mkObj while the spell is on the
// stack. A Squad self-ETB captured the count at enqueue (pendingTrig.defSeat,
// triggers_gen.go) and reads it there, so a Squad permanent needs no
// per-slot cold.mk; a token copy that was never cast reads 0
// (exprs.go timesKicked).

// DebugSquadCountOffByOne is the mutation test's switch: a squad cast stores
// one payment fewer than it pays (kw_squadreplicate.go's answer stage).
var DebugSquadCountOffByOne bool

// repeatAnswerStage reports whether the build's current stage is a
// repeated-payment count ask (the three stages share one answer handler).
func (s *State) repeatAnswerStage() bool {
	st := s.bld().stage
	return st == bsReplicate || st == bsMulti || st == bsSquad
}

// hasKickPickCost reports whether a card's kicked cast owes a non-mana pick
// part (a plain Kicker or a Squad/Replicate repeated payment).
func hasKickPickCost(c *kop.CardDef) bool {
	return c.KickerCost.Flags != 0 || c.Repeated.Flags != 0
}

// repeatCountOptions poses the repeated-payment count ask at the build's
// current stage: one stage for all three keywords, placed where gorge asks
// each (kw_multikicker.go's multikickOptions).
func (s *State) repeatCountOptions(k *prep, d *Decision) bool {
	return s.multikickOptions(k, d, s.bld().stage)
}

// repeatPart is a Squad/Replicate card's full per-payment cost, or nil.
func repeatPart(cd *kop.CardDef) *kop.Cost {
	if cd.RepeatedKind == 0 || cd.RepeatedKind == kop.RepeatMultikicker || cd.Repeated.Flags == 0 {
		return nil // pure mana (Multikicker) has no pick part
	}
	return &cd.Repeated
}

// repeatAskStage is the build stage a repeated cost's count is asked at.
func repeatAskStage(kind uint8) uint8 {
	switch kind {
	case kop.RepeatReplicate:
		return bsReplicate
	case kop.RepeatSquad:
		return bsSquad
	default:
		return bsMulti
	}
}

// repeatScale is c paid n times: the counted part's Cost.N is multiplied.
// ok is false when the product does not fit the table's byte, a count the
// bound then refuses rather than underpaying.
func repeatScale(c kop.Cost, n int) (kop.Cost, bool) {
	if n < 1 {
		n = 1
	}
	if c.N != 0 {
		v := int(c.N) * n
		if v > 255 {
			return kop.Cost{}, false
		}
		c.N = uint8(v)
	}
	return c, true
}

// repeatPickCost is the pick cost the build is paying: the card's repeated
// non-mana part scaled to the answered count, kept in the build scratch so
// buildPickCost can return a pointer to it. nil for no Squad/Replicate pick
// part or a declined (count 0) payment.
func (s *State) repeatPickCost(cd *kop.CardDef, n int) *kop.Cost {
	if repeatPart(cd) == nil || n <= 0 {
		return nil
	}
	c, ok := repeatScale(cd.Repeated, n)
	if !ok {
		return nil
	}
	s.bld().repeatCost = c
	return &s.bld().repeatCost
}
