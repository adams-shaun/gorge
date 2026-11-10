package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDBManaActivatedOnlyForms pins the one activated-only restriction a
// resolving DB$ Mana lowers (an undotted RestrictValid$ Activated, the
// kop.SpmActivated marker; Boommobile) and the neighbours that stay refused.
func TestDBManaActivatedOnlyForms(t *testing.T) {
	newLC := func(types uint16) *lowerCtx {
		return &lowerCtx{ab: &lAbility{Kind: akTriggered}, out: &lCard{Types: types}}
	}
	// (a) the undotted Activated is the marker: one clause, nothing but Special
	lc := newLC(0)
	f, err := lc.dbManaRestrict(newParams(map[string]string{"Produced": "Any", "RestrictValid": "Activated"}))
	if err != nil {
		t.Fatalf("Activated: %v", err)
	}
	if !restrictActOnly(f) || f.Clauses[0].Special != kop.SpmActivated {
		t.Fatalf("Activated: want the SpmActivated marker clause, got %+v", f)
	}
	if c := f.Clauses[0]; c.Types != 0 || c.NotTypes != 0 {
		t.Errorf("Activated: the marker clause carries a card filter: %+v", c)
	}
	if kop.SpmActivated&kop.SpmTurnFaceUp != 0 {
		t.Error("SpmActivated is inside SpmTurnFaceUp: a turn-up special action would spend it")
	}
	// (b) every other activated shape keeps its refusal
	for _, rv := range []string{"Activated.Creature", "Activated,Spell.Creature"} {
		_, err := newLC(0).dbManaRestrict(newParams(map[string]string{"Produced": "Any", "RestrictValid": rv}))
		if err == nil || !strings.Contains(err.Error(), "DB$ Mana RestrictValid$") {
			t.Errorf("RestrictValid$ %s: want the named refusal, got %v", rv, err)
		}
	}
	// (c) the marker lifts the artifact and Produced$ Any refusals; an
	// artifact host with plain mana, or a spell restriction, does not
	for _, tc := range []struct {
		types uint16
		ps    map[string]string
		want  string // "" lowers
	}{
		{tArtifact, map[string]string{"Produced": "Any", "Amount": "4", "RestrictValid": "Activated"}, ""},
		{tArtifact, map[string]string{"Produced": "R"}, "DB$ Mana of an artifact source (typed mana)"},
		{tArtifact, map[string]string{"Produced": "R", "RestrictValid": "Spell.Creature"}, "DB$ Mana of an artifact source (typed mana)"},
		{0, map[string]string{"Produced": "Any", "Amount": "2", "RestrictValid": "Spell.cmcGE4"}, "DB$ Mana RestrictValid$ over Produced$ Any"},
	} {
		lc := newLC(tc.types)
		err := lc.manaEffect(newParams(tc.ps))
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%v: want it lowered, got %v", tc.ps, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%v: want refusal %q, got %v", tc.ps, tc.want, err)
		}
	}
}
