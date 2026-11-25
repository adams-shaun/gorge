package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEffectEOTEmblemSourceCaptureToPlayer: a source-captured until-end-of-turn
// DamageDone emblem whose line reads ValidTarget$ Player,Planeswalker (Hunter's
// Insight) lowers to TfToPlayer beside a Filter2 holding the planeswalker
// clause (kgen build_ab.go), with no FPlayer flag. The recipient-capture test
// (embRecipientCapture) must not misread that Filter2 as the recipient capture:
// the emblem fires for its captured source's damage to a player.
func TestEffectEOTEmblemSourceCaptureToPlayer(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TPlaneswalker, Zone: kop.ZoneBattlefield})
	f2 := kop.FilterID(len(tbl.Filters) - 1)
	line := kop.Ability{Kind: kop.AbTriggered, AFlags: kop.AfLatent, Trig: kop.Trigger{Event: kop.TrigDamageDone, Zone: kop.ZoneCommand, Flags: kop.TfToPlayer, Filter2: f2}, Prog: gain}
	tbl.Emblems = append(tbl.Emblems, kop.Emblem{UntilEOT: true, Abilities: []kop.Ability{line}, Capture: kop.RefTarget0})
	emb := uint8(len(tbl.Emblems))
	spell, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Instant", Types: kop.TInstant, Abilities: []kop.Ability{line}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || sc.k.gen.embCombatEv&(1<<kop.TrigDamageDone) == 0 {
		t.Fatal("precondition: the table's emblem does not register the DamageDone hook")
	}
	srcObj := sc.perm(bear, 0)
	other := sc.perm(bear, 0)
	hurt := sc.perm(bear, 1)
	if srcObj == other || !sc.onBF(srcObj) || !sc.onBF(other) || !sc.onBF(hurt) {
		t.Fatal("precondition: three distinct creatures on the battlefield")
	}
	arm := func(num objWord, rcp TCode, toPlayer bool) int {
		sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0] = emb, spell, srcObj
		sc.s.g.nemb[0] = 1
		sc.s.embSetNum(0, 0, num)
		sc.s.npend = 0
		var latch dmgLatch
		sc.s.fireEmblemsDmg(sc.k, sc.slot(srcObj), 2, toPlayer, false, rcp, false, &latch, nil)
		return int(sc.s.npend)
	}
	if n := arm(objWord(srcObj)+1, PlayerCode(1), true); n != 1 {
		t.Errorf("source capture, the remembered source damages a player: %d triggers, want 1", n)
	}
	if n := arm(objWord(other)+1, PlayerCode(1), true); n != 0 {
		t.Errorf("source capture, another source damages a player: %d triggers, want 0", n)
	}
}
