package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBuildStaleGateLKI finishes a spell's build (the pushed entry carried a
// copy of the build, li included, onto the stack) and then runs a
// resolution-time mana payment. s.b now holds stale bytes, so:
//   - lkiSweep must not hold the finished build's li (it would keep the LKI
//     record referenced after every live holder is gone);
//   - noteTapSrc must not record the resolution payment's source as a cast
//     source (mana.go's plan tap and mana_window.go's window tap);
//   - FeatSubCtrWildcard must not read the finished build's subCtr.
//
// Mutation M1 (make the lkiSweep hold unconditional again) fails this test.
func TestBuildStaleGateLKI(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Cackling Prowler", Count: 1}, {Name: "Forest", Count: 8}})
	s := sc.s
	bear := sc.perm(id["Cackling Prowler"], 0)
	forest := sc.perm(id["Forest"], 0)
	forest2 := sc.perm(id["Forest"], 0) // left untapped for the manual window below
	li := s.snapLKI(bear)
	if li == 0 {
		t.Fatal("no LKI record for the battlefield creature")
	}
	// the source carries a cast-source reader mark (a Treasure / observed
	// source reader exists in this table): a cast tap through it would call
	// noteTapSrc, a resolution tap must not
	sc.k.cards[id["Forest"]].castSrc |= csTreasure
	sc.k.gen.castTapOn = true

	// The cast is over: its build (li from a sacrificed source, obsSl of the
	// observed source it tapped) sits stale in s.b; the stack entry that was
	// pushed with the same li has since resolved.
	*s.bld() = build{e: stackEntry{kind: seSpell, card: id["Cackling Prowler"], ctrl: 0, li: li}}
	s.bld().obsSl = s.objs[bear].slot + 1
	s.bld().subCtr = 1
	s.ctl = ctlResolve
	s.res.active = true
	s.res.ask = raGenPay

	// A resolution-time cost paid through gorge's planner (raGenPay's
	// payReplanned): it taps the Forest, not a cast-source tap.
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 1}}
	s.payReplanned(sc.k, 0, &cost)
	if s.perms[s.objs[forest].slot].flags&pfTapped == 0 {
		t.Fatal("resolution payment did not tap the Forest")
	}
	if s.bld().bfl&bflTre != 0 || s.bld().obsSl != s.objs[bear].slot+1 {
		t.Fatalf("noteTapSrc ran for a resolution tap on the finished build: bfl %#x obsSl %d", s.bld().bfl, s.bld().obsSl)
	}

	// The window tap path (payWindow / a typed source's manual window) reads
	// the same stale kind and the finished build's waterbend picks: it must
	// neither record the tap nor withhold the picked permanent.
	s.bld().bfl &^= bflTre
	s.bld().pickN = 1
	s.bld().picks[0] = forest2
	s.bld().pickKs[0] = pkWaterbend // a pick of the finished cast
	if ok, _ := s.windowTap(sc.k, 0, &cost, true); !ok {
		t.Fatal("resolution window payment found no source (a finished build's waterbend pick withheld it)")
	}
	if s.perms[s.objs[forest2].slot].flags&pfTapped == 0 {
		t.Fatal("resolution window payment did not tap the second Forest")
	}
	if s.bld().bfl&bflTre != 0 {
		t.Fatalf("noteTapSrc ran for a resolution window tap on the finished build: bfl %#x", s.bld().bfl)
	}

	// A resolution counter pick is not Quilled Greatwurm's wildcard cast.
	if s.FeatSubCtrWildcard() {
		t.Fatal("FeatSubCtrWildcard read the finished build's subCtr")
	}

	// The finished build's LKI index is not a live holder any more.
	s.lkiSweep()
	if got := s.g.lki[li-1].ref; got != 0 {
		t.Fatalf("finished build holds LKI record %d: ref %d, want 0", li, got)
	}

	// Control: while the build is live its li is still held (and the
	// wildcard ask is read from it).
	s.ctl = ctlBuild
	s.lkiSweep()
	if got := s.g.lki[li-1].ref; got != 1 {
		t.Fatalf("live build's LKI record %d ref %d, want 1", li, got)
	}
	if !s.FeatSubCtrWildcard() {
		t.Fatal("live build's wildcard cast cost not reported")
	}
}
