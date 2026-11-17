package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

const exileCounterRiderReason = "repl-hard: dies-to-exile counter rider"

func refusalReasons(t *testing.T, name string) []string {
	t.Helper()
	cs := lowerNamed(t, []string{name})
	if len(cs) != 1 {
		t.Fatalf("%s: got %d cards", name, len(cs))
	}
	var out []string
	for _, r := range cs[0].Refusals {
		out = append(out, r.Reason)
	}
	return out
}

// TestExileCounterRiderRefusedByName: a dies-to-exile rider that still has no
// lowering (Ravenloft Adventurer's RememberChanged$ + PutCounter chain) stays
// refused by name instead of the generic "Moved to Graveyard shape". Dauthi
// Voidwalker and Draugr Necromancer lower now (exileonleave_rider_forms_test.go)
// and Rayami's Kind/Amount body does too; its own reader is a different gap.
func TestExileCounterRiderRefusedByName(t *testing.T) {
	rs := refusalReasons(t, "Ravenloft Adventurer")
	found := false
	for _, r := range rs {
		found = found || strings.Contains(r, exileCounterRiderReason)
	}
	if !found {
		t.Errorf("Ravenloft Adventurer: refusals %q do not name %q", rs, exileCounterRiderReason)
	}
}

// TestExileCounterRiderPredicate exercises the predicate directly, so the
// control does not depend on another lowering consuming the body first. The
// amount arm needs a kind, and the sub-ability arm needs RememberChanged$
// True: an amount alone or a bare PutCounter is not a counter rider.
func TestExileCounterRiderPredicate(t *testing.T) {
	body := func(params map[string]string, sub *cards.SA) *cards.SA {
		return &cards.SA{API: "ChangeZone", Params: params, Sub: sub}
	}
	base := func() map[string]string {
		return map[string]string{"Defined": "ReplacedCard", "Destination": "Exile", "Origin": "Battlefield"}
	}
	withType := base()
	withType["WithCountersType"] = "VOID"
	amountOnly := base()
	amountOnly["WithCountersAmount"] = "1"
	remembered := base()
	remembered["RememberChanged"] = "True"
	noRemember := base()
	putCounter := &cards.SA{API: "PutCounter", Params: map[string]string{"Defined": "Remembered"}}
	notRemembered := &cards.SA{API: "PutCounter", Params: map[string]string{"Defined": "ReplacedCard"}}
	library := map[string]string{"Defined": "ReplacedCard", "Destination": "Library", "Origin": "Battlefield", "WithCountersType": "VOID"}

	for _, tc := range []struct {
		name string
		sa   *cards.SA
		want bool
	}{
		{"type", body(withType, nil), true},
		{"type plus sub", body(withType, putCounter), true},
		{"amount without kind", body(amountOnly, nil), false},
		{"putcounter remembered", body(remembered, putCounter), true},
		{"putcounter without RememberChanged", body(noRemember, putCounter), false},
		{"putcounter not on Remembered", body(remembered, notRemembered), false},
		{"plain body", body(base(), nil), false},
		{"destination library", body(library, nil), false},
	} {
		if got := exileCounterRider(tc.sa); got != tc.want {
			t.Errorf("%s: exileCounterRider = %v, want %v", tc.name, got, tc.want)
		}
	}
}
