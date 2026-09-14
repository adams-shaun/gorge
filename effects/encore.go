// encore.go implements the Encore keyword's resolution (CR 702: "Encore
// <cost> — Exile this card from your graveyard: For each opponent, create a
// token copy that attacks that opponent this turn if able. They gain haste.
// Sacrifice them at the beginning of the next end step. Activate only as a
// sorcery.").
//
// The activated ABILITY itself is not written here: cards/keywords.go
// expands K:Encore:<cost> into an AB$ Encore ability in the graveyard whose
// Cost$ carries the printed cost plus ExileFromGrave<1/CARDNAME> (the card
// exiles itself as the cost's payment, settled by the ordinary cast-flow
// exile stage), ActivationZone$ Graveyard and SorcerySpeed$ True -- so the
// offer gate, the payment machinery and the CR 602.2b flow are the ordinary
// ones and this primitive only resolves the effect.
//
// The one CR clause this build does NOT enforce is "attacks that opponent
// this turn if able": there is no attack-requirement machinery to hang it on
// (recorded in the ticket report's Issues). Everything else is exact: one
// CardToken copy per opponent (a copy of the card object itself, whatever
// zone it resolved from -- the cost exiled it, so exile), haste granted
// until end of turn through the layer system, and one end-step delayed
// sacrifice per token (the __kwEncoreSacrifice builtin SVar,
// cards/link.go).
package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Encore", effEncore) }

func effEncore(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	src := g.Obj(c.Source)
	if src == nil || src.Card == nil {
		return
	}
	// "For each opponent": the opponents of the ACTIVATOR (c.Controller),
	// in AliveFrom's fixed seat order -- never a map, so the token order is
	// replay-stable. The card is in exile (its own cost exiled it), but the
	// copy is created from the object wherever it sits.
	for _, p := range g.AliveFrom(c.Controller) {
		if p == c.Controller {
			continue
		}
		want := g.NextID
		h.Emit(events.Event{Kind: events.CardToken, Obj: c.Source, Player: c.Controller})
		tok := g.Obj(want)
		if tok == nil {
			continue
		}
		// "They gain haste": a layer-6 UntilEOT grant scoped to the token
		// itself (the effPump shape). The token's sacrifice at the next end
		// step lands after cleanup would drop the grant anyway; the turn
		// boundary handles the pathological survivor.
		h.AddContinuous(state.ContinuousEffect{
			Source: want, Affects: "Card.Self", Controller: c.Controller,
			Layer: state.LAbilities, AddKeywords: []string{"Haste"}, UntilEOT: true,
		})
		// "Sacrifice them at the beginning of the next end step": one
		// delayed trigger per token, resolved through the ordinary
		// DelayedPush machinery against the __kwEncoreSacrifice builtin.
		h.Emit(events.Event{Kind: events.DelayedRegister, Obj: want,
			Player: c.Controller, Step: state.StepEnd, Counter: "__kwEncoreSacrifice"})
	}
}
