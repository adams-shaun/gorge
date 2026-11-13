package scriptfacts

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The granted Evolve trigger is typed out in this leaf (a granted keyword has
// no face to expand) while the printed one comes from cards.kwEvolve. Pin them
// together so an edit to one cannot leave the other (and dirac's lowering,
// which reads this synthesizer) behind.
func TestGrantedEvolveShapeMatchesPrintedExpansion(t *testing.T) {
	c, diags := cards.ParseBytes("evolve_shape.txt", []byte("Name:Evolver\nManaCost:1 G\nTypes:Creature Test\nPT:1/1\nK:Evolve\n"))
	if len(diags) != 0 || c == nil || len(c.Faces) != 1 {
		t.Fatalf("parse: card=%v diags=%v", c, diags)
	}
	c.Link() // the keyword expansion runs at Link
	var printed *cards.Trigger
	for i := range c.Faces[0].Triggers {
		if c.Faces[0].Triggers[i].ParamStr(cards.PKEvolve) != "" {
			printed = &c.Faces[0].Triggers[i]
		}
	}
	if printed == nil || printed.Effect == nil {
		t.Fatalf("precondition: K:Evolve expanded to no Evolve$ trigger: %+v", c.Faces[0].Triggers)
	}
	granted := GrantedKeywordTrigger("Evolve")
	if granted == nil || granted.Effect == nil {
		t.Fatal("GrantedKeywordTrigger(Evolve) = nil")
	}
	if printed.Mode != granted.Mode {
		t.Fatalf("Mode: printed %q, granted %q", printed.Mode, granted.Mode)
	}
	// The expansion's own bookkeeping (which SVar runs it, which keyword line
	// made it) is not shape: a granted keyword has no face to hang them on.
	shape := func(m map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range m {
			if k != "Execute" && k != "Keyword" && k != "KeywordLine" {
				out[k] = v
			}
		}
		return out
	}
	if p, g := shape(printed.Params), shape(granted.Params); !reflect.DeepEqual(p, g) {
		t.Errorf("trigger params: printed %v, granted %v", p, g)
	}
	if printed.Effect.API != granted.Effect.API || printed.Effect.Kind != granted.Effect.Kind ||
		!reflect.DeepEqual(printed.Effect.Params, granted.Effect.Params) {
		t.Errorf("body: printed %s/%s %v, granted %s/%s %v", printed.Effect.Kind, printed.Effect.API,
			printed.Effect.Params, granted.Effect.Kind, granted.Effect.API, granted.Effect.Params)
	}
}
