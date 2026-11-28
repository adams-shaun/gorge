package effects

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { Register("Myriad", effMyriad) }

// effMyriad implements CR 702.116's Myriad: when a creature with Myriad
// attacks, for each opponent OTHER than the defending player, the controller
// creates a token that is a copy of the attacking creature, tapped and
// attacking that opponent. The trigger (engine/cards/keywords.go) fires on the
// source's own DeclareAttackers event, so the combat is already in progress.
// For EACH eligible opponent, CR 702.109 says the controller MAY create a
// copy. The choices are posed one at a time and resume with a deterministic
// candidate cursor: an answer creates (or declines) precisely that opponent's
// token before asking the next one. This avoids turning several independent
// may choices into a mandatory all-or-none operation.
//
// The copy registers the CR 702.116a / CR 511.2 delayed end-of-combat exile
// ("exile the tokens at end of combat"): one one-shot DelayedRegister per
// mint, the AtEOT$ ExileCombat shape (engine/effects/ateot.go, Geist of Saint
// Traft's Angel token). The registration rides the token itself, so it fires
// at the beginning of this turn's end-of-combat step, goes on the stack and
// gives players priority while the tokens are still on the battlefield; it
// is not a turn-structure sweep.
func effMyriad(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	src := g.Obj(c.Source)
	if src == nil || src.Face() == nil {
		return
	}
	eligible := myriadOpponents(g, c)
	start := 0

	for ; start < len(eligible); start++ {
		q := eligible[start]
		d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
			Min: 1, Max: 1, Source: c.Source, ResumeKind: "myriad", ResumeSA: sa,
			ResumeTarget: start,
			Prompt:       "Myriad: create a copy attacking " + g.SeatFacingName(q) + " or a planeswalker they control?",
			Options:      myriadAskOptions(h, g, c, q)}
		if ans, ok := AskTape(h, d); ok {
			// The resolution kernel's answer in hand: the "myriad" arm's
			// yes creates this opponent's copy, then the walk asks the next
			// opponent, exactly as the re-entry does from ResumeTarget+1.
			if len(ans) == 1 && ans[0].Kind == "yes" {
				// The player option rides the source's Obj, the walker
				// options the walker's: the source is a creature, never one
				// of the walker candidates, so the two never collide.
				battle := ans[0].Obj
				if battle == c.Source {
					battle = 0
				}
				myriadCreate(h, g, c, q, battle)
			}
			continue
		}
		// A host without decisions takes the legal optional decline for this
		// and every remaining opponent; it does not manufacture a token.
		return
	}
}

// myriadAskOptions poses CR 702.116a's choices over one eligible opponent q:
// create a copy attacking the player, one per face-up planeswalker q
// controls, then the decline. Without a planeswalker this is the plain
// two-option ask the keyword has always posed. The walker arm is the same
// CR 508.4 shape Adeline's token ask offers (tokenAttackingRider): the
// controller chooses between the seat and its planeswalkers.
func myriadAskOptions(h Host, g *state.Game, c *Ctx, q state.PlayerID) []decision.Option {
	opts := []decision.Option{{
		Index: 0, Kind: "yes", Label: "Create a copy attacking " + g.SeatFacingName(q),
		Obj: c.Source, Player: c.Controller,
	}}
	for _, id := range g.Zone(state.ZBattlefield, q) {
		o := g.Obj(id)
		if o == nil || o.Controller != q || o.FaceDown ||
			!slices.Contains(h.Chars(id).Types, "Planeswalker") {
			continue
		}
		opts = append(opts, decision.Option{Index: len(opts), Kind: "yes",
			Label: "Create a copy attacking " + objName(g, id), Obj: id, Player: c.Controller})
	}
	opts = append(opts, decision.Option{Index: len(opts), Kind: "no",
		Label: "Don't create a copy", Obj: c.Source, Player: c.Controller})
	return opts
}

// myriadCreate creates the Myriad copy of the source attacking opponent q,
// or, when the ask named one and it is still legal, a planeswalker q
// controls (CR 702.116a).
func myriadCreate(h Host, g *state.Game, c *Ctx, q state.PlayerID, battle state.ObjID) {
	// CR 506.3c / 508.4a-b: the named planeswalker must still be on the
	// battlefield, still a planeswalker and still controlled by q; a choice
	// that no longer holds (a legacy park answer served late) degrades to
	// the player, never to no attack.
	if battle != 0 && !EnteringAttackLegal(h, q, battle) {
		battle = 0
	}
	// Player is the token's controller (the attacking player) and IDs[0]
	// is the opponent it attacks. want is the ID the new copy will get
	// (state.Game.AddObject assigns g.NextID then increments it, the
	// same prediction engine/effects/token.go's TokenCreate loop relies on),
	// captured before the mint so the follow-up MoveZone below names
	// the right object. MyriadCopy only mints the token (in the
	// untracked ZLibrary state AddObject leaves it in); the MoveZone
	// that actually seats it on the battlefield is a genuine
	// ChangesZone-matchable event, so the token's own ETB triggers
	// and every other "a creature enters" trigger observe its entry
	// exactly like an ordinary cast or reanimation (CR 702.109 grants
	// no special exemption from that).
	// The CreateToken replacements size the creation (Doubling
	// Season's "twice that many"): each copy is its own mint and entry.
	copies := proposeCopyTokens(h, c.Controller, c.Source, 1)
	for k := int32(0); k < copies; k++ {
		want := g.NextID
		ids := []state.ObjID{state.ObjID(q)}
		if battle != 0 {
			ids = append(ids, battle)
		}
		h.Emit(events.Event{Kind: events.MyriadCopy, Obj: c.Source, Player: c.Controller, IDs: ids})
		if g.Obj(want) != nil {
			h.Emit(events.Event{Kind: events.MoveZone, Obj: want, From: state.ZLibrary, To: state.ZBattlefield})
			// CR 702.116a / CR 511.2: "exile the tokens at end of combat" is
			// a delayed triggered ability. The mint registers the AtEOT$
			// ExileCombat shape (Geist of Saint Traft's Angel token, the
			// self-sourced __kwWarpExile at this turn's end-of-combat step):
			// at the step's beginning the trigger goes on the stack, players
			// get priority while the token is still attacking, and the
			// resolution exiles it.
			exile, _ := atEOTRiders.Get("ExileCombat")
			scheduleAtEOTRider(h, c.Controller, 0, exile, []state.ObjID{want})
		}
	}
}

// myriadOpponents returns the players Myriad considers, in APNAP order. The
// defending player is carried by triggerRemembered as the final player target,
// so it survives the stack object's context rebuild on every resumed choice.
func myriadOpponents(g *state.Game, c *Ctx) []state.PlayerID {
	defender := c.Controller
	for i := len(c.Remembered) - 1; i >= 0; i-- {
		if c.Remembered[i].IsPlayer {
			defender = c.Remembered[i].Player
			break
		}
	}
	out := make([]state.PlayerID, 0)
	for _, q := range g.AliveFrom(c.Controller) {
		if q != c.Controller && q != defender {
			out = append(out, q)
		}
	}
	return out
}
