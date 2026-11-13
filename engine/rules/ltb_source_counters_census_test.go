package rules

// Corpus-carrier census for the leaves-the-battlefield source-counter read
// (issue lb-ltbcounters). The repair is structural: the bare
// `Count$CardCounters.<KIND>` head now reads the trigger's LKI snapshot when
// its source has left the battlefield, so every card whose departure trigger
// counts its OWN source's counters through that head depends on it -- not only
// Selfless Police Captain, the card the level-B oracle scenario named. This
// census records that class so a corpus-pin bump that adds or removes a carrier
// is named rather than silently joining an untested set.
//
// It is a data census in the established per-card ratchet shape
// (engine/rules/oracle_carrier_census_test.go, engine/rules/paramcensus_test.go): one file per
// card under engine/rules/testdata/corpus-ltb-source-counters/, loaded with the shared
// loadPerCardLabels, and compared in BOTH directions -- a new carrier is a
// regression to classify, a stale entry is a card the corpus no longer carries
// and must be deleted.
//
// Scope: the walk follows a departure trigger's effect chain (through
// SubAbility$ links) and the named SVar each parameter references, and matches
// only the BARE `Count$CardCounters.` head. It deliberately excludes the
// ref-prefixed siblings (`TriggeredCard$CardCounters.`, `RememberedLKI$...`),
// which route through the separate evalRefProperty path and already read LKI;
// and it never walks a face's activation/spell abilities, whose source is the
// live permanent and is read live by contract.

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// censusLabelLTBSourceCounters names the mechanism, not a card: a departure
// trigger whose effect counts its own source's counters via the bare
// Count$CardCounters head.
const censusLabelLTBSourceCounters = "count:CardCounters.SourceLeavesBattlefield"

// bareCardCountersHead is the substring every carrier's SVar body holds. The
// leading "Count$" is what makes it the bare head: the ref-prefixed forms
// (TriggeredCard$CardCounters., RememberedLKI$CardCounters.) do not contain it.
const bareCardCountersHead = "Count$CardCounters."

// validCardIsSelf reports whether a trigger's ValidCard$ selects the source
// itself -- `Card.Self` (with any +predicate suffix) or the CARDNAME token.
// A trigger that selects some other card (an Aura's Card.AttachedBy or
// Card.EnchantedBy, Sierra's Food) fires while the source is still on the
// battlefield, so its counter read is live and it is NOT this class.
func validCardIsSelf(tr *cards.Trigger) bool {
	vc := tr.Params["ValidCard"]
	return strings.Contains(vc, "Card.Self") || strings.Contains(vc, "CARDNAME")
}

// departureTrigger reports whether a trigger fires when its own source leaves
// the battlefield -- the zone-change trigger (CR 603.6c) or a sacrifice trigger
// (CR 603.6d) naming the source itself. A trigger on some OTHER card leaving
// (an Aura's "when enchanted creature leaves", Traveling Plague) is not this
// class: the source stays and is read live.
func departureTrigger(tr *cards.Trigger) bool {
	switch tr.Mode {
	case "ChangesZone":
		return strings.Contains(tr.Params["Origin"], "Battlefield") && validCardIsSelf(tr)
	case "Sacrificed", "LeavesBattlefield":
		return validCardIsSelf(tr)
	}
	return false
}

// svarReachesBareCardCounters resolves a parameter value to the named SVar it
// references (Forge writes `CounterNum$ X` for `SVar:X:Count$CardCounters...`)
// and reports whether that body carries the bare head. A value that is already
// a Count$ body is read directly. Depth bounds the SVar->SVar chase so a
// pathological self-reference cannot loop.
func svarReachesBareCardCounters(f *cards.Face, raw string, depth int) bool {
	if strings.Contains(raw, bareCardCountersHead) {
		return true
	}
	if depth <= 0 {
		return false
	}
	token := strings.TrimLeft(strings.TrimSpace(raw), "+-")
	body, ok := f.SVars[token]
	if !ok {
		return false
	}
	return svarReachesBareCardCounters(f, body, depth-1)
}

// faceLeavesBattlefieldReadsOwnCounters reports whether f has a departure
// trigger whose effect chain (through SubAbility$) counts the source's own
// counters through the bare head.
func faceLeavesBattlefieldReadsOwnCounters(f *cards.Face) bool {
	for _, tr := range f.Triggers {
		if !departureTrigger(&tr) {
			continue
		}
		for sa := tr.Effect; sa != nil; sa = sa.Sub {
			for _, v := range sa.Params {
				if svarReachesBareCardCounters(f, v, 2) {
					return true
				}
			}
		}
	}
	return false
}

// ltbSourceCountersCarriers walks the compiled corpus and returns the set of
// card names whose departure trigger reads its own source's counters. Derived,
// never hand-listed, so a new carrier is measured rather than missed.
func ltbSourceCountersCarriers(reg *cards.Registry) map[string]bool {
	out := map[string]bool{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if f != nil && faceLeavesBattlefieldReadsOwnCounters(f) {
				out[f.Name] = true
			}
		}
	}
	return out
}

// TestCorpusLTBSourceCountersFilesWellFormed holds the census directory to the
// per-card schema without a corpus.
func TestCorpusLTBSourceCountersFilesWellFormed(t *testing.T) {
	t.Parallel()
	loadCensusLabels(t, "corpus-ltb-source-counters")
}

// TestCorpusLTBSourceCountersCensus is the ratchet for the departure-trigger
// source-counter read the LKI snapshot feeds.
func TestCorpusLTBSourceCountersCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	got := ltbSourceCountersCarriers(reg)
	want := loadCensusLabels(t, "corpus-ltb-source-counters")

	// Precondition: the walk reached the class and the card the fix was
	// reported for is a member. A registry that failed to load, or a walk
	// whose trigger/SVar chaining stopped resolving, must fail rather than
	// pass vacuously.
	if len(got) == 0 {
		t.Fatalf("%s: census found no carriers at all; the corpus walk is broken", censusLabelLTBSourceCounters)
	}
	if !got["Selfless Police Captain"] {
		t.Fatalf("%s: precondition: %q is not a measured carrier, got %v",
			censusLabelLTBSourceCounters, "Selfless Police Captain", sortedSetKeys(got))
	}

	var gotNames, wantNames []string
	for n := range got {
		gotNames = append(gotNames, n)
	}
	for n := range want {
		wantNames = append(wantNames, n)
	}
	sort.Strings(gotNames)
	sort.Strings(wantNames)
	if strings.Join(gotNames, "; ") != strings.Join(wantNames, "; ") {
		t.Fatalf("%s carrier cards changed:\n got: %v\nwant: %v", censusLabelLTBSourceCounters, gotNames, wantNames)
	}
	for name, labels := range want {
		if len(labels) != 1 || labels[0] != censusLabelLTBSourceCounters {
			t.Errorf("%s: %s labels = %v, want [%s]", censusLabelLTBSourceCounters, name, labels, censusLabelLTBSourceCounters)
		}
	}
}
