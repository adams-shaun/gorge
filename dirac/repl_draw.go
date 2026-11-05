package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplDrawMore: "If you would draw a card, draw N cards instead" (Mr. Spock,
// Alhammarret's Archive, Bard, Vnwxt). gorge's applyReplacements matches the
// Draw event of a card that really leaves a non-empty library (an empty
// library only marks the player, CR 704.5b), runs the body, and the body's
// N draws are new events that meet every draw replacement but the ones
// already applied to this draw's lineage (CR 614.5, 616.1e: replDrawChain).
// drawExcl and drawMore are that walk: the chain is the sources applied so
// far, the draw plays plainly once none is left.
//
// Two replacements matching one draw draw N1·N2 cards either way -- the
// composition is commutative, so gorge settles the CR 616.1 order in scan
// order, unasked, and the chain walks the body's draws through the rest.
// More than drawChainMax sources in one lineage is the declared
// StallDrawRepl capacity.

// cfgxDrawMore: the table has a ReplDrawMore ability, so draw() consults
// drawMore. Set once at Init (engine.go).
const cfgxDrawMore uint8 = 1 << 5

// StallDrawRepl: one draw's lineage applies more than drawChainMax draw
// replacements (four or more sources). Two or three settle in scan order,
// gorge's commutative drawMoreCompetition, unasked.
const StallDrawRepl = 103

func init() { stallNames[StallDrawRepl] = "drawrepl" }

// drawChainMax is how many draw replacements one draw's lineage holds.
const drawChainMax = 3

// drawChain is the draw replacements (their battlefield slots) already
// applied to a draw's lineage; the zero value is empty.
type drawChain struct {
	n  uint8
	sl [drawChainMax]Slot
}

func (c drawChain) has(sl Slot) bool {
	for i := uint8(0); i < c.n; i++ {
		if c.sl[i] == sl {
			return true
		}
	}
	return false
}

// drawMore replaces the draw player p is about to make when exactly one
// draw replacement outside ch applies, drawing its N cards in the draw's
// place. It reports whether it took the draw (replaced it, or stalled).
//
//go:noinline
func (s *State) drawMore(p uint8, ch drawChain) bool {
	k := s.k()
	g := k.gen
	var sel Slot
	var ab *kop.Ability
	n := 0
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		sl := m.first()
		if g.replBits[s.cardAt(sl)]&rbDrawMore == 0 || ch.has(sl) || s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDrawMore {
				continue
			}
			if a.Repl.Cond != 0 && !s.replHostLive(k, a, sl) { // a granted max-speed gate (kw_maxspeed.go)
				continue
			}
			// NotFirstCardInDrawStep$: the first card the player draws in
			// their own draw step is exempt (gorge pendingDrawIsFirstInDrawStep)
			if a.Repl.C&kop.ReplDrawNotFirst != 0 && s.g.drawFirst && p == s.active && s.step == kop.StepDraw {
				continue
			}
			if n == 0 {
				sel, ab = sl, a // the scan-order first: gorge settles here too
			}
			n++
		}
	}
	switch {
	case n == 0:
		return false
	case ch.n >= drawChainMax:
		s.capFull(StallDrawRepl)
		return true
	}
	// Two or more candidates used to stall (StallDrawRepl): gorge asked the
	// CR 616.1 order. gorge no longer poses that ask -- every candidate a
	// lockstep game can carry is a mandatory "draw N cards instead"
	// (lower_drawrepl.go refuses every other Draw line), so the composition
	// draws N1·N2 cards from the library in the same order either way and
	// gorge applies the scan-order first (engine/rules/replacement.go,
	// drawMoreCompetition). The chain keeps the rest eligible for the body's
	// draws (CR 614.5, 616.1e), so the walk below reaches them.
	ch.sl[ch.n] = sel
	ch.n++
	for i := int16(0); i < ab.Repl.A && !s.over; i++ {
		s.drawExcl(p, ch)
	}
	return true
}
