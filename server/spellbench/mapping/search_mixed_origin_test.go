package mapping_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestHiddenSelectKeepsAVisibleOptionVisible pins the compound-origin search
// shape (Origin$ Library,Graveyard): the library option needs a look id, but
// an option the chooser already sees keeps its visible id. Asking LookRef for
// the visible option is refused, which used to forfeit the v2 seat.
func TestHiddenSelectKeepsAVisibleOptionVisible(t *testing.T) {
	g := untilPending(t, "Wildfire", 1, func(d *decision.Decision, _ *rules.Engine) bool {
		return d.Kind == decision.KChoose && len(d.Options) > 1 && d.Options[0].Kind == "search"
	})
	d := *g.E.Pending()
	var hidden state.ObjID
	for _, o := range d.Options {
		if obj := g.E.G.Obj(o.Obj); obj != nil && obj.Zone == state.ZLibrary && obj.Owner == d.Player {
			hidden = o.Obj
			break
		}
	}
	src := g.E.G.Obj(d.Source)
	if hidden == 0 || src == nil || !observe.Visible(d.Player, src) {
		t.Fatalf("precondition: hidden=%d source=%v visible=%v", hidden, src, src != nil && observe.Visible(d.Player, src))
	}
	d.Options = []decision.Option{{Index: 0, Kind: "search", Obj: hidden}, {Index: 1, Kind: "search", Obj: d.Source}}
	env := envFor(t, g)
	tx, err := mapping.Begin(env, &d)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	p, err := tx.Pose()
	if err != nil {
		t.Fatalf("Pose: %v", err)
	}
	if !p.Look {
		t.Fatal("Pose did not open a look for the hidden library option")
	}
	if len(p.Known) != 1 || p.Known[0].How != "searching" {
		t.Fatalf("Known = %+v, want exactly the hidden card searched", p.Known)
	}
	visible, err := env.Obs.Ref(d.Player, d.Source)
	if err != nil || visible == nil {
		t.Fatalf("Ref(source) = %v, %v", visible, err)
	}
	found := false
	for _, c := range p.Candidates {
		if c.Sem.Kind != "select_object" {
			continue
		}
		if choice, ok := c.Sem.Fields["choice"].(protocol.TargetRef); ok && choice.Object != nil && choice.Object.ObjectID == visible.ObjectID {
			found = true
			if c.Hidden || c.SortID != "" {
				t.Fatalf("visible option was posed as a hidden candidate: %+v", c)
			}
		}
	}
	if !found {
		t.Fatalf("no candidate names the visible option %s", visible.ObjectID)
	}
}
