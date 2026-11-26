package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// grantOnlyOv is the (index+1) of the table's first override record that
// grants exactly one ability and changes nothing else (the shape of Mercurial
// Pretender's AddAbilities$ grant: kop.Override.Grant alone), and the ability
// index it grants.
func grantOnlyOv(t *testing.T, k *prep) (uint8, uint8) {
	t.Helper()
	for i := range k.t.Overrides {
		o := &k.t.Overrides[i]
		if o.Grant[0] == 0 || o.Grant[1] != 0 || o.HasPT || o.AddKw != 0 || o.RemoveKw != 0 ||
			o.AddTypes != 0 || o.HasTypes || o.HasSub || o.AddColors != 0 ||
			o.HasColors || o.LoseAbilities || o.NameCard != 0 || o.UntilUpkeep || o.GrantStatics != 0 ||
			o.AddSub != (kop.SubMask{}) || o.SetSub != (kop.SubMask{}) || o.SetTypes != 0 ||
			o.SetColors != 0 || o.Banding || o.Horsemanship {
			continue
		}
		return uint8(i + 1), o.Grant[0] - 1
	}
	t.Fatal("no grant-only override in the table")
	return 0, 0
}

// cardNamed is the first card of the table named name.
func cardNamed(t *testing.T, k *prep, name string) CardID {
	t.Helper()
	for c := range k.t.Cards {
		if k.t.Cards[c].Name == name {
			return CardID(c)
		}
	}
	t.Fatalf("no card named %s in the table", name)
	return 0
}

// typeAddExc is the (index+1) of the table's first copy exception that only
// adds the artifact type (the shape of a copy's "except it's an artifact").
func typeAddExc(t *testing.T, k *prep) uint8 {
	t.Helper()
	for i := range k.t.Overrides {
		o := &k.t.Overrides[i]
		if o.AddTypes&kop.TArtifact == 0 || o.HasPT || o.AddKw != 0 || o.RemoveTypes != 0 ||
			o.HasTypes || o.SetTypes != 0 || o.LoseAbilities || o.Grant[0] != 0 || o.UntilUpkeep {
			continue
		}
		return uint8(i + 1)
	}
	t.Fatal("no artifact-adding copy exception in the table")
	return 0
}

// TestContractLossCopiesGrantedAbilities: the staged loss copy of a copy
// carrying an as-enters copy grant (the ovCopyGrant shape: Mercurial
// Pretender's AddAbilities$ on an as-enters copy, CR 707.9b) mints a token
// that still has the granted ability. Gorge folds the grant into the copied
// face (engine/effects/clone_etb_sub.go cloneGrantSVars), so its LKI copy
// carries it; the snapshot must keep the (override, maker card) pair and the
// mint must re-apply it.
func TestContractLossCopiesGrantedAbilities(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	entries := []cardset.Entry{{Name: "Miss Highwater", Count: 4}, {Name: "Sol Ring", Count: 6},
		{Name: "Amoeboid Changeling", Count: 2}, {Name: "Island", Count: 20}}
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
	if s.cardGrantsAct(k, sl, basis, 0) {
		t.Fatal("the basis card's own table grants ability 0 as a live activation; the grant record would not be the only way to host the granted ability")
	}
	// The grant's maker is the entering card (an as-enters copy's
	// AddAbilities$ reads the entering card's own table, copy_asenters.go). A
	// Pretender copy carries the Pretender's grant; here the host is a Sol
	// Ring, so the maker is a deck card whose ability ab stands in for the
	// Pretender's (the matchup table has only the deck's cards).
	ov, ab := grantOnlyOv(t, k)
	maker := cardNamed(t, k, "Amoeboid Changeling")
	if len(k.t.Cards[maker].Abilities) <= int(ab) {
		t.Fatalf("precondition: the maker card %d has no ability %d for grant override %d", maker, ab, ov)
	}

	// The host is an as-enters copy of the land carrying the grant, with a
	// copy exception adding the artifact type: what becomeCopyEnter leaves
	// (copy_asenters.go). The artifact type makes the copy one of the
	// artifacts the loss copy snapshots (a plain copy of a land is not).
	p := &s.perms[sl]
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = basis
	exc := typeAddExc(t, k)
	if k.gen == nil {
		t.Fatal("the table has no generated prep")
	}
	k.gen.cpxTy = true // no table ability references the synthetic exception
	s.copyExcPush(sl, exc)
	p.ovr.kw |= s.copyExcKw(k, sl)
	s.refreshTy(k)
	s.touchSl(sl)
	s.copyGrantPut(k, sl, cgrantRef{ov, maker})
	if s.cardAt(sl) != basis {
		t.Fatalf("host shows card %d, want the copy basis %d", s.cardAt(sl), basis)
	}
	hostTypes := s.typesAt(k, sl)
	if hostTypes&kop.TArtifact == 0 {
		t.Fatalf("host types %#x lack the exception's artifact type; the sweep would not snapshot it", hostTypes)
	}
	// Preconditions the post-loss assertions depend on: the live host hosts
	// the granted activation, read from the maker card.
	if got := s.selfGrantHost(k, sl, maker, ab); got != sl {
		t.Fatalf("the live copy hosts the granted ability at slot %d, want %d", got, sl)
	}
	if !s.copyGrantIs(k, sl, maker, ab) {
		t.Fatal("the granted ability is no as-enters copy grant")
	}
	if got := s.grantCardAt(k, sl, ab); got != maker {
		t.Fatalf("the granted activation reads from card %d, want the maker %d", got, maker)
	}
	wantGrants(t, s, sl, cgrantRef{ov, maker})

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
	if tp := &s.perms[tok]; tp.card != printed {
		t.Fatalf("token is card %d, want the host's printed card %d", tp.card, printed)
	}
	if got := s.cardAt(tok); got != basis {
		t.Fatalf("token shows card %d, want the copy basis %d", got, basis)
	}
	if tp := &s.perms[tok]; tp.ovr.flags&ovCopy == 0 || tp.ovr.copyOf != basis {
		t.Fatalf("token copy basis = flags %#x copyOf %d, want ovCopy of %d", tp.ovr.flags, tp.ovr.copyOf, basis)
	}
	if got := s.typesAt(k, tok); got != hostTypes {
		t.Fatalf("token types %#x, want the host's %#x", got, hostTypes)
	}
	// the grant rode the snapshot and was re-applied at the mint
	wantGrants(t, s, tok, cgrantRef{ov, maker})
	if got := s.selfGrantHost(k, tok, maker, ab); got != tok {
		t.Fatalf("the token hosts the granted ability at slot %d, want itself (%d)", got, tok)
	}
	if !s.copyGrantIs(k, tok, maker, ab) {
		t.Fatal("the token's granted ability is no as-enters copy grant")
	}
	if got := s.grantCardAt(k, tok, ab); got != maker {
		t.Fatalf("the token's granted activation reads from card %d, want the maker %d", got, maker)
	}
	// nothing else came along: the token carries the copy basis only (plus the
	// grant's own record), no P/T or keyword override
	if tp := &s.perms[tok]; tp.ovr.flags&^(ovCopy|ovGenTy) != 0 || tp.ovr.setP != 0 || tp.ovr.setT != 0 || tp.ovr.types != 0 || tp.ovr.kw != 0 {
		t.Fatalf("token override = flags %#x %d/%d types %#x kw %#x, want the copy basis only", tp.ovr.flags, tp.ovr.setP, tp.ovr.setT, tp.ovr.types, tp.ovr.kw)
	}
}
