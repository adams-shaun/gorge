package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Avatar Destiny (TLA): a dies trigger whose body mills with RememberMilled$
// True and then fetches ChangeType$ Creature.IsRemembered must offer only the
// milled creature cards. The engine seeds the firing trigger's ctx Remembered
// with its event capture (the creature that died).IsRemembered and could be
// returned. Forge's IsRemembered reads the host card's remembered list, which
// never holds the triggering card (CR 603.7: the ability refers only to what
// it says). Fixtures are inline (never corpus .txt).
const remCapProbe = "Name:Destiny Probe\nManaCost:2 G G\nTypes:Enchantment\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigMill | TriggerDescription$ Whenever a creature you control dies, mill cards; return up to one creature card milled this way.\n" +
	"SVar:TrigMill:DB$ Mill | NumCards$ 60 | RememberMilled$ True | SubAbility$ DBReturnCreature\n" +
	"SVar:DBReturnCreature:DB$ ChangeZone | Hidden$ True | Origin$ Graveyard,Exile | Destination$ Battlefield | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Optional$ True | SubAbility$ DBCleanup\n" +
	"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\n" +
	"Oracle:x\n"

const remCapVictim = "Name:Doomed Fixture\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
const remCapMilled = "Name:Milled Fixture\nManaCost:1 G\nTypes:Creature Elk\nPT:3/3\nOracle:x\n"

func TestRememberMilledIsRememberedExcludesTriggerCapture(t *testing.T) {
	t.Parallel()
	e, cfg, probeID := newFixtureDeck(t, 4711, remCapProbe, remCapVictim, remCapMilled, lbPush)
	victimID := moveSeeded(t, e, 0, remCapVictim, state.ZBattlefield)
	moveSeeded(t, e, 0, remCapProbe, state.ZBattlefield)
	pushID := moveSeeded(t, e, 0, lbPush, state.ZHand)
	var milledID state.ObjID
	for _, z := range []state.Zone{state.ZLibrary, state.ZHand} {
		for _, id := range e.G.Zone(z, 0) {
			if e.G.Obj(id).Face().Name == "Milled Fixture" {
				milledID = id
			}
		}
	}
	if milledID == 0 {
		t.Fatal("precondition: Milled Fixture not in seat 0's library or hand")
	}
	if e.G.Obj(milledID).Zone != state.ZLibrary {
		// Mill reads the library: the remembered creature must start there.
		milledID = moveSeeded(t, e, 0, remCapMilled, state.ZLibrary)
	}
	if e.G.Obj(probeID).Zone != state.ZBattlefield || e.G.Obj(victimID).Zone != state.ZBattlefield {
		t.Fatal("precondition: probe and victim must be on the battlefield")
	}

	addMana(t, e, 0, "R")
	submitChoices(t, e, castOptionFor(t, e, pushID).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending %+v, want the destroy spell's target decision", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Obj == victimID {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatal("precondition: victim not offered as the destroy target")
	}
	submitChoices(t, e, tgt)

	var offered []state.ObjID
	asked := false
	for i := 0; i < 60 && !e.G.Over && (len(e.G.Stack) > 0 || e.Pending() != nil && e.Pending().Kind != decision.KPriority); i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining (stack %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KPriority {
			passFirst(t, e)
			continue
		}
		isFetch := false
		for _, o := range d.Options {
			if o.Obj == milledID || o.Obj == victimID {
				isFetch = true
			}
		}
		if !isFetch {
			// The Optional$ "proceed?" gate and any other ask: take the
			// first option so the fetch is reached.
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		asked = true
		pick := -1
		for _, o := range d.Options {
			if o.Obj != 0 {
				offered = append(offered, o.Obj)
			}
			if o.Obj == milledID {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("the milled creature was not offered; options %+v", d.Options)
		}
		submitChoices(t, e, pick)
	}
	if e.G.Obj(victimID).Zone != state.ZGraveyard && e.G.Obj(victimID).Zone != state.ZBattlefield {
		t.Fatalf("victim zone %s", e.G.Obj(victimID).Zone)
	}
	if !asked {
		t.Fatalf("the up-to-one fetch was never posed (milled zone %s, victim zone %s)",
			e.G.Obj(milledID).Zone, e.G.Obj(victimID).Zone)
	}
	for _, id := range offered {
		if id == victimID {
			t.Fatalf("the creature that died (the trigger capture) was offered as a remembered milled card: %v", offered)
		}
	}
	if z := e.G.Obj(milledID).Zone; z != state.ZBattlefield {
		t.Fatalf("milled creature zone %s, want battlefield", z)
	}
	if z := e.G.Obj(victimID).Zone; z != state.ZGraveyard {
		t.Fatalf("victim zone %s, want graveyard", z)
	}
	replayCheck(t, e, cfg)
}
