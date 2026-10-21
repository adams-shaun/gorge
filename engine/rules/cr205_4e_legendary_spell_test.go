package rules

// CR 205.4e: "Any instant or sorcery spell with the supertype 'legendary' is
// subject to a casting restriction. A player can't cast a legendary instant
// or sorcery spell unless that player controls a legendary creature or a
// legendary planeswalker." Real corpus cards read as script text; the board
// is built eventlessly and funded through the pool.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR205_4eLegendarySorceryNeedsALegend: Urza's Ruinous Blast ({4}{W},
// Legendary Sorcery) with {4}{W} floating in seat 0's main phase. It is NOT
// offered while seat 0 controls only a nonlegendary creature (Grizzly
// Bears), a legendary noncreature artifact (Mox Amber), and the OPPONENT
// controls a legendary creature (Krenko, Mob Boss). It IS offered once seat 0
// controls a legendary creature.
func TestCR205_4eLegendarySorceryNeedsALegend(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "u/urzas_ruinous_blast.txt")))
	blast := e.G.Zone(state.ZHand, 0)[0]
	battlePerm(t, e, 0, corpusCardText(t, "g/grizzly_bears.txt"))
	battlePerm(t, e, 0, corpusCardText(t, "m/mox_amber.txt"))
	battlePerm(t, e, 1, corpusCardText(t, "k/krenko_mob_boss.txt"))
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 5})
	if hasCastOption(e.legalActions(0), blast) {
		t.Fatalf("CR 205.4e: Urza's Ruinous Blast is offered although seat 0 controls no legendary creature or planeswalker")
	}
	if !e.castRestricted(0, blast) {
		t.Errorf("CR 205.4e: castRestricted (the effect-cast gate) admits the legendary sorcery without a legend")
	}
	battlePerm(t, e, 0, corpusCardText(t, "k/krenko_mob_boss.txt"))
	if !hasCastOption(e.legalActions(0), blast) {
		t.Fatalf("CR 205.4e: Urza's Ruinous Blast is withheld although seat 0 controls Krenko, Mob Boss")
	}
}

// TestCR205_4eNonlegendarySorceryUnaffected guards the other direction with
// no legend on the battlefield: a nonlegendary sorcery (Divination) and the
// nonlegendary Adventure half of a legendary creature card (Glóin the
// Mighty's "Easy Pickings", Types: Sorcery Adventure) stay castable.
func TestCR205_4eNonlegendarySorceryUnaffected(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "d/divination.txt")),
		card(t, corpusCardText(t, "g/gloin_the_mighty_easy_pickings.txt")))
	div, gloin := e.G.Zone(state.ZHand, 0)[0], e.G.Zone(state.ZHand, 0)[1]
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 3})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "R", Amount: 3})
	opts := e.legalActions(0)
	if !hasCastOption(opts, div) {
		t.Fatalf("a nonlegendary sorcery must stay castable with no legend")
	}
	adventure := false
	for _, o := range opts {
		if o.Kind == "cast" && o.Obj == gloin && o.Mode == "adventure_alt" {
			adventure = true
		}
	}
	if !adventure {
		t.Fatalf("the nonlegendary Adventure half Easy Pickings must stay castable with no legend: %+v", opts)
	}
}
