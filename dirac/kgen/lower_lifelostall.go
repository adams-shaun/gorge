package kgen

import (
	"strconv"
	"strings"
)

// lifeLostAll lowers Mode$ LifeLostAll, "whenever one or more <players> (each)
// lose (exactly N) life" (gorge trigmatch/life.go lifeLostMatches): the
// kernel's TrigLifeLostAll (dirac/trig_lifelostall.go) matches the line once
// per simultaneous life-loss group against each player's summed loss.
// ValidPlayer$ is the Whose; ValidAmountEach$ EQ<n> rides Trigger.N (0: any
// amount). Any other amount comparison, a ValidCause$ (its kernel gate shares
// Trigger.N) and a trigger off the battlefield stay refused.
func (lc *lowerCtx) lifeLostAll(tp *params, tr *lTrigger, zone string) error {
	if zone != "" || tr.Zone != zBattlefield {
		return refuse("LifeLostAll off the battlefield")
	}
	if v, ok := tp.m["ValidCause"]; ok {
		return refuse("LifeLostAll ValidCause$ %q", v)
	}
	vp := tp.str("ValidPlayer")
	w, ok := whoOfPlayer(vp)
	if !ok {
		return refuse("LifeLostAll ValidPlayer %q", vp)
	}
	tr.Whose = w
	if v, ok := tp.get("ValidAmountEach"); ok {
		n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(v), "EQ"))
		if !strings.HasPrefix(strings.TrimSpace(v), "EQ") || err != nil || n < 1 || n > 254 {
			return refuse("LifeLostAll ValidAmountEach$ %q", v)
		}
		tr.Amount = n
	}
	return nil
}
