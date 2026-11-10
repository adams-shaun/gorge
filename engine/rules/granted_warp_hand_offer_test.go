package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedWarpHandOffer: Tannuk, Steadfast Second gives red creature cards
// in hand warp {2}{R}. The granted card is offered as a "warped" cast at that
// cost, is exiled at the next end step, and is recast from exile on a later
// turn for its normal cost.
func TestGrantedWarpHandOffer(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 953, []string{"Tannuk, Steadfast Second", "Goblin Piker", "Sinister Cryologist"}, nil, nil)
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	id := findCardObj(t, e, 0, "Goblin Piker", state.ZHand)
	// Precondition: the grant is on the derived keywords, not the printed face.
	if _, printed := e.G.Obj(id).Face().KeywordParam("Warp"); printed {
		t.Fatal("Goblin Piker prints Warp; the test needs a granted warp")
	}
	if len(grantedWarpCosts(e, id)) == 0 {
		t.Fatalf("no granted warp cost; derived keywords %v", e.Keywords(id))
	}
	// Two mana pays the printed {1}{R} but not the granted {2}{R}.
	addMana(t, e, 0, "R")
	addMana(t, e, 0, "R")
	if grantedWarpOffered(e, id) {
		t.Fatal("warped row offered with only 2 mana floating; the grant costs {2}{R}")
	}
	addMana(t, e, 0, "R")
	if got := e.G.Players[0].Pool.Total(); got != 3 {
		t.Fatalf("pool before the warp cast: %d, want the 3 floated", got)
	}
	submitChoices(t, e, castModeOption(t, e, id, "warped"))
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped == 0 {
		t.Fatalf("Goblin Piker zone %s flags %d, want a warped permanent", o.Zone, o.CastFlags)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the {2}{R} warp cast: %v (3 floated, the granted cost charges all 3)", e.G.Players[0].Pool)
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZExile {
		t.Fatalf("Goblin Piker after the end step: %s, want exile", o.Zone)
	}
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepMain1)
	addMana(t, e, 0, "R")
	addMana(t, e, 0, "R")
	submitChoices(t, e, castModeOption(t, e, id, "warp_recast"))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped != 0 {
		t.Fatalf("Goblin Piker after the recast: zone=%s flags=%d", o.Zone, o.CastFlags)
	}
	replayCheck(t, e, cfg)
}

// TestGrantedWarpArtifactAndGrantLoss: Tannuk's grant covers artifact cards as
// well as red creature cards, and it is a hand static: with Tannuk gone the
// warped row and the derived warp cost both vanish. With Tannuk back, an
// artifact card (Sol Ring) is warp-cast, exiled at the end step and recast
// from exile two turns later.
func TestGrantedWarpArtifactAndGrantLoss(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 958, []string{"Tannuk, Steadfast Second", "Sol Ring", "Goblin Piker"}, nil, nil)
	tannuk := findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	ring := findCardObj(t, e, 0, "Sol Ring", state.ZHand)
	piker := findCardObj(t, e, 0, "Goblin Piker", state.ZHand)
	for _, id := range []state.ObjID{ring, piker} {
		if z := e.G.Obj(id).Zone; z != state.ZHand {
			t.Fatalf("%s in %s, want the hand the grant reads", e.G.Obj(id).Face().Name, z)
		}
	}
	if z := e.G.Obj(tannuk).Zone; z != state.ZBattlefield {
		t.Fatalf("Tannuk in %s, want the battlefield", z)
	}
	if _, printed := e.G.Obj(ring).Face().KeywordParam("Warp"); printed {
		t.Fatal("Sol Ring prints Warp; the test needs a granted warp")
	}
	want := ParseCost("2 R")
	if got := grantedWarpCosts(e, ring); len(got) != 1 || !got[0].Equal(&want) {
		t.Fatalf("Sol Ring granted warp costs %v, want [{2}{R}]; derived keywords %v", got, e.Keywords(ring))
	}
	addMana(t, e, 0, "RRR")
	for _, id := range []state.ObjID{ring, piker} {
		if !grantedWarpOffered(e, id) {
			t.Fatalf("no warped row for %s under Tannuk with RRR floating", e.G.Obj(id).Face().Name)
		}
	}

	// Tannuk leaves: the grant goes with it.
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZGraveyard)
	if z := e.G.Obj(tannuk).Zone; z != state.ZGraveyard {
		t.Fatalf("Tannuk in %s, want the graveyard", z)
	}
	if got := e.G.Players[0].Pool.Total(); got != 3 {
		t.Fatalf("pool after Tannuk left: %d, want the 3 floated still there", got)
	}
	for _, id := range []state.ObjID{ring, piker} {
		if grantedWarpOffered(e, id) {
			t.Fatalf("warped row for %s offered after Tannuk left", e.G.Obj(id).Face().Name)
		}
		if got := grantedWarpCosts(e, id); len(got) != 0 {
			t.Fatalf("%s still has granted warp costs %v after Tannuk left", e.G.Obj(id).Face().Name, got)
		}
	}

	// Tannuk returns: the artifact version of the Goblin Piker flow.
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	if !grantedWarpOffered(e, ring) {
		t.Fatal("no warped row for Sol Ring after Tannuk returned")
	}
	submitChoices(t, e, castModeOption(t, e, ring, "warped"))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(ring); o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped == 0 {
		t.Fatalf("Sol Ring zone %s flags %d, want a warped permanent", o.Zone, o.CastFlags)
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(ring); o.Zone != state.ZExile {
		t.Fatalf("Sol Ring after the end step: %s, want exile", o.Zone)
	}
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepMain1)
	addMana(t, e, 0, "R")
	submitChoices(t, e, castModeOption(t, e, ring, "warp_recast"))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(ring); o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped != 0 {
		t.Fatalf("Sol Ring after the recast: zone=%s flags=%d", o.Zone, o.CastFlags)
	}
	replayCheck(t, e, cfg)
}

func grantedWarpOffered(e *Engine, id state.ObjID) bool {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "warped" {
			return true
		}
	}
	return false
}

// warpedRows are the AltCostIndex values of id's "warped" cast rows.
func warpedRows(e *Engine, id state.ObjID) []int {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return nil
	}
	var out []int
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "warped" {
			out = append(out, o.AltCostIndex)
		}
	}
	return out
}

// warpedRowOption is the option index of id's "warped" row at instance alt.
func warpedRowOption(t *testing.T, e *Engine, id state.ObjID, alt int) int {
	t.Helper()
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "warped" && o.AltCostIndex == alt {
			return o.Index
		}
	}
	t.Fatalf("no warped row %d for %s in %+v", alt, e.G.Obj(id).Face().Name, e.Pending().Options)
	return -1
}

// TestGrantedWarpInstanceRows: a card that prints Warp and is granted another
// carries one warped row per instance (CR 702.185a), except an instance the
// other dominates. Red Tiger Mechan prints {1}{R}, which dominates Tannuk's
// {2}{R}: one row. Spirited Simulacrum prints {4}, incomparable with {2}{R}:
// two rows, the printed one first.
func TestGrantedWarpInstanceRows(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 954, []string{"Tannuk, Steadfast Second", "Red Tiger Mechan", "Spirited Simulacrum"}, nil, nil)
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	m := altCastFor("warped")
	for _, tc := range []struct {
		name string
		want []Cost
	}{
		{"Red Tiger Mechan", []Cost{ParseCost("1 R")}},
		{"Spirited Simulacrum", []Cost{ParseCost("4"), ParseCost("2 R")}},
	} {
		id := findCardObj(t, e, 0, tc.name, state.ZHand)
		f := e.G.Obj(id).Face()
		if _, printed := f.KeywordParam("Warp"); !printed {
			t.Fatalf("%s prints no Warp", tc.name)
		}
		if len(grantedWarpCosts(e, id)) == 0 {
			t.Fatalf("%s has no granted warp; derived keywords %v", tc.name, e.Keywords(id))
		}
		got := altRowCosts(e, m, id, f)
		if len(got) != len(tc.want) {
			t.Fatalf("%s warp rows %v, want %v", tc.name, got, tc.want)
		}
		for i := range got {
			if !got[i].Equal(&tc.want[i]) {
				t.Fatalf("%s warp row %d = %v, want %v", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

// TestGrantedWarpPrintedRowStaysOffered: under Tannuk, Spirited Simulacrum's
// printed warp {4} is still offered and charged when only blue mana is
// floating, though the granted {2}{R} is smaller in total.
func TestGrantedWarpPrintedRowStaysOffered(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 955, []string{"Tannuk, Steadfast Second", "Spirited Simulacrum"}, nil, nil)
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	id := findCardObj(t, e, 0, "Spirited Simulacrum", state.ZHand)
	if len(grantedWarpCosts(e, id)) == 0 {
		t.Fatalf("no granted warp; derived keywords %v", e.Keywords(id))
	}
	addMana(t, e, 0, "UUUU")
	if got := warpedRows(e, id); len(got) != 1 || got[0] != 0 {
		t.Fatalf("warped rows with UUUU: %v, want the printed {4} row only", got)
	}
	submitChoices(t, e, warpedRowOption(t, e, id, 0))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped == 0 {
		t.Fatalf("Spirited Simulacrum zone %s flags %d, want a warped permanent", o.Zone, o.CastFlags)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the {4} warp cast: %v, want all 4 spent", e.G.Players[0].Pool)
	}
}

// TestGrantedWarpGrantRowBesidePrinted: with RRR floating Spirited Simulacrum
// cannot pay its printed {4}, and the grant's {2}{R} row (instance 1) is the
// one offered and charged.
func TestGrantedWarpGrantRowBesidePrinted(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 956, []string{"Tannuk, Steadfast Second", "Spirited Simulacrum"}, nil, nil)
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	id := findCardObj(t, e, 0, "Spirited Simulacrum", state.ZHand)
	addMana(t, e, 0, "RR")
	if got := warpedRows(e, id); len(got) != 0 {
		t.Fatalf("warped rows with RR: %v, want none", got)
	}
	addMana(t, e, 0, "R")
	if got := warpedRows(e, id); len(got) != 1 || got[0] != 1 {
		t.Fatalf("warped rows with RRR: %v, want the granted {2}{R} row (1) only", got)
	}
	submitChoices(t, e, warpedRowOption(t, e, id, 1))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped == 0 {
		t.Fatalf("Spirited Simulacrum zone %s flags %d, want a warped permanent", o.Zone, o.CastFlags)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the {2}{R} warp cast: %v, want all 3 spent", e.G.Players[0].Pool)
	}
}
