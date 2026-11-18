package combat

import "github.com/adams-shaun/gorge/engine/state"

// Lure (CR 509.1c, "All creatures able to block CARDNAME do so.") binds every
// creature able to block the lure attacker to block it. The bind is a
// whole-declaration requirement -- gorge publishes it per option and the
// required-team validator enforces the maximum -- so the option list offers a
// pair only while some legal declaration still contains it: a duty blocker is
// offered only its lure blocks while the lure attackers host every duty
// blocker, a blocker the capacity leaves out keeps its plain pairs, and a
// non-lure attacker with a blocker floor of two or more (menace, a Min$) is
// offered a pair only while the creatures outside the duty can still field
// the floor. Package dirac's blockerOptionsLure mirrors the same rules pick
// by pick, so the one ask gorge poses and the picks the lockstep poses
// publish the same pairs.

// LureDutyBlockers lists the defenders that may declare a block against some
// lure attacker (admissible says so): the creatures CR 509.1c's duty binds.
func LureDutyBlockers(defenders, lure []state.ObjID, admissible func(blocker, attacker state.ObjID) bool) []state.ObjID {
	var duty []state.ObjID
	for _, b := range defenders {
		for _, a := range lure {
			if admissible(b, a) {
				duty = append(duty, b)
				break
			}
		}
	}
	return duty
}

// LureSpillBlockers lists the duty blockers the lure attackers cannot bind:
// the most duty blockers the lure attackers can host does not fall when the
// blocker leaves the duty, so the duty's surplus is free and its plain pairs
// stay offerable (CR 509.1c's "if able"). single reports whether a lure
// attacker hosts one blocker at most; the others take any number.
func LureSpillBlockers(duty, lure []state.ObjID, single func(attacker state.ObjID) bool, admissible func(blocker, attacker state.ObjID) bool) map[state.ObjID]bool {
	max := lureMaxHost(duty, lure, single, admissible)
	spill := make(map[state.ObjID]bool, len(duty))
	for _, b := range duty {
		rest := make([]state.ObjID, 0, len(duty)-1)
		for _, d := range duty {
			if d != b {
				rest = append(rest, d)
			}
		}
		if lureMaxHost(rest, lure, single, admissible) == max {
			spill[b] = true
		}
	}
	return spill
}

// lureMaxHost is the most duty blockers the lure attackers can host: a lure
// attacker that hosts one blocker at most takes one further blocker, the
// others any number, and every assignment needs an admissible pair.
func lureMaxHost(duty, lure []state.ObjID, single func(attacker state.ObjID) bool, admissible func(blocker, attacker state.ObjID) bool) int {
	var big, one []state.ObjID
	for _, a := range lure {
		if single(a) {
			one = append(one, a)
		} else {
			big = append(big, a)
		}
	}
	free := 0
	rest := make([]state.ObjID, 0, len(duty))
	for _, b := range duty {
		taken := false
		for _, a := range big {
			if admissible(b, a) {
				taken = true
				break
			}
		}
		if taken {
			free++
		} else {
			rest = append(rest, b)
		}
	}
	if free >= len(duty) {
		return len(duty)
	}
	return free + lureMatchHost(rest, one, admissible)
}

// lureMatchHost is the maximum bipartite matching of duty blockers onto the
// lure attackers that host one blocker at most (each edge an admissible
// pair), one augmenting path per blocker.
func lureMatchHost(duty, slots []state.ObjID, admissible func(blocker, attacker state.ObjID) bool) int {
	owner := make([]int, len(slots)) // slot j holds duty[owner[j]-1], 0 when none
	var augment func(bi int, seen []bool) bool
	augment = func(bi int, seen []bool) bool {
		for j, a := range slots {
			if seen[j] || !admissible(duty[bi], a) {
				continue
			}
			seen[j] = true
			if owner[j] == 0 || augment(owner[j]-1, seen) {
				owner[j] = bi + 1
				return true
			}
		}
		return false
	}
	n := 0
	for bi := range duty {
		if augment(bi, make([]bool, len(slots))) {
			n++
		}
	}
	return n
}

// LureFloorMet reports whether blocker may still be offered its pair against
// a non-lure attacker with a blocker floor of two or more: the floor is met
// by blocker itself, the blockers already declared against the attacker, and
// the defender's other creatures outside the lure duty -- a duty blocker is
// bound to the lure (CR 509.1c), so it is no partner. admissible is the pair
// oracle the offer itself uses.
func LureFloorMet(floor, declared int, blocker, attacker state.ObjID, defenders []state.ObjID, duty map[state.ObjID]bool, admissible func(blocker, attacker state.ObjID) bool) bool {
	n := 1 + declared
	for _, b := range defenders {
		if b != blocker && !duty[b] && admissible(b, attacker) {
			n++
		}
	}
	return n >= floor
}
