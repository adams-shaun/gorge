package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

const kctrGreatGoblin = "Goblin.YouCtrl+inRealZoneBattlefield,Orc.YouCtrl+inRealZoneBattlefield,Army.YouCtrl+inRealZoneBattlefield"

// kctrLower lowers a synthetic creature with one CounterPlayerAddedAll
// trigger (The Great Goblin's body) carrying params.
func kctrLower(t *testing.T, params map[string]string) *lCard {
	t.Helper()
	svars := map[string]string{"TrigDealDamage": "DB$ DealDamage | ValidTgts$ Opponent | NumDmg$ 2"}
	params["Mode"] = "CounterPlayerAddedAll"
	params["TriggerZones"] = "Battlefield"
	params["Execute"] = "TrigDealDamage"
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "1 R",
		Types:    []string{"Creature", "Goblin"},
		SVars:    svars,
		Triggers: []cards.Trigger{{
			Mode:   "CounterPlayerAddedAll",
			Params: params,
			Effect: cards.ResolveSVar(svars, "TrigDealDamage"),
		}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

// TestCtrPlayerAddedLowers: Mode$ CounterPlayerAddedAll over object
// recipients lowers to the kernel's any-kind CounterAddedOnce trigger; a
// player recipient or an absent ValidSource$ is refused by name.
func TestCtrPlayerAddedLowers(t *testing.T) {
	lc := kctrLower(t, map[string]string{"ValidObject": kctrGreatGoblin, "ValidSource": "You"})
	if len(lc.Refusals) > 0 {
		t.Fatalf("The Great Goblin's line refused: %v", lc.Refusals)
	}
	var tr *lTrigger
	for _, a := range lc.Abilities {
		if a.Trigger != nil {
			tr = a.Trigger
		}
	}
	if tr == nil {
		t.Fatal("no trigger lowered")
	}
	if tr.Event != "CounterAddedOnce" || tr.Whose != whoYou || tr.Step != "*" {
		t.Fatalf("trigger Event %q Whose %d Step %q, want CounterAddedOnce, You, *", tr.Event, tr.Whose, tr.Step)
	}
	if tr.F == nil || len(tr.F.Clauses) != 3 {
		t.Fatalf("filter %v, want 3 clauses (Goblin, Orc, Army)", tr.F)
	}
	for _, c := range tr.F.Clauses {
		if c.Ctrl != whoYou {
			t.Errorf("clause %+v: not YouCtrl", c)
		}
	}
	lw := &lowered{cards: []*lCard{lc}}
	cb := &cardBuilder{pt: newPoolTables(lw, nil), c: lc}
	kt, err := cb.trigger(tr)
	if err != nil {
		t.Fatal(err)
	}
	if kt.Event != kop.TrigCounterAddedOnce || kt.N != kop.TrigNAnyCounter || kt.Whose != kop.WhoseYou {
		t.Fatalf("built trigger Event %d N %d Whose %d, want CounterAddedOnce, TrigNAnyCounter, WhoseYou", kt.Event, kt.N, kt.Whose)
	}

	// a named CounterType$ keeps its kind
	lc = kctrLower(t, map[string]string{"ValidObject": kctrGreatGoblin, "ValidSource": "You", "CounterType": "P1P1"})
	if len(lc.Refusals) > 0 {
		t.Fatalf("CounterType$ P1P1 refused: %v", lc.Refusals)
	}

	for _, c := range []struct {
		name string
		ps   map[string]string
		want string
	}{
		{"player recipient", map[string]string{"ValidObject": "Permanent.YouCtrl,Player", "ValidSource": "You"},
			`CounterPlayerAddedAll player recipient "Player"`},
		{"no ValidSource", map[string]string{"ValidObject": kctrGreatGoblin}, "CounterPlayerAddedAll ValidSource"},
		{"opponent source", map[string]string{"ValidObject": kctrGreatGoblin, "ValidSource": "Opponent"}, "CounterPlayerAddedAll ValidSource"},
		{"no ValidObject", map[string]string{"ValidSource": "You"}, "CounterPlayerAddedAll without ValidObject"},
		{"ValidObjectToSource", map[string]string{"ValidObject": kctrGreatGoblin, "ValidSource": "You", "ValidObjectToSource": "Card.Self"},
			"CounterPlayerAddedAll ValidObjectToSource$"},
	} {
		rs := kctrLower(t, c.ps).Refusals
		found := false
		for _, r := range rs {
			found = found || strings.Contains(r.Reason, c.want)
		}
		if !found {
			t.Errorf("%s: want a refusal containing %q, got %v", c.name, c.want, rs)
		}
	}
}
