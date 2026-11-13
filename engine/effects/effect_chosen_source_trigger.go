package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// effectEventRegistration is an Effect event-mode trigger's registration body
// and captured IDs: the trigger's name and the Effect's remembered objects, or
// the chosen-source rewrite of effectChosenSourceTrigger when it applies. The
// Effect's remembered players join the capture either way.
func effectEventRegistration(h Host, c *Ctx, sa *cards.SA, tr cards.Trigger, name, raw string, regIDs []state.Target) (string, []state.Target) {
	body := name
	if b, snap, ok := effectChosenSourceTrigger(h, c, tr, raw); ok {
		body, regIDs = b, snap
	}
	return body, withRememberedPlayers(regIDs, effectRememberedPlayers(h, c, sa))
}

// effectChosenSourceTrigger reads an Effect's DamageDone trigger whose
// ValidSource$ is exactly Card.ChosenCardStrict (Azra Oddsmaker: "whenever that
// creature deals combat damage to a player this turn"). Forge's EffectEffect
// copies the host's chosen cards onto the effect card it creates, so the
// trigger keeps naming the card chosen at creation after the chain's own
// `DB$ Cleanup | ClearChosenCard$ True` wipes the source's list. The
// registration therefore captures the resolution's chosen objects now and
// stores the body inline with the source read as Card.IsRemembered, which the
// delayed matcher resolves against that capture (CR 611.2a). ok is false for
// any other trigger, for a chosen set with no object (nothing to name: the
// stored body would otherwise read the source's live list, which the cleanup
// has emptied), and for a body whose ValidSource$ spelling the rewrite does
// not match (the capture would pair with an unrewritten, never-firing body).
func effectChosenSourceTrigger(h Host, c *Ctx, tr cards.Trigger, raw string) (body string, ids []state.Target, ok bool) {
	if tr.Mode != "DamageDone" || strings.TrimSpace(tr.Params["ValidSource"]) != "Card.ChosenCardStrict" ||
		!strings.HasPrefix(raw, "Mode$") {
		return "", nil, false
	}
	const from, to = "ValidSource$ Card.ChosenCardStrict", "ValidSource$ Card.IsRemembered"
	body = strings.Replace(raw, from, to, 1)
	if body == raw {
		return "", nil, false
	}
	for _, t := range resolutionChosenCards(h.Game(), c) {
		if !t.IsPlayer && t.Obj != 0 {
			ids = append(ids, state.Target{Obj: t.Obj})
		}
	}
	if len(ids) == 0 {
		return "", nil, false
	}
	return body, ids, true
}
