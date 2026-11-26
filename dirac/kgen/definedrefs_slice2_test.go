package kgen

import (
	"strings"
	"testing"
)

// definedRefsSlice2 are cards whose only blocker was a Defined$ referent
// gorge resolves and kgen refused, each now lowered onto a kernel reference
// that names the same object or player:
//
//   - hand move DefinedPlayer$ TargetedController / ParentTarget /
//     TriggeredPlayer (gorge handMoveOwners -> searchPlayersFor, the search's
//     owner reader: lower_dpsearch.go);
//   - Phases Defined$ Targeted / Equipped / Enchanted, and Valid <filter> over
//     the controller's own permanents (gorge phasesAffectedObjects);
//   - ExchangeControl Defined$ Self beside one target (the source first);
//   - SacrificeAll Defined$ Enchanted (the source's host);
//   - TriggeredTargetController on a self-source DamageDone to a creature
//     (the per-referent queue's damaged object);
//   - TriggeredAttackerController on a printed become-blocked line (the
//     attacker's controller gorge captures as it queues).
var definedRefsSlice2 = []string{
	"Assembled Alphas", "Avarice Totem", "Bellowing Fiend", "Braids, Conjurer Adept", "Conjured Currency",
	"Death Charmer", "Divine Gambit", "Eyes Everywhere", "Flayed Nim", "Meglonoth", "Metamorphose",
	"Parasitic Implant", "Phyrexian Infiltrator", "Prying Questions", "Robe of Stars", "Slip Out the Back",
	"Slow Motion", "Soul Charmer", "Taniwha", "Vanishing",
}

func TestDefinedRefsSlice2(t *testing.T) {
	for _, name := range definedRefsSlice2 {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// The forms the slice leaves refused: a TriggeredTargetController read on a
// line whose sources are several objects (the per-referent latch would fire
// once where gorge fires per event), a defending player on a become-blocked
// line whose attacker may be another player's (the duel read names the
// trigger controller's opponent), and a Destroy over a compound Defined$
// (gorge destroys both as one batch).
func TestDefinedRefsSlice2StillRefused(t *testing.T) {
	for name, why := range map[string]string{
		"Greatbow Doyen":   `Defined$ "TriggeredTargetController" without a damage object referent`,
		"Tormentor's Helm": `Defined$ "TriggeredDefendingPlayer" outside an attack trigger of the controller's own attack`,
		"Loyal Sentry":     `Defined$ "TriggeredAttackerLKICopy & Self"`,
	} {
		c := lowerNamedCard(t, name)
		found := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, why) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s refusals = %v, want one naming %q", name, c.Refusals, why)
		}
	}
}

// The lowered shapes: the source is the exchange's first member, the
// Valid-filter phase-out is a card loop over the controller's lands, and the
// hand move's owner is the exiled target's controller.
func TestDefinedRefsSlice2Shapes(t *testing.T) {
	cases := []struct {
		card string
		op   string
		obj  refKind
	}{
		{"Avarice Totem", "ExchangeControl", rSelf},
		{"Taniwha", "Phases", rEach},
		{"Divine Gambit", "ChooseMove", rTargetController},
		{"Braids, Conjurer Adept", "ChooseMove", rTriggeredPlayer},
		{"Robe of Stars", "Phases", rEquipped},
		{"Bellowing Fiend", "DealDamage", rTriggeredTgtCtrl},
		{"Meglonoth", "DealDamage", rTriggeredPlayer},
	}
	for _, tc := range cases {
		c := lowerNamedCard(t, tc.card)
		found := false
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op == tc.op && in.Obj.Kind == tc.obj {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: no %s over %s", tc.card, tc.op, refNames[tc.obj])
		}
	}
}
