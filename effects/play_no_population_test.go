package effects

// The no-population `DB$ Play` pin (review finding sol1/MAJOR on ticket
// agent-20260920T071934Z-bae92ac6).
//
// effPlay's Valid$/ValidZone$ arm gained a Forge-faithful default zone --
// the resolving controller's hand -- so the Face of Boe's
// `Valid$ Card.withSuspend` AB, which states a population and no zone, can
// pose its selection. That default must be bound to a STATED population: 13
// raw corpus `Play` lines carry no population param at all (Syrix's
// `SVar:TrigPlay:DB$ Play | Optional$ True`, thunderblade_charge,
// tibalt_the_chaotic, jhoira_of_the_ghitu_avatar x2, the_duke_of_midrange,
// mysterious_confluence, ersta_friend_to_all, really_charming_prince,
// discord_lord_of_disharmony's ST$). Their real bodies are
// self-referent/CopyFromChosenName$/AnySupportedCard$ shapes this effect
// does not implement (Syrix's trigger is "cast CARDNAME from your
// graveyard"), so pairing the hand default with the absent-Valid$ `Card`
// fallback would turn each of them from a silent no-op into an offer of the
// controller's WHOLE hand. They stay fail-closed until their own shapes are
// implemented.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// syrixTrigPlay returns Syrix, Carrier of the Flame's REAL compiled TrigPlay
// body, asserting it is the no-population shape under test: API Play,
// Optional$ True, and not one of the four population params. A fixture that
// only looks like the shape can never stand in for it.
func syrixTrigPlay(t *testing.T) *cards.SA {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Syrix, Carrier of the Flame")
	if !ok {
		t.Fatal("corpus has no Syrix, Carrier of the Flame")
	}
	sa := cards.ResolveSVar(c.Faces[0].SVars, "TrigPlay")
	if sa == nil || sa.API != "Play" {
		t.Fatalf("Syrix TrigPlay = %+v, want a compiled DB$ Play", sa)
	}
	if sa.Params["Optional"] != "True" {
		t.Fatalf("precondition: compiled Optional$ = %q, want True", sa.Params["Optional"])
	}
	for _, p := range []string{"Valid", "ValidZone", "Defined", "ValidTgts"} {
		if v, ok := sa.Params[p]; ok && v != "" {
			t.Fatalf("precondition: compiled body carries %s$ %q; it is no longer the no-population shape", p, v)
		}
	}
	return sa
}

// TestNoPopulationPlayOffersNothingFromTheHand: Syrix's real TrigPlay,
// resolved by a controller holding three castable cards, poses NO decision
// and emits nothing. The hand default belongs to a stated population only.
func TestNoPopulationPlayOffersNothingFromTheHand(t *testing.T) {
	sa := syrixTrigPlay(t)

	h := &askHost{}
	h.g = state.NewGame(names(2))
	bear := mkCard(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	var hand []state.ObjID
	for i := 0; i < 3; i++ {
		hand = append(hand, h.g.AddObject(bear, 0).ID)
	}
	h.g.SetZone(state.ZHand, 0, hand)
	// Syrix itself sits in its own graveyard, the zone its trigger fires
	// from -- so a self-referent read would have somewhere to land too.
	src := h.g.AddObject(mkCard(t, "Name:Syrix, Carrier of the Flame\nManaCost:2 B R\nTypes:Legendary Creature Phoenix\nPT:3/3\nOracle:x\n"), 0)
	src.Zone = state.ZGraveyard
	h.g.SetZone(state.ZGraveyard, 0, []state.ObjID{src.ID})

	c := &Ctx{Source: src.ID, Controller: 0}
	Resolve(h, c, sa)

	if h.asked != nil {
		t.Fatalf("a no-population DB$ Play posed %v with %d option(s): %+v",
			h.asked.Kind, len(h.asked.Options), h.asked.Options)
	}
	if len(h.log) != 0 {
		t.Fatalf("a no-population DB$ Play emitted %d event(s): %+v", len(h.log), h.log)
	}
	if c.Play != 0 {
		t.Fatalf("Ctx.Play = %d, want 0: nothing was chosen to play", c.Play)
	}
}

// TestStatedPopulationPlayKeepsTheHandDefault is the positive half: the same
// effect with a STATED Valid$ and no ValidZone$ still reads the resolving
// controller's hand, so the Face of Boe family the hand default was added
// for is not regressed by the guard above.
func TestStatedPopulationPlayKeepsTheHandDefault(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	bear := mkCard(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	var hand []state.ObjID
	for i := 0; i < 2; i++ {
		hand = append(hand, h.g.AddObject(bear, 0).ID)
	}
	h.g.SetZone(state.ZHand, 0, hand)
	src := h.g.AddObject(mkCard(t, "Name:Asker\nTypes:Sorcery\nOracle:x\n"), 0)
	src.Zone = state.ZBattlefield

	c := &Ctx{Source: src.ID, Controller: 0}
	Resolve(h, c, sa(t, "AB$ Play | Cost$ T | Valid$ Creature | Optional$ True"))

	if h.asked == nil {
		t.Fatal("a stated-population Play with no ValidZone$ posed no ask; the hand default is gone")
	}
	if len(h.asked.Options) == 0 {
		t.Fatalf("stated-population Play offered no options: %+v", h.asked)
	}
}
