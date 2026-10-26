package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// tapChoicesBoard stages Orphans of the Wheat (tapped, as an attacker is)
// beside two untapped vanilla creatures and returns the TrigTap resolution
// posed to seat 0. Seat 1's untapped creature must never be offered.
func tapChoicesBoard(t *testing.T) (e *Engine, cfg Config, orphans, a, b, foe state.ObjID, d func() *effects.Ctx) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	orph := selectionCard(t, reg, "Orphans of the Wheat")
	bear := card(t, "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, ids, cfg := selectionStage(t, reg, Stage{
		Turn: 1, Active: 0, Starting: 0, Step: state.StepMain2,
		Players: []StagedPlayer{{Life: 20}, {Life: 20}},
		Permanents: []StagedPermanent{
			{Card: orph, Owner: 0, Controller: 0, Tapped: true},
			{Card: bear, Owner: 0, Controller: 0},
			{Card: bear, Owner: 0, Controller: 0},
			{Card: bear, Owner: 1, Controller: 1, Sick: true},
		},
	})
	orphans, a, b, foe = ids.Permanents[0], ids.Permanents[1], ids.Permanents[2], ids.Permanents[3]
	if o := e.G.Obj(orphans); o == nil || !o.Tapped || e.Power(orphans) != 2 || e.Toughness(orphans) != 1 {
		t.Fatalf("precondition: Orphans not a tapped 2/1")
	}
	for _, id := range []state.ObjID{a, b, foe} {
		if e.G.Obj(id).Tapped {
			t.Fatalf("precondition: creature %d is tapped", id)
		}
	}
	d = func() *effects.Ctx {
		return &effects.Ctx{Source: orphans, Controller: 0, SVars: selectionSVars(e.G.Obj(orphans).Face())}
	}
	return
}

func TestTapCardChoicesOrphansPumpsPerTapped(t *testing.T) {
	e, cfg, orphans, a, b, foe, mk := tapChoicesBoard(t)
	sa := cards.ResolveSVar(e.G.Obj(orphans).Face().SVars, "TrigTap")
	d := kr0Run(t, e, sa, mk, nil)
	if d == nil || d.Player != 0 || d.Min != 0 || d.Max != 2 || len(d.Options) != 2 {
		t.Fatalf("decision = %+v, want seat 0's 0..2 pick over the two untapped creatures", d)
	}
	for _, o := range d.Options {
		if o.Kind != "tap" || (o.Obj != a && o.Obj != b) {
			t.Fatalf("option %+v: want a tap option on one of seat 0's untapped creatures", o)
		}
	}
	if e.G.Obj(a).Tapped || e.G.Obj(b).Tapped {
		t.Fatal("a creature was tapped before the answer")
	}
	kr0Answer(t, e, kr0Opt(t, d, a), kr0Opt(t, d, b))
	if !e.G.Obj(a).Tapped || !e.G.Obj(b).Tapped {
		t.Fatalf("answered creatures tapped = %v/%v, want both", e.G.Obj(a).Tapped, e.G.Obj(b).Tapped)
	}
	if e.G.Obj(foe).Tapped {
		t.Fatal("the opponent's creature was tapped")
	}
	if p, tough := e.Power(orphans), e.Toughness(orphans); p != 4 || tough != 3 {
		t.Fatalf("Orphans = %d/%d, want 4/3 (2/1 + 1/+1 per creature tapped)", p, tough)
	}
	replayCheck(t, e, cfg)
}

func TestTapCardChoicesAnyNumberZero(t *testing.T) {
	e, cfg, orphans, a, b, _, mk := tapChoicesBoard(t)
	sa := cards.ResolveSVar(e.G.Obj(orphans).Face().SVars, "TrigTap")
	d := kr0Run(t, e, sa, mk, nil)
	if d == nil || d.Min != 0 {
		t.Fatalf("decision = %+v, want a Min 0 pick", d)
	}
	kr0Answer(t, e)
	if e.G.Obj(a).Tapped || e.G.Obj(b).Tapped {
		t.Fatal("a creature was tapped on an empty answer")
	}
	if p, tough := e.Power(orphans), e.Toughness(orphans); p != 2 || tough != 1 {
		t.Fatalf("Orphans = %d/%d, want 2/1", p, tough)
	}
	replayCheck(t, e, cfg)
}

func TestTapCardChoicesTeferiMinusThree(t *testing.T) {
	e, cfg, orphans, _, _, _, mk := tapChoicesBoard(t)
	teferi := kr0Corpus(t, "Invasion of New Phyrexia")
	var sa *cards.SA
	for _, f := range teferi.Faces {
		for _, ab := range f.Abilities {
			if ab.Params["CardChoices"] != "" {
				sa = ab
			}
		}
	}
	if sa == nil {
		t.Fatal("precondition: no CardChoices Tap on Invasion of New Phyrexia")
	}
	d := kr0Run(t, e, sa, mk, nil)
	const want = "Tap any number of untapped creatures you control"
	if d == nil || d.Prompt != want {
		t.Fatalf("decision = %+v, want the ChoicePrompt$ %q", d, want)
	}
	_ = orphans
	_ = cfg
}

// TestTapDefinedUnchanged: a Defined$ Tap (no CardChoices$) still taps its
// referent with no ask, and RememberTapped$ replaces the Remembered set with
// what it tapped.
func TestTapDefinedUnchanged(t *testing.T) {
	e := kr0Engine(t, 2)
	src := kr0Src(t, e, 0, "Name:Fixture src\nTypes:Creature\nPT:1/1\nOracle:x\n", state.ZBattlefield)
	if e.G.Obj(src).Tapped {
		t.Fatal("precondition: source already tapped")
	}
	var last *effects.Ctx
	d := kr0Run(t, e, kr0SA(t, "DB$ Tap | Defined$ Self | RememberTapped$ True"),
		func() *effects.Ctx {
			return &effects.Ctx{Source: src, Controller: 0, Remembered: []state.Target{{Obj: 999}}}
		}, &last)
	if d != nil {
		t.Fatalf("a Defined$ Tap posed %+v", d)
	}
	if !e.G.Obj(src).Tapped {
		t.Fatal("Defined$ Self was not tapped")
	}
	if len(last.Remembered) != 1 || last.Remembered[0].Obj != src {
		t.Fatalf("Remembered = %v, want only the tapped source", last.Remembered)
	}
}
