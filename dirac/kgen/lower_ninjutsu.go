package kgen

import "strings"

// Ninjutsu (CR 702.49). gorge expands K:Ninjutsu:<cost>
// (engine/cards/kw_ninjutsu.go kwNinjutsu) into one hand ability:
//
//	AB$ ChangeZone | Defined$ Self | Origin$ Hand | Destination$ Battlefield
//	| Tapped$ True | Attacking$ True
//	| Cost$ <cost> Return<1/Creature.YouCtrl+attacking+unblocked>
//	| ActivationZone$ Hand
//	| ActivationPhases$ Declare Blockers,Combat Damage,EndCombat
//	| Keyword$ Ninjutsu | SpellDescription$ Ninjutsu <cost>
//
// That exact shape lowers to an activated hand ability flagged Ninjutsu
// (kop.AfNinjutsu): the kernel offers it only in the declare-blockers,
// combat-damage and end-of-combat steps (gorge activationPhasesOK; its one
// "Combat Damage" step is the kernel's two damage steps), captures the
// returned attacker's defender as the Return pick is made (gorge
// cast_payment.go castAnswerReturncost), and resolves OpNinjutsu: the card,
// if still in its owner's hand, enters tapped and attacking that defender
// (gorge zone_change.go: MoveZone, the "entered tapped" Tap, TokenAttacks;
// dirac/kw_ninjutsu.go). Anything else refuses.

// ninjutsuPhases is the expansion's ActivationPhases$ value.
const ninjutsuPhases = "Declare Blockers,Combat Damage,EndCombat"

// ninjutsuAbility checks an ActivationZone$ Hand ability stamped Keyword$
// Ninjutsu against the expansion's exact shape and marks it.
func (lc *lowerCtx) ninjutsuAbility(ab *lAbility, ps *params, cost lCost) error {
	if ab.Kind != akActivated || ab.Zone != zHand {
		return refuse("ninjutsu ability outside the hand")
	}
	if !lc.hasKeywordLine(ps.str("KeywordLine")) || !strings.HasPrefix(ps.str("KeywordLine"), "Ninjutsu:") {
		return refuse("ninjutsu ability without its K:Ninjutsu line")
	}
	if ps.str("ActivationPhases") != ninjutsuPhases {
		return refuse("ninjutsu ActivationPhases$ %q", ps.str("ActivationPhases"))
	}
	if !ps.is("Defined", "Self") || !ps.is("Origin", "Hand") || !ps.is("Destination", "Battlefield") ||
		!ps.is("Tapped", "True") || !ps.is("Attacking", "True") {
		return refuse("ninjutsu body other than the K:Ninjutsu expansion")
	}
	r := cost.Return
	if r == nil || cost.ReturnN != 1 || len(r.Clauses) != 1 || r.Players != 0 {
		return refuse("ninjutsu without its Return<1/...> cost")
	}
	if c := r.Clauses[0]; c != (lClause{Types: tCreature, Ctrl: whoYou, Flags: fAttacking, Unblocked: true}) {
		return refuse("ninjutsu return spec %s", c.String())
	}
	if !cost.Mana.None && (cost.Mana.X || cost.Mana.XX) {
		return refuse("ninjutsu with an {X} cost")
	}
	m := cost
	m.Return, m.ReturnN = nil, 0
	if !m.pureMana() {
		return refuse("ninjutsu cost beside another non-mana part")
	}
	ab.Ninjutsu = true
	return nil
}

// ninjutsuBody emits the ability's one op (the ChangeZone's every
// parameter was read by ninjutsuAbility or putSelfAbility). plain is the
// put-self-from-hand shape: the same op with C 1, no tap and no attack.
func (lc *lowerCtx) ninjutsuBody(ps *params, plain bool) error {
	ps.consume("SpellDescription", "Keyword", "KeywordLine", "Cost", "ActivationZone")
	if left := ps.unread(); len(left) > 0 {
		return refuse("ninjutsu unread params %v", left)
	}
	in := lInstr{Op: "Ninjutsu"}
	if plain {
		in.C = 1
	}
	lc.emit(in)
	return nil
}

// putSelfAbility marks an activated hand ability that is exactly
//
//	AB$ ChangeZone | Cost$ <mana> | ActivationZone$ Hand | Origin$ Hand
//	| Defined$ Self | Destination$ Battlefield
//
// (Talon Gates of Madara's "{4}: Put this card from your hand onto the
// battlefield"): the cost leaves the card in the hand, the ability goes on
// the stack, and on resolution the card enters the battlefield if it is
// still in its owner's hand (gorge changeZoneMoveSkip's Origin$ guard and
// sourceReturnedSincePush; CR 400.7). It reports false for any other shape.
func (lc *lowerCtx) putSelfAbility(api string, ab *lAbility, ps *params, cost lCost) bool {
	if api != "ChangeZone" || ab.Kind != akActivated || ab.Zone != zHand || !cost.pureMana() || cost.Mana.None ||
		cost.Mana.X || cost.Mana.XX || ab.Sorcery || ab.Cycling {
		return false
	}
	if !ps.is("Defined", "Self") || !ps.is("Origin", "Hand") || !ps.is("Destination", "Battlefield") {
		return false
	}
	ab.PutSelf = true
	return true
}

// triggeredSourceValid lowers TriggeredSource$Valid <spec> (gorge
// evalRefProperty: 1 when the damage source, as it now is, matches the
// spec): Moon-Circuit Hacker's "discard a card unless it entered this
// turn", X = TriggeredSource$Valid Card.!ThisTurnEntered. Only a DamageDone
// trigger body names a TriggeredSource (the kernel's trigger object there is
// the damage source). The filter grammar has no negated ThisTurnEntered, so
// that exact spec is 1 minus the Card.ThisTurnEntered count.
func (lc *lowerCtx) triggeredSourceValid(rest string) (lValue, error) {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "DamageDone" || !lc.trigBody || lc.ab.Latent {
		return lValue{}, refuse("TriggeredSource$ outside a DamageDone trigger body")
	}
	spec, ok := strings.CutPrefix(rest, "Valid ")
	if !ok {
		return lValue{}, refuse("TriggeredSource$%s", rest)
	}
	ref := lRef{Kind: rTriggeredSource}
	if spec == "Card.!ThisTurnEntered" {
		f, err := lc.filter("Card.ThisTurnEntered")
		if err != nil {
			return lValue{}, err
		}
		// -(count - 1) = 1 - count (Add, then Neg)
		return lValue{Kind: vRefCount, Ref: ref, F: f, Add: -1, Neg: true}, nil
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vRefCount, Ref: ref, F: f}, nil
}
