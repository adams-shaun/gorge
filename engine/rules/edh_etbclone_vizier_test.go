package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEDHETBCloneRidersVizierEmbalmedToken: Vizier of Many Faces' ETB copy
// exceptions ("except if Vizier of Many Faces was embalmed, the token has no
// mana cost, it's white, and it's a Zombie in addition to its other types")
// apply only when the entering object is the card's own embalm token. The
// embalmed token's copy of a creature is white, a Zombie in addition, with no
// mana cost; a hard-cast Vizier's copy is a plain copy of the template, in
// the template's colours, types and mana cost.
func TestEDHETBCloneRidersVizierEmbalmedToken(t *testing.T) {
	t.Parallel()
	// The embalm route: Vizier in the graveyard, Embalm {3}{U}{U}, and the
	// minted token's ETB copy election copies the battlefield Grizzly Bears.
	e, cfg, _ := altCostEngine(t, 1411, []string{"Vizier of Many Faces", "Grizzly Bears"}, nil, nil)
	viz := findCardObj(t, e, 0, "Vizier of Many Faces", state.ZGraveyard)
	tpl := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	if f := e.G.Obj(tpl).Face(); f.ManaCost == "" || f.ManaCost != "1 G" {
		t.Fatalf("precondition: Grizzly Bears mana cost %q, want \"1 G\"", f.ManaCost)
	}
	addMana(t, e, 0, "CCUUU") // Embalm {3}{U}{U}
	submitChoices(t, e, embalmAbilityOption(t, e, viz, "Embalm"))
	// The minted token's entry poses its ETB copy election mid-resolution;
	// answer it with the battlefield Grizzly Bears, then drain the stack.
	answered := false
	for i := 0; i < 40 && len(e.G.Stack) > 0 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining (stack depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "etb" {
			idx := -1
			for _, o := range d.Options {
				if o.Obj == tpl {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("the battlefield Grizzly Bears was not offered: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			answered = true
			continue
		}
		pi := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pi = o.Index
			}
		}
		if pi < 0 {
			t.Fatalf("no pass option while draining: %+v", d)
		}
		submitChoices(t, e, pi)
	}
	if !answered {
		t.Fatal("the embalmed token's ETB copy election was never posed")
	}
	tok := embalmToken(t, e, "Grizzly Bears")
	if got := e.Colors(tok); got != "W" {
		t.Fatalf("the embalmed token's copy colours %q, want W (the SetColor$ exception)", got)
	}
	if dt := e.Derived(tok); !hasWord(dt.Types, "Zombie") || !hasWord(dt.Types, "Bear") {
		t.Fatalf("the embalmed token's copy types %v, want a Zombie in addition to the copied Bear", dt.Types)
	}
	if f := e.G.Obj(tok).Face(); f.ManaCost != "" || f.Cmc() != 0 {
		t.Fatalf("the embalmed token's copy mana cost %q (mana value %d), want none (CR 707.9b)", f.ManaCost, f.Cmc())
	}
	if f := e.G.Obj(tpl).Face(); f.ManaCost != "1 G" {
		t.Fatalf("the template's mana cost moved to %q", f.ManaCost)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "RemoveCost$") {
			t.Fatalf("the embalmed token's applied no-mana-cost exception was noted unread: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)

	// The cast route: a hard-cast Vizier enters as a plain copy of the
	// template -- the exceptions are the embalmed-token clause only.
	e2, id, tpl2 := edhCloneCast(t, "Vizier of Many Faces", "Grizzly Bears", 1,
		map[int]int32{state.MC: 2, state.MU: 2})
	o := e2.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("cast route: %+v, want a Grizzly Bears copy on the battlefield", o)
	}
	if got := e2.Colors(id); got != "G" {
		t.Fatalf("the hard-cast copy colours %q, want the template's G", got)
	}
	if dt := e2.Derived(id); hasWord(dt.Types, "Zombie") {
		t.Fatalf("the hard-cast copy types %v, want the template's without Zombie", dt.Types)
	}
	if f, tf := o.Face(), e2.G.Obj(tpl2).Face(); f.ManaCost != tf.ManaCost {
		t.Fatalf("the hard-cast copy mana cost %q, want the template's %q", f.ManaCost, tf.ManaCost)
	}
	for _, ev := range e2.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "RemoveCost$") {
			t.Fatalf("the hard-cast Vizier's declined-by-card-text rider was noted unread: %q", ev.Text)
		}
	}
}
