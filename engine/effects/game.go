package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func effBecomeMonarch(h Host, c *Ctx, sa *cards.SA) {
	targets := Defined(h, c, sa)
	if len(targets) == 0 {
		return
	}
	players := definedPlayers(h, c, sa)
	if len(players) == 0 {
		return
	}
	p := players[0]
	if g := h.Game(); g != nil && g.IsMonarch(p) {
		return
	}
	h.Emit(events.Event{Kind: events.MonarchChange, Player: p})
}

// restartHost is the optional Host half of a CR 727 restart (rules' Engine).
// RequestRestart records the restart for the host to perform once this
// resolution returns: keep is the CR 727.5 exempted set, and rest is the
// rest of the chain. The remainder is NOT followed yet -- CR 727.4's
// "finishes resolving just before the first turn's untap step" is cr-g33b,
// so this ticket leaves the exempted cards where they are.
type restartHost interface {
	RequestRestart(c *Ctx, keep []state.ObjID, rest *cards.SA)
}

// effRestartGame restarts the game (CR 727.1, Karn Liberated's [-14]).
// RestrictFromZone$/RestrictFromValid$ name what the restart takes: Forge's
// RestartGame moves into the new game every card in RestrictFromZone that
// matches RestrictFromValid, and leaves the rest of that zone where it is --
// Karn keeps exactly the non-Aura permanent cards exiled with him (his
// ReturnFromExile sub-ability will act on that keep set in the new game;
// cr-g33b). A host
// that implements restartHost performs the restart; the chain stops here
// (Ctx.Restarted). A host without one (a bare effects test host) keeps the
// R-9 degradation: the game ends as a draw (Amount 1, the GameOver draw
// shape; Ruling T22-k).
func effRestartGame(h Host, c *Ctx, sa *cards.SA) {
	keep := restartKeepSet(h, c, sa)
	if rh, ok := h.(restartHost); ok {
		rh.RequestRestart(c, keep, sa.Sub)
		c.Restarted = true
		return
	}
	h.Emit(events.Event{Kind: events.GameOver, Amount: 1, Text: "game restarted: ended as a draw"})
}

// restartKeepSet is the CR 727.5 exempted set: inside RestrictFromZone$ (All
// or an invalid list: every zone), every object RestrictFromValid$ does NOT
// match, in zone, seat and zone order. The shared stack is walked once, under
// the first alive seat (engine/state/game.go Zone), so a stack object is never
// listed twice on an N-seat table. Either parameter missing keeps nothing.
func restartKeepSet(h Host, c *Ctx, sa *cards.SA) []state.ObjID {
	zonesRaw := strings.TrimSpace(sa.ParamStr(cards.PKRestrictFromZone))
	spec := strings.TrimSpace(sa.ParamStr(cards.PKRestrictFromValid))
	if zonesRaw == "" || spec == "" {
		return nil
	}
	zones, all, valid := ParseZones(zonesRaw)
	g := h.Game()
	if !valid || all {
		zones = []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield,
			state.ZGraveyard, state.ZExile, state.ZStack, state.ZCommand}
	}
	var keep []state.ObjID
	for _, z := range zones {
		for si, p := range g.AliveFrom(0) {
			if z == state.ZStack && si > 0 {
				continue
			}
			for _, id := range g.Zone(z, p) {
				if !MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
					keep = append(keep, id)
				}
			}
		}
	}
	return keep
}

// effMana implements "AB$ Mana": add Amount mana of Produced's colour(s) to
// the pool of each ManaRecipients player (the activating player unless
// Defined$ names another). Absorbed from Task 14's stopgap: the
// negative-Amount clamp is Ruling T14-f, kept verbatim for the same reason as
// DealDamage's -- events.Apply's ManaAdd case is a plain "+=", so an
// unclamped negative would drop the pool below zero instead of doing
// nothing.
//
// A resolution-time host now gets the same CR 106.6 colour choice as the
// activation path. A host without a decision channel keeps the R-9 fallback:
// Any becomes colourless and an unasked Combo list retains its old full-listed
// output. A dual-producing ability such as "Add {R}{R}" is walked one symbol
// at a time rather than split on whitespace, since Produced$ carries no spaces
// of its own.
//
// The one thing this primitive must NEVER do is walk a value it does not
// understand. A "Combo R G" reaches effMana from a path with no colour
// chooser (the activation path substitutes the chosen colour in first), and
// walking it one rune at a time turned "Combo R G" into five stray
// colourless plus a red and a green -- o, m, b are not mana symbols. An
// unrecognised Produced$ value therefore emits nothing and records a Note
// naming it, following this repo's fail-closed convention (an unknown token
// never invents a value).
// effReplaceMana rewrites one in-flight ManaAdd event for a ProduceMana
// replacement. The surrounding rules code supplies the amount and colour in
// Ctx, then logs the rewritten ManaAdd; this effect itself has no game-state
// mutation to emit. ReplaceAmount multiplies the whole production.
// ReplaceType/ReplaceColor preserve its amount and replace only its colour;
// ReplaceMana is Forge's "one mana instead of any other type and amount"
// form (Damping Sphere, Contamination), so it sets the amount to exactly one
// as well as replacing the colour. For a choice-valued replacement
// (Any/Chosen), rules parks the ManaAdd and supplies the player's W/U/B/R/G
// answer in Ctx.ManaChoice; without a valid answer this pure effect fails
// closed rather than inventing colourless mana.
