package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Spectacle (CR 702.137a): the hand offers the spectacle cast for the
// keyword's cost only while an opponent of the caster lost life this turn.
func TestSpectacleOfferWaitsOnAnOpponentLosingLife(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 931, []string{"Skewer the Critics"}, nil, nil)
	id := findCardObj(t, e, 0, "Skewer the Critics", state.ZHand)
	addMana(t, e, 0, "R")
	if modes := castModeOptions(e.Pending().Options, id); len(modes) != 0 {
		t.Fatalf("one red mana offers %v before any life was lost; want nothing (the plain cast costs three)", modes)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -3})
	e.priorityRound()
	modes := castModeOptions(e.Pending().Options, id)
	if len(modes) != 1 || modes[0] != "spectacle" {
		t.Fatalf("after the opponent lost life the cast options are %v, want [spectacle]", modes)
	}
}

// The spectacle cast stamps the provenance the `spectacle` filter word reads:
// Rafter Demon's "if its spectacle cost was paid" discard fires on the
// spectacle cast only.
func TestSpectacleProvenanceReadsOnTheEntrant(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 932, []string{"Rafter Demon"}, nil, nil)
	id := findCardObj(t, e, 0, "Rafter Demon", state.ZHand)
	addMana(t, e, 0, "RRRBR")
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	e.priorityRound()
	submitChoices(t, e, castModeOption(t, e, id, "spectacle"))
	var d *decision.Decision
	for i := 0; i < 10; i++ {
		if d = e.Pending(); d == nil || d.Kind != decision.KPriority {
			break
		}
		passOnceP(t, e)
	}
	if d == nil || d.Kind != decision.KModes || d.Player != 1 || d.Options[0].Kind != "discard" {
		t.Fatalf("pending decision %+v, want the opponent's discard from the spectacle ETB", d)
	}
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.CastFlags&state.FlagAltCostPaid == 0 {
		t.Fatalf("Rafter Demon zone %s flags %d, want the battlefield with the alt-cost provenance", o.Zone, o.CastFlags)
	}
}

// The plain cast of the same card poses no discard: its spectacle cost was not
// paid.
func TestSpectacleWordIsFalseOnAPlainCast(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 933, []string{"Rafter Demon"}, nil, nil)
	id := findCardObj(t, e, 0, "Rafter Demon", state.ZHand)
	addMana(t, e, 0, "RRBR")
	submitChoices(t, e, castModeOption(t, e, id, ""))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.CastFlags&state.FlagAltCostPaid != 0 {
		t.Fatalf("plain Rafter Demon zone %s flags %d, want the battlefield with no alt-cost provenance", o.Zone, o.CastFlags)
	}
}
