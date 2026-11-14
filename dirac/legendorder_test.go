package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLegendLosersEntryOrder: the non-kept legends reach the graveyard in
// battlefield (entry) order, as gorge's applyLegendBatch walks its group,
// not in slot order. A freed slot reused by a later arrival makes the two
// orders differ.
func TestLegendLosersEntryOrder(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Giada, Font of Hope", Count: 3}, {Name: "Llanowar Elves", Count: 1}, {Name: "Plains", Count: 8}})
	giada := id["Giada, Font of Hope"]
	// a filler holds slot 0 and leaves, so the newest legend reuses it
	filler := sc.perm(id["Llanowar Elves"], 0)
	a := sc.perm(giada, 0)
	c := sc.perm(giada, 0)
	sc.s.removePerm(sc.k, sc.slot(filler), kop.ZoneGraveyard, false)
	d := sc.perm(giada, 0)
	if sc.slot(d) >= sc.slot(a) {
		t.Fatalf("setup: newest legend slot %d is not below the oldest's %d", sc.slot(d), sc.slot(a))
	}
	if !sc.s.sba(sc.k) || sc.s.ctl != ctlLegend {
		t.Fatalf("no legend ask (ctl %d)", sc.s.ctl)
	}
	sc.s.Apply(MakeAction(AkChoose, TCode(c), 0, 0))
	if !sc.onBF(c) || sc.onBF(a) || sc.onBF(d) {
		t.Fatalf("keep c: a %v c %v d %v on the battlefield", sc.onBF(a), sc.onBF(c), sc.onBF(d))
	}
	pl := &sc.s.pl[0]
	var seq []Obj
	for i := 0; i < int(pl.graveN); i++ {
		seq = append(seq, pl.grave[i])
	}
	if len(seq) < 2 || seq[len(seq)-2] != a || seq[len(seq)-1] != d {
		t.Fatalf("graveyard %v, want ... %d (older) then %d (newer)", seq, a, d)
	}
}
