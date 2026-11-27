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
// match one draw; the kernel asks it too at the sites a draw can be suspended
// on (repl_draworder.go) and stalls (StallDrawRepl) at the rest rather than
// pick an order whose composition it does not read.
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
	if g.hasDrawProgram() {
		return 1
	}
	return 0
})

// drawProgram is one ReplDrawProgram in flight: its body (active), or the
// Optional$ True election parked for an answer (may, repl_drawmay.go: the
// candidate sel/abi; the site's continuation is the order ask's, laid in
// res.left and g.pickZ/pickLeft).
//
// A hidden pick parked as a queued ask (repl_drawpick.go) suspends its
// OpDraw site, and the loop's frame rides here — the cards p still owes
// counting the replaced one (left), the players still owed (rest) and each
// one's count (n) — because the parked search owns res.left and
// g.pickZ/g.pickLeft (startSearch writes them). waiting survives
// drawProgramDone; drawPickResume consumes the record outright.
type drawProgram struct {
	active  bool
	p       uint8
	chain   drawChain
	may     bool
	sel     Slot
	abi     uint8
	waiting bool
	left    uint8
	rest    uint8
	n       uint8
}

// drawState is coldDraw's record, or nil.
func (s *State) drawState() []drawProgram { return coldDraw.of(s) }

// drawCands are the draw replacements matching one draw, in scan order: the
// battlefield slot, the ability index, and the counts the competition rule
// reads (a program body does not commute with another candidate).
type drawCands struct {
	n    int
	sl   [drawCandMax]Slot
	ab   [drawCandMax]uint8
	all  int // every candidate, stored or not
	prog bool
	more int // the plain ReplDrawMore candidates
}

// drawCandMax bounds the candidates one order ask offers.
const drawCandMax = 8

// drawScan collects the draw replacements outside ch that match the draw p is
// about to make, scanning every seat's battlefield: a line may name its
// player (ReplDrawOpponent / ReplDrawAnyPlayer). empty is the attempted draw
// from an empty library: only the lines gorge's emptyDrawReplacement admits
// may meet it (a mandatory non-Draw body or a bodyless Prevent$ skip).
func (s *State) drawScan(k *prep, p uint8, ch drawChain, empty bool) (c drawCands) {
	g := k.gen
	for seat := uint8(0); seat < MaxSeats; seat++ {
		for m := s.statM.and(s.ctrlM[seat]); m.any(); m = m.rest() {
			sl := m.first()
			cc := s.cardAt(sl)
			if g.replBits[cc]&rbDrawMore == 0 && g.replBits2[cc]&rb2DrawProgram == 0 ||
				ch.has(sl) || s.genLost(k, sl) {
				continue
			}
			cd := &k.t.Cards[cc]
			for i := range cd.Abilities {
				a := &cd.Abilities[i]
				if a.Kind != kop.AbReplacement {
					continue
				}
				switch a.Repl.Kind {
				case kop.ReplDrawMore:
					// A "draw N cards instead" body would draw from the same
					// empty library, so it does not meet an empty-library
					// attempt (gorge emptyDrawReplacement).
					if empty {
						continue
					}
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
					c.more++
				case kop.ReplDrawProgram:
					if empty && !drawProgramMeetsEmpty(a) {
						continue
					}
					if !drawProgramPlayer(a.Repl.C, seat, p) {
						continue
					}
					if a.Repl.C&kop.ReplDrawCauseCyclingNonLand != 0 && !s.drawCauseCycling(k) {
						continue // ValidCause$ Activated.Cycling+nonLand: this draw is not a cycling ability's
					}
					if a.Repl.C&kop.ReplDrawNotFirst != 0 && s.g.drawFirst && p == s.active && s.step == kop.StepDraw {
						continue
					}
					if a.Repl.Cond != 0 && !s.replCondHolds(k, sl, seat, a.Repl.Cond) {
						continue
					}
					c.prog = true
				default:
					continue
				}
				if c.n < drawCandMax {
					c.sl[c.n], c.ab[c.n] = sl, uint8(i)
					c.n++
				}
				c.all++
			}
		}
	}
	return c
}

// drawReplace is draw()'s replacement step when the table carries a
// ReplDrawProgram: it scans every seat's battlefield for the draw p is about
// to make, asks the CR 616.1 order on a competition (repl_draworder.go), and
// runs the one body. A table without a program keeps drawMore's own scan.
// empty is the attempted draw from an empty library: only the lines gorge's
// emptyDrawReplacement admits may meet it (a mandatory non-Draw body or a
// bodyless Prevent$ skip).
func (s *State) drawReplace(p uint8, ch drawChain, empty bool) bool {
	k := s.k()
	if !k.gen.hasDrawProgram() {
		if empty {
			return false // a table with only ReplDrawMore has nothing CR 614.11 admits
		}
		return s.drawMore(p, ch)
	}
	c := s.drawScan(k, p, ch, empty)
	switch {
	case c.all == 0:
		return false
	case c.all > 1:
		if !c.prog {
			// the candidates are all ReplDrawMore, whose composition is
			// commutative; drawMore settles them in scan order exactly as
			// gorge's commutative drawMoreCompetition does (repl_draw.go).
			return s.drawMore(p, ch)
		}
		// gorge asks the CR 616.1 order: two draw replacements of one draw
		// need not commute, so the kernel does not pick one.
		return s.drawOrderPose(k, p, ch, &c)
	}
	return s.drawApply(k, p, c.sl[0], c.ab[0], ch)
}

// drawApply applies the one draw replacement candidate at sl: a ReplDrawMore
// by its own walk, a program body by drawProgramTake.
func (s *State) drawApply(k *prep, p uint8, sl Slot, abi uint8, ch drawChain) bool {
	ab := &k.t.Cards[s.cardAt(sl)].Abilities[abi]
	if ab.Repl.Kind == kop.ReplDrawMore {
		return s.drawMore(p, ch) // the one candidate, applied by its own walk
	}
	if ab.Repl.C&kop.ReplDrawOptional != 0 {
		// gorge parks the draw and asks its player (replChoiceDraw): the
		// kernel poses the same election where its answer can resume the
		// site, and stalls where it cannot (repl_drawmay.go)
		if s.drawMayPose(k, p, sl, abi, ch) {
			return true
		}
		s.capFull(StallDrawRepl)
		return true
	}
	return s.drawProgramTake(k, p, sl, abi, ab, ch)
}

// drawProgramMeetsEmpty reports whether a ReplDrawProgram line meets an
// attempted draw from an empty library (gorge's emptyDrawReplacement): a
// bodyless Prevent$ skip, or a mandatory body whose own root is not a draw
// (a Draw body would draw from the same empty library). Every lowered program
// is mandatory; kgen refuses Optional$ True.
func drawProgramMeetsEmpty(a *kop.Ability) bool {
	if a.Repl.C&kop.ReplDrawSkip != 0 {
		return true
	}
	return a.Repl.C&kop.ReplDrawBodyDraw == 0 && len(a.Prog) > 0
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

// drawCauseCycling is the line's ValidCause$ Activated.Cycling+nonLand gate
// (kop.ReplDrawCauseCyclingNonLand). The draw's cause is the resolving stack
// entry, the kernel's twin of gorge's actionCause for the Draw event
// (engine/rules/trigger_cause.go drawCauseAdmits): a draw outside a resolution
// is caused by nothing, so the gate fails closed, and so does every entry that
// is not an activated ability (a spell, a trigger: gorge's StackKindOf test).
// Inside one, the ability must carry the cycling flag (kop.AfCycling, which
// both Keyword$ Cycling and TypeCycling ride: gorge's cyclingCauseKeywords)
// and its source card must not be a land (the nonLand qualifier, read off the
// cause card exactly as gorge matches it against Card.nonLand). A cycling
// ability granted from off the card keeps its own AfCycling, so the entry
// reads it the same way; an entry whose card or ability the table cannot name
// fails closed.
func (s *State) drawCauseCycling(k *prep) bool {
	e := &s.res.e
	if !s.res.active || e.kind != seAbility || e.card == NoCard {
		return false
	}
	cd := &k.t.Cards[e.card]
	if int(e.ab) >= len(cd.Abilities) {
		return false
	}
	a := &cd.Abilities[e.ab]
	return a.Kind == kop.AbActivated && a.AFlags&kop.AfCycling != 0 && k.cards[e.card].types&kop.TLand == 0
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
	if pk := s.drawParkRecord(); len(pk) > 0 {
		pk[0] = drawPark{} // a new program: no park of the last one survives
	}
	nask := s.g.nask
	s.runReplProgAs(k, sel, s.cardAt(sel), abi, 0, sel)
	if s.res.ask == raNone && s.g.nask == nask {
		ds[0] = save // the program finished (a posed or queued ask leaves it live)
	}
	return true
}

// drawProgramDone clears the in-flight program after an answered ask's
// resumed body finishes. A parked pick's draw-loop frame survives it (the
// body is done, so active and the chain die, but the suspended OpDraw still
// owes its remaining cards): drawPickResume reads the frame and clears the
// record outright.
func (s *State) drawProgramDone() {
	if ds := s.drawState(); len(ds) > 0 {
		r := &ds[0]
		// A parked pick's frame (waiting, left, rest, n) survives; a parked
		// Dig body's (repl_drawbody.go coldDrawPark) does not: drawBodyEndRes
		// and drawBodyEnd read it from a copy taken before this call.
		*r = drawProgram{waiting: r.waiting, p: r.p, left: r.left, rest: r.rest, n: r.n}
	}
	if pk := s.drawParkRecord(); len(pk) > 0 {
		pk[0] = drawPark{}
	}
}

// hasDrawProgram: some card has a ReplDrawProgram replacement, so draw() runs
// the program scan. It reads anyRepl2 rather than a field of its own: a new
// genPrep field is fixed State (TestNoNewFixedStateFields).
func (g *genPrep) hasDrawProgram() bool { return g.anyRepl2&rb2DrawProgram != 0 }
