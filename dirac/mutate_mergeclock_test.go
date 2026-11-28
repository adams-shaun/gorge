package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The merge clocks (coldMergeTS) are what orders a copy against a merge
// (CR 613.2a, 730.2a). Only a copy that can land on a permanent already on the
// battlefield needs them, so only its table pays for them.

// addLaterCopyOp adds a card whose program carries x, the op a real table's
// copy of a permanent already on the battlefield comes from.
func addLaterCopyOp(tbl *kop.Table, x kop.ExtOp) {
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Later Copier", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, Prog: []kop.Instr{kop.ExtInstr(x)}}}})
}

func TestMergeClocksOnlyWithALaterCopy(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   kop.ExtOp
		want int
	}{
		{"no later copy", kop.ExtNone, 0},
		{"clone copy", kop.ExtCloneCopy, maxMerged},
		{"aegis copy", kop.ExtAegisClone, maxMerged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl, _, _, _, _ := mutateTable()
			if tc.op != kop.ExtNone {
				addLaterCopyOp(tbl, tc.op)
			}
			sc := newScenTable(t, tbl, Config{NoElide: true})
			if got := len(coldMerge.of(sc.s)); got != maxMerged {
				t.Fatalf("precondition: %d pile records, want %d", got, maxMerged)
			}
			if got := len(coldMergeTS.of(sc.s)); got != tc.want {
				t.Fatalf("%d merge clocks, want %d", got, tc.want)
			}
		})
	}
}

// An Aegis copy newer than the merge takes the under card's keywords off the
// pile, in a table whose only later copy is the Aegis one (CR 707.2, 613.2a).
func TestAegisCopyNewerThanMergeDropsUnderKeywords(t *testing.T) {
	tbl, mutator, bird, _, _ := mutateTable()
	tbl.Cards[mutator].Keywords = kop.KwReach
	tbl.Cards[bird].Keywords = kop.KwFlying
	addLaterCopyOp(tbl, kop.ExtAegisClone)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, v.Forest, 1)
	host := sc.perm(bird, 0)
	mutateCast(t, sc, sc.hand(mutator, 0), host, false)
	resolveStack(t, sc)
	sl := sc.slot(host)
	if got := sc.s.kwGen(sc.k, sl); got != kop.KwFlying|kop.KwReach {
		t.Fatalf("precondition: the pile's keywords %#x, want flying and reach", got)
	}
	addAegisLayer(t, sc, sl, v.HillGiant, v.SerraAngel, sc.s.tick())
	setCopy(sc, sl, v.SerraAngel)
	if top := sc.s.copyTop(sl, NoSlot); top.kind != copyAegis {
		t.Fatalf("precondition: the newest copy layer is %v, want the Aegis one", top.kind)
	}
	want := tbl.Cards[v.SerraAngel].Keywords
	if want&kop.KwReach != 0 {
		t.Fatalf("precondition: the copied card prints reach itself")
	}
	if got := sc.s.kwGen(sc.k, sl); got != want {
		t.Fatalf("keywords under the Aegis copy %#x, want the copied card's %#x", got, want)
	}
}
