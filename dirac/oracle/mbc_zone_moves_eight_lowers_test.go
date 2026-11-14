package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC zone-moves-8 lowerings, one fixture pool per spelling, each played
// in lockstep against gorge:
//
//   - mbclibgrave: Dreadhound's "whenever ... a creature card is put into a
//     graveyard from a library" (ChangesZone Library->Graveyard), a
//     TrigToGraveyard line From Library, fired by its own enters mill and by
//     Thought Scour on either library;
//   - mbcgoblank: Go Blank's "Then exile all cards from that player's
//     graveyard" (ChangeZoneAll Defined$ ParentTarget), the sweep scoped to
//     the discard's target player;
//   - mbcwumpus: Hunted Wumpus's "each other player may put a creature card
//     from their hand onto the battlefield" (hand move DefinedPlayer$
//     Player.Other), a RepeatEach over the opponents; Yarok's Fenlurker's
//     mandatory "each opponent exiles a card from their hand" (Player.Opponent)
//     on the other side is the same loop's required arm.
//
// Each case asserts the lowered shape is in the table, plays clean, and then
// diverges with that shape broken (the pool reaches what it tests).
type mbcZoneMoves8Case struct {
	pool  string
	decks [2][]cardset.Entry
	// shape reports the lowered shape in the table, and mutate breaks it
	shape  func(t *testing.T, tbl *kop.Table)
	mutate func(tbl *kop.Table)
}

var mbcZoneMoves8Cases = []mbcZoneMoves8Case{
	{
		pool: "mbclibgrave",
		decks: [2][]cardset.Entry{
			{
				{Name: "Dreadhound", Count: 8}, {Name: "Thought Scour", Count: 8},
				{Name: "Walking Corpse", Count: 8}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 4},
			},
			{
				{Name: "Raging Goblin", Count: 12}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 24},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if !mbcCardIs(tbl, "Walking Corpse", kop.TCreature) {
				t.Fatal("Walking Corpse is not a creature in the table: a milled card never matches the watcher's filter")
			}
			if mbcLibGraveLine(tbl) == nil {
				t.Fatal("Dreadhound has no TrigToGraveyard line From Library: the spelling did not lower")
			}
		},
		mutate: func(tbl *kop.Table) { mbcLibGraveLine(tbl).Trig.From = kop.ZoneNone },
	},
	{
		pool: "mbcgoblank",
		decks: [2][]cardset.Entry{
			{
				{Name: "Go Blank", Count: 8}, {Name: "Thought Scour", Count: 8},
				{Name: "Walking Corpse", Count: 6}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 6},
			},
			{
				{Name: "Raging Goblin", Count: 12}, {Name: "Shock", Count: 6}, {Name: "Mountain", Count: 22},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if mbcGoBlankSweep(tbl) == nil {
				t.Fatal("Go Blank has no ChangeZoneAllEx scoped to target 0: the spelling did not lower")
			}
		},
		mutate: func(tbl *kop.Table) { mbcGoBlankSweep(tbl).Obj = kop.RefNone },
	},
	{
		pool: "mbcwumpus",
		decks: [2][]cardset.Entry{
			{
				{Name: "Hunted Wumpus", Count: 10}, {Name: "Grizzly Bears", Count: 10}, {Name: "Forest", Count: 20},
			},
			{
				{Name: "Raging Goblin", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Yarok's Fenlurker", Count: 8},
				{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 8},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if mbcCard(tbl, "Yarok's Fenlurker") == nil {
				t.Fatal("Yarok's Fenlurker is not in the mbcwumpus table")
			}
			if mbcWumpusLoop(tbl) == nil {
				t.Fatal("Hunted Wumpus has no RepeatEachPlayer over the opponents around a RefEach ChooseMove: the spelling did not lower")
			}
		},
		// every player, the controller too: gorge never asks the controller
		mutate: func(tbl *kop.Table) { mbcWumpusLoop(tbl).Obj = kop.RefEachPlayer },
	},
}

func TestMBCZoneMoves8Lowers(t *testing.T) {
	t.Parallel()
	_ = corpusDirOrSkip(t)
	for _, c := range mbcZoneMoves8Cases {
		t.Run(c.pool, func(t *testing.T) {
			t.Parallel()
			set, err := cardset.RawByName(c.pool)
			if errors.Is(err, cardset.ErrNoCorpus) {
				t.Skip("no kernel-synth corpus table: " + err.Error())
			}
			if err != nil {
				t.Fatal(err)
			}
			tbl, _, _, err := set.Matchup(c.decks[0], c.decks[1])
			if err != nil {
				t.Fatal(err)
			}
			c.shape(t, tbl)
			lockstepSet(t, c.pool, c.decks, 200, "KMBCZONEMOVES8_SEEDS")
			if lockstepRun(t, c.pool, c.decks, 200, false, c.mutate) == 0 {
				t.Fatalf("%s: no seed diverged with the lowered shape broken: the pool never reaches it", c.pool)
			}
		})
	}
}

func mbcCard(tbl *kop.Table, name string) *kop.CardDef {
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == name {
			return &tbl.Cards[i]
		}
	}
	return nil
}

func mbcCardIs(tbl *kop.Table, name string, ty kop.TypeMask) bool {
	cd := mbcCard(tbl, name)
	return cd != nil && cd.Types&ty != 0
}

// mbcLibGraveLine is Dreadhound's library-to-graveyard watcher.
func mbcLibGraveLine(tbl *kop.Table) *kop.Ability {
	cd := mbcCard(tbl, "Dreadhound")
	if cd == nil {
		return nil
	}
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigToGraveyard && a.Trig.From == kop.ZoneLibrary {
			return a
		}
	}
	return nil
}

// mbcGoBlankSweep is Go Blank's graveyard exile, scoped to its target.
func mbcGoBlankSweep(tbl *kop.Table) *kop.Instr {
	cd := mbcCard(tbl, "Go Blank")
	if cd == nil {
		return nil
	}
	for j := range cd.Abilities {
		prog := cd.Abilities[j].Prog
		for k := range prog {
			if prog[k].Op == kop.OpChangeZoneAllEx && prog[k].Obj == kop.RefTarget0 {
				return &prog[k]
			}
		}
	}
	return nil
}

// mbcWumpusLoop is Hunted Wumpus's per-opponent loop head, its body the
// repeated player's hand move.
func mbcWumpusLoop(tbl *kop.Table) *kop.Instr {
	cd := mbcCard(tbl, "Hunted Wumpus")
	if cd == nil {
		return nil
	}
	for j := range cd.Abilities {
		prog := cd.Abilities[j].Prog
		for k := 0; k+1 < len(prog); k++ {
			if prog[k].Op == kop.OpRepeatEachPlayer && prog[k].Obj == kop.RefEachOpponent &&
				prog[k+1].Op == kop.OpChooseMove && prog[k+1].Obj == kop.RefEach {
				return &prog[k]
			}
		}
	}
	return nil
}
