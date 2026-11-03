package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A permanent that adds mana whenever its controller taps for mana
// (Mode$ TapsForMana | Static$ True, one literal symbol) lets a source pay more
// than it makes: gorge folds the extra into the planned tap, so the kernel's
// availability, plan and tap must count it the same, including the cast that a
// single land only pays with the extra. One deck per carrier kind: any permanent
// (Groundchuck & Dirtbag), a creature (Badgermole Cub) and a tap that declares
// {C} (Forsaken Monument, beside a source that makes {C}).
var ktapextraDecks = map[string][2][]cardset.Entry{
	"any": {
		{
			{Name: "Groundchuck & Dirtbag", Count: 4}, {Name: "Grizzly Bears", Count: 10}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Forest", Count: 14}, {Name: "Plains", Count: 8},
		},
		{{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10}},
	},
	"creature": {
		{
			{Name: "Badgermole Cub", Count: 4}, {Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Forest", Count: 14}, {Name: "Plains", Count: 8},
		},
		{{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10}},
	},
	"colourless": {
		{
			{Name: "Forsaken Monument", Count: 4}, {Name: "Sol Ring", Count: 4}, {Name: "Mind Stone", Count: 4},
			{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 6},
		},
		{{Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10}},
	},
}

func TestTapExtraLockstep(t *testing.T) {
	t.Parallel()
	for name, decks := range ktapextraDecks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lockstepSet(t, "tapextra", decks, 300, "KTAPEXTRA_SEEDS")
		})
	}
}

// TestTapExtraLockstepMutation: with the carrier's static dropped from the
// kernel's table, some seed diverges, so each pool does exercise the extra.
func TestTapExtraLockstepMutation(t *testing.T) {
	t.Parallel()
	for name, decks := range ktapextraDecks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			n := lockstepRun(t, "tapextra", decks, 200, false, func(tbl *kop.Table) {
				cards := append([]kop.CardDef(nil), tbl.Cards...)
				for i := range cards {
					var keep []kop.Ability
					for _, a := range cards[i].Abilities {
						if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatTapExtra {
							continue
						}
						keep = append(keep, a)
					}
					cards[i].Abilities = keep
				}
				tbl.Cards = cards
			})
			if n == 0 {
				t.Fatal("no seed diverged with the tap-extra static dropped")
			}
		})
	}
}

// A source whose tap brings extra mana (an earthbent land beside Badgermole Cub) makes
// gorge's window census incomplete (CR 605.1b), so a waterbend announcement may close
// without a tap.
func TestTapExtraWaterbendCensusLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "waterbend_done_with_tap_extra", "std-tla", Config{})
}

// A cast that only the extra of a tap funds is a planned payment in gorge, paid after
// its targets, not a play the oracle lowers.
func TestTapExtraPlannedCastLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "planned_cast_counts_tap_extra", "std-tla", Config{})
}

// A waterbend pick leaves the window's census: the earthbent land picked to pay for {1}
// no longer taps for mana, so its Cub extra is no unit of what remains, and gorge
// reverses the activation the remaining sources cannot pay.
func TestTapExtraWaterbendPickLeavesCensusLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "waterbend_pick_leaves_census", "std-tla", Config{})
}

// A Talisman (a painless {C} ability beside a painful coloured one) under Forsaken
// Monument: the {C} tap brings an additional {C}, the coloured one nothing, so the
// source is two colourless mana or one coloured, and the cast's X ceiling counts the
// tap's extra.
func TestTapExtraPainfulSourceLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "talisman_beside_monument", "std-trc", Config{})
}

// A source whose own ability a Continuous grant joins (Great Divide Guide's "{T}: Add one
// mana of any color" on every land and Ally) is one availability unit that carries the
// extra its tap brings, and the prefix that activates the granted ability floats it:
// gorge prices the extra on each alternative and a real activation adds it.
func TestTapExtraGrantedUnitLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "tap_extra_granted_unit", "std-tla", Config{GorgeAutopaySurface: true})
}

func TestTapExtraGrantedPrefixLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "tap_extra_granted_prefix", "std-tla", Config{GorgeAutopaySurface: true})
}

// A land that taps for several fixed colours (a basic type added by an effect) and a source
// whose amount is evaluated (Raucous Audience, an any-colour amount of the Legend of Kyoshi)
// bring the same extra as a plain tap, whichever ability or amount.
func TestTapExtraSeveralAbilitiesLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "tap_extra_several_abilities", "std-tla", Config{GorgeAutopaySurface: true})
}

func TestTapExtraEvaluatedAmountLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "tap_extra_evaluated_amount", "std-tla", Config{GorgeAutopaySurface: true})
	synthReproLockstep(t, "tap_extra_evaluated_one_colour", "std-tla", Config{GorgeAutopaySurface: true})
}

// A granted-only source's extra counts toward the offer's availability only: gorge's
// potential walk relaxes a granted ability to its own free unit, which brings none, so a
// play that extra alone funds is not searched (the Rockalanche flashback; Meditation
// Pools' activation).
func TestTapExtraGrantedWalkLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "tap_extra_granted_walk_cap", "std-tla", Config{GorgeAutopaySurface: true})
	synthReproLockstep(t, "tap_extra_granted_walk_activation", "std-tla", Config{GorgeAutopaySurface: true})
}

// A cast's payment window takes a source for the colour the cost still owes when any ability
// of it makes that colour, a granted one too: with Great Divide Guide in play a Swamp pays
// a {G} pip (gorge's clamp reads each ability the source offers), and the window reaches it
// before a later Forest.
func TestWindowGrantedColourLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "window_grant_colour_owed", "std-tla", Config{})
}

// A prefix step that taps a Combo ability tries its colours in the order gorge's ask offers
// them, the printed one: Sun-Blessed Peak's "R or W" floats {R} where W U B R G order
// would float {W}, and the difference reaches the pool the activation spends from.
func TestPrefixComboPrintedOrderLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "prefix_combo_printed_order", "std-tla", Config{})
}
