package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// cr-g178: census-only reclassification. ShowChoice$ (GenericChoice) and
// NoPeek$ (PeekAndReveal) are descriptive (Forge's notifyOfValue / the
// skip-the-private-look flag; gorge's ModeChosen answer and its reveal path
// are already public), and an UnlessCost$ whose whole value names a face
// SVar (cost:Y) is folded at resolution by effects.UnlessCostResolved, not
// unmodelled. Test-only: the two ignoredAbilityParams entries, the
// unlessNamesFaceSVar exemption in cardCensusLabels, and this file.

// TestParamCensusUnlessSVarIsNotACostGap pins the unlessNamesFaceSVar
// exemption in cardCensusLabels: an UnlessCost$ whose whole trimmed value is
// a key of the face's SVar table labels no cost: token (the fold lives in
// effects.UnlessCostResolved, and an SVar body the evaluator cannot read is
// already a count:<head> gap through cards.Registry.Unsupported), while a
// Cost$ with the same value and an UnlessCost$ naming a non-SVar still
// label. A compound value (Sac<1/X>) must not be exempted either.
func TestParamCensusUnlessSVarIsNotACostGap(t *testing.T) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)
	if d == nil {
		t.Fatal("no derived read sets")
	}
	if got := ParseCost("Y").Unknown; len(got) == 0 {
		t.Fatal("fixture precondition: ParseCost(\"Y\") models Y -- re-point the fixture at an unmodelled token")
	}
	c := &cards.Card{Faces: []*cards.Face{{
		Name:  "unless-svar-fixture",
		SVars: map[string]string{"Y": "Count$Valid Creature.YouCtrl"},
		Abilities: []*cards.SA{
			{Kind: "SP", API: "Counter", Params: map[string]string{"UnlessCost": "Y"}},
		},
	}}}
	labels := cardCensusLabels(c, d, nil)
	if slices.Contains(labels, "cost:Y") {
		t.Errorf("UnlessCost$ naming face SVar Y labelled %v -- the exemption is gone", labels)
	}

	// Cost$ is never exempt, even with the same value.
	cCost := &cards.Card{Faces: []*cards.Face{{
		Name:  "cost-fixture",
		SVars: map[string]string{"Y": "Count$Valid Creature.YouCtrl"},
		Abilities: []*cards.SA{
			{Kind: "SP", API: "Counter", Params: map[string]string{"Cost": "Y"}},
		},
	}}}
	if labels := cardCensusLabels(cCost, d, nil); !slices.Contains(labels, "cost:Y") {
		t.Errorf("Cost$ Y = %v, want cost:Y (Cost$ is never exempt)", labels)
	}

	// UnlessCost$ naming a name that is NOT an SVar still labels.
	cQ := &cards.Card{Faces: []*cards.Face{{
		Name:  "unless-unknown-fixture",
		SVars: map[string]string{"Y": "Count$Valid Creature.YouCtrl"},
		Abilities: []*cards.SA{
			{Kind: "SP", API: "Counter", Params: map[string]string{"UnlessCost": "Z"}},
		},
	}}}
	if labels := cardCensusLabels(cQ, d, nil); !slices.Contains(labels, "cost:Z") {
		t.Errorf("UnlessCost$ Z (no SVar Z) = %v, want cost:Z", labels)
	}

	// A token inside a compound value is not exempt; only the whole value.
	// Foo<1/Y> names Y but not as the whole value, so its unmodelled head
	// still labels (ParseCost reports the head Foo).
	cCompound := &cards.Card{Faces: []*cards.Face{{
		Name:  "unless-compound-fixture",
		SVars: map[string]string{"Y": "Count$Valid Creature.YouCtrl"},
		Abilities: []*cards.SA{
			{Kind: "SP", API: "Counter", Params: map[string]string{"UnlessCost": "Foo<1/Y>"}},
		},
	}}}
	if labels := cardCensusLabels(cCompound, d, nil); !slices.Contains(labels, "cost:Foo") {
		t.Errorf("compound UnlessCost$ Foo<1/Y> = %v, want cost:Foo (only a whole-value SVar name is exempt)", labels)
	}
}

// TestParamCensusShowChoiceNoPeekDescriptive pins the two
// ignoredAbilityParams entries and their per-API scope.
func TestParamCensusShowChoiceNoPeekDescriptive(t *testing.T) {
	t.Parallel()
	if !ignoredParam("api:GenericChoice", "ShowChoice") {
		t.Error(`ignoredParam("api:GenericChoice", "ShowChoice") = false, want true`)
	}
	if !ignoredParam("api:PeekAndReveal", "NoPeek") {
		t.Error(`ignoredParam("api:PeekAndReveal", "NoPeek") = false, want true`)
	}
	if ignoredParam("api:Charm", "ShowChoice") {
		t.Error(`ignoredParam("api:Charm", "ShowChoice") = true, want false (the scope is per API)`)
	}
}

// TestShowChoiceSiegeChoiceIsPublic pins the ShowChoice$ verdict
// rules-side: a real Windcrag Siege enters, seat 0 answers Jeskai, and the
// answer is a public ModeChosen from seat 0 that every seat's Describe
// renders as "chooses Jeskai".
func TestShowChoiceSiegeChoiceIsPublic(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Windcrag Siege")
	// Precondition: the Siege's entry choice is genuinely posed, with Jeskai
	// offered -- the ShowChoice$ ask is this test's subject.
	id, d := castSearchSpell(t, e, "Windcrag Siege")
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("Windcrag Siege entry choice = %+v, want modes", d)
	}
	if len(d.Options) != 2 {
		t.Fatalf("Windcrag Siege mode options = %+v, want two (Mardu, Jeskai)", d.Options)
	}
	chosen := -1
	for _, o := range d.Options {
		if o.Label == "Jeskai" {
			chosen = o.Index
		}
	}
	if chosen < 0 {
		t.Fatalf("Jeskai not offered in %+v", d.Options)
	}
	submitChoices(t, e, chosen)
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || !slices.Contains(o.ChosenModes, "Jeskai") {
		t.Fatalf("Windcrag Siege entry = %+v, want battlefield with mode Jeskai", o)
	}
	found := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen && ev.Player == 0 && ev.Text == "Jeskai" {
			found = true
			if line := view.Describe(e.G, ev); !strings.Contains(line, "chooses Jeskai") {
				t.Errorf("ModeChosen renders %q, want \"chooses Jeskai\"", line)
			}
		}
	}
	if !found {
		t.Fatal("no public ModeChosen from seat 0 with Text Jeskai -- the answer was not recorded")
	}
	replayCheck(t, e, cfg)
}
