package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLandFrontAdventurePlayFromExile pins CR 715.3b for a land-front
// Adventure: after the Adventure half resolves, the card rests in exile as
// its main face, and because that face is a LAND the permission to "play it"
// is a land play (CR 305.1), not a cast. The offer must appear on a later
// main phase with a land drop left, must move the land from exile to the
// battlefield, must consume the land drop, and must disappear once the drop
// is spent.
func TestLandFrontAdventurePlayFromExile(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	jidoor := searchCorpusCard(t, reg, "Jidoor, Aristocratic Capital")
	island := searchCorpusCard(t, reg, "Island")
	deck := []*cards.Card{jidoor}
	for len(deck) < 40 {
		deck = append(deck, island)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = island
	}
	cfg := seatZeroStart(Config{Seed: 7403, Names: []string{"adventure", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := searchMoveByName(t, e, "Jidoor, Aristocratic Capital", state.ZHand)

	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || !o.Face().IsLand() {
		t.Fatalf("precondition: Jidoor's front face is not a land: %+v", o)
	}
	af := adventureSpellFace(o)
	if af == nil || af.Name != "Overture" {
		t.Fatalf("precondition: Jidoor has no Overture Adventure face: %+v", af)
	}
	if e.G.Players[0].LandsPlayed != 0 {
		t.Fatalf("precondition: LandsPlayed = %d, want 0", e.G.Players[0].LandsPlayed)
	}

	addMana(t, e, 0, "UUUUUU")
	alt := adventureOption(t, e, id, "adventure_alt")
	if alt == nil || alt.Label != "Cast Overture" {
		t.Fatalf("Overture not offered from hand: %+v", castOptions(t, e))
	}
	submitChoices(t, e, alt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Overture target ask: %+v", d)
	}
	tgt := -1
	for _, opt := range d.Options {
		if opt.Kind == "player" && opt.Player == 1 {
			tgt = opt.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no option targeting the opponent: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 20)

	o = e.G.Obj(id)
	if o.Zone != state.ZExile || int(o.FaceIdx) != 0 {
		t.Fatalf("after Overture resolved: zone=%s faceIdx=%d, want ZExile/0", o.Zone, o.FaceIdx)
	}
	// CR 305.1: the land main face is never a cast, so no recast offer.
	if recast := adventureOption(t, e, id, "adventure_recast"); recast != nil {
		t.Fatalf("land main face offered as adventure_recast: %+v", recast)
	}

	// A later main phase this turn, land drop still open.
	driveToStep(t, e, 1, 0, state.StepMain2)
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("expected seat 0's priority in main2, got %+v", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "play_land" && opt.Obj == id {
			idx = opt.Index
			if opt.Label != "Play Jidoor, Aristocratic Capital" {
				t.Fatalf("play_land label = %q", opt.Label)
			}
		}
	}
	if idx < 0 {
		t.Fatalf("no play_land option for Jidoor from exile: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit play_land: %v", err)
	}

	if got := e.G.Obj(id); got.Zone != state.ZBattlefield {
		t.Fatalf("played land zone = %s, want battlefield", got.Zone)
	}
	if got := e.G.Players[0].LandsPlayed; got != 1 {
		t.Fatalf("LandsPlayed = %d, want 1", got)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected priority after the land play, got %+v", d)
	}
	for _, opt := range d.Options {
		if opt.Kind == "play_land" && opt.Obj == id {
			t.Fatalf("play_land offered after the land drop was spent: %+v", opt)
		}
	}
	replayCheck(t, e, cfg)
}
