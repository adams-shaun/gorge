package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
)

func init() { Register("Investigate", effInvestigate) }

// clueTokenKey is the corpus token script investigate mints
// (.cards/tokenscripts/c_a_clue_draw.txt: an Artifact Clue with the "{2},
// Sacrifice this token: Draw a card" activated ability).
const clueTokenKey = "c_a_clue_draw"

// effInvestigate implements the Investigate primitive (CR 701.36a; 142
// corpus files -- 120 DB$, 16 AB$, 9 SP$): create Num$ Clue tokens for each
// investigating player.
//
// Count: Num$ (default 1) resolves through the shared Num evaluator -- a
// literal, an SVar name, an inline Count$ body or the bare X of a
// Count$xPaid cast; a present-but-unresolvable value degrades to 0 (the
// card resolves to nothing rather than doing something arbitrary), exactly
// the Num contract.
//
// Players: the investigating player(s) resolve through actingPlayers -- the
// shared "Defined$ else ValidTgts$-targeted else the resolving controller"
// selector every player-acting primitive uses. Defined$ Player.withMostType*
// and the Targeted/Remembered referents resolve inside Defined().
//
// Each Clue is one real TokenCreate event (owner = the investigating
// player), so the mint rides events.Apply and fires trig:TokenCreated /
// trig:TokenCreatedOnce (Mirkwood Bats et al.) exactly like effToken's
// mints, and each investigate also emits one events.Investigate marker
// (trig:Investigated matches it — Erdwal Illuminator; a plain Clue-token
// creation never fires an investigate trigger). No direct state.Game writes.
//
// Optional$ True (2 files: will_the_wise, nick_valentine_private_eye) is
// unread: a real mid-resolution yes/no ask needs a rules/resolution.go
// resume arm (the draw_optional precedent), so this takes the documented
// deterministic stand-in -- always investigate, plus ONE loud Note per
// gated resolution naming the unread parameter. RememberInvestigatingPlayers$
// (1 file, will_the_wise) is likewise unread and silently so.
func effInvestigate(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	n := Num(h, c, sa, "Num", 1)
	if n <= 0 {
		return
	}
	def, ok := g.Tokens[clueTokenKey]
	if !ok || def == nil {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "Investigate: unknown token script " + clueTokenKey})
		return
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
		// Deterministic stand-in: the may-investigate election is always
		// "investigate". One Note per gated resolution, not per token.
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
			Text: "Investigate: Optional$ True not asked; investigates (stand-in)"})
	}
	for _, t := range actingPlayers(h, c, sa) {
		p := t
		for i := int32(0); i < n; i++ {
			h.Emit(events.Event{Kind: events.TokenCreate, Player: p, Text: clueTokenKey})
			// One investigate record per created Clue (CR 701.36a: each
			// investigate is one Clue token, so Num$ 2 is two investigates).
			// The marker is what trig:Investigated matches; emitting it only
			// here (not on a plain DB$ Token Clue mint) keeps "create a Clue
			// token" from firing "whenever you investigate" triggers.
			h.Emit(events.Event{Kind: events.Investigate, Obj: c.Source, Player: p})
		}
	}
}
