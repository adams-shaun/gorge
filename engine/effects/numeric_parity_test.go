package effects

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNumericParityPredicates pins Forge's four parity card properties
// (cmcEven, cmcOdd, powerEven, powerOdd) and the CR 202.3 facts they read:
// zero is even (CR 202.3a, Void Winnower's reminder text), a spell's chosen
// X counts on the stack (CR 202.3e: SpecContext.ManaValue), and parity is
// the mathematical one for a negative power (-1 is odd).
func TestNumericParityPredicates(t *testing.T) {
	zero := objFrom(t, "Name:Zero\nManaCost:0\nTypes:Artifact Creature\nPT:0/2\nOracle:x\n")
	two := objFrom(t, "Name:Two\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	one := objFrom(t, "Name:One\nManaCost:G\nTypes:Creature\nPT:1/1\nOracle:x\n")
	xSpell := objFrom(t, "Name:XG\nManaCost:X G\nTypes:Sorcery\nOracle:x\n")
	land := objFrom(t, "Name:Land\nManaCost:no cost\nTypes:Land\nOracle:x\n")
	sorcery := objFrom(t, "Name:Sorc\nManaCost:1 R\nTypes:Sorcery\nOracle:x\n")
	sc := NewSpecContext(0, 0)
	stackX := func(mv int32) SpecContext { c := sc; c.HasManaValue, c.ManaValue = true, mv; return c }
	pt := func(p int32) SpecContext {
		c := sc
		c.HasDerivedPT, c.DerivedPower, c.DerivedToughness = true, p, 1
		return c
	}
	for _, tc := range []struct {
		name string
		pred string
		o    *state.Object
		sc   SpecContext
		want bool
	}{
		{"mana value 0 is even", "cmcEven", zero, sc, true},
		{"mana value 0 is not odd", "cmcOdd", zero, sc, false},
		{"no-cost land has mana value 0, even", "cmcEven", land, sc, true},
		{"mana value 2 is even", "cmcEven", two, sc, true},
		{"mana value 1 is not even", "cmcEven", one, sc, false},
		{"mana value 1 is odd", "cmcOdd", one, sc, true},
		{"{X}{G} in hand is mana value 1 (X=0)", "cmcOdd", xSpell, sc, true},
		{"{X}{G} with X=1 on the stack is mana value 2", "cmcEven", xSpell, stackX(2), true},
		{"chosen X giving mana value 0 on the stack is even", "cmcEven", xSpell, stackX(0), true},
		{"power 2 is even", "powerEven", two, sc, true},
		{"power 1 is odd", "powerOdd", one, sc, true},
		{"power 0 is even", "powerEven", zero, sc, true},
		{"derived power 3 is odd", "powerOdd", two, pt(3), true},
		{"derived power -1 is odd", "powerOdd", two, pt(-1), true},
		{"derived power -1 is not even", "powerEven", two, pt(-1), false},
		{"a P/T-less sorcery has no even power", "powerEven", sorcery, sc, false},
	} {
		got, ok := numericPred(tc.pred, nil, tc.o, tc.sc)
		if !ok || got != tc.want {
			t.Errorf("%s: numericPred(%q) = (%v, %v), want (%v, true)", tc.name, tc.pred, got, ok, tc.want)
		}
	}
	for _, spec := range []string{"Card.cmcEven", "Card.cmcOdd", "Creature.powerEven", "Creature.powerOdd", "Creature.OppCtrl+cmcEven"} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want none", spec, un)
		}
	}
	if SpecNeedsResolver("Card.cmcEven") || SpecNeedsResolver("Creature.powerOdd") {
		t.Error("a parity predicate has no right-hand side and needs no resolver")
	}
}

// TestCmcNotChosenEvenOdd is Forge's negation of cmcChosenEvenOdd
// (Ashling's Prerogative: "each creature without mana value of the chosen
// quality"). With no quality chosen neither predicate matches.
func TestCmcNotChosenEvenOdd(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	two := objFrom(t, "Name:Two\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	one := objFrom(t, "Name:One\nManaCost:G\nTypes:Creature\nPT:1/1\nOracle:x\n")
	chosen := func(q string) SpecContext {
		c := NewSpecContext(0, 0)
		c.SourceLKIValid, c.SourceChosenType = true, q
		return c
	}
	for _, tc := range []struct {
		pred, quality string
		o             *state.Object
		want          bool
	}{
		{"cmcNotChosenEvenOdd", "odd", two, true},
		{"cmcNotChosenEvenOdd", "odd", one, false},
		{"cmcNotChosenEvenOdd", "even", one, true},
		{"cmcNotChosenEvenOdd", "", one, false},
		{"cmcChosenEvenOdd", "odd", one, true},
		{"cmcChosenEvenOdd", "", one, false},
	} {
		got, ok := numericPred(tc.pred, g, tc.o, chosen(tc.quality))
		if !ok || got != tc.want {
			t.Errorf("%s chosen=%q on %s: (%v, %v), want (%v, true)", tc.pred, tc.quality, tc.o.Face().Name, got, ok, tc.want)
		}
	}
	if un := UnknownPredicates("Creature.cmcNotChosenEvenOdd"); len(un) != 0 {
		t.Errorf("UnknownPredicates(cmcNotChosenEvenOdd) = %v, want none", un)
	}
}

// TestNumericComparatorNEAndM2 pins the two Forge comparators beyond the
// five ordered ones (Forge's Expressions.compare): NE is "not equal"
// (Haktos's cmcNEChosenNumber) and M2 is "equal modulo 2" (Obosh's cmcM21,
// Gyruda's cmcM20).
func TestNumericComparatorNEAndM2(t *testing.T) {
	two := objFrom(t, "Name:Two\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	three := objFrom(t, "Name:Three\nManaCost:2 G\nTypes:Creature\nPT:3/3\nOracle:x\n")
	sc := NewSpecContext(0, 0)
	for _, tc := range []struct {
		pred string
		o    *state.Object
		want bool
	}{
		{"cmcNE2", two, false},
		{"cmcNE2", three, true},
		{"cmcM20", two, true},
		{"cmcM20", three, false},
		{"cmcM21", three, true},
		{"cmcM21", two, false},
		{"powerM21", three, true},
		{"counters_NE0_P1P1", two, false},
		{"classLevel_NE1", two, false},
	} {
		got, ok := numericPred(tc.pred, nil, tc.o, sc)
		if !ok || got != tc.want {
			t.Errorf("numericPred(%q) on %s = (%v, %v), want (%v, true)", tc.pred, tc.o.Face().Name, got, ok, tc.want)
		}
	}
	for _, spec := range []string{"Card.cmcM20", "Card.YouCtrl+cmcM21", "Card.cmcNEChosenNumber"} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want none", spec, un)
		}
	}
}

// TestNumericPredicateUnknownComparatorFailsClosed: an unrecognised
// comparator is "not recognised" (ok=false) whatever its right-hand side --
// literal, resolvable or not -- and whatever object it is probed with, so
// UnknownPredicates reports it and the filter fails closed. Before cr-g03 an
// unresolvable RHS or a faceless probe object answered (false, recognised)
// first, which is how cmcEven read as understood.
func TestNumericPredicateUnknownComparatorFailsClosed(t *testing.T) {
	two := objFrom(t, "Name:Two\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	resolveAll := NewSpecContext(0, 0)
	resolveAll.Resolve = func(string) (int32, bool) { return 2, true }
	for _, tok := range []string{"cmcZZ3", "cmcZZX", "powerZZX", "toughnessQQ1", "basePowerZZ1", "cmcEvenX", "counters_ZZ1_P1P1", "counters_ZZX_P1P1", "classLevel_ZZ2"} {
		for _, probe := range []struct {
			name string
			o    *state.Object
			sc   SpecContext
		}{
			{"census probe", &state.Object{}, NewSpecContext(0, 0)},
			{"real object, no resolver", two, NewSpecContext(0, 0)},
			{"real object, resolver", two, resolveAll},
		} {
			if got, ok := numericPred(tok, nil, probe.o, probe.sc); ok {
				t.Errorf("%s: numericPred(%q) = (%v, true), want ok=false (unknown comparator)", probe.name, tok, got)
			}
		}
		un := UnknownPredicates("Card." + tok)
		if len(un) != 1 || un[0] != tok {
			t.Errorf("UnknownPredicates(Card.%s) = %v, want [%s]", tok, un, tok)
		}
	}
}

// numericCensusToken finds a filter-position token (after '.', '+' or '!')
// in numericPred's field vocabulary anywhere in a corpus script.
var numericCensusToken = regexp.MustCompile(`[.+!]((?:power|toughness|cmc|basePower|baseToughness|totalPT|counters_|classLevel_)[A-Za-z0-9_]*)`)

// numericCensusNeedsGame are the recognised tokens whose truth reads the
// spec source's recorded choice, which the object sweep below does not
// bind; TestCmcNotChosenEvenOdd covers them.
var numericCensusNeedsGame = map[string]bool{"cmcChosenEvenOdd": true, "cmcNotChosenEvenOdd": true}

// numericCensusValues is the sweep's characteristic range: wide enough for
// every corpus threshold (powerLT0, counters_GE1000_TIME) and both parities.
var numericCensusValues = [...]int32{-1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 20, 21, 100, 1000}

// numericCensusNotFilters are corpus tokens the regex finds that are not
// filter predicates at all (a bare field word in an AILogic$ value).
var numericCensusNotFilters = map[string]bool{"cmc": true, "power": true, "toughness": true}

// TestNumericPredicateCorpusCensus is the class ratchet for numericPred's
// grammar (cr-g03). Over every distinct numeric-field filter token in the
// corpus it asserts:
//
//  1. recognition does not depend on the object or the resolver: the
//     UnknownPredicates probe (faceless object, no resolver) and a real
//     evaluation agree on ok, so the census can never call a shape
//     understood that the matcher cannot evaluate;
//  2. every recognised token is live: some object in a mana-value /
//     power / toughness / counter / class-level / RHS sweep matches it, so
//     no recognised shape is a constant-false no-op (the cmcEven bug);
//  3. no corpus numeric-field token is unrecognised.
func TestNumericPredicateCorpusCensus(t *testing.T) {
	dir := cards.CorpusDir(testutil.CorpusDirOrSkip(t))
	tokens := map[string]bool{}
	if err := filepath.WalkDir(filepath.Dir(dir), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "Oracle:") || strings.HasPrefix(line, "Name:") {
				continue
			}
			for _, m := range numericCensusToken.FindAllStringSubmatch(line, -1) {
				tokens[m[1]] = true
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(tokens) < 100 {
		t.Fatalf("census found only %d numeric tokens; the corpus walk is broken", len(tokens))
	}
	creature := objFrom(t, "Name:Probe\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n")
	var unknown, inconsistent, dead []string
	for tok := range tokens {
		if numericCensusNotFilters[tok] {
			continue
		}
		_, probeOK := numericPred(tok, nil, &state.Object{}, NewSpecContext(0, 0))
		if !probeOK {
			unknown = append(unknown, tok)
			continue
		}
		if numericCensusNeedsGame[tok] {
			continue
		}
		live := false
	sweep:
		for _, k := range numericCensusValues {
			for _, j := range numericCensusValues {
				for _, r := range [...]int32{0, 1, 2, 5} {
					o := *creature
					o.Counters = nil
					if _, kind, ok := strings.Cut(strings.TrimPrefix(tok, "counters_"), "_"); ok && strings.HasPrefix(tok, "counters_") {
						o.Counters = []state.Counter{{Kind: kind, N: k}}
					}
					o.ClassLevelValue = uint8(k)
					sc := NewSpecContext(0, 0)
					sc.HasManaValue, sc.ManaValue = true, k
					sc.HasDerivedPT, sc.DerivedPower, sc.DerivedToughness = true, k, j
					sc.HasBasePT, sc.BasePower, sc.BaseToughness = true, j, k
					rv := r
					sc.Resolve = func(string) (int32, bool) { return rv, true }
					got, ok := numericPred(tok, nil, &o, sc)
					if !ok {
						inconsistent = append(inconsistent, tok)
						break sweep
					}
					if got {
						live = true
						break sweep
					}
				}
			}
		}
		if !live {
			dead = append(dead, tok)
		}
	}
	sort.Strings(unknown)
	sort.Strings(inconsistent)
	sort.Strings(dead)
	if len(inconsistent) != 0 {
		t.Errorf("numeric tokens the census probe recognises but a real evaluation does not (census and matcher disagree): %v", inconsistent)
	}
	if len(dead) != 0 {
		t.Errorf("recognised numeric tokens no swept object ever matches (silent constant-false): %v", dead)
	}
	if len(unknown) != 0 {
		t.Errorf("corpus numeric-field tokens numericPred does not recognise: %v", unknown)
	}
	t.Logf("numeric census: %d distinct tokens", len(tokens))
}
