package botpolicy

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// AttachRules selects the opt-in A5 attach rules. The zero value is off, so
// every existing Board and both adapters keep their current target and
// ability decisions.
type AttachRules struct {
	// AuraPolarity aims an Aura by its AttachAILogic hint: a boon (Pump,
	// Animate, Curiosity) goes on the seat's own board, a curse (Curse,
	// GainControl, ChangeType, KeepTapped, HighestEvaluation) on the
	// opponent's. An unknown hint ranks as before.
	AuraPolarity bool
	// Carrier ranks the seat's own creatures as carriers for Equipment and
	// boon Auras (a Defender is a last resort, untapped is only a tiebreak)
	// and declines an equip activation when no own creature could use it.
	Carrier bool
	// HoldOwnAura holds a boon Aura (a creature Aura whose hint is Pump,
	// Animate or Curiosity) in hand, and the mana toward it, while the seat
	// controls no creature that could use it.
	HoldOwnAura bool
}

// Enabled reports whether any A5 rule is armed.
func (r AttachRules) Enabled() bool { return r.AuraPolarity || r.Carrier || r.HoldOwnAura }

// attachPolarity reads an AttachAILogic value: +1 when the attachment is a
// boon for its carrier, -1 when it is a burden, 0 when the hint is absent or
// does not say (Reanimate, SpecificCard, anything unlisted).
func attachPolarity(logic string) int8 {
	switch logic {
	case "Pump", "Animate", "Curiosity":
		return 1
	case "Curse", "GainControl", "ChangeType", "KeepTapped", "HighestEvaluation":
		return -1
	}
	return 0
}

const (
	// carrierUntapped is what a carrier gains for being untapped. An
	// attachment outlives this turn, so tapped is only a tiebreak (threat()
	// prices untapped at threatUntapped, which is too much here).
	carrierUntapped int32 = 1
	// carrierDemote sinks a Defender below every other creature: larger than
	// any reachable threat(), so one sort reproduces "skip Defenders unless
	// nothing else remains".
	carrierDemote int32 = 1 << 20
)

// carrierValue is threat() re-priced for choosing what to attach to.
func (c Creature) carrierValue() int32 {
	v := c.threat()
	if !c.Tapped {
		v += carrierUntapped - threatUntapped
	}
	if c.hasKeyword("Defender") {
		v -= carrierDemote
	}
	return v
}

// attachRanking is the ranker and lead order for an API "Attach" ask. An
// equip ask is one whose source is a battlefield permanent; a boon Aura is
// one whose hint has positive polarity. The own-first lead (boon) and the
// ascending own fallback (curse) apply only under AuraPolarity.
func (b Board) attachRanking(d *decision.Decision) (rank func(decision.Option) targetRank, leadOwn, ownAsc bool) {
	me := d.Player
	var pol int8
	if b.Attach.AuraPolarity {
		pol = attachPolarity(d.TargetEffect.AttachLogic)
	}
	equip := b.Cards.Get(d.Source).OnBattlefield
	carrier := b.Attach.Carrier && (equip || attachPolarity(d.TargetEffect.AttachLogic) > 0)
	rank = func(o decision.Option) targetRank {
		r := b.rankOption(o, me)
		if carrier && r.ours {
			if c, ok := b.Creatures.Lookup(o.Obj); ok {
				r.score = c.carrierValue()
			}
		}
		return r
	}
	return rank, pol > 0, pol < 0
}

// ownCarrierExists reports whether the seat controls a creature other than
// exclude that is not a Defender: one that could attack with what it is
// given. exclude covers a Reconfigure permanent that is itself a creature.
func (b Board) ownCarrierExists(me state.PlayerID, exclude state.ObjID) bool {
	for id, c := range b.Creatures.All() {
		if id != exclude && c.Controller == me && !c.hasKeyword("Defender") {
			return true
		}
	}
	return false
}

// auraHeld reports whether the cast of card id is held because it is a boon
// Aura and the seat has no creature to put it on. The same answer gates
// chooseCast, castEntries and the tap gate (castHeld), so no mana floats
// toward a spell the policy will not cast. With HoldOwnAura off, or for any
// card without a boon creature-Aura hint, it is false.
func (b Board) auraHeld(d *decision.Decision, id state.ObjID) bool {
	if !b.Attach.HoldOwnAura {
		return false
	}
	return attachPolarity(b.Cards.Get(id).AuraLogic) > 0 && !b.ownCarrierExists(d.Player, 0)
}

// equipNoCarrier is true for a first-time attach activation (an unattached
// source) when no own creature could use it, so the ability is not worth its
// cost. A re-site of an attached source is equipNoOp's business.
func (b Board) equipNoCarrier(o decision.Option, me state.PlayerID) bool {
	return b.Attach.Carrier && o.Attach && b.Cards.Get(o.Obj).AttachedTo == 0 && !b.ownCarrierExists(me, o.Obj)
}
