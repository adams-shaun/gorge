package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A resolution-generated keyword grant whose Affected$ reads power (Temur
// Charm mode 3: "Creatures with power 3 or less can't block this turn",
// SVar:KWPump Affected$ Creature.powerLE3 AddHiddenKeyword$ CARDNAME can't
// block.) must gate on the object's layer-7 characteristics (CR 613: layers
// apply in order, so by the time the grant's gate is evaluated the Oakenform
// pump on it has applied; CR 611.2c: a continuous effect's affected set is
// determined with the characteristics as they then are), not on the printed
// face. Mesa Cavalier (printed 2/1) carrying Oakenform (+3/+3, layer 7) is
// power 5 when the charm resolves: it must keep blocking, while an unmodified
// Grizzly Bears (2/2) is granted the can't-block text. This is the
// gorge-vs-dirac lockstep pair 1232 class (kernel offered
// block(#defender,#Mesa-Cavalier-like), gorge offered [pass]).
func TestPowerleGrantReadsDerivedPower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Mesa Cavalier"), lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Giant Growth"), lookup(t, reg, "Temur Charm")},
		nil)
	cav := moveCorpusCard(t, e, "Mesa Cavalier", 0, state.ZBattlefield)
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	moveCorpusCard(t, e, "Giant Growth", 0, state.ZHand)
	if e.G.Active != 0 {
		// Both protagonists sit in seat 0's deck; the pump and the charm are
		// casts of its own, so drive to its Main1.
		driveToStep(t, e, e.G.Turn+1, 0, state.StepMain1)
	}

	// Giant Growth onto the Cavalier through the real cast flow (its +3/+3
	// is a layer-7 pump registered by the resolved spell). addMana, not
	// fundMana: the pool must be in the log or the replay check below
	// diverges.
	addMana(t, e, 0, "GG")
	castSpellOption(t, e, "Giant Growth")
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while casting Oakenform")
		}
		if d.Kind == decision.KTarget {
			if idx := indexOfObjOption(d, cav); idx >= 0 {
				submitChoices(t, e, idx)
				continue
			}
			t.Fatalf("no target option for the Cavalier: %+v", d.Options)
		}
		if d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	resolveStack(t, e)
	if p := e.Derived(cav).Power; p < 5 {
		t.Fatalf("precondition: Oakenformed Cavalier power = %d, want >= 5", p)
	}
	if p := e.Derived(bears).Power; p != 2 {
		t.Fatalf("precondition: Grizzly Bears power = %d, want 2", p)
	}

	// Seat 0 casts Temur Charm choosing mode 3 (the no-target mode whose
	// Description says "can't block").
	var charm state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Temur Charm" {
			charm = id
		}
	}
	if charm == 0 {
		t.Fatal("precondition: seat 0 has no Temur Charm in hand")
	}
	addMana(t, e, 0, "GUR")
	castSpellOption(t, e, "Temur Charm")
	answered := false
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		switch d.Kind {
		case decision.KModes:
			pick := -1
			for _, o := range d.Options {
				if strings.Contains(o.Label, "can't block") {
					pick = o.Index
				}
			}
			if pick < 0 {
				t.Fatalf("no can't-block mode option: %+v", d.Options)
			}
			submitChoices(t, e, pick)
			answered = true
		case decision.KPriority:
			i = 40
			continue
		default:
			submitChoices(t, e, d.Options[0].Index)
		}
	}
	if !answered {
		t.Fatal("the charm's mode ask never appeared")
	}
	resolveStack(t, e)

	if !derivedKWHolds(e, bears, "can't block") {
		t.Errorf("Grizzly Bears (power 2) was not granted can't-block: %v", e.Keywords(bears))
	}
	if derivedKWHolds(e, cav, "can't block") {
		t.Errorf("the Oakenformed Cavalier (power 5, printed 2) was granted can't-block: %v", e.Keywords(cav))
	}
	replayCheck(t, e, cfg)
}

// hasDerivedKeyword reports whether one of id's derived keywords contains
// want (a substring match, the way HasCantBlockKeyword reads the granted
// hidden text).
func derivedKWHolds(e *Engine, id state.ObjID, want string) bool {
	for _, k := range e.Keywords(id) {
		if strings.Contains(k, want) {
			return true
		}
	}
	return false
}
