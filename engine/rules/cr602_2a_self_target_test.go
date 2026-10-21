package rules

// CR 115.5: "A spell or ability on the stack can't target itself." An
// activation is pushed before its targets are chosen (CR 602.2a), so the
// pushed object is on the stack at the target ask and must not be offered.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestCR602_2aActivatedAbilityIsNotItsOwnTarget: Azorius Guildmage's
// "{2}{U}: Counter target activated ability" is activated over a Prodigal
// Pyromancer ability already on the stack. The target ask offers the
// Pyromancer's ability and never the Guildmage's own just-pushed ability.
func TestCR602_2aActivatedAbilityIsNotItsOwnTarget(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	pyro := battlePerm(t, e, 0, corpusCardText(t, "p/prodigal_pyromancer.txt"))
	guild := battlePerm(t, e, 0, corpusCardText(t, "a/azorius_guildmage.txt"))
	e.askPriority(0)
	cr601Choose(t, e, "activate pyromancer", func(o decision.Option) bool { return o.Kind == "ability" && o.Obj == pyro })
	cr601Choose(t, e, "pyromancer target", func(o decision.Option) bool { return o.Kind == "player" && o.Label == "b" })
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack %v, want the Pyromancer's ability alone", e.G.Stack)
	}
	pyroAb := e.G.Stack[0]
	for i := 0; i < 3; i++ {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
	}
	e.pending = nil
	e.askPriority(0)
	cr601Choose(t, e, "activate guildmage", func(o decision.Option) bool {
		return o.Kind == "ability" && o.Obj == guild && o.Ability == 1
	})
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("want the Guildmage's target ask, got %+v", d)
	}
	if len(e.G.Stack) != 2 {
		t.Fatalf("precondition: stack %v, want the Guildmage's ability pushed over the Pyromancer's", e.G.Stack)
	}
	own := e.G.Stack[1]
	offered := false
	for _, o := range d.Options {
		if o.Obj == own {
			t.Errorf("CR 115.5: the ability's own object %d is offered as its target", own)
		}
		if o.Obj == pyroAb {
			offered = true
		}
	}
	if !offered {
		t.Errorf("the Pyromancer's ability %d is not offered: %+v", pyroAb, d.Options)
	}
}
