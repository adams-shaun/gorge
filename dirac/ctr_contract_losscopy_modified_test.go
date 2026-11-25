package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestContractLossCopiesModifiedLoserBoard: the staged loss copy mints what
// the departing permanent's copiable values were, not its printed card alone.
// Seat 1's artifact is a copy of another card carrying a copy exception (a
// continuous P/T/type/keyword modification); the token seat 0 gets must show
// the same copy basis and the same modified characteristics (CR 707.2,
// 707.9b).
func TestContractLossCopiesModifiedLoserBoard(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	entries := []cardset.Entry{{Name: "Miss Highwater", Count: 4}, {Name: "Sol Ring", Count: 6},
		{Name: "Island", Count: 20}}
	decks := make([][]cardset.Entry, MaxSeats)
	for p := range decks {
		decks[p] = entries
	}
	tbl, da, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	var cds [MaxSeats][]CardID
	for p := range cds {
		cds[p] = da[p]
	}
	s := new(State)
	s.InitN(tbl, cds, MaxSeats, 1, Config{NoElide: true})
	k := s.k()
	if s.contractCold() == nil {
		t.Fatal("the table keeps no contract record")
	}
	host := lossCopyEnter(t, s, k, 1, "Sol Ring")
	land := lossCopyEnter(t, s, k, 1, "Island") // the copy basis, not copied itself
	sl := s.objs[host].slot
	printed := s.perms[sl].card
	basis := s.objs[land].card
	if printed == basis {
		t.Fatalf("host and basis are the same card %d", printed)
	}
	if s.cardAt(sl) != printed {
		t.Fatalf("host shows card %d before the copy, want its printed card %d", s.cardAt(sl), printed)
	}

	// An override record with a P/T set, added creature/artifact types and a
	// keyword: the shape of a copy exception (CR 707.9b), which is a copiable
	// value. The host is a copy of the land, so the token must be a copy of a
	// copy carrying the exception, not a plain land.
	exc := copyExcWith(t, k, 4, 4, kop.TCreature, kop.KwFlying)
	if k.gen == nil {
		t.Fatal("the table has no generated prep")
	}
	k.gen.cpxTy, k.gen.cpxPT = true, true // no table ability references the synthetic exception
	p := &s.perms[sl]
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = basis
	s.copyExcPush(sl, exc)
	p.ovr.kw |= s.copyExcKw(k, sl)
	s.refreshTy(k)
	s.touchSl(sl)

	// Preconditions the post-loss assertions depend on: the live host shows
	// the basis and the modified characteristics, and the basis card itself
	// is not accidentally modified.
	if s.cardAt(sl) != basis {
		t.Fatalf("host shows card %d, want the copy basis %d", s.cardAt(sl), basis)
	}
	hostTypes := s.typesAt(k, sl)
	if hostTypes&(kop.TCreature|kop.TArtifact) != kop.TCreature|kop.TArtifact {
		t.Fatalf("host types %#x lack the exception's creature and artifact types", hostTypes)
	}
	if pw, tg := s.pt(k, sl); pw != 4 || tg != 4 {
		t.Fatalf("host is %d/%d, want the exception's 4/4", pw, tg)
	}
	if s.kw(k, sl)&kop.KwFlying == 0 {
		t.Fatal("host lacks the exception's flying")
	}
	if s.kw(k, s.objs[land].slot)&kop.KwFlying != 0 {
		t.Fatal("the basis card itself has flying; the assertion would not test the copy")
	}

	s.addContract(k, 1, 1)
	s.setContractOwed(1, 0, host)
	pre := s.outMask()
	s.pl[1].lost = true
	s.sweepDeparted(k, pre)
	if s.npend != 1 || s.pend[0].trigAmt != contractLossTrigAmt {
		t.Fatalf("pending loss-copy trigger = %+v (n=%d), want one contractLossTrigAmt", s.pend[:s.npend], s.npend)
	}
	pt := s.pend[0]
	s.npend = 0
	s.resolveDelayed(k, &stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card,
		ab: pt.ab, ctrl: pt.ctrl, trigPl: pt.trigPl, trigAmt: pt.trigAmt})

	var tok Slot = NoSlot
	for m := s.occ; m.any(); m = m.rest() {
		o := s.perms[m.first()].obj
		if s.perms[m.first()].ctrl != 0 || s.objs[o].of&ofCopy == 0 {
			continue
		}
		if tok != NoSlot {
			t.Fatalf("seat 0 holds two token copies (%d and %d), want one", tok, m.first())
		}
		tok = m.first()
	}
	if tok == NoSlot {
		t.Fatal("the staged loss copy minted no token")
	}
	tp := &s.perms[tok]
	if tp.card != printed {
		t.Fatalf("token is card %d, want the host's printed card %d", tp.card, printed)
	}
	if tp.ovr.flags&ovCopy == 0 || tp.ovr.copyOf != basis {
		t.Fatalf("token copy basis = flags %#x copyOf %d, want ovCopy of %d", tp.ovr.flags, tp.ovr.copyOf, basis)
	}
	if got := s.cardAt(tok); got != basis {
		t.Fatalf("token shows card %d, want the copy basis %d", got, basis)
	}
	if got := s.typesAt(k, tok); got != hostTypes {
		t.Fatalf("token types %#x, want the host's %#x", got, hostTypes)
	}
	if pw, tg := s.pt(k, tok); pw != 4 || tg != 4 {
		t.Fatalf("token is %d/%d, want the host's 4/4", pw, tg)
	}
	if s.kw(k, tok)&kop.KwFlying == 0 {
		t.Fatal("token lacks the host's exception flying")
	}
}

// copyExcWith returns the 1-based Overrides index of a copy exception with
// the requested P/T, added types and keyword.
func copyExcWith(t *testing.T, k *prep, p, tg int8, ty kop.TypeMask, kw kop.KwMask) uint8 {
	t.Helper()
	for i := range k.t.Overrides {
		o := &k.t.Overrides[i]
		if o.HasPT && o.SetP == p && o.SetT == tg && o.AddTypes&ty == ty && o.AddKw&kw == kw {
			return uint8(i + 1)
		}
	}
	t.Fatalf("no copy exception with %d/%d, types %#x, keyword %#x", p, tg, ty, kw)
	return 0
}
