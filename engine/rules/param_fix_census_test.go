package rules

// Derived, compiled-registry corpus census for eight parameter shapes.
//
// The existing engine/effects/ ratchets (engine/effects/unread_param_corpus_test.go
// TestUnreadEffectParamCorpusCarriers and engine/effects/count_pow_test.go
// TestNumberPowCorpusCarriers) scan RAW .txt text with strings.Contains(line,
// api) + strings.Contains(line, key). That is a substring match: api "Animate"
// matches a line whose token is AnimateAll, api "PutCounter" matches
// PutCounterAll/PutCounters, and a per-line key match cannot tell which SA the
// key sits on. No false positive exists at the current pin, but the shape is
// fragile: the next `AnimateAll | ... | Colors$ ChosenColor` line would be a
// false positive a compiled census cannot produce.
//
// This census is the durable, exact-keyed home for the same class. It walks
// the COMPILED registry keyed on the exact (sa.API, key) pair -- the same IR
// the engine runs -- and pins the carrier set per shape, one directory and
// one file per card under engine/rules/testdata/, compared in BOTH directions with a
// loud precondition. A card carrying the key on a different API, or a future
// API whose name merely contains this one, cannot join the set.
//
// Deliberately NOT touched: the engine/effects/ raw-text ratchets stay as they are
// (they remain a cheap independent text-level check); paramcensus_test.go is a
// sequenced hot file and is not edited. See the report for why both homes
// coexist.

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Param census labels: one constant label per shape naming the mechanism
// (mirroring oracle_carrier_census_test.go's censusLabelSacrificedPower style).
// The values are the label strings carried by every file in the matching
// engine/rules/testdata/corpus-param-fix-* directory.
const (
	paramFixLabelPumpReplaceDying      = "param:api:Pump.ReplaceDyingDefined"
	paramFixLabelChangeZoneThisAndTgts = "param:api:ChangeZone.ThisDefinedAndTgts"
	paramFixLabelDigUntilMinTotalCMC   = "param:api:DigUntil.MinTotalCMC"
	paramFixLabelCopyPermanentDefName  = "param:api:CopyPermanent.DefinedName"
	paramFixLabelPutCounterTypes       = "param:api:PutCounter.CounterTypes"
	paramFixLabelAnimateColors         = "param:api:Animate.Colors"
	paramFixLabelChooseTypeTypes       = "param:api:ChooseType.ValidTypes+InvalidTypes"
	paramFixLabelNumberPow             = "count:Number.Pow"
)

// paramFixShape is one measured parameter shape. extra, when non-nil, narrows a
// compiled (API, key) match further -- the Animate shape's Colors$ value must
// name the literal ChosenColor, not any colour token.
type paramFixShape struct {
	label string
	api   string
	key   string
	// value contains a substring the parameter value must carry, when value
	// is non-empty.
	value string
}

// paramFixShapes is the eight-shape registry. It is the ONE home of the shape
// definitions; the walk and every test below derive from it.
var paramFixShapes = []paramFixShape{
	{label: paramFixLabelPumpReplaceDying, api: "Pump", key: "ReplaceDyingDefined"},
	{label: paramFixLabelChangeZoneThisAndTgts, api: "ChangeZone", key: "ThisDefinedAndTgts"},
	{label: paramFixLabelDigUntilMinTotalCMC, api: "DigUntil", key: "MinTotalCMC"},
	{label: paramFixLabelCopyPermanentDefName, api: "CopyPermanent", key: "DefinedName"},
	{label: paramFixLabelPutCounterTypes, api: "PutCounter", key: "CounterTypes"},
	{label: paramFixLabelAnimateColors, api: "Animate", key: "Colors", value: "ChosenColor"},
	{label: paramFixLabelChooseTypeTypes, api: "ChooseType", key: "ValidTypes"},
	{label: paramFixLabelChooseTypeTypes, api: "ChooseType", key: "InvalidTypes"},
}

// paramFixCarriers walks the compiled corpus and returns, per label, the set
// of card names carrying that shape. It follows the same traversal
// Face.Primitives uses -- direct Abilities, Trigger effects, Repl With bodies,
// every SVar-resolved ability chain (EachSVarAbility), and typed raw
// effect children -- because those are exactly the reachable SAs the engine
// runs. The Pow shape is a value body, not an ability, so it is measured from
// the face's SVar text (the compiled form of a `Number$.../Pow.` operand).
func paramFixCarriers(reg *cards.Registry) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	add := func(label, card string) {
		if label == "" || card == "" {
			return
		}
		if out[label] == nil {
			out[label] = map[string]bool{}
		}
		out[label][card] = true
	}
	// matchOne records every shape the compiled SA carries. An exact API
	// comparison, so AnimateAll never matches Animate and PutCounterAll never
	// matches PutCounter.
	matchOne := func(card string, sa *cards.SA) {
		for _, s := range paramFixShapes {
			if sa.API != s.api {
				continue
			}
			v, ok := sa.Params[s.key]
			if !ok {
				continue
			}
			if s.value != "" && !strings.Contains(v, s.value) {
				continue
			}
			add(s.label, card)
		}
	}
	// matchChain walks an SA and its SubAbility$ chain (like Primitives).
	matchChain := func(card string, head *cards.SA) {
		for sa := head; sa != nil; sa = sa.Sub {
			matchOne(card, sa)
		}
	}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if f == nil {
				continue
			}
			for _, a := range f.Abilities {
				matchChain(f.Name, a)
			}
			for _, tr := range f.Triggers {
				matchChain(f.Name, tr.Effect)
			}
			for _, r := range f.Repls {
				matchChain(f.Name, r.With)
			}
			f.EachSVarAbility(func(sa *cards.SA) { matchChain(f.Name, sa) })
			f.EachRawEffectChild(func(ch cards.EffectChild) {
				if ch.Trigger != nil {
					matchChain(f.Name, ch.Trigger.Effect)
				}
			})
			// The Number$/Pow. operand is a value body, reachable through the
			// face's SVar text rather than through an ability chain.
			svar := false
			for _, body := range f.SVars {
				if strings.Contains(body, "/Pow.") {
					svar = true
				}
			}
			if svar {
				add(paramFixLabelNumberPow, f.Name)
			}
		}
	}
	return out
}

// checkFixedParamCensus compares the measured carrier set for one label against the
// per-card table in both directions, with a loud precondition that the walk
// reached the class and a named card is a measured member. A registry that
// failed to load, or a walk whose Sub/SVar traversal stopped, must FAIL rather
// than pass vacuously.
func checkFixedParamCensus(t *testing.T, label, dir, mustCarry string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	got := paramFixCarriers(reg)[label]
	want := loadCensusLabels(t, dir)

	if len(got) == 0 {
		t.Fatalf("%s: census found no carriers at all; the corpus walk is broken", label)
	}
	if mustCarry != "" && !got[mustCarry] {
		t.Fatalf("%s: precondition: %q is not a measured carrier, got %v", label, mustCarry, sortedSetKeys(got))
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
		t.Errorf("%s carrier cards changed:\n got: %v\nwant: %v", label, gotNames, wantNames)
	}
	// Every table file must name exactly this label (a file carrying the wrong
	// label is a copy-paste defect the set comparison cannot see).
	for name, labels := range want {
		if len(labels) != 1 || labels[0] != label {
			t.Errorf("%s: %s labels = %v, want [%s]", label, name, labels, label)
		}
	}
}

// TestFixedParamCensusFilesWellFormed holds the eight census directories to the
// per-card schema without a corpus, so a malformed table file fails even where
// .cards is absent.
func TestFixedParamCensusFilesWellFormed(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{
		"corpus-param-fix-pump-replacedying",
		"corpus-param-fix-changezone-thisdefinedandtgts",
		"corpus-param-fix-diguntil-mintotalcmc",
		"corpus-param-fix-copypermanent-definedname",
		"corpus-param-fix-putcounter-countertypes",
		"corpus-param-fix-animate-chosencolor",
		"corpus-param-fix-choosetype-validinvalidtypes",
		"corpus-param-fix-number-pow",
	} {
		if got := loadCensusLabels(t, dir); len(got) == 0 {
			t.Errorf("%s: no per-card files; the table would compare vacuous", dir)
		}
	}
}

// TestFixedParamCensusPump pins Pump's ReplaceDyingDefined$ carriers.
func TestFixedParamCensusPump(t *testing.T) {
	checkFixedParamCensus(t, paramFixLabelPumpReplaceDying, "corpus-param-fix-pump-replacedying", "Gnashing of Teeth")
}

// TestFixedParamCensusChangeZone pins ChangeZone's ThisDefinedAndTgts$ carriers.
func TestFixedParamCensusChangeZone(t *testing.T) {
	checkFixedParamCensus(t, paramFixLabelChangeZoneThisAndTgts, "corpus-param-fix-changezone-thisdefinedandtgts", "Mangara of Corondor")
}

// TestFixedParamCensusDigUntil pins DigUntil's MinTotalCMC$ carriers.
func TestFixedParamCensusDigUntil(t *testing.T) {
	checkFixedParamCensus(t, paramFixLabelDigUntilMinTotalCMC, "corpus-param-fix-diguntil-mintotalcmc", "Dream Harvest")
}

// TestFixedParamCensusCopyPermanent pins CopyPermanent's DefinedName$ carriers.
func TestFixedParamCensusCopyPermanent(t *testing.T) {
	checkFixedParamCensus(t, paramFixLabelCopyPermanentDefName, "corpus-param-fix-copypermanent-definedname", "Mutable Explorer")
}

// TestFixedParamCensusPutCounter pins PutCounter's CounterTypes$ carriers.
func TestFixedParamCensusPutCounter(t *testing.T) {
	checkFixedParamCensus(t, paramFixLabelPutCounterTypes, "corpus-param-fix-putcounter-countertypes", "Abigale, Eloquent First-Year")
}

// TestFixedParamCensusAnimate pins the Animate Colors$ ChosenColor carriers.
func TestFixedParamCensusAnimate(t *testing.T) {
	checkFixedParamCensus(t, paramFixLabelAnimateColors, "corpus-param-fix-animate-chosencolor", "Puca's Eye")
}

// TestFixedParamCensusChooseType pins the ChooseType ValidTypes$/InvalidTypes$
// carriers (both spellings share one table and one label).
func TestFixedParamCensusChooseType(t *testing.T) {
	checkFixedParamCensus(t, paramFixLabelChooseTypeTypes, "corpus-param-fix-choosetype-validinvalidtypes", "Dawn-Blessed Pennant")
}

// TestFixedParamCensusNumberPow pins the Number$/Pow. carriers.
func TestFixedParamCensusNumberPow(t *testing.T) {
	checkFixedParamCensus(t, paramFixLabelNumberPow, "corpus-param-fix-number-pow", "Mathemagics")
}

// paramFixDirForLabel maps a label back to its table directory so
// TestFixedParamCensusAllShapesMeasured can hold the shape list and the
// directories in sync.
var paramFixDirForLabel = map[string]string{
	paramFixLabelPumpReplaceDying:      "corpus-param-fix-pump-replacedying",
	paramFixLabelChangeZoneThisAndTgts: "corpus-param-fix-changezone-thisdefinedandtgts",
	paramFixLabelDigUntilMinTotalCMC:   "corpus-param-fix-diguntil-mintotalcmc",
	paramFixLabelCopyPermanentDefName:  "corpus-param-fix-copypermanent-definedname",
	paramFixLabelPutCounterTypes:       "corpus-param-fix-putcounter-countertypes",
	paramFixLabelAnimateColors:         "corpus-param-fix-animate-chosencolor",
	paramFixLabelChooseTypeTypes:       "corpus-param-fix-choosetype-validinvalidtypes",
	paramFixLabelNumberPow:             "corpus-param-fix-number-pow",
}

// TestFixedParamCensusAllShapesMeasured is the self-check that every registered
// shape has a table directory and a measured member: a shape added to
// paramFixShapes with no directory, or one whose walk silently measured nothing,
// fails here rather than being quietly untested.
func TestFixedParamCensusAllShapesMeasured(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	got := paramFixCarriers(reg)
	for _, s := range paramFixShapes {
		dir, ok := paramFixDirForLabel[s.label]
		if !ok {
			t.Errorf("shape %s has no census directory mapping", s.label)
			continue
		}
		if filepath.Base(dir) == "" {
			t.Errorf("shape %s maps to an empty directory", s.label)
		}
		if len(got[s.label]) == 0 {
			t.Errorf("shape %s (%s.%s) measured no carriers", s.label, s.api, s.key)
		}
	}
}
