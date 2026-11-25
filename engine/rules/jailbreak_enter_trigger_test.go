package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The fixture is authored here, not copied from the corpus: the same shape
// (a ChangeZone whose sub-ability registers a delayed enters trigger over the
// ability's own target) with its own key order and no prompt text.
const jailbreakScript = `Name:Jailbreak
ManaCost:1 W
Types:Sorcery
A:SP$ ChangeZone | ValidTgts$ Permanent.OppOwn | Origin$ Graveyard | Destination$ Battlefield | SubAbility$ DBWatch
SVar:DBWatch:DB$ DelayedTrigger | Mode$ ChangesZone | RememberObjects$ Targeted | ValidCard$ Card.IsTriggerRemembered | Destination$ Battlefield | Origin$ Any | Execute$ TrigBack
SVar:TrigBack:DB$ ChangeZone | TargetMin$ 0 | TargetMax$ 1 | ValidTgts$ Permanent.YouOwn+cmcLECap | Origin$ Graveyard | Destination$ Battlefield
SVar:Cap:TriggeredCard$CardManaCost
Oracle:x
`

// jailbreakEngine seats Jailbreak in seat 0's hand with its mana paid, an
// opponent's Memnite in the opponent's graveyard and a Plains in seat 0's.
func jailbreakEngine(t *testing.T) (e *Engine, spell, memnite, plains state.ObjID) {
	t.Helper()
	jb := card(t, jailbreakScript)
	e = handEngine(t, jb)
	spell = e.G.Zone(state.ZHand, 0)[0]
	memnite = placeInZone(t, e, 1, state.ZGraveyard, card(t,
		"Name:Memnite\nManaCost:0\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n"))
	plains = placeInZone(t, e, 0, state.ZGraveyard, card(t,
		"Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"))
	addMana(t, e, 0, "1W")
	return e, spell, memnite, plains
}

// CR 603.12 / Forge's waiting-trigger pass: Jailbreak registers its "when that
// permanent enters" delayed trigger AFTER its own ChangeZone returned the
// target, inside the same resolution, and the entry it just made still
// triggers it. The trigger goes on the stack once the spell is done, and its
// resolution returns the Plains from the caster's own graveyard.
func TestJailbreakEnterTriggerFiresForReturnedPermanent(t *testing.T) {
	t.Parallel()
	e, spell, memnite, plains := jailbreakEngine(t)
	if z := e.G.Obj(memnite).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Memnite in %v, want the graveyard", z)
	}
	castSpellObjTarget(t, e, spell, memnite)
	passUntil(t, e, func() bool {
		d := e.Pending()
		return len(e.G.Stack) == 0 || (d != nil && d.Kind == decision.KTarget)
	})
	if o := e.G.Obj(memnite); o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("Memnite in %v under seat %d, want seat 1's battlefield", o.Zone, o.Controller)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after Jailbreak resolved the pending ask is %+v, want the trigger's target ask", d)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack holds %d objects, want only the Jailbreak trigger", len(e.G.Stack))
	}
	if g := e.G.Obj(plains).Zone; g != state.ZGraveyard {
		t.Fatalf("precondition: Plains in %v before the trigger resolves", g)
	}
	answerAsk(t, e, optionIdxForObj(t, d, plains))
	drainQueuedTriggers(t, e)
	if o := e.G.Obj(plains); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("Plains in %v under seat %d, want seat 0's battlefield", o.Zone, o.Controller)
	}
	if n := len(e.G.Delayed); n != 0 {
		t.Fatalf("%d delayed registrations left over, want the one-shot spent", n)
	}
}

// castSpellObjTarget casts id and answers its target ask with target.
func castSpellObjTarget(t *testing.T, e *Engine, id, target state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %+v", d)
	}
	submitChoices(t, e, optionIdxForCast(t, d, id))
	for i := 0; i < 8; i++ {
		d = e.Pending()
		if d != nil && d.Kind == decision.KTarget {
			submitChoices(t, e, optionIdxForObj(t, d, target))
			return
		}
		if d == nil || d.Kind != decision.KChoose {
			break
		}
		submitChoices(t, e, 0)
	}
	t.Fatalf("no target ask after casting %d: %+v", id, e.Pending())
}

func optionIdxForCast(t *testing.T, d *decision.Decision, id state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			return o.Index
		}
	}
	t.Fatalf("no cast option for %d: %+v", id, d.Options)
	return -1
}
