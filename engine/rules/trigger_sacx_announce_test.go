package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A trigger's own announced Sac<X/Spec> cost (Sephiroth, One-Winged Angel's
// "you may sacrifice any number of other creatures. If you do, draw that many
// cards"; CR 601.2b, 118.3): the payer announces X over 0..the other creatures
// they control, sacrifices exactly X of them, and the body draws X. Before the
// fix the window posed decline only. Inline synthetic fixtures only.

const sacXDrawScript = "Name:SacX Draw\nManaCost:B\nTypes:Creature Human\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw\n" +
	"SVar:TrigDraw:AB$ Draw | NumCards$ X | Cost$ Sac<X/Creature.Other/another creature>\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

// sacXWindow puts two other creatures onto seat 0's battlefield, then the
// fixture, and returns the engine, the two creatures and the X ask.
func sacXWindow(t *testing.T) (*Engine, Config, [2]state.ObjID, *decision.Decision) {
	t.Helper()
	const bearA = "Name:SacX Bear A\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
	const bearB = "Name:SacX Bear B\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
	e, cfg, src := newFixtureDeck(t, 1, sacXDrawScript, bearA, bearB)
	e.pending = nil
	bears := [2]state.ObjID{
		namedCardInZones(t, e, 0, "SacX Bear A", state.ZHand, state.ZLibrary),
		namedCardInZones(t, e, 0, "SacX Bear B", state.ZHand, state.ZLibrary),
	}
	for _, id := range append(bears[:], src) {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield})
		}
	}
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the trigger-cost X ask, got %+v", d)
	}
	return e, cfg, bears, d
}

// paySacX answers the announced X, pays the folded sacrifice, and answers the
// settle walk's picks with the first offered candidate.
func paySacX(t *testing.T, e *Engine, d *decision.Decision, x int, candidates ...state.ObjID) {
	t.Helper()
	submitChoices(t, e, xOption(t, d, x))
	d = e.Pending()
	pay, _ := windowPayDecline(t, d)
	if pay < 0 {
		t.Fatalf("Sac<%d> was not offered as payable: %+v", x, d.Options)
	}
	submitChoices(t, e, pay)
	// The settle walk asks for the sacrificed creatures, one pick at a time or
	// as one bulk choose of the required count; answer each with the first
	// offered candidates.
	for i := 0; i < 4; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Player != 0 {
			break
		}
		want := d.Min
		if want < 1 {
			want = 1
		}
		choices := make([]int, 0, want)
		for _, o := range d.Options {
			if o.Obj == 0 {
				continue
			}
			for _, id := range candidates {
				if o.Obj == id {
					choices = append(choices, o.Index)
					break
				}
			}
			if len(choices) == want {
				break
			}
		}
		if len(choices) == 0 {
			break
		}
		submitChoices(t, e, choices...)
	}
}

func TestTriggerSacXAnnouncesAndPays(t *testing.T) {
	t.Parallel()
	e, cfg, bears, d := sacXWindow(t)
	// Two other creatures: X = 0..2, ascending (option 0 the decline-equivalent).
	if len(d.Options) != 3 {
		t.Fatalf("X ask options = %+v, want X = 0..2", d.Options)
	}
	for i, o := range d.Options {
		if o.Kind != "trigger_cost_x" || o.Amount != i {
			t.Fatalf("option %d = %+v, want trigger_cost_x X=%d ascending", i, o, i)
		}
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	paySacX(t, e, d, 2, bears[:]...)
	drainEvidenceXTriggers(t, e)
	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("creature %d in %s, want graveyard (sacrificed)", id, z)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 2 {
		t.Fatalf("cards drawn = %d, want 2", got)
	}
	replayCheck(t, e, cfg)
}

// TestTriggerSacXPartialCount announces an X below the candidate count: X = 1
// of two other creatures must sacrifice exactly one and leave the other, not
// sacrifice all of them (which an all-or-nothing settle would also satisfy the
// full-count test above).
func TestTriggerSacXPartialCount(t *testing.T) {
	t.Parallel()
	e, cfg, bears, d := sacXWindow(t)
	hand := len(e.G.Zone(state.ZHand, 0))
	paySacX(t, e, d, 1, bears[:]...)
	drainEvidenceXTriggers(t, e)
	grave, alive := 0, 0
	for _, id := range bears {
		switch z := e.G.Obj(id).Zone; z {
		case state.ZGraveyard:
			grave++
		case state.ZBattlefield:
			alive++
		default:
			t.Fatalf("creature %d in %s, want graveyard or battlefield", id, z)
		}
	}
	if grave != 1 || alive != 1 {
		t.Fatalf("X=1 sacrificed %d and left %d, want exactly one of the two", grave, alive)
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 1 {
		t.Fatalf("cards drawn = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

func TestTriggerSacXZeroIsDecline(t *testing.T) {
	t.Parallel()
	e, _, bears, d := sacXWindow(t)
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, xOption(t, d, 0))
	if d = e.Pending(); d != nil {
		if pay, decline := windowPayDecline(t, d); pay >= 0 {
			t.Fatalf("X=0 offered a pay option: %+v", d.Options)
		} else {
			submitChoices(t, e, decline)
		}
	}
	drainEvidenceXTriggers(t, e)
	for _, id := range bears {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Fatalf("X=0 moved creature %d to %s", id, z)
		}
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 0 {
		t.Fatalf("X=0 drew %d cards", got)
	}
}

// Nyssa of Traken / Campsite Cuisine shape: the trigger's own cost is an
// ImmediateTrigger `Sac<X/...>`, and the reflexive body's target bound and
// draw read the same announced X (Count$xPaid). An inline fixture covers the
// shape without the corpus.
const sacXReflexiveScript = "Name:SacX Reflexive\nManaCost:B\nTypes:Enchantment\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ SacRefCost\n" +
	"SVar:SacRefCost:AB$ ImmediateTrigger | Cost$ Sac<X/Artifact> | Execute$ SacRefTap\n" +
	"SVar:SacRefTap:DB$ Tap | TargetMin$ 0 | TargetMax$ X | ValidTgts$ Creature | TgtPrompt$ Tap up to X target creatures | SubAbility$ SacRefDraw\n" +
	"SVar:SacRefDraw:DB$ Draw | NumCards$ X\n" +
	"SVar:X:Count$xPaid\nOracle:x\n"

// sacXReflexiveWindow puts three artifact creatures onto seat 0's battlefield,
// then the fixture, and returns the X ask of the ImmediateTrigger's cost.
func sacXReflexiveWindow(t *testing.T) (*Engine, Config, []state.ObjID, *decision.Decision) {
	t.Helper()
	const relicA = "Name:SacX Relic A\nManaCost:0\nTypes:Artifact Creature Golem\nPT:0/1\nOracle:x\n"
	const relicB = "Name:SacX Relic B\nManaCost:0\nTypes:Artifact Creature Golem\nPT:0/1\nOracle:x\n"
	const relicC = "Name:SacX Relic C\nManaCost:0\nTypes:Artifact Creature Golem\nPT:0/1\nOracle:x\n"
	e, cfg, src := newFixtureDeck(t, 1, sacXReflexiveScript, relicA, relicB, relicC)
	e.pending = nil
	relics := []state.ObjID{
		namedCardInZones(t, e, 0, "SacX Relic A", state.ZHand, state.ZLibrary),
		namedCardInZones(t, e, 0, "SacX Relic B", state.ZHand, state.ZLibrary),
		namedCardInZones(t, e, 0, "SacX Relic C", state.ZHand, state.ZLibrary),
	}
	for _, id := range append(append([]state.ObjID(nil), relics...), src) {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield})
		}
	}
	e.putTriggersOnStack()
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the trigger-cost X ask, got %+v", d)
	}
	return e, cfg, relics, d
}

// TestTriggerSacXImmediateTriggerBodySeesX pays X = 2 of three artifacts and
// then asserts the reflexive body's `TargetMax$ X` is 2 and the body draws 2:
// the announced X reaches the minted trigger, not just the sacrifice.
func TestTriggerSacXImmediateTriggerBodySeesX(t *testing.T) {
	t.Parallel()
	e, cfg, relics, d := sacXReflexiveWindow(t)
	if len(d.Options) != 4 {
		t.Fatalf("X ask options = %+v, want X = 0..3", d.Options)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	paySacX(t, e, d, 2, relics...)
	if len(e.pendingTriggers) == 0 {
		t.Fatalf("the paid ImmediateTrigger minted no reflexive trigger")
	}
	e.putTriggersOnStack()
	d = e.Pending()
	if d == nil && len(e.G.Stack) > 0 {
		e.resolveTop()
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("reflexive body target ask = %+v", d)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("reflexive target bounds = %d..%d, want 0..2 (the paid X)", d.Min, d.Max)
	}
	submitChoices(t, e, d.Options[0].Index)
	drainEvidenceXTriggers(t, e)
	grave, alive := 0, 0
	for _, id := range relics {
		switch z := e.G.Obj(id).Zone; z {
		case state.ZGraveyard:
			grave++
		case state.ZBattlefield:
			alive++
		default:
			t.Fatalf("relic %d in %s, want graveyard or battlefield", id, z)
		}
	}
	if grave != 2 || alive != 1 {
		t.Fatalf("X=2 sacrificed %d and left %d of the three artifacts", grave, alive)
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand; got != 2 {
		t.Fatalf("cards drawn = %d, want 2", got)
	}
	replayCheck(t, e, cfg)
}
