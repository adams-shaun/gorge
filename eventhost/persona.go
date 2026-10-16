package eventhost

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/limited/pickbot"
)

// pickbotCardLookup is the card-facts lookup a persona reads through; it
// is pickbot.CardLookup spelled locally so host.go's signature stays
// short.
type pickbotCardLookup = pickbot.CardLookup

// pickbotDrafter and pickbotBuilder are the two half-personas a bot seat
// (and a human's suggestion builder) is built from.
type (
	pickbotDrafter = pickbot.Drafter
	pickbotBuilder = pickbot.Builder
)

// newPersona builds one fresh drafter/builder pair for a named persona
// through the registry's constructors, with the caller's Lookup injected so
// no persona shares a card-facts closure with another. Any registered pair
// is accepted: the stock rating-colour/best23, or a trained drafter a
// server registered at startup (cmd/gorged -event-drafter). Every pickbot
// Drafter answers from the seat's own PickView alone.
func (h *Host) newPersona(name string, lookup pickbotCardLookup) (pickbotDrafter, pickbotBuilder, error) {
	d, b, ok := pickbot.New(name, lookup)
	if !ok {
		return nil, nil, fmt.Errorf("unknown pickbot persona %q (known: %s)", name, strings.Join(pickbot.Names(), ", "))
	}
	if d == nil || b == nil {
		return nil, nil, fmt.Errorf("persona %q has no drafter or builder", name)
	}
	return d, b, nil
}
