package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The untap event's shaping, in one file: the R:Event$ Untap replacements
// ("this permanent doesn't untap during its controller's untap step", the
// CantHappen layer, 154 of the corpus's 156 lines; and the two ReplaceWith$
// reshapes) and the stat:UntapOtherPlayer static ("untap CARDNAME during each
// other player's untap step", Endbringer). Both apply to exactly one event
// site -- the turn-based untap step in Engine.beginTurn -- so beginTurn owns
// the single call and there is no second caller to keep in sync.
//
// The CantHappen replacement deliberately does NOT gate an ability's untap
// (effUntap): Forge scripts the paid untap of a Mana Vault / Basalt Monolith
// as an ordinary AB$ Untap and the card's own oracle ("you may pay {4}. If
// you do, untap this artifact") only works if the payment untaps. The
// R:Event$ Untap replacement's ValidStepTurnToController$ You gate is the
// script's own statement of "during the untap step of its controller's turn",
// and the whole point of that gate is the AUTOMATIC untap of the untap step;
// an ability's untap on any step is a deliberate rules action, not the
// turn-based action the replacement replaces.

// untapReplacementSources walks every object a permanent's R: line may live
// on (and, for the statics, every object a stat:UntapOtherPlayer static may
// live on), in the deterministic scan order (seats in AliveFrom(0) order,
// each seat's battlefield zone then its command zone): the corpus's untap
// replacements are ActiveZones$ Battlefield and ActiveZones$ Command
// (Edge of Malacol), the corpus's command-zone plane (Horizon Boughs) carries
// its untap static in the command zone, and a replacement with no
// ActiveZones$ is active from anywhere -- the same convention
// rules/replacement.go's MoveZone matcher established.
func (e *Engine) untapReplacementSources(each func(id state.ObjID)) {
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			each(id)
		}
		for _, id := range e.G.Zone(state.ZCommand, p) {
			each(id)
		}
	}
}

// untapReplacementFor finds the R:Event$ Untap replacement that applies to
// the untap of subject during the CURRENT step's turn. ok is false when none
// matches, in which case the untap happens as usual. When several match, the
// first WITH a ReplaceWith$ body wins if there is one (an order choice among
// competing untap reshapes is a CR 616.1 refinement this build does not
// pose -- the corpus never competes two untap replacements on one subject --
// and the scan order is the same deterministic one every other replacement
// collection uses); otherwise the first CantHappen match.
func (e *Engine) untapReplacementFor(subject state.ObjID) (replMatch, bool) {
	var with, plain *replMatch
	e.untapReplacementSources(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			return
		}
		for i := range f.Repls {
			r := &f.Repls[i]
			if r.Event != "Untap" || !e.untapReplacementMatches(r, id, subject) {
				continue
			}
			m := replMatch{id: id, repl: r}
			switch {
			case r.With != nil && with == nil:
				with = &m
			case plain == nil:
				plain = &m
			}
		}
	})
	if with != nil {
		return *with, true
	}
	if plain != nil {
		return *plain, true
	}
	return replMatch{}, false
}

// untapReplacementMatches is the R:Event$ Untap matcher. The Untap event
// carries only its subject, so the step-side gates read the live engine:
//
//   - ActiveZones$: the replacement's owner must currently be in a declared
//     zone (CR 611.3b; the command-zone Edge of Malacol shape is the reason
//     the scan walks the command zone).
//   - ValidStepTurnToController$: the CURRENT turn's controller -- the player
//     whose untap step is running -- must match the spec relative to the
//     replacement's controller ("You": your own untap step). This is what
//     makes the artifact's CantHappen read "during YOUR untap step" and what
//     spares the paid untap during an opponent's turn (Basalt Monolith's
//     {3} on an opponent's end step untaps, exactly the AILogic$ AtOppEOT
//     shape its script names).
//   - ValidCard$: the SUBJECT matches, with the replacement's owner as the
//     filter's source (so Card.Self is the owner -- the common "this
//     artifact doesn't untap" -- and Creature.EnchantedBy the aura's
//     "enchanted creature doesn't untap", which the implemented
//     EnchantedBy/AttachedBy/EquippedBy predicates resolve).
func (e *Engine) untapReplacementMatches(r *cards.Repl, source, subject state.ObjID) bool {
	if az, ok := r.Params["ActiveZones"]; ok {
		o := e.G.Obj(source)
		if o == nil || !zoneSpecContains(az, o.Zone) {
			return false
		}
	}
	if vp, ok := r.Params["ValidStepTurnToController"]; ok {
		ctrl := e.controllerOf(source)
		if !effects.MatchesPlayerSpec(e.G, vp, e.G.Active, ctrl) {
			return false
		}
	}
	if v, ok := r.Params["ValidCard"]; ok && !effects.MatchesSpecFrom(e.G, v, subject, e.controllerOf(source), source) {
		return false
	}
	return true
}

// untapTurnPermanent untaps one permanent as part of the untap step's
// turn-based action (CR 302.2/305.2/CR 502.2), under the R:Event$ Untap
// replacements. A CantHappen match suppresses the untap and records a Note
// instead of a silent nothing -- the log must show WHY the permanent stayed
// tapped -- and a ReplaceWith$ match runs that body instead (the untap does
// not happen: both corpus bodies put counters on the subject instead).
func (e *Engine) untapTurnPermanent(subject state.ObjID) {
	o := e.G.Obj(subject)
	if o == nil || !o.Tapped {
		return
	}
	m, ok := e.untapReplacementFor(subject)
	if !ok {
		e.emit(events.Event{Kind: events.Untap, Obj: subject})
		return
	}
	name := "a permanent"
	if f := o.Face(); f != nil {
		name = f.Name
	}
	if m.repl.With != nil {
		e.emit(events.Event{Kind: events.Note, Obj: subject,
			Text: name + " does not untap: replaced"})
		e.runReplaceWith(e.replCtx(m, events.Event{Kind: events.Untap, Obj: subject}), subject, m.repl.With)
		return
	}
	by := "a permanent"
	if so := e.G.Obj(m.id); so != nil && so.Face() != nil {
		by = so.Face().Name
	}
	e.emit(events.Event{Kind: events.Note, Obj: subject,
		Text: name + " does not untap during its controller's untap step (" + by + ")"})
}

// untapOtherStaticsMatch reports whether the permanent id untaps during a
// FOREIGN seat's untap step under some stat:UntapOtherPlayer static: every
// living seat's battlefield AND command zone is scanned for the static (the
// corpus's 15 lines include the Drumbellower "each creature you control"
// shape and the Horizon Boughs command-zone plane "all permanents untap
// during each player's untap step"), each static's ValidCard$ (default
// Card.Self, the primitive's own name) is matched with the static's owner as
// the filter's source, so YouCtrl and Self resolve against the controller
// whose other-players' step this is. A static whose ValidCard$ this build
// cannot evaluate fails closed, like every other restriction-class reader.
func (e *Engine) untapOtherStaticsMatch(subject state.ObjID) bool {
	matched := false
	e.untapReplacementSources(func(id state.ObjID) {
		if matched {
			return
		}
		o := e.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			return
		}
		for _, st := range f.Statics {
			if st.Mode != "UntapOtherPlayer" {
				continue
			}
			spec, ok := st.Params["ValidCard"]
			if !ok || spec == "" {
				spec = "Card.Self"
			}
			if effects.MatchesSpecFrom(e.G, spec, subject, e.controllerOf(id), id) {
				matched = true
				return
			}
		}
	})
	return matched
}
