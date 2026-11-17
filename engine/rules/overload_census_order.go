package rules

import "github.com/adams-shaun/gorge/engine/state"

// overloadTargets turns an overload census into the resolution Ctx.Targets
// list. Player targets are put in the given seat order (the controller's APNAP
// walk, CR 101.4); object targets and the slots the players occupy stay where
// they were. Players absent from order (dead seats) are dropped.
func overloadTargets(cands []targetCandidate, order []state.PlayerID) []state.Target {
	var all []state.Target
	for _, cand := range cands {
		if cand.kind == "player" {
			all = append(all, state.Target{Player: cand.player, IsPlayer: true})
		} else {
			all = append(all, state.Target{Obj: cand.obj})
		}
	}
	byPlayer := make(map[state.PlayerID]bool, len(order))
	for _, t := range all {
		if t.IsPlayer {
			byPlayer[t.Player] = true
		}
	}
	out := make([]state.Target, 0, len(all))
	next := 0
	for _, t := range all {
		if !t.IsPlayer {
			out = append(out, t)
			continue
		}
		for next < len(order) {
			p := order[next]
			next++
			if byPlayer[p] {
				out = append(out, state.Target{Player: p, IsPlayer: true})
				break
			}
		}
	}
	return out
}
