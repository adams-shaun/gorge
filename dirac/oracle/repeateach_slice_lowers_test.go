package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRepeatEachSliceLowers plays the RepeatEach spellings the repeateach
// slice lowers in lockstep with gorge over the repeateachslice fixture pool
// (dirac/cardset/pools/fixtures/repeateachslice):
//
//   - a RepeatEach over players whose body is a lone DealDamage to the
//     repeated player under DamageMap$ True (Acidic Soil, Heartless
//     Hidetsugu, Stormbreath Dragon, Sunspine Lynx, Treacherous Terrain,
//     Typhoon): the loop's damage is one damage batch
//     (kop.RepeatEachDmgBatch; dirac act_repeatdmgbatch.go). The mutation
//     drops the batch bit: Spirit Link on a Sunspine Lynx then gains once per
//     damaged player instead of once for the batch.
//   - an ungated Repeat (MaxRepeat$ passes of the body; kop.OpRepeat): Another
//     Round, Exponential Growth, Full Flowering, Oath of Lim-Dul, Torment of
//     Hailfire, Varchild's War-Riders. The mutation runs each loop once.
//   - a RepeatEach over the opponents whose body mints a token tapped and
//     attacking with the plain TokenAttacking$ Remembered / RememberedPlayer
//     (Ainok Strike Leader, Endless Foot Assault, Stampede Surfer; Veteran
//     Soldier's grant needs a commander): the token attacks the repeated
//     player without Adeline's planeswalker ask (kop.TokAttackPlayer).
//     Precondition: games hold a planeswalker on the defending side when the
//     loop runs; one mutation drops the bit (the kernel then asks the
//     planeswalker pick gorge does not), another the attack.
func TestRepeatEachSliceLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		for name, min := range map[string]int{"Stampede Surfer": 10, "Ainok Strike Leader": 10, "Jace Beleren": 5, "Acidic Soil": 10, "Sunspine Lynx": 10, "Typhoon": 10, "Spirit Link": 10, "Heartless Hidetsugu": 20, "Another Round": 10, "Exponential Growth": 10, "Full Flowering": 10, "Torment of Hailfire": 20, "Oath of Lim-Dûl": 10, "Varchild's War-Riders": 20} {
			if run.casts[name] < min {
				t.Errorf("only %d %s casts (want %d)", run.casts[name], name, min)
			}
		}
	})
	t.Run("mutation/DamageBatch", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			for _, name := range []string{"Heartless Hidetsugu", "Sunspine Lynx", "Acidic Soil", "Typhoon"} {
				mutateCardIfPresent(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpRepeatEachPlayer && in.C&kop.RepeatEachDmgBatch != 0 {
						in.C &^= kop.RepeatEachDmgBatch
						n++
					}
				})
			}
		})
		if n == 0 {
			t.Fatal("no RepeatEachPlayer with the damage batch bit in the table (DamageMap$ is not lowered)")
		}
		run.requireDivergence(t, "a damage loop without its one damage batch")
	})
	t.Run("mutation/RepeatCount", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			for _, name := range []string{"Torment of Hailfire", "Exponential Growth", "Full Flowering", "Varchild's War-Riders"} {
				mutateCardIfPresent(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpRepeat {
						in.A, n = kop.Const(1), n+1
					}
				})
			}
		})
		if n == 0 {
			t.Fatal("no OpRepeat in the table (the ungated Repeat is not lowered)")
		}
		run.requireDivergence(t, "a fixed Repeat run once")
	})
	t.Run("mutation/TokenNoAttack", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			for _, name := range []string{"Ainok Strike Leader", "Endless Foot Assault", "Stampede Surfer"} {
				mutateCardIfPresent(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttackPlayer != 0 {
						in.B = kop.Const(int(in.B.N) &^ (kop.TokAttacking | kop.TokAttackPlayer))
						n++
					}
				})
			}
		})
		if n == 0 {
			t.Fatal("no CreateToken with the player-only attack bit in the table (TokenAttacking$ Remembered is not lowered)")
		}
		run.requireDivergence(t, "the repeated opponent's tokens not attacking")
	})
	t.Run("mutation/TokenPlaneswalkerAsk", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			for _, name := range []string{"Ainok Strike Leader", "Endless Foot Assault", "Stampede Surfer"} {
				mutateCardIfPresent(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpCreateTokenEx && in.B.N&kop.TokAttackPlayer != 0 {
						in.B = kop.Const(int(in.B.N) &^ kop.TokAttackPlayer)
						n++
					}
				})
			}
		})
		if n == 0 {
			t.Fatal("no CreateToken with the player-only attack bit in the table (TokenAttacking$ Remembered is not lowered)")
		}
		run.requireDivergence(t, "the player-only token rider posing Adeline's planeswalker ask")
	})
}

var repeatEachSliceWatch = map[string]bool{
	"Acidic Soil": true, "Sunspine Lynx": true, "Typhoon": true, "Spirit Link": true,
	"Heartless Hidetsugu": true, "Treacherous Terrain": true, "Stormbreath Dragon": true,
	"Ainok Strike Leader": true, "Endless Foot Assault": true, "Stampede Surfer": true,
	"Jace Beleren": true, "Ajani Goldmane": true,
	"Another Round": true, "Exponential Growth": true, "Full Flowering": true,
	"Torment of Hailfire": true, "Oath of Lim-Dûl": true, "Varchild's War-Riders": true,
}

var repeatEachSliceDecks = []fixtureDecks{
	{
		// fixed-count Repeat: X passes of a pump, a populate, an exile-and-return
		{
			{Name: "Exponential Growth", Count: 5}, {Name: "Full Flowering", Count: 5}, {Name: "Raise the Alarm", Count: 4},
			{Name: "Another Round", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Forest", Count: 8}, {Name: "Plains", Count: 4},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 10}, {Name: "Llanowar Elves", Count: 4},
		},
	},
	{
		// fixed-count Repeat over a per-opponent loss-or-sacrifice-or-discard
		// choice, a cumulative upkeep, and a draw loop
		{
			{Name: "Torment of Hailfire", Count: 6}, {Name: "Varchild's War-Riders", Count: 6},
			{Name: "Oath of Lim-Dûl", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 14}, {Name: "Mountain", Count: 6},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Swamp", Count: 10}, {Name: "Island", Count: 8}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Oath of Lim-Dûl", Count: 4},
		},
	},
	{
		// token loops against planeswalkers
		{
			{Name: "Stampede Surfer", Count: 8}, {Name: "Ainok Strike Leader", Count: 8}, {Name: "Endless Foot Assault", Count: 6},
			{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 6},
		},
		{
			{Name: "Jace Beleren", Count: 8}, {Name: "Ajani Goldmane", Count: 8}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Island", Count: 9}, {Name: "Plains", Count: 9},
		},
	},
	{
		{
			{Name: "Heartless Hidetsugu", Count: 8}, {Name: "Spirit Link", Count: 8}, {Name: "Acidic Soil", Count: 4},
			{Name: "Sunspine Lynx", Count: 4}, {Name: "Stormbreath Dragon", Count: 2},
			{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 6},
		},
		{
			{Name: "Typhoon", Count: 6}, {Name: "Treacherous Terrain", Count: 2}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 6},
		},
	},
	{
		{
			{Name: "Typhoon", Count: 6}, {Name: "Sunspine Lynx", Count: 6}, {Name: "Spirit Link", Count: 4},
			{Name: "Acidic Soil", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
		},
		{
			{Name: "Heartless Hidetsugu", Count: 6}, {Name: "Acidic Soil", Count: 4}, {Name: "Spirit Link", Count: 8},
			{Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 8},
		},
	},
}

// mutateCardIfPresent is mutateCard for a card only some matchups hold.
func mutateCardIfPresent(t *testing.T, tbl *kop.Table, name string, fn func(a *kop.Ability, in *kop.Instr)) {
	t.Helper()
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == name {
			mutateCard(t, tbl, name, fn)
			return
		}
	}
}
