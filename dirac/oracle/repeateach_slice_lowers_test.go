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
//     Hailfire, Varchild's War-Riders, and Trade Secrets (whose MaxRepeat$ is
//     the opponent's ChooseNumber answer, Count$ChosenNumber). Each pass's
//     "draw up to four cards" is its own choice. The mutation runs each loop
//     once.
//   - a RepeatEach over the opponents whose body mints a token tapped and
//     attacking with the plain TokenAttacking$ Remembered / RememberedPlayer
//     (Ainok Strike Leader, Endless Foot Assault, Stampede Surfer; Veteran
//     Soldier's grant needs a commander): the token attacks the repeated
//     player without Adeline's planeswalker ask (kop.TokAttackPlayer).
//     Precondition: games hold a planeswalker on the defending side when the
//     loop runs; one mutation drops the bit (the kernel then asks the
//     planeswalker pick gorge does not), another the attack.
//   - a RepeatEach over players whose body is a ChooseCard naming the repeated
//     player's cards (Soul Shatter, Flare of Malice, Rite of Ruin, Grave
//     Consequences, Watchers of the Dead, Disciple of
//     Caelus Nin, Fortunate Few, Ultimate Magic: Meteor, Thieves' Auction):
//     the pick is remembered for the post-loop sacrifice/exile/destroy node or
//     enters under the repeated player. The mutations move the chooser, drop
//     RememberChosen$, drop the counted pick's remember flag, replace the
//     greatest-mana-value filter, and move the stolen card's new controller.
//   - a RepeatEach over players whose body is the per-player opponent choice
//     feeding a battlefield TwoPiles (Bend or Break): the repeated player
//     picks one of their opponents, who splits their nontoken lands and picks
//     a pile; the chosen pile is destroyed, the other tapped. The mutation
//     makes the pick's pile-B answer name pile A (the chosen/unchosen piles
//     swap).
//
// Tariff (the greatest-mana-value pick), Discovery/Dispersal and Flotsam/Jetsam
// (split-card faces) lower too: Tariff's unless-payment rides the kernel's
// unpriced-chosen gate (the decline-only ask gorge's unresolvable
// DefinedCost_ChosenCard poses) and the greatest-mana-value filter on the
// zero-mana-value tokens the loop's players hold; Dispersal's greatest-mv pick
// skips the loop's phased-out tokens (CR 702.26b) like the kernel's slot-mask
// walk; Jetsam casts its graveyard pick free (the play of Defined$ Remembered
// reads the graveyard straight after the pick that remembered it) and Jetsam's
// up-to-one graveyard pick rides the counted picks' remember flag in B.
func TestRepeatEachSliceLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		for name, min := range map[string]int{"Stampede Surfer": 10, "Ainok Strike Leader": 10, "Jace Beleren": 5, "Acidic Soil": 10, "Sunspine Lynx": 10, "Typhoon": 10, "Spirit Link": 10, "Heartless Hidetsugu": 20, "Another Round": 10, "Exponential Growth": 10, "Full Flowering": 10, "Torment of Hailfire": 20, "Oath of Lim-Dûl": 10, "Varchild's War-Riders": 20, "Trade Secrets": 10,
			"Soul Shatter": 30, "Flare of Malice": 10, "Grave Consequences": 10, "Watchers of the Dead": 15,
			"Rite of Ruin": 5, "Disciple of Caelus Nin": 10,
			"Fortunate Few": 6, "Ultimate Magic: Meteor": 10, "Thieves' Auction": 5,
			"Tariff": 10, "Discovery // Dispersal": 10, "Flotsam // Jetsam": 10,
			"Bend or Break": 5} {
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
	t.Run("mutation/ChooseCardChooser", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Soul Shatter", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpChooseCard && in.Obj == kop.RefEach {
					in.Obj, n = kop.RefController, n+1
				}
			})
		})
		if n == 0 {
			t.Fatal("no ChooseCard by RefEach in the table (Defined$ Player.IsRemembered is not lowered)")
		}
		run.requireDivergence(t, "the per-player pick asked of the resolving controller")
	})
	t.Run("mutation/ChooseCardRemember", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			for _, name := range []string{"Soul Shatter", "Flare of Malice"} {
				mutateCardIfPresent(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpChooseCard && in.C == 2 { // RememberChosen$
						in.C, n = 0, n+1
					}
				})
			}
		})
		if n == 0 {
			t.Fatal("no ChooseCard with the Remember code in the table (RememberChosen$ is not lowered)")
		}
		run.requireDivergence(t, "a per-player pick that is not remembered")
	})
	t.Run("mutation/ChooseCardCountedRemember", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Watchers of the Dead", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpChooseCard && in.B.N&1 != 0 { // RememberChosen$ on the counted pick
					in.B, n = kop.Const(0), n+1
				}
			})
		})
		if n == 0 {
			t.Fatal("no counted ChooseCard with the remember flag in the table (RememberChosen$ Amount$ is not lowered)")
		}
		run.requireDivergence(t, "a counted pick that is not remembered")
	})
	t.Run("mutation/ChooseGreatestCMCFilter", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			for _, name := range []string{"Soul Shatter"} {
				mutateCardIfPresent(t, tbl, name, func(_ *kop.Ability, in *kop.Instr) {
					if in.Op == kop.OpChooseCard {
						in.F, n = 0, n+1 // any card instead of the greatest-mana-value pick
					}
				})
			}
		})
		if n == 0 {
			t.Fatal("no ChooseCard in the table (the greatest-CMC filter is not lowered)")
		}
		run.requireDivergence(t, "the greatest-mana-value filter replaced by any card")
	})
	t.Run("mutation/FortunateFewFilter", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Fortunate Few", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpChooseCard {
					in.F, n = 0, n+1 // any card instead of the opponents' (!RememberedPlayerCtrl)
				}
			})
		})
		if n == 0 {
			t.Fatal("no ChooseCard in Fortunate Few (the !RememberedPlayerCtrl filter is not lowered)")
		}
		run.requireDivergence(t, "Fortunate Few's pick widened to the chooser's own permanents")
	})
	t.Run("mutation/ThievesControl", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Thieves' Auction", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpReturnEx && in.W == kop.RefEach {
					in.W, n = kop.RefController, n+1
				}
			})
		})
		if n == 0 {
			t.Fatal("no Return entering under RefEach in the table (GainControl$ Player.IsRemembered is not lowered)")
		}
		run.requireDivergence(t, "the stolen card entering under the resolving controller")
	})
	t.Run("mutation/ChooseUpToFive", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Disciple of Caelus Nin", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpChooseCard && in.C == kop.ChooseCardAmountOpt && in.A.N == 5 {
					in.A, n = kop.Const(1), n+1
				}
			})
		})
		if n == 0 {
			t.Fatal("no up-to-five ChooseCard in the table (MinAmount$ 0 with Amount$ 5 is not lowered)")
		}
		run.requireDivergence(t, "choosing one permanent instead of up to five")
	})
	t.Run("mutation/PileSwap", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachslice", repeatEachSliceDecks, repeatEachSliceWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Bend or Break", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtTwoPilesEach {
					in.B, n = kop.Const(int(in.B.N)|1), n+1
				}
			})
		})
		if n == 0 {
			t.Fatal("no TwoPilesEach in the table (the battlefield pile split is not lowered)")
		}
		run.requireDivergence(t, "the chosen and unchosen piles swap")
	})
}

var repeatEachSliceWatch = map[string]bool{
	"Acidic Soil": true, "Sunspine Lynx": true, "Typhoon": true, "Spirit Link": true,
	"Heartless Hidetsugu": true, "Treacherous Terrain": true, "Stormbreath Dragon": true,
	"Ainok Strike Leader": true, "Endless Foot Assault": true, "Stampede Surfer": true,
	"Jace Beleren": true, "Ajani Goldmane": true,
	"Another Round": true, "Exponential Growth": true, "Full Flowering": true,
	"Torment of Hailfire": true, "Oath of Lim-Dûl": true, "Varchild's War-Riders": true, "Trade Secrets": true,
	"Soul Shatter": true, "Flare of Malice": true, "Grave Consequences": true, "Watchers of the Dead": true,
	"Rite of Ruin": true, "Disciple of Caelus Nin": true,
	"Fortunate Few": true, "Ultimate Magic: Meteor": true, "Thieves' Auction": true,
	"Tariff": true, "Discovery // Dispersal": true, "Flotsam // Jetsam": true,
	"Bend or Break": true,
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
		// Trade Secrets' chosen-number pass count: the opponent chooses how
		// many times the draw pair repeats
		{
			{Name: "Trade Secrets", Count: 4}, {Name: "Swamp", Count: 18}, {Name: "Island", Count: 18},
		},
		{
			{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 10}, {Name: "Llanowar Elves", Count: 4},
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
	{
		// per-player ChooseCard: each repeated player sacrifices (or pays
		// for) the creature with the greatest mana value, exiles graveyard
		// picks, or sacrifices a chosen-order artifact/creature/land
		{
			{Name: "Soul Shatter", Count: 5}, {Name: "Flare of Malice", Count: 2},
			{Name: "Grave Consequences", Count: 2},
			{Name: "Watchers of the Dead", Count: 3}, {Name: "Rite of Ruin", Count: 1},
			{Name: "Tariff", Count: 4},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Llanowar Elves", Count: 2},
			{Name: "Swamp", Count: 6}, {Name: "Plains", Count: 3},
			{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Jace Beleren", Count: 2}, {Name: "Ajani Goldmane", Count: 2},
			{Name: "Raise the Alarm", Count: 2}, {Name: "Llanowar Elves", Count: 2},
			{Name: "Island", Count: 6}, {Name: "Plains", Count: 6},
			{Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 4},
		},
	},
	{
		// per-player ChooseCard over the repeated player's permanents:
		// up to five picks, not-remembered
		// picks, the chosen artifact/land, and the exiled-card steal
		{
			{Name: "Disciple of Caelus Nin", Count: 2},
			{Name: "Fortunate Few", Count: 2}, {Name: "Ultimate Magic: Meteor", Count: 2},
			{Name: "Thieves' Auction", Count: 3},
			{Name: "Discovery // Dispersal", Count: 4},
			{Name: "Flotsam // Jetsam", Count: 2},
			{Name: "Llanowar Elves", Count: 2},
			{Name: "Island", Count: 5}, {Name: "Plains", Count: 5},
			{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 3},
		},
		{
			{Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 3},
			{Name: "Jace Beleren", Count: 2}, {Name: "Ajani Goldmane", Count: 2},
			{Name: "Lightning Bolt", Count: 3}, {Name: "Divination", Count: 3},
			{Name: "Mind Stone", Count: 3},
			{Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
			{Name: "Plains", Count: 5}, {Name: "Forest", Count: 5},
		},
	},
	{
		// Bend or Break: each player's nontoken lands split into two piles,
		// one of their opponents picks; the chosen pile destroyed, the other
		// tapped. A board of nontoken lands on both sides.
		{
			{Name: "Bend or Break", Count: 6}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Hill Giant", Count: 6},
			{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
		},
		{
			{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Raise the Alarm", Count: 2}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Forest", Count: 12}, {Name: "Island", Count: 12},
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
