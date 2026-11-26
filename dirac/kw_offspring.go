package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Offspring (CR 702.175, Bloomburrow), copied from gorge (rules
// kw_offspring.go, legal_walk_hand.go's "(offspring)" offer, cast_begin.go
// castModeOffspring; cards kw_offspring.go's expansion):
//
//   - the offspring cost is an optional ADDITIONAL cost offered as its own
//     cast option beside the plain cast (gorge Mode "offspring", the
//     Buyback shape: base + offspring, never an ask). The kernel's kicked
//     cast is that option: CardDef.Offspring cards carry the offspring
//     cost in CardDef.Kicker (no KwKicker), and the paid flag is sfKicked
//     on the entry (gorge's FlagOffspringPaid / Object.OffspringPaid), so
//     stackEntry and State grow by nothing. kgen refuses a card with both.
//   - as the spell resolves into a permanent the kicked mark moves to the
//     permanent (mechKicked, turn_gen.go), cleared when it leaves.
//   - the expansion's enter trigger is unconditional and mints
//     Count$OffspringPaid copies (lowered as Count$Valid Card.Self+kicked):
//     one when the permanent's cast paid the cost, none for a plain cast,
//     a source that left, or the token copy itself (never cast).
//   - the copy is a 1/1 (SetPower$/SetToughness$ 1): the copy op's
//     exception override, a layer-1 copiable value (copy_exceptions.go).
//
// A layer-6 StatStackKw Offspring grant (Zinnia, Valley's Voice's "Creature
// spells you cast have offspring {2}", the StackKwOffspring lowering) reaches
// the same reads: gorge prices the granted cost through the derived keyword
// (offspring.go offspringRawParam, the printed K:Offspring line first, then
// the grants) at the offer and the cast, and synthesizes the printed
// expansion's own enter trigger for a creature the grant reaches that does
// not print the keyword (trigger_granted.go checkGrantedOffspringTriggers).
// The kernel reads the grant at the same points (kickGrant) and queues the
// carrier's GrantBody latent trigger when the kicked permanent enters
// (queueOffspringGrant; the trigger itself is the printed expansion's
// lowering, held AfLatent on the carrier). A card that also prints a
// different kicked form (Kicker, Multikicker, Buyback, Entwine) beside the
// grant poses gorge's second cast mode the kernel's single kicked offer
// cannot name: it stalls unmodelled (StallStackKwGrant), as webSlingGrant's
// printed-and-granted card does.

// offspringOffer: card c has Offspring and its "(offspring)" cast is
// offered beside the plain one (the kicked cast's offer, legal.go).
func offspringOffer(k *prep, c CardID) bool {
	return k.t.Cards[c].Offspring || k.t.Cards[c].Buyback || k.t.Cards[c].Entwine
}

// kickConflict is the printed kicked form a card must not carry beside a
// reaching Offspring grant: each is gorge's own cast mode beside the
// "(offspring)" one, which the kernel's single kicked action cannot name
// (kw_offspring.go's file comment; an OptionalCost part rides the same
// kicked action, kgen's "optional cost with another optional cost" refusal).
func kickConflict(cd *kop.CardDef) bool {
	return cd.HasMultikicker || cd.Keywords&kop.KwKicker != 0 || cd.Buyback || cd.Entwine ||
		cd.OptCost.Flags != 0 || cd.OptBehold || cd.OptEvidence != 0 || cd.OptWaterbend != 0
}

// kickGrant is the additional cost a live StatStackKw Offspring grant charges
// the kicked cast of card c (object o) by caster p: the first reaching
// grant's GrantCost (gorge offspringRawParam takes the FIRST derived
// Offspring line, a printed K:Offspring before the grants, the same first
// read casualtyN makes). fromHand answers a hand-cast. ok is false with no
// grant reaching; a printed offspring keeps its own cost (gorge's first line)
// and a printed kicked form beside the grant stalls unmodelled. Behind
// genPrep.anyStackKw.
func (s *State) kickGrant(k *prep, p uint8, c CardID, o Obj, fromHand bool) (cost kop.ManaCost, ok bool) {
	if k.gen == nil || !k.gen.anyStackKw {
		return cost, false
	}
	cd := &k.t.Cards[c]
	if cd.Offspring {
		return cost, false // the printed line is the derived list's first: gorge prices it
	}
	n, _, src := s.stackKwScan(k, kop.StackKwOffspring, c, o, p, fromHand)
	if n == 0 {
		return cost, false
	}
	if kickConflict(cd) {
		s.capFull(StallStackKwGrant) // gorge offers the printed mode beside the granted one
		return cost, false
	}
	return src.a.Static.GrantCost, true
}

// offspringCaster is the caster a granted-offspring read of object o uses: a
// hand card's owner (the hand walk offers its own cards), a spell on the
// stack its entry's controller with the entry's cast origin (kw_rebound.go
// reads the same entry). Outside hand and stack the owner answers.
func (s *State) offspringCaster(o Obj) (p uint8, fromHand bool) {
	fromHand = s.objs[o].zone == kop.ZoneHand
	p = s.objs[o].owner()
	if e := s.spellEntryOf(o); e != nil && e.kind == seSpell {
		p, fromHand = e.ctrl, kop.Zone(e.prov&7) == kop.ZoneHand
	}
	return p, fromHand
}

// kickGrantEntry is kickGrant for the spell entry e being built or resolved:
// the cost stage's addKickerPayments, the target affordability walk and the
// view's cast pricing all read the entry.
func (s *State) kickGrantEntry(k *prep, e *stackEntry) (cost kop.ManaCost, ok bool) {
	return s.kickGrant(k, e.ctrl, e.card, e.obj, kop.Zone(e.prov&7) == kop.ZoneHand)
}

// kickPayment is the additional cost a kicked cast of card c (object o) by p
// announces: the first reaching Offspring grant's cost, else the printed
// kicked payment (kickerPayment's read, the offer's pricing). Offer-time
// callers pass fromHand.
func (s *State) kickPayment(k *prep, p uint8, c CardID, o Obj, fromHand bool) kop.ManaCost {
	cd := &k.t.Cards[c]
	if k.gen != nil && k.gen.anyStackKw {
		// the grant read first: a printed kicked form beside a reaching
		// grant stalls inside kickGrant, then the printed cost answers
		if mc, ok := s.kickGrant(k, p, c, o, fromHand); ok {
			return mc
		}
	}
	return kickerPayment(cd)
}

// offGrantOffer reports whether a live StatStackKw Offspring grant reaches
// p's hand card o of card c: the offer gate beside offspringOffer's printed
// read (legal.go).
func (s *State) offGrantOffer(k *prep, p uint8, c CardID, o Obj) bool {
	if k.gen == nil || !k.gen.anyStackKw {
		return false
	}
	n, _ := s.stackKwN(k, kop.StackKwOffspring, c, o, p, true)
	return n > 0
}

// queueOffspringGrant queues the carrier's GrantBody latent trigger for the
// permanent o (slot sl) the kicked spell just became, when a live
// StatStackKw Offspring grant reaches its card and the card does not print
// Offspring: gorge's checkGrantedOffspringTriggers synthesizes the printed
// expansion's own trigger there, the payment's provenance its intervening if
// (CR 702.175a, 603.4) -- a plain cast triggers nothing, so the kicked mark
// the enterGen just copied stands in for the CheckSVar. The trigger's Source
// is the permanent (gorge's queue carries the entering object), its card the
// carrier and ab the latent's index, so the resolution runs the carrier's
// body over the entering object. Behind prep.anyOffGrant.
func (s *State) queueOffspringGrant(k *prep, sl Slot, o Obj, ctrl uint8) {
	c := s.cardAt(sl)
	if k.t.Cards[c].Offspring {
		return // the printed face owns the line: gorge skips it too
	}
	if !s.mSl(k, mechKicked, sl) {
		return // a plain cast or a copy: the intervening if fails (CR 603.4)
	}
	p, _ := s.offspringCaster(o)
	if n, _ := s.stackKwN(k, kop.StackKwOffspring, c, o, p, true); n == 0 {
		return
	}
	carrier, ab, ok := s.offGrantCarrier(k, c, o, p)
	if !ok {
		return
	}
	s.enqueue(o, s.objs[o].ver, carrier, ab, ctrl, o, ctrl, 0)
}

// offGrantCarrier names the first reaching Offspring grant's carrier and its
// GrantBody latent's ability index (Rebound's GrantBody read, reboundPlan):
// the carrier card the latent trigger's entry resolves its program from.
func (s *State) offGrantCarrier(k *prep, c CardID, o Obj, p uint8) (card CardID, ab uint8, ok bool) {
	_, _, src := s.stackKwScan(k, kop.StackKwOffspring, c, o, p, true)
	if src.sl == NoSlot || src.emb != 0 || src.gr {
		return 0, 0, false // an EffectZone$ Stack self grant or an emblem: no carrier card holds the body
	}
	card = s.cardAt(src.sl)
	if gb := src.a.Static.GrantBody; gb != 0 && int(gb-1) < len(k.t.Cards[card].Abilities) {
		return card, gb - 1, true
	}
	return 0, 0, false
}

// FeatOffspringCast reports that object o's kicked cast (AkCast mode bit 0)
// is gorge's "(offspring)" cast: its card prints Offspring, or a live
// StatStackKw Offspring grant reaches the cast (Zinnia; gorge cast_begin
// castModeOffspring charges the derived read). The caster is a hand card's
// owner, a stack spell its entry's controller (offspringCaster).
func (s *State) FeatOffspringCast(o Obj) bool {
	k := s.k()
	c := s.objs[o].card
	if k.t.Cards[c].Offspring {
		return true
	}
	if k.gen == nil || !k.gen.anyStackKw {
		return false
	}
	p, fromHand := s.offspringCaster(o)
	n, _ := s.stackKwN(k, kop.StackKwOffspring, c, o, p, fromHand)
	return n > 0
}

// FeatBuybackCast reports that object o's kicked cast is gorge's
// "(buyback)" cast: its card has Buyback (kw_buyback.go).
func (s *State) FeatBuybackCast(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Buyback
}
