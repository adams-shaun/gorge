package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// drawRepl lowers the controller's "if you would draw a card, draw N cards
// instead" replacement (gorge applyReplacements, a Draw event whose
// ReplaceWith$ body is exactly a draw of N cards by you): a Draw replacement
// active on the battlefield for your own draws, optionally exempting the first
// card of the draw step (NotFirstCardInDrawStep). It also lowers a Draw
// replacement whose ReplaceWith$ body is any program the kernel can run at a
// draw (ReplDrawProgram: the draw's card is replaced by the program, run with
// the draw's player as ReplacedPlayer), the bodyless Prevent$ skip, and either
// shape's Optional$ True form (the draw pipeline poses gorge's apply / decline
// election of the draw's player: kop.ReplDrawOptional, repl_drawmay.go).
// GenericChoice bodies and every other Draw shape refuse.
func (lc *lowerCtx) drawRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ActiveZones") != "Battlefield" {
		return refuse("repl-hard: Draw replacement")
	}
	// AI hints on the line (Obstinate Familiar): no rules read.
	rp.consume("AICheckSVar", "AISVarCompare")
	var bits int
	switch rp.str("ValidPlayer") {
	case "You":
	case "Opponent":
		bits |= kop.ReplDrawOpponent
	case "Player":
		bits |= kop.ReplDrawAnyPlayer
	default:
		return refuse("repl-hard: Draw replacement")
	}
	if w != nil && w.API == "GenericChoice" {
		// the player is asked which kind of card to dig for after the may:
		// the corpus's choice bodies all move library cards (a dig), so the
		// election stays refused until the movement is proven (ticket 02)
		return refuse("repl-hard: Draw replacement with an optional or choice body")
	}
	if rp.is("Optional", "True") {
		// the player is asked whether to replace the draw (gorge's
		// replChoiceDraw, ReplDrawOptional): the body applies on accept, the
		// draw goes off unasked on decline (repl_drawmay.go)
		bits |= kop.ReplDrawOptional
	}
	if rp.is("NotFirstCardInDrawStep", "True") {
		bits |= kop.ReplDrawNotFirst
	}
	// ValidCause$ <stack spec>: the line replaces only the draws its cause
	// admits (gorge engine/rules/trigger_cause.go drawCauseAdmits, read at
	// repl_drawprogram.go drawCauseCycling). The corpus's one Draw carrier is
	// Unpredictable Cyclone's Activated.Cycling+nonLand; any other spec --
	// another base kind, another keyword, another card qualifier -- is one the
	// kernel cannot evaluate, so it fails closed rather than lowering the line
	// unscoped (which would replace every draw the controller makes).
	if v, ok := rp.get("ValidCause"); ok {
		if strings.TrimSpace(v) != "Activated.Cycling+nonLand" {
			return refuse("repl-hard: Draw replacement ValidCause$ %q", v)
		}
		bits |= kop.ReplDrawCauseCyclingNonLand
	}
	if w != nil && w.API == "WinsGame" {
		// Laboratory Maniac, Jace, Wielder of Mysteries: gorge does apply
		// the line to an attempted draw from an empty library (CR 614.11),
		// but the kernel's win arm beside the state-based loss pass is a
		// separate slice; until it lands the line stays refused.
		return refuse("repl-hard: Draw replacement WinsGame")
	}
	// The line's own gates. A bodyless Prevent$ is the skip kind; a line
	// with neither a body nor Prevent$ (or with both) is not a shape the
	// kernel reads.
	prevent := rp.is("Prevent", "True")
	switch {
	case prevent && w != nil, !prevent && w == nil:
		return refuse("repl-hard: Draw replacement")
	case prevent:
		bits |= kop.ReplDrawSkip
	}
	if w != nil && w.API == "Draw" {
		// A "draw N instead" body does not meet an attempted draw from an
		// empty library (gorge emptyDrawReplacement): mark the program so the
		// kernel's empty-library scan skips it.
		bits |= kop.ReplDrawBodyDraw
	}
	if err := lc.drawReplCond(rp, rl); err != nil {
		return err
	}
	// The plain "draw N cards instead" body keeps its own kind (and its old
	// acceptance; a chained sub-ability on the body was read as nothing
	// there). The program path is for every other body.
	if w != nil && withIs(w, "Draw", map[string]string{"Defined": "You", "NumCards": w.Params["NumCards"]}) &&
		bits&(kop.ReplDrawOpponent|kop.ReplDrawAnyPlayer|kop.ReplDrawOptional|kop.ReplDrawCauseCyclingNonLand) == 0 {
		// ReplDrawMore is the controller's own draw only (drawMore scans
		// ctrlM[p]): a line that names an opponent or any player must keep
		// its player bits and take the program path instead of silently
		// becoming the controller's replacement. Its walk consults no cause
		// either, so a ValidCause$-gated line takes the program path too
		// (repl_drawprogram.go drawCauseCycling) rather than lose the gate.
		if n, err := strconv.Atoi(w.Params["NumCards"]); err == nil && n >= 2 && n <= 8 {
			rl.Kind, rl.N, rl.Bits = "DrawMore", cst(n), bits&kop.ReplDrawNotFirst
			return nil
		}
	}
	if w == nil {
		rl.Kind, rl.Bits = "DrawProgram", bits
		return nil
	}
	// The body is a program of ordinary effects, run at the draw. Every ask
	// it poses is the kernel's ask tape (q.v. repl_drawprogram.go); a body
	// the lowerer cannot represent keeps the card refused.
	ab := &lAbility{Kind: akLatent}
	saved := lc.drawReplBody
	lc.drawReplBody = true
	err := lc.withAbility(ab, func() error { return lc.chain(w, nil) })
	lc.drawReplBody = saved
	if err != nil {
		if lc.p != nil && lc.p.allBlockers {
			return err
		}
		return refuse("repl-hard: Draw replacement")
	}
	if len(ab.Prog) == 0 {
		return refuse("repl-hard: Draw replacement")
	}
	for i := range ab.Prog {
		// This slice's bodies: a draw, a life change, a token or a counter.
		// A body that moves library or graveyard cards (a dig, a ChooseMove,
		// a Play) keeps the old refusal until its own slice proves the
		// kernel's movement against gorge's (repl_drawprogram.go).
		switch ab.Prog[i].Op {
		case "Draw", "LoseLife", "GainLife", "CreateToken", "CreateTokenEx", "PutCounter":
		default:
			return refuse("repl-hard: Draw replacement")
		}
	}
	if len(ab.Prog) == 1 && ab.Prog[0].Op == "Draw" && bits&(kop.ReplDrawOpponent|kop.ReplDrawAnyPlayer) == 0 {
		// "draw N cards instead" is ReplDrawMore's body, not a program; a
		// lone self Draw program would also catch cards whose other
		// replacement lines this slice has not proven (Phial of Galadriel).
		return refuse("repl-hard: Draw replacement")
	}
	rl.Kind, rl.Bits, rl.Prog = "DrawProgram", bits, ab.Prog
	return nil
}

// drawReplCond reads the R: line's own replacement gates into rl.Cond:
// Hellbent$ True (the controller has no cards in hand) and IsPresent$ (a
// battlefield count, or the library's emptiness when PresentZone$ Library is
// named). PresentCompare$ EQ0 over the library is Out of the Tombs' "while
// your library has no cards in it".
func (lc *lowerCtx) drawReplCond(rp *params, rl *lRepl) error {
	if v, ok := rp.get("Hellbent"); ok {
		if strings.TrimSpace(v) != "True" {
			return refuse("repl-hard: Draw replacement Hellbent$ %q", v)
		}
		if rl.Cond.Kind != cNone {
			return refuse("two replacement conditions")
		}
		rl.Cond = lCond{Kind: cValue, A: lValue{Kind: vHandSizeOf, Ref: lRef{Kind: rYou}}, Op: opEQ, B: cst(0)}
	}
	ip, ok := rp.get("IsPresent")
	if !ok {
		return nil
	}
	zone := strings.TrimSpace(rp.str("PresentZone"))
	cmp, hasCmp := rp.get("PresentCompare")
	if zone == "Library" {
		// "while your library has no cards in it" (Ormos, Archive Keeper;
		// Living Conundrum; Out of the Tombs): the kernel's empty-library
		// draw scan meets this line (drawExcl, CR 614.11). The owned-cards
		// count over an empty library is the controller's library size; any
		// other compare literal keeps the refusal.
		if !strings.EqualFold(strings.TrimSpace(ip), "Card.YouOwn") || !strings.EqualFold(strings.TrimSpace(cmp), "EQ0") {
			return refuse("repl-hard: Draw replacement IsPresent$ %q PresentZone$ Library PresentCompare$ %q", ip, cmp)
		}
		if rl.Cond.Kind != cNone {
			return refuse("two replacement conditions")
		}
		rl.Cond = lCond{Kind: cValue, A: lValue{Kind: vLibrarySizeOf, Ref: lRef{Kind: rYou}}, Op: opEQ, B: cst(0)}
		return nil
	}
	if zone != "" || hasCmp {
		return refuse("repl-hard: Draw replacement IsPresent$ with PresentZone$ %q", zone)
	}
	if rl.Cond.Kind != cNone {
		return refuse("two replacement conditions")
	}
	f, err := lc.filter(ip)
	if err != nil {
		return err
	}
	rl.Cond = lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
	return nil
}
