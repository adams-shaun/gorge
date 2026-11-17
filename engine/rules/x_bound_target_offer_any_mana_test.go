package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 601.2b, 601.2c, 601.2h and 602.2b: an activated ability that announces an
// {X} and targets a creature card with mana value X is offered only when some
// announcement X both pays and leaves a legal target. One Treasure Token makes
// ONE mana of any colour; it cannot fund an X of 3, so a graveyard holding only
// a mana value 3 creature card must withhold the activation exactly as an empty
// graveyard does. The potential bound prices an open "any colour" production as
// unbounded in every slot, which offered Sequence Engine's activation at
// decision 588 of synth tail-29 pair 33.
func TestXBoundTargetAnyColourSourceFundsOneAnnouncement(t *testing.T) {
	t.Parallel()
	const gem = "Name:One Gem\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T Sac<1/CARDNAME/this artifact> | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any colour.\nOracle:x\n"
	const summoner = "Name:Summoner\nTypes:Creature Human\nPT:1/1\n" +
		"A:AB$ ChangeZone | Cost$ X T | ValidTgts$ Creature.cmcEQX | TgtPrompt$ Pick a creature card | Origin$ Graveyard | Destination$ Exile | SpellDescription$ Exile a creature card with mana value X.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const brute = "Name:Three Brute\nManaCost:1 R R\nTypes:Creature Cyclops\nPT:4/5\nOracle:x\n"
	e := handEngine(t)
	id := onBoardReady(t, e, 0, summoner)
	onBoard(t, e, 0, gem)
	bruteID := e.G.AddObject(card(t, brute), 0)
	bruteID.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{bruteID.ID})
	if xBoundOffered(e, "ability", id) {
		t.Fatal("offered X=3 with one mana of any colour: no announcement both pays and leaves a legal target")
	}
	// The same ability IS offered once the mana for X=3 is floating, so the
	// assertion above fails on the over-bound and not on a broken fixture.
	e.G.Players[0].Pool[state.MC] = 4
	if !xBoundOffered(e, "ability", id) {
		t.Fatal("not offered with four mana floating and a mana value 3 creature card in the graveyard")
	}
}
