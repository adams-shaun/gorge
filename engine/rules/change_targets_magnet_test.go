package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	magnetTwoTargets = "Name:Pair\nTypes:Instant\nManaCost:R\nA:SP$ Pump | ValidTgts$ Creature | TargetMin$ 2 | TargetMax$ 2 | NumAtt$ +1\nOracle:x\n"
	magnetOneTarget  = "Name:Solo\nTypes:Instant\nManaCost:R\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +1\nOracle:x\n"
	magnetTwoLinks   = "Name:Chain\nTypes:Instant\nManaCost:R\nA:SP$ Pump | ValidTgts$ Creature | NumAtt$ +1 | SubAbility$ DBPump\nSVar:DBPump:DB$ Pump | ValidTgts$ Creature | NumAtt$ +1\nOracle:x\n"
)

// magnetBoard puts seat 1's spell on the stack aimed at nRoot of seat 0's bears and
// then nLink more on its SubAbility$ link, and resolves seat 0's Spellskite's
// ability at it (CR 115.7b: a target of the spell changes to Spellskite).
func magnetBoard(t *testing.T, script string, nRoot, nLink int) (e *Engine, spell *state.Object, skite state.ObjID, bears []state.ObjID) {
	t.Helper()
	e = stealEngine(t, 738)
	for i := 0; i < nRoot+nLink; i++ {
		bears = append(bears, onBoard(t, e, 0, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	}
	skite = onBoardCard(t, e, 0, choiceCorpusCard(t, "Spellskite"))
	spell = e.G.AddObject(card(t, script), 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: spell.ID, From: state.ZLibrary, To: state.ZStack})
	for i, id := range bears[:nRoot] {
		e.emit(events.Event{Kind: events.TargetsChosen, Obj: spell.ID, IDs: []state.ObjID{id}, Amount: int32(2 * min(i, 1))})
	}
	if nLink > 0 {
		var ts []state.Target
		for _, id := range bears[nRoot:] {
			ts = append(ts, state.Target{Obj: id})
		}
		e.recordCastSub(spell.ID, spell.Face().SpellAbility().Sub.Line, ts)
		e.recordSubTargets(spell.ID, ts)
	}
	ab := e.G.Obj(skite).Face().Abilities[0]
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: skite, Controller: 0, Targets: []state.Target{{Obj: spell.ID}},
			TargetsOffered: true, SVars: e.G.Obj(skite).Face().SVars}
	}, ab)
	return e, spell, skite, bears
}

func magnetIDs(ts []state.Target) []state.ObjID {
	var out []state.ObjID
	for _, t := range ts {
		out = append(out, t.Obj)
	}
	return out
}

// magnetAsk checks the pending decision is the one "which target" ask over want.
func magnetAsk(t *testing.T, e *Engine, want ...state.ObjID) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("Spellskite did not ask which target to change: %+v", d)
	}
	if len(d.Options) != len(want) {
		t.Fatalf("options = %+v, want the targets %v", d.Options, want)
	}
	for _, id := range want {
		if !offersObj(d, id) {
			t.Fatalf("options = %+v, want the targets %v", d.Options, want)
		}
	}
	return d
}

// CR 115.7b: the controller of "change a target of target spell to Spellskite"
// chooses which target changes; the engine used to move the first one unasked.
func TestChangeTargetMagnetAsksWhichTarget(t *testing.T) {
	t.Parallel()
	for _, pick := range []int{0, 1} {
		e, spell, skite, bears := magnetBoard(t, magnetTwoTargets, 2, 0)
		if got := magnetIDs(e.G.Obj(spell.ID).Targets); !slices.Equal(got, bears) {
			t.Fatalf("precondition: spell targets %v, want %v", got, bears)
		}
		d := magnetAsk(t, e, bears...)
		if offersObj(d, skite) {
			t.Fatalf("offered Spellskite as the target to change: %+v", d.Options)
		}
		submitChoices(t, e, objOptionIndex(t, d, bears[pick]))
		want := []state.ObjID{skite, bears[1]}
		if pick == 1 {
			want = []state.ObjID{bears[0], skite}
		}
		if got := magnetIDs(e.G.Obj(spell.ID).Targets); !slices.Equal(got, want) {
			t.Fatalf("picked %d: targets = %v, want %v", pick, got, want)
		}
	}
}

// With one place there is no choice: nothing is asked and the target moves.
func TestChangeTargetMagnetSinglePlaceNeverAsks(t *testing.T) {
	t.Parallel()
	e, spell, skite, bears := magnetBoard(t, magnetOneTarget, 1, 0)
	if skite == bears[0] || len(bears) != 1 {
		t.Fatalf("precondition: Spellskite %d must be a different object from the bear %v", skite, bears)
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("asked with one place: %+v", d)
	}
	if got := magnetIDs(e.G.Obj(spell.ID).Targets); !slices.Equal(got, []state.ObjID{skite}) {
		t.Fatalf("targets = %v, want [Spellskite %d]", got, skite)
	}
}

// Two declarations holding one target each: the offer spans both and the pick
// chooses the declaration.
func TestChangeTargetMagnetChoosesTheDeclaration(t *testing.T) {
	t.Parallel()
	for _, pick := range []int{0, 1} {
		e, spell, skite, bears := magnetBoard(t, magnetTwoLinks, 1, 1)
		sub := spell.Face().SpellAbility().Sub
		if got := magnetIDs(e.AnnouncedLinkTargets(spell.ID, sub)); !slices.Equal(got, bears[1:]) {
			t.Fatalf("precondition: link targets %v, want %v", got, bears[1:])
		}
		d := magnetAsk(t, e, bears...)
		submitChoices(t, e, objOptionIndex(t, d, bears[pick]))
		root, link := bears[0], bears[1]
		if pick == 0 {
			root = skite
		} else {
			link = skite
		}
		if got := magnetIDs(e.G.Obj(spell.ID).Targets); !slices.Equal(got, []state.ObjID{root}) {
			t.Fatalf("picked %d: root targets = %v, want [%d]", pick, got, root)
		}
		if got := magnetIDs(e.AnnouncedLinkTargets(spell.ID, sub)); !slices.Equal(got, []state.ObjID{link}) {
			t.Fatalf("picked %d: link targets = %v, want [%d]", pick, got, link)
		}
	}
}

// TestChangeTargetMagnet runs the magnet scenarios together.
func TestChangeTargetMagnet(t *testing.T) {
	t.Run("asks", TestChangeTargetMagnetAsksWhichTarget)
	t.Run("single", TestChangeTargetMagnetSinglePlaceNeverAsks)
	t.Run("links", TestChangeTargetMagnetChoosesTheDeclaration)
}
