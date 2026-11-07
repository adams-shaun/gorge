package rules

// mayplay_cast_payment_cost_test.go pins MayPlayCastPaymentCost (payment_plan.go),
// the read the oracle's convoke frame filters a may-play cast's announcement by
// (CR 702.51b, "up to two green creatures and up to three other creatures"):
// the plain cast a may-play permission grants a card in exile must price
// beside the plain hand casts, or the oracle offers contributions the
// announcement cannot absorb and gorge's Submit refuses them. CastPaymentCost
// prices hand casts only, which is exactly the gap the pair-18 std-ecl
// evidence game fell through (Sanar's Vivid exiling Lofty Dreams and a
// may-play convoke cast of it).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// sanarMayPlayCastCostScenario casts nothing: Sanar sits on the battlefield
// through turn 1's first main phase, where Vivid reveals her controller's top
// two nonland cards (X = 2 colours among permanents controlled: Sanar's own
// U and R) and exiles Lofty Dreams (blue) and Eclipsed Flamekin (red) with
// the delivering effect remembering them, so its STPlay static ("You may
// cast the exiled cards this turn") grants p0 a may-play cast of each.
const sanarMayPlayCastCostScenario = `{"name":"mayplay-cast-payment-cost","cr":["702.51b"],"why":"inline",` +
	`"setup":{"p0":{"battlefield":["Sanar, Innovative First-Year"],` +
	`"hand":["Island"],` +
	`"library_top":["Lofty Dreams","Eclipsed Flamekin"]},"p1":{}},"steps":[]}`

func TestMayPlayCastPaymentCostPricesSanarsExileGrant(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(sanarMayPlayCastCostScenario))
	if err != nil {
		t.Fatal(err)
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%v", fails, transcript)
	}
	e := run.e
	named := func(zone state.Zone, p state.PlayerID, name string) state.ObjID {
		for _, id := range e.G.Zone(zone, p) {
			if o := e.G.Obj(id); o != nil && o.Face().Name == name {
				return id
			}
		}
		return 0
	}
	lofty := named(state.ZExile, 0, "Lofty Dreams")
	flamekin := named(state.ZExile, 0, "Eclipsed Flamekin")
	if lofty == 0 || flamekin == 0 {
		t.Fatalf("precondition: Vivid did not exile both revealed cards (exile has Lofty %d, Flamekin %d)\n%v", lofty, flamekin, transcript)
	}
	island := named(state.ZHand, 0, "Island")
	if island == 0 {
		t.Fatalf("precondition: Island is not in p0's hand\n%v", transcript)
	}

	// The grant offers the exiled aura: Sanar's effect-delivered STPlay
	// static ("You may cast the exiled cards this turn") surfaces it through
	// mayPlaySpellIds, with the effect's untyped free/raise read (an empty
	// permission key) and the printed cost priced.
	key := ""
	offered := false
	for _, off := range e.mayPlaySpellIds(0) {
		if off.id == lofty {
			offered = true
			key = off.key
			if off.free {
				t.Errorf("precondition: Sanar's grant reads free, want a priced cast of the printed cost")
			}
		}
	}
	if !offered {
		t.Fatalf("precondition: Lofty Dreams %d is not offered as a may-play cast\n%v", lofty, transcript)
	}

	pc, ok := e.MayPlayCastPaymentCost(0, lofty, key)
	if !ok {
		t.Fatalf("MayPlayCastPaymentCost(p0, Lofty Dreams, %q) = not ok, want the printed {3}{U}{U}", key)
	}
	if pc.Generic != 3 || pc.Mana[state.ManaIndex('U')] != 2 {
		t.Errorf("MayPlayCastPaymentCost = generic %d, mana %v, want {3}{U}{U}", pc.Generic, pc.Mana)
	}
	var other decision.ManaAmount
	other[state.ManaIndex('U')] = 2
	if pc.Mana != other {
		t.Errorf("MayPlayCastPaymentCost mana %v, want exactly Lofty Dreams' two {U}", pc.Mana)
	}

	// The read stays a pure read: nothing moved. (Vivid's shuffle already
	// ran; the zones after it must be exactly as the scenario left them.)
	if z := e.G.Obj(lofty).Zone; z != state.ZExile {
		t.Errorf("the read moved Lofty Dreams to %s", z)
	}

	// The hand-only read prices no may-play card: it is the gap the oracle's
	// convoke frame used to fall through.
	if _, ok := e.CastPaymentCost(0, lofty); ok {
		t.Errorf("CastPaymentCost(p0, exiled Lofty Dreams) = ok, want not ok (the read prices hand casts only)")
	}
	// And the may-play read prices no hand card.
	if _, ok := e.MayPlayCastPaymentCost(0, island, key); ok {
		t.Errorf("MayPlayCastPaymentCost(p0, hand Island) = ok, want not ok (nothing is granted in hand)")
	}
}
