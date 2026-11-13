package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/scriptfacts"
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

// grantEvolve takes the bare Evolve terms out of a static's granted keyword
// list and appends one latent trigger per instance to st.Grant (CR 702.100a,
// 113.2c), returning the rest of the list. Lives here so lowerCtx.continuous
// gains one call, not another per-instance loop.
func (lc *lowerCtx) grantEvolve(kw string, st *lStatic) (string, error) {
	rest, n := splitEvolveGrant(kw)
	for ; n > 0; n-- {
		idx, err := lc.evolveGrantTrigger()
		if err != nil {
			return "", err
		}
		st.Grant = append(st.Grant, idx)
	}
	return rest, nil
}

// evolveGrantTrigger lowers the trigger a granted Evolve gives its holder
// (CR 702.100a; Tyranid Prime, Propagator Drone) as a latent ability and
// returns its index, one per card. It has the printed expansion's shape
// (cards.kwEvolve, via scriptfacts): ChangesZone to the battlefield of another creature you
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
	// One shared shape: the same synthesizer gorge queues the granted
	// trigger from (scriptfacts.GrantedKeywordTrigger), so the kernel and the
	// engine cannot drift apart on what a granted Evolve is.
	tp := scriptfacts.GrantedKeywordTrigger("Evolve")
	if tp == nil {
		return 0, refuse("granted Evolve lowering")
	}
	t := *tp
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
