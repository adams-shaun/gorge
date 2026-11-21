package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pairWhere: where the second Goblin (Bolg's Company's sacrifice fodder, and
// a hybrid {2}{B/R} cast) is.
type pairWhere uint8

const (
	pairAbsent pairWhere = iota
	pairInHand
	pairOnBattlefield
	numPairWhere
)

// equipBoard is one board of the sweep: seat 0's mountains and swamps (the
// first tappedLands of them tapped), Dwarven Mauler and Bolg's Company
// (0 untapped, 1 tapped, 2 summoning sick), Crude Bent Blade (Equip {2},
// which Dwarven Mauler reduces by {2} when it is the target) attached to the
// Mauler or not.
type equipBoard struct {
	mountains, swamps int
	landMask          uint
	mauler, bolg      uint8
	pair              pairWhere
	equipped          bool
}

// equipTable is the equipautopay table of the sweep, built once: the
// sweep opens a scenario per board and path, and the table is read-only.
type equipTable struct {
	tbl  *kop.Table
	id   map[string]CardID
	skip string
	err  error
}

// equipTableMemo is set by the sweep's first scenario; only
// TestCrudeBentBladeEquipAutopayPlans opens one, from one goroutine.
var equipTableMemo *equipTable

func equipTableOf(set cardset.Set) *equipTable {
	et := &equipTable{}
	names := []string{"Crude Bent Blade", "Dwarven Mauler", "Bolg's Company", "Fearsome Goblin Pair", "Mountain", "Swamp"}
	deck := []cardset.Entry{}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			et.skip = e.Name + " not supported in the equipautopay table"
			return et
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		et.err = err
		return et
	}
	et.tbl = tbl
	et.id = map[string]CardID{}
	for _, c := range da {
		et.id[tbl.Cards[c].Name] = c
	}
	return et
}

func equipAutopayScen(t *testing.T, cfg Config, b equipBoard) *scen {
	t.Helper()
	if equipTableMemo == nil {
		equipTableMemo = equipTableOf(cardsettest.Set(t, "equipautopay"))
	}
	et := equipTableMemo
	if et.err != nil {
		t.Fatal(et.err)
	}
	if et.skip != "" {
		t.Skip(et.skip)
	}
	tbl, id := et.tbl, et.id
	sc := newScenTableLib(t, tbl, id["Swamp"], cfg)
	set1 := func(o Obj, st uint8) Obj {
		switch st {
		case 1:
			sc.s.perms[sc.slot(o)].flags |= pfTapped
		case 2:
			sc.s.perms[sc.slot(o)].flags |= pfSick
		}
		return o
	}
	n := uint(0)
	for i := 0; i < b.mountains+b.swamps; i++ {
		name := "Mountain"
		if i >= b.mountains {
			name = "Swamp"
		}
		set1(sc.perm(id[name], 0), uint8(b.landMask>>n&1))
		n++
	}
	mauler := set1(sc.perm(id["Dwarven Mauler"], 0), b.mauler)
	set1(sc.perm(id["Bolg's Company"], 0), b.bolg)
	blade := sc.perm(id["Crude Bent Blade"], 0)
	if b.equipped {
		sc.s.perms[sc.slot(blade)].attached = sc.slot(mauler)
	}
	switch b.pair {
	case pairInHand:
		sc.hand(id["Fearsome Goblin Pair"], 0)
	case pairOnBattlefield:
		sc.perm(id["Fearsome Goblin Pair"], 0)
	}
	return sc
}

// equipWalk replays path (option indices, the first one at priority) on board
// b, then explores every option of each question the offer raises (a target,
// a cost pick) down to depth 4 and until priority returns. A panic is
// reported with the path.
func equipWalk(t *testing.T, cfg Config, b equipBoard, path []int, offers *int) {
	t.Helper()
	defer func() {
		if e := recover(); e != nil {
			t.Errorf("cfg %+v board %+v path %v: panicked: %v", cfg, b, path, e)
		}
	}()
	sc := equipAutopayScen(t, cfg, b)
	var d Decision
	for _, i := range path {
		d = sc.s.Step()
		sc.s.Apply(d.Options[i])
	}
	d = sc.s.Step()
	if len(path) > 0 && d.Kind == DkPriority || len(path) >= 4 {
		return
	}
	for i := 0; i < int(d.N); i++ {
		if len(path) == 0 {
			if k := d.Options[i].Kind(); k != AkActivate && k != AkCast {
				continue
			}
			*offers++
		}
		equipWalk(t, cfg, b, append(append([]int(nil), path...), i), offers)
	}
}

// TestCrudeBentBladeEquipAutopayPlans: every cast and activation offered on
// a board with Crude Bent Blade, Dwarven Mauler and Bolg's Company is
// payable, whichever target or cost pick follows (a panic "autopay found no
// plan" is a kernel defect, xengine farm RAND-c4-g19).
func TestCrudeBentBladeEquipAutopayPlans(t *testing.T) {
	t.Parallel()
	offers := 0
	cfgs := []Config{{NoElide: true}, {NoElide: true, GorgeLowering: true}, {NoElide: true, GorgeLowering: true, GorgeAutopaySurface: true}}
	for _, cfg := range cfgs {
		for m := 1; m <= 2; m++ {
			for sw := 1; sw <= 3; sw++ {
				for mask := uint(0); mask < 1<<uint(m+sw); mask++ {
					for ma := uint8(0); ma < 3; ma++ {
						for bo := uint8(0); bo < 3; bo++ {
							for pw := pairAbsent; pw < numPairWhere; pw++ {
								for eq := 0; eq < 2; eq++ {
									equipWalk(t, cfg, equipBoard{m, sw, mask, ma, bo, pw, eq == 1}, nil, &offers)
								}
							}
						}
					}
				}
			}
		}
	}
	t.Logf("%d offers walked", offers)
	if offers == 0 {
		t.Errorf("no cast or activation was offered: the test no longer exercises the offer")
	}
}
