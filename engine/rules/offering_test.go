// offering_test.go proves CR 702.48: "[Quality] offering" is an optional
// additional cost (sacrifice a [quality] permanent) that reduces the spell's
// total cost by the sacrificed permanent's mana cost, symbol by symbol
// (702.48c, NOT by mana value like Emerge), and lets the spell be cast any
// time its controller could cast an instant. The carriers are real corpus
// cards (Patron of the Moon, Patron of the Akki); the sacrificed permanents
// are freely authored fixtures.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	// {1}{U} Moonfolk: Patron of the Moon's {5}{U}{U} drops to {4}{U}.
	offeringMoonfolkSrc = "Name:Offering Moonfolk\nManaCost:1 U\nTypes:Creature Moonfolk\nPT:1/1\nOracle:x\n"
	// {1}{R} Moonfolk: its red pip matches nothing in {5}{U}{U}.
	offeringRedMoonfolkSrc = "Name:Offering Red Moonfolk\nManaCost:1 R\nTypes:Creature Moonfolk\nPT:1/1\nOracle:x\n"
	// {1}{R} Goblin: Patron of the Akki's {4}{R}{R} drops to {3}{R}
	// (mana-value reduction would give {2}{R}{R}).
	offeringGoblinSrc = "Name:Offering Goblin\nManaCost:1 R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
	// {4}{R}{R} Goblin: reduces Patron of the Akki's whole cost to {0}.
	offeringGoblinColossusSrc = "Name:Offering Goblin Colossus\nManaCost:4 R R\nTypes:Creature Goblin\nPT:6/6\nOracle:x\n"
)

// castOfferingSacrificing submits the "offering" cast option for id, answers
// the sacrifice ask with sacObj, and drains the cast until the spell has
// resolved onto the battlefield.
func castOfferingSacrificing(t *testing.T, e *Engine, id, sacObj state.ObjID) {
	t.Helper()
	submitChoices(t, e, castModeOption(t, e, id, "offering"))
	for i := 0; i < 80; i++ {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZBattlefield && len(e.G.Stack) == 0 {
			return
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("offering cast stalled with no pending decision (stack depth %d)", len(e.G.Stack))
		}
		switch d.Kind {
		case decision.KChoose:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "sacrifice" && o.Obj == sacObj {
					idx = o.Index
				}
			}
			if idx < 0 {
				for _, o := range d.Options {
					if o.Kind == "done" {
						idx = o.Index
					}
				}
			}
			if idx < 0 {
				t.Fatalf("unexpected KChoose during offering cast: %+v", d)
			}
			submitChoices(t, e, idx)
		case decision.KPriority:
			passOnceP(t, e)
		default:
			t.Fatalf("unexpected decision during offering cast: %+v", d)
		}
	}
	t.Fatal("offering cast never resolved")
}

// TestPatronOfTheMoonOfferingReducesByMoonfolkCost: sacrificing a Moonfolk with
// mana cost {1}{U} reduces Patron of the Moon's {5}{U}{U} to {4}{U}.
func TestPatronOfTheMoonOfferingReducesByMoonfolkCost(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 5101, []string{"Patron of the Moon"}, []string{offeringMoonfolkSrc}, nil)
	patron := findCardObj(t, e, 0, "Patron of the Moon", state.ZHand)
	moon := findCardObj(t, e, 0, "Offering Moonfolk", state.ZBattlefield)

	if q, ok := offeringQuality(e.G.Obj(patron).Face()); !ok || q != "Moonfolk" {
		t.Fatalf("offering quality = %q ok=%v, want Moonfolk", q, ok)
	}
	if base := e.castOfferBase(0, patron); base.Generic != 5 || base.Colored[state.MU] != 2 {
		t.Fatalf("printed cost %+v, want {5}{U}{U}", base)
	}
	if mf := e.G.Obj(moon); mf.Zone != state.ZBattlefield || mf.Controller != 0 {
		t.Fatalf("Moonfolk not a controlled permanent: %+v", mf)
	}

	// Fund 2 blue + 5 generic: exactly the unreduced cost. Only {4}{U} may be
	// taken, so 2 mana must remain (and the offer must exist at all: a pool
	// of 5 would not pay {5}{U}{U}).
	addMana(t, e, 0, "UUCCCCC")
	castOfferingSacrificing(t, e, patron, moon)
	if o := e.G.Obj(moon); o.Zone != state.ZGraveyard {
		t.Fatalf("sacrificed Moonfolk in %s, want graveyard", o.Zone)
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 2 {
		t.Fatalf("mana remaining %+v, want 2 ({5}{U}{U} reduced to {4}{U} from 7)", pool)
	}
	replayCheck(t, e, cfg)
}

// TestOfferingReductionIsPerSymbolNotManaValue: the reduction subtracts the
// sacrificed cost symbol by symbol (CR 702.48c), so a red pip never reduces a
// blue pip, and a Goblin {1}{R} takes Patron of the Akki {4}{R}{R} to {3}{R}
// (a mana-value reduction would leave {2}{R}{R}).
func TestOfferingReductionIsPerSymbolNotManaValue(t *testing.T) {
	t.Parallel()
	// The pure reducer first.
	patronMoon := ParseCost("5 U U")
	got := offeringReduce(patronMoon, "1 R")
	if got.Colored[state.MU] != 2 || got.Colored[state.MR] != 0 {
		t.Fatalf("red pip touched the blue requirement: %+v", got)
	}
	// 702.48c: the unmatched red pip is excess and reduces generic.
	if got.Generic != 3 {
		t.Fatalf("{5}{U}{U} less {1}{R} = %+v, want generic 3 (generic 1 + excess red 1)", got)
	}
	got = offeringReduce(ParseCost("4 R R"), "1 R")
	if got.Generic != 3 || got.Colored[state.MR] != 1 {
		t.Fatalf("{4}{R}{R} less {1}{R} = %+v, want {3}{R}", got)
	}
	if got = offeringReduce(ParseCost("U U"), "6"); got.Generic != 0 || got.Colored[state.MU] != 2 {
		t.Fatalf("{U}{U} less {6} = %+v, want {U}{U} (no negative cost, coloured intact)", got)
	}
	// A hybrid pip reduces the half the spell's cost has.
	if got = offeringReduce(ParseCost("3 U"), "W/U"); got.Generic != 3 || got.Colored[state.MU] != 0 {
		t.Fatalf("{3}{U} less {W/U} = %+v, want {3}", got)
	}

	// End to end: the cast charges {3}{R}.
	e, cfg, _ := altCostEngine(t, 5102, []string{"Patron of the Akki"}, []string{offeringGoblinSrc}, nil)
	akki := findCardObj(t, e, 0, "Patron of the Akki", state.ZHand)
	gob := findCardObj(t, e, 0, "Offering Goblin", state.ZBattlefield)
	// 2 red + 4 generic funds the unreduced {4}{R}{R}; {3}{R} leaves 2.
	addMana(t, e, 0, "RRCCCC")
	castOfferingSacrificing(t, e, akki, gob)
	if pool := e.G.Players[0].Pool; pool.Total() != 2 {
		t.Fatalf("mana remaining %+v, want 2 ({4}{R}{R} reduced to {3}{R} from 6)", pool)
	}
	replayCheck(t, e, cfg)
}

// TestOfferingCastAtInstantSpeed: with a Goblin in play, Patron of the Akki is
// offered in the opponent's end step; without one it is not.
func TestOfferingCastAtInstantSpeed(t *testing.T) {
	t.Parallel()
	for _, withGoblin := range []bool{true, false} {
		fixtures := []string{}
		if withGoblin {
			fixtures = append(fixtures, offeringGoblinColossusSrc)
		}
		e, _, _ := altCostEngine(t, 5103, []string{"Patron of the Akki"}, fixtures, nil)
		akki := findCardObj(t, e, 0, "Patron of the Akki", state.ZHand)
		if withGoblin {
			findCardObj(t, e, 0, "Offering Goblin Colossus", state.ZBattlefield)
		}
		driveToStep(t, e, e.G.Turn+1, 1, state.StepEnd)
		// Seat 1 holds priority first; seat 0 then sees the end step.
		submitPass(t, e)
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority || d.Player != 0 || e.G.Active != 1 || e.G.Step != state.StepEnd {
			t.Fatalf("withGoblin=%v: want seat 0 priority in seat 1's end step, got %+v (active %d, %s)", withGoblin, d, e.G.Active, e.G.Step)
		}
		offered, plain := false, false
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == akki {
				switch o.Mode {
				case "offering":
					offered = true
				case "":
					plain = true
				}
			}
		}
		if plain {
			t.Fatalf("withGoblin=%v: plain sorcery-speed cast offered at the opponent's end step", withGoblin)
		}
		if offered != withGoblin {
			t.Fatalf("withGoblin=%v: offering offered=%v", withGoblin, offered)
		}
	}
}

// TestOfferingIsOptional: the normal cast stays offered at sorcery timing next
// to the offering cast, and casting it normally leaves the Goblin alone.
func TestOfferingIsOptional(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 5104, []string{"Patron of the Akki"}, []string{offeringGoblinSrc}, nil)
	akki := findCardObj(t, e, 0, "Patron of the Akki", state.ZHand)
	gob := findCardObj(t, e, 0, "Offering Goblin", state.ZBattlefield)
	addMana(t, e, 0, "RRCCCC")
	d := e.Pending()
	plain, offered := false, false
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == akki {
			plain = plain || o.Mode == ""
			offered = offered || o.Mode == "offering"
		}
	}
	if !plain || !offered {
		t.Fatalf("main phase: plain=%v offering=%v, want both: %+v", plain, offered, d.Options)
	}
	submitChoices(t, e, castModeOption(t, e, akki, ""))
	for i := 0; i < 40 && e.G.Obj(akki).Zone != state.ZBattlefield; i++ {
		switch d := e.Pending(); d.Kind {
		case decision.KPriority:
			passOnceP(t, e)
		default:
			idx := d.Options[0].Index
			for _, o := range d.Options {
				if o.Kind == "done" {
					idx = o.Index
				}
			}
			submitChoices(t, e, idx)
		}
	}
	if e.G.Obj(akki).Zone != state.ZBattlefield {
		t.Fatal("plain cast never resolved")
	}
	if o := e.G.Obj(gob); o.Zone != state.ZBattlefield {
		t.Fatalf("the Goblin was sacrificed by the plain cast (zone %s)", o.Zone)
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("plain cast left %+v, want the full {4}{R}{R} paid", pool)
	}
	replayCheck(t, e, cfg)
}

// TestEveryOfferingCardIsPlayable: the six Offering cards leave Unsupported.
func TestEveryOfferingCardIsPlayable(t *testing.T) {
	t.Parallel()
	if !effects.Supported()["kw:Offering"] {
		t.Fatal(`effects.Supported() is missing "kw:Offering"`)
	}
	reg := testutil.CorpusRegistry(t)
	names := []string{"Blast-Furnace Hellkite", "Patron of the Akki", "Patron of the Kitsune",
		"Patron of the Moon", "Patron of the Nezumi", "Patron of the Orochi"}
	for _, name := range names {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if _, ok := offeringQuality(c.Faces[0]); !ok {
			t.Errorf("%s: no parseable Offering quality", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
