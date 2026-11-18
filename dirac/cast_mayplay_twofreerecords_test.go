package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Two free permission records over one exiled card (two Dauthi Voidwalkers
// each sacrificed to choose the same VOID-countered card) list the card's one
// free cast, as gorge's per-card dedupe does (rules mayPlaySpellIds). The
// lockstep's both-riders deck showed the kernel listing it twice
// (TestExileOnLeaveRiderLockstep).
func TestTwoFreeMayPlayRecordsOfferOneCast(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	o := sc.obj(v.ThinkTwice, 0)
	s.objs[o].zone = kop.ZoneExile
	rec := mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, kind: kop.MayPlayFree, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.g.may[0], s.g.may[1] = rec, rec
	s.g.nmay = 2

	count := func(sc *scen) (n int) {
		var d Decision
		var av avail
		sc.s.priorityGen(sc.k, &d, 0, true, &av, false)
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
				n++
			}
		}
		return n
	}
	// precondition: both records are live and offerable, so a walk that
	// offers once per record and one that offers once per card differ
	for i := 0; i < 2; i++ {
		if r := *s.mayAt(i); !s.mayRecordOfferable(r, 0) || r.kind&kop.MayPlayFree == 0 {
			t.Fatalf("precondition: record %d is not a live free permission", i)
		}
	}
	if n := count(sc); n != 1 {
		t.Fatalf("two free records over one card offer %d casts, want 1", n)
	}

	// the one free record alone still offers its cast
	s.g.nmay = 1
	if n := count(sc); n != 1 {
		t.Fatalf("one free record offers %d casts, want 1", n)
	}

	// two identical pay-life free records over one card (the same pay-life
	// cast granted twice, e.g. two Inside Information resolutions) also list
	// the card's one cast: gorge's per-card dedupe does not count grants
	recPL := rec
	recPL.until = mayUntilPayLife // DurEOT is 0, so the byte carries the pay-life mark alone
	s.g.may[0], s.g.may[1] = recPL, recPL
	s.g.nmay = 2
	for i := 0; i < 2; i++ {
		if r := *s.mayAt(i); !s.mayRecordOfferable(r, 0) || !r.payLife() {
			t.Fatalf("precondition: record %d is not a live pay-life permission", i)
		}
	}
	if n := count(sc); n != 1 {
		t.Fatalf("two pay-life records over one card offer %d casts, want 1", n)
	}
	// the one pay-life record alone still offers its cast
	s.g.nmay = 1
	if n := count(sc); n != 1 {
		t.Fatalf("one pay-life record offers %d casts, want 1", n)
	}
}
