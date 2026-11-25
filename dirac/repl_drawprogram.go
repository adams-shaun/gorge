package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ReplDrawProgram: a Draw replacement whose ReplaceWith$ body is not a plain
// "draw N cards instead" (ReplDrawMore). gorge's applyReplacements runs the
// body in the draw's place (engine/rules/replacement.go), with the draw's
// player available as ReplacedPlayer. The ability's Prog is that body, run by
// runReplProgAs with the replacement's permanent as the source; a bodyless
// Prevent$ line (ReplDrawSkip) replaces the draw with nothing.
//
// The scan is the whole battlefield, not the drawer's: a line may name its
// player (ReplDrawOpponent / ReplDrawAnyPlayer: Hullbreacher, Notion Thief,
// Possessed Portal). gorge asks the CR 616.1 order when two replacements
// match one draw; the kernel stalls (StallDrawRepl) rather than pick an order
// whose composition it does not read.
//
// The draw's lineage (drawChain) rides coldDraw: a body's own OpDraw must not
// meet the replacement that is replacing it (CR 614.5), and the program may
// pause on an ask, so the chain and the draw's player live in a cold region
// until the program finishes.

// coldDraw is the draw replacement program in flight: its player (p), its
// draw lineage (chain) and whether it is active. A table without a
// ReplDrawProgram has no region.
func init() { genNeed(func(t *kop.Table) bool { return tableHasReplKind(t, kop.ReplDrawProgram) }) }

var coldDraw = newCold[drawProgram]("drawProgram", func(g *genPrep) int {
	if g.anyDrawProgram {
		return 1
	}
	return 0
})

// drawProgram is one ReplDrawProgram body in flight.
type drawProgram struct {
	active bool
	p      uint8
	chain  drawChain
}

// drawState is coldDraw's record, or nil.
func (s *State) drawState() []drawProgram { return coldDraw.of(s) }

// drawReplace is draw()'s replacement step when the table carries a
// ReplDrawProgram: it scans every seat's battlefield for the draw p is about
// to make, stalls on a competition, and runs the one body. A table without a
// program keeps drawMore's own scan.
func (s *State) drawReplace(p uint8, ch drawChain) bool {
	k := s.k()
	if !k.gen.anyDrawProgram {
		return s.drawMore(p, ch)
	}
	g := k.gen
	var sel Slot
	var ab *kop.Ability
	abi := 0
	n := 0
	hasProgram := false
	for seat := uint8(0); seat < MaxSeats; seat++ {
		for m := s.statM.and(s.ctrlM[seat]); m.any(); m = m.rest() {
			sl := m.first()
			c := s.cardAt(sl)
			if g.replBits[c]&rbDrawMore == 0 && g.replBits2[c]&rb2DrawProgram == 0 ||
				ch.has(sl) || s.genLost(k, sl) {
				continue
			}
			cd := &k.t.Cards[c]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbReplacement {
					continue
				}
				isProg := false
				switch a.Repl.Kind {
				case kop.ReplDrawMore:
					// ReplDrawMore is the controller's own draw (ValidPlayer$ You)
					if seat != p {
						continue
					}
					if a.Repl.C&kop.ReplDrawNotFirst != 0 && s.g.drawFirst && p == s.active && s.step == kop.StepDraw {
						continue
					}
					if a.Repl.Cond != 0 && !s.replHostLive(k, a, sl) {
						continue
					}
				case kop.ReplDrawProgram:
					if !drawProgramPlayer(a.Repl.C, seat, p) {
						continue
					}
					isProg = true
					if a.Repl.C&kop.ReplDrawNotFirst != 0 && s.g.drawFirst && p == s.active && s.step == kop.StepDraw {
						continue
					}
					if a.Repl.Cond != 0 && !s.replCondHolds(k, sl, seat, a.Repl.Cond) {
						continue
					}
				default:
					continue
				}
				if n == 0 {
					sel, ab, abi = sl, a, i
				}
				if isProg {
					hasProgram = true
				}
				n++
			}
		}
	}
	switch {
	case n == 0:
		return false
	case n > 1:
		if !hasProgram {
			// the candidates are all ReplDrawMore, whose composition is
			// commutative; drawMore settles them in scan order exactly as
			// gorge's commutative drawMoreCompetition does (repl_draw.go).
			return s.drawMore(p, ch)
		}
		// gorge asks the CR 616.1 order: two draw replacements of one draw
		// need not commute, so the kernel does not pick one.
		s.capFull(StallDrawRepl)
		return true
	}
	if ab.Repl.Kind == kop.ReplDrawMore {
		return s.drawMore(p, ch) // the one candidate, applied by its own walk
	}
	return s.drawProgramTake(k, p, sel, uint8(abi), ab, ch)
}

// drawProgramPlayer reports whether a ReplDrawProgram's ValidPlayer$ admits the
// draw by p when the replacement is controlled by seat ctrl.
func drawProgramPlayer(bits int16, ctrl, p uint8) bool {
	switch {
	case bits&kop.ReplDrawAnyPlayer != 0:
		return true
	case bits&kop.ReplDrawOpponent != 0:
		return ctrl != p
	default:
		return ctrl == p
	}
}

// drawProgramTake applies the one ReplDrawProgram candidate: it extends the
// draw's lineage, records the in-flight program in coldDraw, and runs the
// body (or skips the draw). The chain is cleared when the program finishes;
// an ask leaves it in place until the resumed program does.
func (s *State) drawProgramTake(k *prep, p uint8, sel Slot, abi uint8, ab *kop.Ability, ch drawChain) bool {
	if ch.n >= drawChainMax {
		s.capFull(StallDrawRepl)
		return true
	}
	ch.sl[ch.n] = sel
	ch.n++
	if ab.Repl.C&kop.ReplDrawSkip != 0 {
		return true // Prevent$: the draw is replaced with nothing
	}
	ds := s.drawState()
	if len(ds) == 0 {
		s.capFull(StallDrawRepl) // a program with no region: the table is inconsistent
		return true
	}
	save := ds[0]
	ds[0] = drawProgram{active: true, p: p, chain: ch}
	nask := s.g.nask
	s.runReplProgAs(k, sel, s.cardAt(sel), abi, 0, sel)
	if s.res.ask == raNone && s.g.nask == nask {
		ds[0] = save // the program finished (a posed or queued ask leaves it live)
	}
	return true
}

// drawProgramDone clears the in-flight program after an answered ask's
// resumed body finishes.
func (s *State) drawProgramDone() {
	if ds := s.drawState(); len(ds) > 0 {
		ds[0] = drawProgram{}
	}
}
