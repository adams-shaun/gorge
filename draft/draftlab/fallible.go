package draftlab

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// FallibleDrafter is an optional pickbot.Drafter extension for a drafter
// whose answers come from outside the process and can fail (an external
// agent that dies mid-draft). runDraftLoop prefers PickErr over Pick when
// the drafter implements it; the built-in drafters never do, so their path
// is unchanged byte for byte.
type FallibleDrafter interface {
	pickbot.Drafter
	// PickErr answers one pick; an error fails the draft naming the seat,
	// round and pick.
	PickErr(v pickbot.PickView, r *rand.Rand) (string, error)
}

// FallibleBuilder is an optional pickbot.Builder extension for a builder
// whose answers come from outside the process and can fail. buildNamed
// prefers BuildErr over Build when the builder implements it; the built-in
// builders never do, so their path is unchanged byte for byte.
type FallibleBuilder interface {
	pickbot.Builder
	// BuildErr turns a finished pool into a deck; an error fails the build.
	BuildErr(set string, pool []limited.CardRef) (build.Deck, error)
}
