package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMinTargetSurvivorCensusAppliesSetConstraints: the witness survivor
// census judges the survivors the way a full census's constraint capacities
// do (gorge's targetChoiceFeasible over survivorsCanTarget's survivors,
// engine/rules/legal_targets.go), not by their count. A payment plan that
// consumes one candidate of a Min-2 slot leaves two survivors whose count
// passes while the slot's cross-target constraint refuses: "two target
// creatures that share a controller" beside a plan consuming a candidate
// leaves the two other creatures under DIFFERENT controllers -- the count is
// two, but no one seat supplies two, so gorge refuses the cast and the census
// must too. The capacities, hasTargetsMin's tail:
//
//   - TgfSameController: one seat supplies the survivors;
//   - TgfDiffController: the survivors come from that many seats;
//   - TgfSharedCrType: one creature type is shared that often.
func TestMinTargetSurvivorCensusAppliesSetConstraints(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Subtypes = append([]string(nil), v.Table.Subtypes...)
	tbl.Conds = []kop.Cond{} // a generated table (prepareGen's gate)
	filter := func(f kop.Filter) kop.FilterID {
		tbl.Filters = append(tbl.Filters, f)
		return kop.FilterID(len(tbl.Filters) - 1)
	}
	sub := func(name string) uint8 {
		for i, n := range tbl.Subtypes {
			if n == name {
				return uint8(i)
			}
		}
		tbl.Subtypes = append(tbl.Subtypes, name)
		return uint8(len(tbl.Subtypes) - 1)
	}
	bearT, wolfT := sub("Berserker"), sub("Wolf")
	creatureF := filter(kop.Filter{Types: kop.TCreature})
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Probe Bear", Types: kop.TCreature,
		Power: 2, Toughness: 2, Subtypes: kop.SubMask{}.With(int(bearT))})
	wolf := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Probe Wolf", Types: kop.TCreature,
		Power: 2, Toughness: 2, Subtypes: kop.SubMask{}.With(int(wolfT))})

	sc := newScenTableLib(t, &tbl, v.Plains, Config{})
	x := sc.perm(bear, 0) // the candidate the payment plan consumes
	sc.perm(bear, 0)
	z := sc.perm(wolf, 1)

	// The payment fork that consumed x: the survivors are y (seat 0) and z
	// (seat 1). And the one that consumed z: the survivors are both seat 0's.
	fkA := sc.s.fork()
	fkA.place(x, kop.ZoneGraveyard, false)
	fkB := sc.s.fork()
	fkB.place(z, kop.ZoneGraveyard, false)
	defer sc.s.unfork()
	defer sc.s.unfork()

	for _, tc := range []struct {
		name  string
		flags kop.TargetFlags
		fk    *State
		want  bool
	}{
		{"same-controller survivors under two seats are no declaration", kop.TgfSameController, fkA, false},
		{"same-controller survivors under one seat are", kop.TgfSameController, fkB, true},
		{"different-controller survivors from two seats are", kop.TgfDiffController, fkA, true},
		{"different-controller survivors from one seat are not", kop.TgfDiffController, fkB, false},
		{"survivors sharing no creature type are no declaration", kop.TgfSharedCrType, fkA, false},
		{"survivors of one creature type are", kop.TgfSharedCrType, fkB, true},
	} {
		sp := &kop.TargetSpec{Kind: kop.TgtObject, Filter: creatureF, Min: 2, Max: 2, TFlags: tc.flags}
		if got := sc.s.targetsKept(sc.k, 0, sp, tc.fk, NoObj); got != tc.want {
			t.Errorf("%s: targetsKept=%v, want %v", tc.name, got, tc.want)
		}
	}
}
