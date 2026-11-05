package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The blind face-down Dig (Vulcan Mind-Meld): ExileFaceDown$ without
// WithMayLook$. gorge applyMoveFaceDown stamps a plain exile as the blind
// exiled_with_face_down marker (the card is face down, nobody is the
// looker); the look is then delivered by the following Effect's
// MayLookAt$ True static over Card.IsRemembered, held by the Effect's owner.
// Two exilers occur, and the player who exiled each card is the one who may
// play it (the Effect's MayPlay$ holder):
//   - the controller: "Defined$ Targeted" with no DefinedExiler$ (the
//     controller exiles the target's cards, applyMoveFaceDown's default),
//     whose Effect is the controller's (EffectOwner$ You);
//   - the controller's opponent: "Defined$ You | DefinedExiler$ Targeted |
//     ForceReveal$ Targeted | ValidTgts$ Opponent" (the target exiles the
//     controller's cards after looking at them, kop.DigExilerOther), whose
//     Effect belongs to that opponent (EffectOwner$ Targeted).

// blindDig is the pending blind Dig whose Effect lowers next.
type blindDig struct {
	exilerOther bool // the exiler (and the Effect's owner) is the controller's opponent
}

// blindDigShape reads a Dig's blind face-down shape. ok false: not this
// shape (the caller refuses). It consumes DefinedExiler$ and ForceReveal$,
// and records the pending Effect in lc.blind.
func (lc *lowerCtx) blindDigShape(ps *params) (ok bool) {
	exiler, hasExiler := ps.get("DefinedExiler")
	reveal, hasReveal := ps.get("ForceReveal")
	other := false
	switch {
	case !hasExiler && !hasReveal:
		// the controller exiles the target player's cards
		if ps.str("Defined") != "Targeted" {
			return false
		}
	case hasExiler && hasReveal && exiler == "Targeted" && reveal == "Targeted":
		// the target looks at the top card of the controller's library and
		// exiles it
		if ps.str("Defined") != "You" || ps.str("ValidTgts") != "Opponent" {
			return false
		}
		other = true
	default:
		return false
	}
	if !lc.blindDigEffect(ps) {
		return false
	}
	lc.blind = &blindDig{exilerOther: other}
	return true
}

// blindDigEffect: the Dig's SubAbility$ is an Effect whose single static
// grants the look (MayLookAt$ True) and the play of the remembered cards.
func (lc *lowerCtx) blindDigEffect(ps *params) bool {
	sub := cards.ResolveSVar(lc.face.SVars, ps.str("SubAbility"))
	if sub == nil || sub.API != "Effect" {
		return false
	}
	ss, ok := cards.ParseStaticLines(lc.face.SVars[sub.Params["StaticAbilities"]])
	if !ok || len(ss) != 1 {
		return false
	}
	st := ss[0].Params
	return strings.TrimSpace(st["MayLookAt"]) == "True" && strings.TrimSpace(st["MayPlay"]) == "True"
}

// blindEffectOwner reads the pending blind Dig's Effect owner: EffectOwner$
// Targeted for the opponent exiler, You (or none) for the controller. A
// mismatch refuses (the owner must be the exiler, who may play the card).
func (lc *lowerCtx) blindEffectOwner(ps *params) error {
	owner := strings.TrimSpace(ps.m["EffectOwner"])
	switch {
	case lc.blind.exilerOther && owner == "Targeted", !lc.blind.exilerOther && (owner == "You" || owner == ""):
		ps.consume("EffectOwner")
		return nil
	}
	return refuse("Effect EffectOwner$ %q beside a face-down Dig by the %s", owner, map[bool]string{true: "opponent", false: "controller"}[lc.blind.exilerOther])
}

// repeatBlindDig reads Doomsday Excruciator's blind exile: inside a
// RepeatEach player body, the repeated player's own cards exiled face down
// with no look and no play to follow (no SubAbility$). gorge applyMoveFaceDown
// stamps the blind exiled_with_face_down marker (nobody is the looker); the
// kernel's face-down mark is the same state (FFaceDown), its looker bit
// read by no decision.
func (lc *lowerCtx) repeatBlindDig(ps *params) bool {
	if lc.repeatPl == 0 || ps.str("SubAbility") != "" {
		return false
	}
	_, ex := ps.get("DefinedExiler")
	_, rv := ps.get("ForceReveal")
	d := ps.str("Defined")
	return !ex && !rv && (d == "Player.IsRemembered" || d == "Remembered")
}
