package gorged

import (
	"errors"
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/server/host/httpapi"
)

// defaultDecks is -decks' default, which -engine dirac reads as its bench decks.
const defaultDecks = "testkit/testutil/decks"

// diracOn reports whether this server's tables run on dirac.
func (c config) diracOn() bool { return c.engine == adapter.Name }

// checkDiracFlags names a flag -engine dirac cannot serve.
func (c config) checkDiracFlags() error {
	if !c.diracOn() {
		return nil
	}
	switch {
	case c.seats != 2:
		return errors.New("dirac plays two seats: pass -seats 2")
	case slices.Contains(strings.Split(c.formatsRaw, ","), "commander"):
		return errors.New("-format commander: dirac plays constructed only")
	case !c.autoMana:
		return errors.New("-auto-mana=false: dirac pays for every cast and has no manual mana")
	case !c.botAutoPayMana:
		return errors.New("-bot-auto-mana=false: dirac pays for every cast and has no manual mana")
	case c.events:
		return errors.New("-events: a draft event's decks need the gorge engine")
	}
	for _, a := range c.nets {
		if a.net != "" {
			return fmt.Errorf("-%s-net: a network bot searches, and dirac deals no search roots", a.policy)
		}
	}
	return nil
}

// diracDecks is -decks under -engine dirac: left at its default, the bench decks of
// the first set.
func (c config) diracDecks() string {
	if c.decks == defaultDecks {
		first, _, _ := strings.Cut(c.diracSets, ",")
		return filepath.Join("dirac", "testdata", "bench", first)
	}
	return c.decks
}

// openDirac opens the tables of -dirac-sets. A set whose tables are not built fails
// here, naming the command that builds them.
func (c config) openDirac(reg *cards.Registry) (*adapter.Engine, error) {
	var sets []string
	for _, s := range strings.Split(c.diracSets, ",") {
		if s = strings.TrimSpace(s); s != "" {
			sets = append(sets, s)
		}
	}
	surface := dirac.SurfaceFull
	if c.sbv2 {
		surface = dirac.SurfaceAutopay // what the v2 wire poses, which casts by engine auto-pay
	}
	return adapter.New(sets, reg, surface)
}

// checkDecks splits the deck pool into what the engine plays and why it refuses the
// rest. A pool with nothing to play is a startup error, naming the first refusals.
func checkDecks(e *adapter.Engine, reg *cards.Registry, dir string, pool []string) ([]string, map[string]string, error) {
	var playable []string
	refused := map[string]string{}
	for _, id := range pool {
		_, d, err := deck.Load(reg, filepath.Join(dir, id+".json"))
		if err != nil {
			return nil, nil, err
		}
		if err := e.CheckDeck(d); err != nil {
			refused[id] = err.Error()
			log.Printf("dirac: deck %s refused: %v", id, err)
			continue
		}
		playable = append(playable, id)
	}
	if len(playable) > 0 {
		return playable, refused, nil
	}
	ids := slices.Sorted(maps.Keys(refused))
	var first []string
	for _, id := range ids[:min(3, len(ids))] {
		first = append(first, id+": "+refused[id])
	}
	return nil, nil, fmt.Errorf("-engine dirac: no deck in %s is playable; for example %s", dir, strings.Join(first, "; "))
}

// rotationDecks is the pool a bot table's rotation can play: match k seats decks k and
// k+1, so each deck kept can follow the one before it, and the last can come before
// the first. A deck that cannot is skipped, with a log line; a pool with no legal pair
// is a startup error.
func rotationDecks(pool []string, accepts func(a, b string) error) ([]string, error) {
	var kept []string
	for _, id := range pool {
		if len(kept) > 0 {
			if err := accepts(kept[len(kept)-1], id); err != nil {
				log.Printf("dirac: deck %s skipped in the rotation after %s: %v", id, kept[len(kept)-1], err)
				continue
			}
		}
		kept = append(kept, id)
	}
	for len(kept) > 1 {
		last := kept[len(kept)-1]
		err := accepts(last, kept[0])
		if err == nil {
			return kept, nil
		}
		log.Printf("dirac: deck %s skipped in the rotation before %s: %v", last, kept[0], err)
		kept = kept[:len(kept)-1]
	}
	if err := accepts(kept[0], kept[0]); err != nil {
		return nil, fmt.Errorf("-engine dirac: no legal pair of decks to rotate: %w", err)
	}
	return kept, nil
}

// pairAccepts says whether e plays two of dir's decks against each other.
func pairAccepts(e *adapter.Engine, reg *cards.Registry, dir string) func(a, b string) error {
	return func(a, b string) error {
		_, da, err := deck.Load(reg, filepath.Join(dir, a+".json"))
		if err != nil {
			return err
		}
		_, db, err := deck.Load(reg, filepath.Join(dir, b+".json"))
		if err != nil {
			return err
		}
		return e.Accepts(game.Config{Decks: [][]*cards.Card{da, db}})
	}
}

// markRefused writes each refused deck's reason into the catalogue.
func markRefused(catalogue []httpapi.DeckInfo, refused map[string]string) {
	for i := range catalogue {
		catalogue[i].Refused = refused[catalogue[i].ID]
	}
}

// seatablePolicies is what dirac can seat of the offered bot policies: a search needs
// a root, and a decklist reader a list dirac does not keep.
func seatablePolicies(offered []string) []string {
	var out []string
	for _, name := range offered {
		if e, ok := bots.Lookup(name); !ok || !e.Search && !e.ReadsDecklists() {
			out = append(out, name)
		}
	}
	return out
}

// withDirac is the engines of a server whose tables run on e.
func (c *config) withDirac(e *adapter.Engine) {
	engines := maps.Clone(c.ext.Engines)
	if engines == nil {
		engines = map[string]game.Engine{}
	}
	engines[adapter.Name] = e
	c.ext.Engines = engines
}
