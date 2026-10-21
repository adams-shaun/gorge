package rules

// Animate/AnimateAll RemoveLandTypes$ (cr-g48). CR 305.7: "If an effect
// sets a land's subtype to one or more of the basic land types, the land no
// longer has its old land type." Graceful Antelope's "target land become a
// Plains" and Nightcreep's "all lands become Swamps" are written
// `Types$ <basic> | RemoveLandTypes$ True`; the strip must reach the
// layer-4 effect exactly as the printed-static family's (Blood Moon) does.

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCRAnimateRemoveLandTypesSetsBasicType: Graceful Antelope's TrigAnimate
// on seat 1's Volcanic Island leaves it a Plains only.
func TestCRAnimateRemoveLandTypesSetsBasicType(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Graceful Antelope")},
		[]*cards.Card{lookup(t, reg, "Volcanic Island")})
	ant := moveByName(t, e, 0, "Graceful Antelope", state.ZBattlefield)
	isl := moveByName(t, e, 1, "Volcanic Island", state.ZBattlefield)
	if ant == 0 || isl == 0 {
		t.Fatal("fixture cards not placed")
	}
	sa := resolveSourceFaceSA(t, e, ant, "TrigAnimate")
	effects.Resolve(e, &effects.Ctx{Source: ant, Controller: 0, SVars: e.G.Obj(ant).Face().SVars,
		Targets: []state.Target{{Obj: isl}}, TargetsOffered: true}, sa)
	got := e.Derived(isl).Types
	if !slices.Contains(got, "Plains") {
		t.Fatalf("Volcanic Island types = %v, want Plains added", got)
	}
	for _, old := range []string{"Island", "Mountain"} {
		if slices.Contains(got, old) {
			t.Fatalf("CR 305.7: Volcanic Island types = %v after \"becomes a Plains\", want %s gone (RemoveLandTypes$ True)", got, old)
		}
	}
	if !slices.Contains(got, "Land") {
		t.Fatalf("Volcanic Island types = %v, want the Land card type to survive", got)
	}
}

// TestCRAnimateAllRemoveLandTypesSetsBasicType: Nightcreep's AnimateSwamp
// sweep makes Volcanic Island a Swamp only, with no "not implemented" note.
func TestCRAnimateAllRemoveLandTypesSetsBasicType(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Nightcreep")},
		[]*cards.Card{lookup(t, reg, "Volcanic Island")})
	nc := moveByName(t, e, 0, "Nightcreep", state.ZHand)
	isl := moveByName(t, e, 1, "Volcanic Island", state.ZBattlefield)
	if nc == 0 || isl == 0 {
		t.Fatal("fixture cards not placed")
	}
	before := len(e.L.Events)
	cgResolveSVar(t, e, nc, "AnimateSwamp")
	got := e.Derived(isl).Types
	if !slices.Contains(got, "Swamp") || slices.Contains(got, "Island") || slices.Contains(got, "Mountain") {
		t.Fatalf("CR 305.7: Volcanic Island types = %v after Nightcreep, want Swamp only", got)
	}
	for _, ev := range e.L.Events[before:] {
		if ev.Kind == events.Note && ev.Text == "AnimateAll RemoveLandTypes$ not implemented; ignored" {
			t.Fatalf("AnimateAll still notes RemoveLandTypes$ as unread: %q", ev.Text)
		}
	}
}

// TestCRAnimateChosenTypeResolvesStrips: the 14 corpus carriers that write
// `Types$ ChosenType | RemoveLandTypes$ True` (Elsewhere Flask's
// AB$ ChooseType | SubAbility$ DBAnimate chain, Terraformer, Reef Shaman,
// Dream Thrush, ...) must resolve the element against the SOURCE's recorded
// type choice -- choose.go records it on the resolving source, never on the
// animated land -- and then strip + set exactly that basic type. A second
// land outside the sweep (AnimateAll's ValidCards$ Land.YouCtrl) stays
// untouched.
func TestCRAnimateChosenTypeResolvesStrips(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Elsewhere Flask"), lookup(t, reg, "Volcanic Island")},
		[]*cards.Card{lookup(t, reg, "Volcanic Island")})
	flask := moveByName(t, e, 0, "Elsewhere Flask", state.ZBattlefield)
	isl := moveByName(t, e, 0, "Volcanic Island", state.ZBattlefield)
	opp := moveByName(t, e, 1, "Volcanic Island", state.ZBattlefield)
	if flask == 0 || isl == 0 || opp == 0 {
		t.Fatal("fixture cards not placed")
	}
	if before := e.Derived(isl).Types; slices.Contains(before, "Plains") || !slices.Contains(before, "Island") {
		t.Fatalf("precondition: Volcanic Island types = %v, want the printed Island present, no Plains", before)
	}
	// A real AB$ ChooseType (Type$ Basic Land) records the answer on the
	// source object before SubAbility$ DBAnimate resolves (choose.go).
	e.G.Obj(flask).ChosenType = "Plains"
	cgResolveSVar(t, e, flask, "DBAnimate")
	got := e.Derived(isl).Types
	if !slices.Contains(got, "Plains") || slices.Contains(got, "Island") || slices.Contains(got, "Mountain") {
		t.Fatalf("CR 305.7: Volcanic Island types = %v after Elsewhere Flask (Plains chosen), want Plains only", got)
	}
	if !slices.Contains(got, "Land") {
		t.Fatalf("Volcanic Island types = %v, want the Land card type to survive", got)
	}
	if oppGot := e.Derived(opp).Types; slices.Contains(oppGot, "Plains") || !slices.Contains(oppGot, "Island") {
		t.Fatalf("opponent's Volcanic Island types = %v, want untouched (Land.YouCtrl)", oppGot)
	}
}

// TestCRAnimateChosenTypeWithoutChoiceFailsClosed: with no recorded choice
// the ChosenType element is dropped and the RemoveLandTypes$ strip is NOT
// applied (a strip with nothing added would leave the land unable to tap for
// mana), the land is untouched, no literal "ChosenType" word leaks onto it,
// and the drop is loud.
func TestCRAnimateChosenTypeWithoutChoiceFailsClosed(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := landTypesEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Elsewhere Flask"), lookup(t, reg, "Volcanic Island")},
		nil)
	flask := moveByName(t, e, 0, "Elsewhere Flask", state.ZBattlefield)
	isl := moveByName(t, e, 0, "Volcanic Island", state.ZBattlefield)
	if flask == 0 || isl == 0 {
		t.Fatal("fixture cards not placed")
	}
	before := len(e.L.Events)
	cgResolveSVar(t, e, flask, "DBAnimate")
	got := e.Derived(isl).Types
	for _, want := range []string{"Land", "Island", "Mountain"} {
		if !slices.Contains(got, want) {
			t.Fatalf("CR 305.7 fail-closed: Volcanic Island types = %v, want %s to survive untouched", got, want)
		}
	}
	for _, banned := range []string{"Plains", "Swamp", "ChosenType"} {
		if slices.Contains(got, banned) {
			t.Fatalf("CR 305.7 fail-closed: Volcanic Island types = %v, want no %q (no strip, no literal)", got, banned)
		}
	}
	noted := false
	for _, ev := range e.L.Events[before:] {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "ChosenType names a card with no recorded choice") {
			noted = true
		}
	}
	if !noted {
		t.Fatalf("the dropped ChosenType element is not loud: no Note among events %v", e.L.Events[before:])
	}
}
