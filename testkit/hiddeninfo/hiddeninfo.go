// Package hiddeninfo is a test oracle for what a seat may read: it follows a
// game and says whether a piece of text names a card that seat cannot see.
//
// A seat knows a card's name once a public zone or a public reveal has held the
// card, once the card has been in that seat's own hand, or once the seat was
// privately shown it: its own Secret event, a face-down permanent it controls,
// a library top its view shows, or an option a decision offered it. A seat's
// own library does not count; it knows its deck list, but not which card a line
// is talking about. Basic land names are never hidden, because rules text uses
// them as land types whatever the decks hold, and neither are the rules words
// in ruleWords.
package hiddeninfo

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// Leak is one piece of text naming a card its reader cannot see.
type Leak struct {
	Seat  state.PlayerID
	Where string // the field or event the text came from
	Name  string
	Text  string
}

func (l Leak) String() string {
	return "seat " + strconv.Itoa(int(l.Seat)) + " " + l.Where + " names " + strconv.Quote(l.Name) + ": " + l.Text
}

// Oracle follows one engine's game.
type Oracle struct {
	e *rules.Engine
	// names are the printed names of every card in the game's decks.
	names []string
	// known holds, per name, a bit for each seat that may read it.
	known map[string]uint64
	// folded is how many log events have been read.
	folded int
}

var basicLands = map[string]bool{
	"Plains": true, "Island": true, "Swamp": true, "Mountain": true, "Forest": true, "Wastes": true,
	"Snow-Covered Plains": true, "Snow-Covered Island": true, "Snow-Covered Swamp": true,
	"Snow-Covered Mountain": true, "Snow-Covered Forest": true, "Snow-Covered Wastes": true,
}

// ruleWords are card names rules text and card facts use as ordinary words: a
// keyword action, a zone verb, a spell's API, an aura's AI hint.
var ruleWords = map[string]bool{"Sacrifice": true, "Exile": true, "Fog": true, "Curiosity": true}

// New starts an oracle on e before or at its first decision.
func New(e *rules.Engine) *Oracle {
	o := &Oracle{e: e, known: map[string]uint64{}}
	seen := map[string]bool{}
	for i := range e.G.Objs {
		obj := &e.G.Objs[i]
		if obj.Card == nil {
			continue
		}
		for _, f := range obj.Card.Faces {
			if f.Name != "" && !seen[f.Name] && !basicLands[f.Name] && !ruleWords[f.Name] {
				seen[f.Name] = true
				o.names = append(o.names, f.Name)
			}
		}
	}
	return o
}

const everyone = ^uint64(0)

// bit is seat's bit in known; a spectator (view.NoSeat) has the last one.
func bit(seat state.PlayerID) uint64 {
	if seat == view.NoSeat {
		return 1 << 63
	}
	return 1 << seat
}

func (o *Oracle) learn(obj *state.Object, seats uint64) {
	if obj == nil || obj.Card == nil {
		return
	}
	for _, f := range obj.Card.Faces {
		o.known[f.Name] |= seats
	}
}

// Observe folds what has happened since the last call and returns those events.
// Call it at a decision, with that decision, before reading any seat's text.
func (o *Oracle) Observe(d *decision.Decision) []events.Event {
	g := o.e.G
	evs := o.e.L.Events[o.folded:]
	for _, ev := range evs {
		o.foldEvent(g, ev)
	}
	o.folded = len(o.e.L.Events)
	for i := range g.Objs {
		obj := &g.Objs[i]
		switch {
		case obj.FaceDown:
			o.learn(obj, bit(obj.Controller))
		case obj.Zone == state.ZHand:
			o.learn(obj, bit(obj.Owner))
			// CR 720.4: a seat controlling another sees what that seat sees.
			if ctl, ok := g.LiveController(obj.Owner); ok {
				o.learn(obj, bit(ctl))
			}
			// A live reveal grant (Telepathy) shows the card to every seat.
			if o.e.HandCardRevealed(obj.ID) {
				o.learn(obj, everyone)
			}
		case !obj.Zone.Hidden():
			o.learn(obj, everyone)
		}
	}
	// A live look-at-the-top grant (Mystic Forge: "you may look at the top
	// card of your library any time") shows that card to its seat, and to a
	// seat controlling it (CR 720.4); a play-revealed grant shows it to every
	// seat (CR 401.5). The projection fills LibraryTop behind the same two
	// engine reads.
	for pi := range g.Players {
		p := state.PlayerID(pi)
		lib := g.Zone(state.ZLibrary, p)
		if len(lib) == 0 {
			continue
		}
		top := g.Obj(lib[0])
		if o.e.LibraryTopRevealed(p) {
			o.learn(top, everyone)
		} else if o.e.MayLookAtLibraryTop(p) {
			o.learn(top, bit(p))
			if ctl, ok := g.LiveController(p); ok {
				o.learn(top, bit(ctl))
			}
		}
	}
	if d != nil {
		for _, opt := range d.Options {
			if obj := g.Obj(opt.Obj); obj != nil && obj.Zone.Hidden() {
				o.learn(obj, bit(d.Player))
			}
		}
	}
	return evs
}

func (o *Oracle) foldEvent(g *state.Game, ev events.Event) {
	if ev.Secret {
		for seat := range g.Players {
			r := view.RedactEvent(g, ev, state.PlayerID(seat))
			o.learn(g.Obj(r.Obj), bit(state.PlayerID(seat)))
			for _, id := range r.IDs {
				o.learn(g.Obj(id), bit(state.PlayerID(seat)))
			}
		}
		return
	}
	switch ev.Kind {
	case events.Note:
		// A public Note's ids are a reveal to every seat (view.RedactEvent's rule 3).
		for _, id := range ev.IDs {
			o.learn(g.Obj(id), everyone)
		}
	case events.MoveZone, events.PutOnStack:
		if obj := g.Obj(ev.Obj); obj != nil && !obj.FaceDown && (!ev.From.Hidden() || !ev.To.Hidden()) {
			o.learn(obj, everyone)
		}
	}
}

// Check returns a leak for each name in text that seat may not read.
func (o *Oracle) Check(seat state.PlayerID, where, text string) []Leak {
	if text == "" {
		return nil
	}
	var out []Leak
	for _, name := range o.names {
		if o.known[name]&bit(seat) != 0 || !containsName(text, name) {
			continue
		}
		// "Exploration" is no leak in a line naming Valakut Exploration.
		rest := text
		for _, longer := range o.names {
			if longer != name && o.known[longer]&bit(seat) != 0 && strings.Contains(longer, name) {
				rest = strings.ReplaceAll(rest, longer, "")
			}
		}
		if containsName(rest, name) {
			out = append(out, Leak{Seat: seat, Where: where, Name: name, Text: text})
		}
	}
	for _, name := range o.faceDownNames(seat) {
		if containsName(text, name) {
			out = append(out, Leak{Seat: seat, Where: where, Name: name, Text: text})
		}
	}
	return out
}

// faceDownNames lists the printed names of the face-down permanents and spells
// seat does not control, leaving out a name a public object also holds: text
// naming it cannot be told apart from text naming that object.
func (o *Oracle) faceDownNames(seat state.PlayerID) []string {
	g := o.e.G
	var out []string
	for i := range g.Objs {
		obj := &g.Objs[i]
		if !obj.FaceDown || obj.Controller == seat || obj.Card == nil || (obj.Zone != state.ZBattlefield && obj.Zone != state.ZStack) {
			continue
		}
		name := obj.Card.Faces[obj.FaceIdx].Name
		if basicLands[name] || o.publiclyHeld(name) {
			continue
		}
		out = append(out, name)
	}
	return out
}

func (o *Oracle) publiclyHeld(name string) bool {
	g := o.e.G
	for i := range g.Objs {
		obj := &g.Objs[i]
		if obj.Zone.Hidden() || obj.FaceDown || obj.Card == nil {
			continue
		}
		if f := obj.Face(); f != nil && f.Name == name {
			return true
		}
	}
	return false
}

// containsName reports whether name appears in text with no word character
// joined to either end of it.
func containsName(text, name string) bool {
	for from := 0; ; {
		i := strings.Index(text[from:], name)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(name)
		if (i == 0 || !isWordByte(text[i-1])) && (end == len(text) || !isWordByte(text[end])) {
			return true
		}
		from = i + 1
	}
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// CheckView checks seat's whole projection, as JSON, except what names its own
// library: its deck manifest and library list. A decision is checked by
// CheckDecision, a stack ability's source by checkStack.
func (o *Oracle) CheckView(seat state.PlayerID, v *view.View) []Leak {
	// The projection may be handed to a seat, so it is copied, not edited.
	shown := *v
	shown.OwnDeck, shown.Decision = nil, nil
	shown.Players = append([]view.PlayerView(nil), v.Players...)
	if int(seat) < len(shown.Players) {
		shown.Players[seat].Library = nil
	}
	out := o.CheckJSON(seat, "view", shown)
	out = append(out, o.checkStack(seat, v)...)
	if v.Decision != nil {
		out = append(out, o.CheckDecision(seat, v.Decision)...)
	}
	return out
}

// PublicChoice reports whether d's text may name cards the seat hasn't otherwise
// seen: a look's acknowledgement names what the seat is being shown, and a
// card-name choice lists the card universe, not the game's decks.
func PublicChoice(d *decision.Decision) bool {
	return d.ResumeKind == "look_ack" || len(d.Options) > 0 && d.Options[0].Kind == "name"
}

// CheckDecision checks every text a decision carries.
func (o *Oracle) CheckDecision(seat state.PlayerID, d *decision.Decision) []Leak {
	if PublicChoice(d) {
		return nil
	}
	return o.CheckJSON(seat, "decision", d)
}

// checkStack checks a stack ability against its source: a name seat knows from
// elsewhere still doesn't belong on an ability from a card it can't see now.
func (o *Oracle) checkStack(seat state.PlayerID, v *view.View) []Leak {
	var out []Leak
	g := o.e.G
	for i := range v.Stack {
		s := &v.Stack[i]
		so := g.Obj(s.ID)
		if so == nil || so.Ability == nil {
			continue
		}
		src := g.Obj(so.Source)
		if src == nil || src.Card == nil || !hiddenFrom(src, seat) {
			continue
		}
		for _, f := range src.Card.Faces {
			if f.Name == "" {
				continue
			}
			if s.Name == f.Name {
				out = append(out, Leak{Seat: seat, Where: "stack.name, hidden source", Name: f.Name, Text: s.Name})
			}
			if containsName(s.Text, f.Name) {
				out = append(out, Leak{Seat: seat, Where: "stack.text, hidden source", Name: f.Name, Text: s.Text})
			}
		}
	}
	return out
}

// CheckEvents checks the events as seat is served them: each line of the log and
// each event's own text.
func (o *Oracle) CheckEvents(seat state.PlayerID, vis view.Visibility, evs []events.Event) []Leak {
	var out []Leak
	for _, ev := range evs {
		r := view.RedactEventFor(o.e.G, ev, seat, vis)
		where := "log." + ev.Kind.String()
		out = append(out, o.Check(seat, where, view.Describe(o.e.G, r))...)
		out = append(out, o.Check(seat, where+".text", r.Text)...)
	}
	return out
}

// CheckJSON marshals v and checks every string value in it.
func (o *Oracle) CheckJSON(seat state.PlayerID, where string, v any) []Leak {
	blob, err := json.Marshal(v)
	if err != nil {
		return []Leak{{Seat: seat, Where: where, Text: err.Error()}}
	}
	var tree any
	if err := json.Unmarshal(blob, &tree); err != nil {
		return []Leak{{Seat: seat, Where: where, Text: err.Error()}}
	}
	var out []Leak
	var walk func(path string, x any)
	walk = func(path string, x any) {
		switch t := x.(type) {
		case string:
			out = append(out, o.Check(seat, path, t)...)
		case []any:
			for _, e := range t {
				walk(path, e)
			}
		case map[string]any:
			for k, e := range t {
				walk(path+"."+k, e)
			}
		}
	}
	walk(where, tree)
	return out
}

// AnyFaceDown reports whether a game holds a face-down object, for a test to
// show its games reach one.
func AnyFaceDown(g *state.Game) bool {
	for i := range g.Objs {
		if g.Objs[i].FaceDown {
			return true
		}
	}
	return false
}

// hiddenFrom reports whether seat cannot see obj where it now sits: a library is
// hidden from every seat (CR 401.2), a hand from all but its owner (CR 402.3), a
// face-down object from all but its controller (CR 708.5).
func hiddenFrom(obj *state.Object, seat state.PlayerID) bool {
	switch {
	case obj.FaceDown:
		return obj.Controller != seat
	case obj.Ephemeral():
		return false
	case obj.Zone == state.ZLibrary:
		return true
	case obj.Zone == state.ZHand:
		return obj.Owner != seat
	}
	return false
}
