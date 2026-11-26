package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// lossCopyFaceDown puts the first card named name from seat p's library onto
// the battlefield under p face down, marked manifested or cloaked (CR
// 701.40a, 701.58a, faceDownChosen's shape).
func lossCopyFaceDown(t *testing.T, s *State, k *prep, p uint8, name string, cloaked bool) Obj {
	t.Helper()
	pl := &s.pl[p]
	for i := pl.libLo; i < pl.libHi; i++ {
		o := pl.lib[i]
		if k.t.Cards[s.objs[o].card].Name == name {
			from := s.detach(o)
			sl := s.enterFaceDown(k, o, p, kop.MorphMorph, from)
			s.markFaceDown(sl, cloaked)
			return s.perms[sl].obj
		}
	}
	t.Fatalf("seat %d: no %s in the library", p, name)
	return NoObj
}

// TestContractLossCopiesFaceDownBoard: seat 1 holds a contract counter that
// seat 0's Miss Highwater Effect put there and loses the game while one of
// its artifacts or creatures is face down (a manifested card and a cloaked
// one). The copies show the face-down copiable values (CR 707.2 as modified
// by CR 708.2a: a nameless 2/2 colourless creature with no abilities, a
// cloak's ward {2}, CR 701.58a; a manifested card's none, CR 701.40a) and
// never the hidden card, as gorge's copyTokenBasis mints them over
// faceDownCopyCard; the face-up permanent beside them still copies its card.
func TestContractLossCopiesFaceDownBoard(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	entries := []cardset.Entry{{Name: "Miss Highwater", Count: 4}, {Name: "Birds of Paradise", Count: 10},
		{Name: "Sol Ring", Count: 6}, {Name: "Island", Count: 20}}
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
	up := lossCopyEnter(t, s, k, 1, "Birds of Paradise") // face up: still a plain card copy
	man := lossCopyFaceDown(t, s, k, 1, "Sol Ring", false)
	clo := lossCopyFaceDown(t, s, k, 1, "Birds of Paradise", true)

	// the preconditions the assertions depend on: two face-down permanents
	// with their hidden cards, one face-up creature
	manSl, cloSl, upSl := s.objs[man].slot, s.objs[clo].slot, s.objs[up].slot
	solRing, birdCard := s.perms[manSl].card, s.perms[cloSl].card
	if !s.faceDown(k, manSl) || !s.manifested(manSl) || s.cardAt(manSl) != solRing {
		t.Fatalf("the manifested permanent = faceDown %v manifested %v card %d", s.faceDown(k, manSl), s.manifested(manSl), s.cardAt(manSl))
	}
	if !s.faceDown(k, cloSl) || !s.cloaked(cloSl) || s.cardAt(cloSl) != birdCard {
		t.Fatalf("the cloaked permanent = faceDown %v cloaked %v card %d", s.faceDown(k, cloSl), s.cloaked(cloSl), s.cardAt(cloSl))
	}
	if s.faceDown(k, upSl) || s.typesAt(k, upSl) != k.cards[birdCard].types {
		t.Fatalf("the face-up control: faceDown %v types %d, want the card's %d", s.faceDown(k, upSl), s.typesAt(k, upSl), k.cards[birdCard].types)
	}
	s.addContract(k, 1, 1)
	s.setContractOwed(1, 0, up)
	if c, _, ok := s.contractOwedBy(1); !ok || c != 0 || s.contracts(1) == 0 {
		t.Fatalf("owed record before the loss: seat %d ok %v, contracts %d", c, ok, s.contracts(1))
	}
	if n := len(lossCopyTokens(s, 0)); n != 0 {
		t.Fatalf("seat 0 holds %d token kinds before the loss, want none", n)
	}

	pre := s.outMask()
	s.pl[1].lost = true
	s.sweepDeparted(k, pre)
	if n := len(lossCopyTokens(s, 0)); n != 0 {
		t.Fatalf("the loss sweep minted %d token kinds before the trigger resolved", n)
	}
	if s.npend != 1 || s.pend[0].trigAmt != contractLossTrigAmt || s.pend[0].ctrl != 0 ||
		s.pend[0].trigPl != 1 {
		t.Fatalf("pending loss-copy trigger = %+v (n=%d), want one contractLossTrigAmt for seat 1",
			s.pend[:s.npend], s.npend)
	}
	pt := s.pend[0]
	s.npend = 0
	s.resolveDelayed(k, &stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card,
		ab: pt.ab, ctrl: pt.ctrl, trigPl: pt.trigPl, trigAmt: pt.trigAmt})

	// seat 0 gets three copy tokens: the face-up creature's full card copy,
	// and one face-down-valued copy each for the manifested and the cloaked
	// permanent -- face up, nameless 2/2 colourless creatures with no
	// abilities, the cloak's ward {2}, never the hidden card's characteristics.
	toks := lossCopyTokens(s, 0)
	nToks := 0
	for _, n := range toks {
		nToks += n
	}
	if nToks != 3 {
		t.Fatalf("seat 0's token copies = %v, want three tokens", toks)
	}
	plain, manif, cloaked := 0, 0, 0
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		pm := &s.perms[sl]
		if pm.ctrl != 0 || s.objs[pm.obj].of&ofCopy == 0 {
			continue
		}
		var po ProjObj
		s.projObj(k, pm.obj, &po)
		switch {
		case !s.faceDown(k, sl):
			// the face-up control: the card's full characteristics, a
			// projected name
			if s.cardAt(sl) != birdCard || s.typesAt(k, sl) != k.cards[birdCard].types {
				t.Fatalf("the face-up control's copy = card %d types %d, want the card's", s.cardAt(sl), s.typesAt(k, sl))
			}
			if pw, tg := s.pt(k, sl); pw != 0 || tg != 1 {
				t.Fatalf("the face-up control's copy = %d/%d, want 0/1", pw, tg)
			}
			if po.Flags&ProjFaceDown != 0 || po.Flags2&ProjNameless != 0 {
				t.Fatalf("the face-up control's copy projects face-down %v nameless %v", po.Flags&ProjFaceDown != 0, po.Flags2&ProjNameless != 0)
			}
			plain++
		case s.manifested(sl):
			// the manifested card's copy: face-down copiable values, no ward
			if s.cardAt(sl) != solRing {
				t.Fatalf("the manifested card's copy carries card %d, want the hidden card's basis", s.cardAt(sl))
			}
			manif += checkFaceDownCopy(t, s, k, sl, &po, false)
		case s.cloaked(sl):
			// the cloaked permanent's copy: face-down copiable values and ward {2}
			if s.cardAt(sl) != birdCard {
				t.Fatalf("the cloaked permanent's copy carries card %d, want the hidden card's basis", s.cardAt(sl))
			}
			cloaked += checkFaceDownCopy(t, s, k, sl, &po, true)
		default:
			t.Fatalf("seat 0's copy at slot %d carries neither provenance mark", sl)
		}
	}
	if plain != 1 || manif != 1 || cloaked != 1 {
		t.Fatalf("seat 0's copies = %d plain, %d manifested, %d cloaked, want 1 of each", plain, manif, cloaked)
	}
	if _, _, ok := s.contractOwedBy(1); ok {
		t.Fatal("the owed record outlived the loss")
	}
}

// checkFaceDownCopy asserts the face-down-valued copy at sl: a face-up
// nameless 2/2 colourless creature with no abilities, ward {2} for a cloaked
// source only.
func checkFaceDownCopy(t *testing.T, s *State, k *prep, sl Slot, po *ProjObj, ward bool) int {
	t.Helper()
	if !s.copiedFaceDown(sl) {
		t.Fatalf("the copy at slot %d is not marked a copy of a face-down permanent", sl)
	}
	if s.faceDownWard(k, sl) != ward {
		t.Fatalf("the copy at slot %d has ward %v, want %v", sl, s.faceDownWard(k, sl), ward)
	}
	if ty := s.typesAt(k, sl); ty != kop.TCreature {
		t.Fatalf("the copy at slot %d has types %d, want only TCreature", sl, ty)
	}
	if pw, tg := s.pt(k, sl); pw != 2 || tg != 2 {
		t.Fatalf("the copy at slot %d is %d/%d, want 2/2", sl, pw, tg)
	}
	if po.Flags&ProjFaceDown != 0 || po.Flags2&ProjNameless == 0 {
		t.Fatalf("the copy at slot %d projects face-down %v nameless %v, want face up and nameless",
			sl, po.Flags&ProjFaceDown != 0, po.Flags2&ProjNameless != 0)
	}
	return 1
}

// TestContractLossCopyFaceDownFiresTokenCreated: the loss copy of a face-down
// permanent is a token creation (CR 111.1) like the face-up copy: a "whenever
// you create a creature token" watcher seat 0 controls triggers for the
// manifested copy alone, as copyTokenEnterFd's tail fires it (gorge emits
// TokenCreated for every copy token) and res.lastTok names the new token.
func TestContractLossCopyFaceDownFiresTokenCreated(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	entries := []cardset.Entry{{Name: "Miss Highwater", Count: 4}, {Name: "Birds of Paradise", Count: 10},
		{Name: "Sol Ring", Count: 6}, {Name: "Island", Count: 20}}
	decks := make([][]cardset.Entry, MaxSeats)
	for p := range decks {
		decks[p] = entries
	}
	tbl, da, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Owner: kop.CtrlYou})
	yours := kop.FilterID(len(tbl.Filters) - 1)
	staff := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Staff", Types: kop.TArtifact, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigTokenCreated, Filter: yours, Flags2: kop.Tf2Batch},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	var cds [MaxSeats][]CardID
	for p := range cds {
		cds[p] = da[p]
	}
	s := new(State)
	s.InitN(tbl, cds, MaxSeats, 1, Config{NoElide: true})
	k := s.k()
	if !k.gen.anyTokCr {
		t.Fatal("anyTokCr unset: the table has no TokenCreated watcher")
	}
	so := Obj(s.nobj)
	s.nobj++
	s.objs[so] = object{card: staff, chosen: 255, slot: NoSlot, acts: 0 << objOwnerShift}
	s.enter(k, so, 0, false, kop.ZoneNone)
	man := lossCopyFaceDown(t, s, k, 1, "Sol Ring", false)
	if !s.faceDown(k, s.objs[man].slot) {
		t.Fatal("the manifested permanent is not face down")
	}
	s.addContract(k, 1, 1)
	s.setContractOwed(1, 0, man)
	pre := s.outMask()
	s.pl[1].lost = true
	s.sweepDeparted(k, pre)
	if s.npend != 1 || s.pend[0].trigAmt != contractLossTrigAmt {
		t.Fatalf("pending after the loss = %+v (n=%d), want the one loss-copy trigger", s.pend[:s.npend], s.npend)
	}
	pt := s.pend[0]
	s.npend = 0
	s.resolveDelayed(k, &stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card,
		ab: pt.ab, ctrl: pt.ctrl, trigPl: pt.trigPl, trigAmt: pt.trigAmt})
	toks := lossCopyTokens(s, 0)
	if len(toks) != 1 {
		t.Fatalf("seat 0's token copies = %v, want the one face-down copy", toks)
	}
	queued := 0
	for i := 0; i < int(s.npend); i++ {
		if s.pend[i].obj == so {
			queued++
		}
	}
	if queued != 1 {
		t.Fatalf("the face-down loss copy queued %d TokenCreated triggers, want 1", queued)
	}
	if tok := s.res.lastTok; tok == NoObj || s.objs[tok].of&ofCopy == 0 {
		t.Fatalf("res.lastTok = %d, want the loss copy token", tok)
	}
}
