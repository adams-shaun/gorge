package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// splitFirebendingGrant takes every "Firebending:<N>" term (N a literal
// 1..20) out of a `&` keyword list: the rest of the list and the amounts in
// list order. Like splitWardGrant, but each term counts: gorge
// checkGrantedAttackKeywordTriggers (rules/trigger_granted.go) queues one
// trigger per derived Firebending:<N> entry beyond the printed ones, so two
// grants stack. A non-literal N is refused.
func splitFirebendingGrant(kw string) (string, []int, error) {
	var rest []string
	var ns []int
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		v, ok := strings.CutPrefix(k, "Firebending:")
		if !ok {
			rest = append(rest, k)
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 20 {
			return "", nil, refuse("granted Firebending %q", k)
		}
		ns = append(ns, n)
	}
	return strings.Join(rest, " & "), ns, nil
}

// firebendGrantTrigger lowers the trigger a granted Firebending N gives its
// holder (CR 702.189a, CR 113.2c: each instance triggers separately) as a
// latent Attacks Self ability and returns its index; one per distinct N on
// the card. gorge's granted body is the bare DB$ Mana (Produced$ R,
// Amount$ N, PersistentUntilEndOfCombat$ True) with no ElementalBend marker
// (rules/trigger_granted.go), which manaEffect admits only under fbGranted.
// The holder may be an artifact creature (a Pump's target): gorge tags the
// mana typed (ManaProducerTag), but the pool it enters already refuses a
// planned payment over a persistent unit (PlanPoolOK; the kernel's artFloats
// counts fbR), so the tag adds nothing observable (kfiregrant lockstep, the
// Memnite targets). A grant beside another is listed in layer-6 timestamp
// order (the kernel's grantedAbilitiesN merges override records with global
// grants by timestamp).
func (lc *lowerCtx) firebendGrantTrigger(n int) (int, error) {
	name := "GrantedFirebending" + strconv.Itoa(n)
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	t := cards.Trigger{
		Mode:   "Attacks",
		Params: map[string]string{"Mode": "Attacks", "ValidCard": "Card.Self"},
		Effect: &cards.SA{Kind: "DB", API: "Mana", Params: map[string]string{
			"Produced": "R", "Amount": strconv.Itoa(n), "PersistentUntilEndOfCombat": "True",
		}},
	}
	lc.fbGranted = true
	defer func() { lc.fbGranted = false }()
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Firebending lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}
