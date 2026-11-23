package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// battleLinkScen is a source creature whose one program is the battlefield
// pick the Duration$ UntilHostLeavesPlay lowering emits (Glorious Protector,
// Abdel Adrian): any two other creatures, from every seat's battlefield, exiled
// until the source leaves (kop.SrBattleLink).
func battleLinkScen(t *testing.T, link bool) (sc *scen, src, mine, theirs Obj) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{} // a generated-table extension, as every real pool has
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Flags: kop.FOther})
	f := kop.FilterID(len(tbl.Filters) - 1)
	b := kop.Const(0)
	if link {
		b = kop.Const(kop.SrBattleLink)
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Battle Link", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(2), B: b, F: f,
				C: int16(kop.ZoneExile) | int16(kop.SrRemember)<<3},
		}}},
	})
	card := CardID(len(tbl.Cards) - 1)
	sc = newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	src = sc.perm(card, 0)
	mine = sc.perm(v.GrizzlyBears, 0)
	theirs = sc.perm(v.HillGiant, 1)
	s.res = resCtx{e: stackEntry{kind: seTrigger, obj: src, srcVer: s.objs[src].ver, card: card, ctrl: 0}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.ctl = ctlResolve
	return sc, src, mine, theirs
}

// answerPick takes the pick's decision, answering with the offered card o.
func answerPick(t *testing.T, s *State, o Obj) {
	t.Helper()
	d := s.Step()
	if d.Sub != ChGenHiddenPick {
		t.Fatalf("decision kind=%d sub=%d, want the hidden pick", d.Kind, d.Sub)
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(uint8(o)) {
			s.Apply(d.Options[i])
			return
		}
	}
	t.Fatalf("object %d not offered among %d options", o, d.N)
}

// TestBattlefieldPickUntilLeavesReturnsAsOneBatch: two permanents picked off
// the battlefield with SrBattleLink are linked to the source (one link each,
// in pick order), and when the source leaves they re-enter the battlefield
// under their OWNERS' control as one batch -- the departure frame holds the
// enters checks until the last is back. Without the bit the pick exiles for
// good. A source that has already left moves nothing (gorge
// exileHostGoneFor), and asks nothing.
func TestBattlefieldPickUntilLeavesReturnsAsOneBatch(t *testing.T) {
	sc, src, mine, theirs := battleLinkScen(t, true)
	s := sc.s
	if !sc.k.gen.anyLinkRet {
		t.Fatal("the table's departure frames do not hold: tableLinkRet missed the battlefield pick")
	}
	s.runProg(sc.k)
	if s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the hidden pick", s.res.ask)
	}
	answerPick(t, s, theirs)
	answerPick(t, s, mine)
	if s.Stalled() {
		t.Fatalf("the pick stalled: %s", StallName(s.StallReason()))
	}
	if s.objs[mine].zone != kop.ZoneExile || s.objs[theirs].zone != kop.ZoneExile {
		t.Fatalf("picked cards in zones %v/%v, want exile", s.objs[mine].zone, s.objs[theirs].zone)
	}
	if s.g.nlinks != 2 || s.g.links[0].src != src || s.g.links[0].obj != theirs || s.g.links[1].src != src || s.g.links[1].obj != mine {
		t.Fatalf("links %v (n=%d), want the source's two in pick order", s.g.links[:s.g.nlinks], s.g.nlinks)
	}
	if s.g.linkHand != 0 {
		t.Fatalf("linkHand %#x: a battlefield link returns to the battlefield, not a hand", s.g.linkHand)
	}

	var m slotMask
	m.set(sc.slot(src))
	s.leaveAll(sc.k, m, kop.ZoneGraveyard)
	if s.Stalled() {
		t.Fatalf("the return stalled: %s", StallName(s.StallReason()))
	}
	for _, c := range []struct {
		o     Obj
		owner uint8
	}{{mine, 0}, {theirs, 1}} {
		if !sc.onBF(c.o) {
			t.Fatalf("object %d is in zone %v, want back on the battlefield", c.o, s.objs[c.o].zone)
		}
		if got := s.perms[sc.slot(c.o)].ctrl; got != c.owner {
			t.Errorf("object %d returned under seat %d, want its owner %d", c.o, got, c.owner)
		}
	}
	if s.g.nlinks != 0 || s.g.nretHeld != 0 || s.g.batchLatch&batchHeld != 0 || s.g.held != 0 {
		t.Errorf("after the return: nlinks %d nretHeld %d latch %#x held %#x, want the frame closed and empty", s.g.nlinks, s.g.nretHeld, s.g.batchLatch, s.g.held)
	}
	if a, b := s.perms[sc.slot(theirs)].ts, s.perms[sc.slot(mine)].ts; b != a+1 {
		t.Errorf("entry stamps %d then %d: the returns are not consecutive entries of one batch, in link order", a, b)
	}

	t.Run("host gone moves nothing", func(t *testing.T) {
		sc, src, mine, theirs := battleLinkScen(t, true)
		s := sc.s
		var m slotMask
		m.set(sc.slot(src))
		s.leaveAll(sc.k, m, kop.ZoneGraveyard)
		s.res.e.srcVer = s.objs[src].ver - 1 // the ability was put on the stack by the old incarnation
		s.runProg(sc.k)
		if s.res.ask == raGenSearch || !sc.onBF(mine) || !sc.onBF(theirs) || s.g.nlinks != 0 {
			t.Fatalf("host gone: ask %d, mine on battlefield %v, theirs %v, links %d; want no pick and nothing exiled", s.res.ask, sc.onBF(mine), sc.onBF(theirs), s.g.nlinks)
		}
	})

	t.Run("without the bit the exile is permanent", func(t *testing.T) {
		sc, src, mine, theirs := battleLinkScen(t, false)
		s := sc.s
		s.runProg(sc.k)
		answerPick(t, s, theirs)
		answerPick(t, s, mine)
		if s.g.nlinks != 0 {
			t.Fatalf("%d links without SrBattleLink", s.g.nlinks)
		}
		var m slotMask
		m.set(sc.slot(src))
		s.leaveAll(sc.k, m, kop.ZoneGraveyard)
		if s.objs[mine].zone != kop.ZoneExile || s.objs[theirs].zone != kop.ZoneExile {
			t.Fatalf("zones %v/%v, want both still exiled", s.objs[mine].zone, s.objs[theirs].zone)
		}
	})
}

// TestGraveyardPickCounterKindIsNotABattleLink: bit 9 of an OpChooseMove's B is
// also the entry counter kind's (kind+1)<<4 field of a graveyard pick, and
// (CtrFinality+1)<<4 sets it. Such a pick, resolving from a spell that is no
// longer on the battlefield (its source is on the stack), is not a battlefield
// link: it still asks and moves the card, and it does not make the table hold
// departure frames (Charnel Serenade, Scavenger's Talent).
func TestGraveyardPickCounterKindIsNotABattleLink(t *testing.T) {
	b := (int(kop.CtrFinality) + 1) << 4
	if b&kop.SrBattleLink == 0 {
		t.Fatalf("B %#x does not set bit 9: the regression no longer collides", b)
	}
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneGraveyard})
	f := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Finality Pick", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpChooseMove, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(b), F: f,
				C: int16(kop.ZoneBattlefield)},
		}}},
	})
	card := CardID(len(tbl.Cards) - 1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	if sc.k.gen.anyLinkRet {
		t.Fatal("tableLinkRet read the finality counter kind as SrBattleLink")
	}
	src := sc.obj(card, 0) // the spell: never on the battlefield
	dead := sc.grave(v.GrizzlyBears, 0, 1)[0]
	if s.objs[src].zone == kop.ZoneBattlefield {
		t.Fatal("precondition: the source must be off the battlefield")
	}
	s.res = resCtx{e: stackEntry{kind: seSpell, obj: src, srcVer: s.objs[src].ver, card: card, ctrl: 0}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.ctl = ctlResolve
	if !s.handExileHostGone() {
		t.Fatal("precondition: the host-gone check is true, so a misread bit would skip the pick")
	}
	s.runProg(sc.k)
	if s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the graveyard pick (a battlefield-link misread skips it)", s.res.ask)
	}
	answerPick(t, s, dead)
	if !sc.onBF(dead) {
		t.Fatalf("picked card is in zone %v, want the battlefield", s.objs[dead].zone)
	}
	if s.g.nlinks != 0 {
		t.Fatalf("%d links from a graveyard pick", s.g.nlinks)
	}
}

// TestBattlefieldPickUntilLeavesSkipsTokens: a picked token is exiled but not
// linked (gorge recordExileReturnFor skips o.IsToken: the token ceases and
// never returns). In a generated-table game the token sits in exile until the
// next state-based check, so the zone test alone would link it and the return
// trip would put the ceased token back on the battlefield when the source
// leaves.
func TestBattlefieldPickUntilLeavesSkipsTokens(t *testing.T) {
	sc, src, mine, _ := battleLinkScen(t, true)
	s := sc.s
	tok := s.newToken(v.GrizzlyBears, 0)
	s.enter(sc.k, tok, 0, false, kop.ZoneNone)
	s.npend = 0
	if s.objs[tok].of&ofToken == 0 || !sc.onBF(tok) {
		t.Fatalf("precondition: object %d of %#x is not a battlefield token", tok, s.objs[tok].of)
	}
	s.runProg(sc.k)
	answerPick(t, s, tok)
	answerPick(t, s, mine)
	if s.Stalled() {
		t.Fatalf("the pick stalled: %s", StallName(s.StallReason()))
	}
	if s.objs[tok].zone != kop.ZoneExile {
		t.Fatalf("the token is in zone %v, want exile before the state-based check", s.objs[tok].zone)
	}
	if s.g.nlinks != 1 || s.g.links[0].obj != mine {
		t.Fatalf("links %v (n=%d), want only the non-token card %d", s.g.links[:s.g.nlinks], s.g.nlinks, mine)
	}

	var m slotMask
	m.set(sc.slot(src))
	s.leaveAll(sc.k, m, kop.ZoneGraveyard)
	if s.Stalled() {
		t.Fatalf("the return stalled: %s", StallName(s.StallReason()))
	}
	if sc.onBF(tok) {
		t.Fatalf("the ceased token %d returned to the battlefield", tok)
	}
	if !sc.onBF(mine) {
		t.Fatalf("the linked card is in zone %v, want back on the battlefield", s.objs[mine].zone)
	}
}

// TestBattlefieldPickUntilLeavesLinkTableFullStalls: an any-number pick can
// exile more permanents than the link table holds (Glorious Protector and
// Abdel Adrian pick any number), and gorge links every non-token card. A full
// table is a declared capacity limit, so it stalls visibly (StallLinks) rather
// than silently dropping the link and orphaning the card in exile.
func TestBattlefieldPickUntilLeavesLinkTableFullStalls(t *testing.T) {
	sc, _, mine, theirs := battleLinkScen(t, true)
	s := sc.s
	s.g.nlinks = uint8(len(s.g.links))
	s.runProg(sc.k)
	answerPick(t, s, theirs)
	answerPick(t, s, mine)
	if !s.Stalled() {
		t.Fatal("the full link table did not stall")
	}
	if got := s.StallReason(); got != StallLinks {
		t.Fatalf("stall %s, want %s", StallName(got), StallName(StallLinks))
	}
	if s.g.nlinks != uint8(len(s.g.links)) {
		t.Fatalf("nlinks %d after a full-table pick, want the table still full (no drop counted as a link)", s.g.nlinks)
	}
	if s.objs[theirs].zone != kop.ZoneExile {
		t.Fatalf("the picked card is in zone %v, want exile", s.objs[theirs].zone)
	}
}
