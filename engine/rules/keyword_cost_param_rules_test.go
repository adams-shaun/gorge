// keyword_cost_param_rules_test.go — the two live charge bugs from handing
// ParseCost the whole KeywordParam remainder of a cost-bearing keyword.
//
// Forge's KeywordWithCost reads a cost as the FIRST colon-field
// (KeywordWithCost.java:22), so "Disguise:5 R:X:This cost is reduced by …"
// costs {5}{R} and "Flashback:8 U U:ReduceCost$ X:…" costs {8}{U}{U}. Before
// cards.Face.KeywordCostParam these two readers saw the trailing fields too:
// Fugitive Codebreaker's Disguise turn-up parsed as 20 phantom generic plus
// Unknown -> morphFaceUpCost failed closed and withheld the special action, and
// Visions of Duplicity's flashback cost parsed as 36 generic + {U}.
//
// Both carriers are real corpus cards (no Forge text committed here). The
// helpers come from morph_turnup_test.go, morph_test.go, manifest_test.go,
// cast_test.go and search_library_test.go.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFugitiveCodebreakerDisguiseTurnFaceUpOffersAndChargesReducedCost: a
// disguised Fugitive Codebreaker's printed rider ("This cost is reduced by {1}
// for each instant and sorcery card in your graveyard") is part of its
// Disguise cost, so with two such cards in its controller's graveyard the
// turn-up costs {3}{R}: the offer appears at 4 mana and choosing it charges
// exactly 4, then flips the card face up. The cost is the KeywordWithCost
// line's first colon-field; the rider amount is the SECOND (the SVar "X").
func TestFugitiveCodebreakerDisguiseTurnFaceUpOffersAndChargesReducedCost(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Fugitive Codebreaker", "Lightning Bolt", "Lightning Bolt")
	// Fund {3} for the face-down cast and nothing more (RRR), so the pool is
	// empty after it.
	id := morphDownCast(t, e, "Fugitive Codebreaker", "disguised", "RRR", 0)

	// Preconditions: the real must differ from the compared value -- the
	// card is on the battlefield face down, the printed Disguise cost parses
	// to the {5}{R} charge (not the polluted 20-generic parse), and the
	// reduction token is the line's second colon-field, not the reminder.
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || !o.FaceDown || o.Controller != 0 {
		t.Fatalf("precondition: Fugitive Codebreaker zone=%s faceDown=%v controller=%d, want a face-down battlefield object seat 0",
			o.Zone, o.FaceDown, o.Controller)
	}
	f := o.Face()
	raw, ok := f.KeywordCostParam("Disguise")
	if !ok || raw != "5 R" {
		t.Fatalf("precondition: KeywordCostParam(Disguise) = %q %v, want %q", raw, ok, "5 R")
	}
	if whole, ok := f.KeywordParam("Disguise"); !ok || whole == "5 R" {
		t.Fatalf("precondition: KeywordParam(Disguise) = %q %v, want the whole remainder (so the two reads differ)", whole, ok)
	}
	mf, ok := morphFaceUpCost(o)
	if !ok || mf.cost.Generic != 5 || mf.cost.Colored[state.MR] != 1 || len(mf.cost.Unknown) != 0 {
		t.Fatalf("precondition: morphFaceUpCost = %+v ok=%v, want generic 5 + one red, no Unknown", mf.cost, ok)
	}
	if mf.reduceToken != "X" {
		t.Fatalf("precondition: reduceToken = %q, want the second colon-field %q", mf.reduceToken, "X")
	}

	// Seed TWO instants/sorceries in seat 0's graveyard (real corpus cards,
	// moved with a logged MoveZone): the rider counts them.
	g1 := searchMoveByName(t, e, "Lightning Bolt", state.ZGraveyard)
	g2 := searchMoveByName(t, e, "Lightning Bolt", state.ZGraveyard)
	if g1 == g2 || e.G.Obj(g1).Zone != state.ZGraveyard || e.G.Obj(g2).Zone != state.ZGraveyard || e.G.Obj(g1).Owner != 0 {
		t.Fatalf("precondition: graveyard cards = %d/%d zones %s/%s owner %d, want two distinct seat-0 Bolt Moves to ZGraveyard",
			g1, g2, e.G.Obj(g1).Zone, e.G.Obj(g2).Zone, e.G.Obj(g1).Owner)
	}
	// The composed cost really is {3}{R}, not the printed {5}{R}: the two
	// values differ, so the assertions below are about the rider.
	mods, mok := e.morphTurnUpMods(0, id, mf)
	if !mok {
		t.Fatal("precondition: morphTurnUpMods withheld the turn-up")
	}
	if got := mods.Apply(mf.cost); got.Generic != 3 || got.Colored[state.MR] != 1 || got.CMC() != 4 {
		t.Fatalf("precondition: composed turn-up cost = %+v, want {3}{R}", got)
	}

	// The pool is empty, so the turn-up must not be offered yet.
	if turnFaceUpOptionPresent(t, e, id) {
		t.Fatal("turn_face_up offered with an empty pool, want withheld")
	}
	// Three mana is one short of the reduced {3}{R}: still withheld.
	addMana(t, e, 0, "RRR")
	if turnFaceUpOptionPresent(t, e, id) {
		t.Fatal("turn_face_up offered for 3 mana, want withheld (the reduced cost is 4)")
	}

	// Fund exactly {3}{R} (4 mana); now the offer must appear and the action
	// pay it.
	addMana(t, e, 0, "R")
	if !turnFaceUpOptionPresent(t, e, id) {
		t.Fatal("turn_face_up not offered after funding the reduced {3}{R}")
	}
	mark := len(e.L.Events)
	before := e.G.Players[0].Pool.Total()
	submitChoices(t, e, turnFaceUpIndex(t, e, id))

	// The turn-up is a special action: the CARD itself never reaches the
	// stack (assertTurnUpEventOnce checks no PutOnStack for id). Fugitive's
	// own TurnFaceUp trigger may sit on the stack afterwards.
	assertTurnUpEventOnce(t, e, id, mark)
	if got, want := e.G.Players[0].Pool.Total(), before-4; got != want {
		t.Fatalf("pool after turn-up = %d, want %d (exactly the reduced {3}{R})", got, want)
	}
	if o := e.G.Obj(id); o.FaceDown {
		t.Fatal("Fugitive Codebreaker FaceDown=true after the turn-up, want false")
	}
	replayCheck(t, e, cfg)
}

// TestVisionsOfDuplicityFlashbackCostIsPrintedNotPolluted pins the second live
// charge bug: the flashback cost parses as {8}{U}{U}, not the polluted
// 36-generic + {U} the whole-remainder read produced.
func TestVisionsOfDuplicityFlashbackCostIsPrintedNotPolluted(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Visions of Duplicity")
	// The flashback permission is read off the card in the graveyard.
	id := searchMoveByName(t, e, "Visions of Duplicity", state.ZGraveyard)

	o := e.G.Obj(id)
	if o.Zone != state.ZGraveyard || o.Face() == nil {
		t.Fatalf("precondition: Visions of Duplicity zone=%s face=%v, want graveyard with a face", o.Zone, o.Face())
	}
	f := o.Face()
	raw, ok := f.KeywordCostParam("Flashback")
	if !ok || raw != "8 U U" {
		t.Fatalf("precondition: KeywordCostParam(Flashback) = %q %v, want %q", raw, ok, "8 U U")
	}
	if whole, ok := f.KeywordParam("Flashback"); !ok || whole == "8 U U" {
		t.Fatalf("precondition: KeywordParam(Flashback) = %q %v, want the whole remainder (so the two reads differ)", whole, ok)
	}

	c := e.flashbackCost(id)
	if c.Generic != 8 || c.Colored[state.MU] != 2 {
		t.Fatalf("flashbackCost = generic %d coloured %v, want {8}{U}{U}", c.Generic, c.Colored)
	}
	if len(c.Unknown) != 0 {
		t.Fatalf("flashbackCost reported Unknown %v, want none", c.Unknown)
	}
	// The cost string the offer/charge path uses is the same printed field.
	if got, want := flashbackCostString(f), "8 U U"; got != want {
		t.Fatalf("flashbackCostString = %q, want %q", got, want)
	}
	replayCheck(t, e, cfg)
}

// TestKeywordCostParamFlashbackReaderLeavesOtherHeadsAlone is the boundary
// check: the accessor is a KeywordWithCost read,
// and the heads Forge leads with a value rather than a cost (Suspend,
// Impending, Kicker) keep their own readers. It asserts the accessor would
// return the wrong field for those lines, which is exactly why they must not
// call it, and that the real readers still resolve the cost after the change.
func TestKeywordCostParamFlashbackReaderLeavesOtherHeadsAlone(t *testing.T) {
	t.Parallel()
	c, _ := cards.ParseBytes("k.txt", []byte(
		"Name:K\nTypes:Sorcery\n"+
			"K:Suspend:5:W\n"+
			"K:Impending:4:2 B\n"+
			"K:Kicker:1 R:1 G\n"+
			"Oracle:x\n"))
	f := c.Faces[0]
	// The accessor's first-field read is the WRONG field for these families --
	// the value, not the cost -- which is the documented reason each has its
	// own reader (engine/rules/cast_altcost.go suspendInfo / keywordAltCost /
	// twoPartKickerCosts).
	if got, _ := f.KeywordCostParam("Suspend"); got != "5" {
		t.Fatalf("KeywordCostParam(Suspend) = %q, want the leading time field %q (so Suspend must not use it)", got, "5")
	}
	if got, _ := f.KeywordCostParam("Impending"); got != "4" {
		t.Fatalf("KeywordCostParam(Impending) = %q, want the leading count field %q (so Impending must not use it)", got, "4")
	}
	one, two, both := twoPartKickerCosts(f)
	if !both || one.Generic != 1 || one.Colored[state.MR] != 1 || two.Generic != 1 || two.Colored[state.MG] != 1 {
		t.Fatalf("twoPartKickerCosts = one %+v two %+v both %v, want {1}{R} + {1}{G}", one, two, both)
	}
}
