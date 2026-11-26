package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGreatestCMCTightens: the greatestCMC_<set> property arm fails closed on
// the two shapes its lowering cannot express exactly (cli-20261125T191555Z-
// fdbcc0d7's round-2 finding; gorge engine/effects/filter.go's greatestCMC_
// branch). The corpus's spellings (Artifact.…_Artifact+YouCtrl,
// Creature.…_Creature, Padeem, Consul of Innovation) keep lowering.
func TestGreatestCMCTightens(t *testing.T) {
	known := func(string) bool { return false }

	// The candidate's head types and the set's type join as an any-of mask,
	// where gorge needs head types AND the set's membership: a head type
	// that differs from the set's refuses.
	f, err := parseFilter("Artifact.greatestCMC_Artifact", known, known)
	if err != nil {
		t.Fatalf("Artifact.greatestCMC_Artifact: %v", err)
	} else if f.Clauses[0].Types != tArtifact || f.Clauses[0].CmpStat != cmpCMC {
		t.Fatalf("Artifact.greatestCMC_Artifact clause = %+v", f.Clauses[0])
	}
	f, err = parseFilter("Creature.greatestCMC_Creature", known, known)
	if err != nil {
		t.Fatalf("Creature.greatestCMC_Creature: %v", err)
	}
	if f, err = parseFilter("Card.greatestCMC_Creature", known, known); err != nil {
		t.Fatalf("Card.greatestCMC_Creature: %v", err)
	} else if f.Clauses[0].Types != tCreature {
		t.Fatalf("Card.greatestCMC_Creature clause types = %#x, want the set's", f.Clauses[0].Types)
	}
	f, err = parseFilter("Artifact.greatestCMC_NonLandPermanent", known, known)
	if err != nil {
		t.Fatalf("Artifact.greatestCMC_NonLandPermanent: %v", err)
	} else if f.Clauses[0].NotTypes&tLand == 0 {
		t.Fatalf("Artifact.greatestCMC_NonLandPermanent clause = %+v, want the nonland mark", f.Clauses[0])
	}
	for _, p := range []string{"Creature.greatestCMC_Artifact", "Artifact.greatestCMC_Creature"} {
		if _, err := parseFilter(p, known, known); err == nil ||
			!strings.Contains(err.Error(), "beside a different head type") {
			t.Errorf("parseFilter(%q) err = %v, want the head-type refusal", p, err)
		}
	}

	// The bound is the greatest mana value over every battlefield, and gorge's
	// candidate read requires the candidate itself on the battlefield; a
	// clause compiled in another zone would match an off-battlefield card at
	// the tied maximum, so it refuses there.
	cb := &cardBuilder{pt: &poolTables{filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}}}
	set := &lFilter{Clauses: []lClause{{Types: tArtifact}}}
	greatest := lClause{Types: tArtifact, CmpStat: cmpCMC, CmpOp: opEQ,
		CmpV: &lValue{Kind: vValidAgg, N: kop.AggGreatestMV, F: set}}
	if _, err := cb.clause(greatest, kop.ZoneBattlefield); err != nil {
		t.Errorf("battlefield read: %v", err)
	}
	if _, err := cb.clause(greatest, kop.ZoneGraveyard); err == nil ||
		!strings.Contains(err.Error(), "greatestCMC beside a non-battlefield zone") {
		t.Errorf("graveyard read: err %v, want the named refusal", err)
	}
	if _, err := cb.clause(greatest, kop.ZoneStack); err == nil ||
		!strings.Contains(err.Error(), "greatestCMC beside a non-battlefield zone") {
		t.Errorf("stack read: err %v, want the named refusal", err)
	}

	// An ordinary CMC == value compare beside the same aggregate-free shape
	// still lowers in any zone: the guard keys on the bound, not the compare.
	plain := lClause{Types: tArtifact, CmpStat: cmpCMC, CmpOp: opEQ, CmpN: 3}
	if _, err := cb.clause(plain, kop.ZoneGraveyard); err != nil {
		t.Errorf("plain CMC compare in a graveyard: %v", err)
	}
}
