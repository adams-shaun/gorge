package effects

import "github.com/adams-shaun/gorge/engine/cards"

// DB$ Recover's resolution is engine/rules/recover.go (the CR 702.59
// pay-or-exile election); the registry arm is the inert placeholder the
// Echo precedent uses: a body that reaches Resolve outside the rules flow
// must not panic on a missing primitive.
func init() { Register("Recover", func(Host, *Ctx, *cards.SA) {}) }
