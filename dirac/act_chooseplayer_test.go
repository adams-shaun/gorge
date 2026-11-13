package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC chosen-player-card-6 kernel op (act_chooseplayer.go): gorge
// effChoosePlayer's Random$ True branch over Choices$
// Player.Opponent+!EnchantedBy (Maddening Hex). The probe is the
// cursecurses pool's Archnemesis (an Enchant:Opponent aura, the same
// destination shape) whose program is replaced by the lowered Maddening Hex
// chain: OpChoosePlayer then the Attach to the chosen player (the same two
// instructions kgen emits for the card — see dirac/kgen's
// TestChoosePlayerRandomLowers).
func choosePlayerProbe(t *testing.T) (*scen, Slot, []kop.Instr) {
	t.Helper()
	set := cardsettest.Set(t, "cursecurses")
	deck := []cardset.Entry{{Name: "Archnemesis", Count: 4}, {Name: "Island", Count: 36}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cd := tbl.Cards[id["Archnemesis"]]
	cd.Abilities = nil
	tbl.Cards[id["Archnemesis"]] = cd
	sc := newScenTableLib(t, tbl, id["Island"], Config{})
	o := sc.perm(id["Archnemesis"], 0) // the resolving controller is seat 0
	prog := []kop.Instr{
		{Op: kop.OpChoosePlayer, Obj: kop.RefController, C: kop.ChPlRandom},
		{Op: kop.OpAttach, W: kop.RefChosenPlayer},
	}
	return sc, sc.s.objs[o].slot, prog
}

// runChoosePlayer plays the probe's program from pc 0 to the end.
func runChoosePlayer(sc *scen, prog []kop.Instr) bool {
	s := sc.s
	for ; int(s.res.pc) < len(prog); s.res.pc++ {
		if !s.exec(sc.k, &prog[s.res.pc]) {
			return false
		}
	}
	return true
}

// TestChoosePlayerOpEmptyPool: the aura attached to the one opponent leaves
// no candidate (gorge MatchesPlayerSpecWithSVars' !EnchantedBy excludes it),
// so the pick draws nothing (gorge randomChoices clamps to the pool) and
// records no choice: the Attach resolves to no player and attaches nothing.
func TestChoosePlayerOpEmptyPool(t *testing.T) {
	sc, sl, prog := choosePlayerProbe(t)
	s := sc.s
	s.attachPlayer(sc.k, sl, 1)
	draws := s.rng.Draws
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: s.perms[sl].obj, srcVer: s.objs[s.perms[sl].obj].ver,
		card: s.objs[s.perms[sl].obj].card, ab: 0, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	if !runChoosePlayer(sc, prog) {
		t.Fatalf("the program stopped at an ask %d, want it to run to the end (the random pick poses none)", s.res.ask)
	}
	if s.res.ask != raNone {
		t.Errorf("ask %d, want none", s.res.ask)
	}
	if s.res.chosenPl != 255 {
		t.Errorf("chosenPl %d, want 255 (no pick recorded)", s.res.chosenPl)
	}
	if s.rng.Draws != draws {
		t.Errorf("the empty pool drew %d times, want none", s.rng.Draws-draws)
	}
	if s.atchSeat(sc.k, sl) != 2 {
		t.Errorf("the aura is on seat %d, want it unmoved (seat 1)", s.atchSeat(sc.k, sl))
	}
}

// TestChoosePlayerOpRandomPick: the aura attached to its own controller
// leaves the opponent the only candidate; the pick is the engine rng's, and
// the Attach moves the aura to it (gorge effAttach's EnchantSelf walk over
// the chosen player).
func TestChoosePlayerOpRandomPick(t *testing.T) {
	sc, sl, prog := choosePlayerProbe(t)
	s := sc.s
	s.attachPlayer(sc.k, sl, 0)
	draws := s.rng.Draws
	o := s.perms[sl].obj
	s.res = resCtx{e: stackEntry{kind: seAbility, obj: o, srcVer: s.objs[o].ver,
		card: s.objs[o].card, ab: 0, ctrl: 0, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	if !runChoosePlayer(sc, prog) {
		t.Fatalf("the program stopped at an ask %d, want it to run to the end (the random pick poses none)", s.res.ask)
	}
	if s.res.ask != raNone {
		t.Errorf("ask %d, want none", s.res.ask)
	}
	if s.rng.Draws != draws+1 {
		t.Errorf("%d rng draws, want one (gorge randomChoices picks one of one)", s.rng.Draws-draws)
	}
	if s.res.chosenPl != 1 {
		t.Errorf("chosenPl %d, want the one candidate (seat 1)", s.res.chosenPl)
	}
	if s.atchSeat(sc.k, sl) != 2 {
		t.Errorf("the aura is on seat %d, want it attached to the pick (seat 1)", s.atchSeat(sc.k, sl))
	}
}
