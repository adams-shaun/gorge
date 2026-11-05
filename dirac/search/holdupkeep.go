package search

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Options.HoldUpkeep is a :holdupkeep rule first written for gorge's
// azmcts, on dirac. A measured network-guided search cast 38% of
// its instant-speed spells in its own upkeep or draw step with an empty
// stack, mostly at near-ties over pass. No sorcery-speed cast is legal
// there, so each is an instant-speed spell's (an instant, a flash card, a
// flashback cast), and the same cast in the first main phase has the same
// lands untapped and the same response windows while it has seen the draw
// (an upkeep cast has not) and the land drop (neither has). 038 measured
// the rule null on gorge (50.2%, 48.4-52.0, over 500 pairs; 4,147 roots
// changed, casts per turn 0.297 -> 0.294); 039 keeps it as an arm.
//
// The rule, as the fork's:
//
//   - The window: a priority decision of the active player in its own
//     upkeep or draw step with an empty stack (holdOn). Any step with
//     something on the stack, any other step and the other player's turn
//     are unaffected.
//   - In the window every candidate that casts a spell is dropped before
//     the candidates are capped (priorityCands): every AkCast, the kicked
//     and flashback modes included -- dirac's FDN has no "cast"-kind
//     special action (the fork's Foretell, Suspend and Plot carve-out has
//     nothing to keep here), and its mana is autopaid, so the fork's
//     payment and legacy casts are the one AkCast. Pass and activations
//     stay (land plays are never legal there). A card named in
//     HoldUpkeepExempt is never held.
//   - A root left with fewer than two candidates is skipped and the bot's
//     answer played, there a pass. A bot answer that is itself a held cast
//     is outside the candidates, so the decision is not searched, and pass
//     is played in its place (heldAnswer): at the root on every path that
//     plays the bot's answer (Stats.HeldUpkeepBot), and at the in-walk
//     decisions the rule applies to, whether or not they are points.
//   - It applies at the root and at the searching seat's in-walk
//     decisions; at the opponent's (its own upkeep and draw) only under
//     OpponentNodes with OpponentMirror (the fork's oppHold), never to an
//     unmirrored opponent or one whose decisions are not in the tree.
//   - Counters (the fork's): Stats.HeldUpkeep, HeldUpkeepBot,
//     HeldUpkeepWalk, HeldUpkeepOpp.
//
// The fork's other fixes have no dirac counterpart: a held payment cast
// still introducing its card to the observer (dirac keys by object or
// card, no collector numbering), the prior-only seat (there is none), and
// the root macros (dirac's priority candidates are single actions).

// holdOn reports whether Options.HoldUpkeep's window is open at decision d
// of s for this searcher (the caller checks HoldUpkeep): a priority
// decision of the active player in its upkeep or draw step with an empty
// stack, the searching seat's own or, under OpponentNodes with
// OpponentMirror, the opponent's.
func (sr *Searcher) holdOn(s *dirac.State, d *dirac.Decision) bool {
	if d.Kind != dirac.DkPriority || d.Player != s.ActivePlayer() || s.StackLen() != 0 {
		return false
	}
	if d.Player != sr.seat && !(sr.Opts.OpponentNodes && sr.Opts.OpponentMirror) {
		return false
	}
	st := s.StepNow()
	return st == kop.StepUpkeep || st == kop.StepDraw
}

// holdsCast reports whether option a is a held cast: an AkCast of a card
// HoldUpkeepExempt does not name (by its table name).
func (sr *Searcher) holdsCast(s *dirac.State, a dirac.Action) bool {
	if a.Kind() != dirac.AkCast {
		return false
	}
	ex := &sr.Opts.HoldUpkeepExempt
	if ex[0] == "" {
		return true
	}
	name := s.Table().Cards[s.ObjCard(dirac.Obj(a.Source()))].Name
	for _, n := range ex {
		if n == "" {
			break
		}
		if n == name {
			return false
		}
	}
	return true
}

// heldAnswer is the option the rule plays in place of the bot's answer bot
// at decision d of s: d's pass where the window is open and bot is a held
// cast, -1 (bot stands) anywhere else or with no pass offered.
func (sr *Searcher) heldAnswer(s *dirac.State, d *dirac.Decision, bot int) int {
	if !sr.holdOn(s, d) || !sr.holdsCast(s, d.Options[bot]) {
		return -1
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPass {
			return i
		}
	}
	return -1
}

// holdWalk is the rule at an in-walk decision d of s (advance, HoldUpkeep
// on) after its enumeration: point says whether d became a point, drops is
// holdDrops before it. A decision that is no point plays pass in place of a
// held bot cast (heldAnswer); either change counts the decision in
// HeldUpkeepWalk (the searching seat's) or HeldUpkeepOpp. It returns the
// answer to play.
func (sr *Searcher) holdWalk(s *dirac.State, d *dirac.Decision, bot int, point bool, drops int) int {
	if !point {
		if p := sr.heldAnswer(s, d, bot); p >= 0 {
			bot = p
			sr.holdDrops++
		}
	}
	if sr.holdDrops != drops {
		if d.Player == sr.seat {
			sr.St.HeldUpkeepWalk++
		} else {
			sr.St.HeldUpkeepOpp++
		}
	}
	return bot
}
