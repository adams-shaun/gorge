package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Preparation (face_prepare.go) on the generated prepare pool: the
// DurPrepared record, the exile copy and its cast as gorge's prepared_copy.

func prepScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "prepare")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the prepare table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// preparedRecords returns the live DurPrepared records.
func preparedRecords(s *State) (out []mayPlay) {
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.g.may[i]; r.until == kop.DurPrepared {
			out = append(out, r)
		}
	}
	return out
}

// preparedCastOffers lists the prepared-copy casts offered in d.
func preparedCastOffers(s *State, d *Decision) (out []Obj) {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Mode()&castFace != 0 && a.Target() == 1 && s.objs[a.Source()].of&ofCopy != 0 {
			out = append(out, Obj(a.Source()))
		}
	}
	return out
}

// enterPrepared casts Adventurous Eater with three Swamps and resolves it.
func enterPrepared(t *testing.T, sc *scen, id map[string]CardID) Obj {
	t.Helper()
	sc.lands(0, id["Swamp"], 4)
	eater := sc.hand(id["Adventurous Eater"], 0)
	sc.castCard(t, eater)
	sc.buildToStack(t)
	sc.resolveAll(t)
	if !sc.onBF(eater) {
		t.Fatal("Adventurous Eater did not resolve")
	}
	return eater
}

// TestPrepareEntersPrepared: a creature that "enters prepared" (an Updated
// ETB replacement) has a DurPrepared record and its spell's copy at the end
// of its controller's exile: an ofCopy object showing face 1 (CR 722.3a, c).
func TestPrepareEntersPrepared(t *testing.T) {
	sc, id := prepScen(t, "Adventurous Eater", "Swamp")
	if !sc.k.gen.anyPrepare {
		t.Fatal("anyPrepare not set")
	}
	front := id["Adventurous Eater"]
	eater := enterPrepared(t, sc, id)
	s := sc.s
	rs := preparedRecords(s)
	if len(rs) != 1 || !s.prepared(eater) {
		t.Fatalf("records %v prepared %v", rs, s.prepared(eater))
	}
	cp := rs[0].obj
	ob := &s.objs[cp]
	if ob.of&ofCopy == 0 || ob.zone != kop.ZoneExile || ob.card != sc.k.t.Cards[front].Faces[1] || s.pl[0].exile[s.pl[0].exileN-1] != cp {
		t.Fatalf("copy %+v exile %v", *ob, s.pl[0].exile[:s.pl[0].exileN])
	}
	if rs[0].turn != uint16(eater)|uint16(s.objs[eater].ver)<<8 || rs[0].ver != ob.ver {
		t.Fatalf("record %+v", rs[0])
	}
}

// TestPrepareSecondGrantNoop: a second grant on a prepared permanent
// mints nothing (gorge's fold: !o.Prepared).
func TestPrepareSecondGrantNoop(t *testing.T) {
	sc, id := prepScen(t, "Adventurous Eater", "Swamp")
	eater := enterPrepared(t, sc, id)
	s := sc.s
	nobj, nmay, nex := s.nobj, s.g.nmay, s.pl[0].exileN
	s.prepare(sc.k, eater)
	if s.nobj != nobj || s.g.nmay != nmay || s.pl[0].exileN != nex {
		t.Fatalf("second grant minted: nobj %d->%d nmay %d->%d", nobj, s.nobj, nmay, s.g.nmay)
	}
}

// TestPrepareCastUnprepares: the copy is cast from exile for its printed
// cost; the push removes the record (unprepared), the copy resolves and
// ceases, and nothing is offered again (CR 722.3c).
func TestPrepareCastUnprepares(t *testing.T) {
	sc, id := prepScen(t, "Adventurous Eater", "Swamp")
	eater := enterPrepared(t, sc, id)
	s := sc.s
	cp := preparedRecords(s)[0].obj
	d := s.Step()
	if got := preparedCastOffers(s, &d); len(got) != 1 || got[0] != cp {
		t.Fatalf("offers %v, want [%d]", got, cp)
	}
	s.Apply(MakeAction(AkCast, TCode(cp), 1, castFace))
	sc.buildToStack(t)
	if len(preparedRecords(s)) != 0 || s.prepared(eater) {
		t.Fatal("the push kept the prepared designation")
	}
	if s.objs[cp].zone != kop.ZoneStack {
		t.Fatalf("copy zone %d, want stack", s.objs[cp].zone)
	}
	sc.resolveAll(t)
	if s.objs[cp].zone != kop.ZoneNone {
		t.Fatalf("copy zone %d after resolving, want ceased", s.objs[cp].zone)
	}
	d = s.Step()
	if got := preparedCastOffers(s, &d); len(got) != 0 {
		t.Fatalf("offered again: %v", got)
	}
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatal("pool not empty")
	}
}

// TestPrepareSourceLeaves: the source dies; the copy is unlinked, offered
// to nobody and ceases at the next state-based check.
func TestPrepareSourceLeaves(t *testing.T) {
	sc, id := prepScen(t, "Adventurous Eater", "Swamp")
	eater := enterPrepared(t, sc, id)
	s := sc.s
	cp := preparedRecords(s)[0].obj
	s.destroy(sc.k, sc.slot(eater))
	d := s.Step()
	if got := preparedCastOffers(s, &d); len(got) != 0 {
		t.Fatalf("offered with the source gone: %v", got)
	}
	if s.objs[cp].zone != kop.ZoneNone {
		t.Fatalf("copy zone %d, want ceased", s.objs[cp].zone)
	}
	if s.prepared(eater) {
		t.Fatal("a dead permanent is prepared")
	}
}

// TestPrepareNotFromHand: the inset spell is never offered from the hand.
func TestPrepareNotFromHand(t *testing.T) {
	sc, id := prepScen(t, "Adventurous Eater", "Swamp")
	sc.lands(0, id["Swamp"], 4)
	eater := sc.hand(id["Adventurous Eater"], 0)
	d := sc.s.Step()
	n := 0
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(eater)) {
			n++
			if a.Mode()&castFace != 0 {
				t.Fatalf("face cast offered from the hand: %v", a)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d casts of the front offered, want 1", n)
	}
}

// TestPrepareTiming: a sorcery copy waits for sorcery timing; an instant
// copy (Lightning Bolt) does not.
func TestPrepareTiming(t *testing.T) {
	sc, id := prepScen(t, "Adventurous Eater", "Emeritus of Conflict", "Swamp", "Mountain")
	s := sc.s
	sc.lands(0, id["Swamp"], 2)
	sc.lands(0, id["Mountain"], 2)
	eater := sc.perm(id["Adventurous Eater"], 0)
	emer := sc.perm(id["Emeritus of Conflict"], 0)
	s.prepare(sc.k, eater)
	s.prepare(sc.k, emer)
	recs := preparedRecords(s)
	if len(recs) != 2 {
		t.Fatalf("records %v", recs)
	}
	var d Decision
	s.priorityOptions(sc.k, &d)
	if got := preparedCastOffers(s, &d); len(got) != 2 {
		t.Fatalf("main phase offers %v, want both", got)
	}
	s.active = 1
	var inst Decision
	s.priorityOptions(sc.k, &inst)
	got := preparedCastOffers(s, &inst)
	if len(got) != 1 || got[0] != recs[1].obj {
		t.Fatalf("instant speed offers %v, want only the instant %d", got, recs[1].obj)
	}
}

// TestPrepareValidSelf: ExSelfPrepared before and after the grant.
func TestPrepareValidSelf(t *testing.T) {
	sc, id := prepScen(t, "Emeritus of Conflict", "Mountain")
	s := sc.s
	emer := sc.perm(id["Emeritus of Conflict"], 0)
	ctx := evalCtx{src: emer, srcSl: sc.slot(emer)}
	pos, neg := kop.Expr{Kind: kop.ExSelfPrepared}, kop.Expr{Kind: kop.ExSelfPrepared, N: 1}
	if s.selfPrepared(&pos, ctx) != 0 || s.selfPrepared(&neg, ctx) != 1 {
		t.Fatal("unprepared: IsPrepared must be 0 and !IsPrepared 1")
	}
	s.prepare(sc.k, emer)
	if s.selfPrepared(&pos, ctx) != 1 || s.selfPrepared(&neg, ctx) != 0 {
		t.Fatal("prepared: IsPrepared must be 1 and !IsPrepared 0")
	}
}

// TestPrepareReversalKeepsFace: a reversed prepared_copy cast returns the
// copy to exile still showing its spell face (gorge records no faceBefore
// for the route), unprepared (CR 733.1 as gorge's abortCast).
func TestPrepareReversalKeepsFace(t *testing.T) {
	sc, id := prepScen(t, "Adventurous Eater", "Swamp")
	eater := enterPrepared(t, sc, id)
	s := sc.s
	cp := preparedRecords(s)[0].obj
	face := s.objs[cp].card
	s.Step()
	s.Apply(MakeAction(AkCast, TCode(cp), 1, castFace))
	s.Step()
	s.reverseCast(sc.k)
	if ob := &s.objs[cp]; ob.zone != kop.ZoneExile || ob.card != face {
		t.Fatalf("after the reversal: zone %d card %d, want exile card %d", ob.zone, ob.card, face)
	}
	if s.prepared(eater) {
		t.Fatal("the reversal restored the designation (gorge keeps it unprepared)")
	}
}
