package rules

// CR 614.12 / 122.6: a lent "other creatures you control enter with an
// additional +1/+1 counter" replacement is matched against the entering
// permanent as it would exist on the battlefield, counters it enters with
// included. A Spacecraft that enters with its threshold of charge counters is
// a creature (its own static), so the rider applies; one that enters without
// them is not, so it does not.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func entryBodyForeignCreatureRun(t *testing.T, withCounters string) int {
	t.Helper()
	search := card(t, "Name:Charge Search\nManaCost:U\nTypes:Sorcery\n"+
		"A:SP$ ChangeZone | Origin$ Library | Destination$ Battlefield | ChangeType$ Spacecraft | ChangeNum$ 1 | "+
		withCounters+"Mandatory$ True | SpellDescription$ x\nOracle:x\n")
	ship := card(t, "Name:Test Ship\nManaCost:2\nTypes:Artifact Spacecraft\nPT:0/4\n"+
		"S:Mode$ Continuous | Affected$ Card.Self+counters_GE6_CHARGE | AddType$ Creature | Description$ x\nOracle:x\n")
	lord := card(t, "Name:Test Counsel\nManaCost:2 U\nTypes:Legendary Creature Human\nPT:1/1\n"+
		"R:Event$ Moved | ValidCard$ Creature.Other+YouCtrl | Destination$ Battlefield | ReplacementResult$ Updated | ReplaceWith$ AddExtraCounter | Description$ x\n"+
		"SVar:AddExtraCounter:DB$ PutCounter | Defined$ ReplacedCard | CounterNum$ 1 | CounterType$ P1P1 | ETB$ True\nOracle:x\n")
	reg := searchTestRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{search, ship, lord}, nil)
	moveByName(t, e, 0, "Charge Search", state.ZHand)
	moveByName(t, e, 0, "Test Counsel", state.ZBattlefield)
	shipID := findByName(e, "Test Ship", 0)
	if shipID == 0 {
		t.Fatal("fixture ship not seeded")
	}
	if o := e.G.Obj(shipID); o.Zone != state.ZLibrary {
		moveByName(t, e, 0, "Test Ship", state.ZLibrary)
	}
	if o := e.G.Obj(findByName(e, "Test Counsel", 0)); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: lord zone = %s", o.Zone)
	}
	e.priorityRound()
	addMana(t, e, 0, "U")
	castNamed(t, e, "Charge Search")
	for i := 0; i < 12; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		switch {
		case d.Kind == decision.KChoose && d.ResumeKind == "search":
			submitChoices(t, e, d.Options[0].Index)
		case d.Kind == decision.KPriority:
			pass := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pass = o.Index
				}
			}
			if pass < 0 {
				t.Fatalf("no pass option: %+v", d.Options)
			}
			submitChoices(t, e, pass)
		default:
			t.Fatalf("unexpected decision: %+v", d)
		}
		if e.G.Obj(shipID).Zone == state.ZBattlefield {
			break
		}
	}
	passUntilStackEmpty(t, e, 20)
	o := e.G.Obj(shipID)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("ship zone = %s, want battlefield", o.Zone)
	}
	return int(o.Counter("P1P1"))
}

func TestEntryBodyForeignCreatureSeesEntryCounterStatics(t *testing.T) {
	if got := entryBodyForeignCreatureRun(t, "WithCountersType$ CHARGE | WithCountersAmount$ 7 | "); got != 1 {
		t.Fatalf("Spacecraft entering with seven charge counters: P1P1 = %d, want 1", got)
	}
	if got := entryBodyForeignCreatureRun(t, ""); got != 0 {
		t.Fatalf("Spacecraft entering without counters: P1P1 = %d, want 0", got)
	}
}
