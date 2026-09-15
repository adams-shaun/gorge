package effects

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// emitLook is the ONE construction point for a private look: an effect (or
// engine flow) that shows hidden-zone cards to a SUBSET of the table rather
// than revealing them to everyone. Every looker-scoped effect in the engine
// routes through here — the round-2 review of the Gitaxian Probe merge found
// one of them (RevealHand's Look$ arm) emitting its look as a PUBLIC Note,
// handing the target's whole hand to every seat and spectator, so the class
// was fixed by funnelling all of them through this helper:
//
//   - effReveal's Look$ arm (RevealHand/Reveal/PeekAndReveal with Look$ True;
//     28 corpus RevealHand lines — Gitaxian Probe, Glasses of Urza, Slayer's
//     Bounty): the activator looks at the target's hand. CR 701.20e: a card
//     looked at this way is shown only to the player the effect specifies.
//   - effDig's ask-path look (the window the library's owner is choosing
//     from).
//   - effRearrangeTopOfLibrary, effScry and effSurveil (the KArrange looks
//     at the top of the library).
//
// It emits ONE Secret Note per looker with Player = that looker, so rule (1)
// of view.RedactEvents scopes the payload (ids and text) to them alone:
// every other seat and a Public spectator get the bare shape (and the
// generic "looks at hidden cards" transcript line), and an Omniscient
// spectator keeps the payload only for a HAND look (From == ZHand — their
// visibility already includes every hand, per Ruling FL-9's library-order
// carve-out not applying to one). The looked-at zone rides From so the
// looker's transcript line can name it even after the cards have moved on;
// existing library-top callers pass ZLibrary, which is also the zero value
// their events always carried, so their encoded bytes are unchanged.
//
// text is the clause after the looker's name on their own copy's transcript
// line; the hand look passes "" so view.Describe's ids branch renders the
// full line (who looked, whose hand, which cards) from the payload itself.
func emitLook(h Host, lookers []state.PlayerID, from state.Zone, ids []state.ObjID, text string) {
	for _, looker := range lookers {
		h.Emit(events.Event{Kind: events.Note, Player: looker, From: from,
			IDs: ids, Text: text, Secret: true})
	}
}
