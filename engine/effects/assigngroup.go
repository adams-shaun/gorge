package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("AssignGroup", effAssignGroup)
}

// effAssignGroup implements Forge's AssignGroupEffect, the Battlebond
// "for each player, choose friend or foe" shape (Pir's Whim, Khorvath's Fury,
// Zndrsplt's Judgment, Regna's Sanction, Virtus's Maneuver). The spell's
// controller assigns EACH player Defined$ names (themself included) to one of
// the Choices$ groups -- one KChoose ask per player, in APNAP order from the
// controller (CR 101.4), answered through AskTape so the choice is a recorded
// intent and replay re-derives it. Then each non-empty group's SVar body runs
// in Choices$ order with exactly that group's players as its Remembered set,
// so the bodies' Defined$ Remembered / RepeatPlayers$ Remembered /
// Creature.RememberedPlayerCtrl selectors read the group. Nothing here moves
// state: the bodies' own primitives emit every event.
//
// The no-host stand-in (no answer served) assigns the controller to the first
// group and everyone else to the last, which is the Friend/Foe split the bot
// answers as well.
//
// Scoped out, each ONE loud Note and nothing moves: a Chooser$ other than the
// controller, a Choices$ name that is no SVar of the resolving face.
func effAssignGroup(h Host, c *Ctx, sa *cards.SA) {
	ap := compileAssignGroup(sa)
	loud := func(text string) {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "unimplemented AssignGroup shape: " + text})
	}
	if ap.Chooser != "" && ap.Chooser != "You" {
		loud("Chooser$ " + ap.Chooser)
		return
	}
	bodies := make([]*cards.SA, len(ap.Choices))
	for i, name := range ap.Choices {
		if bodies[i] = cards.ResolveSVar(c.SVars, name); bodies[i] == nil {
			loud("Choices$ " + name + " does not resolve")
			return
		}
	}
	if len(bodies) == 0 {
		loud("no Choices$")
		return
	}
	g := h.Game()
	var players []state.PlayerID
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer && int(t.Player) < len(g.Players) && !g.Players[t.Player].Lost {
			players = append(players, t.Player)
		}
	}
	groups := make([][]state.Target, len(bodies))
	for _, p := range players {
		i := assignGroupPick(h, c, sa, ap, p)
		groups[i] = append(groups[i], state.Target{Player: p, IsPlayer: true})
	}
	for i, body := range bodies {
		if len(groups[i]) == 0 {
			continue
		}
		cc := *c
		cc.Remembered = groups[i]
		Resolve(h, &cc, body)
		if cc.Restarted {
			c.Restarted = true
			return
		}
		if c.FlipMemory == nil && cc.FlipMemory != nil {
			c.FlipMemory = cc.FlipMemory
		}
		if h.Suspended() {
			return
		}
	}
}

// assignGroupPick poses the ask that assigns player p to one group and
// returns the group's index. With a single group nothing is asked.
func assignGroupPick(h Host, c *Ctx, sa *cards.SA, ap *AssignGroupParams, p state.PlayerID) int {
	last := len(ap.Choices) - 1
	pick := last
	if p == c.Controller {
		pick = 0
	}
	if last == 0 {
		return 0
	}
	g := h.Game()
	d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
		Min: 1, Max: 1, Source: c.Source, ResumeKind: "assigngroup", ResumeSA: sa,
		Prompt: "Choose a group for " + g.SeatFacingName(p)}
	for i, name := range ap.Choices {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "group",
			Label: assignGroupLabel(ap, i, name), Player: p})
	}
	if ans, ok := AskTape(h, d); ok && len(ans) > 0 {
		if i := ans[0].Index; i >= 0 && i <= last {
			return i
		}
	}
	return pick
}

// assignGroupLabel names group i for the ask: Friend/Foe for the AILogic$
// FriendOrFoe cycle, else the SVar name.
func assignGroupLabel(ap *AssignGroupParams, i int, name string) string {
	if ap.FriendOrFoe && len(ap.Choices) == 2 {
		return [...]string{"Friend", "Foe"}[i]
	}
	return name
}
