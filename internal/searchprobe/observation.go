package searchprobe

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

type Identity struct {
	ID    uint32
	Name  string
	Owner state.PlayerID
}
type ObservedEvent struct {
	Kind          events.Kind
	Player        state.PlayerID
	Obj           uint32
	From, To      state.Zone
	Amount        int32
	Step          state.Step
	Counter, Text string
	IDs           []uint32
	Pairs         [][2]uint32
	Secret        bool
}
type Frame struct {
	Board      json.RawMessage
	Identities []Identity
	Events     []ObservedEvent
	Decision   *ObservedDecision
}

// Collector alone can read a source engine. History frames contain owned values,
// never this raw-id dictionary or a pointer/callback into the source engine.
type Collector struct {
	actor      state.PlayerID
	known      map[state.ObjID]uint32
	introduced []Identity
}

func NewCollector(actor state.PlayerID) *Collector {
	return &Collector{actor: actor, known: make(map[state.ObjID]uint32)}
}

func (c *Collector) clone() *Collector {
	out := NewCollector(c.actor)
	for id, ref := range c.known {
		out.known[id] = ref
	}
	out.introduced = append([]Identity(nil), c.introduced...)
	return out
}
func (c *Collector) Capture(e *rules.Engine, burst []events.Event) (Frame, error) {
	if e == nil || int(c.actor) >= len(e.G.Players) {
		return Frame{}, fmt.Errorf("invalid observation seat or engine")
	}
	c.introduced = nil
	v := view.Project(e.G, e, c.actor, e.Pending())
	v.Round = view.RoundOf(e.G, e.L.Events)
	// Introduce only cards explicitly displayed to this seat. Traversal order is
	// fixed, so observed identities do not encode hidden arena allocation.
	for _, p := range v.Players {
		if len(p.Commanders) > 0 {
			return Frame{}, fail("unsupported", "commander observation is outside the constructed probe")
		}
		for _, zone := range [][]view.CardView{p.Battlefield, p.Hand, p.Graveyard, p.Exile, p.Command} {
			for _, card := range zone {
				c.introduce(e, card.ID)
			}
		}
	}
	for _, s := range v.Stack {
		c.introduce(e, s.ID)
		c.introduce(e, s.Source)
	}
	for _, p := range v.Pending {
		c.introduce(e, p.Source)
	}
	if d := v.Decision; d != nil {
		c.introduce(e, d.Source)
		for _, o := range d.Options {
			c.introduce(e, o.Obj)
			c.introduce(e, o.Attacker)
		}
	}
	redacted := make([]events.Event, len(burst))
	for i, raw := range burst {
		ev := view.RedactEvent(e.G, raw, c.actor)
		if ev.Kind == events.Shuffle || ev.Kind == events.LibraryOrder {
			// Retain occurrence, never any whole-library payload, even for owner.
			ev = events.Event{Kind: ev.Kind, Player: ev.Player, Secret: ev.Secret}
		}
		if ev.Kind == events.DecisionMade {
			ev.Text = ""
		}
		// Explicit reveal/look channels and visible zone transitions can name a
		// card absent from the end-of-burst board. No other unknown reference is
		// a license to query that object's hidden card identity.
		if !ev.Secret || ev.Player == c.actor {
			switch ev.Kind {
			case events.Note:
				if len(ev.IDs) > 0 && ev.Text != "" && ev.Text != "looks at the top of the library" && !(strings.HasPrefix(ev.Text, "revealed ") && strings.HasSuffix(ev.Text, " as a cost")) {
					return Frame{}, fail("unsupported", "identity-bearing note: %q", ev.Text)
				}
				for _, id := range ev.IDs {
					c.introduce(e, id)
				}
			case events.MoveZone, events.Draw, events.PutOnStack:
				if !ev.From.Hidden() || !ev.To.Hidden() || (ev.Secret && ev.Player == c.actor) {
					c.introduce(e, ev.Obj)
				}
			}
		}
		redacted[i] = ev
	}
	frame := Frame{Identities: append([]Identity(nil), c.introduced...)}
	for _, ev := range redacted {
		out := ObservedEvent{Kind: ev.Kind, Player: ev.Player, Obj: c.ref(ev.Obj), From: ev.From, To: ev.To, Amount: ev.Amount, Step: ev.Step, Counter: ev.Counter, Text: ev.Text, Secret: ev.Secret}
		for _, id := range ev.IDs {
			if ref := c.ref(id); ref != 0 {
				out.IDs = append(out.IDs, ref)
			}
		}
		for _, pair := range ev.Pairs {
			a, b := c.ref(pair[0]), c.ref(pair[1])
			if a != 0 && b != 0 {
				out.Pairs = append(out.Pairs, [2]uint32{a, b})
			}
		}
		frame.Events = append(frame.Events, out)
	}
	var err error
	frame.Decision, err = c.observeDecision(v.Decision)
	if err != nil {
		return Frame{}, err
	}
	v.Decision = nil // never serialize in-memory engine continuation state
	for i := range v.Players {
		p := &v.Players[i]
		p.Hand = c.cards(p.Hand)
		p.Battlefield = c.cards(p.Battlefield)
		p.Graveyard = c.cards(p.Graveyard)
		p.Exile = c.cards(p.Exile)
		p.Command = c.cards(p.Command)
	}
	for i := range v.Stack {
		s := &v.Stack[i]
		s.ID = state.ObjID(c.ref(s.ID))
		s.Source = state.ObjID(c.ref(s.Source))
		if s.Card != nil {
			card := c.card(*s.Card)
			s.Card = &card
		}
		s.Targets = append([]view.TargetView(nil), s.Targets...)
		for j := range s.Targets {
			s.Targets[j].Obj = state.ObjID(c.ref(s.Targets[j].Obj))
		}
	}
	for i := range v.Pending {
		v.Pending[i].Source = state.ObjID(c.ref(v.Pending[i].Source))
	}
	frame.Board, err = json.Marshal(v)
	return frame, err
}

func (c *Collector) introduce(e *rules.Engine, id state.ObjID) {
	if id == 0 || c.known[id] != 0 {
		return
	}
	o := e.G.Obj(id)
	if o == nil {
		return
	}
	ref := uint32(len(c.known) + 1)
	name := ""
	if o.Card != nil && len(o.Card.Faces) > 0 {
		name = o.Card.Faces[0].Name
	}
	c.known[id] = ref
	c.introduced = append(c.introduced, Identity{ID: ref, Name: name, Owner: o.Owner})
}

func (c *Collector) ref(id state.ObjID) uint32 {
	if _, ok := id.PlayerRef(); ok {
		return uint32(id)
	}
	return c.known[id]
}

func (c *Collector) card(card view.CardView) view.CardView {
	card.ID = state.ObjID(c.ref(card.ID))
	card.Token = ""
	card.AttachedTo = state.ObjID(c.ref(card.AttachedTo))
	card.BlockedBy = append([]state.ObjID(nil), card.BlockedBy...)
	for i := range card.BlockedBy {
		card.BlockedBy[i] = state.ObjID(c.ref(card.BlockedBy[i]))
	}
	return card
}

func (c *Collector) cards(cards []view.CardView) []view.CardView {
	if cards == nil {
		return nil
	}
	out := make([]view.CardView, len(cards))
	for i, card := range cards {
		out[i] = c.card(card)
	}
	return out
}
