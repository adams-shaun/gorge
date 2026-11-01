package kgen

import (
	"strings"
)

// the spell-side reads of "Collected" on a card whose
// OptionalCost part is CollectEvidence<N> (CR 701.59a; Analyze the Pollen,
// Extract a Confession, Behind the Mask). gorge reads the group through
// CostMovesInWindow over the spell's own push (effects/conditions.go
// collectedGroup, effects/count_ref.go): non-empty exactly when the optional
// part was paid, because CollectEvidence<N>, N >= 1, exiles at least one
// card. So the group's presence is the kicked bit (cKicked / vKicked, the
// kernel's sfKicked). Every other spelling refuses.

// faceEvidenceOpt reports whether the face prints an OptionalCost static
// whose cost is CollectEvidence<N>.
func (lc *lowerCtx) faceEvidenceOpt() bool {
	for _, s := range lc.face.Statics {
		if s.Mode == "OptionalCost" && strings.HasPrefix(strings.TrimSpace(s.Params["Cost"]), "CollectEvidence<") {
			return true
		}
	}
	return false
}

// collectedOK gates a Collected read: the spell's own chain of a card with an
// evidence OptionalCost part.
func (lc *lowerCtx) collectedOK(what string) error {
	if !lc.faceEvidenceOpt() {
		return refuse("%s without a CollectEvidence OptionalCost", what)
	}
	if lc.ab == nil || lc.ab.Kind != akSpell {
		return refuse("%s outside a spell ability", what)
	}
	return nil
}

// collectedCondition lowers ConditionDefined$ Collected | ConditionPresent$
// Card [| ConditionCompare$ GE1|EQ0]: paid, or (EQ0) not paid.
func (lc *lowerCtx) collectedCondition(ps *params, present string) (lCond, error) {
	if err := lc.collectedOK("ConditionDefined$ Collected"); err != nil {
		return lCond{}, err
	}
	if strings.TrimSpace(present) != "Card" {
		return lCond{}, refuse("ConditionDefined$ Collected with ConditionPresent$ %q", present)
	}
	c := lCond{Kind: cKicked}
	if cmp, ok := ps.get("ConditionCompare"); ok {
		switch strings.TrimSpace(cmp) {
		case "GE1":
		case "EQ0":
			c.Neg = true
		default:
			return lCond{}, refuse("ConditionDefined$ Collected with ConditionCompare$ %q", cmp)
		}
	}
	return c, nil
}

// collectedCompare gates Count$Compare <SVar = Collected$Amount> <op><n>: only
// GE1, the paid test.
func (lc *lowerCtx) collectedCompare(op string, th int) error {
	if err := lc.collectedOK("Collected$Amount"); err != nil {
		return err
	}
	if op != "GE" || th != 1 {
		return refuse("Collected$Amount compared with %s%d", op, th)
	}
	return nil
}

// "CastSA>Collected" on a permanent spell whose
// OptionalCost part is CollectEvidence<N> (Crimestopper Sprite, Vitu-Ghazi
// Inspector). gorge reads the evidence group off the permanent's own cast
// (effects/conditions.go castCollectedGroup -> rules/host_cast_ledger.go
// castWindowMoves): non-empty exactly when the optional part was paid, because
// CollectEvidence<N>, N >= 1, exiles at least one card, and empty once the
// permanent has left the battlefield (WasCast ends on any later zone change).
// The kernel carries that fact as the permanent's kicked provenance
// (mechKicked, F2Kicked), false for a gone source, so both spellings lower to
// Card.Self+kicked on the source. Only the card's own self-ETB trigger chain
// reads it; every other spelling refuses.

// selfETBTrigger reports whether the ability being lowered is this card's own
// "When CARDNAME enters" trigger (ValidCard$ Card.Self, Destination
// Battlefield, a printed trigger).
func (lc *lowerCtx) selfETBTrigger() bool {
	if lc.ab == nil || lc.ab.Kind != akTriggered || lc.ab.Trigger == nil || lc.ab.Latent {
		return false
	}
	tr := lc.ab.Trigger
	if tr.Event != "ETB" || tr.F == nil || tr.F.Players != 0 || len(tr.F.Clauses) != 1 {
		return false
	}
	return tr.F.Clauses[0] == lClause{Flags: fSelf}
}

// castCollectedPerm lowers a permanent's CastSA>Collected read to the paid
// test on the source: the condition Card.Self+kicked.
func (lc *lowerCtx) castCollectedPerm(what string) (lCond, error) {
	if !lc.faceEvidenceOpt() {
		return lCond{}, refuse("%s without a CollectEvidence OptionalCost", what)
	}
	if !lc.selfETBTrigger() {
		return lCond{}, refuse("%s outside the card's own self-ETB trigger", what)
	}
	f, err := lc.filter("Card.Self+kicked")
	if err != nil {
		return lCond{}, err
	}
	return lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}, nil
}

// castCollectedCondition lowers ConditionDefined$ CastSA>Collected |
// ConditionPresent$ Card (the paid test; no ConditionCompare$ carrier).
func (lc *lowerCtx) castCollectedCondition(ps *params, present string) (lCond, error) {
	if strings.TrimSpace(present) != "Card" {
		return lCond{}, refuse("ConditionDefined$ CastSA>Collected with ConditionPresent$ %q", present)
	}
	if cmp, ok := ps.get("ConditionCompare"); ok {
		return lCond{}, refuse("ConditionDefined$ CastSA>Collected with ConditionCompare$ %q", cmp)
	}
	return lc.castCollectedPerm("ConditionDefined$ CastSA>Collected")
}

// castCollectedSVar reports whether a trigger's CheckSVar$ names an SVar
// whose body is exactly CastSA>Collected$Amount, and lowers it.
func (lc *lowerCtx) castCollectedSVar(tp *params, name string) (lCond, bool, error) {
	if strings.TrimSpace(lc.face.SVars[strings.TrimSpace(name)]) != "CastSA>Collected$Amount" {
		return lCond{}, false, nil
	}
	if cmp, ok := tp.get("SVarCompare"); ok && strings.TrimSpace(cmp) != "GE1" {
		return lCond{}, true, refuse("CastSA>Collected$Amount compared with %q", cmp)
	}
	c, err := lc.castCollectedPerm("CheckSVar$ CastSA>Collected$Amount")
	return c, true, err
}
