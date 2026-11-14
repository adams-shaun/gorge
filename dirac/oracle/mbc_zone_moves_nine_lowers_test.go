package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The MBC zone-moves-9 lowerings, one fixture pool per spelling, each played
// in lockstep against gorge:
//
//   - mbcspinmyth: Spin into Myth's fateseal 2 (Dig LibraryPosition2$ 0
//     with two cards that can stay on top): the caster orders the cards not
//     put on the bottom back on top (gorge digTopOrderAsk), the kernel's
//     kop.DigRestTopAsk;
//   - digtoporder: the same spelling's class over the controller's own
//     library and a target player's (Ransack, Cruel Fate, Diabolic Vision,
//     Dimir Machinations, Bamboozle; Ancestral Knowledge's enters trigger);
//   - mbcmindclaw: Mindclaw Shaman's free cast of an instant or sorcery from
//     the target opponent's revealed hand (Play Valid$
//     Card.TargetedPlayerCtrl ValidZone$ Hand): CastFree over the player
//     target's hand;
//   - mbcghostform: Kaya's Ghostform's "when enchanted permanent dies or is
//     put into exile" (ChangesZone Battlefield->Graveyard,Exile ValidCard$
//     Card.AttachedBy), a Leaves line over the destination set read on the
//     host as it leaves (CR 603.10a), killed by Shock and exiled by Swords
//     to Plowshares;
//   - mbciwamori: Iwamori of the Open Fist's "each opponent may put a
//     legendary creature card from their hand onto the battlefield" (hand
//     move DefinedPlayer$ Player.Opponent), a RepeatEach over the opponents
//     around the repeated player's may-move.
//
// Each case asserts the lowered shape is in the table, plays clean, and then
// diverges with that shape broken (the pool reaches what it tests).
type mbcZoneMoves9Case struct {
	pool  string
	decks [2][]cardset.Entry
	// shape reports the lowered shape in the table, and mutate breaks it
	shape  func(t *testing.T, tbl *kop.Table)
	mutate func(tbl *kop.Table)
}

var mbcZoneMoves9Cases = []mbcZoneMoves9Case{
	{
		pool: "mbcspinmyth",
		decks: [2][]cardset.Entry{
			{
				{Name: "Spin into Myth", Count: 16}, {Name: "Hill Giant", Count: 4},
				{Name: "Island", Count: 14}, {Name: "Mountain", Count: 6},
			},
			{
				{Name: "Raging Goblin", Count: 12}, {Name: "Hill Giant", Count: 6}, {Name: "Mountain", Count: 22},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if mbcSpinMythDig(tbl) == nil {
				t.Fatal("Spin into Myth has no DigEx with DigRestTopAsk: the spelling did not lower")
			}
		},
		// the rest kept on top in window order: gorge's order ask is not posed
		mutate: func(tbl *kop.Table) { mbcSpinMythDig(tbl).C &^= kop.DigRandomOrder << 3 },
	},
	{
		pool: "digtoporder",
		decks: [2][]cardset.Entry{
			{
				{Name: "Ransack", Count: 5}, {Name: "Cruel Fate", Count: 3}, {Name: "Diabolic Vision", Count: 5},
				{Name: "Dimir Machinations", Count: 3}, {Name: "Bamboozle", Count: 4}, {Name: "Ancestral Knowledge", Count: 4},
				{Name: "Island", Count: 10}, {Name: "Swamp", Count: 6},
			},
			{
				{Name: "Raging Goblin", Count: 12}, {Name: "Hill Giant", Count: 6}, {Name: "Mountain", Count: 22},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			for _, name := range []string{"Ransack", "Cruel Fate", "Diabolic Vision", "Dimir Machinations", "Bamboozle", "Ancestral Knowledge"} {
				if len(digTopAskInstrs(tbl, name)) == 0 {
					t.Fatalf("%s has no DigEx with DigRestTopAsk: the spelling did not lower", name)
				}
			}
		},
		mutate: func(tbl *kop.Table) {
			for i := range tbl.Cards {
				for _, in := range digTopAskInstrs(tbl, tbl.Cards[i].Name) {
					in.C &^= kop.DigRandomOrder << 3
				}
			}
		},
	},
	{
		pool: "mbcmindclaw",
		decks: [2][]cardset.Entry{
			{
				{Name: "Mindclaw Shaman", Count: 12}, {Name: "Mountain", Count: 28},
			},
			{
				{Name: "Shock", Count: 10}, {Name: "Lava Axe", Count: 6}, {Name: "Raging Goblin", Count: 6},
				{Name: "Mountain", Count: 18},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if mbcTgtPlayerCast(tbl, "Mindclaw Shaman") == nil {
				t.Fatal("Mindclaw Shaman has no CastFree over a target slot: the spelling did not lower")
			}
		},
		// the controller's own hand instead of the target's
		mutate: func(tbl *kop.Table) { mbcTgtPlayerCast(tbl, "Mindclaw Shaman").Obj = kop.RefNone },
	},
	{
		pool: "mbcghostform",
		decks: [2][]cardset.Entry{
			{
				{Name: "Kaya's Ghostform", Count: 12}, {Name: "Walking Corpse", Count: 12}, {Name: "Swamp", Count: 16},
			},
			{
				{Name: "Shock", Count: 8}, {Name: "Swords to Plowshares", Count: 8}, {Name: "Raging Goblin", Count: 6},
				{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 8},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if mbcGhostformLine(tbl) == nil {
				t.Fatal("Kaya's Ghostform has no Leaves line over graveyard and exile: the spelling did not lower")
			}
		},
		// the exile half dropped: an exiled host returns in gorge only
		mutate: func(tbl *kop.Table) { mbcGhostformLine(tbl).Trig.N &^= 1 << kop.ZoneExile },
	},
	{
		pool: "mbciwamori",
		decks: [2][]cardset.Entry{
			{
				{Name: "Iwamori of the Open Fist", Count: 12}, {Name: "Forest", Count: 28},
			},
			{
				{Name: "Isamaru, Hound of Konda", Count: 14}, {Name: "Plains", Count: 26},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			if mbcIwamoriLoop(tbl) == nil {
				t.Fatal("Iwamori of the Open Fist has no RepeatEachPlayer over the opponents around a RefEach ChooseMove: the spelling did not lower")
			}
		},
		// every player, the controller too: gorge never asks the controller
		mutate: func(tbl *kop.Table) { mbcIwamoriLoop(tbl).Obj = kop.RefEachPlayer },
	},
}

func TestMBCZoneMoves9Lowers(t *testing.T) {
	t.Parallel()
	_ = corpusDirOrSkip(t)
	for _, c := range mbcZoneMoves9Cases {
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
			lockstepSet(t, c.pool, c.decks, 200, "KMBCZONEMOVES9_SEEDS")
			if lockstepRun(t, c.pool, c.decks, 200, false, c.mutate) == 0 {
				t.Fatalf("%s: no seed diverged with the lowered shape broken: the pool never reaches it", c.pool)
			}
		})
	}
}

// mbcSpinMythDig is Spin into Myth's fateseal: the Dig whose rest goes back
// on top in the caster's order.
func mbcSpinMythDig(tbl *kop.Table) *kop.Instr {
	if ins := digTopAskInstrs(tbl, "Spin into Myth"); len(ins) > 0 {
		return ins[0]
	}
	return nil
}

// digTopAskInstrs are the card's Digs whose rest goes back on top in the
// controller's order (kop.DigRestTopAsk).
func digTopAskInstrs(tbl *kop.Table, name string) []*kop.Instr {
	cd := mbcCard(tbl, name)
	if cd == nil {
		return nil
	}
	var out []*kop.Instr
	for j := range cd.Abilities {
		prog := cd.Abilities[j].Prog
		for k := range prog {
			if prog[k].Op == kop.OpDigEx && (prog[k].C>>3)&kop.DigRestTopAsk == kop.DigRestTopAsk {
				out = append(out, &prog[k])
			}
		}
	}
	return out
}

// mbcTgtPlayerCast is the card's free cast whose population is a target
// slot (the targeted player's hand).
func mbcTgtPlayerCast(tbl *kop.Table, name string) *kop.Instr {
	cd := mbcCard(tbl, name)
	if cd == nil {
		return nil
	}
	for j := range cd.Abilities {
		prog := cd.Abilities[j].Prog
		for k := range prog {
			if prog[k].Op == kop.OpCastFree && prog[k].Obj == kop.RefTarget0 {
				return &prog[k]
			}
		}
	}
	return nil
}

// mbcGhostformLine is Kaya's Ghostform's leave line: a Leaves trigger whose
// destination set holds the graveyard and exile.
func mbcGhostformLine(tbl *kop.Table) *kop.Ability {
	cd := mbcCard(tbl, "Kaya's Ghostform")
	if cd == nil {
		return nil
	}
	const want = 1<<kop.ZoneGraveyard | 1<<kop.ZoneExile
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigLeaves && a.Trig.N&want == want {
			return a
		}
	}
	return nil
}

// mbcIwamoriLoop is Iwamori of the Open Fist's per-opponent loop head, its
// body the repeated player's hand move.
func mbcIwamoriLoop(tbl *kop.Table) *kop.Instr {
	cd := mbcCard(tbl, "Iwamori of the Open Fist")
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
