package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// embPlHexScen is the shield table's vanilla matchup (basic lands and Shock)
// with one hand-built next-turn emblem carrying a plain StatPlayerHexproof
// appended, as kgen lowers Blossoming Calm's "you gain hexproof until your
// next turn", put on player 0.
func embPlHexScen(t *testing.T) *scen {
	t.Helper()
	deck := []cardset.Entry{{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Shock", Count: 1}}
	tbl, da, _, err := cardsettest.Set(t, "shield").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var mountain CardID
	for _, c := range da {
		if tbl.Cards[c].Name == "Mountain" {
			mountain = c
		}
	}
	tbl.Emblems = append(tbl.Emblems, kop.Emblem{UntilNextTurn: true, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatPlayerHexproof}}}})
	var lib [20]CardID
	for i := range lib {
		lib[i] = mountain
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	sc := &scen{s: s, k: s.k()}
	sc.s.g.emb[0][0] = uint8(len(tbl.Emblems))
	sc.s.g.nemb[0] = 1
	return sc
}

// TestEmbPlHexReadsEmblem: a runtime emblem's StatPlayerHexproof gives its
// player hexproof (CR 702.11c): an opponent cannot target them, they can
// target themselves, and the other player is untouched. DebugEmbPlHexOff
// (the emblem ignored) makes the opponent's target legal again.
func TestEmbPlHexReadsEmblem(t *testing.T) {
	sc := embPlHexScen(t)
	if sc.k.gen == nil || !sc.k.gen.anyEmbPlHex {
		t.Fatal("precondition: anyEmbPlHex unset with a StatPlayerHexproof emblem in the table")
	}
	if sc.k.gen.anyRepl&rbPlHex != 0 {
		t.Fatal("precondition: rbPlHex set; the emblem must be the only hexproof source")
	}
	if sc.s.g.nemb[0] != 1 || sc.s.g.nemb[1] != 0 {
		t.Fatalf("precondition: emblems %d/%d, want player 0 to hold the one emblem", sc.s.g.nemb[0], sc.s.g.nemb[1])
	}
	if !sc.s.playerHexproof(sc.k, 0) {
		t.Fatal("player 0 holds the hexproof emblem but is not hexproof")
	}
	if sc.s.playerHexproof(sc.k, 1) {
		t.Fatal("player 1 has no emblem but is hexproof")
	}
	spec := kop.TargetSpec{Kind: kop.TgtAny}
	if sc.s.targetOK(sc.k, &spec, 1, NoSlot, NoObj, PlayerCode(0)) {
		t.Fatal("an opponent (player 1) can target the hexproof player 0")
	}
	if !sc.s.targetOK(sc.k, &spec, 0, NoSlot, NoObj, PlayerCode(0)) {
		t.Fatal("player 0 cannot target themselves through their own hexproof")
	}
	if !sc.s.targetOK(sc.k, &spec, 0, NoSlot, NoObj, PlayerCode(1)) {
		t.Fatal("player 0 cannot target player 1, who has no hexproof")
	}
	DebugEmbPlHexOff = true
	defer func() { DebugEmbPlHexOff = false }()
	if !sc.s.targetOK(sc.k, &spec, 1, NoSlot, NoObj, PlayerCode(0)) {
		t.Fatal("DebugEmbPlHexOff: the opponent's target is still illegal")
	}
}
