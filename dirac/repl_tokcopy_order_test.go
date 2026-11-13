package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// tokCopyTable is tokTable plus a copy-instead replacement (Moonlit
// Meditation's shape: while attached to a permanent and no token its
// controller owns has entered this turn, the token may instead be a copy).
func tokCopyTable(t *testing.T) (*kop.Table, tokIDs, CardID) {
	t.Helper()
	tbl, id := tokTable(t)
	copyCard := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Copy First",
		Types: kop.TEnchantment,
		Abilities: []kop.Ability{{
			Kind: kop.AbReplacement,
			Repl: kop.Repl{Kind: kop.ReplCopyFirstToken},
		}},
	})
	return tbl, id, copyCard
}

// tokCopyScen puts board onto seat 0's battlefield in order, then attaches
// copyCard (one of board) to a host card. It returns the copy source and the
// host object.
func tokCopyScen(t *testing.T, tbl *kop.Table, copyCard, host CardID, board ...CardID) (*scen, Obj, Obj) {
	t.Helper()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	var copyObj Obj
	for _, c := range board {
		o := sc.perm(c, 0)
		if c == copyCard {
			copyObj = o
		}
	}
	hs := sc.perm(host, 0)
	sc.s.perms[sc.slot(copyObj)].attached = sc.slot(hs)
	sc.s.res.e.ctrl = 0
	return sc, copyObj, hs
}

// tokCopyElection returns the copy-instead decision the walk is parked on.
func tokCopyElection(t *testing.T, sc *scen) Decision {
	t.Helper()
	if sc.s.res.ask != raGenTokCopy {
		t.Fatalf("res.ask %d, want the copy-instead election %d (stalled %d)", sc.s.res.ask, raGenTokCopy, sc.s.g.stalled)
	}
	var d Decision
	if !sc.s.resolveOptionsGen(sc.k, &d) || d.Kind != DkChoose || d.Sub != ChGenTokCopy {
		t.Fatalf("decision kind %d sub %d, want DkChoose/ChGenTokCopy", d.Kind, d.Sub)
	}
	return d
}

// tokCopyCreateScen is a resolving CreateToken program (the pair-332 shape)
// whose battlefield holds Doubling Season, the copy-instead source and its
// host, in that entry order.
func tokCopyCreateScen(t *testing.T) (*scen, []kop.Instr, tokIDs, CardID, Obj, Obj) {
	t.Helper()
	var id tokIDs
	var copyCard CardID
	sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
		id = addTokCards(tbl)
		copyCard = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{
			Name:  "Test Copy First",
			Types: kop.TEnchantment,
			Abilities: []kop.Ability{{
				Kind: kop.AbReplacement,
				Repl: kop.Repl{Kind: kop.ReplCopyFirstToken},
			}},
		})
		return []kop.Instr{{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(1), C: int16(id.soldier)}}
	})
	sc.perm(id.ds, 0)
	cp := sc.perm(copyCard, 0)
	host := sc.perm(id.illusion, 0)
	sc.s.perms[sc.slot(cp)].attached = sc.slot(host)
	sc.s.res.e.ctrl = 0
	return sc, prog, id, copyCard, cp, host
}

// TestTokCopyFirstBesideMultiplierLockstepCase: a copy-instead replacement
// beside Doubling Season is gorge's CR 616.1 order ask
// (replacement_token.go driveTokenReplacements asks while two candidates that
// do not all commute remain), and the accepted election copies every token
// the multiplier leaves. This is the pair-332 std_eoe stall: Moonlit
// Meditation beside Exalted Sunborn on a Station Monitor Drone creation.
func TestTokCopyFirstBesideMultiplierLockstepCase(t *testing.T) {
	for _, pick := range []int{0, 1} {
		for _, accept := range []bool{false, true} {
			sc, prog, id, copyCard, copyObj, host := tokCopyCreateScen(t)
			s := sc.s
			// precondition: the source is attached where tokCopyApplies looks,
			// and the election applies now
			if s.perms[s.objs[copyObj].slot].attached != s.objs[host].slot {
				t.Fatal("the copy-instead source is not attached to the host")
			}
			a := &s.k().t.Cards[s.cardAt(s.objs[copyObj].slot)].Abilities[0]
			if _, ok := s.tokCopyApplies(s.k(), s.objs[copyObj].slot, a, 0); !ok {
				t.Fatal("the copy-instead replacement does not apply: the fixture never reaches the election")
			}
			if s.exec(sc.k, &prog[0]) {
				t.Fatal("the op ran on: no order ask beside a multiplier")
			}
			d := tokAsk(t, sc)
			if d.N != 2 || d.Player != 0 {
				t.Fatalf("pick %d: %d options to player %d, want 2 to the creator", pick, d.N, d.Player)
			}
			for i := 0; i < 2; i++ {
				want := []CardID{id.ds, copyCard}[i]
				if got := s.cardAt(s.objs[Obj(d.Options[i].Source())].slot); got != want {
					t.Fatalf("option %d source %s, want %s", i, s.k().t.Cards[got].Name, s.k().t.Cards[want].Name)
				}
			}
			s.applyResolveGen(sc.k, d.Options[pick])
			e := tokCopyElection(t, sc)
			if e.N != 2 || e.Options[0].Mode() != 0 || e.Options[1].Mode() != 1 {
				t.Fatalf("pick %d: election options %v, want decline then the host", pick, e.Options[:e.N])
			}
			if e.Options[1].Source() != uint8(host) {
				t.Fatalf("pick %d: the election's host %d is not the attached permanent %d", pick, e.Options[1].Source(), host)
			}
			ans := e.Options[0]
			if accept {
				ans = e.Options[1]
			}
			s.applyResolveGen(sc.k, ans)
			if s.g.stalled != 0 || s.res.ask != raNone {
				t.Fatalf("pick %d accept %v: stall %d ask %d, want the token settled", pick, accept, s.g.stalled, s.res.ask)
			}
			wantIllusions := 1 // the host
			if accept {
				wantIllusions = 3 // the host and the two copied tokens
			}
			if got := tokOf(sc, id.illusion); got != wantIllusions {
				t.Fatalf("pick %d accept %v: %d Illusions, want %d", pick, accept, got, wantIllusions)
			}
			wantSoldiers := 0
			if !accept {
				wantSoldiers = 2
			}
			if got := tokOf(sc, id.soldier); got != wantSoldiers {
				t.Fatalf("pick %d accept %v: %d Soldiers, want %d", pick, accept, got, wantSoldiers)
			}
		}
	}
}

// TestTokCopySiteDeclinesElection: a copy proposal cannot park the copy effect
// mid-resolution, so gorge declines the copy-instead election and applies the
// other bodies in scan order (replacement_copytoken.go
// continueCopyTokenProposal). A multiplier still applies; the election must
// not stall. This is the pair-313 std-eoe stall: Moonlit Meditation on a
// Devastating Onslaught CopyPermanent.
func TestTokCopySiteDeclinesElection(t *testing.T) {
	tbl, id, copyCard := tokCopyTable(t)
	sc, copyObj, host := tokCopyScen(t, tbl, copyCard, id.illusion, id.ds, copyCard)
	s := sc.s
	if s.perms[s.objs[copyObj].slot].attached != s.objs[host].slot {
		t.Fatal("the copy-instead source is not attached to the host")
	}
	a := &s.k().t.Cards[s.cardAt(s.objs[copyObj].slot)].Abilities[0]
	if _, ok := s.tokCopyApplies(s.k(), s.objs[copyObj].slot, a, 0); !ok {
		t.Fatal("the copy-instead replacement does not apply: the fixture never reaches the replacement")
	}
	card, n, rewritten, ok := s.copyTokenRewrite(sc.k, 0, id.soldier, 1, true)
	if !ok {
		t.Fatalf("copy site stalled %d (%s), want the election declined", s.g.stalled, stallNames[s.g.stalled])
	}
	if s.g.stalled != 0 || card != id.soldier || rewritten || n != 2 {
		t.Fatalf("copy site: card %s n %d rewritten %v stall %d, want the doubled soldier and no rewrite",
			s.k().t.Cards[card].Name, n, rewritten, s.g.stalled)
	}
}
