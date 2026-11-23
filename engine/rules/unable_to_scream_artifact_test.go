package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	utsPhone = "Name:Uts Phone\nManaCost:2 W\nTypes:Artifact Creature Toy\nPT:2/1\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ TrigDig | TriggerDescription$ x\n" +
		"SVar:TrigDig:DB$ Dig | DigNum$ 5 | ChangeNum$ 1 | Optional$ True | ChangeValid$ Creature.powerLE2\nOracle:x\n"
	utsAura = "Name:Uts Aura\nManaCost:U\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ Continuous | AffectedDefined$ Enchanted | SetPower$ 0 | SetToughness$ 2 | RemoveAllAbilities$ True | AddType$ Artifact & Creature & Toy | Description$ x\nOracle:x\n"
)

// A natively artifact creature wearing a type-adding Aura stays an artifact.
func TestAddTypeAuraKeepsNativeArtifact(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, utsPhone), card(t, utsAura))
	phone := e.G.Zone(state.ZHand, 0)[0]
	placeOnBattlefield(t, e, phone)
	if !e.IsArtifact(phone) {
		t.Fatalf("bare phone is not an artifact")
	}
	lossAttach(t, e, "Uts Aura", 0, phone)
	if !e.IsArtifact(phone) {
		t.Fatalf("enchanted phone is not an artifact")
	}
	if !e.IsCreature(phone) {
		t.Fatalf("enchanted phone is not a creature")
	}
}

const utsExorcise = "Name:Uts Exorcise\nManaCost:1 W\nTypes:Sorcery\n" +
	"A:SP$ ChangeZone | ValidTgts$ Artifact,Enchantment,Creature.powerGE4 | TgtPrompt$ x | Origin$ Battlefield | Destination$ Exile | SpellDescription$ x\nOracle:x\n"

// A manifested (face-down) card an Aura makes the artifact creature it prints is
// an artifact for a target filter: the derived-type table keeps its entry (CR 708.2).
func TestExorciseOffersEnchantedArtifact(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, utsPhone), card(t, utsAura), card(t, utsExorcise))
	hand := e.G.Zone(state.ZHand, 0)
	phone, ex := hand[0], hand[2]
	e.emit(events.Event{Kind: events.MoveZone, Obj: phone, From: state.ZHand, To: state.ZBattlefield, Counter: events.ManifestEntryCounter})
	if !e.G.Obj(phone).FaceDown {
		t.Fatal("precondition: not face down")
	}
	lossAttach(t, e, "Uts Aura", 0, phone)
	if !e.IsArtifact(phone) {
		t.Fatal("precondition: enchanted face-down phone is not an artifact")
	}
	addMana(t, e, 0, "WW")
	ppChoose(t, e, func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == ex })
	d := e.Pending()
	found := false
	for _, o := range d.Options {
		if o.Obj == phone {
			found = true
		}
	}
	if !found {
		t.Fatalf("phone %d not among targets %+v", phone, d.Options)
	}
}
