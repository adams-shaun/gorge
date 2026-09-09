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
	case events.LibraryOrder:
		return player(g, ev.Player) + " rearranges the top of their library"
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
	case events.CastInfo:
		// Records how a spell was cast, right before the PutOnStack line
		// (Task 4): Amount is the value chosen for {X}, Counter the comma-
		// separated mode flags ("kicked", ...). Described, not "": X and
		// the kicker are the caster's own decisions and nothing else in the
		// transcript shows them (mana-spend lines record totals, not what
		// they paid for), and the line is self-contained, so a DVR scrub
		// landing on it needs no neighbouring line. It carries no Player
		// field, so the caster is the card's controller. The two lines read
		// as one cast: the how, then the fact.
		cast := player(g, objController(g, ev.Obj)) + " casts " + obj(g, ev.Obj)
		how := make([]string, 0, 2)
		if ev.Amount != 0 {
			how = append(how, "X = "+itoa(int64(ev.Amount)))
		}
		if ev.Counter != "" {
			how = append(how, ev.Counter)
		}
		if len(how) == 0 {
			return cast
		}
		return cast + " (" + strings.Join(how, ", ") + ")"
	case events.Choose:
		// Records an "as this enters, choose ..." answer (etbAsk/etbAnswer):
		// Counter discriminates the shape ("name", "type", "number"; the
		// chosen name/type rides on Text, the number on Amount). Like
		// CastInfo it carries no Player field, so the chooser is the card's
		// controller. An unrecognized Counter (a fuzz event, a future
		// shape) degrades to a generic "chooses a value" line rather than
		// inventing a field.
		what := "a value"
		switch ev.Counter {
		case "name":
			what = "the name " + ev.Text
		case "type":
			what = "the type " + ev.Text
		case "number":
			what = "the number " + itoa(int64(ev.Amount))
		}
		return player(g, objController(g, ev.Obj)) + " chooses " + what + " for " + obj(g, ev.Obj)
	case events.TokenCreate:
		// A token minted onto the battlefield (Player is its owner and
		// controller). The event names the token by its script key -- the
		// minted object's id is assigned inside Apply, so there is nothing
		// to read from ev.Obj -- and the display name comes from the game's
		// token table; an unresolvable key (a hand-built game, the fuzz)
		// degrades to a plain "creates a token".
		if name := tokenName(g, ev.Text); name != "" {
			return player(g, ev.Player) + " creates a " + name + " token"
		}
		return player(g, ev.Player) + " creates a token"
	case events.StackCopy:
		// A copy of the stack object Obj, controlled by Player (CR
		// 707.10a), placed on top of the stack. The copy itself gets a new
		// id Apply assigns, so the line names the original it duplicates.
		return player(g, ev.Player) + " copies " + obj(g, ev.Obj)
	case events.Attach:
		// Aura/Equipment permanent Obj attaches to (IDs[0]) or detaches
		// from (empty IDs) its bearer. rules/attach.go's one detach-with-
		// reason site carries the why on Text; when present it is appended,
		// because "detaches" alone cannot explain an Equipment floating
		// free of its bearer mid-combat.
		if len(ev.IDs) > 0 {
			return obj(g, ev.Obj) + " attaches to " + obj(g, ev.IDs[0])
		}
		s := obj(g, ev.Obj) + " detaches"
		if ev.Text != "" {
			s += " (" + ev.Text + ")"
		}
		return s
	case events.AbilityPush:
		// An activated ability minted onto the stack (the same shape
		// TriggerPush uses for triggers, Ruling T20-a): Player is the
		// activator, Obj the source permanent. The mirror of "X triggers":
		// the IR carries no ability names, so the source permanent is what
		// a line can name.
		return player(g, ev.Player) + " activates " + obj(g, ev.Obj)
	case events.ModeChosen:
		// The answer to a mid-resolution modal decision (M2d-2): the
		// "modes" Charm pick or the "unless_pay" yes/no. Player chose;
		// Text carries the chosen option labels as csv. Mirrors
		// DecisionMade's "answers" shape; an empty Text (a fuzz event)
		// degrades to "a mode".
		labels := strings.ReplaceAll(ev.Text, ",", ", ")
		if labels == "" {
			labels = "a mode"
		}
		return player(g, ev.Player) + " chooses " + labels
	case events.CmdDamage:
		// Commander combat damage to a player (CR 903.10, task m33): Obj
		// is the source commander, Player the damaged player, Amount what
		// actually landed. The ordinary Damage event already says "Bob
		// takes N damage"; this line is the second clock -- the
		// per-commander cumulative tally and its lethal threshold -- the
		// thing this event adds and the only reason it exists. The tally
		// is read as of g (the batch's last Apply already folded this hit
		// in); it is unknown in a non-Commander game or when the source is
		// not on any roster, so the parenthetical drops then.
		s := obj(g, ev.Obj) + " deals " + itoa(int64(ev.Amount)) + " commander damage to " + player(g, ev.Player)
		if total, ok := cmdDamageTally(g, ev.Player, ev.Obj); ok {
			s += " (" + itoa(int64(total)) + " total; 21 is lethal)"
		}
		return s
	case events.DelayedRegister:
		// A delayed trigger being registered (CR 603.7, dt1): Obj is the
		// source that created it, Text the phase it waits for. The line says
		// only that the promise was made -- what it will DO is the Execute$
		// sub-ability, which is named by the DelayedPush line below when it
		// actually happens, so saying it twice here would double-report an
		// effect that may never fire (the source can leave, the controller
		// can lose, the game can end first).
		if ev.Text == "" {
			return obj(g, ev.Obj) + " sets up a delayed trigger"
		}
		return obj(g, ev.Obj) + " sets up a delayed trigger for " + ev.Text
	case events.DelayedPush:
		// The registered phase arrived and the delayed ability went on the
		// stack. Obj is the minted stack object; its source name is what a
		// reader recognises, so prefer it and fall back to the minted id.
		return obj(g, ev.Obj) + " triggers (delayed)"
	}
	return "unknown event"
}

// obj names an object as "Name #id", "<Name>'s ability #id" for a faceless
// stack object whose source card is resolvable, "an ability #id" for a
// faceless object whose source is not (0, unresolvable, or itself faceless),
// "a card" for the redacted id 0, and "#id" for an id the game cannot
// resolve (nil g, stale or tampered data).
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
	if src := abilitySource(g, o); src != "" {
		return src + "'s ability " + tag
	}
	return "an ability " + tag
}

// abilitySource resolves a faceless ability object's source (state.Object.
// Source, set by events.Apply's TriggerPush/AbilityPush/DelayedPush minting
// paths) to the name of the card it came from, or "" when that path cannot
// produce one: a zero source id, an id the game no longer holds, or a source
// that is itself faceless (a copied or minted ability) — a source whose own
// name is unknowable must not yield an empty possessive, so it degrades to
// the bare "an ability #id" instead. It mirrors obj's own care about nil g,
// id == 0 and stale ids, because this is called on log lines for tampered
// and historical data.
func abilitySource(g *state.Game, o *state.Object) string {
	if o.Source == 0 {
		return ""
	}
	src := g.Obj(o.Source)
	if src == nil {
		return ""
	}
	if f := src.Face(); f != nil && f.Name != "" {
		return f.Name
	}
	return ""
}

func objs(g *state.Game, ids []state.ObjID) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, obj(g, id))
	}
	return strings.Join(parts, ", ")
}

// objController is the player who controls id, or seat 0 when g cannot
// resolve it. CastInfo and Choose carry no Player field of their own (a
// card's controller is its caster/chooser at cast time), so their lines
// derive it from the object rather than reading the event's zero Player.
func objController(g *state.Game, id state.ObjID) state.PlayerID {
	if g == nil {
		return 0
	}
	if o := g.Obj(id); o != nil {
		return o.Controller
	}
	return 0
}

// player is the seat's display name (an independent PlayerName when one was
// configured, else the deck identity the engine hashes), or "seat N" when g
// cannot resolve it. Transcript lines read the same name the player box
// shows, so the log and the box never disagree about who is who.
func player(g *state.Game, p state.PlayerID) string {
	if g != nil && int(p) < len(g.Players) {
		if g.Players[p].PlayerName != "" {
			return g.Players[p].PlayerName
		}
		if g.Players[p].Name != "" {
			return g.Players[p].Name
		}
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

// tokenName is the display name of the token definition a TokenCreate
// event's script key names ("Goblin" for "r_1_1_goblin"), or "" when the
// game has no such definition (a hand-built game, an unknown key, the
// fuzz).
func tokenName(g *state.Game, key string) string {
	if g == nil {
		return ""
	}
	c, ok := g.Tokens[key]
	if !ok || c == nil || len(c.Faces) == 0 {
		return ""
	}
	return c.Faces[0].Name
}

// cmdDamageTally returns player p's cumulative commander damage as of g
// from the commander id -- CR 903.10's second clock, the tally 21 is
// lethal against -- and whether id is on any roster. It mirrors the
// match-wide dense index events.Apply folds CmdDamage into (seat order,
// then each seat's genesis Commanders order; never a map), so the total
// reads the same slot a log-only reconstruction would rebuild.
func cmdDamageTally(g *state.Game, p state.PlayerID, id state.ObjID) (int32, bool) {
	if g == nil || int(p) >= len(g.Players) {
		return 0, false
	}
	idx := 0
	for s := range g.Players {
		for _, c := range g.Players[s].Commanders {
			if c == id {
				if idx < len(g.Players[p].CmdDamage) {
					return g.Players[p].CmdDamage[idx], true
				}
				return 0, false
			}
			idx++
		}
	}
	return 0, false
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
