package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A may-play alternative's evidence beside the spell's own mandatory
// CollectEvidence part (dirac/cost_collectevidence.go buildEvidenceN): gorge
// evidenceAmount sums every CollectEvidence part of the pending cast into one
// ask (engine/rules/cast_targets.go), so a cast of a CollectEvidence<X> spell
// (kop.EvidenceTargetsMV; Urgent Necropsy) at Conspiracy Unraveler's
// alternative owes the targets' mana value AND the static's amount, paid from
// one pick; a graveyard that cannot reach the sum reverses the cast (gorge
// evidenceAsk's abort, CR 733.1). mayPlayHandAltTable provides the evidence
// permission (cast_mayplayhandalt_test.go).
func evidenceTargetsTable() (tbl *kop.Table, permC, necropsy CardID) {
	tbl, _, _, _, permC = mayPlayHandAltTable()
	tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	filter := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	creatureF := filter(kop.Filter{Types: kop.TCreature})
	necropsy = CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Necropsy", Cost: kop.ManaCost{Generic: 2},
		Types: kop.TInstant, Colors: kop.ColB | kop.ColG, Timing: kop.TimingInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: creatureF}},
			NTargets: 1, Evidence: kop.EvidenceTargetsMV,
			Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}})
	return tbl, permC, necropsy
}

// evidenceTargetsScen seats the evidence permission under seat 0 with two
// Islands, the opponent's bear (the target), the spell in seat 0's hand and
// n Think Twice (mana value 2 each) in its graveyard.
func evidenceTargetsScen(t *testing.T, tbl *kop.Table, permC, necropsy CardID, graveN int) (sc *scen, bears, card Obj, grave []Obj) {
	sc = newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(permC, 0)
	sc.perm(v.Island, 0)
	sc.perm(v.Island, 0)
	bears = sc.perm(v.GrizzlyBears, 1)
	card = sc.hand(necropsy, 0)
	grave = sc.grave(v.ThinkTwice, 0, graveN)
	return sc, bears, card, grave
}

// TestMayPlayEvidenceTargetsAltCast casts the targets'-mana-value evidence
// spell at the evidence alternative: the pick must reach the SUM (the bear's
// mana value + the static's 10) before Done is offered, a graveyard short of
// the sum reverses the cast (the permission's turn comes back), and a
// reaching one is exiled whole -- no mana, no life, no energy.
func TestMayPlayEvidenceTargetsAltCast(t *testing.T) {
	tbl, permC, necropsy := evidenceTargetsTable()
	bmv := int(v.Table.Cards[v.GrizzlyBears].Cost.MV())
	need := bmv + 10
	if bmv != 2 || int(v.Table.Cards[v.ThinkTwice].Cost.MV()) != 2 {
		t.Fatalf("precondition: bear mana value %d, Think Twice %d, want 2 and 2", bmv, v.Table.Cards[v.ThinkTwice].Cost.MV())
	}

	// a graveyard worth 10 (>= the static's evidence, < the sum) cannot pay:
	// the cast is reversed with nothing exiled and the alternative's turn back
	sc, _, card, _ := evidenceTargetsScen(t, tbl, permC, necropsy, 5)
	if _, alt := handAltOpts(t, sc, card); !alt {
		t.Fatalf("precondition: at grave total 10 the evidence alternative is not offered (opts %v)", sc.s.Step().Options)
	}
	sc.do(t, AkCast, TCode(card), TCode(zeroAltTgt))
	d := sc.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the target ask", d.Kind)
	}
	sc.s.Apply(d.Options[0]) // the only creature: the bear
	for i := 0; ; i++ {
		if i == 40 {
			t.Fatal("the reversed cast never returned to priority")
		}
		d = sc.s.Step()
		if d.N == 0 || d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if z := sc.s.objs[card].zone; z != kop.ZoneHand {
		t.Fatalf("the spell owes evidence %d and the graveyard reaches 10: it reversed, but zone is %d, want hand", need, z)
	}
	if n := int(sc.s.pl[0].graveN); n != 5 {
		t.Fatalf("the graveyard holds %d cards after the reversal, want 5", n)
	}

	// a graveyard worth the sum pays from one pick: the spell reaches the
	// stack with its whole graveyard exiled
	sc, bears, card, _ := evidenceTargetsScen(t, tbl, permC, necropsy, 6)
	if plain, alt := handAltOpts(t, sc, card); !plain || !alt {
		t.Fatalf("precondition: at grave total %d plain %v alt %v, want both", 12, plain, alt)
	}
	sc.do(t, AkCast, TCode(card), TCode(zeroAltTgt))
	d = sc.s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the target ask", d.Kind)
	}
	sc.s.Apply(d.Options[0])
	for i := 0; i < 24; i++ {
		d = sc.s.Step()
		if d.N == 0 || d.Kind == DkPriority {
			break
		}
		if d.Kind == DkChoose && d.Sub == ChCostEvidence {
			// the combined pick: take Done once the picked total reaches the
			// sum, else the first grave card
			done := -1
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Kind() == AkDone {
					done = j
				}
			}
			if done >= 0 {
				sc.s.Apply(d.Options[done])
			} else {
				sc.s.Apply(d.Options[0])
			}
			continue
		}
		sc.s.Apply(d.Options[0])
	}
	if z := sc.s.objs[card].zone; z != kop.ZoneStack {
		t.Fatalf("the evidence cast did not reach the stack (zone %d)", z)
	}
	if n := int(sc.s.pl[0].graveN); n != 0 {
		t.Fatalf("the graveyard holds %d cards after the payment, want none", n)
	}
	if z := sc.s.objs[bears].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the target bear is in zone %d, want the battlefield (the spell has not resolved)", z)
	}
	if sc.s.pl[0].life != 20 || sc.s.g.energy[0] != 0 {
		t.Fatalf("life %d energy %d after the cast, want 20 and 0", sc.s.pl[0].life, sc.s.g.energy[0])
	}
}
