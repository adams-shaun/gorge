package scriptfacts

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The granted Afterlife trigger is typed out in this leaf while the printed one
// comes from cards.kwAfterlife. Pin them together, for each count, so an edit to
// one cannot leave the other behind.
func TestGrantedAfterlifeShapeMatchesPrintedExpansion(t *testing.T) {
	for _, n := range []string{"1", "2", "3"} {
		line := "Afterlife:" + n
		c, diags := cards.ParseBytes("afterlife_shape.txt", []byte("Name:Afterlifer\nManaCost:1 W\nTypes:Creature Test\nPT:1/1\nK:"+line+"\n"))
		if len(diags) != 0 || c == nil || len(c.Faces) != 1 {
			t.Fatalf("parse: card=%v diags=%v", c, diags)
		}
		c.Link() // the keyword expansion runs at Link
		var printed *cards.Trigger
		for i := range c.Faces[0].Triggers {
			if c.Faces[0].Triggers[i].ParamStr(cards.PKKeywordLine) == line {
				printed = &c.Faces[0].Triggers[i]
			}
		}
		if printed == nil {
			t.Fatalf("precondition: K:%s expanded to no keyword trigger: %+v", line, c.Faces[0].Triggers)
		}
		body := cards.ResolveSVar(c.Faces[0].SVars, printed.ParamStr(cards.PKExecute))
		if body == nil {
			t.Fatalf("precondition: the printed %s trigger's Execute$ body does not resolve", line)
		}
		granted := GrantedKeywordTrigger(line)
		if granted == nil || granted.Effect == nil {
			t.Fatalf("GrantedKeywordTrigger(%s) = nil", line)
		}
		shape := func(m map[string]string) map[string]string {
			out := map[string]string{}
			for k, v := range m {
				if k != "Execute" && k != "Keyword" && k != "KeywordLine" {
					out[k] = v
				}
			}
			return out
		}
		if printed.Mode != granted.Mode || !reflect.DeepEqual(shape(printed.Params), shape(granted.Params)) {
			t.Errorf("%s trigger: printed %s %v, granted %s %v", line, printed.Mode, printed.Params, granted.Mode, granted.Params)
		}
		if body.API != granted.Effect.API || body.Kind != granted.Effect.Kind || !reflect.DeepEqual(body.Params, granted.Effect.Params) {
			t.Errorf("%s body: printed %s/%s %v, granted %s/%s %v", line, body.Kind, body.API, body.Params,
				granted.Effect.Kind, granted.Effect.API, granted.Effect.Params)
		}
		if got, want := GrantedKeywordPayload(line), GrantedAfterlifePayloadPrefix+n; got != want {
			t.Errorf("payload(%s) = %q, want %q", line, got, want)
		}
	}
	for _, bad := range []string{"Afterlife", "Afterlife:", "Afterlife:0", "Afterlife:X", "Afterlife:1 | X", "Afterlife:01", "Afterlife:-1"} {
		if GrantedKeywordTrigger(bad) != nil {
			t.Errorf("GrantedKeywordTrigger(%q) synthesized a trigger; a non-positive-literal count must fail closed", bad)
		}
	}
}
