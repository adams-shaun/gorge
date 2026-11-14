package gorged

import (
	"fmt"
	"log"

	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The pod bot: azredeal's closed-list max^n search (azredeal.PolicyClosedMaxN)
// over a belief of drafted decks, for the bot opponents of a Commander pod and
// for advice at one. It needs a belief to guess an opponent's list from, so
// it is registered only when -pod-bot-decks names one, and before the offered
// policy set is resolved: once registered it is offered like any closed-list
// search -- at tables, to draft events (-events-bot-policy) and among the
// paired-bot choices, where a 4-seat table at the default gets it.

// enablePodBot loads -pod-bot-decks and registers the pod bot over it. No
// flag, no policy.
func (c config) enablePodBot(reg *cards.Registry) error {
	if c.podBotDecks == "" {
		return nil
	}
	if c.diracOn() {
		return fmt.Errorf("-pod-bot-decks: %s searches gorge's rules engine, which -engine %s does not run", azredeal.PolicyClosedMaxN, c.engine)
	}
	if c.podBotSims < 1 {
		return fmt.Errorf("-pod-bot-sims %d, want >= 1", c.podBotSims)
	}
	belief, err := loadBelief(reg, c.podBotDecks, "", "-pod-bot-decks", "-pod-bot-decks")
	if err != nil {
		return err
	}
	if err := azredeal.EnableClosedMaxN(belief, c.podBotSims); err != nil {
		return fmt.Errorf("-pod-bot-decks: %w", err)
	}
	log.Printf("bot policy %s: %d simulations a decision, guessing opponents' lists from %s", azredeal.PolicyClosedMaxN, c.podBotSims, c.podBotDecks)
	return nil
}
