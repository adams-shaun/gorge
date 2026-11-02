package kgen

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestOutlawFilterHead: the Outlaw filter head lowers to the Card.Outlaw clause
// (gorge matchesBaseOutlaw), the LeftBattlefieldThisTurn / Mod.N /
// GreatestCardPower heads lower, and the shapes outside the read refuse.
func TestOutlawFilterHead(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(s string) bool { return false }}
	a, err := lc.filter("Card.Outlaw+YouCtrl")
	if err != nil {
		t.Fatal(err)
	}
	b, err := lc.filter("Outlaw.YouCtrl")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Outlaw.YouCtrl = %v, Card.Outlaw+YouCtrl = %v: want the same clause", b, a)
	}
	for body, want := range map[string]valKind{
		"Count$LeftBattlefieldThisTurn Creature.YouCtrl":         vLeftZone,
		"Count$LeftBattlefieldThisTurn":                          vLeftZone,
		"Count$CardCounters.ALL/Mod.2":                           vMod,
		"Count$ValidGraveyard Creature.YouOwn$GreatestCardPower": vGraveAgg,
	} {
		v, err := lc.svarValue(body, 0)
		if err != nil || v.Kind != want {
			t.Errorf("%s: got %s, %v; want %s", body, v, err, valNames[want])
		}
	}
	for _, body := range []string{
		"Count$LeftBattlefieldThisTurn Creature.YouCtrl$Sum", "Count$LeftBattlefieldThisTurn Artifact",
		"Count$CardCounters.ALL/Mod.0", "Count$CardCounters.ALL/Mod.X", "Count$CardCounters.ALL/Mod.-2",
		"Count$CardCounters.ALL/Twice/Mod.2", "Count$ValidGraveyard Creature.YouOwn$LeastCardPower",
	} {
		if v, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%s: lowered to %s, want a refusal", body, v)
		}
	}
}
