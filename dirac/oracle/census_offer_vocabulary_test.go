package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestCensusOfferReadsAsNoOfferedPlay pins the oracle's gorge side against the
// priority menu's census-gap arm (engine/rules autoPayCensus): the arm's
// "ability" offer must not enter the offered-options pass, or add()'s dedupe
// would drop the canonical pricing's script route for the same activation and
// the oracle would submit the bare option, whose route is a manual payment
// window. Every other option keeps its own kind.
func TestCensusOfferReadsAsNoOfferedPlay(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		o    decision.Option
		want string
	}{
		{decision.Option{Kind: "ability", CensusOffer: true}, censusOfferKind},
		{decision.Option{Kind: "ability"}, "ability"},
		{decision.Option{Kind: "cast", CensusOffer: true}, "cast"},
		{decision.Option{Kind: "pass"}, "pass"},
	} {
		if got := priorityOptionKind(&c.o); got != c.want {
			t.Errorf("priorityOptionKind(%+v) = %q, want %q", c.o, got, c.want)
		}
	}
}
