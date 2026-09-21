package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Pair", effPair) }

// effPair implements Soulbond's optional pairing choice (CR 702.103). The
// keyword expansion supplies both trigger cases; this common body only pairs
// an unpaired Soulbond creature with another unpaired creature controlled by
// the same player. The selected partner is recorded by the decision intent and
// the reciprocal mutation by one Pair event, so replay never depends on board
// scan order.
func effPair(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	src := g.Obj(c.Source)
	if src == nil || !isBattlefieldCreature(src) || src.Paired != 0 {
		return
	}
	if c.SoulbondDone {
		partner := g.Obj(c.SoulbondPartner)
		c.SoulbondDone = false
		c.SoulbondPartner = 0
		if soulbondPartner(src, partner, c.Controller) {
			h.Emit(events.Event{Kind: events.Pair, Obj: c.Source, IDs: []state.ObjID{partner.ID}})
		}
		return
	}
	// RestrictToRemembered$ True (the "another creature enters" half of
	// Soulbond's expansion, cards/keywords.go's k+"#other" trigger) narrows
	// the candidate scan to the specific creature that triggered THIS
	// resolution -- CR 702.103a's "you may pair this creature with that
	// creature", not any other unpaired creature the controller happens to
	// have on the battlefield. Ctx.Remembered already carries the triggering
	// entrant (checkTriggers' triggerRemembered, threaded through every
	// ChangesZone trigger). The keyword's own first-entry trigger (#self)
	// carries no such restriction and keeps the broad scan.
	var restrictTo state.ObjID
	if strings.EqualFold(sa.Params["RestrictToRemembered"], "True") {
		for _, t := range c.Remembered {
			if !t.IsPlayer && t.Obj != 0 {
				restrictTo = t.Obj
				break
			}
		}
		if restrictTo == 0 {
			// No remembered entrant to restrict to: nothing to offer, rather
			// than falling back to the broad (wrong) scan.
			return
		}
	}
	options := make([]decision.Option, 0)
	for _, id := range g.Zone(state.ZBattlefield, c.Controller) {
		if restrictTo != 0 && id != restrictTo {
			continue
		}
		o := g.Obj(id)
		if !soulbondPartner(src, o, c.Controller) {
			continue
		}
		options = append(options, decision.Option{Index: len(options), Kind: "pair", Label: objName(g, id), Obj: id, Player: c.Controller})
	}
	if len(options) == 0 {
		return
	}
	d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose, Min: 0, Max: 1,
		Source: c.Source, ResumeKind: "soulbond", ResumeSA: sa,
		Prompt:  "You may pair " + objName(g, c.Source) + " with another unpaired creature",
		Options: options}
	if h.Ask(d) {
		return
	}
	// A host without decisions takes the legal optional decline.
}

func soulbondPartner(src, partner *state.Object, controller state.PlayerID) bool {
	return partner != nil && partner.ID != src.ID && partner.Controller == controller &&
		partner.Paired == 0 && isBattlefieldCreature(partner)
}

func isBattlefieldCreature(o *state.Object) bool {
	return o != nil && o.Zone == state.ZBattlefield && o.EffectiveIsCreature()
}
