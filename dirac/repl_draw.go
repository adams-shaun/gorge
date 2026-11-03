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
// Two replacements matching one draw are gorge's CR 616.1 order ask, which
// the kernel does not model: it stalls (StallDrawRepl).

// cfgxDrawMore: the table has a ReplDrawMore ability, so draw() consults
// drawMore. Set once at Init (engine.go).
const cfgxDrawMore uint8 = 1 << 5

// StallDrawRepl: two draw replacements match one draw (gorge asks the CR
// 616.1 order), or one lineage applies more than drawChainMax of them.
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
			sel, ab = sl, a
			n++
		}
	}
	switch {
	case n == 0:
		return false
	case n > 1 || ch.n >= drawChainMax:
		s.capFull(StallDrawRepl)
		return true
	}
	ch.sl[ch.n] = sel
	ch.n++
	for i := int16(0); i < ab.Repl.A && !s.over; i++ {
		s.drawExcl(p, ch)
	}
	return true
}
