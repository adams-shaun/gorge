package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHandMove: OpChooseMove from a hand (gorge effChangeZoneHand). With
// SrConfirm the owner is first asked yes/no (ChGenConfirm); yes poses the
// optional hand_move pick (Done allowed) and the picked land enters
// tapped; no skips. A required move with no more eligible cards than the
// bound takes them all unasked.
func TestHandMove(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	pl := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Zone: kop.ZoneHand})
	f := kop.FilterID(len(tbl.Filters) - 1)
	confirm := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(1), F: f,
		C: int16(kop.ZoneBattlefield) | int16(kop.SrConfirm|kop.SrTapped)<<3}
	must := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(2), F: f,
		C: int16(kop.ZoneBattlefield) | int16(kop.SrMandatory)<<3}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Mover", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{confirm, must}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	l1, l2 := sc.hand(pl, 0), sc.hand(pl, 0)
	sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc = 0, src, 0, 0
	if sc.s.startSearch(sc.k, &confirm) || sc.s.res.ask != raGenConfirm {
		t.Fatalf("ask %d, want the confirmation first", sc.s.res.ask)
	}
	var d Decision
	sc.s.askOptionsCold1(sc.k, &d)
	if d.Sub != ChGenConfirm || d.N != 2 {
		t.Fatalf("confirmation: sub %d with %d options, want ChGenConfirm yes/no", d.Sub, d.N)
	}
	// yes: the pick (the program is not running, so answer the pick by hand)
	sc.s.res.ask = raNone
	if sc.s.searchPick(sc.k) || sc.s.res.ask != raGenSearch {
		t.Fatalf("after yes: ask %d, want the hand pick", sc.s.res.ask)
	}
	d = Decision{}
	sc.s.resolveOptionsGen(sc.k, &d)
	if d.Sub != ChGenHandMove || d.N != 3 { // two lands and Done
		t.Fatalf("hand pick: sub %d with %d options, want ChGenHandMove over 2 lands + Done", d.Sub, d.N)
	}
	sc.s.resArm().look[0], sc.s.g.nlook = l1, 1
	sc.s.finishSearchNow(sc.k)
	if sc.s.objs[l1].zone != kop.ZoneBattlefield || !sc.tapped(l1) || sc.s.objs[l2].zone != kop.ZoneHand {
		t.Fatalf("after the pick: l1 zone %d tapped %v, l2 zone %d", sc.s.objs[l1].zone, sc.tapped(l1), sc.s.objs[l2].zone)
	}
	// required, one eligible, bound 2: taken unasked
	sc.s.res.pc = 1
	if !sc.s.startSearch(sc.k, &must) || sc.s.objs[l2].zone != kop.ZoneBattlefield {
		t.Fatalf("required move: l2 zone %d, want taken unasked", sc.s.objs[l2].zone)
	}
}

// TestBattlefieldPick: a Hidden$ battlefield pick (OpChooseMove with a
// zoneless filter, gorge effHiddenPick): the candidates are the matching
// permanents of both battlefields (here "creature you control"), offered as
// hidden_pick without Done when mandatory; the pick leaves for its owner's
// hand, remembered with SrRemember.
func TestBattlefieldPick(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(tbl.Filters) - 1)
	pick := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(1), F: f,
		C: int16(kop.ZoneHand) | int16(kop.SrMandatory|kop.SrRemember)<<3}
	bear, src := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Mover", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{pick}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a, b := sc.perm(bear, 0), sc.perm(bear, 0)
	sc.perm(bear, 1)
	sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc = 0, src, 0, 0
	sc.s.res.e.obj = NoObj
	if sc.s.startSearch(sc.k, &pick) || sc.s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the hidden pick", sc.s.res.ask)
	}
	var d Decision
	sc.s.resolveOptionsGen(sc.k, &d)
	if d.Sub != ChGenHiddenPick || d.N != 2 {
		t.Fatalf("pick: sub %d with %d options, want ChGenHiddenPick over the two own creatures, no Done", d.Sub, d.N)
	}
	sc.s.resArm().look[0], sc.s.g.nlook = b, 1
	sc.s.finishSearchNow(sc.k)
	if sc.s.objs[b].zone != kop.ZoneHand || sc.s.objs[a].zone != kop.ZoneBattlefield {
		t.Fatalf("after the pick: b zone %d, a zone %d", sc.s.objs[b].zone, sc.s.objs[a].zone)
	}
	if sc.s.g.nrem != 1 || sc.s.g.rem[0] != b {
		t.Fatalf("remembered %v, want the returned creature", sc.s.g.rem[:sc.s.g.nrem])
	}
}

// TestHandMoveChooserYou: a hand move over the opponent's hand with
// SrChooserYou (gorge Chooser$ You) is asked of the resolving controller,
// over the opponent's cards; the pick goes to exile.
func TestHandMoveChooserYou(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	pl := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneHand})
	f := kop.FilterID(len(tbl.Filters) - 1)
	mv := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefOpponent, A: kop.Const(1), F: f,
		C: int16(kop.ZoneExile) | int16(kop.SrMandatory|kop.SrChooserYou)<<3}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Mover", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{mv}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o1, _ := sc.hand(pl, 1), sc.hand(pl, 1)
	sc.hand(pl, 0)
	sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc = 0, src, 0, 0
	if sc.s.startSearch(sc.k, &mv) || sc.s.res.ask != raGenSearch || sc.s.res.askP != 0 {
		t.Fatalf("ask %d of player %d, want the hand pick asked of the controller", sc.s.res.ask, sc.s.res.askP)
	}
	var d Decision
	sc.s.resolveOptionsGen(sc.k, &d)
	if d.Sub != ChGenHandMove || d.N != 2 {
		t.Fatalf("pick: sub %d with %d options, want the opponent's two cards, no Done", d.Sub, d.N)
	}
	sc.s.resArm().look[0], sc.s.g.nlook = o1, 1
	sc.s.finishSearchNow(sc.k)
	if sc.s.objs[o1].zone != kop.ZoneExile {
		t.Fatalf("picked card zone %d, want exile", sc.s.objs[o1].zone)
	}
}

// TestRevealPick: a SrReveal choose-move (gorge effReveal's reveal_pick)
// asks over the hand's matching cards as ChGenReveal, moves nothing and
// remembers the pick; with no matching card its Optional$ ask is skipped.
func TestRevealPick(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	pl := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Zone: kop.ZoneHand}, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneHand})
	lands, crea := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	rv := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(1), F: lands,
		C: int16(kop.ZoneNone) | int16(kop.SrMandatory|kop.SrRemember|kop.SrReveal)<<3}
	opt := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(1), F: crea,
		C: int16(kop.ZoneNone) | int16(kop.SrMandatory|kop.SrConfirm|kop.SrReveal)<<3}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Revealer", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{rv, opt}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	l1, l2 := sc.hand(pl, 0), sc.hand(pl, 0)
	sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc = 0, src, 0, 0
	if sc.s.startSearch(sc.k, &rv) || sc.s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the reveal pick", sc.s.res.ask)
	}
	var d Decision
	sc.s.resolveOptionsGen(sc.k, &d)
	if d.Sub != ChGenReveal || d.N != 2 {
		t.Fatalf("reveal pick: sub %d with %d options, want ChGenReveal over 2 lands, no Done", d.Sub, d.N)
	}
	sc.s.resArm().look[0], sc.s.g.nlook = l2, 1
	sc.s.finishSearchNow(sc.k)
	if sc.s.objs[l1].zone != kop.ZoneHand || sc.s.objs[l2].zone != kop.ZoneHand || sc.s.g.nrem != 1 || sc.s.g.rem[0] != l2 {
		t.Fatal("the reveal moved a card or did not remember the pick")
	}
	sc.s.res.pc = 1
	if !sc.s.startSearch(sc.k, &opt) || sc.s.res.ask == raGenConfirm {
		t.Fatal("an optional reveal with no creature in hand asked")
	}
}

// TestHandMoveToLibrary: a hand move to the library (gorge
// handLibraryTail, LibraryPosition$ 0) puts the card on top.
func TestHandMoveToLibrary(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	pl := CardID(255)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			pl = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneHand})
	f := kop.FilterID(len(tbl.Filters) - 1)
	mv := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefOpponent, A: kop.Const(1), F: f,
		C: int16(kop.ZoneLibrary) | int16(kop.SrMandatory)<<3}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Imp", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{mv}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.hand(pl, 1)
	sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc = 0, src, 0, 0
	if !sc.s.startSearch(sc.k, &mv) {
		t.Fatal("a required move of the only card asked")
	}
	p := &sc.s.pl[1]
	if sc.s.objs[o].zone != kop.ZoneLibrary || p.lib[p.libLo] != o {
		t.Fatal("the card is not on top of its owner's library")
	}
}

// TestBattlefieldPickOwnZone: with SrOwnZone a battlefield pick offers only
// player Obj's permanents and asks that player (gorge effHiddenPick with
// DefinedPlayer$ Targeted, Chooser$ Targeted: "target opponent exiles a
// creature they control").
func TestBattlefieldPickOwnZone(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	f := kop.FilterID(len(tbl.Filters) - 1)
	pick := kop.Instr{Op: kop.OpChooseMove, Obj: kop.RefOpponent, A: kop.Const(1), F: f,
		C: int16(kop.ZoneExile) | int16(kop.SrMandatory|kop.SrOwnZone)<<3}
	bear, src := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Debt", Types: kop.TInstant, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{pick}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(bear, 0)
	b1, b2 := sc.perm(bear, 1), sc.perm(bear, 1)
	sc.s.res.e.ctrl, sc.s.res.e.card, sc.s.res.e.ab, sc.s.res.pc, sc.s.res.e.obj = 0, src, 0, 0, NoObj
	if sc.s.startSearch(sc.k, &pick) || sc.s.res.ask != raGenSearch || sc.s.res.askP != 1 {
		t.Fatalf("ask %d of %d, want the hidden pick asked of the opponent", sc.s.res.ask, sc.s.res.askP)
	}
	var d Decision
	sc.s.resolveOptionsGen(sc.k, &d)
	if d.N != 2 {
		t.Fatalf("%d options, want the opponent's two creatures only", d.N)
	}
	sc.s.resArm().look[0], sc.s.g.nlook = b2, 1
	sc.s.finishSearchNow(sc.k)
	if sc.s.objs[b2].zone != kop.ZoneExile || sc.s.objs[b1].zone != kop.ZoneBattlefield {
		t.Fatal("the picked creature was not exiled alone")
	}
}
