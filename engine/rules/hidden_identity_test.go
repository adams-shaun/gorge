package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// fillerTable is the three Tape Filler cards of the kernelFixture engine as a card
// table, sorted by name.
func fillerTable(t *testing.T, e *Engine) *cards.Universe {
	t.Helper()
	var cs []*cards.Card
	for _, name := range []string{"Tape Filler A", "Tape Filler B", "Tape Filler C"} {
		var c *cards.Card
		for i := range e.G.Objs {
			if o := &e.G.Objs[i]; o.Card != nil && o.Card.Faces[0].Name == name {
				c = o.Card
				break
			}
		}
		if c == nil {
			t.Fatalf("fixture has no %s", name)
		}
		cs = append(cs, c)
	}
	return cards.UniverseOf(cs)
}

// ReidentifyHidden is for hypothetical worlds only: a match engine is
// refused, a world's hidden card takes the new identity through one Secret
// HiddenIdentity event, and a second, different table is refused.
func TestReidentifyHiddenIsHypotheticalOnly(t *testing.T) {
	e, _ := kernelFixture(t, 2, 9902, tapeDrawVoteSrc,
		"Name:Tape Filler A\nManaCost:1\nTypes:Artifact\nOracle:x\n",
		"Name:Tape Filler B\nManaCost:2\nTypes:Artifact\nOracle:x\n",
		"Name:Tape Filler C\nManaCost:3\nTypes:Artifact\nOracle:x\n")
	u := fillerTable(t, e)
	top := e.G.Zone(state.ZLibrary, 1)[0]
	if err := ReidentifyHidden(e, u, []state.ObjID{top}, []int32{2}); err == nil {
		t.Fatal("a match engine was re-identified")
	}
	head := e.L.Head()
	w := e.CloneHypothetical(5)
	n := len(w.L.Events)
	if err := ReidentifyHidden(w, u, []state.ObjID{top}, []int32{2}); err != nil {
		t.Fatal(err)
	}
	if got := w.G.Obj(top).Card.Faces[0].Name; got != "Tape Filler C" {
		t.Fatalf("re-identified card is %q", got)
	}
	if ev := w.L.Events[n]; len(w.L.Events) != n+1 || ev.Kind != events.HiddenIdentity || !ev.Secret || ev.Obj != top || ev.Player != 1 {
		t.Fatalf("want one Secret HiddenIdentity event, got %+v", w.L.Events[n:])
	}
	if e.G.Obj(top).Card.Faces[0].Name == "Tape Filler C" || e.L.Head() != head {
		t.Fatal("re-identifying the world changed the real engine")
	}
	if err := ReidentifyHidden(w, fillerTable(t, e), []state.ObjID{top}, []int32{0}); err == nil {
		t.Fatal("a second card table was accepted")
	}
	if bf := e.G.Zone(state.ZBattlefield, 0); len(bf) > 0 {
		if err := ReidentifyHidden(w, u, []state.ObjID{bf[0]}, []int32{0}); err == nil {
			t.Fatal("a public card was re-identified")
		}
	}
}

// Spec §7.3 with a closed-list world: a world forked at a posed tape
// resolution whose library was RE-IDENTIFIED re-runs from S0 (the real
// cards) and re-applies the HiddenIdentity events, so the card it draws
// after the fork is the re-identified one -- the world's card table and pool
// gates survive the restore.
func TestReidentifiedWorldSurvivesATapeReRun(t *testing.T) {
	e, _ := kernelFixture(t, 2, 9901, tapeDrawVoteSrc,
		"Name:Tape Filler A\nManaCost:1\nTypes:Artifact\nOracle:x\n",
		"Name:Tape Filler B\nManaCost:2\nTypes:Artifact\nOracle:x\n",
		"Name:Tape Filler C\nManaCost:3\nTypes:Artifact\nOracle:x\n")
	moveByName(t, e, 0, "Tape Filler A", state.ZLibrary)
	moveByName(t, e, 0, "Tape Filler B", state.ZLibrary)
	moveByName(t, e, 0, "Tape Filler C", state.ZLibrary)
	u := fillerTable(t, e)
	addMana(t, e, 0, "B")
	submitChoices(t, e, castOptionFor(t, e, fixtureInHand(t, e, "Tape Draw Vote")).Index)
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d.Kind == decision.KChoose && d.Player == 1 {
			break
		}
		if d.Kind == decision.KPriority {
			submitChoices(t, e, tapePassIndex(d))
			continue
		}
		submitChoices(t, e, 0)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Player != 1 || !TapePosed(e) {
		t.Fatalf("precondition: seat 1's vote not posed on a tape resolution: %+v", d)
	}
	w := e.CloneHypothetical(0xfeed)
	lib := append([]state.ObjID(nil), w.G.Zone(state.ZLibrary, 0)...)
	top := lib[0]
	// Every library card becomes the filler the true top is not.
	ord := int32(2)
	if e.G.Obj(top).Card.Faces[0].Name == "Tape Filler C" {
		ord = 1
	}
	want := u.Name(int(ord))
	ords := make([]int32, len(lib))
	for i := range ords {
		ords[i] = ord
	}
	if err := ReidentifyHidden(w, u, lib, ords); err != nil {
		t.Fatal(err)
	}
	if err := w.SubmitHypothetical(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{1}}); err != nil {
		t.Fatalf("world submit: %v", err)
	}
	hand := w.G.Zone(state.ZHand, 0)
	if drawn := hand[len(hand)-1]; drawn != top || w.G.Obj(drawn).Card.Faces[0].Name != want {
		t.Fatalf("post-fork draw %d is %q, want the re-identified top %d (%q)", drawn, w.G.Obj(drawn).Card.Faces[0].Name, top, want)
	}
	if w.G.HypotheticalCards != u {
		t.Fatal("the re-run dropped the world's card table")
	}
	if e.G.HypotheticalCards != nil {
		t.Fatal("the real engine took the world's table")
	}
}

// A closed world's genesis manifest table holds only the searching seat's
// own list (ClosedDeckManifests), a real engine refuses the swap, and the
// table survives a posed-resolution re-run from S0 -- whose own table is the
// real one with every seat's list.
func TestClosedManifestsSurviveATapeReRun(t *testing.T) {
	e, _ := kernelFixture(t, 2, 9903, tapeDrawVoteSrc,
		"Name:Tape Filler A\nManaCost:1\nTypes:Artifact\nOracle:x\n",
		"Name:Tape Filler B\nManaCost:2\nTypes:Artifact\nOracle:x\n",
		"Name:Tape Filler C\nManaCost:3\nTypes:Artifact\nOracle:x\n")
	if len(e.OwnDeck(0).Main) == 0 || len(e.OwnDeck(1).Main) == 0 {
		t.Fatal("precondition: the fixture's seats have no genesis lists")
	}
	ms := e.ClosedDeckManifests(1)
	if len(ms) != 2 || len(ms[0].Main) != 0 || ms[0].Name != "" || len(ms[1].Main) != len(e.OwnDeck(1).Main) {
		t.Fatalf("closed table: %d seats, seat 0 %q/%d rows, seat 1 %d rows", len(ms), ms[0].Name, len(ms[0].Main), len(ms[1].Main))
	}
	if err := e.UseDeckManifests(ms); err == nil {
		t.Fatal("a match engine took a closed manifest table")
	}
	addMana(t, e, 0, "B")
	submitChoices(t, e, castOptionFor(t, e, fixtureInHand(t, e, "Tape Draw Vote")).Index)
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d.Kind == decision.KChoose && d.Player == 1 {
			break
		}
		if d.Kind == decision.KPriority {
			submitChoices(t, e, tapePassIndex(d))
			continue
		}
		submitChoices(t, e, 0)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Player != 1 || !TapePosed(e) {
		t.Fatalf("precondition: seat 1's vote not posed on a tape resolution: %+v", d)
	}
	w := e.CloneHypothetical(0xbeef)
	if err := w.UseDeckManifests(w.ClosedDeckManifests(1)); err != nil {
		t.Fatal(err)
	}
	if err := w.SubmitHypothetical(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{1}}); err != nil {
		t.Fatalf("world submit: %v", err)
	}
	if got := w.OwnDeck(0); len(got.Main) != 0 {
		t.Fatalf("the re-run restored seat 0's real list into the closed world (%d rows)", len(got.Main))
	}
	if len(w.OwnDeck(1).Main) == 0 {
		t.Fatal("the re-run dropped the searching seat's own list")
	}
	if len(e.OwnDeck(0).Main) == 0 {
		t.Fatal("closing the world changed the real engine's table")
	}
}
