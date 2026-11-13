package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCastTargetsAll: Tf2TargetsFilter2 (gorge TargetsValid$) holds
// for a spell with at least one target in its whole chain that is a
// battlefield object matching the filter (Card.Self here: the heroic
// source); gorge made it any-match.
func TestCastTargetsAll(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Downwind Ambusher", Count: 2}, {Name: "Plains", Count: 8}})
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Flags: kop.FSelf})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	src, other := sc.perm(id["Downwind Ambusher"], 0), sc.perm(id["Downwind Ambusher"], 0)
	l := trigListener{obj: src, sl: sc.slot(src), ctrl: 0}
	var e stackEntry
	if sc.s.castTargetsAll(sc.k, f, &e, l) {
		t.Fatal("a spell with no target matched")
	}
	e.nt, e.tgt[0] = 1, uint8(src)
	if !sc.s.castTargetsAll(sc.k, f, &e, l) {
		t.Fatal("a spell targeting only the source did not match")
	}
	e.nt, e.tgt[1] = 2, uint8(other)
	if !sc.s.castTargetsAll(sc.k, f, &e, l) {
		t.Fatal("a spell also targeting another creature did not match")
	}
	e.tgt[0], e.tgt[1] = uint8(other), uint8(src)
	if !sc.s.castTargetsAll(sc.k, f, &e, l) {
		t.Fatal("the source in a later slot did not match")
	}
	e.nt, e.tgt[0] = 1, uint8(other)
	if sc.s.castTargetsAll(sc.k, f, &e, l) {
		t.Fatal("a spell targeting only another creature matched")
	}
	e.nt, e.tgt[0] = 1, uint8(PlayerTarget0)
	if sc.s.castTargetsAll(sc.k, f, &e, l) {
		t.Fatal("a player target matched")
	}
}
