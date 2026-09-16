package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/cards"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestDbgWardMatch(t *testing.T) {
	e, bear := hexingEngine(t)
	bolt := e.G.Zone(state.ZHand, 1)[0]
	d := e.Pending()
	for _, o := range d.Options {
		if o.Kind == "pass" {
			submitChoices(t, e, o.Index)
			break
		}
	}
	d = e.Pending()
	opt := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == bolt {
			opt = o.Index
		}
	}
	submitChoices(t, e, opt)
	dt := e.Pending()
	idx := -1
	for _, o := range dt.Options {
		if o.Obj == bear {
			idx = o.Index
		}
	}
	fmt.Printf("target idx=%d of %d\n", idx, len(dt.Options))
	submitChoices(t, e, idx)
	// Manually test the matcher
	f := e.G.Obj(bear).Face()
	_ = f
	tt := cards.Trigger{Mode: "BecomesTarget",
		Params: map[string]string{"ValidTarget": "Card.Self", "Ward": "True", "TriggerDescription": "Ward"},
		Effect: nil}
	ev := events.Event{Kind: events.TargetsChosen, Obj: 0}
	for _, x := range e.L.Events {
		if x.Kind == events.TargetsChosen {
			ev = x
		}
	}
	fmt.Printf("targets_chosen ev=%+v bear=%d\n", ev, bear)
	fmt.Printf("matches=%v kw=%v\n", e.triggerMatches(tt, bear, ev, nil), e.Derived(bear).Keywords)
	fmt.Printf("pendingTriggers=%d\n", len(e.pendingTriggers))
}
