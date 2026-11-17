package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// sylexEngine puts the real corpus The Filigree Sylex on seat 0's battlefield
// carrying oil oil counters. Its third ability is
// "{T}, Remove ten oil counters from among permanents you control and
// sacrifice The Filigree Sylex: It deals 10 damage to any target."
func sylexEngine(t *testing.T, oil int32) (e *Engine, sylex state.ObjID) {
	t.Helper()
	e = handEngine(t,
		corpusAlternativeCard(t, "The Filigree Sylex"),
		corpusAlternativeCard(t, "Grizzly Bears"),
	)
	sylex = e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, sylex)
	e.emit(events.Event{Kind: events.CounterChange, Obj: sylex, Counter: "OIL", Amount: oil})
	e.pending = nil
	toMain1(t, e)
	e.priorityRound()
	if o := e.G.Obj(sylex); o.Zone != state.ZBattlefield || o.Counter("OIL") != oil {
		t.Fatalf("setup: Sylex zone %v oil %d, want battlefield with %d", o.Zone, o.Counter("OIL"), oil)
	}
	return e, sylex
}

// sylexOptions lists the priority options that activate the Sylex.
func sylexOptions(e *Engine, sylex state.ObjID) []int {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return nil
	}
	var out []int
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == sylex {
			out = append(out, o.Index)
		}
	}
	return out
}

// TestSylexRemovesItsOwnCountersBeforeItsSacrifice: the ten oil counters come
// from among permanents its controller controls, and the Sylex itself is the
// only permanent that can carry them. Costs are paid in any order (CR 118.3,
// 601.2h), so removing the counters from the Sylex and then sacrificing it is
// a legal payment: with ten counters all three of its abilities are offered,
// with nine only the first two are.
func TestSylexRemovesItsOwnCountersBeforeItsSacrifice(t *testing.T) {
	t.Parallel()
	e9, s9 := sylexEngine(t, 9)
	if got := len(sylexOptions(e9, s9)); got != 2 {
		t.Fatalf("precondition: Sylex with nine oil counters offers %d abilities, want 2 (counter, destroy)", got)
	}
	e, sylex := sylexEngine(t, 10)
	opts := sylexOptions(e, sylex)
	if len(opts) != 3 {
		t.Fatalf("Sylex with ten oil counters offers %d abilities, want 3 (counter, destroy, ten damage): pending=%+v", len(opts), e.Pending())
	}
}

// TestSylexTenDamagePaysFromItsOwnCounters activates the offered ability and
// settles it: the Sylex is sacrificed, its counters are gone, and the chosen
// opponent takes ten.
func TestSylexTenDamagePaysFromItsOwnCounters(t *testing.T) {
	t.Parallel()
	e, sylex := sylexEngine(t, 10)
	opts := sylexOptions(e, sylex)
	if len(opts) != 3 {
		t.Fatalf("precondition: Sylex offers %d abilities, want 3", len(opts))
	}
	life := e.G.Players[1].Life
	submitChoices(t, e, opts[2])
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		pick := d.Options[0].Index
		for _, o := range d.Options {
			if o.Player == 1 && o.Obj == 0 {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)
	}
	passUntilStackEmpty(t, e, 30)
	if z := e.G.Obj(sylex).Zone; z != state.ZGraveyard {
		t.Fatalf("Sylex zone after the activation = %v, want graveyard", z)
	}
	if got := e.G.Players[1].Life; got != life-10 {
		t.Fatalf("opponent life = %d, want %d", got, life-10)
	}
}
