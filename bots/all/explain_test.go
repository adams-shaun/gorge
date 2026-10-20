package all_test

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
)

// unwrapper is the wrapper convention the registry's adapters share
// (azredeal's hostedSeat, sanity's guard, sb-tactical's adapter).
type unwrapper interface{ UnwrapSeat() seat.Seat }

// chain is s and every seat it wraps, outermost first.
func chain(s seat.Seat) []seat.Seat {
	out := []seat.Seat{s}
	for len(out) < 16 {
		u, ok := out[len(out)-1].(unwrapper)
		if !ok {
			break
		}
		in := u.UnwrapSeat()
		if in == nil {
			break
		}
		out = append(out, in)
	}
	return out
}

// TestRegisteredSearchAndNetBotsExplain is the decision overlay's
// enforcement test (match-reviewer design D6), modelled on
// TestRegisteredBotsConsumePaymentPlans. The predicate, stated honestly:
// a registered policy "searches or scores with a net" when its entry
// declares Info.Search, or when any seat in its wrapper chain is a
// searchseat.SearchSeat (the driver hands it an engine to search), a
// *heuristic.PolicyNetBot (the scoring net), or already an explain.Emitter.
// Such a policy's OUTERMOST seat must implement explain.Emitter, so a
// driver's SetExplain reaches the searcher through every wrapper. A plain
// heuristic policy needs none: the driver notes it as heuristic.
func TestRegisteredSearchAndNetBotsExplain(t *testing.T) {
	checked := 0
	for _, e := range bots.Entries() {
		name := e.Info.Name
		s, err := bots.New(name, bots.Options{Seed: 1, Deps: bots.Deps{Cards: &cards.Registry{}}})
		if err != nil {
			t.Errorf("%s: build: %v", name, err)
			continue
		}
		why := ""
		if e.Info.Search {
			why = "Info.Search"
		}
		for _, in := range chain(s) {
			switch in.(type) {
			case searchseat.SearchSeat:
				why = "wraps a search seat"
			case *heuristic.PolicyNetBot:
				why = "wraps a scoring net"
			case explain.Emitter:
				why = "wraps an emitter"
			}
		}
		if why == "" {
			t.Logf("%s (%T): neither searches nor scores; the driver notes it heuristic", name, s)
			continue
		}
		checked++
		if _, ok := s.(explain.Emitter); !ok {
			t.Errorf("%s (%T) %s but does not implement explain.Emitter; forward SetExplain to what it wraps", name, s, why)
		}
	}
	if checked == 0 {
		t.Fatal("no registered policy searches or scores: the enforcement test checked nothing (is az-redeal still linked?)")
	}
}
