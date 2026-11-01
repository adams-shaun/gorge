package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// OpTransform (act_transform.go) on the generated setstate pool:
// Monica Rambeau's front face turns to Photon, Living Light, in place.

func setstateScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "setstate")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the setstate table", n)
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

// TestSetstateTransform: the permanent turns over, and counters, damage,
// attachments, tapped state and summoning sickness stay (CR 701.27a-b).
func TestSetstateTransform(t *testing.T) {
	sc, id := setstateScen(t, "Monica Rambeau", "Hill Giant")
	front := id["Monica Rambeau"]
	back := sc.k.t.Cards[front].Faces[1]
	monica := sc.perm(front, 0)
	host := sc.perm(id["Hill Giant"], 0)
	sl := sc.slot(monica)
	p := &sc.s.perms[sl]
	p.setCtrN(sc.s, int(kop.CtrP1P1), 2)
	p.damage = 3
	p.flags |= pfTapped | pfSick
	sc.s.attachTo(sc.k, sc.slot(host), sl)
	ts := p.ts
	sc.s.res.e.obj, sc.s.res.e.card = monica, front
	sc.s.transform(sc.k, &kop.Instr{Obj: kop.RefSelf})
	if got := sc.s.cardAt(sl); got != back || sc.s.objs[monica].card != back {
		t.Fatalf("card %d, want the back face %d", got, back)
	}
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Errorf("+1/+1 counters %d, want 2", n)
	}
	if p.damage != 3 {
		t.Errorf("damage %d, want 3", p.damage)
	}
	if p.flags&pfTapped == 0 || p.flags&pfSick == 0 {
		t.Errorf("tapped/sick flags %#x, want both", p.flags)
	}
	if got := sc.s.perms[sc.slot(host)].attached; got != sl {
		t.Errorf("attachment host %d, want slot %d", got, sl)
	}
	if p.ts != ts {
		t.Error("the permanent's timestamp changed")
	}
}

// TestSetstateTransformFaces: two turns wrap back to the front, and a
// source that left the battlefield is left untouched in its graveyard
// (CR 701.27a: only a permanent transforms; faceReset already put the
// front up).
func TestSetstateTransformFaces(t *testing.T) {
	sc, id := setstateScen(t, "Monica Rambeau")
	front := id["Monica Rambeau"]
	back := sc.k.t.Cards[front].Faces[1]
	monica := sc.perm(front, 0)
	sl := sc.slot(monica)
	sc.s.res.e.obj, sc.s.res.e.card = monica, front
	sc.s.transform(sc.k, &kop.Instr{Obj: kop.RefSelf})
	if sc.s.cardAt(sl) != back {
		t.Fatal("first transform: not the back face")
	}
	sc.s.transform(sc.k, &kop.Instr{Obj: kop.RefSelf})
	if sc.s.cardAt(sl) != front {
		t.Fatal("second transform: not the front face")
	}
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if sc.s.objs[monica].card != front {
		t.Fatal("in the graveyard: not the front face")
	}
	sc.s.transform(sc.k, &kop.Instr{Obj: kop.RefSelf})
	if sc.s.objs[monica].card != front || sc.s.objs[monica].zone != kop.ZoneGraveyard {
		t.Fatal("off the battlefield: not the front face in the graveyard")
	}
	var pr Projection
	sc.s.Project(&pr)
	if pr.Objs[monica].Card != front {
		t.Fatal("projection: not the front face in the graveyard")
	}
}

// TestTransformOffBattlefieldNoop: a transform ability resolving after its
// source left the battlefield flips nothing (CR 701.27a; gorge).
func TestTransformOffBattlefieldNoop(t *testing.T) {
	sc, id := setstateScen(t, "Monica Rambeau")
	front := id["Monica Rambeau"]
	monica := sc.perm(front, 0)
	sl := sc.slot(monica)
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if got := sc.s.objs[monica].card; got != front {
		t.Fatalf("in the graveyard: card %d, want the front face %d", got, front)
	}
	sc.s.res.e.obj, sc.s.res.e.card = monica, front
	sc.s.transform(sc.k, &kop.Instr{Obj: kop.RefSelf})
	if got := sc.s.objs[monica].card; got != front {
		t.Fatalf("off-battlefield transform flipped to %d, want it left on the front face %d", got, front)
	}
	if got := sc.s.objs[monica].zone; got != kop.ZoneGraveyard {
		t.Fatalf("zone %d, want the graveyard", got)
	}
}

// TestSetstateTransformCopy: CR 712.9's two examples. A non-DFC Clone that
// copied a DFC cannot transform; a DFC that copied a creature can, but its
// visible face stays the copied one. A token copy of a DFC is itself a
// double-faced token and turns over; a token copy of a single-faced card has
// no other face.
func TestSetstateTransformCopy(t *testing.T) {
	sc, id := setstateScen(t, "Monica Rambeau", "Hill Giant")
	front, giant := id["Monica Rambeau"], id["Hill Giant"]
	back := sc.k.t.Cards[front].Faces[1]

	clone := sc.perm(giant, 0)
	csl := sc.slot(clone)
	sc.s.perms[csl].ovr.flags |= ovCopy
	sc.s.perms[csl].ovr.copyOf = front
	sc.s.res.e.obj, sc.s.res.e.card = clone, giant
	sc.s.transform(sc.k, &kop.Instr{Obj: kop.RefSelf})
	if got := sc.s.cardAt(csl); got != front || sc.s.objs[clone].card != giant {
		t.Fatalf("Clone: card %d/%d, want the copied Monica %d on its own Hill Giant %d", got, sc.s.objs[clone].card, front, giant)
	}

	monica := sc.perm(front, 0)
	msl := sc.slot(monica)
	sc.s.perms[msl].ovr.flags |= ovCopy
	sc.s.perms[msl].ovr.copyOf = giant
	sc.s.res.e.obj, sc.s.res.e.card = monica, front
	sc.s.transform(sc.k, &kop.Instr{Obj: kop.RefSelf})
	if got := sc.s.cardAt(msl); got != giant {
		t.Fatalf("DFC copy: visible card %d, want the copied Hill Giant %d", got, giant)
	}
	if sc.s.objs[monica].card != back {
		t.Fatalf("DFC copy: hidden card %d, want the back face %d", sc.s.objs[monica].card, back)
	}

	tok, id2 := setstateScen(t, "Monica Rambeau", "Hill Giant")
	tfront, tgiant := id2["Monica Rambeau"], id2["Hill Giant"]
	tback := tok.k.t.Cards[tfront].Faces[1]
	tm := tok.perm(tfront, 0)
	tok.s.objs[tm].of |= ofCopy
	tok.s.res.e.obj, tok.s.res.e.card = tm, tfront
	tok.s.transform(tok.k, &kop.Instr{Obj: kop.RefSelf})
	if got := tok.s.cardAt(tok.slot(tm)); got != tback {
		t.Fatalf("token copy of a DFC: card %d, want the back face %d", got, tback)
	}
	tg := tok.perm(tgiant, 0)
	tok.s.objs[tg].of |= ofCopy
	tok.s.res.e.obj, tok.s.res.e.card = tg, tgiant
	tok.s.transform(tok.k, &kop.Instr{Obj: kop.RefSelf})
	if got := tok.s.cardAt(tok.slot(tg)); got != tgiant {
		t.Fatalf("token copy of a single-faced card: card %d, want Hill Giant %d", got, tgiant)
	}
}
