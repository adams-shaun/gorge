package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// splitEvolveGrant takes every bare "Evolve" term out of a `&` keyword list:
// the rest of the list and how many there were. Each term is its own
// instance (CR 702.100a, 113.2c): gorge checkGrantedStaticTriggersUsing
// (rules/trigger_granted.go) queues one trigger per granted keyword entry,
// beside the printed K:Evolve's own.
func splitEvolveGrant(kw string) (string, int) {
	var rest []string
	n := 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if k == "Evolve" {
			n++
			continue
		}
		rest = append(rest, k)
	}
	return strings.Join(rest, " & "), n
}

// evolveGrantTrigger lowers the trigger a granted Evolve gives its holder
// (CR 702.100a; Tyranid Prime, Propagator Drone) as a latent ability and
// returns its index, one per card. It has the printed expansion's shape
// (cards.kwEvolve): ChangesZone to the battlefield of another creature you
// control, Evolve$ True (readEvolveCondition folds it into Tf2Evolve), body
// DB$ PutCounter P1P1 on Self. The kernel lists it once per granting static
// among the holder's granted triggers (grantedAbilitiesN), so the holder
// triggers once per instance beside any printed Evolve.
func (lc *lowerCtx) evolveGrantTrigger() (int, error) {
	const name = "GrantedEvolve"
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	t := cards.Trigger{
		Mode: "ChangesZone",
		Params: map[string]string{"Mode": "ChangesZone", "Destination": "Battlefield",
			"ValidCard": "Creature.YouCtrl+Other", "Evolve": "True", "TriggerDescription": "Evolve"},
		Effect: &cards.SA{Kind: "DB", API: "PutCounter", Params: map[string]string{
			"Defined": "Self", "CounterType": "P1P1", "CounterNum": "1",
		}},
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Evolve lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}
