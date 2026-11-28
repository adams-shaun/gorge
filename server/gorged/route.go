package gorged

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/server/host"
)

// deckChecker is the dirac engine's deck check; *adapter.Engine satisfies it.
type deckChecker interface{ CheckDeck([]*cards.Card) error }

// podRoute is the engine a new table runs on and, for gorge with routing on, why.
type podRoute struct{ Engine, Reason string }

// routeDeck is one seat's deck as routePod judges it.
type routeDeck struct {
	PoolID string        // non-empty for a pool deck
	Cards  []*cards.Card // a submitted deck's seated cards
}

// checkRouteFlags names a -route-pods-dirac this server cannot honour.
func (c config) checkRouteFlags() error {
	switch {
	case !c.routePodsDirac:
		return nil
	case c.diracOn():
		return errors.New("-route-pods-dirac routes a gorge server's pods; -engine dirac already plays every table on dirac")
	case c.diracCheckSets == "":
		return errors.New("-route-pods-dirac needs -dirac-check-sets: the sets a routed pod plays on")
	}
	return nil
}

// checkEngineFlags names an -engine dirac or -route-pods-dirac flag this
// server cannot honour; serve runs it before it opens the corpus.
func (c config) checkEngineFlags() error {
	if err := c.checkDiracFlags(); err != nil {
		return err
	}
	return c.checkRouteFlags()
}

// armRouting, with -route-pods-dirac, seats dirac in the host's engines and
// keeps each commander-pool deck's verdict on c.diracCheck. It runs before
// c.hostOptions and before any listener opens; c.diracPool is read-only after.
func (c *config) armRouting(reg *cards.Registry, cmdPool []string) error {
	if !c.routePodsDirac {
		return nil
	}
	c.withDirac(c.diracCheck)
	var err error
	c.diracPool, err = diracPoolVerdicts(c.diracCheck, reg, c.decks, cmdPool)
	return err
}

// routePod applies the six routing conditions in order. seatDecks is the deck
// each seat plays, in seat order (pool ids resolved through c.diracPool,
// submitted decks through check).
func (c config) routePod(check deckChecker, format host.Format, seats int, policy, pairedBot string,
	allowPaired bool, seatDecks []routeDeck) podRoute {
	gorge := func(why string) podRoute { return podRoute{Engine: gorgegame.Name, Reason: why} }
	switch {
	case !c.routePodsDirac:
		return gorge("")
	case seats != diracPodSeats || format != host.FormatCommander:
		return gorge(fmt.Sprintf("pods route to dirac only at %d seats in commander", diracPodSeats))
	case len(seatablePolicies([]string{policy})) == 0:
		return gorge(fmt.Sprintf("bot policy %q cannot be seated on dirac", policy))
	case pairedBot != "" || allowPaired:
		return gorge("a paired bot needs gorge")
	case !c.autoMana || !c.botAutoPayMana:
		return gorge("dirac has no manual mana")
	}
	for seat, d := range seatDecks {
		if err := c.seatDeckOnDirac(check, d); err != nil {
			return gorge(fmt.Sprintf("seat %d deck: %v", seat, err))
		}
	}
	return podRoute{Engine: adapter.Name}
}

// seatDeckOnDirac is why dirac does not play one seat's deck, or nil. A pool
// deck takes its startup verdict; one serve never checked does not route.
func (c config) seatDeckOnDirac(check deckChecker, d routeDeck) error {
	if d.PoolID == "" {
		return check.CheckDeck(d.Cards)
	}
	err, ok := c.diracPool[d.PoolID]
	if !ok {
		return fmt.Errorf("deck %q was not checked on dirac at startup", d.PoolID)
	}
	return err
}

// diracPoolVerdicts checks every deck of pool on e once, at startup.
func diracPoolVerdicts(e deckChecker, reg *cards.Registry, dir string, pool []string) (map[string]error, error) {
	out := make(map[string]error, len(pool))
	for _, id := range pool {
		_, d, err := deck.Load(reg, filepath.Join(dir, id+".json"))
		if err != nil {
			return nil, err
		}
		out[id] = e.CheckDeck(d)
		if out[id] != nil {
			log.Printf("route-pods-dirac: deck %s stays on gorge: %v", id, out[id])
		}
	}
	return out, nil
}

// addRoutedTable adds cfg on the engine route names. A pod dirac refuses as a
// whole falls back to gorge once, the refusal becoming the route's reason.
func (c config) addRoutedTable(r *host.Registry, cfg host.TableConfig, route podRoute) (podRoute, error) {
	cfg.Engine = c.engine
	if route.Engine == adapter.Name {
		cfg.Engine = adapter.Name
	}
	err := r.AddTable(cfg)
	if err != nil && route.Engine == adapter.Name {
		cfg.Engine = ""
		route = podRoute{Engine: gorgegame.Name, Reason: fmt.Sprintf("dirac refused the pod: %v", err)}
		err = r.AddTable(cfg)
	}
	if err != nil {
		return podRoute{}, err
	}
	if route.Engine != adapter.Name {
		route.Engine = gorgegame.Name
		if cfg.Engine != "" {
			route.Engine = cfg.Engine
		}
	}
	return route, nil
}

// seatRouteDecks is the deck each seat of want plays for routePod: a seat with
// an import id takes its submitted (seated) deck, any other its pool id.
func seatRouteDecks(want, imported []string, submitted map[int]host.Deck) []routeDeck {
	out := make([]routeDeck, len(want))
	for seat, id := range want {
		if seat < len(imported) && imported[seat] != "" {
			out[seat] = routeDeck{Cards: submitted[seat].Cards}
			continue
		}
		out[seat] = routeDeck{PoolID: id}
	}
	return out
}
