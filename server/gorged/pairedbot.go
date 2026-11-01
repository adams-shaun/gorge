package gorged

import (
	"fmt"

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

// pairedBotEntry resolves -paired-bot: nil when it is off, the built-in search
// over a belief, or a registered policy. The host checks the entry's seats.
// conPool is what the built-in search believes opponents play when
// -paired-bot-decks is not given: the constructed decks this server deals.
func (c config) pairedBotEntry(reg *cards.Registry, load func(string) (host.Deck, error), conPool []string) (*bots.Entry, error) {
	if c.pairedBot == "" {
		return nil, nil
	}
	if c.engine != gorgegame.Name {
		return nil, fmt.Errorf("-paired-bot needs -engine %s: it searches gorge's rules engine", gorgegame.Name)
	}
	if c.pairedBotSearches < 1 {
		return nil, fmt.Errorf("-paired-bot-searches %d, want >= 1", c.pairedBotSearches)
	}
	if c.pairedBot != pairedBotBuiltin {
		e, ok := bots.Lookup(c.pairedBot)
		if !ok {
			return nil, fmt.Errorf("-paired-bot: unknown bot %q (%s, or a registered closed-list search policy)", c.pairedBot, pairedBotBuiltin)
		}
		// bots.New fills the policy's declared option defaults; its factory
		// alone would not.
		name := c.pairedBot
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
