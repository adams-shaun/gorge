package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestIsCommanderWordOnBattlefield: the IsCommander filter word holds for the
// seat's commander permanent in a Commander game and for no other permanent;
// in a game without a commander it matches nothing.
func TestIsCommanderWordOnBattlefield(t *testing.T) {
	tbl := *v.Table
	tbl.Filters = append(append([]kop.Filter(nil), v.Table.Filters...),
		kop.Filter{Pred: kop.PredWord7, PredN: kop.W7IsCommander})
	fid := kop.FilterID(len(tbl.Filters) - 1)

	sc := newScenTableLib(t, &tbl, v.LegendaryBear, Config{
		NoElide: true, Commander: true, Commanders: [MaxSeats]int16{0, 0},
	})
	if sc.s.pl[0].cmd == NoObj {
		t.Fatal("Commander game seated no commander for seat 0")
	}
	cmd := sc.s.pl[0].cmd
	from := sc.s.detach(cmd)
	csl := sc.s.enter(sc.k, cmd, 0, false, from)
	sc.s.perms[csl].flags &^= pfSick
	other := sc.perm(v.LegendaryBear, 0)
	if !sc.onBF(cmd) || !sc.onBF(other) {
		t.Fatalf("commander on battlefield=%v, other on battlefield=%v", sc.onBF(cmd), sc.onBF(other))
	}
	matches := func(s *scen, o Obj) bool {
		return s.s.matchPermOne(s.k, &s.k.t.Filters[fid], s.slot(o), 0, NoSlot, layerFull)
	}
	if !matches(sc, cmd) {
		t.Error("Commander game: the IsCommander filter does not match the seat's commander permanent")
	}
	if matches(sc, other) {
		t.Error("Commander game: the IsCommander filter matches a legendary creature that is not the commander")
	}

	plain := newScenTableLib(t, &tbl, v.LegendaryBear, Config{NoElide: true})
	if plain.s.pl[0].cmd != NoObj {
		t.Fatalf("game without Commander seated a commander: %d", plain.s.pl[0].cmd)
	}
	legend := plain.perm(v.LegendaryBear, 0)
	if matches(plain, legend) {
		t.Error("game without Commander: the IsCommander filter matches a legendary creature")
	}
}
