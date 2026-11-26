package kgen

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// stationThresholdLower lowers a probe Spacecraft carrying lines and returns
// the lowered card.
func stationThresholdLower(t *testing.T, lines ...string) *lCard {
	t.Helper()
	src := strings.Join(append([]string{"Name:Probe", "ManaCost:1 G", "Types:Artifact Spacecraft", "K:Station"}, lines...), "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerPool(p).cards[0]
}

// TestStationThresholdSpellingsLowerAlike: Card-Forge acaef15177 re-spelled the
// Spacecraft STATION threshold static from Affected$ Card.counters_GE<N>_CHARGE
// to Station$ <N>. Both spellings lower to the same statics, a card mixing them
// lowers, and a Station$ the threshold gate cannot carry stays refused.
func TestStationThresholdSpellingsLowerAlike(t *testing.T) {
	old := stationThresholdLower(t, "PT:3/2",
		"S:Mode$ Continuous | AffectedDefined$ Self | Affected$ Card.counters_GE8_CHARGE | AddType$ Creature | AddKeyword$ Flying & Lifelink | Description$ STATION 8+")
	nw := stationThresholdLower(t, "PT:3/2",
		"S:Mode$ Continuous | AffectedDefined$ Self | Station$ 8 | AddType$ Creature | AddKeyword$ Flying & Lifelink | Description$ STATION 8+")
	if len(old.Refusals) != 0 {
		t.Fatalf("old spelling refused: %v", old.Refusals)
	}
	if len(nw.Refusals) != 0 {
		t.Fatalf("Station$ spelling refused: %v", nw.Refusals)
	}
	if len(nw.Statics) == 0 {
		t.Fatal("precondition: the Station$ static lowered to no statics")
	}
	if !reflect.DeepEqual(old.Statics, nw.Statics) {
		t.Fatalf("spellings lower differently:\nold %+v\nnew %+v", old.Statics, nw.Statics)
	}
	mixed := stationThresholdLower(t, "PT:3/2",
		"S:Mode$ Continuous | AffectedDefined$ Self | Station$ 3 | AddType$ Creature | Description$ STATION 3+",
		"S:Mode$ Continuous | AffectedDefined$ Self | Station$ 12 | AddKeyword$ Flying | Description$ STATION 12+")
	if len(mixed.Refusals) != 0 || len(mixed.Statics) < 2 {
		t.Fatalf("two Station$ statics: refusals %v, %d statics", mixed.Refusals, len(mixed.Statics))
	}
	for _, bad := range []string{"0", "256", "X", ""} {
		c := stationThresholdLower(t, "PT:3/2",
			"S:Mode$ Continuous | AffectedDefined$ Self | Station$ "+bad+" | AddKeyword$ Flying | Description$ STATION")
		if len(c.Refusals) == 0 {
			t.Errorf("Station$ %q lowered; want a refusal", bad)
		}
	}
}
