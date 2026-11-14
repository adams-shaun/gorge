package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A planner-deferred source (a depletion land's counter rider, a Taps-deferred
// any-colour source: scriptTapsDefer) beside a granted mana ability or a
// tap-another-permanent source. gorge's payment planner defers the one and its
// census omits the other, so one prefix search (potential_script.go
// potentialPrefixScript) activates every census gap in battlefield order, the
// deferred source among them. Each search of the kernel lists the deferred
// source as a gap candidate too: Saprazzan Skerry's {U}{U} (a depletion
// counter spent) beside Gemhide Sliver's own granted {T}: Add one mana of any
// color, or beside Birchlore Rangers's tap-two-Elves mana, pays Bloodrite
// Invoker's {8} with the printed lands (CR 601.2g-h, 605.1a, 605.3b).

// stepTapsDefer is the gi of a prefix step that taps a planner-deferred source
// (scriptGapTap's answer col).
const stepTapsDefer int16 = -7

// tdTap taps the planner-deferred gap sl for play a with the search's answer
// ans: the source's own cost and rider run (a depletion counter spent, the
// source sacrificed on its last), its mana floating plain.
func (s *State) tdTap(k *prep, p uint8, a Action, sl Slot, ans int) {
	var spill [kop.NumManaKinds]uint8
	s.scriptGapTap(k, p, a, sl, slotMask{}, slotMask{}, slotMask{}, slotMask{}.with(sl), ans, &spill)
	s.sba(k) // the script returns to priority after the step (as ctrGap's does)
}

// tdBranch taps the planner-deferred candidate i (slot sl) when it is still
// an activatable gap: one ask node when its activation asks a colour, then each
// answer in the search's order.
func (g *grantSrch) tdBranch(t *State, i int, sl Slot) bool {
	if !t.scriptTapsDefer(g.k, g.p).has(sl) {
		return false
	}
	n := int(t.deferGapAnswers(g.k, sl))
	if n > 1 && !g.node() {
		return false
	}
	for ans := 0; ans < n; ans++ {
		if g.tdTry(t, i, sl, ans) {
			return true
		}
	}
	return false
}

// tdTry taps candidate i by answer ans on a copy of t and continues the search
// there.
func (g *grantSrch) tdTry(t *State, i int, sl Slot, ans int) bool {
	if g.np >= len(g.path) {
		return false
	}
	c := t.fork()
	c.tdTap(g.k, g.p, g.d.play, sl, ans)
	g.path[g.np] = grantStep{i: uint8(i), col: uint8(ans), gi: stepTapsDefer, sl: sl}
	g.np++
	ok := g.next(c, sl)
	if !ok {
		g.np--
	}
	t.unfork()
	return ok
}

// tdBesideGrant reports whether p has a planner-deferred gap while a permanent
// of p's holds a live granted mana ability: the grant search is then gorge's one
// prefix search over both, and an activation's prefix runs it first
// (applyPriority).
func (s *State) tdBesideGrant(k *prep, p uint8) bool {
	return s.scriptTapsDefer(k, p).any() && s.liveGrantGap(k, p)
}
