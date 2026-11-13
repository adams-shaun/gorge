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
//     Dimir Machinations, Bamboozle);
//   - mbcmindclaw: Mindclaw Shaman's free cast of an instant or sorcery from
//     the target opponent's revealed hand (Play Valid$
//     Card.TargetedPlayerCtrl ValidZone$ Hand): CastFree over the player
//     target's hand.
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
				{Name: "Ransack", Count: 6}, {Name: "Cruel Fate", Count: 4}, {Name: "Diabolic Vision", Count: 6},
				{Name: "Dimir Machinations", Count: 4}, {Name: "Bamboozle", Count: 4},
				{Name: "Island", Count: 10}, {Name: "Swamp", Count: 6},
			},
			{
				{Name: "Raging Goblin", Count: 12}, {Name: "Hill Giant", Count: 6}, {Name: "Mountain", Count: 22},
			},
		},
		shape: func(t *testing.T, tbl *kop.Table) {
			for _, name := range []string{"Ransack", "Cruel Fate", "Diabolic Vision", "Dimir Machinations", "Bamboozle"} {
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
