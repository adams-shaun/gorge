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
// "You"/"Opponent" pair in context.go). Every other TokenOwner$ form the
// corpus uses (a fidelity gap this task does not close) still falls back to
// the controller rather than doing nothing, but now says so: a Note names
// the unrecognised value, so the gap is visible rather than silently
// papered over the way an unqualified fallback would be.
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
	switch v := sa.Params["TokenOwner"]; v {
	case "", "You":
		// The default: the controller, already set above.
	case "Opponent":
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				owner = p
				break
			}
		}
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "unrecognized TokenOwner " + v + ", defaulting to the controller"})
	}
	remember := sa.Params["RememberTokens"] == "True"
	// AttachedTo$ names the permanent the token enters attached to (the Wicked
	// Role of Charming Scoundrel's ETB, 50+ corpus lines): the value is a
	// Defined$-grammar selector, resolved with the ordinary resolver against
	// a shallow SA that carries it in Defined$, so every corpus spelling
	// (Targeted, Self, Remembered, ChosenCard, ...) works without a second
	// resolver. Each minted token is attached to the FIRST resolved object
	// target; an object that has left play by resolution time attaches
	// nothing (the token simply enters unattached, the Aura's unattached
	// state).
	attachedTo := strings.TrimSpace(sa.Params["AttachedTo"])
	var attachTo state.ObjID
	if attachedTo != "" {
		sub := *sa
		sub.Params = map[string]string{"Defined": attachedTo}
		for _, t := range Defined(h, c, &sub) {
			if !t.IsPlayer {
				attachTo = t.Obj
				break
			}
		}
	}

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
			// want is the ID the new object will get if TokenCreate's own
			// Apply case actually mints one (state.Game.AddObject assigns
			// NextID, then increments it) -- a direct, positive identity
			// check, rather than inferring a mint happened from g.Objs
			// having grown by watching its length before and after.
			want := g.NextID
			h.Emit(events.Event{Kind: events.TokenCreate, Player: owner, Text: key})
			if remember && g.Obj(want) != nil {
				c.Remembered = append(c.Remembered, state.Target{Obj: want})
				eventRemember(h, c, want)
			}
			if attachTo != 0 && g.Obj(want) != nil && g.Obj(attachTo) != nil {
				h.Emit(events.Event{Kind: events.Attach, Obj: want, IDs: []state.ObjID{attachTo}})
			}
			if strings.EqualFold(strings.TrimSpace(sa.Params["ImprintTokens"]), "True") && g.Obj(want) != nil {
				// ImprintTokens$ True (Ugin, the Ineffable's [+1] spirit token):
				// the created token is IMPRINTED with the cards the resolution
				// remembered -- the face-down-exiled card the preceding Dig
				// captured -- so Card.IsImprinted matches the token exactly as
				// Forge's imprintedCards association would. An empty remembered
				// set records nothing: an imprint of nothing is not an imprint.
				ids := make([]state.ObjID, 0, len(c.Remembered))
				for _, t := range c.Remembered {
					if !t.IsPlayer && t.Obj != 0 {
						ids = append(ids, t.Obj)
					}
				}
				if len(ids) > 0 {
					h.Emit(events.Event{Kind: events.Imprint, Obj: want, IDs: ids})
				}
			}
		}
	}
}
