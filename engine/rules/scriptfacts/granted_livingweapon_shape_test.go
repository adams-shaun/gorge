package scriptfacts

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The granted Living Weapon trigger is typed out in this leaf (a granted
// keyword has no face to expand) while the printed one comes from
// cards.kwLivingWeapon. Pin them together, body chain included, so an edit to
// one cannot leave the other (and dirac's lowering, which reads this
// synthesizer) behind.
func TestGrantedLivingWeaponShapeMatchesPrintedExpansion(t *testing.T) {
	c, diags := cards.ParseBytes("lw_shape.txt", []byte("Name:Wielder\nManaCost:1\nTypes:Artifact Equipment\nK:Living Weapon\n"))
	if len(diags) != 0 || c == nil || len(c.Faces) != 1 {
		t.Fatalf("parse: card=%v diags=%v", c, diags)
	}
	c.Link()
	var printed *cards.Trigger
	for i := range c.Faces[0].Triggers {
		if c.Faces[0].Triggers[i].ParamStr(cards.PKKeywordLine) == "Living Weapon" {
			printed = &c.Faces[0].Triggers[i]
		}
	}
	if printed == nil || printed.Effect == nil || printed.Effect.Sub == nil {
		t.Fatalf("precondition: K:Living Weapon expanded to no trigger with a two-step body: %+v", c.Faces[0].Triggers)
	}
	granted := GrantedKeywordTrigger("Living Weapon")
	if granted == nil || granted.Effect == nil || granted.Effect.Sub == nil {
		t.Fatal("GrantedKeywordTrigger(Living Weapon) = nil or has no two-step body")
	}
	if GrantedKeywordTrigger("Living Weapon:1") != nil {
		t.Fatal("a parametrised Living Weapon spelling must fail closed")
	}
	if printed.Mode != granted.Mode {
		t.Fatalf("Mode: printed %q, granted %q", printed.Mode, granted.Mode)
	}
	shape := func(m map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range m {
			if k != "Execute" && k != "Keyword" && k != "KeywordLine" && k != "SubAbility" {
				out[k] = v
			}
		}
		return out
	}
	if p, g := shape(printed.Params), shape(granted.Params); !reflect.DeepEqual(p, g) {
		t.Errorf("trigger params: printed %v, granted %v", p, g)
	}
	pe, ge := printed.Effect, granted.Effect
	for step := 0; pe != nil || ge != nil; step, pe, ge = step+1, nextSub(pe), nextSub(ge) {
		if pe == nil || ge == nil {
			t.Fatalf("body step %d: printed chain %v, granted chain %v differ in length", step, pe != nil, ge != nil)
		}
		if pe.Kind != ge.Kind || pe.API != ge.API || !reflect.DeepEqual(shape(pe.Params), shape(ge.Params)) {
			t.Errorf("body step %d: printed %s/%s %v, granted %s/%s %v", step, pe.Kind, pe.API, pe.Params, ge.Kind, ge.API, ge.Params)
		}
	}
	if got := GrantedKeywordPayload("Living Weapon"); got != GrantedLivingWeaponPayload {
		t.Fatalf("payload = %q, want %q", got, GrantedLivingWeaponPayload)
	}
}

func nextSub(sa *cards.SA) *cards.SA {
	if sa == nil {
		return nil
	}
	return sa.Sub
}
