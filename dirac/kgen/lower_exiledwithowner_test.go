package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// exiledWithOwnerProbe lowers an inline script against the corpus registry.
const exiledWithOwnerSrc = `Name:Unidentified Hovership
ManaCost:1 W W
Types:Artifact Vehicle
PT:2/2
K:Flying
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigChangeZone | TriggerDescription$ enters: exile up to one creature of toughness 5 or less.
SVar:TrigChangeZone:DB$ ChangeZone | TargetMin$ 0 | TargetMax$ 1 | ValidTgts$ Creature.toughnessLE5 | TgtPrompt$ Pick a creature of toughness 5 or less. | Origin$ Battlefield | Destination$ Exile
T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | Execute$ TrigDreadOwner | TriggerDescription$ leaves: the exiled card's owner manifests dread.
SVar:TrigDreadOwner:DB$ ManifestDread | DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1
K:Crew:1
Oracle:Flying\nWhen Unidentified Hovership enters, exile up to one target creature with toughness 5 or less.\nWhen Unidentified Hovership leaves the battlefield, the exiled card's owner manifests dread.\nCrew 1
`

// TestUnidentifiedHovershipExiledWithOwnerLowers: the post-ff076d4432 script
// lowers the leave trigger's ManifestDread over
// DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1 to
// RefExiledWithOwner -- the shared player filter's HasCardsIn property, the
// seat whose exile holds a card exiled with the source (gorge exiledBySource;
// the kernel's exw link records). The ETB exile no longer remembers and no
// Cleanup follows, so the remembered-persist shape is NOT in play: the card
// must lower WITHOUT CardDef.RemPersist (the exw forward link carries the
// association, not the source's remembered set).
func TestUnidentifiedHovershipExiledWithOwnerLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, diags := cards.ParseBytes("exiled_with_owner.txt", []byte(exiledWithOwnerSrc))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	if lc.RemPersist {
		t.Error("RemPersist set: the new script remembers nothing, the exw link carries the association")
	}
	var dreads int
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			if in.Op != "ManifestDread" {
				continue
			}
			dreads++
			if in.Obj.Kind != rExiledWithOwner {
				t.Errorf("ManifestDread Obj %v, want rExiledWithOwner", in.Obj)
			}
		}
	}
	if dreads != 1 {
		t.Errorf("ManifestDread instructions %d, want 1", dreads)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	b := &all[0]
	if b.def.RemPersist {
		t.Error("built card set CardDef.RemPersist")
	}
	if !b.def.ExiledWith || !b.def.ExiledWithRev {
		t.Errorf("built card reader bits ExiledWith %v ExiledWithRev %v, want both: the ETB exile must record the link (genPrep.exwRec)",
			b.def.ExiledWith, b.def.ExiledWithRev)
	}
	saw := false
	for i := range b.def.Abilities {
		for j := range b.def.Abilities[i].Prog {
			in := &b.def.Abilities[i].Prog[j]
			if in.Op == kop.OpManifestDread && in.Obj == kop.RefExiledWithOwner {
				saw = true
			}
		}
	}
	if !saw {
		t.Error("built card has no ManifestDread over RefExiledWithOwner")
	}
}

// TestExiledWithOwnerOtherZonesRefuse pins the fail-closed side of the
// spelling: the kernel's RefExiledWithOwner is the exile zone only, so any
// other HasCardsIn<zone> spelling (even one gorge's player filter can
// evaluate) refuses rather than lowering to a ref that reads a different
// zone.
func TestExiledWithOwnerOtherZonesRefuse(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, sel := range []string{
		"Player.HasCardsInGraveyard_Card.ExiledWithSource_GE1",
		"Player.HasCardsInHand_Card.ExiledWithSource_GE1",
		"Player.HasCardsInExile_Creature.ExiledWithSource_GE1",
		"Player.HasCardsInExile_Card.ExiledWithSource_GE2",
	} {
		src := replaceTrigDreadDefinedPlayer(t, exiledWithOwnerSrc, sel)
		card, diags := cards.ParseBytes("exiled_with_owner_refuse.txt", []byte(src))
		if card == nil {
			t.Fatalf("script did not parse (%s): %v", sel, diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("link (%s): %v", sel, d)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		refused := false
		for _, rf := range lc.Refusals {
			if strings.Contains(rf.Reason, "HasCardsIn") {
				refused = true
			}
		}
		if !refused {
			t.Errorf("%s lowered with no refusal: %v", sel, lc.Refusals)
		}
	}
}

// replaceTrigDreadDefinedPlayer swaps the probe's DefinedPlayer$ value.
func replaceTrigDreadDefinedPlayer(t *testing.T, src, sel string) string {
	t.Helper()
	old := "SVar:TrigDreadOwner:DB$ ManifestDread | DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1"
	swapped := "SVar:TrigDreadOwner:DB$ ManifestDread | DefinedPlayer$ " + sel
	if !strings.Contains(src, old) {
		t.Fatal("probe does not spell the reference DefinedPlayer$")
	}
	return strings.Replace(src, old, swapped, 1)
}
