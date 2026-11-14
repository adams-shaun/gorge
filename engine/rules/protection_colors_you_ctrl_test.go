package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestProtectionFromColorsYouControl pins Forge's "Protection from
// colorsYouCtrl" (Empty-Shrine Kannushi): protection from the colors of the
// permanents its CONTROLLER controls, read live. Each direction asserts the
// value it depends on (the red source really is red, the controller really
// does or does not control a red permanent), and the control cases differ
// from the withheld ones.
func TestProtectionFromColorsYouControl(t *testing.T) {
	t.Parallel()
	const (
		kannushiSrc = "Name:Test Kannushi\nManaCost:W\nTypes:Creature Human Cleric\nPT:1/1\n" +
			"S:Mode$ Continuous | AffectedDefined$ Self | AddKeyword$ Protection from colorsYouCtrl | Description$ x\nOracle:x\n"
		redSrc   = "Name:Opp Red Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		greenSrc = "Name:Opp Green Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		// A colourless artifact that turns itself red (layer 5), then the same
		// shape left colourless: the controller's colour change is read live.
		painterSrc = "Name:Test Painter\nManaCost:2\nTypes:Artifact\n" +
			"S:Mode$ Continuous | Affected$ Card.Self | SetColor$ Red | Description$ x\nOracle:x\n"
		redOwnSrc = "Name:Own Red Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	)
	e, _, _ := newFixtureDeck(t, 74, kannushiSrc)
	k := putCreature(t, e, 0, kannushiSrc)
	oppRed := putToken(t, e, 1, redSrc, state.ZBattlefield)
	oppGreen := putToken(t, e, 1, greenSrc, state.ZBattlefield)

	// Preconditions: the keyword is derived verbatim, the sources have the
	// colours the test names, and the Kannushi (white) is all seat 0 controls.
	found := false
	for _, kw := range e.Keywords(k) {
		found = found || kw == "Protection from colorsYouCtrl"
	}
	if !found {
		t.Fatalf("precondition: Kannushi keywords = %v, want Protection from colorsYouCtrl", e.Keywords(k))
	}
	if got := e.Colors(oppRed); got != "R" {
		t.Fatalf("precondition: red source colours = %q, want R", got)
	}
	if got := e.Colors(oppGreen); got != "G" {
		t.Fatalf("precondition: green source colours = %q, want G", got)
	}
	if got := e.Colors(k); got != "W" {
		t.Fatalf("precondition: Kannushi colours = %q, want W", got)
	}

	// Controlling only itself (white): red and green sources are not withheld.
	if e.protectedFrom(k, oppRed) || e.protectedFrom(k, oppGreen) {
		t.Fatal("protectedFrom(Kannushi, red/green source) = true while its controller controls only a white permanent")
	}

	// A red permanent of the controller's: red is withheld, green is not.
	ownRed := putToken(t, e, 0, redOwnSrc, state.ZBattlefield)
	if got := e.Colors(ownRed); got != "R" {
		t.Fatalf("precondition: own red permanent colours = %q, want R", got)
	}
	if !e.protectedFrom(k, oppRed) {
		t.Fatal("protectedFrom(Kannushi, red source) = false with a red permanent controlled")
	}
	if e.protectedFrom(k, oppGreen) {
		t.Fatal("protectedFrom(Kannushi, green source) = true; green is not among the controlled colours")
	}

	// The red permanent leaves: protection from red goes with it.
	e.G.Obj(ownRed).Zone = state.ZGraveyard
	e.G.SetZone(state.ZBattlefield, 0, removeObj(e.G.Zone(state.ZBattlefield, 0), ownRed))
	if e.protectedFrom(k, oppRed) {
		t.Fatal("protectedFrom(Kannushi, red source) = true after the controlled red permanent left")
	}

	// A colourless artifact does not protect from red; once its layer-5
	// colour is red it does (the colour is read at check time).
	plainArtifact := "Name:Test Plain\nManaCost:2\nTypes:Artifact\nOracle:x\n"
	plain := putToken(t, e, 0, plainArtifact, state.ZBattlefield)
	if got := e.Colors(plain); got != "" {
		t.Fatalf("precondition: plain artifact colours = %q, want colourless", got)
	}
	if e.protectedFrom(k, oppRed) {
		t.Fatal("protectedFrom(Kannushi, red source) = true with only a colourless artifact beside it")
	}
	painter := putToken(t, e, 0, painterSrc, state.ZBattlefield)
	if got := e.Colors(painter); got != "R" {
		t.Fatalf("precondition: painter colours = %q, want R", got)
	}
	if !e.protectedFrom(k, oppRed) {
		t.Fatal("protectedFrom(Kannushi, red source) = false after a controlled permanent became red")
	}

	// A phased-out permanent is treated as though it doesn't exist (CR
	// 702.26b): an intrinsically red permanent, phased out, protects from
	// nothing (the painter is phased out too so its layer-5 red cannot stand
	// in for it), and does again once it phases back in.
	phasedRed := putToken(t, e, 0, redOwnSrc, state.ZBattlefield)
	if got := e.Colors(phasedRed); got != "R" {
		t.Fatalf("precondition: phased red permanent colours = %q, want R", got)
	}
	e.G.Obj(painter).PhasedOut = true
	e.G.Obj(phasedRed).PhasedOut = true
	if e.protectedFrom(k, oppRed) {
		t.Fatal("protectedFrom(Kannushi, red source) = true with every red controlled permanent phased out")
	}
	e.G.Obj(phasedRed).PhasedOut = false
	if !e.protectedFrom(k, oppRed) {
		t.Fatal("protectedFrom(Kannushi, red source) = false once a red permanent phased back in")
	}

	// The opponent's own red permanents are not "permanents you control":
	// the check is relative to the Kannushi's controller, not the source's.
	if e.controllerOf(oppRed) != 1 {
		t.Fatal("precondition: red source is seat 1's")
	}
}

func removeObj(ids []state.ObjID, id state.ObjID) []state.ObjID {
	out := make([]state.ObjID, 0, len(ids))
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}
