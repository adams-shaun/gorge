package gorged

import (
	"fmt"
	"log"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/host"
)

// pairedBotBuiltin is the -paired-bot name of the search that needs no trained
// network: az-redeal's search, closed under a belief. It is not a registered
// policy, because no table may seat it.
const pairedBotBuiltin = "az-closed"

// pairedBotBuiltinCommander is the same search over a belief of the server's
// Commander decks, for 2-seat Commander tables.
const pairedBotBuiltinCommander = "az-closed-commander"

// pairedBotEntry resolves -paired-bot: nil when it is off (empty, or "off",
// which a deploy's environment can spell), the built-in search over a belief,
// or a registered policy. The host checks the entry's seats. conPool is what
// the built-in search believes opponents play when -paired-bot-decks is not
// given: the constructed decks this server deals.
func (c config) pairedBotEntry(reg *cards.Registry, load func(string) (host.Deck, error), conPool, cmdPool []string) (*bots.Entry, error) {
	if c.pairedBot == "" || c.pairedBot == host.PairedBotOff {
		return nil, nil
	}
	if c.engine != gorgegame.Name {
		return nil, fmt.Errorf("-paired-bot needs -engine %s: it searches gorge's rules engine", gorgegame.Name)
	}
	if c.pairedBotSearches < 1 {
		return nil, fmt.Errorf("-paired-bot-searches %d, want >= 1", c.pairedBotSearches)
	}
	return c.pairedBotNamed(c.pairedBot, reg, load, conPool, cmdPool)
}

// pairedBotChoices lists what a table may choose besides -paired-bot: the
// built-in search and every registered closed-list search policy. A server
// with no -paired-bot offers none, since searching is its operator's to allow.
// A choice that cannot be built is logged and left out, because the operator
// did not ask for it by name.
func (c config) pairedBotChoices(o host.Options, reg *cards.Registry, load func(string) (host.Deck, error), conPool, cmdPool []string) []*bots.Entry {
	if o.PairedBot == nil {
		return nil
	}
	names := []string{pairedBotBuiltin}
	if len(cmdPool) > 0 {
		names = append(names, pairedBotBuiltinCommander)
	}
	for _, e := range bots.Entries() {
		if e.ClosedLists && e.Search {
			names = append(names, e.Name)
		}
	}
	var out []*bots.Entry
	for _, name := range names {
		if name == o.PairedBot.Name {
			continue
		}
		e, err := c.pairedBotNamed(name, reg, load, conPool, cmdPool)
		if err == nil {
			err = host.ValidatePairedBot(*e, o)
		}
		if err != nil {
			log.Printf("paired bot %s is not offered: %v", name, err)
			continue
		}
		out = append(out, e)
	}
	return out
}

// pairedBotNamed builds the entry for a paired bot's name.
func (c config) pairedBotNamed(name string, reg *cards.Registry, load func(string) (host.Deck, error), conPool, cmdPool []string) (*bots.Entry, error) {
	if name == pairedBotBuiltinCommander {
		if c.pairedBotSims < 1 {
			return nil, fmt.Errorf("-paired-bot-sims %d, want >= 1", c.pairedBotSims)
		}
		belief, err := pairedBotCommanderBelief(load, cmdPool)
		if err != nil {
			return nil, err
		}
		return &bots.Entry{
			Info: bots.Info{Name: pairedBotBuiltinCommander, Label: "az-closed (search, Commander)", Tier: bots.Experimental,
				Env: true, Search: true, ClosedLists: true, Formats: []string{"commander"}, MaxSeats: 2},
			New: azredeal.ClosedFactory(belief, c.pairedBotSims),
		}, nil
	}
	if name != pairedBotBuiltin {
		e, ok := bots.Lookup(name)
		if !ok {
			return nil, fmt.Errorf("-paired-bot: unknown bot %q (%s, %s, or a registered closed-list search policy)", name, pairedBotBuiltin, pairedBotBuiltinCommander)
		}
		// bots.New fills the policy's declared option defaults; its factory
		// alone would not.
		e.New = func(o bots.Options) (seat.Seat, error) { return bots.New(name, o) }
		return &e, nil
	}
	if c.pairedBotSims < 1 {
		return nil, fmt.Errorf("-paired-bot-sims %d, want >= 1", c.pairedBotSims)
	}
	belief, err := c.pairedBotBelief(reg, load, conPool)
	if err != nil {
		return nil, err
	}
	return &bots.Entry{
		Info: bots.Info{Name: pairedBotBuiltin, Label: "az-closed (search)", Tier: bots.Experimental,
			Env: true, Search: true, ClosedLists: true, Formats: []string{"constructed"}, MaxSeats: 2},
		New: azredeal.ClosedFactory(belief, c.pairedBotSims),
	}, nil
}

func (c config) pairedBotBelief(reg *cards.Registry, load func(string) (host.Deck, error), conPool []string) (*searchprobe.Belief, error) {
	if c.pairedBotDecks != "" {
		return loadBelief(reg, c.pairedBotDecks, "", "-paired-bot-decks", "-paired-bot-decks")
	}
	var decks [][]*cards.Card
	for _, name := range conPool {
		d, err := load(name)
		if err != nil {
			return nil, fmt.Errorf("-paired-bot: deck %s: %w", name, err)
		}
		decks = append(decks, d.Cards)
	}
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		return nil, fmt.Errorf("-paired-bot: -decks holds no constructed deck to guess from: %w", err)
	}
	return b, nil
}

// pairedBotCommanderBelief is the cards of every deck in the server's Commander
// pool: what the Commander search believes an unseen opponent plays.
func pairedBotCommanderBelief(load func(string) (host.Deck, error), cmdPool []string) (*searchprobe.Belief, error) {
	if len(cmdPool) == 0 {
		return nil, fmt.Errorf("-paired-bot %s: this server deals no Commander deck", pairedBotBuiltinCommander)
	}
	var decks [][]*cards.Card
	for _, name := range cmdPool {
		d, err := load(name)
		if err != nil {
			return nil, fmt.Errorf("-paired-bot: deck %s: %w", name, err)
		}
		decks = append(decks, d.Cards)
	}
	b, err := searchprobe.NewBelief(decks)
	if err != nil {
		return nil, fmt.Errorf("-paired-bot %s: no Commander deck to guess from: %w", pairedBotBuiltinCommander, err)
	}
	return b, nil
}
