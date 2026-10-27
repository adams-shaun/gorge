package rules

// cr-g222: Tam, the Possibility's activated ability is
// "{W}{U}{B}{R}{G}, {T}: Proliferate X times, where X is the number of
// planeswalker types among planeswalkers you control", with
// `SVar:X:Count$Valid Planeswalker.YouCtrl$PlaneswalkerType`. The property
// was unrecognised, so X read 0 and the ability only noted
// "Proliferate Amount$ X unresolvable (no X announced)".

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	tamFixtureAjani   = "Name:Ajani, Fixture Flame\nManaCost:1 W W\nTypes:Legendary Planeswalker Ajani\nLoyalty:3\nOracle:x\n"
	tamFixtureChandra = "Name:Chandra, Fixture Spark\nManaCost:1 R R\nTypes:Legendary Planeswalker Chandra\nLoyalty:3\nOracle:x\n"
)

// tamProliferateWalkerOpt returns the option index of walker in a pending
// proliferate ask.
func tamProliferateWalkerOpt(t *testing.T, d *decision.Decision, walker state.ObjID) int {
	t.Helper()
	for i, o := range d.Options {
		if o.Obj == walker {
			return i
		}
	}
	t.Fatalf("proliferate ask does not offer walker %d: %+v", walker, d.Options)
	return -1
}

// TestTamProliferatesOncePerPlaneswalkerType drives the real Tam with an
// Ajani and a Chandra on the battlefield: two distinct planeswalker types,
// so two proliferate actions, each taking both walkers from 3 loyalty to 5,
// and no "unresolvable" Note.
func TestTamProliferatesOncePerPlaneswalkerType(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	tamCard, ok := reg.Lookup("Tam, the Possibility")
	if !ok {
		t.Fatal("corpus missing Tam, the Possibility")
	}
	e, cfg := tokenReplGame(t, 5, tamCard,
		card(t, tamFixtureAjani), card(t, tamFixtureChandra))

	tam := putNamedOnBattlefield(t, e, "Tam, the Possibility")
	ajani := putNamedOnBattlefield(t, e, "Ajani, Fixture Flame")
	chandra := putNamedOnBattlefield(t, e, "Chandra, Fixture Spark")

	// PRECONDITION: all three permanents are on seat 0's battlefield, the
	// two walkers carry their printed 3 entry loyalty counters, and their
	// subtypes are distinct planeswalker types -- exactly where the count
	// looks and what it folds.
	for _, tc := range []struct {
		name string
		id   state.ObjID
		typ  string
	}{
		{"Tam, the Possibility", tam, ""},
		{"Ajani, Fixture Flame", ajani, "Ajani"},
		{"Chandra, Fixture Spark", chandra, "Chandra"},
	} {
		o := e.G.Obj(tc.id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("%s is not on the battlefield (obj %+v)", tc.name, o)
		}
		if tc.typ != "" && !slices.Contains(o.Face().Types, tc.typ) {
			t.Fatalf("%s types = %v, want subtype %q", tc.name, o.Face().Types, tc.typ)
		}
	}
	for _, walker := range []state.ObjID{ajani, chandra} {
		if got := e.G.Obj(walker).Counter("LOYALTY"); got != 3 {
			t.Fatalf("walker %d loyalty = %d before the activation, want 3", walker, got)
		}
	}
	// Tam's ability costs {T}, so drive a full round past its entry turn to
	// clear CR 302.6 summoning sickness before the activation is offered.
	driveToStep(t, e, 3, 0, state.StepMain1)

	addMana(t, e, 0, "WUBRG")
	opt, ok := findAbilityOption(e, tam, 0)
	if !ok {
		t.Fatalf("Tam's activated ability not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("decision = %+v, want the first proliferate ask (X should be 2)", d)
	}

	// First proliferation: both walkers.
	d = answerProliferate(t, e, d,
		tamProliferateWalkerOpt(t, d, ajani), tamProliferateWalkerOpt(t, d, chandra))
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("after the first answer the pending decision is %+v, want the SECOND proliferate ask", d)
	}
	for _, walker := range []state.ObjID{ajani, chandra} {
		if got := e.G.Obj(walker).Counter("LOYALTY"); got != 4 {
			t.Fatalf("walker %d loyalty = %d after the first proliferation, want 4", walker, got)
		}
	}

	// Second proliferation: both walkers again.
	answerProliferate(t, e, d,
		tamProliferateWalkerOpt(t, d, ajani), tamProliferateWalkerOpt(t, d, chandra))
	for _, walker := range []state.ObjID{ajani, chandra} {
		if got := e.G.Obj(walker).Counter("LOYALTY"); got != 5 {
			t.Fatalf("walker %d loyalty = %d after two proliferations, want 5", walker, got)
		}
	}
	if hasNote(e, "unresolvable") {
		t.Fatalf("the log carries an unresolvable Note: %+v", e.L.Events)
	}
	replayCheck(t, e, cfg)
}
