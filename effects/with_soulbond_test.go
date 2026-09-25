package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestPairedWithSoulbond(t *testing.T) {
	g, ids := board(t)
	source := g.Obj(ids["myBear"])
	partner := g.Obj(ids["myFlier"])
	if source.Zone != state.ZBattlefield || partner.Zone != state.ZBattlefield {
		t.Fatal("precondition: paired objects must be on battlefield")
	}
	source.Paired, partner.Paired = partner.ID, source.ID
	if source.Paired == 0 || partner.Paired != source.ID {
		t.Fatal("precondition: objects must be paired")
	}
	partner.Face().Keywords = append(partner.Face().Keywords, "Soulbond")
	if !partner.Face().HasKeyword("Soulbond") {
		t.Fatal("precondition: candidate must have Soulbond")
	}
	spec := "Creature.PairedWith+withSoulbond"
	if got := UnknownPredicates(spec); len(got) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v", spec, got)
	}
	if !MatchesSpecFrom(g, spec, partner.ID, 0, source.ID) {
		t.Fatal("paired creature with Soulbond did not match")
	}
	partner.Face().Keywords = nil
	if partner.Face().HasKeyword("Soulbond") {
		t.Fatal("precondition: negative candidate must lack Soulbond")
	}
	if MatchesSpecFrom(g, spec, partner.ID, 0, source.ID) {
		t.Fatal("paired creature without Soulbond matched")
	}
}
