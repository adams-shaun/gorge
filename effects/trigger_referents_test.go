package effects

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestTriggerReferentFilters(t *testing.T) {
	g, ids := board(t)
	sc := SpecContext{Source: ids["myBear"], TriggerContext: TriggerContext{
		TriggerTarget: state.Target{Obj: ids["theirBig"]}, TriggerSource: ids["myBear"],
		TriggerCard: ids["theirBig"], TriggerPlayer: state.Target{IsPlayer: true, Player: 0},
		DefendingPlayer: state.Target{IsPlayer: true, Player: 1},
	}}
	for _, ref := range []string{"TriggeredTarget", "TriggeredCard", "TriggeredPlayer", "TriggeredDefendingPlayer"} {
		for _, op := range []string{"ControlledBy", "OwnedBy"} {
			spec := "Card." + op + " " + ref
			want := state.PlayerID(1)
			if ref == "TriggeredPlayer" {
				want = 0
			}
			// Deliberately distinct controller and owner on an object value: ownership
			// is not control. Neither the live game nor its objects are mutated here.
			o := &state.Object{Owner: 0, Controller: 1}
			got := MatchesObjectCtx(g, spec, o, sc)
			expected := (op == "OwnedBy" && want == 0) || (op == "ControlledBy" && want == 1)
			if got != expected {
				t.Errorf("%s = %v, want %v", spec, got, expected)
			}
			if unknown := UnknownPredicates(spec); len(unknown) != 0 {
				t.Errorf("recognised %s reported %v", spec, unknown)
			}
		}
	}
	// Both player and object target forms; seat zero is a PRESENT player.
	sc.TriggerTarget = state.Target{IsPlayer: true, Player: 0}
	if !MatchesSpecCtx(g, "Creature.ControlledBy TriggeredTarget", ids["myBear"], sc) {
		t.Fatal("seat zero target lost")
	}
	sc.TriggerTarget = state.Target{Obj: 999999}
	if MatchesSpecCtx(g, "Creature.ControlledBy TriggeredTarget", ids["myBear"], sc) {
		t.Fatal("missing object widened")
	}
}

func TestTriggerReferentWithoutContextFailsClosed(t *testing.T) {
	g, ids := board(t)
	for _, ref := range []string{"TriggeredTarget", "TriggeredCard", "TriggeredPlayer", "TriggeredDefendingPlayer"} {
		for _, op := range []string{"ControlledBy", "OwnedBy"} {
			for _, bang := range []string{"", "!"} {
				spec := "Creature." + bang + op + " " + ref
				for _, id := range []state.ObjID{ids["myBear"], ids["theirBig"]} {
					if MatchesSpecCtx(g, spec, id, SpecContext{You: 0, Source: ids["myBear"]}) {
						t.Errorf("zero context matched %s object %d", spec, id)
					}
				}
			}
		}
	}
}

func TestTriggerReferentGrammarScope(t *testing.T) {
	g, ids := board(t)
	sc := SpecContext{TriggerContext: TriggerContext{TriggerTarget: state.Target{IsPlayer: true, Player: 0}}}
	// A raw Targeted word is not itself a predicate. Its recognised use is as
	// the argument to ControlledBy/OwnedBy and is covered separately by the
	// resolution-only Targeted* leaf below.
	for _, ref := range []string{"Spawner>TriggeredTarget", "Targeted", "TriggeredTargetController"} {
		for _, prefix := range []string{"", "ControlledBy ", "OwnedBy "} {
			token := prefix + ref
			if ref == "Targeted" && prefix != "" {
				continue
			}
			spec := "Creature." + token
			if MatchesSpecCtx(g, spec, ids["myBear"], sc) {
				t.Errorf("out-of-scope %s matched", spec)
			}
			if got := UnknownPredicates(spec); !reflect.DeepEqual(got, []string{token}) {
				t.Errorf("%s unknown=%v, want [%s]", spec, got, token)
			}
		}
	}
}
