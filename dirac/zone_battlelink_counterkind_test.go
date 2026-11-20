package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

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
