// Package gorgedevents plugs the draft-event runtime (draft/eventhost) into
// gorge's reference server through gorged.EventsPlugin: a binary that links
// it passes Plugin{} in gorged.Extensions.Events, and -events then
// serves 4-seat draft events under /api/events with persistence under
// <dir>/events.
package gorgedevents

import (
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/draft/eventhost"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/gorged"
	"github.com/adams-shaun/gorge/server/host"
)

// eventhost.Host is the /api/events backend gorged closes on shutdown.
var _ gorged.EventRuntime = (*eventhost.Host)(nil)

// Plugin is the draft-event runtime.
type Plugin struct{}

// Prepare opens the event store under dir (memory when dir is "") and chains
// its deck loader in front of load, so a host deck load of an event deck id
// reads the event's saved deck file before the ordinary loaders see the name.
func (Plugin) Prepare(dir string, reg *cards.Registry, load func(string) (host.Deck, error)) (gorged.EventsWiring, func(string) (host.Deck, error), error) {
	st, err := eventhost.OpenStore(dir)
	if err != nil {
		return nil, load, err
	}
	return &wiring{store: st, relay: &eventhost.Relay{}, series: eventhost.NewLazySeries()}, st.DeckLoader(reg, load), nil
}

// ArtNames is every card name on any sheet of any collation of every
// embedded set, sorted and de-duplicated: no deck file names most of a set's
// cards, so without it a draft pack is the first request for its art.
func (Plugin) ArtNames() (string, []string, error) {
	label := "draft sets " + strings.Join(setdata.Codes(), ",")
	seen := make(map[string]bool)
	for _, code := range setdata.Codes() {
		for _, c := range setdata.Collations(code) {
			spec, err := setdata.LoadCollation(code, c)
			if err != nil {
				return label, nil, fmt.Errorf("set %s collation %s: %w", code, c, err)
			}
			for _, sh := range spec.Config.Sheets {
				for _, sc := range sh.Cards {
					seen[sc.Name] = true
				}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return label, names, nil
}

// wiring is what Prepare builds before host.New: the store whose deck loader
// the registry needs, the OnMatchEnd relay and the lazy series hook.
type wiring struct {
	store  *eventhost.Store
	relay  *eventhost.Relay
	series *eventhost.LazySeries
}

// Hook routes host.Options.OnMatchEnd through the relay, keeping any hook
// already set as the relay's Next. The series hook is the lazy series: New
// resumes synchronously and can start series tables, so the target cannot
// be installed here; a call landing before Start's Set parks on the lazy
// hook instead of reading no plan (which would end the table and wedge the
// event with its GameStarted unresolved).
func (w *wiring) Hook(o *host.Options) {
	w.relay.Next = o.OnMatchEnd
	o.OnMatchEnd = w.relay.OnMatchEnd
	if o.Series == nil {
		o.Series = w.series.Series
	}
}

// Start builds the eventhost, which resumes every stored event, and points
// the relay and series hooks at it.
func (w *wiring) Start(env gorged.EventsEnv) (gorged.EventRuntime, error) {
	evh, err := eventhost.New(eventhost.Options{
		Registry:       env.Registry,
		Store:          w.store,
		Cards:          env.Cards,
		Policies:       env.Policies,
		DefaultPolicy:  env.DefaultPolicy,
		MaxLiveEvents:  env.MaxLiveEvents,
		Mulligans:      env.Mulligans,
		AutoMana:       env.AutoMana,
		BotAutoPayMana: env.BotAutoPayMana,
		BotCommonSense: env.BotCommonSense,
		MintSeat:       env.MintSeat,
	})
	if err != nil {
		return nil, err
	}
	w.relay.Set(evh.OnMatchEnd)
	w.series.Set(evh.Series)
	return evh, nil
}
