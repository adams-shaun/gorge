package effects

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// scriptCounterKindParams are the card-script parameters that carry a counter
// kind into an event. WithCounters/WithCountersType/ExileWithCounters feed
// the entry/exile counter helpers, and CounterType/CounterType2/CounterTypes
// feed the PutCounter, RemoveCounter, MoveCounter, MultiplyCounter and
// PutCounterAll readers.
var scriptCounterKindParams = []string{
	"CounterType", "CounterType2", "CounterTypes",
	"WithCounters", "WithCountersType", "ExileWithCounters",
}

// TestScriptCounterKindsAreUpperCased is the corpus census for the
// script-string-to-counter-kind boundary. Forge resolves counter kinds
// case-insensitively (CounterEnumType.getType upper-cases the name), so the
// corpus may spell a kind differently from the engine's storage spelling:
// Elspeth Resplendent's `WithCounters$ Shield` must place the REAL "SHIELD"
// counter (CR 122.1c), never the engine's mixed-case "Shield" regeneration
// marker. Every script kind is folded through state.CanonicalCounterKind
// before an event, and this census asserts the fold never leaves a kind equal
// to state.InternalCounterMarker.
//
// The census is deliberately sensitive to removing the fold: the corpus
// carries the raw "Shield" spelling, so dropping the fold makes the
// normalised kind equal the internal marker and fails here (as
// TestElspethResplendentShieldCounter fails behaviourally).
func TestScriptCounterKindsAreUpperCased(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var (
		checked       int
		rawNonUpper   []string
		sawShield     bool
		shieldFolded  bool
		badNormalised []string
	)
	countKind := func(face *cards.Face, key, raw string) {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			checked++
			norm := state.CanonicalCounterKind(part)
			if norm != part {
				// The corpus contains a raw spelling the fold changes:
				// prove it here so removing the fold cannot pass vacuously.
				if part == "Shield" && norm == "SHIELD" {
					shieldFolded = true
				}
			}
			if part == "Shield" {
				sawShield = true
			}
			if part != strings.ToUpper(part) {
				rawNonUpper = append(rawNonUpper, face.Name+" "+key+"$ "+part)
			}
			if state.InternalCounterMarker(norm) {
				badNormalised = append(badNormalised, face.Name+" "+key+"$ "+part+" -> "+norm)
			}
		}
	}
	countParams := func(face *cards.Face, params map[string]string) {
		for _, key := range scriptCounterKindParams {
			if raw, ok := params[key]; ok {
				countKind(face, key, raw)
			}
		}
	}
	for _, card := range reg.AllCards() {
		for _, face := range card.Faces {
			var sas []*cards.SA
			sas = append(sas, face.Abilities...)
			for i := range face.Triggers {
				countParams(face, face.Triggers[i].Params)
				sas = append(sas, face.Triggers[i].Effect)
			}
			for i := range face.Statics {
				countParams(face, face.Statics[i].Params)
			}
			for i := range face.Repls {
				countParams(face, face.Repls[i].Params)
				sas = append(sas, face.Repls[i].With)
			}
			for name := range face.SVars {
				sas = append(sas, cards.ResolveSVar(face.SVars, name))
			}
			for _, sa := range sas {
				for ; sa != nil; sa = sa.Sub {
					countParams(face, sa.Params)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("census found no counter-kind parameters: the corpus is missing or the parameter keys changed")
	}
	if len(rawNonUpper) == 0 {
		t.Fatal("no raw script counter kind is non-upper-case: the census can no longer observe the fold")
	}
	if !sawShield {
		t.Fatal("no script carries `Shield` as a counter kind: the regression this census guards (Elspeth Resplendent's -3) is no longer in the corpus")
	}
	if !shieldFolded {
		t.Fatalf("raw script kind %q did not fold to %q: state.CanonicalCounterKind lost the Shield entry", "Shield", "SHIELD")
	}
	if len(badNormalised) > 0 {
		sort.Strings(badNormalised)
		for _, bad := range badNormalised {
			t.Errorf("script counter kind normalises to an internal marker: %s", bad)
		}
	}
}
