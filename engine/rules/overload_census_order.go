package rules

import "github.com/adams-shaun/gorge/engine/state"

// candidatesPlayersFrom reorders the player candidates of an overload census
// into the given seat order (the controller's APNAP walk, CR 101.4), leaving
// object candidates and the slots the players occupy where they were. Players
// absent from order (dead seats) are dropped.
func candidatesPlayersFrom(cands []targetCandidate, order []state.PlayerID) []targetCandidate {
	byPlayer := make(map[state.PlayerID]targetCandidate, len(order))
	for _, c := range cands {
		if c.kind == "player" {
			byPlayer[c.player] = c
		}
	}
	out := make([]targetCandidate, 0, len(cands))
	next := 0
	for _, c := range cands {
		if c.kind != "player" {
			out = append(out, c)
			continue
		}
		for next < len(order) {
			p := order[next]
			next++
			if pc, ok := byPlayer[p]; ok {
				out = append(out, pc)
				break
			}
		}
	}
	return out
}

// overloadTargets turns an overload census into the resolution Ctx.Targets
// list, players in the controller's APNAP order.
func overloadTargets(cands []targetCandidate, order []state.PlayerID) []state.Target {
	var out []state.Target
	for _, cand := range candidatesPlayersFrom(cands, order) {
		if cand.kind == "player" {
			out = append(out, state.Target{Player: cand.player, IsPlayer: true})
		} else {
			out = append(out, state.Target{Obj: cand.obj})
		}
	}
	return out
}
