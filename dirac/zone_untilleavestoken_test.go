package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// untilLeavesScen is a source creature whose one program is the until-leaves
// exile the Banisher Priest shape lowers (OpExileUntilLeaves over Target0):
// the targeted battlefield permanent goes to exile until the source leaves.
func untilLeavesScen(t *testing.T) (sc *scen, card CardID, src, mine Obj) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{} // a generated-table extension, as every real pool has
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test UntilLeaves", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpExileUntilLeaves, Obj: kop.RefTarget0},
		}}},
	})
	card = CardID(len(tbl.Cards) - 1)
	sc = newScenTable(t, tbl, Config{NoElide: true})
	src = sc.perm(card, 0)
	mine = sc.perm(v.GrizzlyBears, 0)
	if !sc.k.gen.anyLinkRet {
		t.Fatal("the table has no until-leaves program")
	}
	return sc, card, src, mine
}

// runTarget puts a resolving trigger of src on the interpreter aimed at o.
func runTarget(sc *scen, card CardID, src, o Obj) {
	s := sc.s
	s.res = resCtx{e: stackEntry{kind: seTrigger, obj: src, srcVer: s.objs[src].ver, card: card, ctrl: 0},
		active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e.tgt[0] = TCode(o)
	s.runProg(sc.k)
}

// TestExileUntilLeavesSkipsTokens: a token exiled by OpExileUntilLeaves is not
// linked (gorge recordExileReturnFor skips o.IsToken: the token ceases when it
// leaves the battlefield and never returns, CR 111.7). It still sits in exile
// here, so the zone test alone would link it and the source's departure would
// put the ceased token back on the battlefield.
func TestExileUntilLeavesSkipsTokens(t *testing.T) {
	sc, card, src, _ := untilLeavesScen(t)
	s := sc.s
	tok := s.newToken(v.GrizzlyBears, 0)
	s.enter(sc.k, tok, 0, false, kop.ZoneNone)
	s.npend = 0
	if s.objs[tok].of&ofToken == 0 || !sc.onBF(tok) {
		t.Fatalf("precondition: object %d of %#x is not a battlefield token", tok, s.objs[tok].of)
	}
	runTarget(sc, card, src, tok)
	if s.Stalled() {
		t.Fatalf("the exile stalled: %s", StallName(s.StallReason()))
	}
	if z := s.objs[tok].zone; z != kop.ZoneExile {
		t.Fatalf("the token is in zone %d, want exile", z)
	}
	if s.g.nlinks != 0 {
		t.Fatalf("%d link(s) from a token exile (%v), want none", s.g.nlinks, s.g.links[:s.g.nlinks])
	}

	var m slotMask
	m.set(sc.slot(src))
	s.leaveAll(sc.k, m, kop.ZoneGraveyard)
	if s.Stalled() {
		t.Fatalf("the departure stalled: %s", StallName(s.StallReason()))
	}
	if sc.onBF(tok) {
		t.Fatalf("the ceased token %d returned to the battlefield", tok)
	}
	if s.g.nlinks != 0 || s.g.nretHeld != 0 || s.g.batchLatch&batchHeld != 0 || s.g.held != 0 {
		t.Errorf("after the departure: nlinks %d nretHeld %d latch %#x held %#x, want the frame closed and empty",
			s.g.nlinks, s.g.nretHeld, s.g.batchLatch, s.g.held)
	}
}

// TestExileUntilLeavesLinksAndReturns: the same program exiling a non-token
// card still links it, and the card returns to the battlefield under its
// owner's control when the source leaves.
func TestExileUntilLeavesLinksAndReturns(t *testing.T) {
	sc, card, src, mine := untilLeavesScen(t)
	s := sc.s
	runTarget(sc, card, src, mine)
	if s.Stalled() {
		t.Fatalf("the exile stalled: %s", StallName(s.StallReason()))
	}
	if z := s.objs[mine].zone; z != kop.ZoneExile {
		t.Fatalf("the card is in zone %d, want exile", z)
	}
	if s.g.nlinks != 1 || s.g.links[0].src != src || s.g.links[0].obj != mine || s.g.links[0].sver != s.objs[src].ver {
		t.Fatalf("links %v (n=%d), want the source's link to card %d", s.g.links[:s.g.nlinks], s.g.nlinks, mine)
	}

	var m slotMask
	m.set(sc.slot(src))
	s.leaveAll(sc.k, m, kop.ZoneGraveyard)
	if s.Stalled() {
		t.Fatalf("the return stalled: %s", StallName(s.StallReason()))
	}
	if !sc.onBF(mine) {
		t.Fatalf("the linked card is in zone %d, want back on the battlefield", s.objs[mine].zone)
	}
	if got := s.perms[sc.slot(mine)].ctrl; got != 0 {
		t.Errorf("the card returned under seat %d, want its owner 0", got)
	}
	if s.g.nlinks != 0 || s.g.nretHeld != 0 || s.g.batchLatch&batchHeld != 0 || s.g.held != 0 {
		t.Errorf("after the return: nlinks %d nretHeld %d latch %#x held %#x, want the frame closed and empty",
			s.g.nlinks, s.g.nretHeld, s.g.batchLatch, s.g.held)
	}
}
