package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// splitExaltedGrant takes every bare "Exalted" term out of a `&` keyword
// list: the rest of the list and how many there were. Each term is its own
// instance (CR 702.83b, 113.2c): gorge checkGrantedExaltedTriggers
// (rules/exalted_granted.go) queues one trigger per derived Exalted entry
// beyond the printed lines.
func splitExaltedGrant(kw string) (string, int) {
	var rest []string
	n := 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if k == "Exalted" {
			n++
			continue
		}
		rest = append(rest, k)
	}
	return strings.Join(rest, " & "), n
}

// exaltedGrantTrigger lowers the trigger a granted Exalted gives its holder
// (CR 702.83a) as a latent ability and returns its index, one per card. It
// has the printed expansion's shape (cards.kwExalted): Attacks, ValidCard$
// Creature.YouCtrl, Alone$ True, DB$ Pump the triggering attacker +1/+1.
// The kernel lists it once per granting static among the holder's granted
// triggers (grantedAbilitiesN), so a creature holding the keyword through
// two grants, or one grant beside a printed line, triggers once per
// instance.
func (lc *lowerCtx) exaltedGrantTrigger() (int, error) {
	const name = "GrantedExalted"
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	t := cards.Trigger{
		Mode: "Attacks",
		Params: map[string]string{"Mode": "Attacks", "ValidCard": "Creature.YouCtrl", "Alone": "True",
			"TriggerDescription": "Exalted"},
		Effect: &cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{
			"Defined": "TriggeredAttacker", "NumAtt": "+1", "NumDef": "+1",
		}},
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Exalted lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}
