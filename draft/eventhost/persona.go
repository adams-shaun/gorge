package eventhost

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
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

// newPersona builds one fresh drafter/builder pair for a named persona.
// Only the concrete v1 types the registry ships are supported: the pair is
// rebuilt with the caller's Lookup so no persona ever shares a card-facts
// closure with the registry's zero values.
func (h *Host) newPersona(name string, lookup pickbotCardLookup) (pickbotDrafter, pickbotBuilder, error) {
	d, b, ok := pickbot.Lookup(name)
	if !ok {
		return nil, nil, fmt.Errorf("unknown pickbot persona %q (known: %s)", name, strings.Join(pickbot.Names(), ", "))
	}
	switch d.(type) {
	case *pickbot.RatingColour:
	default:
		return nil, nil, fmt.Errorf("persona %q drafts with unsupported type %T", name, d)
	}
	switch b.(type) {
	case *pickbot.Best23:
	default:
		return nil, nil, fmt.Errorf("persona %q builds with unsupported type %T", name, b)
	}
	return &pickbot.RatingColour{Lookup: lookup}, &pickbot.Best23{Lookup: lookup}, nil
}
