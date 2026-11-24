package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// lostBoardPrefix is the Defined$ spelling of a LosesGame trigger body's
// last-known battlefield (Miss Highwater: "for each artifact and creature they
// controlled"): `TriggeredLastStateBattlefieldLKICopy.<filter>+ControlledBy
// TriggeredPlayer[,<filter>+ControlledBy TriggeredPlayer]`.
const lostBoardPrefix = "TriggeredLastStateBattlefieldLKICopy."

// lostBoardControlSuffix is the one control clause the selector models. The
// captured board IS what the triggered player controlled when they lost, so
// the clause holds by membership: the departed objects' live Controller has
// been reset by the move, and re-reading it would lose a stolen permanent.
const lostBoardControlSuffix = "+ControlledBy TriggeredPlayer"

// lostBoardDefined resolves the selector against Ctx.TriggerLostBoard, the
// battlefield rules captured at the PlayerLost emit. Each comma-separated
// group filters the captured permanents by its type qualifier; the groups are
// a union in script order, and a permanent matching two groups (an artifact
// creature) is listed once ("for each artifact and creature" is one copy per
// permanent). Any group without the control clause is not modelled: the whole
// spelling is unknown (fail closed). It reports ok=false when spec is not
// this selector. Filtering reads the object's printed face once it has left
// the battlefield, the same approximation every LKI qualifier read makes.
func lostBoardDefined(h Host, c *Ctx, spec string) ([]state.Target, bool) {
	rest, ok := strings.CutPrefix(spec, lostBoardPrefix)
	if !ok {
		return nil, false
	}
	groups := strings.Split(rest, ",")
	quals := make([]string, len(groups))
	for i, grp := range groups {
		q, ok := strings.CutSuffix(strings.TrimSpace(grp), lostBoardControlSuffix)
		if !ok {
			return nil, false
		}
		quals[i] = q
	}
	g := h.Game()
	var out []state.Target
	seen := make(map[state.ObjID]bool, len(c.TriggerLostBoard))
	for _, q := range quals {
		for _, id := range c.TriggerLostBoard {
			if seen[id] {
				continue
			}
			if o := g.Obj(id); o != nil && definedCardQualifierMatches(h, g, c, q, o) {
				seen[id] = true
				out = append(out, state.Target{Obj: id})
			}
		}
	}
	return out, true
}
