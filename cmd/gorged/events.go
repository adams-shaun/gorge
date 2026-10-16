package main

import (
	"fmt"
	"path/filepath"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/eventhost"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/host/httpapi"
	"github.com/adams-shaun/gorge/internal/draftpersona"
	"github.com/adams-shaun/gorge/state"
)

// eventhost.Host already satisfies the /api/events backend interface; the
// assertion keeps the two packages' shapes in step when either changes.
var _ httpapi.EventService = (*eventhost.Host)(nil)

// eventsWiring is what serve builds before host.New (the store whose deck
// loader the registry needs, the OnMatchEnd relay and the lazy series
// hook) for startEvents. The zero value means -events is off.
type eventsWiring struct {
	store  *eventhost.Store
	relay  *eventhost.Relay
	series *eventhost.LazySeries
}

// prepareEvents opens the event store under <dir>/events (memory when dir
// is "") and chains its deck loader in front of load, so a host deck load
// of an event deck id reads the event's saved deck file before the ordinary
// loaders see the name. With -events off it returns load unchanged and a
// zero wiring, and a server that never turned events on keeps its old deck
// loader and options exactly.
func (c config) prepareEvents(reg *cards.Registry, load func(string) (host.Deck, error)) (eventsWiring, func(string) (host.Deck, error), error) {
	if !c.events {
		return eventsWiring{}, load, nil
	}
	st, err := eventhost.OpenStore(filepath.Join(c.dir, "events"))
	if err != nil {
		return eventsWiring{}, load, err
	}
	return eventsWiring{store: st, relay: &eventhost.Relay{}, series: eventhost.NewLazySeries()},
		st.DeckLoader(reg, load), nil
}

// hook routes host.Options.OnMatchEnd through the wiring's relay, keeping
// any hook already set as the relay's Next. A no-op for a zero wiring, so
// a server with -events off keeps its OnMatchEnd untouched.
func (w eventsWiring) hook(o *host.Options) {
	if w.relay == nil {
		return
	}
	w.relay.Next = o.OnMatchEnd
	o.OnMatchEnd = w.relay.OnMatchEnd
	// E3-4: the series hook is the wiring's LazySeries. New resumes
	// synchronously and can start series tables, so the target cannot be
	// installed here; a call landing before startEvents' Set parks on the
	// lazy hook instead of reading no plan (which would end the table and
	// wedge the event with its GameStarted unresolved).
	if o.Series == nil {
		o.Series = w.series.Series
	}
}

// startEvents builds the eventhost — which resumes every stored event —
// and points the wiring's relay at it, so the event games' results recorded
// by host OnMatchEnd reach it. The offered policy set is the server's
// -bot-policies set filtered by eventPolicies, and the default is -bot-policy unless it reads decklists,
// else bots.Default. Returns nil when -events is off.
func (c config) startEvents(w eventsWiring, r *host.Registry, reg *cards.Registry, gate *seatGate) (*eventhost.Host, error) {
	if w.store == nil {
		return nil, nil
	}
	if gate == nil {
		return nil, fmt.Errorf("gorged: draft events need a seat gate to mint game tokens")
	}
	policies := c.eventPolicies()
	drafter, err := draftpersona.Register(c.eventDrafter)
	if err != nil {
		return nil, fmt.Errorf("-event-drafter: %w", err)
	}
	def := c.botPolicy
	if bots.ReadsDecklists(def) {
		def = bots.Default
	}
	evh, err := eventhost.New(eventhost.Options{
		Registry:       r,
		Store:          w.store,
		Cards:          reg,
		Policies:       policies,
		DefaultPolicy:  def,
		DefaultDrafter: drafter,
		MaxLiveEvents:  c.maxEvents,
		Mulligans:      c.mulligans,
		AutoMana:       c.autoMana,
		BotAutoPayMana: c.botAutoPayMana,
		BotCommonSense: c.botCommonSense,
		MintSeat: func(t host.TableID, seat int) (string, error) {
			return gate.mint(t, state.PlayerID(seat))
		},
	})
	if err != nil {
		return nil, err
	}
	w.relay.Set(evh.OnMatchEnd)
	w.series.Set(evh.Series)
	return evh, nil
}

// eventPolicies is the policy set draft events offer: -bot-policies minus
// every decklist reader (an Env policy without closed lists never seats in a
// draft event). A reserved-pool policy is offered only when it does not read
// decklists -- the closed-list az policies (az-c4, az-c5, az-c6;
// bots.Info.ClosedLists): the event deals its own drafted FDN limited decks
// and the policy plays them, guessing its opponent's list from its belief as
// at any table. az-c3, a reserved-pool decklist reader, stays out.
func (c config) eventPolicies() []string {
	policies := make([]string, 0, len(c.botPolicies))
	for _, p := range c.botPolicies {
		if !bots.ReadsDecklists(p) {
			policies = append(policies, p)
		}
	}
	return policies
}
