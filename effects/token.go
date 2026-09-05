package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Token", effToken) }

// effToken creates TokenAmount$ tokens of each TokenScript$ (a comma-
// separated list of Game.Tokens stems) for TokenOwner$ (the controller by
// default; only "Opponent" is resolved specially, matching Defined's own
// "You"/"Opponent" pair in context.go -- every other TokenOwner$ form the
// corpus uses falls back to the controller rather than doing nothing).
//
// Every token is its own TokenCreate event, in the order this loop visits
// them (outer: TokenScript$ stems left to right; inner: TokenAmount$ copies
// in index order) -- never a map-range order, so replay reproduces the same
// objects with the same IDs. events.Apply's TokenCreate case is what
// actually mints the object from Game.Tokens; this function only ever
// proposes the event through h.Emit, so it stays a pure proposer like every
// other primitive in this package (Ruling: effects never writes state.Game
// directly, only events.Apply does).
//
// An unknown TokenScript$ key is a Note ("unknown token script <key>") and
// creates nothing -- a Link-time diagnostic ought to have already caught a
// script that names a stem outside Game.Tokens, but Resolve's own totality
// stance (never panic on untrusted/unexpected input) applies here too.
//
// RememberTokens$ True hands every object this call actually created to the
// rest of the chain via Ctx.Remembered, which is how Living Weapon's own
// keyword expansion (cards/keywords.go) attaches the Germ it just made: its
// SubAbility is `DB$ Attach | Defined$ Remembered`, and Resolve walks Sub
// with the SAME *Ctx, so appending here is what that Attach later reads.
func effToken(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	n := Num(h, c, sa, "TokenAmount", 1)
	owner := c.Controller
	switch sa.Params["TokenOwner"] {
	case "Opponent":
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				owner = p
				break
			}
		}
	}
	remember := sa.Params["RememberTokens"] == "True"

	for _, key := range strings.Split(sa.Params["TokenScript"], ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := g.Tokens[key]; !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "unknown token script " + key})
			continue
		}
		for i := int32(0); i < n; i++ {
			before := len(g.Objs)
			h.Emit(events.Event{Kind: events.TokenCreate, Player: owner, Text: key})
			if remember && len(g.Objs) > before {
				c.Remembered = append(c.Remembered, state.Target{Obj: g.Objs[len(g.Objs)-1].ID})
			}
		}
	}
}
