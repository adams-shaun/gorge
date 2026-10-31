package gorged

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
)

// Extensions are what a binary links into gorged beyond the platform. The
// zero value is the bare platform: no draft events and no review loader
// (gorge's own cmd/gorged links draft events and the feedback-snapshot review
// loader).
type Extensions struct {
	// Events, when set, serves draft events under /api/events with -events
	// (draft/gorgedevents). Without it -events is refused at startup.
	Events EventsPlugin
	// Flags, when set, registers the extensions' own flags on gorged's
	// command line (Main, Serve and ServeFlagSetFor call it on the fresh
	// flag set before parsing). The values it binds are read by the
	// extensions themselves, e.g. a plugin's Start; a flag name gorged
	// already defines panics.
	Flags func(fs *flag.FlagSet)
	// Review, when set, is the loader `gorged review` turns each input path
	// into a recorded match with (review.go). Without it review mode refuses
	// to start; the live server ignores it entirely.
	Review ReviewLoader
	// Engines are the engines this binary links beside gorge, by the name
	// -engine selects them with.
	Engines map[string]game.Engine
}

// EventsPlugin is a draft-event runtime: it owns a store under the server's
// persistence directory, may front the deck loader (an event's saved decks),
// and once the registry exists it starts the /api/events backend.
type EventsPlugin interface {
	// Prepare opens the store under dir and returns the wiring plus load,
	// wrapped so an event deck id resolves before the ordinary loaders.
	Prepare(dir string, reg *cards.Registry, load func(string) (host.Deck, error)) (EventsWiring, func(string) (host.Deck, error), error)
	// ArtNames lists card names the art prewarm should fill beyond the
	// deck files (every card of every draftable set).
	ArtNames() (label string, names []string, err error)
}

// EventsWiring is a prepared runtime: Hook installs its host callbacks
// (match results, series plans) before host.New; Start builds the backend.
type EventsWiring interface {
	Hook(o *host.Options)
	Start(env EventsEnv) (EventRuntime, error)
}

// EventsEnv is what gorged hands a starting events runtime.
type EventsEnv struct {
	Registry       *host.Registry
	Cards          *cards.Registry
	Policies       []string // the offered policy set minus decklist readers (eventPolicies)
	DefaultPolicy  string
	MaxLiveEvents  int
	Mulligans      int
	AutoMana       bool
	BotAutoPayMana bool
	BotCommonSense bool
	// MintSeat mints the game token for a human seat of an event table.
	MintSeat func(t host.TableID, seat int) (string, error)
}

// EventRuntime is the /api/events backend, closed before the registry on
// shutdown.
type EventRuntime interface {
	httpapi.EventService
	Close() error
}

var errEventsNeedPlugin = errors.New("-events needs a binary with draft events linked in (gorge's cmd/gorged links them)")

// eventsWiring is serve's handle on a prepared runtime; the zero value means
// -events is off.
type eventsWiring struct{ w EventsWiring }

// prepareEvents opens the runtime's store under <dir>/events and chains its
// deck loader in front of load. With -events off it returns load unchanged
// and a zero wiring, so a server that never turned events on keeps its old
// deck loader and options exactly.
func (c config) prepareEvents(reg *cards.Registry, load func(string) (host.Deck, error)) (eventsWiring, func(string) (host.Deck, error), error) {
	if !c.events {
		return eventsWiring{}, load, nil
	}
	if c.ext.Events == nil {
		return eventsWiring{}, load, errEventsNeedPlugin
	}
	w, l, err := c.ext.Events.Prepare(filepath.Join(c.dir, "events"), reg, load)
	if err != nil {
		return eventsWiring{}, load, err
	}
	return eventsWiring{w: w}, l, nil
}

// hook routes host callbacks through the runtime; a no-op for a zero wiring.
func (w eventsWiring) hook(o *host.Options) {
	if w.w != nil {
		w.w.Hook(o)
	}
}

// startEvents starts the runtime, which resumes every stored event. The
// offered policy set is eventPolicies, and the default is -bot-policy unless
// it reads decklists, else bots.Default. Returns nil when -events is off.
func (c config) startEvents(w eventsWiring, r *host.Registry, reg *cards.Registry, gate *seatGate) (EventRuntime, error) {
	if w.w == nil {
		return nil, nil
	}
	if gate == nil {
		return nil, fmt.Errorf("gorged: draft events need a seat gate to mint game tokens")
	}
	policies := c.eventPolicies()
	def := c.botPolicy
	if bots.ReadsDecklists(def) {
		def = bots.Default
	}
	return w.w.Start(EventsEnv{
		Registry:       r,
		Cards:          reg,
		Policies:       policies,
		DefaultPolicy:  def,
		MaxLiveEvents:  c.maxEvents,
		Mulligans:      c.mulligans,
		AutoMana:       c.autoMana,
		BotAutoPayMana: c.botAutoPayMana,
		BotCommonSense: c.botCommonSense,
		MintSeat: func(t host.TableID, seat int) (string, error) {
			return gate.mint(t, state.PlayerID(seat))
		},
	})
}

// eventPolicies is the policy set draft events offer: -bot-policies minus
// every decklist reader (bots.ReadsDecklists: an Env policy without closed
// lists never seats in a draft event). A policy with a reserved deck pool is
// offered when it does not read decklists -- the closed-list net policies
// (bots.Info.ClosedLists): the event deals its own drafted decks and the
// policy plays them, guessing its opponent's list from its belief as at any
// table. az-c3, a reserved-pool decklist reader, stays out.
func (c config) eventPolicies() []string {
	policies := make([]string, 0, len(c.botPolicies))
	for _, p := range c.botPolicies {
		if !bots.ReadsDecklists(p) {
			policies = append(policies, p)
		}
	}
	return policies
}

// prewarmExtensionArt fills the cache for the names the events runtime
// lists (every card of every draftable set), after the deck prewarm: no deck
// file names most of them, so without it a draft pack is the first request
// for its art and renders blank until the fetches settle.
func prewarmExtensionArt(ctx context.Context, ac *artCache, ext Extensions, logf func(string, ...any)) {
	if ext.Events == nil {
		return
	}
	label, names, err := ext.Events.ArtNames()
	if err != nil {
		logf("art prewarm (%s): %v", label, err)
		return
	}
	_, _ = fillNames(ctx, ac, names, label, "art prewarm ("+label+")", logf, fillLimits{})
}
