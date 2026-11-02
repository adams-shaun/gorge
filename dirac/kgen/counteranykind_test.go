package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// kcaLower lowers a synthetic creature carrying one Mode$ CounterAdded
// trigger (Mikey & Leo's draw line) with params merged over the base.
func kcaLower(t *testing.T, extra map[string]string) *lCard {
	t.Helper()
	svars := map[string]string{"TrigDraw": "DB$ Draw | Defined$ You | NumCards$ 1"}
	params := map[string]string{
		"Mode":         "CounterAdded",
		"ValidCard":    "Creature.YouCtrl",
		"ValidSource":  "You",
		"TriggerZones": "Battlefield",
		"Execute":      "TrigDraw",
	}
	for k, v := range extra {
		params[k] = v
	}
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "1 G",
		Types:    []string{"Creature", "Turtle"},
		SVars:    svars,
		Triggers: []cards.Trigger{{
			Mode:   "CounterAdded",
			Params: params,
			Effect: cards.ResolveSVar(svars, "TrigDraw"),
		}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

func kcaRefused(lc *lCard, want string) bool {
	for _, r := range lc.Refusals {
		if strings.Contains(r.Reason, want) {
			return true
		}
	}
	return false
}

// TestCounterAnyKindLowers: Mode$ CounterAdded with no CounterType$ and
// ActivationLimit$ 1 lowers to the kernel's any-kind one-per-turn trigger; the
// other shapes refuse by name.
func TestCounterAnyKindLowers(t *testing.T) {
	lc := kcaLower(t, map[string]string{"ActivationLimit": "1"})
	if len(lc.Refusals) > 0 {
		t.Fatalf("any-kind CounterAdded refused: %v", lc.Refusals)
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
	if tr.Event != "CounterAdded" || tr.Step != "*" || tr.Whose != whoYou {
		t.Fatalf("trigger Event %q Step %q Whose %d, want CounterAdded, *, You", tr.Event, tr.Step, tr.Whose)
	}
	lw := &lowered{cards: []*lCard{lc}}
	cb := &cardBuilder{pt: newPoolTables(lw, nil), c: lc}
	kt, err := cb.trigger(tr)
	if err != nil {
		t.Fatal(err)
	}
	if kt.Event != kop.TrigCounterAddedOnce || kt.N != kop.TrigNAnyCounter || kt.Flags&kop.TfOncePerTurn == 0 {
		t.Fatalf("built trigger Event %d N %d Flags %#x, want CounterAddedOnce, TrigNAnyCounter, TfOncePerTurn", kt.Event, kt.N, kt.Flags)
	}

	if lc := kcaLower(t, nil); !kcaRefused(lc, "CounterAdded per-counter (no CounterAmount$)") {
		t.Errorf("no ActivationLimit$: want the per-counter refusal, got %v", lc.Refusals)
	}
	if lc := kcaLower(t, map[string]string{"CounterAmount": "EQ3"}); !kcaRefused(lc, "CounterAdded CounterAmount$ without CounterType") {
		t.Errorf("CounterAmount$ EQ3: want the named refusal, got %v", lc.Refusals)
	}
}
