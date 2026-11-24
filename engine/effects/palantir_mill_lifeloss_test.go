package effects

// Palantir of Orthanc's end-step trigger (REAL corpus script): "put an
// influence counter on it and scry 2. Then target opponent may have you draw
// a card. If that player doesn't, you mill X cards, where X is the number of
// influence counters on it, and that player loses life equal to the total
// mana value of those cards." The target opponent belongs to DBChoice, a
// depth-2 GenericChoice link; the chosen CombustDamage mode's LoseLife names
// it through Defined$ ParentTarget. The GenericChoice body resolves the mode
// INSIDE its own dispatch, so the link's answer must already be the nearest
// parent while the mode runs (CR 608.2c), not only after it returns.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestPalantirMillLifeLoss(t *testing.T) {
	card, root := corpusCardSA(t, "Palantír of Orthanc", "TrigPutCounter")
	choice := root.Sub
	for choice != nil && choice.API != "GenericChoice" {
		choice = choice.Sub
	}
	if choice == nil || !TargetsOf(choice).Targeted() {
		t.Fatalf("precondition: no targeting GenericChoice link under TrigPutCounter")
	}
	combust := cards.ResolveSVar(card.Faces[0].SVars, "CombustDamage")
	if combust == nil || combust.API != "Mill" || combust.Sub == nil || combust.Sub.API != "LoseLife" ||
		combust.Sub.ParamStr(cards.PKDefined) != "ParentTarget" {
		t.Fatalf("precondition: CombustDamage = %+v, want Mill -> LoseLife Defined$ ParentTarget", combust)
	}
	h := newHost(t, 2)
	for i := range h.g.Players {
		h.g.Players[i].Life = 20
	}
	o := h.g.AddObject(card, 0)
	o.Zone = state.ZBattlefield
	o.AddCounter("INFLUENCE", 1) // the trigger adds the second
	// Library top to bottom: MV 3, MV 4, MV 5 (the third is not milled).
	for _, mc := range []string{"1 R R", "2 G G", "3 U U"} {
		f := h.g.AddObject(mkCard(t, "Name:Fodder "+mc+"\nManaCost:"+mc+"\nTypes:Sorcery\nOracle:x\n"), 0)
		h.g.SetZone(state.ZLibrary, 0, append(h.g.Zone(state.ZLibrary, 0), f.ID))
	}
	c := &Ctx{Source: o.ID, Controller: 0, SVars: card.Faces[0].SVars,
		// The target opponent chose CombustDamage: the chooser cursor's
		// resume shape (index past that chooser, its answer in Modes).
		Modes:     []string{"CombustDamage"},
		SubPreAsk: map[string][]state.Target{choice.Line: {{Player: 1, IsPlayer: true}}},
	}
	c.Choosers.Choosers = []state.Target{{Player: 1, IsPlayer: true}}
	c.Choosers.ChooserIndex = 1
	Resolve(h, c, root)
	if got := o.Counter("INFLUENCE"); got != 2 {
		t.Fatalf("precondition: %d influence counters, want 2", got)
	}
	if n := len(h.g.Zone(state.ZGraveyard, 0)); n != 2 {
		t.Fatalf("precondition: milled %d cards, want 2", n)
	}
	if got := h.g.Players[1].Life; got != 13 {
		t.Errorf("target opponent life = %d, want 13 (lost 3+4)", got)
	}
	if got := h.g.Players[0].Life; got != 20 {
		t.Errorf("controller life = %d, want 20", got)
	}
}
