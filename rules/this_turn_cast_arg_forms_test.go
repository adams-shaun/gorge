package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// The ARGUMENTED !CastSaSource count forms (task castprov2), pinned end to
// end on their two real corpus carriers:
//
//   - Thunder Salvo's `Count$ThisTurnCast_Card.YouCtrl+!CastSaSource/Plus.2`:
//     the op rides the PREDICATE TOKEN, so the exclusion count runs first and
//     the /Plus.2 applies after (X = 2 + other spells you've cast this turn).
//   - Call Forth the Tempest's
//     `Count$ThisTurnCast_Card.YouCtrl+!CastSaSource$CardManaCost`: the
//     matching casts' mana values AGGREGATED instead of counted one each.
//
// The bare-form carriers' pins (Hotheaded Giant et al.) cover the shared
// exclusion read this builds on and stay green byte-identically.

// castSalvoTargeting answers a committed Thunder Salvo cast through its
// target ask and drains the resolution, then returns the damage the target
// took (its printed toughness minus current marked damage is not tracked —
// the assertion reads the Damage events through the object's damage total).
func castSalvoAt(t *testing.T, e *Engine, salvo, target state.ObjID) int {
	t.Helper()
	e.G.Players[0].Pool[state.MC]++
	e.G.Players[0].Pool[state.MR]++
	if d := e.Pending(); d != nil && d.Kind == decision.KPriority {
		// After a previous cast resolved through finishCast the pending ask is
		// the priority round: cast through it (the ordinary flow).
		cidx := -1
		for i, o := range d.Options {
			if o.Kind == "cast" && o.Obj == salvo {
				cidx = i
			}
		}
		if cidx < 0 {
			t.Fatalf("no cast option for the salvo: %+v", d.Options)
		}
		submitChoices(t, e, cidx)
	} else {
		castMode(t, e, salvo, "")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the salvo target ask, got %+v", d)
	}
	tidx := -1
	for i, o := range d.Options {
		if o.Obj == target {
			tidx = i
		}
	}
	if tidx < 0 {
		t.Fatalf("no target option for the wall: %+v", d.Options)
	}
	submitChoices(t, e, tidx)
	finishCast(t, e, salvo)
	return int(e.G.Obj(target).Damage)
}

func TestThunderSalvoXIsTwoPlusOtherSpellsCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Thunder Salvo"), corpusAlternativeCard(t, "Grizzly Bears"))
	wall := e.G.AddObject(card(t, "Name:Wall\nTypes:Creature Wall\nPT:0/9\nOracle:x\n"), 1)
	wall.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{wall.ID})
	// A 2-mana-value spell cast first: Grizzly Bears (1G). The hand is
	// [salvo, bears] — cast by name.
	bears := e.G.Zone(state.ZHand, 0)[1]
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MG] = 3, 1
	castMode(t, e, bears, "")
	finishCast(t, e, bears)
	if e.G.Obj(bears).Zone != state.ZBattlefield {
		t.Fatalf("the Bears did not resolve: %s", e.G.Obj(bears).Zone)
	}
	salvo := e.G.Zone(state.ZHand, 0)[0]
	if got := castSalvoAt(t, e, salvo, wall.ID); got != 3 {
		t.Fatalf("Thunder Salvo dealt %d damage, want 3 (2 + the one other spell cast this turn)", got)
	}
}

func TestThunderSalvoAloneStillDealsTwo(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Thunder Salvo"))
	wall := e.G.AddObject(card(t, "Name:Wall\nTypes:Creature Wall\nPT:0/9\nOracle:x\n"), 1)
	wall.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{wall.ID})
	salvo := e.G.Zone(state.ZHand, 0)[0]
	if got := castSalvoAt(t, e, salvo, wall.ID); got != 2 {
		t.Fatalf("Thunder Salvo alone dealt %d damage, want 2 (the Plus.2 base, nothing else cast)", got)
	}
}

func TestCallForthTheTempestXIsTotalManaValueOfOtherSpellsCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Call Forth the Tempest"),
		corpusAlternativeCard(t, "Grizzly Bears"), corpusAlternativeCard(t, "Centaur Courser"))
	// Two opposing creatures to receive the DamageAll.
	a := e.G.AddObject(card(t, "Name:Servant\nTypes:Creature Goblin\nPT:1/9\nOracle:x\n"), 1)
	b := e.G.AddObject(card(t, "Name:Butler\nTypes:Creature Goblin\nPT:1/9\nOracle:x\n"), 1)
	a.Zone, b.Zone = state.ZBattlefield, state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{a.ID, b.ID})
	// A 2-mana-value and a 3-mana-value spell cast first — picked by name,
	// never by hand index (the casts reshuffle the hand's front).
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MR] = 20, 4, 3
	for _, name := range []string{"Grizzly Bears", "Centaur Courser"} {
		id := state.ObjID(0)
		for _, hid := range e.G.Zone(state.ZHand, 0) {
			if o := e.G.Obj(hid); o != nil && o.Face() != nil && o.Face().Name == name {
				id = hid
			}
		}
		if id == 0 {
			t.Fatalf("%s not in hand", name)
		}
		castMode(t, e, id, "")
		finishCast(t, e, id)
	}
	tempest := state.ObjID(0)
	for _, hid := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(hid); o != nil && o.Face() != nil && o.Face().Name == "Call Forth the Tempest" {
			tempest = hid
		}
	}
	if tempest == 0 {
		t.Fatal("Call Forth the Tempest not in hand")
	}
	castMode(t, e, tempest, "")
	finishCast(t, e, tempest)
	if got := int(a.Damage); got != 5 {
		t.Fatalf("Call Forth dealt %d damage to the first opposing creature, want 5 (mana values 2+3)", got)
	}
	if got := int(b.Damage); got != 5 {
		t.Fatalf("Call Forth dealt %d damage to the second opposing creature, want 5", got)
	}
}

func TestCallForthTheTempestAloneDealsZero(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Call Forth the Tempest"))
	a := e.G.AddObject(card(t, "Name:Servant\nTypes:Creature Goblin\nPT:1/9\nOracle:x\n"), 1)
	a.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{a.ID})
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MR] = 8, 3
	tempest := e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, tempest, "")
	finishCast(t, e, tempest)
	if got := int(a.Damage); got != 0 {
		t.Fatalf("Call Forth alone dealt %d damage, want 0 (no other spells cast this turn)", got)
	}
}
