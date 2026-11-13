package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The damaged player's figures a combat-damage-to-a-player trigger compares
// against everyone else's: Battle Angels of Tyr's "draw a card if that
// player has more cards in hand than each other player ... controls more
// lands ... has more life". gorge reads the left side through
// evalPlayerRefProperty's default arm (definedSpec(TriggeredTarget), the
// damaged player; defaultRefPropertyModelled admits CardsInHand and the
// Valid family for TriggeredTarget only) and the right side through
// playerCountNonTriggeredTarget (engine/effects/count_player.go: every
// living player but the damaged one, the controller included). Both are
// lowered only where the kernel's trigger player is the damaged player
// (damagedPlayerIsTriggerPlayer: the damaged seat at any seat count, dirac
// dmgSeat), as the TriggeredTarget$LifeTotal arm is.

// trigTargetPlayerValue lowers TriggeredTarget$CardsInHand (the raw hand
// length, gorge len(g.Zone(ZHand, p))) and TriggeredTarget$Valid <spec>
// (gorge: the damaged player's battlefield, matched with that player as the
// spec's You). Each Valid alternative "<Type>[.<props>]" has its YouCtrl
// property (or, with none, the whole battlefield read gorge scopes to the
// player) rewritten to the trigger-player controller predicate
// (kop.PredTrigPlayerCtrl, triggeredTargetCtrl's word); any other player word
// (You, Opp, Owner, ...) would bind to the controller's side, so it refuses,
// and a second comparison beside the predicate refuses in the filter parser.
func (lc *lowerCtx) trigTargetPlayerValue(rest string) (lValue, error) {
	if !lc.damagedPlayerIsTriggerPlayer() {
		return lValue{}, refuse("TriggeredTarget$%s outside a damage-to-player trigger the kernel's trigger player names", rest)
	}
	if rest == "CardsInHand" {
		return lValue{Kind: vHandSizeOf, Ref: lRef{Kind: rTriggeredPlayer}}, nil
	}
	spec := strings.TrimSpace(strings.TrimPrefix(rest, "Valid "))
	alts := strings.Split(spec, ",")
	for i, alt := range alts {
		typ, props, _ := strings.Cut(strings.TrimSpace(alt), ".")
		if typ == "" || typ == "Card" || strings.ContainsAny(typ, "+_ ") {
			return lValue{}, refuse("TriggeredTarget$Valid %q: alternative type %q", spec, typ)
		}
		kept := []string{}
		for _, p := range strings.Split(props, "+") {
			switch {
			case p == "" || p == "YouCtrl":
			case strings.Contains(p, "You") || strings.Contains(p, "Opp") || strings.Contains(p, "Own") || strings.Contains(p, "Ctrl") || strings.Contains(p, "Player"):
				return lValue{}, refuse("TriggeredTarget$Valid %q: player word %q", spec, p)
			default:
				kept = append(kept, p)
			}
		}
		alts[i] = typ + "." + strings.Join(append(kept, trigPlayerCtrlWord), "+")
	}
	f, err := lc.filter(strings.Join(alts, ","))
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vCount, F: f}, nil
}

// nonTrigTargetExtreme lowers PlayerCountDefinedNonTriggeredTarget$
// HighestCardsInHand / HighestLifeTotal / HighestValid <spec> (gorge
// playerCountNonTriggeredTarget: lifeExtreme, then playerCountExtreme's
// scalar and battlefield-count arms, each member read from their own side)
// as ExOppExtreme over kop.OppExtNonTrigPl's group. Every other property,
// the Lowest forms included (no corpus carrier), refuses.
func (lc *lowerCtx) nonTrigTargetExtreme(rest string) (lValue, error) {
	if !lc.damagedPlayerIsTriggerPlayer() {
		return lValue{}, refuse("PlayerCountDefinedNonTriggeredTarget$%s outside a damage-to-player trigger the kernel's trigger player names", rest)
	}
	switch {
	case rest == "HighestCardsInHand":
		return lValue{Kind: vOppExtreme, N: kop.OppExtNonTrigPl | kop.OppExtHand}, nil
	case rest == "HighestLifeTotal":
		return lValue{Kind: vOppExtreme, N: kop.OppExtNonTrigPl | kop.OppExtLife}, nil
	case strings.HasPrefix(rest, "HighestValid "):
		f, err := lc.filter(strings.TrimPrefix(rest, "HighestValid "))
		if err != nil {
			return lValue{}, err
		}
		// the kernel rebinds only the controller words to the member, as
		// the PlayerCountOpponents$HighestValid arm checks
		if _, err := opponentView(f); err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vOppExtreme, N: kop.OppExtNonTrigPl | kop.OppExtValid, F: f}, nil
	}
	return lValue{}, refuse("PlayerCountDefinedNonTriggeredTarget$%s", rest)
}
