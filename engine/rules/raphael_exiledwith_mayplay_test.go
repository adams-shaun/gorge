package rules

// raphael_exiledwith_mayplay_test.go pins the Effect-delivered spelling of
// exile provenance: Raphael, Most Attitude names Card.ExiledWithEffectSource
// ONLY in its attack trigger's MayPlay static (an SVar). Its Alliance Dig
// must still record "exiled with Raphael", or the attack trigger's "until end
// of turn, you may play a card exiled with Raphael" never offers anything.
//
// The link has to outlive Raphael himself: exiledBySourceAt keeps the reverse
// link live after the source's single battlefield departure, so the grant
// still finds the card after Raphael dies. One MayPlayLimit$ grant is one
// play (CR 607.2a; the grant's own cap).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// raphaelInZone fails unless exactly one Raphael, Most Attitude is in zone
// for seat -- the precondition every provenance assertion reads.
func raphaelInZone(t *testing.T, e *Engine, zone state.Zone, seat state.PlayerID) *state.Object {
	t.Helper()
	var found *state.Object
	for _, id := range e.G.Zone(zone, seat) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Raphael, Most Attitude" {
			if found != nil {
				t.Fatalf("precondition: two Raphael in %v", zone)
			}
			found = o
		}
	}
	if found == nil {
		t.Fatalf("precondition: Raphael is not in %v", zone)
	}
	return found
}

// TestRaphaelAllianceExileLinksToRaphael: the Alliance trigger's Dig exiles
// the top card; the filter the attack trigger's static reads must match that
// card from Raphael.
func TestRaphaelAllianceExileLinksToRaphael(t *testing.T) {
	t.Parallel()
	run := runScenarioOrFail(t, oracleScenario{
		Name: "raphael-alliance-exile-link",
		CR:   []string{"607.2a"},
		Why:  "inline",
		Setup: map[string]oracleSeat{
			"p0": {
				Battlefield: []string{"Raphael, Most Attitude", "Forest", "Forest"},
				Hand:        []string{"Grizzly Bears"},
				LibraryTop:  []string{"Savannah Lions"},
			},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Grizzly Bears", Mana: "GG"},
			{Op: "resolve"},
			{Op: "resolve"},
		},
	})
	e := run.e
	raphael := raphaelInZone(t, e, state.ZBattlefield, 0)
	// digExileNamed asserts the precondition: the Dig really moved the card
	// to exile, where the filter looks.
	lions := digExileNamed(t, e, "Savannah Lions")
	if lions.ExiledWith != raphael.ID {
		t.Errorf("Savannah Lions ExiledWith = %d, want Raphael %d", lions.ExiledWith, raphael.ID)
	}
	if !effects.MatchesSpecFrom(e.G, "Card.ExiledWithEffectSource", lions.ID, 0, raphael.ID) {
		t.Errorf("Savannah Lions does not match Card.ExiledWithEffectSource from Raphael")
	}
}

// TestRaphaelGrantOutlivesRaphaelUntilEOT: after Raphael attacks and then
// dies, the exiled card is still offered for its controller to cast this
// turn. At the next turn's main phase the until-end-of-turn grant is gone.
func TestRaphaelGrantOutlivesRaphaelUntilEOT(t *testing.T) {
	t.Parallel()
	run := runScenarioOrFail(t, oracleScenario{
		Name: "raphael-grant-outlives-raphael",
		CR:   []string{"607.2a", "611.2"},
		Why:  "inline",
		Setup: map[string]oracleSeat{
			"p0": {
				Battlefield: []string{"Raphael, Most Attitude", "Forest", "Forest"},
				Hand:        []string{"Grizzly Bears"},
				LibraryTop:  []string{"Savannah Lions"},
			},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Grizzly Bears", Mana: "GG"},
			{Op: "resolve"},
			{Op: "attack", Seat: 0, Attackers: []string{"p0:Raphael, Most Attitude"}, Defender: "p1"},
			{Op: "resolve"},
			{Op: "move", Card: "p0:Raphael, Most Attitude", To: "graveyard"},
			{Op: "pass_to", Step: "main2", Active: "p0"},
			// The offer is priced against the floating pool, so float the
			// White and the generic the exiled Savannah Lions needs: the
			// assertion is that the grant is live after Raphael left, not
			// that the card is unaffordable.
			{Op: "mana", Seat: 0, Mana: "WG", Expect: []oracleExpect{
				{Offered: &oracleOffered{Seat: 0, Kind: "cast", Card: "p0:Savannah Lions"}, Want: boolp(true)},
			}},
			{Op: "pass_to", Step: "main1", Active: "p0"},
			// Next turn the UntilEOT grant is gone; float the same mana so
			// the false result is the expired grant, not an empty pool.
			{Op: "mana", Seat: 0, Mana: "WG", Expect: []oracleExpect{
				{Offered: &oracleOffered{Seat: 0, Kind: "cast", Card: "p0:Savannah Lions"}, Want: boolp(false)},
			}},
		},
	})
	e := run.e
	// The grant lived to main2, so it was still live while Raphael sat in the
	// graveyard: the reverse link, not a live battlefield source, carried it.
	raphael := raphaelInZone(t, e, state.ZGraveyard, 0)
	lions := digExileNamed(t, e, "Savannah Lions")
	if lions.ExiledWith != raphael.ID {
		t.Errorf("Savannah Lions ExiledWith = %d, want Raphael %d", lions.ExiledWith, raphael.ID)
	}
}

// TestRaphaelGrantLimitOnePlay: one attack-trigger Effect is one MayPlay
// grant, so it offers and pays for one card from exile; the other card the
// same attack's grant covers is then no longer offered that turn.
func TestRaphaelGrantLimitOnePlay(t *testing.T) {
	t.Parallel()
	run := runScenarioOrFail(t, oracleScenario{
		Name: "raphael-grant-limit-one",
		CR:   []string{"607.2a", "611.2"},
		Why:  "inline",
		Setup: map[string]oracleSeat{
			"p0": {
				Battlefield: []string{"Raphael, Most Attitude", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest"},
				Hand:        []string{"Grizzly Bears", "Grizzly Bears"},
				LibraryTop:  []string{"Llanowar Elves", "Grizzly Bears", "Forest"},
			},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Grizzly Bears", Mana: "GG"},
			{Op: "resolve"},
			{Op: "cast", Seat: 0, Card: "p0:Grizzly Bears#2", Mana: "GG"},
			{Op: "resolve"},
			{Op: "attack", Seat: 0, Attackers: []string{"p0:Raphael, Most Attitude"}, Defender: "p1"},
			{Op: "resolve"},
			// A may-play cast is offered at sorcery timing, so leave the
			// declare-attackers step first.
			{Op: "pass_to", Step: "main2", Active: "p0"},
			// Extra G is deliberately floated past Llanowar Elves' cost, so
			// the second exiled card is affordable and its absence from the
			// options is the spent MayPlayLimit, not an empty pool.
			{Op: "cast", Seat: 0, Card: "p0:Llanowar Elves", Mana: "GGG", Expect: []oracleExpect{
				{Offered: &oracleOffered{Seat: 0, Kind: "cast", Card: "p0:Grizzly Bears"}, Want: boolp(false)},
			}},
			{Op: "resolve", Expect: []oracleExpect{
				{Card: "p0:Llanowar Elves", Zone: "battlefield"},
			}},
		},
	})
	e := run.e
	raphael := raphaelInZone(t, e, state.ZBattlefield, 0)
	bears := digExileNamed(t, e, "Grizzly Bears")
	if bears.ExiledWith != raphael.ID {
		t.Errorf("Grizzly Bears ExiledWith = %d, want Raphael %d", bears.ExiledWith, raphael.ID)
	}
}
