package rules

// Destroy's RememberLKI$ must hand its follow-up condition the destroyed
// permanent's last-known controller and layer-5 colours (CR 608.2h), not the
// graveyard card's: the Move resets the controller to the owner and a
// graveyard object answers its printed colours.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const drbEnchantSrc = "Name:Probe Enchantment\nManaCost:1\nTypes:Enchantment\nOracle:x\n"

// breakTheSpellHand casts Break the Spell from seat 0 on a Probe Enchantment
// owned by seat 1 and controlled by `ctrl`, and returns the hand size before
// the cast and after resolution.
func breakTheSpellHand(t *testing.T, ctrl state.PlayerID, indestructible bool) (before, after int) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	bts := mustCorpusCard(t, reg, "Break the Spell")
	ench := card(t, drbEnchantSrc)
	e, _ := tokenReplGameSeats(t, 71, []*cards.Card{bts}, []*cards.Card{ench})
	id := moveSeededCard(t, e, 1, ench, state.ZBattlefield)
	e.G.Obj(id).Controller = ctrl
	if o := e.G.Obj(id); o.Controller != ctrl || o.Owner != 1 || o.Zone != state.ZBattlefield {
		t.Fatalf("enchantment precondition: %+v", o)
	}
	if indestructible {
		e.AddContinuous(state.ContinuousEffect{Source: id, Affects: "Card.Self", Layer: state.LAbilities,
			AddKeywords: []string{"Indestructible"}})
	}
	ensureInHand(t, e, 0, "Break the Spell")
	before = handSize(e, 0)
	castCorpusSpell(t, e, "Break the Spell", "W", id)
	if want := state.ZGraveyard; !indestructible && e.G.Obj(id).Zone != want {
		t.Fatalf("enchantment zone = %v, want graveyard", e.G.Obj(id).Zone)
	}
	if indestructible && e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatalf("indestructible enchantment left the battlefield: %v", e.G.Obj(id).Zone)
	}
	return before, handSize(e, 0)
}

func TestBreakTheSpellStolenEnchantmentDraws(t *testing.T) {
	t.Parallel()
	// Seat 0 controls the enchantment seat 1 owns: spell leaves (-1), draw (+1).
	if before, after := breakTheSpellHand(t, 0, false); after != before {
		t.Fatalf("stolen enchantment: hand before=%d after=%d, want equal (drew a card)", before, after)
	}
	// Control: seat 1 owns and controls it, so no draw.
	if before, after := breakTheSpellHand(t, 1, false); after != before-1 {
		t.Fatalf("opponent's enchantment: hand before=%d after=%d, want %d", before, after, before-1)
	}
}

// A destroy that does not happen records no last-known capture: an
// indestructible enchantment the caster controls gives no draw.
func TestDestroyRememberLKIOnlyRecordsMovedPermanents(t *testing.T) {
	t.Parallel()
	if before, after := breakTheSpellHand(t, 0, true); after != before-1 {
		t.Fatalf("indestructible own enchantment: hand before=%d after=%d, want %d", before, after, before-1)
	}
}

func TestGloomwidowsFeastReadsDerivedColourOfDestroyedFlyer(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, recolour string) int {
		reg := testutil.CorpusRegistry(t)
		feast := mustCorpusCard(t, reg, "Gloomwidow's Feast")
		flyer := card(t, "Name:Probe Flyer\nManaCost:U\nTypes:Creature\nPT:1/1\nK:Flying\nOracle:x\n")
		e, _ := tokenReplGameSeats(t, 71, []*cards.Card{feast}, []*cards.Card{flyer})
		id := moveSeededCard(t, e, 1, flyer, state.ZBattlefield)
		if got := e.LayerColors(e.G.Obj(id)); got != "U" {
			t.Fatalf("flyer printed colours = %q, want U", got)
		}
		if recolour != "" {
			e.AddContinuous(state.ContinuousEffect{Source: id, Affects: "Card.Self", Layer: state.LColor,
				AddColors: []string{recolour}, OverwriteColors: true})
			if got := e.LayerColors(e.G.Obj(id)); got != recolour {
				t.Fatalf("flyer derived colours = %q, want %q", got, recolour)
			}
		}
		castCorpusSpell(t, e, "Gloomwidow's Feast", "CCCG", id)
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("flyer zone = %v, want graveyard", z)
		}
		return countTokensNamed(t, e, "Spider Token")
	}
	t.Run("recoloured green gets no Spider", func(t *testing.T) {
		if n := run(t, "G"); n != 0 {
			t.Fatalf("spiders = %d, want 0 (the flyer was green when destroyed)", n)
		}
	})
	t.Run("unmodified blue gets one Spider", func(t *testing.T) {
		if n := run(t, ""); n != 1 {
			t.Fatalf("spiders = %d, want 1", n)
		}
	})
}
