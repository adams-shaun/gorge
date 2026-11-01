package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestUneasyAllianceExilesEnchantedAfterSacrifice pins Defined$ Enchanted
// resolving off the Aura's last bearer once the Aura itself was sacrificed
// as the activation cost ("{5}, Sacrifice this Aura: Exile enchanted
// creature"): the move fold has cleared AttachedTo by resolution, so the
// resolver must fall back to LastBearer (CR 608.2h last known information).
func TestUneasyAllianceExilesEnchantedAfterSacrifice(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, ids := attachedToFixture(t, reg, 9411, []string{"Uneasy Alliance"}, []string{"Hill Giant"}, nil)
	giant, aura := ids["Hill Giant"], ids["Uneasy Alliance"]

	addMana(t, e, 0, "1W")
	castNamed(t, e, "Uneasy Alliance")
	attachDrain(t, e, 40)
	ao := e.G.Obj(aura)
	if ao == nil || ao.Zone != state.ZBattlefield || ao.AttachedTo != giant {
		t.Fatalf("precondition: Aura must be attached to the Giant on the battlefield, got %+v", ao)
	}

	addMana(t, e, 0, "CCCCC")
	submitChoices(t, e, abilityOption(t, e, aura, 0).Index)
	attachDrain(t, e, 40)

	if ao := e.G.Obj(aura); ao.Zone != state.ZGraveyard {
		t.Fatalf("Aura zone = %s, want graveyard (sacrificed as a cost)", ao.Zone)
	}
	if gz := e.G.Obj(giant).Zone; gz != state.ZExile {
		t.Fatalf("Hill Giant zone = %s, want exile", gz)
	}
}

// TestUneasyAllianceDoesNotReachBouncedBearer is the negative case: the
// enchanted creature is bounced in response to the activation, so by
// resolution the Aura is in the graveyard (the source left the battlefield)
// and the Giant is in hand. The LKI fallback names the Giant, but Uneasy's
// ChangeZone carries Origin$ Battlefield, so the card in hand is never moved:
// the ability must not reach it.
func TestUneasyAllianceDoesNotReachBouncedBearer(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, ids := attachedToFixture(t, reg, 9411, []string{"Uneasy Alliance"}, []string{"Hill Giant"}, nil)
	giant, aura := ids["Hill Giant"], ids["Uneasy Alliance"]

	addMana(t, e, 0, "1W")
	castNamed(t, e, "Uneasy Alliance")
	attachDrain(t, e, 40)
	if ao := e.G.Obj(aura); ao.Zone != state.ZBattlefield || ao.AttachedTo != giant {
		t.Fatalf("precondition: Aura must be attached to the Giant on the battlefield, got %+v", ao)
	}

	addMana(t, e, 0, "CCCCC")
	submitChoices(t, e, abilityOption(t, e, aura, 0).Index)
	// Bounce the enchanted creature in response to the activation.
	e.emit(events.Event{Kind: events.MoveZone, Obj: giant, From: state.ZBattlefield, To: state.ZHand})
	attachDrain(t, e, 40)

	if gz := e.G.Obj(giant).Zone; gz != state.ZHand {
		t.Fatalf("Hill Giant zone = %s, want hand (the ability must not reach a bounced bearer)", gz)
	}
	if ao := e.G.Obj(aura); ao.Zone != state.ZGraveyard {
		t.Fatalf("Aura zone = %s, want graveyard (sacrificed as a cost)", ao.Zone)
	}
}

// TestRingOfEvosIsleReturnedUnattachedGivesNoHexproof pins CR 400.7: an
// Equipment that left the battlefield and returned unattached is a new
// object, so its "{2}: Equipped creature gains hexproof" has no equipped
// creature and must do nothing.
func TestRingOfEvosIsleReturnedUnattachedGivesNoHexproof(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, ids := attachedToFixture(t, reg, 9411, nil, []string{"Ring of Evos Isle", "Hill Giant"}, nil)
	giant, ring := ids["Hill Giant"], ids["Ring of Evos Isle"]

	e.emit(events.Event{Kind: events.Attach, Obj: ring, IDs: []state.ObjID{giant}})
	if got := e.G.Obj(ring).AttachedTo; got != giant {
		t.Fatalf("precondition: Ring AttachedTo = %d, want %d", got, giant)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: ring, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: ring, From: state.ZGraveyard, To: state.ZBattlefield})
	ro := e.G.Obj(ring)
	if ro.Zone != state.ZBattlefield || ro.AttachedTo != 0 || ro.LastBearer != 0 {
		t.Fatalf("precondition: returned Ring %+v, want battlefield / AttachedTo 0 / LastBearer 0", ro)
	}
	if e.HasKeyword(giant, "Hexproof") {
		t.Fatalf("precondition: the Giant already has hexproof before the activation")
	}

	addMana(t, e, 0, "CC")
	submitChoices(t, e, abilityOption(t, e, ring, 0).Index)
	attachDrain(t, e, 40)

	if e.HasKeyword(giant, "Hexproof") {
		t.Errorf("BROKE: an unattached returned Ring of Evos Isle gave hexproof to its pre-zone-change bearer")
	}
}

// attachLKICarriers is every corpus card measured on 2026-10-12 to carry an
// activated ability whose cost sacrifices the source itself AND whose
// Defined$ is Enchanted|Equipped|AttachedTo (the bearer is read after the
// source left the battlefield):
//
//	/usr/bin/grep -rlE '^A:.*Sac<1/CARDNAME[^>]*>.*Defined\$ (Enchanted|Equipped|AttachedTo)\b|^A:.*Defined\$ (Enchanted|Equipped|AttachedTo)\b.*Sac<1/CARDNAME' .cards/cardsfolder | wc -l  ==  10
//
// All ride effects ResolveDefined's definedSpecEquipped LastBearer fallback.
var attachLKICarriers = []string{
	"Briar Shield", "Candletrap", "Carapace", "Choking Restraints",
	"Path to Redemption", "Phantom Wings", "Spiral into Solitude", "Stamina",
	"Thrull Retainer", "Uneasy Alliance",
}

// attachLKIShapeRE matches an activated-ability line carrying BOTH a
// self-sacrifice cost of the source and a Defined$ Enchanted|Equipped|
// AttachedTo read, in either order. It is the
// same expression the pinned grep in attachLKICarriers' comment uses.
var attachLKIShapeRE = regexp.MustCompile(`(?m)^A:.*Sac<1/CARDNAME[^>]*>.*Defined\$ (Enchanted|Equipped|AttachedTo)\b|^A:.*Defined\$ (Enchanted|Equipped|AttachedTo)\b.*Sac<1/CARDNAME`)

// attachLKICarrierName reports the corpus card name a file declares when the
// file carries the shape.
func attachLKICarrierName(src string) (string, bool) {
	if !attachLKIShapeRE.MatchString(src) {
		return "", false
	}
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "Name:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Name:")), true
		}
	}
	return "", true
}

// cardHasAttachLKIShape is the PARSED half of the shape test (the walk is the
// raw-line half): a card that still compiles to a self-sacrifice ability whose
// Defined$ is Enchanted|Equipped|AttachedTo.
func cardHasAttachLKIShape(c *cards.Card) bool {
	for _, f := range c.Faces {
		for _, sa := range f.Abilities {
			switch sa.ParamStr(cards.PKDefined) {
			case "Enchanted", "Equipped", "AttachedTo":
				if strings.Contains(sa.ParamStr(cards.PKCost), "Sac<1/CARDNAME") {
					return true
				}
			}
		}
	}
	return false
}

// TestAttachLKICarrierCensus ratchets the class in BOTH directions: every
// pinned carrier still parses to the shape, and the pinned list is exactly the
// corpus carriers the raw-line expression finds -- so a script that drops the
// shape fails AND a new carrier fails.
func TestAttachLKICarrierCensus(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	for _, name := range attachLKICarriers {
		if c := searchCorpusCard(t, reg, name); !cardHasAttachLKIShape(c) {
			t.Errorf("%s no longer carries a self-sacrifice + Defined$ Enchanted|Equipped|AttachedTo ability", name)
		}
	}
	root := filepath.Join("..", "..", ".cards", "cardsfolder")
	var got []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if name, ok := attachLKICarrierName(string(b)); ok {
			got = append(got, name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	want := append([]string(nil), attachLKICarriers...)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("self-sacrifice + Defined$ Enchanted|Equipped|AttachedTo corpus carriers changed: got %d, pinned %d\n got: %v\nwant: %v",
			len(got), len(want), got, want)
	}
}
