package rules

// Infernal Tutor's ask shape and answer: "Reveal a card from your hand" is a CHOICE posed to the
// revealing player (never the front of the hand), the chosen card is what
// the public reveal names and what the chained same-name search reads, and
// an Optional$ hand reveal poses its pick only after the "yes" -- with no
// re-posed optional ask once the pick is answered.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// tutorBoard seats the real Infernal Tutor in seat 0's hand beside two
// inline reveal candidates (Lightning Bolt first, then Forest) and puts one
// card of each name on top of the library.
func tutorBoard(t *testing.T) (e *Engine, tutor state.ObjID, hand, lib map[string]state.ObjID) {
	t.Helper()
	e = kernelEngine(t, 2)
	tutor = putCardIn(t, e, 0, corpusFixtureCard(t, "Infernal Tutor"), state.ZHand, false)
	hand, lib = map[string]state.ObjID{}, map[string]state.ObjID{}
	for _, n := range []string{"Lightning Bolt", "Forest"} {
		hand[n] = putCardIn(t, e, 0, cardOfSrc(t, "Name:"+n+"\nTypes:Sorcery\nOracle:x\n"), state.ZHand, false)
	}
	for _, n := range []string{"Forest", "Lightning Bolt"} {
		lib[n] = putCardIn(t, e, 0, cardOfSrc(t, "Name:"+n+"\nTypes:Sorcery\nOracle:x\n"), state.ZLibrary, true)
	}
	return e, tutor, hand, lib
}

func TestInfernalTutorRevealPickIsAskedAndDrivesTheSearch(t *testing.T) {
	t.Parallel()
	e, tutor, hand, lib := tutorBoard(t)
	from := len(e.L.Events)
	d := requireResumeAsk(t, castWithMana(t, e, 0, tutor), "reveal_pick")
	if d.Player != 0 || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("reveal pick = %+v, want a 1-of KChoose for the revealing seat 0", d)
	}
	if len(d.Options) != 2 || d.Options[0].Obj != hand["Lightning Bolt"] || d.Options[1].Obj != hand["Forest"] {
		t.Fatalf("reveal pick options = %+v, want the two hand cards in zone order", d.Options)
	}
	if n := publicRevealsFrom(e, from); len(n) != 0 {
		t.Fatalf("a reveal Note landed before the pick was answered: %+v", n)
	}

	// Reveal the SECOND hand card: the reveal and the search both read it.
	d = requireResumeAsk(t, answerDecision(t, e, d, objOptionIndex(t, d, hand["Forest"])), "search")
	notes := publicRevealsFrom(e, from)
	if len(notes) != 1 || !slices.Equal(notes[0].IDs, []state.ObjID{hand["Forest"]}) {
		t.Fatalf("reveal Notes = %+v, want exactly one naming the chosen Forest %d", notes, hand["Forest"])
	}
	if len(d.Options) != 1 || d.Options[0].Obj != lib["Forest"] {
		t.Fatalf("search offered %+v, want only the chosen name's library card %d", d.Options, lib["Forest"])
	}
	if d = answerDecision(t, e, d, d.Options[0].Index); d != nil {
		t.Fatalf("unexpected ask after the search: %+v", d)
	}
	if z := e.G.Obj(lib["Forest"]).Zone; z != state.ZHand {
		t.Fatalf("library Forest is on %s, want hand", z)
	}
	if z := e.G.Obj(lib["Lightning Bolt"]).Zone; z != state.ZLibrary {
		t.Fatalf("library Lightning Bolt is on %s, want library (the unchosen name)", z)
	}
}

func TestOptionalHandRevealPosesThePickAfterYes(t *testing.T) {
	t.Parallel()
	e := kernelEngine(t, 2)
	var hand []state.ObjID
	for _, n := range []string{"Alpha", "Beta", "Gamma"} {
		hand = append(hand, putCardIn(t, e, 0, cardOfSrc(t, "Name:"+n+"\nManaCost:9\nTypes:Creature\nPT:1/1\nOracle:x\n"), state.ZHand, false))
	}
	spell := putCardIn(t, e, 0, sorceryCard(t, "May Reveal", "A:SP$ Reveal | Defined$ You | Optional$ True"), state.ZHand, false)
	from := len(e.L.Events)
	d := requireResumeAsk(t, castWithMana(t, e, 0, spell), "reveal_optional")
	d = requireResumeAsk(t, answerDecision(t, e, d, optionKindIndex(t, d, "yes")), "reveal_pick")
	if d.Min != 1 || d.Max != 1 || len(d.Options) != 3 {
		t.Fatalf("after yes = %+v, want a 1-of-3 reveal_pick", d)
	}
	if d = answerDecision(t, e, d, objOptionIndex(t, d, hand[1])); d != nil {
		t.Fatalf("the pick's answer re-posed an ask (soft-lock): %+v", d)
	}
	notes := publicRevealsFrom(e, from)
	if len(notes) != 1 || !slices.Equal(notes[0].IDs, []state.ObjID{hand[1]}) {
		t.Fatalf("reveal Notes = %+v, want exactly one naming the chosen Beta", notes)
	}
}
