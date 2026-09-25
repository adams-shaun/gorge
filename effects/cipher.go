package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Cipher", effCipher) }

// effCipher implements DB$ Cipher, the body cards/kw_cipher.go expands K:Cipher
// into: "Then you may exile this spell card encoded on a creature you
// control." (CR 702.99a.) It is the reflexive trigger's resolution, fired by
// the spell card's own stack->graveyard move, and it is the ONE home for the
// encode ask and the association -- the combat-damage copy trigger the
// association grants lives rules-side (checkCipherTriggers), because only
// rules can queue a trigger.
//
// The offer is a single mid-resolution KModes ask over the resolving
// controller's creatures: Min 0 (an empty answer declines -- "you may"), Max
// 1 (exactly one creature is encoded). The choice is carried across the
// suspension in Ctx.CipherDone/CipherPick (the demonstrate transport shape),
// and a host that cannot ask takes the deterministic decline -- the R-9
// no-host contract, matching every other optional keyword the engine poses.
//
// The performing half moves the card to exile and writes the association as
// an ordinary events.Imprint with the "encoded" Text discriminator onto the
// chosen creature (state.Object.EncodedCards), so replay rebuilds it and
// events.Move prunes/clears it when either side leaves its zone.
func effCipher(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	// The encoded card is the resolving trigger's source (ValidCard$
	// Card.Self). An absent or departed object is a no-op -- the totality
	// stance every effect takes.
	card := c.Source
	co := g.Obj(card)
	if co == nil {
		return
	}
	controller := c.Controller
	if int(controller) < len(g.Players) && g.Players[controller].Lost {
		return
	}

	answered := c.CipherDone
	pick := c.CipherPick
	// fx42 scoping: consume and clear the answered transport before anything
	// below, so a nested Cipher never inherits this answer.
	c.CipherDone = false
	c.CipherPick = nil

	if answered {
		// The answer is settled. An empty pick is the decline; a pick naming
		// an object that has since left the battlefield (or is not a legal
		// creature any more) is a stale answer and encodes nothing.
		if len(pick) == 0 {
			return
		}
		creature := pick[0].Obj
		target := g.Obj(creature)
		if co.Zone != state.ZGraveyard || target == nil || target.Zone != state.ZBattlefield ||
			!MatchesSpecCtx(g, "Creature.YouCtrl", creature, c.SpecContext(controller)) {
			h.Emit(events.Event{Kind: events.Note, Obj: card,
				Text: "cipher encode found its chosen creature no longer on the battlefield"})
			return
		}
		// "Encoded" means exiled AND associated (CR 702.99a). Move the card
		// to exile from wherever it currently sits, then write the link. The
		// Imprint event's IDs[0] is the encoded card; Apply appends it to the
		// creature's EncodedCards.
		h.Emit(events.Event{Kind: events.MoveZone, Obj: card, From: state.ZGraveyard, To: state.ZExile,
			Text: "encoded"})
		if co = g.Obj(card); co == nil || co.Zone != state.ZExile {
			return // a replacement prevented the exile; nothing was encoded.
		}
		h.Emit(events.Event{Kind: events.Imprint, Obj: creature, IDs: []state.ObjID{card},
			Text: "encoded"})
		return
	}

	// The card must still be somewhere we can exile it from -- normally the
	// graveyard it just resolved into. A card that has already left (a
	// replacement exiled it instead) is nothing to encode.
	if co.Zone != state.ZGraveyard {
		return
	}
	// Candidates: the resolving controller's creatures, in deterministic
	// battlefield-zone order (the zone list is insertion-ordered).
	sc := c.SpecContext(controller)
	var candidates []state.ObjID
	for _, id := range g.Zone(state.ZBattlefield, controller) {
		if MatchesSpecCtx(g, "Creature.YouCtrl", id, sc) {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		// No legal host: make the no-ask path replay-visible so it cannot
		// pass a negative test simply because the handler was unregistered.
		h.Emit(events.Event{Kind: events.Note, Obj: card,
			Text: "cipher encode has no creature you control"})
		return
	}

	// The optional encode: a single-pick KModes over the candidates, Min 0
	// (decline). Naming the card keeps the prompt readable.
	name := ""
	if f := co.Face(); f != nil {
		name = f.Name
	}
	d := &decision.Decision{Player: controller, Kind: decision.KModes,
		Min: 0, Max: 1, Source: card, ResumeKind: "cipher", ResumeSA: sa,
		Prompt: "Encode " + name + " on a creature you control?", ResumeRemembered: copyTargets(c.Remembered)}
	for _, id := range candidates {
		label := "Encode"
		if o := g.Obj(id); o != nil && o.Face() != nil {
			label = "Encode on " + o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mode",
			Label: label, Obj: id, Player: controller})
	}
	if h.Ask(d) {
		return // suspended; rules' "cipher" arm re-enters this walk with the answer.
	}
	// No host to ask (the R-9 fuzz/test contract): the deterministic decline.
	h.Emit(events.Event{Kind: events.Note, Obj: card,
		Text: "cipher encode resolved as the decline (no engine host to ask)"})
}
