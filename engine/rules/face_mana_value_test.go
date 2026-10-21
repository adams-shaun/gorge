package rules

// CR 202.3 object mana value (cr-g13-face-characteristics-mva). A mana-value
// read asks for the OBJECT's mana value, which differs from its current
// face's printed cost for a transformed back face (CR 712.8e/202.3b: the
// front face's cost), a melded permanent (CR 712.8g/202.3c: the sum of both
// fronts), a spell on the stack with {X} (CR 202.3e: the announced X) and a
// twobrid symbol (CR 202.3f: the larger component). Every assertion reads through the shared filter
// predicate (effects.MatchesSpec's cmcEQ<n>) or a real card flow, so the
// tests pin the readers the cards actually use.

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// wantCmc fails unless id matches "Card.cmcEQ<want>" and no neighbouring
// value, so a wrong mana value names itself in the message.
func wantCmc(t *testing.T, e *Engine, id state.ObjID, want int, what string) {
	t.Helper()
	got := -1
	for n := 0; n <= 16; n++ {
		if effects.MatchesSpec(e.G, "Card.cmcEQ"+strconv.Itoa(n), id, 0) {
			got = n
			break
		}
	}
	if got != want {
		t.Fatalf("%s: mana value = %d, want %d", what, got, want)
	}
}

// TestManaValueTransformedBackFaceIsFrontFace: CR 712.8e. Sephiroth, Fabled
// SOLDIER (repo deck, {2}{B}) transformed into Sephiroth, One-Winged Angel
// (ManaCost "no cost") still has mana value 3, so Fatal Push without revolt
// ("cmcLE2") does not destroy it.
func TestManaValueTransformedBackFaceIsFrontFace(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	sep := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Sephiroth, Fabled SOLDIER"))
	wantCmc(t, e, sep, 3, "front face up")
	e.emit(events.Event{Kind: events.FlipFace, Obj: sep, Amount: 1})
	if f := e.G.Obj(sep).Face(); f == nil || f.Name != "Sephiroth, One-Winged Angel" {
		t.Fatalf("precondition: transform left face %v", f)
	}
	wantCmc(t, e, sep, 3, "back face up (CR 712.8e)")
	if effects.MatchesSpec(e.G, "Creature.cmcLE2", sep, 0) {
		t.Fatal("transformed Sephiroth matched Fatal Push's cmcLE2 (read the back face's no-cost as 0)")
	}
}

// TestManaValueMeldedPermanentSumsFrontFaces: CR 712.8g. Gisela, the Broken
// Blade ({2}{W}{W}) melded with Bruna, the Fading Light ({5}{W}{W}) is
// Brisela, Voice of Nightmares with mana value 11.
func TestManaValueMeldedPermanentSumsFrontFaces(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const gisela, bruna, brisela = "Gisela, the Broken Blade", "Bruna, the Fading Light", "Brisela, Voice of Nightmares"
	e, _ := meldGame(t, reg, []string{gisela, bruna}, nil)
	g, b := meldPut(t, e, gisela, 0), meldPut(t, e, bruna, 0)
	res, ok := effects.Meld(e, 0, g, b, state.MeldOptions{ResultName: brisela})
	if !ok {
		t.Fatal("precondition: Gisela and Bruna did not meld")
	}
	wantCmc(t, e, res, 11, "melded Brisela (CR 712.8g)")
}

// TestManaValueXOnTheStackIsTheChosenValue: CR 202.3e. Fireball ({X}{R}) on
// the stack with X=5 has mana value 6 (Disdainful Stroke's cmcGE4 sees it);
// in the hand X is 0 and it is 1.
func TestManaValueXOnTheStackIsTheChosenValue(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	fb := handCard(e, mustCorpusCard(t, reg, "Fireball"), 0)
	wantCmc(t, e, fb, 1, "Fireball in hand (X is 0 off the stack)")
	o := e.G.Obj(fb)
	o.Zone = state.ZStack
	o.X = 5
	wantCmc(t, e, fb, 6, "Fireball on the stack with X=5 (CR 202.3e)")
}

// TestManaValueTwobridCountsTheGenericComponent: CR 202.3f. Beseech the
// Queen ({2/B}{2/B}{2/B}) has mana value 6 in every reader, including the
// cmc filter predicate.
func TestManaValueTwobridCountsTheGenericComponent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bq := handCard(e, mustCorpusCard(t, reg, "Beseech the Queen"), 0)
	wantCmc(t, e, bq, 6, "Beseech the Queen (CR 202.3f)")
}

// TestManaValueModalDFCBackFaceIsItsOwn guards CR 712.8f: a modal DFC's back
// face up on the battlefield has that face's own characteristics, so Sink
// into Stupor's land back face (repo deck, no cost) has mana value 0, not the
// front face's 3. Passes before and after the fix.
func TestManaValueModalDFCBackFaceIsItsOwn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	id := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Sink into Stupor // Soporific Springs"))
	e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1})
	wantCmc(t, e, id, 0, "MDFC back face (CR 712.8f)")
}
