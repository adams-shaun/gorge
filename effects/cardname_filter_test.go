package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestCardnameFilterIsSourceIdentity(t *testing.T) {
	g, ids := board(t)
	source := ids["myBear"]
	// A value snapshot with the SAME face/name but a different object identity.
	// No game mutation is needed to distinguish CR 201.5 from a name match.
	other := *g.Obj(source)
	other.ID = ids["myFlier"]
	sc := SpecContext{You: 0, Source: source}
	for _, tc := range []struct {
		spec string
		obj  *state.Object
		ctx  SpecContext
		want bool
	}{
		{"CARDNAME", g.Obj(source), sc, true},
		{"CARDNAME", &other, sc, false},
		{"CARDNAME", g.Obj(source), SpecContext{You: 0}, false},
		{"CARDNAME.YouCtrl", g.Obj(source), sc, true},
		{"CARDNAME.OppCtrl", g.Obj(source), sc, false},
		{"CARDNAME.Unknown", g.Obj(source), sc, false},
		{"CARDNAME,Land", g.Obj(ids["myLand"]), sc, true},
		{"CARDNAMES", g.Obj(source), sc, false},
	} {
		if got := MatchesObjectCtx(g, tc.spec, tc.obj, tc.ctx); got != tc.want {
			t.Errorf("%s obj=%d source=%d: got %v want %v", tc.spec, tc.obj.ID, tc.ctx.Source, got, tc.want)
		}
	}
	if !MatchesSpecCtx(g, "CARDNAME", source, sc) || !MatchesSpecFrom(g, "CARDNAME", source, 0, source) {
		t.Fatal("source-aware wrappers must match source")
	}
	if MatchesSpec(g, "CARDNAME", source, 0) || MatchesSpecFrom(g, "CARDNAME", 0, 0, source) {
		t.Fatal("absent source/candidate must not match")
	}
}
