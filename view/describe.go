package view

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Describe renders one event as one line of rules transcript, the same
// line on every replay: it reads names from g and fields from ev and
// nothing else — no clock, no map iteration, no engine. Call it with the
// game as of the last event in ev's batch, the RedactEvents convention, so
// a redacted Obj (0) reads as "a card" and a visible one by name. The
// client never composes rules text; this is where the words come from.
//
// ClockTick describes as "" (the client hides empty lines); an unknown
// Kind as "unknown event" rather than a panic.
func Describe(g *state.Game, ev events.Event) string {
	switch ev.Kind {
	case events.GameStart:
		return "Game starts with " + itoa(int64(ev.Amount)) + " players"
	case events.Shuffle:
		return player(g, ev.Player) + " shuffles their library"
	case events.MoveZone:
		return obj(g, ev.Obj) + " moves from " + zone(ev.From) + " to " + zone(ev.To)
	case events.Draw:
		if ev.Obj == 0 {
			return player(g, ev.Player) + " draws a card"
		}
		return player(g, ev.Player) + " draws " + obj(g, ev.Obj)
	case events.LifeChange:
		verb, n := "gains", ev.Amount
		if n < 0 {
			verb, n = "loses", -n
		}
		return player(g, ev.Player) + " " + verb + " " + itoa(int64(n)) + " life (" + life(g, ev.Player) + ")"
	case events.Damage:
		if g != nil && g.Obj(ev.Obj) != nil {
			return obj(g, ev.Obj) + " takes " + itoa(int64(ev.Amount)) + " damage"
		}
		return player(g, ev.Player) + " takes " + itoa(int64(ev.Amount)) + " damage"
	case events.Tap:
		return obj(g, ev.Obj) + " taps"
	case events.Untap:
		return obj(g, ev.Obj) + " untaps"
	case events.StepChange:
		return "Step: " + ev.Step.String()
	case events.TurnChange:
		return "Turn " + itoa(int64(ev.Amount)) + ": " + player(g, ev.Player)
	case events.Priority:
		return player(g, ev.Player) + " has priority"
	case events.PutOnStack:
		return player(g, ev.Player) + " casts " + obj(g, ev.Obj)
	case events.Resolve:
		return obj(g, ev.Obj) + " resolves"
	case events.ManaAdd:
		if ev.Amount < 0 {
			return player(g, ev.Player) + " spends " + mana(ev.Counter, -ev.Amount)
		}
		return player(g, ev.Player) + " adds " + mana(ev.Counter, ev.Amount)
	case events.ManaClear:
		return player(g, ev.Player) + "'s mana pool empties"
	case events.CounterChange:
		verb, n := "gets", ev.Amount
		if n < 0 {
			verb, n = "loses", -n
		}
		s := obj(g, ev.Obj) + " " + verb + " " + itoa(int64(n)) + " " + ev.Counter + " counter"
		if n != 1 {
			s += "s"
		}
		return s
	case events.DeclareAttackers:
		if len(ev.IDs) == 0 {
			return "No attackers"
		}
		return objs(g, ev.IDs) + " " + plural(len(ev.IDs), "attacks", "attack") + " " + player(g, ev.Player)
	case events.DeclareBlockers:
		if len(ev.Pairs) == 0 {
			return "No blocks"
		}
		parts := make([]string, 0, len(ev.Pairs))
		for _, p := range ev.Pairs {
			parts = append(parts, obj(g, p[1])+" blocks "+obj(g, p[0]))
		}
		return strings.Join(parts, "; ")
	case events.PlayerLost:
		return player(g, ev.Player) + " loses the game"
	case events.GameOver:
		if ev.Amount == 1 {
			return "The game is a draw"
		}
		return player(g, ev.Player) + " wins the game"
	case events.DecisionAsk:
		return player(g, ev.Player) + " is asked: " + ev.Text
	case events.DecisionMade:
		return player(g, ev.Player) + " answers " + ev.Text
	case events.Note:
		if ev.Text != "" {
			return ev.Text
		}
		if ev.Secret {
			return player(g, ev.Player) + " looks at hidden cards"
		}
		// Neither a message nor Secret: a malformed or defensively-tested
		// Note (TestDescribeCoversEveryKind's generic fuzz event hits this
		// exact shape). "" is reserved for ClockTick alone, so this still
		// needs a word.
		return "Note"
	case events.LandPlayed:
		return player(g, ev.Player) + " plays a land"
	case events.TargetsChosen:
		if ev.Amount == 1 {
			return obj(g, ev.Obj) + " targets " + player(g, ev.Player)
		}
		return obj(g, ev.Obj) + " targets " + objs(g, ev.IDs)
	case events.FlipFace:
		return obj(g, ev.Obj) + " turns to face " + itoa(int64(ev.Amount))
	case events.ClockTick:
		return ""
	case events.TriggerPush:
		return obj(g, ev.Obj) + " triggers"
	case events.EndCombatReset:
		return "Combat ends"
	}
	return "unknown event"
}

// obj names an object as "Name #id", "an ability #id" for a faceless
// stack object, "a card" for the redacted id 0, and "#id" for an id the
// game cannot resolve (nil g, stale or tampered data).
func obj(g *state.Game, id state.ObjID) string {
	if id == 0 {
		return "a card"
	}
	tag := "#" + strconv.FormatUint(uint64(id), 10)
	if g == nil {
		return tag
	}
	o := g.Obj(id)
	if o == nil {
		return tag
	}
	if f := o.Face(); f != nil && f.Name != "" {
		return f.Name + " " + tag
	}
	return "an ability " + tag
}

func objs(g *state.Game, ids []state.ObjID) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, obj(g, id))
	}
	return strings.Join(parts, ", ")
}

// player is the seat's name, or "seat N" when g cannot resolve it.
func player(g *state.Game, p state.PlayerID) string {
	if g != nil && int(p) < len(g.Players) && g.Players[p].Name != "" {
		return g.Players[p].Name
	}
	return "seat " + strconv.Itoa(int(p))
}

// life is the seat's life total as of g, or "?" when unresolvable.
func life(g *state.Game, p state.PlayerID) string {
	if g != nil && int(p) < len(g.Players) {
		return itoa(int64(g.Players[p].Life))
	}
	return "?"
}

// zone is the zone's name, total over out-of-range values.
func zone(z state.Zone) string {
	if !z.Valid() {
		return "nowhere"
	}
	return z.String()
}

// mana renders n symbols of one colour: "{G}{G}". An empty symbol is
// colourless.
func mana(sym string, n int32) string {
	if sym == "" {
		sym = "C"
	}
	if n <= 0 {
		return ""
	}
	if n > 20 {
		return itoa(int64(n)) + " {" + sym + "}"
	}
	return strings.Repeat("{"+sym+"}", int(n))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
