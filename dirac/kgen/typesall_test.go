package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// typesall: a Forge clause naming two disjoint card types is gorge's type
// head AND type property (filter_word wordType), so the clause keeps the
// any-of union in Types and carries the conjunction in TypesAll, emitted
// as PredTypesAll; nested sets still narrow; a third single type joins the
// conjunction; a conjunction beside another predicate refuses (one Pred
// slot).
func TestTypesallFilterForms(t *testing.T) {
	known := func(s string) bool { return s == "Equipment" }
	for _, tc := range []struct {
		expr     string
		types    uint16
		typesAll uint16
	}{
		{"Artifact.Creature", tArtifact | tCreature, tArtifact | tCreature},
		{"Creature.Artifact+YouCtrl", tArtifact | tCreature, tArtifact | tCreature},
		{"Land.Artifact", tLand | tArtifact, tLand | tArtifact},
		{"Artifact.Creature+Land", tArtifact | tCreature | tLand, tArtifact | tCreature | tLand},
		{"Artifact.Creature+Permanent", tArtifact | tCreature, tArtifact | tCreature},
		{"Permanent.Creature", tCreature, 0},
		{"Card.Creature", tCreature, 0},
		{"Creature.Permanent", tCreature, 0},
		{"Artifact.Equipment", tArtifact, 0},
	} {
		f, err := parseFilter(tc.expr, known, known)
		if err != nil || len(f.Clauses) != 1 {
			t.Errorf("%s: %s, %v", tc.expr, f, err)
			continue
		}
		if c := f.Clauses[0]; c.Types != tc.types || c.TypesAll != tc.typesAll {
			t.Errorf("%s: types=%#x all=%#x, want types=%#x all=%#x", tc.expr, c.Types, c.TypesAll, tc.types, tc.typesAll)
		}
	}
	// a type set that does not contain the conjunction refuses, as
	// does a set head disjoint from a single property
	for _, bad := range []string{"Artifact.Instant+Permanent", "Instant.Permanent"} {
		if f, err := parseFilter(bad, known, known); err == nil {
			t.Errorf("%s: lowered to %s; want a refusal", bad, f)
		}
	}
	// the emitted filter: PredTypesAll with the conjunction as PredN
	cb := &cardBuilder{}
	kf, err := cb.clause(lClause{Types: tArtifact | tCreature, TypesAll: tArtifact | tCreature, Ctrl: whoYou}, kop.ZoneBattlefield)
	if err != nil || kf.Pred != kop.PredTypesAll || kop.TypeMask(uint16(kf.PredN)) != kop.TArtifact|kop.TCreature || kf.Types != kop.TArtifact|kop.TCreature {
		t.Errorf("Artifact.Creature emitted %+v, %v", kf, err)
	}
	// one Pred slot: a conjunction beside a comparison refuses by name
	if _, err := cb.clause(lClause{Types: tArtifact | tCreature, TypesAll: tArtifact | tCreature, CmpStat: cmpPower, CmpOp: opGE, CmpN: 2}, kop.ZoneBattlefield); err == nil || !strings.Contains(err.Error(), "one Pred slot") {
		t.Errorf("conjunction beside powerGE2: %v; want the one-Pred-slot refusal", err)
	}
}

// ktypesallCarriers: the corpus forms, lowered end to end (skips without
// .cards): the battlefield conjunction (Steel Overseer, Tempered Steel) and
// the library one (Sojourner's Companion's Land.Artifact typecycling).
var ktypesallCarriers = map[string]string{
	"Steel Overseer":        "PutCounterAll A=1 CS=P1P1 F={types=0x6 all=0x6 ctrl=1}",
	"Tempered Steel":        "static Continuous F={types=0x6 all=0x6 ctrl=1}",
	"Master of Etherium":    "static Continuous F={types=0x6 all=0x6 ctrl=1 Other}",
	"Sojourner's Companion": "Search obj=You A=1 C=2 F={types=0x5 all=0x5}",
}

func TestTypesallCarriers(t *testing.T) {
	var names []string
	for n := range ktypesallCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := ktypesallCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), want) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", c.Name, want, sb.String())
		}
	}
	if seen != len(ktypesallCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(ktypesallCarriers))
	}
}
