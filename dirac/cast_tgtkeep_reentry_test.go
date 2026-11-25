package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckMinTargetsKeptNoReentry pins the may-play offer's keep-targets read
// against re-entering itself (Lzeutschler/commander_ai issue #5: a fatal stack
// overflow in search). Rack and Ruin ("Destroy two target artifacts", {2}{R})
// is exiled under an ignore-colour permission (MayPlayIgnoreColor$, CR 609.4b)
// over an Island, Heap Gate and two Blood Pets (Daxos of Meletis puts a
// kind-rider permission on the table): the plain census pays the
// rider-folded {3} but not the printed {2}{R}, which only the Gate's
// "{1}, {T}: Add one mana of any color" gap funds beside a Blood Pet's
// sacrifice. The offer's keep-targets read for a Min-2 slot (minTargetsKept)
// plays gorge's prefix search on a fork, and the search's plain probe lists the
// may-play cast again, which read the same guard on the same board without end.
// gorge's script search is never nested (potential_script.go: a node's options
// are the plain priority list and potentialPlayVerdict, neither of which
// searches a prefix), so the nested read prices the plan where it stands.
func CheckMinTargetsKeptNoReentry(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	for _, n := range []string{"Island", "Heap Gate", "Blood Pet", "Rack and Ruin", "Ornithopter", "Daxos of Meletis"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: the table lacks %s", n)
		}
	}
	cfg := LockstepConfig(SurfaceFull)
	cfg.NoElide = true
	sc := newScenTableLib(t, tbl, id["Island"], cfg)
	sc.perm(id["Island"], 0)
	sc.perm(id["Heap Gate"], 0)
	sc.perm(id["Blood Pet"], 0)
	sc.perm(id["Blood Pet"], 0)
	sc.perm(id["Ornithopter"], 1)
	sc.perm(id["Ornithopter"], 1)
	s := sc.s
	o := sc.obj(id["Rack and Ruin"], 0)
	s.objs[o].zone = kop.ZoneExile
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), kind: kop.MayPlayIgnoreColor, turn: mayWord(s.turn)}
	s.g.nmay = 1
	d := s.Step()
	// the rider folds {2}{R} to {3}, which the Island, the Gate's {C} and a
	// Blood Pet pay, and no plan consumes an artifact: gorge offers the cast
	if !hasOption(&d, MakeAction(AkCast, TCode(o), 0, castMayPlay)) {
		t.Errorf("the ignore-colour may-play Rack and Ruin is not offered: options %v", d.Options[:d.N])
	}
}
