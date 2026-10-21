// copiable_values_test.go pins CR 707.2/707.3/708.2a: every copy of a
// permanent copies its COPIABLE VALUES -- the face-down characteristics of a
// face-down permanent, the copied values of a permanent that is itself a
// copy -- never its hidden or printed card.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// copyDrain answers the pending decisions until the stack is empty: an ETB
// copy election takes the option naming template (template 0 = the first
// option), a KTarget ask takes the option naming target, and everything else
// goes through passUntilStackEmpty's one-step drain.
func copyDrain(t *testing.T, e *Engine, template, target state.ObjID) {
	t.Helper()
	for i := 0; i < 60 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending while draining")
		}
		pick := state.ObjID(0)
		switch {
		case d.Kind == decision.KChoose && d.ResumeKind == "etb":
			pick = template
		case d.Kind == decision.KTarget:
			pick = target
		default:
			if len(e.G.Stack) == 0 {
				return
			}
			passUntilStackEmpty(t, e, 1)
			continue
		}
		idx := -1
		for _, o := range d.Options {
			if o.Obj == pick || (pick == 0 && idx < 0) {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("no option naming %d in %+v", pick, d.Options)
		}
		submitChoices(t, e, idx)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack not drained: %d entries", len(e.G.Stack))
	}
}

// castCopyCard casts the named seat-0 hand card for mode "" and drains it,
// answering an ETB copy election with template and a target ask with target.
func castCopyCard(t *testing.T, e *Engine, id, template, target state.ObjID) {
	t.Helper()
	submitChoices(t, e, castModeOption(t, e, id, ""))
	copyDrain(t, e, template, target)
}

// seat0TokenNamed returns the single seat-0 battlefield token whose face
// name is name (0 when there is none).
func seat0TokenNamed(e *Engine, name string) state.ObjID {
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	return 0
}

// TestCloneOfAFaceDownCreatureCopiesTheFaceDownValues is the CR 707.2 example
// (and CR 708.2a): a Clone entering as a copy of a face-down Serra Angel is a
// face-UP colourless 2/2 creature with no name, no subtypes and no abilities
// -- the face-down permanent's copiable values -- never a 4/4 flying Angel.
func TestCloneOfAFaceDownCreatureCopiesTheFaceDownValues(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1401, []string{"Clone", "Serra Angel"}, nil, nil)
	angel := findCardObj(t, e, 0, "Serra Angel", state.ZBattlefield)
	e.emit(events.Event{Kind: events.TurnFaceDown, Obj: angel})
	e.pending = nil
	e.priorityRound()
	clone := findCardObj(t, e, 0, "Clone", state.ZHand)
	addMana(t, e, 0, "CCCU")
	castCopyCard(t, e, clone, angel, 0)

	o := e.G.Obj(clone)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Clone not on the battlefield: %+v", o)
	}
	if o.FaceDown {
		t.Fatal("the Clone copied face-down status; CR 707.2: it is face up")
	}
	if name := o.Face().Name; name != "" {
		t.Fatalf("Clone of a face-down creature is named %q, want no name (CR 707.2/708.2a)", name)
	}
	d := e.Derived(clone)
	if d.Power != 2 || d.Toughness != 2 {
		t.Fatalf("Clone of a face-down creature is %d/%d, want 2/2", d.Power, d.Toughness)
	}
	if got := e.Colors(clone); got != "" {
		t.Fatalf("Clone of a face-down creature has colours %q, want colourless", got)
	}
	if e.HasKeyword(clone, "Flying") {
		t.Fatal("Clone of a face-down Serra Angel has Flying: it copied the hidden face")
	}
	if len(d.Types) != 1 || d.Types[0] != "Creature" {
		t.Fatalf("Clone of a face-down creature types %v, want exactly [Creature]", d.Types)
	}
	replayCheck(t, e, cfg)
}

// TestTokenCopyOfAFaceDownCreatureIsANamelessTwoTwo is CR 707.2/708.2a on
// the token route: Cackling Counterpart's token copy of a face-down Serra
// Angel is a face-up colourless nameless 2/2 creature, and the token does
// not carry the hidden card (its card's only face is the face-down set), so
// no projection can read "Serra Angel" off it.
func TestTokenCopyOfAFaceDownCreatureIsANamelessTwoTwo(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1408, []string{"Serra Angel", "Cackling Counterpart"}, nil, nil)
	angel := findCardObj(t, e, 0, "Serra Angel", state.ZBattlefield)
	e.emit(events.Event{Kind: events.TurnFaceDown, Obj: angel})
	e.pending = nil
	e.priorityRound()
	counterpart := findCardObj(t, e, 0, "Cackling Counterpart", state.ZHand)
	addMana(t, e, 0, "CUU")
	castCopyCard(t, e, counterpart, 0, angel)

	tok := seat0TokenNamed(e, "")
	if tok == 0 {
		t.Fatal("no nameless token: the token copy of a face-down creature copied the hidden face (CR 707.2)")
	}
	o := e.G.Obj(tok)
	if o.FaceDown {
		t.Fatal("the token copied face-down status; CR 707.2: it is face up")
	}
	for _, f := range o.Card.Faces {
		if f != nil && f.Name != "" {
			t.Fatalf("the token carries the hidden card %q", f.Name)
		}
	}
	if d := e.Derived(tok); d.Power != 2 || d.Toughness != 2 || e.HasKeyword(tok, "Flying") {
		t.Fatalf("token copy is %d/%d flying=%v, want a vanilla 2/2", d.Power, d.Toughness, e.HasKeyword(tok, "Flying"))
	}
	replayCheck(t, e, cfg)
}

// TestTokenCopyOfACloneCopiesWhatTheCloneCopies is CR 707.3: Cackling
// Counterpart targeting a Clone that entered as a copy of Grizzly Bears makes
// a Grizzly Bears token (the Clone's copiable values), never a 0/0 "Clone"
// token that dies to state-based actions.
func TestTokenCopyOfACloneCopiesWhatTheCloneCopies(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1402, []string{"Clone", "Grizzly Bears", "Cackling Counterpart"}, nil, nil)
	bears := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	clone := findCardObj(t, e, 0, "Clone", state.ZHand)
	counterpart := findCardObj(t, e, 0, "Cackling Counterpart", state.ZHand)
	addMana(t, e, 0, "CCCU")
	castCopyCard(t, e, clone, bears, 0)
	if o := e.G.Obj(clone); o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("setup: Clone is not a Grizzly Bears copy: zone %s name %q", o.Zone, o.Face().Name)
	}
	addMana(t, e, 0, "CUU")
	castCopyCard(t, e, counterpart, 0, clone)

	if seat0TokenNamed(e, "Clone") != 0 {
		t.Fatal("the token copied the Clone's printed card instead of its copiable values (CR 707.3)")
	}
	tok := seat0TokenNamed(e, "Grizzly Bears")
	if tok == 0 {
		t.Fatal("no Grizzly Bears token: a token copy of a Clone copying Grizzly Bears must be a Grizzly Bears (CR 707.3)")
	}
	if d := e.Derived(tok); d.Power != 2 || d.Toughness != 2 {
		t.Fatalf("token copy P/T %d/%d, want 2/2", d.Power, d.Toughness)
	}
	replayCheck(t, e, cfg)
}

// TestTokenCopyOfAPrintedCreatureKeepsThePrintedFace is the guard: a token
// copy of an ordinary face-up creature that copies nothing still reads its
// printed face directly (no copy basis is installed), so the common copy path
// stays byte-identical.
func TestTokenCopyOfAPrintedCreatureKeepsThePrintedFace(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1403, []string{"Grizzly Bears", "Cackling Counterpart"}, nil, nil)
	bears := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	counterpart := findCardObj(t, e, 0, "Cackling Counterpart", state.ZHand)
	addMana(t, e, 0, "CUU")
	castCopyCard(t, e, counterpart, 0, bears)
	tok := seat0TokenNamed(e, "Grizzly Bears")
	if tok == 0 {
		t.Fatal("no Grizzly Bears token")
	}
	if o := e.G.Obj(tok); o.CopyFace != nil {
		t.Fatal("a token copy of a printed creature installed a copy basis; it must read its printed face")
	}
	replayCheck(t, e, cfg)
}
