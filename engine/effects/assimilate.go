package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { Register("Assimilate", effAssimilate) }

// assimilateGrant is what assimilate makes the card: a Borg artifact creature
// that loses its other creature types and keeps every other type it has (Star
// Trek release notes: an assimilated Dryad Arbor is an artifact land creature
// with the Forest and Borg subtypes).
var assimilateGrant = staticGrant{types: []string{"Artifact", "Creature", "Borg"}, removeCreatureTypes: true}

// effAssimilate is the Star Trek keyword action: you gain control of a creature,
// or put a creature card onto the battlefield under your control, with a +1/+1
// counter, and it becomes a Borg artifact creature. The characteristics apply as
// a card enters, so a "whenever an artifact enters" trigger sees one: the grant
// is registered ahead of the move, anchored to the incarnation the entry begins,
// and ends when that permanent leaves.
func effAssimilate(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	targets := Defined(h, c, sa)
	if objectTargetCount(targets) > 1 {
		h.BeginZoneBatch()
		defer h.EndZoneBatch()
	}
	for _, t := range targets {
		o := g.Obj(t.Obj)
		if t.IsPlayer || o == nil || tokenCannotChangeZone(o) {
			continue
		}
		if o.Zone == state.ZBattlefield {
			registerStaticEffectGrant(h, c, o.ID, "Card.Self", assimilateGrant, staticGrantLifetime{})
			if o.Controller != c.Controller {
				h.Emit(events.Event{Kind: events.ControlChange, Obj: o.ID, Player: c.Controller})
			}
			emitChangeZoneCounters(h, o.ID, "P1P1", 1)
			continue
		}
		registerStaticEffectGrant(h, c, o.ID, "Card.Self", assimilateGrant, staticGrantLifetime{SourceIncarnation: o.Incarnation + 1})
		ev := moveZoneEvent(c, o.ID, o.Zone, state.ZBattlefield)
		entryWithCounters(&ev, state.ZBattlefield, "P1P1", 1)
		emitEntryMove(h, c, ParamText{Text: "True", Present: true}, state.ZBattlefield, ev)
	}
}
